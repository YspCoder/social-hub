package shopeeads

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	tokenExchangePath = "/api/v2/auth/token/get"
	tokenRefreshPath  = "/api/v2/auth/access_token/get"
	maxOAuthBytes     = 1 << 20
)

// OAuthClient implements Shopee's signed seller authorization and rotating
// access-token flow. Shopee does not define an OAuth state parameter.
type OAuthClient struct {
	mu         sync.RWMutex
	PartnerID  int64
	PartnerKey string
	BaseURL    string
	AuthURL    string
	HTTPClient *http.Client
	Clock      socialhub.Clock
	closed     bool
}

type oauthSnapshot struct {
	partnerID  int64
	partnerKey string
	baseURL    string
	authURL    string
	httpClient *http.Client
	clock      socialhub.Clock
}

// ExchangeRequest identifies the authorization code subject. Exactly one of
// ShopID and MainAccountID is required.
type ExchangeRequest struct {
	Code          string
	ShopID        int64
	MainAccountID int64
}

type tokenExchangeBody struct {
	Code          string `json:"code"`
	PartnerID     int64  `json:"partner_id"`
	ShopID        int64  `json:"shop_id,omitempty"`
	MainAccountID int64  `json:"main_account_id,omitempty"`
}

type tokenRefreshBody struct {
	RefreshToken string `json:"refresh_token"`
	PartnerID    int64  `json:"partner_id"`
	ShopID       int64  `json:"shop_id"`
}

// TokenResult preserves all subject IDs returned by Shopee.
type TokenResult struct {
	Token        socialhub.Token
	RequestID    string
	PartnerID    int64
	PrincipalID  int64
	ShopID       int64
	MerchantID   int64
	SupplierID   int64
	UserID       int64
	ShopIDs      []int64
	MerchantIDs  []int64
	SupplierIDs  []int64
	UserIDs      []int64
	PrincipalIDs []int64
}

// AuthorizationURL creates the signed seller authorization URL.
func (client *OAuthClient) AuthorizationURL(redirectURI string) (string, error) {
	snapshot, err := client.snapshot("oauth_authorize")
	if err != nil {
		return "", err
	}
	if !validCallbackURL(redirectURI) {
		return "", invalidArgument("oauth_authorize", "redirect URI is invalid")
	}
	parsed, _ := url.Parse(snapshot.authURL)
	timestamp := snapshot.clock.Now().Unix()
	if timestamp <= 0 {
		return "", invalidArgument("oauth_authorize", "clock returned an invalid timestamp")
	}
	query := parsed.Query()
	query.Set("partner_id", formatID(snapshot.partnerID))
	query.Set("timestamp", strconv.FormatInt(timestamp, 10))
	query.Set("redirect", redirectURI)
	query.Set("sign", publicSignature(snapshot.partnerKey, snapshot.partnerID, authorizePath, timestamp))
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

// Exchange exchanges a one-time seller authorization code for a token pair.
func (client *OAuthClient) Exchange(ctx context.Context, input ExchangeRequest) (TokenResult, error) {
	shopSubject := validShopID(input.ShopID)
	mainAccountSubject := input.MainAccountID > 0
	if ctx == nil || !validOpaque(input.Code, 4096) || input.ShopID != 0 && !shopSubject ||
		input.MainAccountID < 0 || shopSubject == mainAccountSubject {
		return TokenResult{}, invalidArgument("oauth_exchange", "authorization code and exactly one shop_id or main_account_id are required")
	}
	snapshot, err := client.snapshot("oauth_exchange")
	if err != nil {
		return TokenResult{}, err
	}
	result, err := exchangeShopeeToken(ctx, "oauth_exchange", tokenExchangePath, tokenExchangeBody{
		Code: input.Code, PartnerID: snapshot.partnerID,
		ShopID: input.ShopID, MainAccountID: input.MainAccountID,
	}, "", false, snapshot)
	if err != nil {
		return TokenResult{}, err
	}
	if input.ShopID > 0 && !containsID(result.ShopIDs, input.ShopID) {
		return TokenResult{}, platformContractError("oauth_exchange", "Shopee token response was not bound to the requested shop")
	}
	return result, nil
}

// Refresh rotates a shop token pair. Shopee refresh tokens are single-use.
func (client *OAuthClient) Refresh(ctx context.Context, refreshToken string, shopID int64) (socialhub.Token, error) {
	if ctx == nil || !validOpaque(refreshToken, 16_384) || !validShopID(shopID) {
		return socialhub.Token{}, invalidArgument("oauth_refresh", "refresh token and shop_id are required")
	}
	snapshot, err := client.snapshot("oauth_refresh")
	if err != nil {
		return socialhub.Token{}, err
	}
	result, err := exchangeShopeeToken(ctx, "oauth_refresh", tokenRefreshPath, tokenRefreshBody{
		RefreshToken: refreshToken, PartnerID: snapshot.partnerID, ShopID: shopID,
	}, refreshToken, true, snapshot)
	if err != nil {
		return socialhub.Token{}, err
	}
	if result.ShopID != shopID || result.PartnerID != snapshot.partnerID {
		return socialhub.Token{}, platformContractError("oauth_refresh", "Shopee refreshed a token for a different partner or shop")
	}
	return result.Token, nil
}

type tokenFields struct {
	PartnerID    int64   `json:"partner_id"`
	PrincipalID  int64   `json:"principal_id"`
	ShopID       int64   `json:"shop_id"`
	MerchantID   int64   `json:"merchant_id"`
	SupplierID   int64   `json:"supplier_id"`
	UserID       int64   `json:"user_id"`
	AccessToken  string  `json:"access_token"`
	RefreshToken string  `json:"refresh_token"`
	ExpireIn     int64   `json:"expire_in"`
	ShopIDs      []int64 `json:"shop_id_list"`
	MerchantIDs  []int64 `json:"merchant_id_list"`
	SupplierIDs  []int64 `json:"supplier_id_list"`
	UserIDs      []int64 `json:"user_id_list"`
	PrincipalIDs []int64 `json:"principal_id_list"`
}

type tokenEnvelope struct {
	Error     string `json:"error"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
	tokenFields
	Response *tokenFields `json:"response"`
}

func exchangeShopeeToken(
	ctx context.Context,
	operation string,
	path string,
	input any,
	oldRefreshToken string,
	requireRotation bool,
	snapshot oauthSnapshot,
) (TokenResult, error) {
	started := snapshot.clock.Now()
	if !started.After(time.Unix(0, 0)) {
		return TokenResult{}, invalidArgument(operation, "clock must return a time after the Unix epoch")
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return TokenResult{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	timestamp := started.Unix()
	sign := publicSignature(snapshot.partnerKey, snapshot.partnerID, path, timestamp)
	query := url.Values{
		"partner_id": {formatID(snapshot.partnerID)},
		"timestamp":  {strconv.FormatInt(timestamp, 10)},
		"sign":       {sign},
	}
	endpoint := snapshot.baseURL + path + "?" + query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return TokenResult{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Accept-Encoding", "identity")
	request.Header.Set("Content-Type", "application/json")
	response, err := cloneHTTPClient(snapshot.httpClient).Do(request)
	if err != nil {
		return TokenResult{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxOAuthBytes+1))
	if err != nil {
		return TokenResult{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if len(body) > maxOAuthBytes {
		return TokenResult{}, platformContractError(operation, "Shopee token response exceeded 1 MiB")
	}
	sensitive := []string{snapshot.partnerKey, sign, oldRefreshToken}
	switch value := input.(type) {
	case tokenExchangeBody:
		sensitive = append(sensitive, value.Code)
	case tokenRefreshBody:
		sensitive = append(sensitive, value.RefreshToken)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return TokenResult{}, withOperation(decodeHTTPError(response.StatusCode, response.Header, body, started, sensitive...), operation)
	}
	if response.StatusCode != http.StatusOK {
		return TokenResult{}, platformContractError(operation, "Shopee returned an unexpected successful token HTTP status")
	}
	if !validJSONMediaType(response.Header.Get("Content-Type")) {
		return TokenResult{}, platformContractError(operation, "Shopee token response was not JSON")
	}
	var envelope tokenEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return TokenResult{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if envelope.Error != "" {
		return TokenResult{}, apiErrorValue(
			operation, response.StatusCode, response.Header, envelope.Error, envelope.Message,
			envelope.RequestID, started, sensitive...,
		)
	}
	fields := envelope.tokenFields
	if envelope.Response != nil && envelope.Response.AccessToken != "" {
		fields = *envelope.Response
	}
	expiresAt, ok := tokenExpiry(started, fields.ExpireIn)
	if !validOpaque(fields.AccessToken, 16_384) || !validOpaque(fields.RefreshToken, 16_384) || !ok ||
		!validTokenSubjectIDs(fields) {
		return TokenResult{}, platformContractError(operation, "Shopee returned invalid token fields")
	}
	if requireRotation && fields.RefreshToken == oldRefreshToken {
		return TokenResult{}, platformContractError(operation, "Shopee did not rotate the single-use refresh token")
	}
	return TokenResult{
		Token: socialhub.Token{
			AccessToken: fields.AccessToken, RefreshToken: fields.RefreshToken,
			TokenType: "Bearer", ExpiresAt: expiresAt,
		},
		RequestID: safeRequestID(envelope.RequestID, sensitive),
		PartnerID: fields.PartnerID, PrincipalID: fields.PrincipalID, ShopID: fields.ShopID,
		MerchantID: fields.MerchantID, SupplierID: fields.SupplierID, UserID: fields.UserID,
		ShopIDs: append([]int64(nil), fields.ShopIDs...), MerchantIDs: append([]int64(nil), fields.MerchantIDs...),
		SupplierIDs: append([]int64(nil), fields.SupplierIDs...), UserIDs: append([]int64(nil), fields.UserIDs...),
		PrincipalIDs: append([]int64(nil), fields.PrincipalIDs...),
	}, nil
}

func (client *OAuthClient) snapshot(operation string) (oauthSnapshot, error) {
	if client == nil {
		return oauthSnapshot{}, invalidArgument(operation, "OAuth client is required")
	}
	client.mu.RLock()
	defer client.mu.RUnlock()
	if client.closed {
		return oauthSnapshot{}, platformError(operation, socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	snapshot := oauthSnapshot{
		partnerID: client.PartnerID, partnerKey: client.PartnerKey, baseURL: client.BaseURL,
		authURL: client.AuthURL, httpClient: client.HTTPClient, clock: client.Clock,
	}
	if !validPartnerID(snapshot.partnerID) || !validOpaque(snapshot.partnerKey, 16_384) ||
		!validShopeeOrigin(snapshot.baseURL) || snapshot.authURL != snapshot.baseURL+authorizePath ||
		snapshot.httpClient == nil || snapshot.clock == nil {
		return oauthSnapshot{}, invalidArgument(operation, "OAuth client is incomplete")
	}
	return snapshot, nil
}

func tokenExpiry(now time.Time, value int64) (time.Time, bool) {
	if value <= 0 {
		return time.Time{}, false
	}
	var expiresAt time.Time
	if value >= 1_000_000_000 {
		expiresAt = time.Unix(value, 0)
	} else {
		expiresAt = now.Add(time.Duration(value) * time.Second)
	}
	if !expiresAt.After(now) || expiresAt.After(now.Add(24*time.Hour)) {
		return time.Time{}, false
	}
	return expiresAt, true
}

// Close clears OAuth credentials and prevents new authorization work.
func (client *OAuthClient) Close() {
	if client == nil {
		return
	}
	client.mu.Lock()
	client.PartnerID, client.PartnerKey, client.BaseURL, client.AuthURL = 0, "", "", ""
	client.HTTPClient, client.Clock, client.closed = nil, nil, true
	client.mu.Unlock()
}

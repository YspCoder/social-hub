package jinghongads

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

const maxOAuthResponseBytes int64 = 1 << 20

// OAuthClient implements Huawei OAuth 2.0 authorization-code and refresh
// flows. The mainland credential contract requires an HTTPS callback without a
// query string and the six documented Marketing API scopes.
type OAuthClient struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	HTTPClient   *http.Client
	Clock        socialhub.Clock
}

func (client *OAuthClient) AuthorizationURL(redirectURI, state string) (string, error) {
	if !validClientID(client.ClientID) || !validCallbackURL(redirectURI) || !validOpaque(state, 1024) || client.AuthURL != defaultAuthURL {
		return "", invalidArgument("oauth_authorize", "client ID, HTTPS redirect URI, state, or authorization endpoint is invalid")
	}
	parsed, _ := url.Parse(client.AuthURL)
	query := parsed.Query()
	query.Set("response_type", "code")
	query.Set("client_id", client.ClientID)
	query.Set("scope", strings.Join(requiredOAuthScopes, " "))
	query.Set("redirect_uri", redirectURI)
	query.Set("access_type", "offline")
	query.Set("state", state)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (client *OAuthClient) Exchange(ctx context.Context, code, redirectURI string) (socialhub.Token, error) {
	if !validOpaque(code, 8192) || !validCallbackURL(redirectURI) {
		return socialhub.Token{}, invalidArgument("oauth_exchange", "authorization code and HTTPS redirect URI are required")
	}
	values := url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {redirectURI},
		"client_id": {client.ClientID}, "client_secret": {client.ClientSecret},
	}
	return client.token(ctx, "oauth_exchange", values, "", true)
}

func (client *OAuthClient) Refresh(ctx context.Context, refreshToken string) (socialhub.Token, error) {
	if !validOpaque(refreshToken, 8192) {
		return socialhub.Token{}, invalidArgument("oauth_refresh", "refresh token is required")
	}
	values := url.Values{
		"grant_type": {"refresh_token"}, "refresh_token": {refreshToken},
		"client_id": {client.ClientID}, "client_secret": {client.ClientSecret},
	}
	return client.token(ctx, "oauth_refresh", values, refreshToken, false)
}

func (client *OAuthClient) token(ctx context.Context, operation string, values url.Values, existingRefreshToken string, requireRefreshToken bool) (socialhub.Token, error) {
	if !validClientID(client.ClientID) || !validOpaque(client.ClientSecret, 8192) || client.HTTPClient == nil ||
		client.Clock == nil || client.TokenURL != defaultTokenURL {
		return socialhub.Token{}, invalidArgument(operation, "OAuth client is incomplete")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.TokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := client.HTTPClient.Do(request)
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxOAuthResponseBytes+1))
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(body)) > maxOAuthResponseBytes {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("OAuth response exceeded size limit"))
	}
	var payload struct {
		AccessToken  string          `json:"access_token"`
		ExpiresIn    json.RawMessage `json:"expires_in"`
		RefreshToken string          `json:"refresh_token"`
		Scope        string          `json:"scope"`
		TokenType    string          `json:"token_type"`
		Error        json.RawMessage `json:"error"`
		SubError     json.RawMessage `json:"sub_error"`
	}
	decodeErr := json.Unmarshal(body, &payload)
	platformErrorCode := scalarCode(payload.Error)
	if response.StatusCode < 200 || response.StatusCode >= 300 || platformErrorCode != "" {
		if decodeErr != nil {
			return socialhub.Token{}, withOperation(decodeHTTPErrorAt(response.StatusCode, response.Header, body, client.Clock.Now()), operation)
		}
		return socialhub.Token{}, oauthError(operation, response.StatusCode, response.Header, platformErrorCode, scalarCode(payload.SubError), client.Clock.Now())
	}
	if decodeErr != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, decodeErr)
	}
	expiresIn, err := decodeNonnegativeInt(payload.ExpiresIn, int((7*24*time.Hour)/time.Second))
	if err != nil || expiresIn == 0 {
		return socialhub.Token{}, platformContractError(operation, "Huawei OAuth returned an invalid expiry")
	}
	refreshToken := payload.RefreshToken
	if refreshToken == "" {
		refreshToken = existingRefreshToken
	}
	if !validOpaque(payload.AccessToken, 8192) || !validOpaque(refreshToken, 8192) || requireRefreshToken && payload.RefreshToken == "" {
		return socialhub.Token{}, platformContractError(operation, "Huawei OAuth returned incomplete token credentials")
	}
	if payload.TokenType != "" && !strings.EqualFold(payload.TokenType, "bearer") {
		return socialhub.Token{}, platformContractError(operation, "Huawei OAuth returned an unsupported token type")
	}
	return socialhub.Token{
		AccessToken: payload.AccessToken, RefreshToken: refreshToken, TokenType: "Bearer",
		ExpiresAt: client.Clock.Now().Add(time.Duration(expiresIn) * time.Second), Scopes: strings.Fields(payload.Scope),
	}, nil
}

func oauthError(operation string, status int, header http.Header, platformCode, subError string, now time.Time) error {
	code, class := socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	parameterErrors := map[string]struct{}{
		"20001": {}, "20002": {}, "20171": {}, "20172": {}, "20174": {}, "20175": {},
		"20181": {}, "20182": {}, "12303": {}, "12304": {}, "20191": {}, "20192": {},
		"20154": {}, "31218": {}, "31202": {},
	}
	if _, found := parameterErrors[subError]; found {
		code, class = socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	}
	if subError == "11205" || subError == "31204" {
		code, class = socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	}
	if status == http.StatusTooManyRequests {
		code, class = socialhub.CodeRateLimited, socialhub.ClassRetryable
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation, HTTPStatus: status,
		PlatformCode:    boundedMessage(joinPlatformCodes(platformCode, subError), 128),
		PlatformMessage: "Huawei OAuth token request failed",
		RequestID:       firstSafeHeader(header, 256, "X-Request-ID", "X-Correlation-ID"),
		RetryAfter:      parseRetryAfter(header.Get("Retry-After"), now),
	}
}

package petalads

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

const maxOAuthResponseBytes int64 = 1 << 20

// OAuthClient implements Huawei's OAuth 2.0 authorization-code and refresh
// flows for Petal Ads. Authorization URLs always request offline access and the
// six scopes prescribed by the current official contract.
type OAuthClient struct {
	clientID     string
	clientSecret string
	httpClient   *http.Client
	clock        socialhub.Clock
}

func (client *OAuthClient) AuthorizationURL(redirectURI, state string) (string, error) {
	if !validClientID(client.clientID) || !validCallbackURL(redirectURI) || !validOpaque(state, 1024) {
		return "", invalidArgument("oauth_authorize", "client ID, redirect URI, or state is invalid")
	}
	parsed, _ := url.Parse(defaultAuthURL)
	query := parsed.Query()
	query.Set("response_type", "code")
	query.Set("client_id", client.clientID)
	query.Set("scope", strings.Join(requiredOAuthScopes, " "))
	query.Set("redirect_uri", redirectURI)
	query.Set("access_type", "offline")
	query.Set("state", state)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (client *OAuthClient) Exchange(ctx context.Context, code, redirectURI string) (socialhub.Token, error) {
	if !validOpaque(code, 8192) || !validCallbackURL(redirectURI) {
		return socialhub.Token{}, invalidArgument("oauth_exchange", "authorization code and redirect URI are required")
	}
	values := url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {redirectURI},
		"client_id": {client.clientID}, "client_secret": {client.clientSecret},
	}
	return client.token(ctx, "oauth_exchange", values, "", true)
}

func (client *OAuthClient) Refresh(ctx context.Context, refreshToken string) (socialhub.Token, error) {
	if !validOpaque(refreshToken, 8192) {
		return socialhub.Token{}, invalidArgument("oauth_refresh", "refresh token is required")
	}
	values := url.Values{
		"grant_type": {"refresh_token"}, "refresh_token": {refreshToken},
		"client_id": {client.clientID}, "client_secret": {client.clientSecret},
	}
	return client.token(ctx, "oauth_refresh", values, refreshToken, false)
}

func (client *OAuthClient) token(ctx context.Context, operation string, values url.Values, existingRefreshToken string, requireRefreshToken bool) (socialhub.Token, error) {
	if !validClientID(client.clientID) || !validOpaque(client.clientSecret, 8192) || client.httpClient == nil ||
		client.clock == nil {
		return socialhub.Token{}, invalidArgument(operation, "OAuth client is incomplete")
	}
	now := client.clock.Now()
	if now.Unix() <= 0 {
		return socialhub.Token{}, invalidArgument(operation, "clock must return a time after the Unix epoch")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, defaultTokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Accept-Encoding", "identity")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := client.httpClient.Do(request)
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxOAuthResponseBytes+1))
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(body)) > maxOAuthResponseBytes {
		return socialhub.Token{}, platformContractError(operation, "Huawei OAuth response exceeded 1 MiB")
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
	platformErrorCode := numericCode(payload.Error)
	if response.StatusCode < 200 || response.StatusCode >= 300 || platformErrorCode != "" {
		if decodeErr != nil {
			return socialhub.Token{}, withOperation(decodeHTTPErrorAt(
				response.StatusCode, response.Header, body, now,
				client.clientID, client.clientSecret, existingRefreshToken, values.Get("code"), values.Get("refresh_token"),
			), operation)
		}
		return socialhub.Token{}, oauthError(
			operation, response.StatusCode, response.Header, platformErrorCode, numericCode(payload.SubError), now,
			client.clientID, client.clientSecret, existingRefreshToken, values.Get("code"), values.Get("refresh_token"),
		)
	}
	if decodeErr != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, decodeErr)
	}
	if !validJSONContentType(response.Header.Get("Content-Type")) {
		return socialhub.Token{}, platformContractError(operation, "Huawei OAuth success response was not application/json")
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
	if !strings.EqualFold(payload.TokenType, "bearer") {
		return socialhub.Token{}, platformContractError(operation, "Huawei OAuth returned an unsupported token type")
	}
	scopes := strings.Fields(payload.Scope)
	if len(scopes) == 0 {
		scopes = RequiredOAuthScopes()
	} else if !completeOAuthScopes(scopes) {
		return socialhub.Token{}, platformContractError(operation, "Huawei OAuth returned invalid scopes")
	}
	scopes = RequiredOAuthScopes()
	return socialhub.Token{
		AccessToken: payload.AccessToken, RefreshToken: refreshToken, TokenType: "Bearer",
		ExpiresAt: now.Add(time.Duration(expiresIn) * time.Second), Scopes: scopes,
	}, nil
}

func oauthError(operation string, status int, header http.Header, platformCode, subError string, now time.Time, requestIDValues ...string) error {
	code, class := classifyHTTPError(status)
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
	combinedCode := strings.Trim(platformCode+"/"+subError, "/")
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation, HTTPStatus: status,
		PlatformCode: boundedOpaque(combinedCode, 128), PlatformMessage: "Huawei OAuth rejected the request",
		RequestID:  responseRequestID(header, requestIDValues...),
		RetryAfter: parseRetryAfter(header.Get("Retry-After"), now),
	}
}

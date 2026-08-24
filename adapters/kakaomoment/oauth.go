package kakaomoment

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

const maxOAuthResponseBytes int64 = 1 << 20

// OAuthClient implements account-bound Kakao Business Authentication. Kakao
// issues no refresh token; an expired or revoked grant must be authorized again.
type OAuthClient struct {
	ClientID     string
	ClientSecret string
	AdAccountID  int64
	HTTPClient   *http.Client
}

func (client *OAuthClient) AuthorizationURL(redirectURI, state string, scopes ...string) (string, error) {
	if !validOpaque(client.ClientID, 1024) || client.AdAccountID <= 0 || !validCallbackURL(redirectURI) ||
		!validOpaque(state, 1024) || len(scopes) == 0 || !validScopes(scopes) {
		return "", invalidArgument("oauth_authorize", "client ID, ad account, redirect URI, state, or scopes are invalid")
	}
	for _, scope := range scopes {
		if scope != ScopeManagement && scope != ScopeDelete {
			return "", invalidArgument("oauth_authorize", "this adapter requests only moment_management and moment_delete")
		}
	}
	parsed, _ := url.Parse(defaultAuthURL)
	query := parsed.Query()
	query.Set("client_id", client.ClientID)
	query.Set("response_type", "code")
	query.Set("redirect_uri", redirectURI)
	query.Set("scope", strings.Join(scopes, ","))
	query.Set("resource_ids", "moment:"+formatID(client.AdAccountID))
	query.Set("state", state)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (client *OAuthClient) Exchange(ctx context.Context, code, redirectURI string) (socialhub.Token, error) {
	const operation = "oauth_exchange"
	if !validOpaque(client.ClientID, 1024) || client.AdAccountID <= 0 || !validOpaque(code, 4096) ||
		!validCallbackURL(redirectURI) || client.HTTPClient == nil {
		return socialhub.Token{}, invalidArgument(operation, "OAuth client, authorization code, or redirect URI is invalid")
	}
	values := url.Values{
		"grant_type": {"authorization_code"}, "client_id": {client.ClientID},
		"code": {code}, "redirect_uri": {redirectURI},
	}
	if client.ClientSecret != "" {
		values.Set("client_secret", client.ClientSecret)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, defaultTokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")
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
		return socialhub.Token{}, platformContractError(operation, "Business Authentication response exceeded 1 MiB")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return socialhub.Token{}, decodeOAuthError(response.StatusCode, body, operation)
	}
	var payload struct {
		TokenType   string `json:"token_type"`
		AccessToken string `json:"access_token"`
		Scope       string `json:"scope"`
	}
	if json.Unmarshal(body, &payload) != nil || !validOpaque(payload.AccessToken, 16_384) ||
		!strings.EqualFold(payload.TokenType, "bearer") || len(payload.Scope) > 64<<10 {
		return socialhub.Token{}, platformContractError(operation, "Business Authentication returned an invalid token response")
	}
	scopes := strings.Fields(payload.Scope)
	if len(scopes) == 0 || !validScopes(scopes) {
		return socialhub.Token{}, platformContractError(operation, "Business Authentication returned invalid scopes")
	}
	return socialhub.Token{
		AccessToken: payload.AccessToken, TokenType: "Bearer", Scopes: scopes,
	}, nil
}

func decodeOAuthError(status int, body []byte, operation string) error {
	var response struct {
		Error       string `json:"error"`
		Description string `json:"error_description"`
		ErrorCode   string `json:"error_code"`
	}
	_ = json.Unmarshal(body, &response)
	code, class := socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	platformCode := firstNonEmpty(response.ErrorCode, response.Error)
	switch {
	case status == http.StatusTooManyRequests:
		code, class = socialhub.CodeRateLimited, socialhub.ClassRetryable
	case response.Error == "invalid_client" || response.Error == "invalid_grant" || status == http.StatusUnauthorized:
		code, class = socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case response.Error == "access_denied" || status == http.StatusForbidden:
		code, class = socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case response.Error == "temporarily_unavailable" || response.Error == "server_error" || status >= 500:
		code, class = socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		HTTPStatus: status, PlatformCode: boundedOpaque(platformCode, 128),
		PlatformMessage: "Kakao Business Authentication rejected the request",
	}
}

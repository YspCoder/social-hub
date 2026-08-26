package ebaybrowse

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

// OAuthClient implements eBay's OAuth 2.0 Client Credentials grant for an
// Application access token.
type OAuthClient struct {
	ClientID     string
	ClientSecret string
	TokenURL     string
	HTTPClient   *http.Client
	Clock        socialhub.Clock
}

// ClientCredentials obtains a short-lived Application access token. This
// grant has no refresh token; expiry is handled by minting another token.
func (client *OAuthClient) ClientCredentials(ctx context.Context) (socialhub.Token, error) {
	if !validOpaque(client.ClientID, 1024) || !validOpaque(client.ClientSecret, 16_384) ||
		client.HTTPClient == nil || client.Clock == nil || !validEndpoint(client.TokenURL) {
		return socialhub.Token{}, invalidArgument("oauth_client_credentials", "client ID, client secret, token URL, HTTP client, and clock are required")
	}
	values := url.Values{"grant_type": {"client_credentials"}, "scope": {applicationScope}}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.TokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return socialhub.Token{}, platformError("oauth_client_credentials", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.SetBasicAuth(client.ClientID, client.ClientSecret)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := client.HTTPClient.Do(request)
	if err != nil {
		return socialhub.Token{}, platformError("oauth_client_credentials", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxOAuthResponseBytes+1))
	if err != nil {
		return socialhub.Token{}, platformError("oauth_client_credentials", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(body)) > maxOAuthResponseBytes {
		return socialhub.Token{}, platformError("oauth_client_credentials", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("OAuth response exceeded size limit"))
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return socialhub.Token{}, decodeOAuthError(
			response.StatusCode, response.Header, body, client.Clock.Now(), client.ClientID, client.ClientSecret,
		)
	}
	if !validJSONContentType(response.Header.Get("Content-Type")) {
		return socialhub.Token{}, platformContractError("oauth_client_credentials", "eBay returned a non-JSON OAuth response")
	}
	var payload struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return socialhub.Token{}, platformError("oauth_client_credentials", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if !validOpaque(payload.AccessToken, 16_384) || payload.ExpiresIn <= 0 ||
		payload.ExpiresIn > int64((24*time.Hour)/time.Second) || !validOpaque(payload.TokenType, 128) {
		return socialhub.Token{}, platformError("oauth_client_credentials", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("invalid token response fields"))
	}
	return socialhub.Token{
		AccessToken: payload.AccessToken, TokenType: "Bearer",
		ExpiresAt: client.Clock.Now().Add(time.Duration(payload.ExpiresIn) * time.Second),
		Scopes:    []string{applicationScope},
	}, nil
}

func decodeOAuthError(status int, header http.Header, body []byte, now time.Time, secrets ...string) error {
	var payload struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	_ = json.Unmarshal(body, &payload)
	code, class := classifyHTTPError(status)
	switch strings.ToLower(strings.TrimSpace(payload.Error)) {
	case "invalid_client":
		code, class = socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "invalid_scope":
		code, class = socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	case "invalid_request", "unsupported_grant_type":
		code, class = socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	}
	result := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: "oauth_client_credentials",
		HTTPStatus: status, PlatformCode: boundedMessage(redactSensitive(redactExact(payload.Error, secrets...)), 256),
		PlatformMessage: boundedMessage(redactSensitive(redactExact(payload.ErrorDescription, secrets...)), 1024),
		RequestID:       boundedMessage(firstHeader(header, "X-EBAY-C-REQUEST-ID", "X-EBAY-CORRELATION-ID", "X-Request-ID"), 256),
		RetryAfter:      parseRetryAfter(header.Get("Retry-After"), now),
	}
	if code == socialhub.CodeApprovalRequired {
		result.RequiredScopes = []string{applicationScope}
		result.ApprovalURL = documentationURL
	}
	return result
}

package criteo

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

// OAuthClient implements Criteo's OAuth 2.0 Client Credentials grant.
type OAuthClient struct {
	ClientID     string
	ClientSecret string
	TokenURL     string
	HTTPClient   *http.Client
	Clock        socialhub.Clock
}

// ClientCredentials obtains a short-lived bearer token. Criteo does not issue
// a refresh token for this grant; callers obtain another access token.
func (client *OAuthClient) ClientCredentials(ctx context.Context) (socialhub.Token, error) {
	if !validOpaque(client.ClientID, 1024) || !validOpaque(client.ClientSecret, 16384) ||
		client.HTTPClient == nil || client.Clock == nil || !validEndpoint(client.TokenURL) {
		return socialhub.Token{}, invalidArgument("oauth_client_credentials", "client ID, client secret, token URL, HTTP client, and clock are required")
	}
	values := url.Values{
		"client_id": {client.ClientID}, "client_secret": {client.ClientSecret}, "grant_type": {"client_credentials"},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.TokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return socialhub.Token{}, platformError("oauth_client_credentials", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
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
		return socialhub.Token{}, decodeOAuthError(response.StatusCode, response.Header, body)
	}
	var payload struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return socialhub.Token{}, platformError("oauth_client_credentials", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if !validOpaque(payload.AccessToken, 16384) || payload.ExpiresIn <= 0 || payload.ExpiresIn > int64((24*time.Hour)/time.Second) ||
		(payload.TokenType != "" && !strings.EqualFold(payload.TokenType, "bearer")) {
		return socialhub.Token{}, platformError("oauth_client_credentials", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("invalid token response fields"))
	}
	return socialhub.Token{
		AccessToken: payload.AccessToken, TokenType: "Bearer",
		ExpiresAt: client.Clock.Now().Add(time.Duration(payload.ExpiresIn) * time.Second),
		Scopes:    append([]string(nil), managedScopes...),
	}, nil
}

func decodeOAuthError(status int, header http.Header, body []byte) error {
	var payload struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	_ = json.Unmarshal(body, &payload)
	code, class := classifyHTTPError(status)
	if strings.EqualFold(strings.TrimSpace(payload.Error), "invalid_client") {
		code, class = socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: "oauth_client_credentials",
		HTTPStatus: status, PlatformCode: boundedMessage(redactSensitive(payload.Error), 256),
		PlatformMessage: boundedMessage(redactSensitive(payload.ErrorDescription), 512),
		RequestID:       boundedMessage(firstNonEmpty(header.Get("x-request-id"), header.Get("x-correlation-id")), 256),
		RetryAfter:      retryDelay(header),
	}
}

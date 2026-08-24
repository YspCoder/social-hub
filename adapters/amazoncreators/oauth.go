package amazoncreators

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

const maxOAuthResponseBytes int64 = 1 << 20

// OAuthClient implements the current Amazon Creators API v3 Login with Amazon
// Client Credentials contract.
type OAuthClient struct {
	CredentialID      string
	CredentialSecret  string
	CredentialVersion string
	HTTPClient        *http.Client
	Clock             socialhub.Clock
}

// ClientCredentials obtains a short-lived bearer token. The grant has no
// refresh token; callers obtain a new token before expiry.
func (client *OAuthClient) ClientCredentials(ctx context.Context) (socialhub.Token, error) {
	if !validOpaque(client.CredentialID, 1024) || !validOpaque(client.CredentialSecret, 16_384) ||
		client.HTTPClient == nil || client.Clock == nil || !validCredentialVersion(client.CredentialVersion) {
		return socialhub.Token{}, invalidArgument("oauth_client_credentials", "credential ID, credential secret, credential version, HTTP client, and clock are required")
	}
	payload, err := json.Marshal(struct {
		GrantType    string `json:"grant_type"`
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		Scope        string `json:"scope"`
	}{
		GrantType: "client_credentials", ClientID: client.CredentialID,
		ClientSecret: client.CredentialSecret, Scope: oauthScope,
	})
	if err != nil {
		return socialhub.Token{}, platformError("oauth_client_credentials", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint(client.CredentialVersion), bytes.NewReader(payload))
	if err != nil {
		return socialhub.Token{}, platformError("oauth_client_credentials", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Accept-Encoding", "identity")
	request.Header.Set("Content-Type", "application/json")
	response, err := cloneHTTPClient(client.HTTPClient).Do(request)
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
		return socialhub.Token{}, decodeOAuthError(response.StatusCode, response.Header, body, client.Clock.Now())
	}
	var result struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return socialhub.Token{}, platformError("oauth_client_credentials", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if !validOpaque(result.AccessToken, 16_384) || result.ExpiresIn <= 0 ||
		result.ExpiresIn > int64((24*time.Hour)/time.Second) ||
		(result.TokenType != "" && !strings.EqualFold(result.TokenType, "Bearer")) {
		return socialhub.Token{}, platformError("oauth_client_credentials", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("invalid token response fields"))
	}
	return socialhub.Token{
		AccessToken: result.AccessToken, TokenType: "Bearer",
		ExpiresAt: client.Clock.Now().Add(time.Duration(result.ExpiresIn) * time.Second),
		Scopes:    []string{oauthScope},
	}, nil
}

func decodeOAuthError(status int, header http.Header, body []byte, now time.Time) error {
	var payload struct {
		Error string `json:"error"`
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
		HTTPStatus: status, PlatformCode: boundedMessage(redactSensitive(payload.Error), 256),
		PlatformMessage: "Amazon rejected the Client Credentials request",
		RequestID:       boundedMessage(firstHeader(header, "x-amzn-RequestId", "x-amzn-requestid", "X-Request-ID"), 256),
		RetryAfter:      parseRetryAfter(header.Get("Retry-After"), now),
	}
	if code == socialhub.CodeApprovalRequired {
		result.RequiredScopes = []string{oauthScope}
		result.ApprovalURL = documentationURL
	}
	return result
}

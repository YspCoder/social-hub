package line

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

const maxTokenResponseBytes int64 = 1 << 20

// TokenClient manages short-lived, stateless, and legacy long-lived channel
// access tokens. It does not persist issued tokens.
type TokenClient struct {
	ChannelID     string
	ChannelSecret string
	BaseURL       string
	HTTPClient    *http.Client
	Clock         socialhub.Clock
}

func (c *TokenClient) IssueShortLived(ctx context.Context) (socialhub.Token, error) {
	return c.issue(ctx, "/v2/oauth/accessToken", "issue_short_lived_token")
}

func (c *TokenClient) IssueStateless(ctx context.Context) (socialhub.Token, error) {
	return c.issue(ctx, "/oauth2/v3/token", "issue_stateless_token")
}

func (c *TokenClient) issue(ctx context.Context, path, operation string) (socialhub.Token, error) {
	if err := c.validate(operation); err != nil {
		return socialhub.Token{}, err
	}
	var response struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	err := c.form(ctx, path, url.Values{
		"grant_type": {"client_credentials"}, "client_id": {c.ChannelID}, "client_secret": {c.ChannelSecret},
	}, &response, operation)
	if err != nil {
		return socialhub.Token{}, err
	}
	if strings.TrimSpace(response.AccessToken) == "" || response.ExpiresIn <= 0 || response.ExpiresIn > int64((365*24*time.Hour)/time.Second) {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	tokenType := response.TokenType
	if tokenType == "" || strings.EqualFold(tokenType, "bearer") {
		tokenType = "Bearer"
	}
	return socialhub.Token{
		AccessToken: response.AccessToken, TokenType: tokenType,
		ExpiresAt: c.Clock.Now().Add(time.Duration(response.ExpiresIn) * time.Second),
	}, nil
}

func (c *TokenClient) Verify(ctx context.Context, accessToken string) (*TokenInfo, error) {
	if err := c.validate("verify_token"); err != nil {
		return nil, err
	}
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return nil, invalidArgument("verify_token", "channel access token is required")
	}
	var response struct {
		ClientID  string `json:"client_id"`
		ExpiresIn int64  `json:"expires_in"`
		Scope     string `json:"scope"`
	}
	if err := c.form(ctx, "/v2/oauth/verify", url.Values{"access_token": {accessToken}}, &response, "verify_token"); err != nil {
		return nil, err
	}
	maxSeconds := int64((time.Duration(1<<63 - 1)) / time.Second)
	if strings.TrimSpace(response.ClientID) == "" || response.ExpiresIn < 0 || response.ExpiresIn > maxSeconds {
		return nil, platformError("verify_token", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &TokenInfo{
		ChannelID: response.ClientID, ExpiresAt: c.Clock.Now().Add(time.Duration(response.ExpiresIn) * time.Second),
		Scopes: strings.Fields(response.Scope),
	}, nil
}

func (c *TokenClient) Revoke(ctx context.Context, accessToken string) error {
	if err := c.validate("revoke_token"); err != nil {
		return err
	}
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return invalidArgument("revoke_token", "channel access token is required")
	}
	return c.form(ctx, "/v2/oauth/revoke", url.Values{"access_token": {accessToken}}, nil, "revoke_token")
}

func (c *TokenClient) validate(operation string) error {
	if strings.TrimSpace(c.ChannelID) == "" || strings.TrimSpace(c.ChannelSecret) == "" || c.HTTPClient == nil || c.Clock == nil || !validEndpoint(c.BaseURL) {
		return invalidArgument(operation, "token client is incomplete")
	}
	return nil
}

func (c *TokenClient) form(ctx context.Context, path string, values url.Values, output any, operation string) error {
	base, err := url.Parse(c.BaseURL)
	if err != nil {
		return invalidArgument(operation, "token base URL is invalid")
	}
	requestURL := *base
	requestURL.Path = strings.TrimRight(base.Path, "/") + path
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL.String(), strings.NewReader(values.Encode()))
	if err != nil {
		return platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxTokenResponseBytes+1))
	if err != nil {
		return platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(body)) > maxTokenResponseBytes {
		return platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return decodeTokenError(response.StatusCode, response.Header, body, operation)
	}
	if output == nil {
		return nil
	}
	if len(body) == 0 || json.Unmarshal(body, output) != nil {
		return platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("invalid token response"))
	}
	return nil
}

func decodeTokenError(status int, header http.Header, body []byte, operation string) error {
	var response struct {
		Error       string `json:"error"`
		Description string `json:"error_description"`
	}
	_ = json.Unmarshal(body, &response)
	code, class := socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	if status == http.StatusUnauthorized || response.Error == "invalid_client" {
		code, class = socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	} else if status == http.StatusForbidden {
		code, class = socialhub.CodePermissionDenied, socialhub.ClassUserAction
	} else if status == http.StatusTooManyRequests {
		code, class = socialhub.CodeRateLimited, socialhub.ClassRetryable
	} else if status >= 500 {
		code, class = socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: "line", Product: productName, Op: operation, HTTPStatus: status,
		PlatformCode: response.Error, PlatformMessage: boundedMessage(response.Description, 512),
		RequestID: header.Get("X-Line-Request-Id"), RetryAfter: retryAfter(header.Get("Retry-After")),
	}
}

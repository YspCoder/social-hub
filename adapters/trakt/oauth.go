package trakt

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

const maxOAuthResponseBytes int64 = 1 << 20

// AuthorizationRequest configures Trakt's browser authorization URL.
type AuthorizationRequest struct {
	RedirectURI string
	State       string
	Signup      bool
	ForceLogin  bool
}

// DeviceAuthorization contains the values needed to display and poll a device flow.
type DeviceAuthorization struct {
	DeviceCode      string        `json:"device_code"`
	UserCode        string        `json:"user_code"`
	VerificationURL string        `json:"verification_url"`
	ExpiresAt       time.Time     `json:"expires_at"`
	Interval        time.Duration `json:"interval"`
}

// AuthorizationURL builds the browser-based OAuth authorization URL.
func (c *Client) AuthorizationURL(input AuthorizationRequest) (string, error) {
	if !validRedirectURI(input.RedirectURI) || !validCredential(input.State) {
		return "", invalidArgument("oauth_authorize", "redirect URI and state are required")
	}
	parsed, _ := url.Parse(strings.TrimRight(c.authURL, "/") + "/oauth/authorize")
	query := parsed.Query()
	query.Set("response_type", "code")
	query.Set("client_id", c.clientID)
	query.Set("redirect_uri", input.RedirectURI)
	query.Set("state", input.State)
	if input.Signup {
		query.Set("signup", "true")
	}
	if input.ForceLogin {
		query.Set("prompt", "login")
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

// Exchange trades an authorization code for a seven-day access token.
func (c *Client) Exchange(ctx context.Context, code, redirectURI string) (socialhub.Token, error) {
	if err := c.requireSecret("oauth_exchange"); err != nil {
		return socialhub.Token{}, err
	}
	if !validCredential(code) || !validRedirectURI(redirectURI) {
		return socialhub.Token{}, invalidArgument("oauth_exchange", "code and redirect URI are required")
	}
	return c.exchangeToken(ctx, map[string]any{
		"code": code, "client_id": c.clientID, "client_secret": c.clientSecret,
		"redirect_uri": redirectURI, "grant_type": "authorization_code",
	}, "oauth_exchange")
}

// Refresh rotates Trakt's single-use refresh token.
func (c *Client) Refresh(ctx context.Context, refreshToken, redirectURI string) (socialhub.Token, error) {
	if err := c.requireSecret("oauth_refresh"); err != nil {
		return socialhub.Token{}, err
	}
	if !validCredential(refreshToken) || !validRedirectURI(redirectURI) {
		return socialhub.Token{}, invalidArgument("oauth_refresh", "refresh token and redirect URI are required")
	}
	return c.exchangeToken(ctx, map[string]any{
		"refresh_token": refreshToken, "client_id": c.clientID, "client_secret": c.clientSecret,
		"redirect_uri": redirectURI, "grant_type": "refresh_token",
	}, "oauth_refresh")
}

// RequestDeviceCode starts Trakt's device authorization flow.
func (c *Client) RequestDeviceCode(ctx context.Context) (*DeviceAuthorization, error) {
	var response struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURL string `json:"verification_url"`
		ExpiresIn       int64  `json:"expires_in"`
		Interval        int64  `json:"interval"`
	}
	if err := c.oauthJSON(ctx, "/oauth/device/code", map[string]string{"client_id": c.clientID}, &response, "oauth_device_code"); err != nil {
		return nil, err
	}
	if !validCredential(response.DeviceCode) || !validCredential(response.UserCode) ||
		!validEndpoint(response.VerificationURL) || response.ExpiresIn <= 0 || response.Interval <= 0 {
		return nil, platformError("oauth_device_code", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &DeviceAuthorization{
		DeviceCode: response.DeviceCode, UserCode: response.UserCode, VerificationURL: response.VerificationURL,
		ExpiresAt: c.clock.Now().Add(time.Duration(response.ExpiresIn) * time.Second),
		Interval:  time.Duration(response.Interval) * time.Second,
	}, nil
}

// PollDevice performs one device-token poll. Callers must honor authorization.Interval.
func (c *Client) PollDevice(ctx context.Context, authorization DeviceAuthorization) (socialhub.Token, error) {
	if err := c.requireSecret("oauth_device_poll"); err != nil {
		return socialhub.Token{}, err
	}
	if !validCredential(authorization.DeviceCode) || authorization.Interval <= 0 || authorization.ExpiresAt.IsZero() {
		return socialhub.Token{}, invalidArgument("oauth_device_poll", "device authorization is invalid")
	}
	if !c.clock.Now().Before(authorization.ExpiresAt) {
		return socialhub.Token{}, &socialhub.Error{
			Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction,
			Platform: "trakt", Product: productName, Op: "oauth_device_poll", PlatformCode: "expired_token",
		}
	}
	var response tokenResponse
	err := c.oauthJSON(ctx, "/oauth/device/token", map[string]string{
		"code": authorization.DeviceCode, "client_id": c.clientID, "client_secret": c.clientSecret,
	}, &response, "oauth_device_poll")
	if err != nil {
		var platformErr *socialhub.Error
		if errors.As(err, &platformErr) {
			switch platformErr.HTTPStatus {
			case http.StatusBadRequest:
				if platformErr.PlatformCode == "" {
					platformErr.Code, platformErr.Class = socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
					platformErr.PlatformCode, platformErr.RetryAfter = "authorization_pending", authorization.Interval
				}
			case http.StatusGone:
				platformErr.Code, platformErr.Class, platformErr.PlatformCode = socialhub.CodeUnauthenticated, socialhub.ClassUserAction, "expired_token"
			case http.StatusTeapot:
				platformErr.Code, platformErr.Class, platformErr.PlatformCode = socialhub.CodePermissionDenied, socialhub.ClassUserAction, "access_denied"
			}
		}
		return socialhub.Token{}, err
	}
	return c.mapToken(response, "oauth_device_poll")
}

// Revoke disconnects an access token from the Trakt application.
func (c *Client) Revoke(ctx context.Context, accessToken string) error {
	if err := c.requireSecret("oauth_revoke"); err != nil {
		return err
	}
	if !validCredential(accessToken) {
		return invalidArgument("oauth_revoke", "access token is required")
	}
	return c.oauthJSON(ctx, "/oauth/revoke", map[string]string{
		"token": accessToken, "client_id": c.clientID, "client_secret": c.clientSecret,
	}, nil, "oauth_revoke")
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	CreatedAt    int64  `json:"created_at"`
}

func (c *Client) exchangeToken(ctx context.Context, input any, operation string) (socialhub.Token, error) {
	var response tokenResponse
	if err := c.oauthJSON(ctx, "/oauth/token", input, &response, operation); err != nil {
		return socialhub.Token{}, err
	}
	return c.mapToken(response, operation)
}

func (c *Client) mapToken(response tokenResponse, operation string) (socialhub.Token, error) {
	if !validCredential(response.AccessToken) || !validCredential(response.RefreshToken) || response.ExpiresIn <= 0 || response.ExpiresIn > int64((365*24*time.Hour)/time.Second) {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	createdAt := c.clock.Now()
	if response.CreatedAt > 0 {
		createdAt = time.Unix(response.CreatedAt, 0)
	}
	tokenType := response.TokenType
	if tokenType == "" {
		tokenType = "Bearer"
	}
	return socialhub.Token{
		AccessToken: response.AccessToken, RefreshToken: response.RefreshToken, TokenType: tokenType,
		ExpiresAt: createdAt.Add(time.Duration(response.ExpiresIn) * time.Second), Scopes: strings.Fields(response.Scope),
	}, nil
}

func (c *Client) oauthJSON(ctx context.Context, path string, input, output any, operation string) error {
	encoded, err := json.Marshal(input)
	if err != nil {
		return platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.authURL, "/")+path, bytes.NewReader(encoded))
	if err != nil {
		return platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("trakt-api-key", c.clientID)
	request.Header.Set("trakt-api-version", apiVersion)
	request.Header.Set("User-Agent", c.userAgent)
	response, err := c.httpClient.Do(request)
	if err != nil {
		return platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, sanitizeTransportError(err))
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxOAuthResponseBytes+1))
	if err != nil {
		return platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(body)) > maxOAuthResponseBytes {
		return platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		decoded := decodeHTTPError(response.StatusCode, response.Header, body)
		if platformErr, ok := decoded.(*socialhub.Error); ok {
			platformErr.Op = operation
		}
		return decoded
	}
	if output == nil || len(body) == 0 {
		return nil
	}
	if err := json.Unmarshal(body, output); err != nil {
		return platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return nil
}

func sanitizeTransportError(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}

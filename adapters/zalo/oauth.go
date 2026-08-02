package zalo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

const maxOAuthResponseBytes int64 = 1 << 20

// OAuthClient implements Zalo OA OAuth v4 token exchange and rotating refresh.
type OAuthClient struct {
	AppID      string
	AppSecret  string
	BaseURL    string
	HTTPClient *http.Client
	Clock      socialhub.Clock
}

// Exchange obtains an OA token pair from a single-use authorization code and
// the PKCE verifier associated with the developer-console authorization URL.
func (c *OAuthClient) Exchange(ctx context.Context, code, codeVerifier string) (socialhub.Token, error) {
	if !validOpaqueID(code, 4096) || !validCodeVerifier(codeVerifier) {
		return socialhub.Token{}, invalidArgument("oauth_exchange", "authorization code and 43-character PKCE verifier are required")
	}
	return c.token(ctx, url.Values{
		"code": {code}, "app_id": {c.AppID}, "grant_type": {"authorization_code"}, "code_verifier": {codeVerifier},
	}, "oauth_exchange")
}

// Refresh consumes one refresh token and returns the replacement token pair.
// Callers must atomically persist both returned values because Zalo refresh
// tokens are single-use.
func (c *OAuthClient) Refresh(ctx context.Context, refreshToken string) (socialhub.Token, error) {
	if !validOpaqueID(refreshToken, 4096) {
		return socialhub.Token{}, invalidArgument("oauth_refresh", "refresh token is required")
	}
	return c.token(ctx, url.Values{
		"refresh_token": {refreshToken}, "app_id": {c.AppID}, "grant_type": {"refresh_token"},
	}, "oauth_refresh")
}

func (c *OAuthClient) token(ctx context.Context, values url.Values, operation string) (socialhub.Token, error) {
	if !validNumericID(c.AppID) || strings.TrimSpace(c.AppSecret) == "" || c.HTTPClient == nil || c.Clock == nil || !validEndpoint(c.BaseURL) {
		return socialhub.Token{}, invalidArgument(operation, "OAuth client is incomplete")
	}
	base, _ := url.Parse(c.BaseURL)
	requestURL := *base
	requestURL.Path = strings.TrimRight(base.Path, "/") + "/v4/oa/access_token"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL.String(), strings.NewReader(values.Encode()))
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("secret_key", c.AppSecret)
	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxOAuthResponseBytes+1))
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(body)) > maxOAuthResponseBytes {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return socialhub.Token{}, decodeOAuthError(response.StatusCode, body, operation)
	}
	var payload struct {
		AccessToken  string          `json:"access_token"`
		RefreshToken string          `json:"refresh_token"`
		ExpiresIn    json.RawMessage `json:"expires_in"`
		Error        int             `json:"error"`
		Message      string          `json:"message"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if payload.Error != 0 {
		return socialhub.Token{}, mapAPIError(operation, payload.Error, payload.Message)
	}
	expiresIn, err := parseFlexibleInt(payload.ExpiresIn)
	maxSeconds := int64((365 * 24 * time.Hour) / time.Second)
	if err != nil || expiresIn <= 0 || expiresIn > maxSeconds || !validOpaqueID(payload.AccessToken, 4096) || !validOpaqueID(payload.RefreshToken, 4096) {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return socialhub.Token{
		AccessToken: payload.AccessToken, RefreshToken: payload.RefreshToken,
		ExpiresAt: c.Clock.Now().Add(time.Duration(expiresIn) * time.Second),
	}, nil
}

func decodeOAuthError(status int, body []byte, operation string) error {
	var payload struct {
		Error            any    `json:"error"`
		ErrorDescription string `json:"error_description"`
		Message          string `json:"message"`
	}
	_ = json.Unmarshal(body, &payload)
	platformCode := ""
	if payload.Error != nil {
		platformCode = boundedMessage(fmt.Sprint(payload.Error), 128)
	}
	message := firstNonEmpty(payload.ErrorDescription, payload.Message)
	code, class := socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		code, class = socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	} else if status == http.StatusTooManyRequests {
		code, class = socialhub.CodeRateLimited, socialhub.ClassRetryable
	} else if status >= 500 {
		code, class = socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: "zalo", Product: productName, Op: operation, HTTPStatus: status,
		PlatformCode: platformCode, PlatformMessage: boundedMessage(message, 512),
	}
}

func parseFlexibleInt(raw json.RawMessage) (int64, error) {
	value := strings.TrimSpace(string(raw))
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		value = value[1 : len(value)-1]
	}
	return strconv.ParseInt(value, 10, 64)
}

func validCodeVerifier(value string) bool {
	if len(value) != 43 {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') {
			continue
		}
		return false
	}
	return true
}

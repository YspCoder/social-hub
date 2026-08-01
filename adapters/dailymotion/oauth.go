package dailymotion

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

const maxOAuthResponseSize int64 = 1 << 20

// OAuthClient implements Dailymotion's OAuth 2.0 Client Credentials grant.
type OAuthClient struct {
	ClientID     string
	ClientSecret string
	TokenURL     string
	HTTPClient   *http.Client
	Clock        socialhub.Clock
}

// ClientCredentials obtains a short-lived bearer token. Dailymotion does not
// return a refresh token for this grant.
func (c *OAuthClient) ClientCredentials(ctx context.Context, scopes []string) (socialhub.Token, error) {
	if strings.TrimSpace(c.ClientID) == "" || strings.TrimSpace(c.ClientSecret) == "" || c.HTTPClient == nil || c.Clock == nil || !validEndpoint(c.TokenURL) || !validScopes(scopes) {
		return socialhub.Token{}, invalidArgument("oauth_client_credentials", "client ID, client secret, token URL, HTTP client, clock, and valid scopes are required")
	}
	values := url.Values{
		"grant_type": {"client_credentials"}, "client_id": {c.ClientID},
		"client_secret": {c.ClientSecret}, "scope": {strings.Join(scopes, " ")},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.TokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return socialhub.Token{}, platformError("oauth_client_credentials", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return socialhub.Token{}, platformError("oauth_client_credentials", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxOAuthResponseSize+1))
	if err != nil {
		return socialhub.Token{}, platformError("oauth_client_credentials", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(body)) > maxOAuthResponseSize {
		return socialhub.Token{}, platformError("oauth_client_credentials", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("OAuth response exceeded size limit"))
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return socialhub.Token{}, decodeOAuthError(response.StatusCode, body)
	}
	var payload struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
		Scope       string `json:"scope"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return socialhub.Token{}, platformError("oauth_client_credentials", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if strings.TrimSpace(payload.AccessToken) == "" || len(payload.AccessToken) > 8192 || payload.ExpiresIn <= 0 || payload.ExpiresIn > int64((24*time.Hour)/time.Second) {
		return socialhub.Token{}, platformError("oauth_client_credentials", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("invalid token response fields"))
	}
	tokenType := payload.TokenType
	if tokenType == "" || strings.EqualFold(tokenType, "bearer") {
		tokenType = "Bearer"
	}
	granted := strings.Fields(payload.Scope)
	if len(granted) == 0 {
		granted = append([]string(nil), scopes...)
	}
	return socialhub.Token{AccessToken: payload.AccessToken, TokenType: tokenType, ExpiresAt: c.Clock.Now().Add(time.Duration(payload.ExpiresIn) * time.Second), Scopes: granted}, nil
}

func decodeOAuthError(status int, body []byte) error {
	var payload struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
		Detail           []struct {
			Message string `json:"msg"`
			Type    string `json:"type"`
		} `json:"detail"`
	}
	_ = json.Unmarshal(body, &payload)
	code, class := classifyError(status, payload.Error)
	if status == http.StatusUnprocessableEntity {
		code, class = socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	}
	message := payload.ErrorDescription
	if message == "" && len(payload.Detail) > 0 {
		message = payload.Detail[0].Message
	}
	return &socialhub.Error{Code: code, Class: class, Platform: "dailymotion", Product: productName, Op: "oauth_client_credentials", HTTPStatus: status, PlatformCode: boundedMessage(firstNonEmpty(payload.Error, func() string {
		if len(payload.Detail) > 0 {
			return payload.Detail[0].Type
		}
		return ""
	}()), 128), PlatformMessage: boundedMessage(message, 512)}
}

func validScopes(scopes []string) bool {
	if len(scopes) == 0 {
		return false
	}
	seen := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		switch scope {
		case "account.read", "account.manage", "profile.read", "profile.manage", "video.read", "video.manage", "playlist.read", "playlist.manage", "live.read", "live.manage", "player.read", "player.manage", "organization.read", "organization.manage", "analytics.manage", "bundle.public", "bundle.user", "bundle.publisher", "bundle.organization":
		default:
			return false
		}
		if _, exists := seen[scope]; exists {
			return false
		}
		seen[scope] = struct{}{}
	}
	return true
}

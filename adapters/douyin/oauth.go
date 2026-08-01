package douyin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

// UserToken preserves the app-scoped open ID and both token lifetimes.
type UserToken struct {
	Token            socialhub.Token
	OpenID           string
	RefreshExpiresAt time.Time
}

// RefreshToken is returned when a refresh token is explicitly renewed.
type RefreshToken struct {
	Value     string
	ExpiresAt time.Time
}

// OAuthClient distinguishes user tokens from application-level client tokens.
type OAuthClient struct {
	ClientKey    string
	ClientSecret string
	AuthURL      string
	TokenBaseURL string
	HTTPClient   *http.Client
	Clock        socialhub.Clock
}

// AuthorizationURL returns the Douyin authorization-code URL.
func (c *OAuthClient) AuthorizationURL(redirectURI, state string, scopes []string) (string, error) {
	if c.ClientKey == "" || redirectURI == "" || state == "" || len(scopes) == 0 {
		return "", fmt.Errorf("douyin: client key, redirect URI, state, and scopes are required")
	}
	parsed, err := url.Parse(c.AuthURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("douyin: invalid authorization URL")
	}
	query := parsed.Query()
	query.Set("client_key", c.ClientKey)
	query.Set("response_type", "code")
	query.Set("scope", strings.Join(scopes, ","))
	query.Set("redirect_uri", redirectURI)
	query.Set("state", state)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

// Exchange exchanges a one-time authorization code for a user token.
func (c *OAuthClient) Exchange(ctx context.Context, code string) (UserToken, error) {
	if code == "" {
		return UserToken{}, fmt.Errorf("douyin: authorization code is required")
	}
	return c.userTokenRequest(ctx, "/oauth/access_token/", url.Values{
		"client_key":    {c.ClientKey},
		"client_secret": {c.ClientSecret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
	}, false)
}

// Refresh refreshes or extends a user access token.
func (c *OAuthClient) Refresh(ctx context.Context, refreshToken string) (UserToken, error) {
	if refreshToken == "" {
		return UserToken{}, fmt.Errorf("douyin: refresh token is required")
	}
	return c.userTokenRequest(ctx, "/oauth/refresh_token/", url.Values{
		"client_key":    {c.ClientKey},
		"refresh_token": {refreshToken},
		"grant_type":    {"refresh_token"},
	}, true)
}

// RenewRefreshToken rotates a refresh token. The scope renew_refresh_token is required.
func (c *OAuthClient) RenewRefreshToken(ctx context.Context, refreshToken string) (RefreshToken, error) {
	if refreshToken == "" {
		return RefreshToken{}, fmt.Errorf("douyin: refresh token is required")
	}
	payload, err := c.tokenRequest(ctx, "/oauth/renew_refresh_token/", url.Values{
		"client_key":    {c.ClientKey},
		"refresh_token": {refreshToken},
	}, true)
	if err != nil {
		return RefreshToken{}, err
	}
	if payload.RefreshToken == "" {
		return RefreshToken{}, wrapError("renew_refresh_token", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("missing refresh token"))
	}
	return RefreshToken{Value: payload.RefreshToken, ExpiresAt: c.Clock.Now().Add(time.Duration(payload.ExpiresIn) * time.Second)}, nil
}

// ClientToken obtains an application-level token for APIs that do not act as a user.
func (c *OAuthClient) ClientToken(ctx context.Context) (socialhub.Token, error) {
	payload, err := c.tokenRequest(ctx, "/oauth/client_token/", url.Values{
		"client_key":    {c.ClientKey},
		"client_secret": {c.ClientSecret},
		"grant_type":    {"client_credential"},
	}, true)
	if err != nil {
		return socialhub.Token{}, err
	}
	if payload.AccessToken == "" {
		return socialhub.Token{}, wrapError("client_token", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("missing access token"))
	}
	return socialhub.Token{AccessToken: payload.AccessToken, TokenType: "DouyinClient", ExpiresAt: c.Clock.Now().Add(time.Duration(payload.ExpiresIn) * time.Second)}, nil
}

func (c *OAuthClient) userTokenRequest(ctx context.Context, path string, form url.Values, multipartForm bool) (UserToken, error) {
	payload, err := c.tokenRequest(ctx, path, form, multipartForm)
	if err != nil {
		return UserToken{}, err
	}
	if payload.AccessToken == "" || payload.OpenID == "" {
		return UserToken{}, wrapError("user_token", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("missing user token fields"))
	}
	token := socialhub.Token{
		AccessToken:  payload.AccessToken,
		RefreshToken: payload.RefreshToken,
		TokenType:    "DouyinUser",
		ExpiresAt:    c.Clock.Now().Add(time.Duration(payload.ExpiresIn) * time.Second),
		Scopes:       splitScopes(payload.Scope),
	}
	return UserToken{Token: token, OpenID: payload.OpenID, RefreshExpiresAt: c.Clock.Now().Add(time.Duration(payload.RefreshExpiresIn) * time.Second)}, nil
}

type tokenPayload struct {
	APIResponse
	AccessToken      string        `json:"access_token"`
	RefreshToken     string        `json:"refresh_token"`
	OpenID           string        `json:"open_id"`
	ExpiresIn        flexibleInt64 `json:"expires_in"`
	RefreshExpiresIn flexibleInt64 `json:"refresh_expires_in"`
	Scope            string        `json:"scope"`
}

func (c *OAuthClient) tokenRequest(ctx context.Context, path string, form url.Values, multipartForm bool) (tokenPayload, error) {
	if c.ClientKey == "" || c.ClientSecret == "" || c.TokenBaseURL == "" || c.HTTPClient == nil || c.Clock == nil {
		return tokenPayload{}, fmt.Errorf("douyin: incomplete OAuth client")
	}
	body, contentType, err := encodeTokenForm(form, multipartForm)
	if err != nil {
		return tokenPayload{}, err
	}
	endpoint := strings.TrimRight(c.TokenBaseURL, "/") + path
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return tokenPayload{}, wrapError("token", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Content-Type", contentType)
	request.Header.Set("Accept", "application/json")
	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return tokenPayload{}, wrapError("token", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil {
		return tokenPayload{}, wrapError("token", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if len(responseBody) > 1<<20 {
		return tokenPayload{}, wrapError("token", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("response exceeded size limit"))
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return tokenPayload{}, decodeHTTPError(response.StatusCode, response.Header, responseBody)
	}
	var envelope struct {
		Data  tokenPayload  `json:"data"`
		Extra responseExtra `json:"extra"`
	}
	if err := json.Unmarshal(responseBody, &envelope); err != nil {
		return tokenPayload{}, wrapError("token", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if err := responseError(envelope.Data.APIResponse, envelope.Extra, "token", response.StatusCode, response.Header); err != nil {
		return tokenPayload{}, err
	}
	return envelope.Data, nil
}

func encodeTokenForm(form url.Values, useMultipart bool) (io.Reader, string, error) {
	if !useMultipart {
		return strings.NewReader(form.Encode()), "application/x-www-form-urlencoded", nil
	}
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	for key, values := range form {
		for _, value := range values {
			if err := writer.WriteField(key, value); err != nil {
				return nil, "", err
			}
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return &buffer, writer.FormDataContentType(), nil
}

func splitScopes(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ' ' })
}

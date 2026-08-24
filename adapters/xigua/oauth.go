package xigua

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

const maxOAuthResponseBytes int64 = 1 << 20

// UserToken preserves the app-scoped open ID and both token lifetimes.
type UserToken struct {
	Token            socialhub.Token
	OpenID           string
	RefreshExpiresAt time.Time
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

// AuthorizationURL returns the Xigua authorization-code URL.
func (c *OAuthClient) AuthorizationURL(redirectURI, state string, scopes []string) (string, error) {
	if !validOpaque(c.ClientKey, 512) || !validRedirectURI(redirectURI) || !validOpaque(state, 512) || !validScopes(scopes) {
		return "", invalidArgument("authorization_url", "client key, redirect URI, state, and valid scopes are required")
	}
	if !validEndpoint(c.AuthURL) {
		return "", invalidArgument("authorization_url", "authorization URL is invalid")
	}
	parsed, _ := url.Parse(c.AuthURL)
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
	if !validOpaque(code, maxOpaqueLength) {
		return UserToken{}, invalidArgument("exchange", "authorization code is required")
	}
	return c.userTokenRequest(ctx, "/oauth/access_token/", url.Values{
		"client_key": {c.ClientKey}, "client_secret": {c.ClientSecret},
		"code": {code}, "grant_type": {"authorization_code"},
	}, false)
}

// Refresh refreshes or extends a user access token. Xigua does not expose a
// separate refresh-token renewal operation, so reauthorization is eventually required.
func (c *OAuthClient) Refresh(ctx context.Context, refreshToken string) (UserToken, error) {
	if !validOpaque(refreshToken, maxOpaqueLength) {
		return UserToken{}, invalidArgument("refresh", "refresh token is required")
	}
	return c.userTokenRequest(ctx, "/oauth/refresh_token/", url.Values{
		"client_key": {c.ClientKey}, "refresh_token": {refreshToken}, "grant_type": {"refresh_token"},
	}, true)
}

// ClientToken obtains an application-level token for APIs that do not act as a user.
func (c *OAuthClient) ClientToken(ctx context.Context) (socialhub.Token, error) {
	payload, err := c.tokenRequest(ctx, "/oauth/client_token/", url.Values{
		"client_key": {c.ClientKey}, "client_secret": {c.ClientSecret}, "grant_type": {"client_credential"},
	}, true)
	if err != nil {
		return socialhub.Token{}, err
	}
	if !validOpaque(payload.AccessToken, maxOpaqueLength) || payload.ExpiresIn <= 0 {
		return socialhub.Token{}, invalidPlatformResponse("client_token", "response omitted a valid access token or lifetime")
	}
	return socialhub.Token{
		AccessToken: payload.AccessToken, TokenType: "XiguaClient",
		ExpiresAt: c.Clock.Now().Add(time.Duration(payload.ExpiresIn) * time.Second),
	}, nil
}

type tokenPayload struct {
	apiResponse
	AccessToken      string        `json:"access_token"`
	RefreshToken     string        `json:"refresh_token"`
	OpenID           string        `json:"open_id"`
	ExpiresIn        flexibleInt64 `json:"expires_in"`
	RefreshExpiresIn flexibleInt64 `json:"refresh_expires_in"`
	Scope            string        `json:"scope"`
}

func (c *OAuthClient) userTokenRequest(ctx context.Context, path string, form url.Values, multipartForm bool) (UserToken, error) {
	payload, err := c.tokenRequest(ctx, path, form, multipartForm)
	if err != nil {
		return UserToken{}, err
	}
	if !validOpaque(payload.AccessToken, maxOpaqueLength) || !validOpaque(payload.OpenID, 512) || payload.ExpiresIn <= 0 {
		return UserToken{}, invalidPlatformResponse("user_token", "response omitted valid user token fields")
	}
	if payload.RefreshToken != "" && !validOpaque(payload.RefreshToken, maxOpaqueLength) {
		return UserToken{}, invalidPlatformResponse("user_token", "response contained an invalid refresh token")
	}
	scopes := splitScopes(payload.Scope)
	if payload.Scope != "" && !validScopes(scopes) {
		return UserToken{}, invalidPlatformResponse("user_token", "response contained invalid scopes")
	}
	var refreshExpiresAt time.Time
	if payload.RefreshToken != "" {
		if payload.RefreshExpiresIn <= 0 {
			return UserToken{}, invalidPlatformResponse("user_token", "response omitted a valid refresh-token lifetime")
		}
		refreshExpiresAt = c.Clock.Now().Add(time.Duration(payload.RefreshExpiresIn) * time.Second)
	}
	return UserToken{
		Token: socialhub.Token{
			AccessToken: payload.AccessToken, RefreshToken: payload.RefreshToken, TokenType: "XiguaUser",
			ExpiresAt: c.Clock.Now().Add(time.Duration(payload.ExpiresIn) * time.Second), Scopes: scopes,
		},
		OpenID:           payload.OpenID,
		RefreshExpiresAt: refreshExpiresAt,
	}, nil
}

func (c *OAuthClient) tokenRequest(ctx context.Context, path string, form url.Values, multipartForm bool) (tokenPayload, error) {
	if !validOpaque(c.ClientKey, 512) || !validOpaque(c.ClientSecret, maxOpaqueLength) ||
		!validEndpoint(c.TokenBaseURL) || c.HTTPClient == nil || c.Clock == nil {
		return tokenPayload{}, invalidArgument("token", "OAuth client is incomplete")
	}
	body, contentType, err := encodeTokenForm(form, multipartForm)
	if err != nil {
		return tokenPayload{}, platformError("token", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	endpoint := strings.TrimRight(c.TokenBaseURL, "/") + path
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return tokenPayload{}, platformError("token", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Content-Type", contentType)
	request.Header.Set("Accept", "application/json")
	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return tokenPayload{}, platformError("token", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, sanitizeOAuthError(err))
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxOAuthResponseBytes+1))
	if err != nil {
		return tokenPayload{}, platformError("token", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(responseBody)) > maxOAuthResponseBytes {
		return tokenPayload{}, invalidPlatformResponse("token", "response exceeded size limit")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return tokenPayload{}, decodeHTTPError(response.StatusCode, response.Header, responseBody)
	}
	var envelope struct {
		Data  tokenPayload  `json:"data"`
		Extra responseExtra `json:"extra"`
	}
	if err := json.Unmarshal(responseBody, &envelope); err != nil {
		return tokenPayload{}, platformError("token", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if err := responseError(envelope.Data.apiResponse, envelope.Extra, "token", response.StatusCode, response.Header); err != nil {
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
	return strings.FieldsFunc(value, func(character rune) bool { return character == ',' || unicodeSpace(character) })
}

func unicodeSpace(character rune) bool {
	return character == ' ' || character == '\t' || character == '\r' || character == '\n'
}

func sanitizeOAuthError(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}

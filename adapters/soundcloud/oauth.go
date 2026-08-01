package soundcloud

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

const maxOAuthResponseSize int64 = 1 << 20

// PKCE contains an OAuth 2.1 S256 verifier and challenge.
type PKCE struct {
	Verifier  string
	Challenge string
}

// NewPKCE creates a cryptographically random PKCE pair.
func NewPKCE() (PKCE, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return PKCE{}, fmt.Errorf("soundcloud: generate PKCE: %w", err)
	}
	verifier := base64.RawURLEncoding.EncodeToString(random)
	digest := sha256.Sum256([]byte(verifier))
	return PKCE{Verifier: verifier, Challenge: base64.RawURLEncoding.EncodeToString(digest[:])}, nil
}

// OAuthClient implements SoundCloud OAuth 2.1 authorization-code, refresh, and client-credentials flows.
type OAuthClient struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	HTTPClient   *http.Client
	Clock        socialhub.Clock
}

func (c *OAuthClient) AuthorizationURL(redirectURI, state string, pkce PKCE) (string, error) {
	if strings.TrimSpace(c.ClientID) == "" || !validEndpoint(c.AuthURL) || !validRedirectURI(redirectURI) || strings.TrimSpace(state) == "" || !validPKCEValue(pkce.Challenge) {
		return "", invalidArgument("oauth_authorize", "client ID, redirect URI, state, and PKCE S256 challenge are required")
	}
	parsed, err := url.Parse(c.AuthURL)
	if err != nil {
		return "", platformError("oauth_authorize", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	query := parsed.Query()
	query.Set("client_id", c.ClientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("response_type", "code")
	query.Set("code_challenge", pkce.Challenge)
	query.Set("code_challenge_method", "S256")
	query.Set("state", state)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (c *OAuthClient) Exchange(ctx context.Context, code, redirectURI, verifier string) (socialhub.Token, error) {
	if strings.TrimSpace(code) == "" || !validRedirectURI(redirectURI) || !validPKCEValue(verifier) {
		return socialhub.Token{}, invalidArgument("oauth_exchange", "code, redirect URI, and PKCE verifier are required")
	}
	values := url.Values{
		"grant_type": {"authorization_code"}, "client_id": {c.ClientID}, "client_secret": {c.ClientSecret},
		"redirect_uri": {redirectURI}, "code_verifier": {verifier}, "code": {code},
	}
	return c.token(ctx, values, "oauth_exchange", false, false)
}

func (c *OAuthClient) ClientCredentials(ctx context.Context) (socialhub.Token, error) {
	return c.token(ctx, url.Values{"grant_type": {"client_credentials"}}, "oauth_client_credentials", true, false)
}

// Refresh rotates a single-use SoundCloud refresh token.
func (c *OAuthClient) Refresh(ctx context.Context, refreshToken string) (socialhub.Token, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return socialhub.Token{}, invalidArgument("oauth_refresh", "refresh token is required")
	}
	values := url.Values{
		"grant_type": {"refresh_token"}, "client_id": {c.ClientID},
		"client_secret": {c.ClientSecret}, "refresh_token": {refreshToken},
	}
	return c.token(ctx, values, "oauth_refresh", false, true)
}

func (c *OAuthClient) token(ctx context.Context, values url.Values, operation string, basicAuth, requireRefresh bool) (socialhub.Token, error) {
	if strings.TrimSpace(c.ClientID) == "" || strings.TrimSpace(c.ClientSecret) == "" || c.HTTPClient == nil || c.Clock == nil || !validEndpoint(c.TokenURL) {
		return socialhub.Token{}, invalidArgument(operation, "OAuth client credentials, HTTP client, clock, and token URL are required")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.TokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json; charset=utf-8")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if basicAuth {
		request.SetBasicAuth(c.ClientID, c.ClientSecret)
	}
	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxOAuthResponseSize+1))
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(body)) > maxOAuthResponseSize {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		decoded := decodeHTTPError(response.StatusCode, response.Header, body)
		if hubError, ok := decoded.(*socialhub.Error); ok {
			hubError.Op = operation
		}
		return socialhub.Token{}, decoded
	}
	var payload struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
		Scope        string `json:"scope"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || strings.TrimSpace(payload.AccessToken) == "" {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if requireRefresh && strings.TrimSpace(payload.RefreshToken) == "" {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, errors.New("SoundCloud did not rotate the single-use refresh token"))
	}
	if payload.ExpiresIn < 0 || payload.ExpiresIn > int64((365*24*time.Hour)/time.Second) {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	var expiresAt time.Time
	if payload.ExpiresIn > 0 {
		expiresAt = c.Clock.Now().Add(time.Duration(payload.ExpiresIn) * time.Second)
	}
	return socialhub.Token{
		AccessToken: payload.AccessToken, RefreshToken: payload.RefreshToken, TokenType: "OAuth",
		ExpiresAt: expiresAt, Scopes: strings.Fields(payload.Scope),
	}, nil
}

func validRedirectURI(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme != "" && parsed.Host != "" && parsed.User == nil && parsed.Fragment == ""
}

func validPKCEValue(value string) bool {
	if len(value) < 43 || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || strings.ContainsRune("-._~", character) {
			continue
		}
		return false
	}
	return true
}

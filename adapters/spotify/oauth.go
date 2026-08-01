package spotify

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
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

// PKCE contains a Spotify OAuth S256 verifier and challenge.
type PKCE struct {
	Verifier  string
	Challenge string
}

// NewPKCE creates a cryptographically random PKCE pair.
func NewPKCE() (PKCE, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return PKCE{}, fmt.Errorf("spotify: generate PKCE: %w", err)
	}
	verifier := base64.RawURLEncoding.EncodeToString(random)
	digest := sha256.Sum256([]byte(verifier))
	return PKCE{Verifier: verifier, Challenge: base64.RawURLEncoding.EncodeToString(digest[:])}, nil
}

// OAuthClient implements Spotify OAuth 2.0 PKCE, refresh, and client-credentials flows.
type OAuthClient struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	HTTPClient   *http.Client
	Clock        socialhub.Clock
}

// AuthorizationURL builds a state-protected Authorization Code with PKCE URL.
func (c *OAuthClient) AuthorizationURL(redirectURI, state string, scopes []string, pkce PKCE) (string, error) {
	if strings.TrimSpace(c.ClientID) == "" || !validEndpoint(c.AuthURL) || !validSpotifyRedirectURI(redirectURI) ||
		strings.TrimSpace(state) == "" || !validPKCEValue(pkce.Challenge) || !validScopes(scopes) {
		return "", invalidArgument("oauth_authorize", "client ID, Spotify redirect URI, state, valid scopes, and an S256 PKCE challenge are required")
	}
	parsed, err := url.Parse(c.AuthURL)
	if err != nil {
		return "", platformErrorWithCause("oauth_authorize", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	query := parsed.Query()
	query.Set("client_id", c.ClientID)
	query.Set("response_type", "code")
	query.Set("redirect_uri", redirectURI)
	query.Set("state", state)
	query.Set("code_challenge", pkce.Challenge)
	query.Set("code_challenge_method", "S256")
	if len(scopes) > 0 {
		query.Set("scope", strings.Join(scopes, " "))
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

// Exchange exchanges a Spotify authorization code and PKCE verifier.
func (c *OAuthClient) Exchange(ctx context.Context, code, redirectURI, verifier string) (socialhub.Token, error) {
	if strings.TrimSpace(code) == "" || !validSpotifyRedirectURI(redirectURI) || !validPKCEValue(verifier) {
		return socialhub.Token{}, invalidArgument("oauth_exchange", "code, Spotify redirect URI, and PKCE verifier are required")
	}
	values := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {c.ClientID},
		"code_verifier": {verifier},
	}
	return c.token(ctx, values, "oauth_exchange", "", true, false)
}

// Refresh obtains a new access token and retains the old refresh token if Spotify omits rotation.
func (c *OAuthClient) Refresh(ctx context.Context, refreshToken string) (socialhub.Token, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return socialhub.Token{}, invalidArgument("oauth_refresh", "refresh token is required")
	}
	values := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refreshToken}}
	values.Set("client_id", c.ClientID)
	return c.token(ctx, values, "oauth_refresh", refreshToken, false, false)
}

// ClientCredentials obtains an app token for endpoints that do not need user authorization.
func (c *OAuthClient) ClientCredentials(ctx context.Context) (socialhub.Token, error) {
	if strings.TrimSpace(c.ClientSecret) == "" {
		return socialhub.Token{}, invalidArgument("oauth_client_credentials", "client secret is required for client credentials")
	}
	return c.token(ctx, url.Values{"grant_type": {"client_credentials"}}, "oauth_client_credentials", "", false, true)
}

func (c *OAuthClient) token(ctx context.Context, values url.Values, operation, fallbackRefresh string, requireRefresh, basicAuth bool) (socialhub.Token, error) {
	if strings.TrimSpace(c.ClientID) == "" || c.HTTPClient == nil || c.Clock == nil || !validEndpoint(c.TokenURL) {
		return socialhub.Token{}, invalidArgument(operation, "OAuth client ID, HTTP client, clock, and token URL are required")
	}
	if basicAuth && strings.TrimSpace(c.ClientSecret) == "" {
		return socialhub.Token{}, invalidArgument(operation, "client secret is required")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.TokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return socialhub.Token{}, platformErrorWithCause(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if basicAuth {
		request.SetBasicAuth(c.ClientID, c.ClientSecret)
	}
	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return socialhub.Token{}, platformErrorWithCause(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxOAuthResponseSize+1))
	if err != nil {
		return socialhub.Token{}, platformErrorWithCause(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(body)) > maxOAuthResponseSize {
		return socialhub.Token{}, platformErrorWithCause(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("Spotify OAuth response exceeded size limit"))
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return socialhub.Token{}, decodeOAuthError(operation, response.StatusCode, response.Header, body)
	}
	var payload struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
		Scope        string `json:"scope"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return socialhub.Token{}, platformErrorWithCause(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if strings.TrimSpace(payload.AccessToken) == "" || payload.ExpiresIn <= 0 || payload.ExpiresIn > int64((24*time.Hour)/time.Second) {
		return socialhub.Token{}, platformErrorWithCause(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("Spotify returned an invalid access token lifetime"))
	}
	if payload.RefreshToken == "" {
		payload.RefreshToken = fallbackRefresh
	}
	if requireRefresh && payload.RefreshToken == "" {
		return socialhub.Token{}, platformErrorWithCause(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("Spotify did not return a refresh token"))
	}
	tokenType := payload.TokenType
	if tokenType == "" {
		tokenType = "Bearer"
	}
	return socialhub.Token{
		AccessToken: payload.AccessToken, RefreshToken: payload.RefreshToken, TokenType: tokenType,
		ExpiresAt: c.Clock.Now().Add(time.Duration(payload.ExpiresIn) * time.Second), Scopes: strings.Fields(payload.Scope),
	}, nil
}

func decodeOAuthError(operation string, status int, header http.Header, body []byte) error {
	var payload struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	_ = json.Unmarshal(body, &payload)
	code, class := classifyError(status)
	if payload.Error == "invalid_grant" || payload.Error == "invalid_client" {
		code, class = socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: "spotify", Product: productName, Op: operation,
		HTTPStatus: status, PlatformCode: boundedMessage(payload.Error, 128),
		PlatformMessage: boundedMessage(payload.ErrorDescription, 512),
		RequestID:       boundedMessage(firstNonEmpty(header.Get("X-Request-Id"), header.Get("Spotify-Request-Id")), 512),
		RetryAfter:      parseRetryAfter(header.Get("Retry-After")),
	}
}

func validSpotifyRedirectURI(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil || parsed.User != nil || parsed.Fragment != "" || parsed.Host == "" || strings.EqualFold(parsed.Hostname(), "localhost") {
		return false
	}
	if parsed.Scheme == "https" {
		return true
	}
	return parsed.Scheme == "http" && parsed.Hostname() == "127.0.0.1"
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

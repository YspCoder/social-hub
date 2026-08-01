package mastodon

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

	"social-hub/pkg/socialhub"
)

const maxOAuthResponseBytes int64 = 1 << 20

// PKCE contains an OAuth S256 verifier and challenge.
type PKCE struct {
	Verifier  string
	Challenge string
}

// NewPKCE creates a cryptographically random PKCE pair.
func NewPKCE() (PKCE, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return PKCE{}, fmt.Errorf("mastodon: generate PKCE: %w", err)
	}
	verifier := base64.RawURLEncoding.EncodeToString(random)
	digest := sha256.Sum256([]byte(verifier))
	return PKCE{Verifier: verifier, Challenge: base64.RawURLEncoding.EncodeToString(digest[:])}, nil
}

// OAuthClient implements Mastodon OAuth 2.0 authorization-code and
// client-credentials grants. Mastodon access tokens do not expire automatically.
type OAuthClient struct {
	InstanceURL  string
	ClientID     string
	ClientSecret string
	HTTPClient   *http.Client
}

func (c *OAuthClient) AuthorizationURL(redirectURI, state string, scopes []string) (string, error) {
	return c.authorizationURL(redirectURI, state, scopes, "")
}

func (c *OAuthClient) AuthorizationURLPKCE(redirectURI, state string, scopes []string, challenge string) (string, error) {
	if challenge == "" {
		return "", fmt.Errorf("mastodon: PKCE challenge is required")
	}
	return c.authorizationURL(redirectURI, state, scopes, challenge)
}

func (c *OAuthClient) authorizationURL(redirectURI, state string, scopes []string, challenge string) (string, error) {
	if c.ClientID == "" || redirectURI == "" || state == "" || len(scopes) == 0 || !validInstanceURL(c.InstanceURL) {
		return "", fmt.Errorf("mastodon: instance, client ID, redirect URI, state, and scopes are required")
	}
	parsed, _ := url.Parse(normalizeInstanceURL(c.InstanceURL) + "/oauth/authorize")
	query := parsed.Query()
	query.Set("response_type", "code")
	query.Set("client_id", c.ClientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("scope", strings.Join(scopes, " "))
	query.Set("state", state)
	if challenge != "" {
		query.Set("code_challenge", challenge)
		query.Set("code_challenge_method", "S256")
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (c *OAuthClient) Exchange(ctx context.Context, code, redirectURI string) (socialhub.Token, error) {
	return c.ExchangeWithVerifier(ctx, code, redirectURI, "")
}

func (c *OAuthClient) ExchangeWithVerifier(ctx context.Context, code, redirectURI, verifier string) (socialhub.Token, error) {
	if code == "" || redirectURI == "" {
		return socialhub.Token{}, fmt.Errorf("mastodon: code and redirect URI are required")
	}
	values := url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {redirectURI},
	}
	if verifier != "" {
		values.Set("code_verifier", verifier)
	}
	return c.token(ctx, values, "oauth_exchange")
}

func (c *OAuthClient) ClientCredentials(ctx context.Context, redirectURI string, scopes []string) (socialhub.Token, error) {
	if redirectURI == "" || len(scopes) == 0 {
		return socialhub.Token{}, fmt.Errorf("mastodon: redirect URI and scopes are required")
	}
	return c.token(ctx, url.Values{
		"grant_type": {"client_credentials"}, "redirect_uri": {redirectURI}, "scope": {strings.Join(scopes, " ")},
	}, "oauth_client_credentials")
}

func (c *OAuthClient) token(ctx context.Context, values url.Values, operation string) (socialhub.Token, error) {
	if c.ClientID == "" || c.ClientSecret == "" || c.HTTPClient == nil || !validInstanceURL(c.InstanceURL) {
		return socialhub.Token{}, fmt.Errorf("mastodon: incomplete OAuth client")
	}
	values.Set("client_id", c.ClientID)
	values.Set("client_secret", c.ClientSecret)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, normalizeInstanceURL(c.InstanceURL)+"/oauth/token", strings.NewReader(values.Encode()))
	if err != nil {
		return socialhub.Token{}, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
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
		return socialhub.Token{}, decodeHTTPError(response.StatusCode, response.Header, body)
	}
	var payload struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Scope       string `json:"scope"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || payload.AccessToken == "" {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	tokenType := payload.TokenType
	if strings.EqualFold(tokenType, "bearer") || tokenType == "" {
		tokenType = "Bearer"
	}
	return socialhub.Token{AccessToken: payload.AccessToken, TokenType: tokenType, Scopes: strings.Fields(payload.Scope)}, nil
}

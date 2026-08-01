package x

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

// PKCE contains a verifier and its S256 challenge.
type PKCE struct {
	Verifier  string
	Challenge string
}

// NewPKCE creates a cryptographically random OAuth2 PKCE pair.
func NewPKCE() (PKCE, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return PKCE{}, fmt.Errorf("x: generate PKCE: %w", err)
	}
	verifier := base64.RawURLEncoding.EncodeToString(random)
	digest := sha256.Sum256([]byte(verifier))
	return PKCE{Verifier: verifier, Challenge: base64.RawURLEncoding.EncodeToString(digest[:])}, nil
}

// OAuthClient implements X OAuth2 Authorization Code with PKCE.
type OAuthClient struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	HTTPClient   *http.Client
}

// AuthorizationURL returns an OAuth2 authorization URL.
func (c *OAuthClient) AuthorizationURL(redirectURI, state string, scopes []string, pkce PKCE) (string, error) {
	if c.ClientID == "" || redirectURI == "" || state == "" || pkce.Challenge == "" {
		return "", fmt.Errorf("x: client ID, redirect URI, state, and PKCE challenge are required")
	}
	parsed, err := url.Parse(c.AuthURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("x: invalid authorization URL")
	}
	query := parsed.Query()
	query.Set("response_type", "code")
	query.Set("client_id", c.ClientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("scope", strings.Join(scopes, " "))
	query.Set("state", state)
	query.Set("code_challenge", pkce.Challenge)
	query.Set("code_challenge_method", "S256")
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

// Exchange exchanges an authorization code for a token bundle.
func (c *OAuthClient) Exchange(ctx context.Context, code, redirectURI, verifier string) (socialhub.Token, error) {
	if code == "" || redirectURI == "" || verifier == "" {
		return socialhub.Token{}, fmt.Errorf("x: code, redirect URI, and verifier are required")
	}
	return c.requestToken(ctx, url.Values{
		"code":          {code},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {redirectURI},
		"code_verifier": {verifier},
	})
}

// Refresh exchanges a refresh token for a new token bundle.
func (c *OAuthClient) Refresh(ctx context.Context, refreshToken string) (socialhub.Token, error) {
	if refreshToken == "" {
		return socialhub.Token{}, fmt.Errorf("x: refresh token is required")
	}
	return c.requestToken(ctx, url.Values{
		"refresh_token": {refreshToken},
		"grant_type":    {"refresh_token"},
	})
}

func (c *OAuthClient) requestToken(ctx context.Context, form url.Values) (socialhub.Token, error) {
	if c.ClientID == "" || c.TokenURL == "" || c.HTTPClient == nil {
		return socialhub.Token{}, fmt.Errorf("x: incomplete OAuth client")
	}
	form.Set("client_id", c.ClientID)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return socialhub.Token{}, fmt.Errorf("x: create token request: %w", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	if c.ClientSecret != "" {
		request.SetBasicAuth(c.ClientID, c.ClientSecret)
	}
	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return socialhub.Token{}, &socialhub.Error{Code: socialhub.CodeTemporarilyUnavailable, Class: socialhub.ClassRetryable, Platform: "x", Product: "oauth2", Op: "token", Cause: err}
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil {
		return socialhub.Token{}, &socialhub.Error{Code: socialhub.CodeTemporarilyUnavailable, Class: socialhub.ClassRetryable, Platform: "x", Product: "oauth2", Op: "token", Cause: err}
	}
	if len(body) > 1<<20 {
		return socialhub.Token{}, &socialhub.Error{Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent, Platform: "x", Product: "oauth2", Op: "token", HTTPStatus: response.StatusCode, PlatformMessage: "response exceeded size limit"}
	}
	var payload struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
		Scope        string `json:"scope"`
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var problem struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		_ = json.Unmarshal(body, &problem)
		return socialhub.Token{}, &socialhub.Error{Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction, Platform: "x", Product: "oauth2", Op: "token", HTTPStatus: response.StatusCode, PlatformCode: problem.Error, PlatformMessage: problem.ErrorDescription}
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return socialhub.Token{}, &socialhub.Error{Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent, Platform: "x", Product: "oauth2", Op: "token", Cause: err}
	}
	if payload.AccessToken == "" {
		return socialhub.Token{}, &socialhub.Error{Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent, Platform: "x", Product: "oauth2", Op: "token", PlatformMessage: "missing access token"}
	}
	return socialhub.Token{
		AccessToken:  payload.AccessToken,
		RefreshToken: payload.RefreshToken,
		TokenType:    payload.TokenType,
		ExpiresAt:    time.Now().Add(time.Duration(payload.ExpiresIn) * time.Second),
		Scopes:       strings.Fields(payload.Scope),
	}, nil
}

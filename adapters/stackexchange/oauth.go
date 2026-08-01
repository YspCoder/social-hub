package stackexchange

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

const maxOAuthResponseBytes int64 = 1 << 20

// OAuthClient implements Stack Exchange's OAuth 2.0 authorization-code flow.
// The platform does not issue refresh tokens.
type OAuthClient struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	UserAgent    string
	HTTPClient   *http.Client
	Clock        socialhub.Clock
}

func (client *OAuthClient) AuthorizationURL(redirectURI, state string, scopes []string) (string, error) {
	return client.authorizationURL(redirectURI, state, scopes, "")
}

// AuthorizationURLPKCE builds the recommended S256 PKCE authorization URL.
func (client *OAuthClient) AuthorizationURLPKCE(redirectURI, state string, scopes []string, codeChallenge string) (string, error) {
	if !validCodeChallenge(codeChallenge) {
		return "", invalidArgument("oauth_authorize", "code challenge must be a 43-character base64url SHA-256 value")
	}
	return client.authorizationURL(redirectURI, state, scopes, codeChallenge)
}

func (client *OAuthClient) authorizationURL(redirectURI, state string, scopes []string, codeChallenge string) (string, error) {
	if !validOpaque(client.ClientID, 512) || !validHTTPURL(redirectURI) || strings.TrimSpace(state) == "" {
		return "", invalidArgument("oauth_authorize", "client ID, redirect URI, and state are required")
	}
	parsed, err := url.Parse(client.AuthURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", invalidArgument("oauth_authorize", "authorization URL is invalid")
	}
	query := parsed.Query()
	query.Set("client_id", client.ClientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("state", state)
	if len(scopes) > 0 {
		query.Set("scope", strings.Join(scopes, " "))
	}
	if codeChallenge != "" {
		query.Set("code_challenge", codeChallenge)
		query.Set("code_challenge_method", "S256")
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (client *OAuthClient) Exchange(ctx context.Context, code, redirectURI string) (socialhub.Token, error) {
	return client.exchange(ctx, code, redirectURI, "")
}

// ExchangeWithVerifier exchanges an authorization code using the recommended
// PKCE verifier. A client secret is optional for this flow.
func (client *OAuthClient) ExchangeWithVerifier(ctx context.Context, code, redirectURI, codeVerifier string) (socialhub.Token, error) {
	if !validCodeVerifier(codeVerifier) {
		return socialhub.Token{}, invalidArgument("oauth_exchange", "code verifier must contain 43-128 unreserved characters")
	}
	return client.exchange(ctx, code, redirectURI, codeVerifier)
}

func (client *OAuthClient) exchange(ctx context.Context, code, redirectURI, codeVerifier string) (socialhub.Token, error) {
	if !validOpaque(code, 4096) || !validHTTPURL(redirectURI) {
		return socialhub.Token{}, invalidArgument("oauth_exchange", "authorization code and redirect URI are required")
	}
	if !validOpaque(client.ClientID, 512) || client.HTTPClient == nil || client.Clock == nil || !validUserAgent(client.UserAgent) || (codeVerifier == "" && !validOpaque(client.ClientSecret, 4096)) {
		return socialhub.Token{}, invalidArgument("oauth_exchange", "OAuth client is incomplete")
	}
	values := url.Values{"client_id": {client.ClientID}, "code": {code}, "redirect_uri": {redirectURI}}
	if client.ClientSecret != "" {
		values.Set("client_secret", client.ClientSecret)
	}
	if codeVerifier != "" {
		values.Set("code_verifier", codeVerifier)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.TokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return socialhub.Token{}, platformError("oauth_exchange", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("User-Agent", client.UserAgent)
	response, err := client.HTTPClient.Do(request)
	if err != nil {
		return socialhub.Token{}, platformError("oauth_exchange", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxOAuthResponseBytes+1))
	if err != nil {
		return socialhub.Token{}, platformError("oauth_exchange", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(body)) > maxOAuthResponseBytes {
		return socialhub.Token{}, platformError("oauth_exchange", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("OAuth response exceeded size limit"))
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return socialhub.Token{}, decodeHTTPError(response.StatusCode, response.Header, body)
	}
	var payload struct {
		AccessToken  string `json:"access_token"`
		Expires      int64  `json:"expires"`
		ExpiresIn    int64  `json:"expires_in"`
		ErrorID      int    `json:"error_id"`
		ErrorName    string `json:"error_name"`
		ErrorMessage string `json:"error_message"`
		Error        struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return socialhub.Token{}, platformError("oauth_exchange", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if payload.ErrorID != 0 || payload.ErrorName != "" || payload.ErrorMessage != "" {
		return socialhub.Token{}, wrapperError("oauth_exchange", payload.ErrorID, payload.ErrorName, payload.ErrorMessage, 0)
	}
	if payload.Error.Type != "" {
		return socialhub.Token{}, wrapperError("oauth_exchange", 0, payload.Error.Type, payload.Error.Message, 0)
	}
	if !validOpaque(payload.AccessToken, 4096) {
		return socialhub.Token{}, platformError("oauth_exchange", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	var expiresAt time.Time
	expiresIn := firstPositive(payload.Expires, payload.ExpiresIn)
	if expiresIn > 0 {
		expiresAt = client.Clock.Now().Add(time.Duration(expiresIn) * time.Second)
	}
	return socialhub.Token{AccessToken: payload.AccessToken, TokenType: "Bearer", ExpiresAt: expiresAt}, nil
}

func validCodeChallenge(value string) bool {
	if len(value) != 43 {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') &&
			(character < '0' || character > '9') && character != '-' && character != '_' {
			return false
		}
	}
	return true
}

func validCodeVerifier(value string) bool {
	if len(value) < 43 || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') &&
			(character < '0' || character > '9') && !strings.ContainsRune("-._~", character) {
			return false
		}
	}
	return true
}

func validHTTPURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil
}

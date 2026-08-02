package deviantart

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

const maxOAuthResponseBytes int64 = 1 << 20

// PKCE contains an OAuth 2.1 S256 verifier and challenge.
type PKCE struct {
	Verifier  string
	Challenge string
}

// NewPKCE creates a cryptographically random RFC 7636 verifier and S256 challenge.
func NewPKCE() (PKCE, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return PKCE{}, fmt.Errorf("deviantart: generate PKCE: %w", err)
	}
	verifier := base64.RawURLEncoding.EncodeToString(random)
	digest := sha256.Sum256([]byte(verifier))
	return PKCE{Verifier: verifier, Challenge: base64.RawURLEncoding.EncodeToString(digest[:])}, nil
}

// OAuthClient implements DeviantArt OAuth 2.1 authorization-code, client
// credentials, refresh, and revocation flows.
type OAuthClient struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	RevokeURL    string
	UserAgent    string
	HTTPClient   *http.Client
	Clock        socialhub.Clock
}

func (client *OAuthClient) AuthorizationURL(redirectURI, state string, pkce PKCE, scopes []string) (string, error) {
	if !validOpaque(client.ClientID, 512) || !validEndpoint(client.AuthURL) || !validCallbackURL(redirectURI) ||
		!validOpaque(state, 1024) || !validPKCEValue(pkce.Challenge) || len(scopes) == 0 || !validScopes(scopes) {
		return "", invalidArgument("oauth_authorize", "client ID, redirect URI, state, PKCE S256 challenge, and unique documented scopes are required")
	}
	parsed, err := url.Parse(client.AuthURL)
	if err != nil {
		return "", platformError("oauth_authorize", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	query := parsed.Query()
	query.Set("response_type", "code")
	query.Set("client_id", client.ClientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("scope", strings.Join(scopes, " "))
	query.Set("state", state)
	query.Set("code_challenge", pkce.Challenge)
	query.Set("code_challenge_method", "S256")
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (client *OAuthClient) Exchange(ctx context.Context, code, redirectURI, verifier string) (socialhub.Token, error) {
	if !validOpaque(code, 4096) || !validCallbackURL(redirectURI) || !validPKCEValue(verifier) {
		return socialhub.Token{}, invalidArgument("oauth_exchange", "authorization code, redirect URI, and PKCE verifier are required")
	}
	return client.token(ctx, "oauth_exchange", url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {redirectURI}, "code_verifier": {verifier},
	}, false, true)
}

func (client *OAuthClient) ClientCredentials(ctx context.Context) (socialhub.Token, error) {
	return client.token(ctx, "oauth_client_credentials", url.Values{"grant_type": {"client_credentials"}}, true, false)
}

func (client *OAuthClient) Refresh(ctx context.Context, refreshToken string) (socialhub.Token, error) {
	if !validOpaque(refreshToken, 4096) {
		return socialhub.Token{}, invalidArgument("oauth_refresh", "refresh token is required")
	}
	return client.token(ctx, "oauth_refresh", url.Values{
		"grant_type": {"refresh_token"}, "refresh_token": {refreshToken},
	}, false, true)
}

func (client *OAuthClient) token(ctx context.Context, operation string, values url.Values, requireSecret, requireRefresh bool) (socialhub.Token, error) {
	if !validOpaque(client.ClientID, 512) || client.HTTPClient == nil || client.Clock == nil || !validEndpoint(client.TokenURL) ||
		!validUserAgent(client.UserAgent) || requireSecret && !validOpaque(client.ClientSecret, 4096) {
		return socialhub.Token{}, invalidArgument(operation, "OAuth client is incomplete")
	}
	values.Set("client_id", client.ClientID)
	if client.ClientSecret != "" {
		values.Set("client_secret", client.ClientSecret)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.TokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("User-Agent", client.UserAgent)
	response, err := client.HTTPClient.Do(request)
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxOAuthResponseBytes+1))
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(body)) > maxOAuthResponseBytes {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("OAuth response exceeded size limit"))
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		decoded := decodeHTTPError(response.StatusCode, response.Header, body)
		if hubError, ok := decoded.(*socialhub.Error); ok {
			hubError.Op = operation
		}
		return socialhub.Token{}, decoded
	}
	var payload struct {
		Status           string `json:"status"`
		AccessToken      string `json:"access_token"`
		RefreshToken     string `json:"refresh_token"`
		ExpiresIn        int64  `json:"expires_in"`
		Scope            string `json:"scope"`
		TokenType        string `json:"token_type"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if payload.Error != "" {
		code, class := classifyError(response.StatusCode, payload.Error)
		return socialhub.Token{}, &socialhub.Error{
			Code: code, Class: class, Platform: "deviantart", Product: productName, Op: operation,
			PlatformCode: boundedMessage(payload.Error, 256), PlatformMessage: boundedMessage(payload.ErrorDescription, 512),
		}
	}
	if !validOpaque(payload.AccessToken, 4096) || requireRefresh && !validOpaque(payload.RefreshToken, 4096) ||
		payload.ExpiresIn <= 0 || payload.ExpiresIn > int64((365*24*time.Hour)/time.Second) || payload.Status != "" && payload.Status != "success" {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if payload.TokenType != "" && !strings.EqualFold(payload.TokenType, "bearer") {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return socialhub.Token{
		AccessToken: payload.AccessToken, RefreshToken: payload.RefreshToken, TokenType: "Bearer",
		ExpiresAt: client.Clock.Now().Add(time.Duration(payload.ExpiresIn) * time.Second), Scopes: strings.Fields(payload.Scope),
	}, nil
}

// Revoke invalidates all app tokens for a user unless refreshOnly is true.
func (client *OAuthClient) Revoke(ctx context.Context, token string, refreshOnly bool) error {
	if !validOpaque(token, 4096) || client.HTTPClient == nil || !validEndpoint(client.RevokeURL) || !validUserAgent(client.UserAgent) {
		return invalidArgument("oauth_revoke", "token or OAuth client is invalid")
	}
	values := url.Values{"token": {token}}
	if refreshOnly {
		values.Set("revoke_refresh_only", "true")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.RevokeURL, strings.NewReader(values.Encode()))
	if err != nil {
		return platformError("oauth_revoke", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("User-Agent", client.UserAgent)
	response, err := client.HTTPClient.Do(request)
	if err != nil {
		return platformError("oauth_revoke", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxOAuthResponseBytes+1))
	if err != nil {
		return platformError("oauth_revoke", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(body)) > maxOAuthResponseBytes {
		return platformError("oauth_revoke", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		decoded := decodeHTTPError(response.StatusCode, response.Header, body)
		if hubError, ok := decoded.(*socialhub.Error); ok {
			hubError.Op = "oauth_revoke"
		}
		return decoded
	}
	var payload struct {
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || !payload.Success {
		return platformError("oauth_revoke", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return nil
}

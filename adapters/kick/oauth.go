package kick

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

var validOAuthScopes = map[string]struct{}{
	"user:read": {}, "channel:read": {}, "channel:write": {}, "channel:rewards:read": {},
	"channel:rewards:write": {}, "chat:write": {}, "streamkey:read": {}, "events:subscribe": {},
	"moderation:ban": {}, "moderation:chat_message:manage": {}, "kicks:read": {},
}

// PKCE contains an OAuth 2.1 S256 verifier and challenge.
type PKCE struct {
	Verifier  string
	Challenge string
}

// NewPKCE creates a cryptographically random RFC 7636 verifier and S256 challenge.
func NewPKCE() (PKCE, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return PKCE{}, fmt.Errorf("kick: generate PKCE: %w", err)
	}
	verifier := base64.RawURLEncoding.EncodeToString(random)
	digest := sha256.Sum256([]byte(verifier))
	return PKCE{Verifier: verifier, Challenge: base64.RawURLEncoding.EncodeToString(digest[:])}, nil
}

// OAuthClient implements Kick user authorization, app tokens, refresh, revoke, and introspection.
type OAuthClient struct {
	ClientID      string
	ClientSecret  string
	AuthURL       string
	TokenURL      string
	RevokeURL     string
	IntrospectURL string
	HTTPClient    *http.Client
	Clock         socialhub.Clock
}

func (client *OAuthClient) AuthorizationURL(redirectURI, state string, pkce PKCE, scopes []string) (string, error) {
	if strings.TrimSpace(client.ClientID) == "" || !validEndpoint(client.AuthURL) || !validRedirectURI(redirectURI) ||
		strings.TrimSpace(state) == "" || !validPKCEValue(pkce.Challenge) || len(scopes) == 0 {
		return "", invalidArgument("oauth_authorize", "client ID, redirect URI, state, PKCE S256 challenge, and scopes are required")
	}
	seen := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		if _, valid := validOAuthScopes[scope]; !valid {
			return "", invalidArgument("oauth_authorize", "scope is not supported by this Kick API version")
		}
		if _, duplicate := seen[scope]; duplicate {
			return "", invalidArgument("oauth_authorize", "duplicate OAuth scopes are not allowed")
		}
		seen[scope] = struct{}{}
	}
	parsed, err := url.Parse(client.AuthURL)
	if err != nil {
		return "", platformError("oauth_authorize", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	query := parsed.Query()
	query.Set("client_id", client.ClientID)
	query.Set("response_type", "code")
	query.Set("redirect_uri", redirectURI)
	query.Set("state", state)
	query.Set("scope", strings.Join(scopes, " "))
	query.Set("code_challenge", pkce.Challenge)
	query.Set("code_challenge_method", "S256")
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (client *OAuthClient) Exchange(ctx context.Context, code, redirectURI, verifier string) (socialhub.Token, error) {
	if strings.TrimSpace(code) == "" || !validRedirectURI(redirectURI) || !validPKCEValue(verifier) {
		return socialhub.Token{}, invalidArgument("oauth_exchange", "code, redirect URI, and PKCE verifier are required")
	}
	values := url.Values{
		"grant_type": {"authorization_code"}, "client_id": {client.ClientID}, "client_secret": {client.ClientSecret},
		"redirect_uri": {redirectURI}, "code_verifier": {verifier}, "code": {code},
	}
	return client.token(ctx, values, "oauth_exchange", true)
}

func (client *OAuthClient) ClientCredentials(ctx context.Context) (socialhub.Token, error) {
	values := url.Values{
		"grant_type": {"client_credentials"}, "client_id": {client.ClientID}, "client_secret": {client.ClientSecret},
	}
	return client.token(ctx, values, "oauth_client_credentials", false)
}

func (client *OAuthClient) Refresh(ctx context.Context, refreshToken string) (socialhub.Token, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return socialhub.Token{}, invalidArgument("oauth_refresh", "refresh token is required")
	}
	values := url.Values{
		"grant_type": {"refresh_token"}, "client_id": {client.ClientID}, "client_secret": {client.ClientSecret},
		"refresh_token": {refreshToken},
	}
	return client.token(ctx, values, "oauth_refresh", true)
}

func (client *OAuthClient) token(ctx context.Context, values url.Values, operation string, requireRefresh bool) (socialhub.Token, error) {
	if strings.TrimSpace(client.ClientID) == "" || strings.TrimSpace(client.ClientSecret) == "" ||
		client.HTTPClient == nil || client.Clock == nil || !validEndpoint(client.TokenURL) {
		return socialhub.Token{}, invalidArgument(operation, "OAuth client credentials, HTTP client, clock, and token URL are required")
	}
	var response struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
		Scope        string `json:"scope"`
	}
	if err := client.execute(ctx, http.MethodPost, client.TokenURL, strings.NewReader(values.Encode()), "", operation, &response); err != nil {
		return socialhub.Token{}, err
	}
	if !validBearerToken(response.AccessToken) || (response.TokenType != "" && !strings.EqualFold(response.TokenType, "bearer")) ||
		(requireRefresh && strings.TrimSpace(response.RefreshToken) == "") || response.ExpiresIn < 0 || response.ExpiresIn > int64((366*24*time.Hour)/time.Second) {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	var expiresAt time.Time
	if response.ExpiresIn > 0 {
		expiresAt = client.Clock.Now().Add(time.Duration(response.ExpiresIn) * time.Second)
	}
	return socialhub.Token{
		AccessToken: strings.TrimSpace(response.AccessToken), RefreshToken: response.RefreshToken, TokenType: "Bearer",
		ExpiresAt: expiresAt, Scopes: strings.Fields(response.Scope),
	}, nil
}

func (client *OAuthClient) Revoke(ctx context.Context, token, tokenHintType string) error {
	if strings.TrimSpace(token) == "" || (tokenHintType != "access_token" && tokenHintType != "refresh_token") || !validEndpoint(client.RevokeURL) {
		return invalidArgument("oauth_revoke", "token and access_token or refresh_token hint are required")
	}
	parsed, err := url.Parse(client.RevokeURL)
	if err != nil {
		return platformError("oauth_revoke", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	query := parsed.Query()
	query.Set("token", token)
	query.Set("token_hint_type", tokenHintType)
	parsed.RawQuery = query.Encode()
	return client.execute(ctx, http.MethodPost, parsed.String(), strings.NewReader(""), "", "oauth_revoke", nil)
}

func (client *OAuthClient) Introspect(ctx context.Context, accessToken string) (*TokenIntrospection, error) {
	if !validBearerToken(accessToken) || !validEndpoint(client.IntrospectURL) {
		return nil, invalidArgument("oauth_introspect", "access token and introspection URL are required")
	}
	var response responseEnvelope[TokenIntrospection]
	if err := client.execute(ctx, http.MethodPost, client.IntrospectURL, strings.NewReader(""), strings.TrimSpace(accessToken), "oauth_introspect", &response); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

func (client *OAuthClient) execute(ctx context.Context, method, endpoint string, body io.Reader, bearer, operation string, output any) error {
	if client.HTTPClient == nil {
		return invalidArgument(operation, "HTTP client is required")
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if bearer != "" {
		request.Header.Set("Authorization", "Bearer "+bearer)
	}
	response, err := client.HTTPClient.Do(request)
	if err != nil {
		return platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, sanitizeTransportError(err))
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, maxOAuthResponseBytes+1))
	if err != nil {
		return platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(payload)) > maxOAuthResponseBytes {
		return platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		decoded := decodeHTTPError(response.StatusCode, response.Header, payload)
		if hubError, ok := decoded.(*socialhub.Error); ok {
			hubError.Op = operation
		}
		return decoded
	}
	if output == nil || len(payload) == 0 {
		return nil
	}
	if err := json.Unmarshal(payload, output); err != nil {
		return platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return nil
}

func validRedirectURI(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && parsed.Fragment == ""
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

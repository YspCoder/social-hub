package mercadodisplayads

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"social-hub/pkg/socialhub"
)

const maxOAuthResponseBytes = 1 << 20

type OAuthClient struct {
	mu           sync.RWMutex
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	HTTPClient   *http.Client
	Clock        socialhub.Clock
	closed       bool
}

type oauthSnapshot struct {
	clientID     string
	clientSecret string
	authURL      string
	tokenURL     string
	httpClient   *http.Client
	clock        socialhub.Clock
}

type AuthorizationRequest struct {
	RedirectURI         string
	State               string
	CodeChallenge       string
	CodeChallengeMethod string
}

type ExchangeRequest struct {
	Code         string
	RedirectURI  string
	CodeVerifier string
}

type TokenResult struct {
	Token  socialhub.Token
	UserID int64
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	Scope        string `json:"scope"`
	UserID       int64  `json:"user_id"`
	RefreshToken string `json:"refresh_token"`
}

func (client *OAuthClient) AuthorizationURL(input AuthorizationRequest) (string, error) {
	if !validCallbackURL(input.RedirectURI) || !validOpaque(input.State, 1024) {
		return "", invalidArgument("oauth_authorize", "OAuth client, redirect_uri, and state are required")
	}
	if !validPKCEChallenge(input.CodeChallenge, input.CodeChallengeMethod) {
		return "", invalidArgument("oauth_authorize", "code_challenge and code_challenge_method must be omitted together or use a valid S256/plain PKCE pair")
	}
	snapshot, err := client.snapshot("oauth_authorize")
	if err != nil {
		return "", err
	}
	parsed, _ := url.Parse(snapshot.authURL)
	query := parsed.Query()
	query.Set("response_type", "code")
	query.Set("client_id", snapshot.clientID)
	query.Set("redirect_uri", input.RedirectURI)
	query.Set("state", input.State)
	if input.CodeChallenge != "" {
		query.Set("code_challenge", input.CodeChallenge)
		query.Set("code_challenge_method", input.CodeChallengeMethod)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (client *OAuthClient) Exchange(ctx context.Context, input ExchangeRequest) (TokenResult, error) {
	if ctx == nil || !validOpaque(input.Code, 4096) || !validCallbackURL(input.RedirectURI) ||
		input.CodeVerifier != "" && !validPKCEVerifier(input.CodeVerifier) {
		return TokenResult{}, invalidArgument("oauth_exchange", "authorization code, redirect_uri, or PKCE code_verifier is invalid")
	}
	snapshot, err := client.snapshot("oauth_exchange")
	if err != nil {
		return TokenResult{}, err
	}
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {snapshot.clientID},
		"client_secret": {snapshot.clientSecret},
		"code":          {input.Code},
		"redirect_uri":  {input.RedirectURI},
	}
	if input.CodeVerifier != "" {
		form.Set("code_verifier", input.CodeVerifier)
	}
	return exchangeOAuthToken(ctx, "oauth_exchange", form, "", false, snapshot)
}

func (client *OAuthClient) Refresh(ctx context.Context, refreshToken string) (socialhub.Token, error) {
	if ctx == nil || !validOpaque(refreshToken, 16_384) {
		return socialhub.Token{}, invalidArgument("oauth_refresh", "refresh token is required")
	}
	snapshot, err := client.snapshot("oauth_refresh")
	if err != nil {
		return socialhub.Token{}, err
	}
	result, err := exchangeOAuthToken(ctx, "oauth_refresh", url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {snapshot.clientID},
		"client_secret": {snapshot.clientSecret},
		"refresh_token": {refreshToken},
	}, refreshToken, true, snapshot)
	if err != nil {
		return socialhub.Token{}, err
	}
	return result.Token, nil
}

func exchangeOAuthToken(ctx context.Context, operation string, form url.Values, oldRefreshToken string, requireRotation bool, snapshot oauthSnapshot) (TokenResult, error) {
	started := snapshot.clock.Now()
	if !started.After(time.Unix(0, 0)) {
		return TokenResult{}, invalidArgument(operation, "clock must return a time after the Unix epoch")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, snapshot.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return TokenResult{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Accept-Encoding", "identity")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := cloneHTTPClient(snapshot.httpClient).Do(request)
	if err != nil {
		return TokenResult{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxOAuthResponseBytes+1))
	if err != nil {
		return TokenResult{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if len(body) > maxOAuthResponseBytes {
		return TokenResult{}, platformContractError(operation, "Mercado Libre token response exceeded 1 MiB")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		sensitive := []string{snapshot.clientSecret, form.Get("code"), form.Get("code_verifier"), form.Get("refresh_token")}
		return TokenResult{}, withOperation(decodeHTTPError(response.StatusCode, response.Header, body, started, sensitive...), operation)
	}
	if response.StatusCode != http.StatusOK {
		return TokenResult{}, platformContractError(operation, "Mercado Libre returned an unexpected successful token HTTP status")
	}
	if !validJSONMediaType(response.Header.Get("Content-Type")) {
		return TokenResult{}, platformContractError(operation, "Mercado Libre token response was not JSON")
	}
	var wire tokenResponse
	if err := json.Unmarshal(body, &wire); err != nil {
		return TokenResult{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	expiresAt, ok := tokenExpiry(started, wire.ExpiresIn)
	scopes, scopesValid := splitScopes(wire.Scope)
	if !validOpaque(wire.AccessToken, 16_384) || wire.UserID <= 0 || !ok ||
		!strings.EqualFold(wire.TokenType, "bearer") || !scopesValid {
		return TokenResult{}, platformContractError(operation, "Mercado Libre returned invalid access-token fields")
	}
	refreshToken := wire.RefreshToken
	if refreshToken == "" && !requireRotation {
		refreshToken = oldRefreshToken
	}
	if requireRotation && (!validOpaque(refreshToken, 16_384) || refreshToken == oldRefreshToken) {
		return TokenResult{}, platformContractError(operation, "Mercado Libre did not rotate the single-use refresh token")
	}
	if refreshToken != "" && !validOpaque(refreshToken, 16_384) {
		return TokenResult{}, platformContractError(operation, "Mercado Libre returned an invalid refresh token")
	}
	return TokenResult{Token: socialhub.Token{
		AccessToken: wire.AccessToken, RefreshToken: refreshToken, TokenType: "Bearer",
		ExpiresAt: expiresAt, Scopes: scopes,
	}, UserID: wire.UserID}, nil
}

func (client *OAuthClient) snapshot(operation string) (oauthSnapshot, error) {
	if client == nil {
		return oauthSnapshot{}, invalidArgument(operation, "OAuth client is required")
	}
	client.mu.RLock()
	defer client.mu.RUnlock()
	if client.closed {
		return oauthSnapshot{}, platformError(operation, socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	snapshot := oauthSnapshot{
		clientID: client.ClientID, clientSecret: client.ClientSecret, authURL: client.AuthURL,
		tokenURL: client.TokenURL, httpClient: client.HTTPClient, clock: client.Clock,
	}
	if !validPositiveDecimal(snapshot.clientID) || !validOpaque(snapshot.clientSecret, 16_384) ||
		!validAuthorizationEndpoint(snapshot.authURL) || !validTokenEndpoint(snapshot.tokenURL) ||
		snapshot.httpClient == nil || snapshot.clock == nil {
		return oauthSnapshot{}, invalidArgument(operation, "OAuth client is incomplete")
	}
	return snapshot, nil
}

func tokenExpiry(now time.Time, seconds int64) (time.Time, bool) {
	if seconds <= 0 || seconds > int64((24*time.Hour)/time.Second) {
		return time.Time{}, false
	}
	return now.Add(time.Duration(seconds) * time.Second), true
}

func splitScopes(value string) ([]string, bool) {
	if len(value) > 4096 {
		return nil, false
	}
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return nil, true
	}
	if len(fields) > 32 {
		return nil, false
	}
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		if !validOpaque(field, 256) {
			return nil, false
		}
		if _, duplicate := seen[field]; duplicate {
			return nil, false
		}
		seen[field] = struct{}{}
	}
	return append([]string(nil), fields...), true
}

// Close clears OAuth credentials and prevents new authorization work.
func (client *OAuthClient) Close() {
	if client == nil {
		return
	}
	client.mu.Lock()
	client.ClientID, client.ClientSecret, client.AuthURL, client.TokenURL = "", "", "", ""
	client.HTTPClient, client.Clock, client.closed = nil, nil, true
	client.mu.Unlock()
}

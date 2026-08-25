package merchantapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"social-hub/pkg/socialhub"
)

const maxOAuthResponseBytes int64 = 1 << 20

// OAuthClient implements Google's OAuth2 web-server flow with offline access.
type OAuthClient struct {
	mu           sync.RWMutex
	ClientID     string
	ClientSecret string
	HTTPClient   *http.Client
	Clock        socialhub.Clock
	requestIDs   *requestIDFilter
	closed       bool
}

func (client *OAuthClient) AuthorizationURL(redirectURI, state string) (string, error) {
	if client == nil {
		return "", invalidArgument("oauth_authorize", "OAuth client is required")
	}
	client.mu.RLock()
	defer client.mu.RUnlock()
	if client.closed {
		return "", platformError("oauth_authorize", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	if !validOpaque(client.ClientID, 1024) || !validCallbackURL(redirectURI) || !validOpaque(state, 1024) {
		return "", invalidArgument("oauth_authorize", "client ID, redirect URI, or state is invalid")
	}
	parsed, _ := url.Parse(defaultAuthURL)
	query := parsed.Query()
	query.Set("client_id", client.ClientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("response_type", "code")
	query.Set("scope", contentScope)
	query.Set("state", state)
	query.Set("access_type", "offline")
	query.Set("include_granted_scopes", "true")
	query.Set("prompt", "consent")
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (client *OAuthClient) Exchange(ctx context.Context, code, redirectURI string) (socialhub.Token, error) {
	if client == nil {
		return socialhub.Token{}, invalidArgument("oauth_exchange", "OAuth client is required")
	}
	if ctx == nil || !validOpaque(code, 4096) || !validCallbackURL(redirectURI) {
		return socialhub.Token{}, invalidArgument("oauth_exchange", "authorization code and redirect URI are required")
	}
	client.mu.RLock()
	if client.closed {
		client.mu.RUnlock()
		return socialhub.Token{}, platformError("oauth_exchange", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	clientID, clientSecret := client.ClientID, client.ClientSecret
	httpClient, clock := client.HTTPClient, client.Clock
	requestIDs := client.requestIDs.with(clientID, clientSecret, code)
	client.mu.RUnlock()
	defer requestIDs.clear()
	values := url.Values{
		"client_id": {clientID}, "client_secret": {clientSecret}, "code": {code},
		"grant_type": {"authorization_code"}, "redirect_uri": {redirectURI},
	}
	return exchangeOAuthToken(ctx, "oauth_exchange", values, "", clientID, clientSecret, httpClient, clock, requestIDs)
}

func (client *OAuthClient) Refresh(ctx context.Context, refreshToken string) (socialhub.Token, error) {
	if client == nil {
		return socialhub.Token{}, invalidArgument("oauth_refresh", "OAuth client is required")
	}
	if ctx == nil || !validOpaque(refreshToken, 16384) {
		return socialhub.Token{}, invalidArgument("oauth_refresh", "refresh token is required")
	}
	client.mu.RLock()
	if client.closed {
		client.mu.RUnlock()
		return socialhub.Token{}, platformError("oauth_refresh", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	clientID, clientSecret := client.ClientID, client.ClientSecret
	httpClient, clock := client.HTTPClient, client.Clock
	requestIDs := client.requestIDs.with(clientID, clientSecret, refreshToken)
	client.mu.RUnlock()
	defer requestIDs.clear()
	values := url.Values{
		"client_id": {clientID}, "client_secret": {clientSecret},
		"grant_type": {"refresh_token"}, "refresh_token": {refreshToken},
	}
	return exchangeOAuthToken(ctx, "oauth_refresh", values, refreshToken, clientID, clientSecret, httpClient, clock, requestIDs)
}

func exchangeOAuthToken(ctx context.Context, operation string, values url.Values, existingRefreshToken, clientID, clientSecret string, httpClient *http.Client, clock socialhub.Clock, requestIDs *requestIDFilter) (socialhub.Token, error) {
	if !validOpaque(clientID, 1024) || !validOpaque(clientSecret, 4096) || httpClient == nil || clock == nil {
		return socialhub.Token{}, invalidArgument(operation, "OAuth client is incomplete")
	}
	started := clock.Now()
	if !started.After(time.Unix(0, 0)) {
		return socialhub.Token{}, invalidArgument(operation, "clock must return a time after the Unix epoch")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, defaultTokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Accept-Encoding", "identity")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := cloneHTTPClient(httpClient).Do(request)
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
	var payload struct {
		AccessToken      string `json:"access_token"`
		ExpiresIn        int64  `json:"expires_in"`
		RefreshToken     string `json:"refresh_token"`
		Scope            string `json:"scope"`
		TokenType        string `json:"token_type"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	decodeErr := json.Unmarshal(body, &payload)
	if response.StatusCode != http.StatusOK {
		if decodeErr != nil {
			return socialhub.Token{}, withOperation(decodeHTTPError(response.StatusCode, response.Header, body, started, requestIDs), operation)
		}
		return socialhub.Token{}, oauthError(operation, response.StatusCode, response.Header, payload.Error, payload.ErrorDescription, started, requestIDs)
	}
	if !validJSONContentType(response.Header.Get("Content-Type")) {
		return socialhub.Token{}, platformContractError(operation, "OAuth success response was not JSON")
	}
	if decodeErr != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, decodeErr)
	}
	requestIDs.add(payload.AccessToken, payload.RefreshToken)
	if payload.Error != "" {
		return socialhub.Token{}, oauthError(operation, response.StatusCode, response.Header, payload.Error, payload.ErrorDescription, started, requestIDs)
	}
	refreshToken := payload.RefreshToken
	if refreshToken == "" {
		refreshToken = existingRefreshToken
	}
	if !validOpaque(payload.AccessToken, 16384) || payload.ExpiresIn <= 0 ||
		payload.ExpiresIn > int64((24*time.Hour)/time.Second) ||
		(payload.TokenType != "" && !strings.EqualFold(payload.TokenType, "bearer")) {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("invalid token response fields"))
	}
	if !validOpaque(refreshToken, 16384) {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("offline OAuth response omitted refresh token"))
	}
	scopes := strings.Fields(payload.Scope)
	if len(scopes) == 0 {
		scopes = []string{contentScope}
	}
	if !containsScope(scopes, contentScope) {
		return socialhub.Token{}, &socialhub.Error{
			Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction,
			Platform: platformName, Product: productName, Op: operation,
			RequiredScopes: []string{contentScope}, ApprovalURL: defaultAuthURL,
			PlatformMessage: "Google OAuth token omitted the Merchant API content scope",
		}
	}
	return socialhub.Token{
		AccessToken: payload.AccessToken, RefreshToken: refreshToken, TokenType: "Bearer",
		ExpiresAt: started.Add(time.Duration(payload.ExpiresIn) * time.Second), Scopes: scopes,
	}, nil
}

func containsScope(scopes []string, expected string) bool {
	for _, scope := range scopes {
		if scope == expected {
			return true
		}
	}
	return false
}

// Close clears the OAuth client secret and prevents new authorization work.
func (client *OAuthClient) Close() {
	if client == nil {
		return
	}
	client.mu.Lock()
	client.ClientID, client.ClientSecret = "", ""
	client.closed = true
	if client.requestIDs != nil {
		client.requestIDs.clear()
	}
	client.mu.Unlock()
}

func oauthError(operation string, status int, header http.Header, platformCode, message string, now time.Time, requestIDs *requestIDFilter) error {
	platformCode = requestIDs.redact(platformCode)
	message = requestIDs.redact(message)
	code, class := classifyError(status, "", platformCode)
	switch platformCode {
	case "invalid_client", "invalid_grant", "unauthorized_client", "access_denied":
		code, class = socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "invalid_request", "unsupported_grant_type", "invalid_scope":
		code, class = socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case "temporarily_unavailable", "server_error":
		code, class = socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation, HTTPStatus: status,
		PlatformCode: boundedMessage(platformCode, 256), PlatformMessage: boundedMessage(message, 1024),
		RequestID:  responseRequestID(header, requestIDs),
		RetryAfter: parseRetryAfter(header.Get("Retry-After"), now),
	}
}

func validCallbackURL(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return false
	}
	if parsed.Scheme == "https" {
		return true
	}
	if parsed.Scheme != "http" {
		return false
	}
	host := parsed.Hostname()
	address := net.ParseIP(host)
	return strings.EqualFold(host, "localhost") || address != nil && address.IsLoopback()
}

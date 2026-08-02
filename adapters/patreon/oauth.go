package patreon

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

var oauthScopes = map[string]struct{}{
	"identity": {}, "identity[email]": {}, "identity.memberships": {}, "campaigns": {},
	"campaigns.members": {}, "campaigns.members[email]": {}, "campaigns.members.address": {},
	"campaigns.posts": {}, "w:campaigns.webhook": {}, "campaigns.lives": {}, "w:campaigns.lives": {},
}

// OAuthClient implements Patreon API v2 authorization-code and rotating
// refresh-token grants.
type OAuthClient struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	HTTPClient   *http.Client
	Clock        socialhub.Clock
}

func (client *OAuthClient) AuthorizationURL(redirectURI, state string, scopes []string) (string, error) {
	if !validOpaque(client.ClientID, 512) || !validCallbackURL(redirectURI) || !validOpaque(state, 1024) || len(scopes) == 0 || !validEndpoint(client.AuthURL) {
		return "", invalidArgument("oauth_authorize", "client ID, redirect URI, state, scopes, or authorization endpoint is invalid")
	}
	for _, scope := range scopes {
		if _, found := oauthScopes[scope]; !found {
			return "", invalidArgument("oauth_authorize", "scope is not documented by Patreon API v2")
		}
	}
	parsed, _ := url.Parse(client.AuthURL)
	query := parsed.Query()
	query.Set("response_type", "code")
	query.Set("client_id", client.ClientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("state", state)
	query.Set("scope", strings.Join(scopes, " "))
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (client *OAuthClient) Exchange(ctx context.Context, code, redirectURI string) (socialhub.Token, error) {
	if !validOpaque(code, 4096) || !validCallbackURL(redirectURI) {
		return socialhub.Token{}, invalidArgument("oauth_exchange", "authorization code and redirect URI are required")
	}
	return client.token(ctx, "oauth_exchange", url.Values{
		"code": {code}, "grant_type": {"authorization_code"}, "client_id": {client.ClientID},
		"client_secret": {client.ClientSecret}, "redirect_uri": {redirectURI},
	})
}

func (client *OAuthClient) Refresh(ctx context.Context, refreshToken string) (socialhub.Token, error) {
	if !validOpaque(refreshToken, 4096) {
		return socialhub.Token{}, invalidArgument("oauth_refresh", "refresh token is required")
	}
	return client.token(ctx, "oauth_refresh", url.Values{
		"grant_type": {"refresh_token"}, "refresh_token": {refreshToken},
		"client_id": {client.ClientID}, "client_secret": {client.ClientSecret},
	})
}

func (client *OAuthClient) token(ctx context.Context, operation string, values url.Values) (socialhub.Token, error) {
	if !validOpaque(client.ClientID, 512) || !validOpaque(client.ClientSecret, 4096) || client.HTTPClient == nil || client.Clock == nil || !validEndpoint(client.TokenURL) {
		return socialhub.Token{}, invalidArgument(operation, "OAuth client is incomplete")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.TokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
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
		return socialhub.Token{}, decodeHTTPError(response.StatusCode, response.Header, body)
	}
	var payload struct {
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
		return socialhub.Token{}, oauthError(operation, payload.Error, payload.ErrorDescription)
	}
	if !validOpaque(payload.AccessToken, 4096) || !validOpaque(payload.RefreshToken, 4096) || payload.ExpiresIn <= 0 || payload.ExpiresIn > int64((365*24*time.Hour)/time.Second) {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	tokenType := payload.TokenType
	if tokenType == "" || strings.EqualFold(tokenType, "bearer") {
		tokenType = "Bearer"
	}
	if !validOpaque(tokenType, 64) {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return socialhub.Token{
		AccessToken: payload.AccessToken, RefreshToken: payload.RefreshToken, TokenType: tokenType,
		ExpiresAt: client.Clock.Now().Add(time.Duration(payload.ExpiresIn) * time.Second), Scopes: strings.Fields(payload.Scope),
	}, nil
}

func oauthError(operation, code, message string) error {
	errorCode, class := socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	if code == "invalid_request" || code == "invalid_scope" || code == "unsupported_grant_type" {
		errorCode, class = socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	} else if code == "temporarily_unavailable" || code == "server_error" {
		errorCode, class = socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	}
	return &socialhub.Error{
		Code: errorCode, Class: class, Platform: "patreon", Product: productName, Op: operation,
		PlatformCode: boundedMessage(code, 256), PlatformMessage: boundedMessage(message, 512),
	}
}

func validCallbackURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil && parsed.Fragment == ""
}

package ads

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

// OAuthClient implements Pinterest OAuth2 authorization-code, refresh-token,
// and client-credentials grants for ads scopes.
type OAuthClient struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	HTTPClient   *http.Client
	Clock        socialhub.Clock
}

type TokenResult struct {
	Token            socialhub.Token
	ResponseType     string
	RefreshExpiresAt time.Time
}

func (client *OAuthClient) AuthorizationURL(redirectURI, state string, scopes []string) (string, error) {
	if !validOpaque(client.ClientID, 1024) || !validCallbackURL(redirectURI) || !validOpaque(state, 1024) ||
		!validScopes(scopes) || !validEndpoint(client.AuthURL) {
		return "", invalidArgument("oauth_authorize", "client ID, redirect URI, state, ads scopes, or authorization endpoint is invalid")
	}
	parsed, _ := url.Parse(client.AuthURL)
	query := parsed.Query()
	query.Set("client_id", client.ClientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("response_type", "code")
	query.Set("scope", strings.Join(scopes, ","))
	query.Set("state", state)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (client *OAuthClient) Exchange(ctx context.Context, code, redirectURI string) (TokenResult, error) {
	if !validOpaque(code, 4096) || !validCallbackURL(redirectURI) {
		return TokenResult{}, invalidArgument("oauth_exchange", "authorization code and redirect URI are required")
	}
	return client.token(ctx, "oauth_exchange", url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {redirectURI},
	}, "")
}

func (client *OAuthClient) Refresh(ctx context.Context, refreshToken string, scopes []string) (TokenResult, error) {
	if !validOpaque(refreshToken, 4096) || len(scopes) > 0 && !validScopes(scopes) {
		return TokenResult{}, invalidArgument("oauth_refresh", "refresh token or ads scopes are invalid")
	}
	values := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refreshToken}}
	if len(scopes) > 0 {
		values.Set("scope", strings.Join(scopes, ","))
	}
	return client.token(ctx, "oauth_refresh", values, refreshToken)
}

func (client *OAuthClient) ClientCredentials(ctx context.Context, scopes []string) (TokenResult, error) {
	if !validScopes(scopes) {
		return TokenResult{}, invalidArgument("oauth_client_credentials", "ads scopes are required")
	}
	return client.token(ctx, "oauth_client_credentials", url.Values{
		"grant_type": {"client_credentials"}, "scope": {strings.Join(scopes, ",")},
	}, "")
}

func (client *OAuthClient) token(ctx context.Context, operation string, values url.Values, existingRefreshToken string) (TokenResult, error) {
	if !validOpaque(client.ClientID, 1024) || !validOpaque(client.ClientSecret, 4096) || client.HTTPClient == nil || client.Clock == nil || !validEndpoint(client.TokenURL) {
		return TokenResult{}, invalidArgument(operation, "OAuth client is incomplete")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.TokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return TokenResult{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.SetBasicAuth(client.ClientID, client.ClientSecret)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := client.HTTPClient.Do(request)
	if err != nil {
		return TokenResult{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxOAuthResponseBytes+1))
	if err != nil {
		return TokenResult{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(body)) > maxOAuthResponseBytes {
		return TokenResult{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("OAuth response exceeded size limit"))
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return TokenResult{}, decodeHTTPError(response.StatusCode, response.Header, body)
	}
	var payload struct {
		AccessToken           string `json:"access_token"`
		RefreshToken          string `json:"refresh_token"`
		TokenType             string `json:"token_type"`
		ResponseType          string `json:"response_type"`
		ExpiresIn             int64  `json:"expires_in"`
		RefreshTokenExpiresIn int64  `json:"refresh_token_expires_in"`
		RefreshTokenExpiresAt int64  `json:"refresh_token_expires_at"`
		Scope                 string `json:"scope"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return TokenResult{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if !validOpaque(payload.AccessToken, 8192) || payload.ExpiresIn <= 0 || payload.ExpiresIn > int64((2*365*24*time.Hour)/time.Second) {
		return TokenResult{}, platformContractError(operation, "Pinterest returned an invalid OAuth token response")
	}
	refreshToken := payload.RefreshToken
	if refreshToken == "" {
		refreshToken = existingRefreshToken
	}
	if refreshToken != "" && !validOpaque(refreshToken, 8192) {
		return TokenResult{}, platformContractError(operation, "Pinterest returned an invalid refresh token")
	}
	tokenType := payload.TokenType
	if tokenType == "" || strings.EqualFold(tokenType, "bearer") {
		tokenType = "Bearer"
	}
	now := client.Clock.Now()
	refreshExpiresAt := time.Time{}
	if payload.RefreshTokenExpiresAt > 0 {
		refreshExpiresAt = time.Unix(payload.RefreshTokenExpiresAt, 0)
	} else if payload.RefreshTokenExpiresIn > 0 {
		refreshExpiresAt = now.Add(time.Duration(payload.RefreshTokenExpiresIn) * time.Second)
	}
	return TokenResult{
		Token: socialhub.Token{
			AccessToken: payload.AccessToken, RefreshToken: refreshToken, TokenType: tokenType,
			ExpiresAt: now.Add(time.Duration(payload.ExpiresIn) * time.Second), Scopes: splitScopes(payload.Scope),
		},
		ResponseType: payload.ResponseType, RefreshExpiresAt: refreshExpiresAt,
	}, nil
}

func validScopes(scopes []string) bool {
	if len(scopes) == 0 || len(scopes) > 2 {
		return false
	}
	seen := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		if !validOAuthScope(scope) {
			return false
		}
		if _, found := seen[scope]; found {
			return false
		}
		seen[scope] = struct{}{}
	}
	return true
}

func splitScopes(value string) []string {
	return strings.FieldsFunc(value, func(character rune) bool { return character == ',' || unicodeSpace(character) })
}

func unicodeSpace(character rune) bool {
	return character == ' ' || character == '\t' || character == '\r' || character == '\n'
}

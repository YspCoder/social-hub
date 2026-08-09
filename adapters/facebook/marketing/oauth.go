package marketing

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

const maxOAuthResponseBytes int64 = 1 << 20

// OAuthClient implements Meta's server-side authorization-code flow and the
// short-lived to long-lived user-token exchange.
type OAuthClient struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	HTTPClient   *http.Client
	Clock        socialhub.Clock
}

func (client *OAuthClient) AuthorizationURL(redirectURI, state string, scopes []string) (string, error) {
	if !validOpaque(client.ClientID, 1024) || !validCallbackURL(redirectURI) || !validOpaque(state, 4096) {
		return "", invalidArgument("oauth_authorize", "client ID, callback URL, and state are required")
	}
	for _, scope := range scopes {
		if !validFieldName(scope) {
			return "", invalidArgument("oauth_authorize", "scope is invalid")
		}
	}
	parsed, err := url.Parse(client.AuthURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", invalidArgument("oauth_authorize", "authorization URL is invalid")
	}
	query := parsed.Query()
	query.Set("client_id", client.ClientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("state", state)
	query.Set("response_type", "code")
	query.Set("scope", strings.Join(scopes, ","))
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (client *OAuthClient) Exchange(ctx context.Context, code, redirectURI string) (socialhub.Token, error) {
	if !validOpaque(code, 4096) || !validCallbackURL(redirectURI) {
		return socialhub.Token{}, invalidArgument("oauth_exchange", "code and callback URL are required")
	}
	return client.token(ctx, "oauth_exchange", url.Values{
		"client_id": {client.ClientID}, "client_secret": {client.ClientSecret},
		"redirect_uri": {redirectURI}, "code": {code},
	})
}

func (client *OAuthClient) ExchangeLongLived(ctx context.Context, shortLivedToken string) (socialhub.Token, error) {
	if !validOpaque(shortLivedToken, 8192) {
		return socialhub.Token{}, invalidArgument("oauth_long_lived", "short-lived token is required")
	}
	return client.token(ctx, "oauth_long_lived", url.Values{
		"grant_type": {"fb_exchange_token"}, "client_id": {client.ClientID},
		"client_secret": {client.ClientSecret}, "fb_exchange_token": {shortLivedToken},
	})
}

func (client *OAuthClient) token(ctx context.Context, operation string, form url.Values) (socialhub.Token, error) {
	if !validOpaque(client.ClientID, 1024) || !validOpaque(client.ClientSecret, 4096) || !validEndpoint(client.TokenURL) || client.HTTPClient == nil || client.Clock == nil {
		return socialhub.Token{}, invalidArgument(operation, "OAuth client is incomplete")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
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
		return socialhub.Token{}, platformContractError(operation, "OAuth response exceeded size limit")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return socialhub.Token{}, decodeHTTPError(response.StatusCode, response.Header, body)
	}
	var payload struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if !validOpaque(payload.AccessToken, 8192) || payload.ExpiresIn < 0 || payload.ExpiresIn > int64((365*24*time.Hour)/time.Second) {
		return socialhub.Token{}, platformContractError(operation, "OAuth response is missing a valid token lifetime")
	}
	tokenType := payload.TokenType
	if tokenType == "" {
		tokenType = "Bearer"
	}
	var expiresAt time.Time
	if payload.ExpiresIn > 0 {
		expiresAt = client.Clock.Now().Add(time.Duration(payload.ExpiresIn) * time.Second)
	}
	return socialhub.Token{
		AccessToken: payload.AccessToken, TokenType: tokenType,
		ExpiresAt: expiresAt,
	}, nil
}

func validCallbackURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil && parsed.Fragment == ""
}

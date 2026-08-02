package letterboxd

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

const maxOAuthResponseBytes int64 = 1 << 20

func (c *Client) AuthorizationURL(input AuthorizationRequest) (string, error) {
	if !validCredential(c.clientID) || !validEndpoint(c.authURL) || !validRedirectURI(input.RedirectURI) ||
		!validOpaque(input.State, 2048) || !validScopes(input.Scopes) {
		return "", invalidArgument("oauth_authorize", "client ID, redirect URI, state, or scopes are invalid")
	}
	parsed, err := url.Parse(c.authURL)
	if err != nil {
		return "", platformError("oauth_authorize", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	query := parsed.Query()
	query.Set("response_type", "code")
	query.Set("client_id", c.clientID)
	query.Set("redirect_uri", input.RedirectURI)
	query.Set("state", input.State)
	if len(input.Scopes) > 0 {
		query.Set("scope", strings.Join(input.Scopes, " "))
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (c *Client) ClientCredentials(ctx context.Context, scopes []string) (socialhub.Token, error) {
	if !validScopes(scopes) {
		return socialhub.Token{}, invalidArgument("oauth_client_credentials", "scopes contain an unsupported, first-party, or duplicate value")
	}
	values := url.Values{"grant_type": {"client_credentials"}}
	if len(scopes) > 0 {
		values.Set("scope", strings.Join(scopes, " "))
	}
	return c.token(ctx, "oauth_client_credentials", values, "")
}

func (c *Client) Exchange(ctx context.Context, code, redirectURI string) (socialhub.Token, error) {
	if !validOpaque(code, maxCredentialLength) || !validRedirectURI(redirectURI) {
		return socialhub.Token{}, invalidArgument("oauth_exchange", "authorization code or redirect URI is invalid")
	}
	return c.token(ctx, "oauth_exchange", url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {redirectURI},
	}, "")
}

func (c *Client) Refresh(ctx context.Context, refreshToken string) (socialhub.Token, error) {
	if !validOpaque(refreshToken, maxCredentialLength) {
		return socialhub.Token{}, invalidArgument("oauth_refresh", "refresh token is invalid")
	}
	return c.token(ctx, "oauth_refresh", url.Values{
		"grant_type": {"refresh_token"}, "refresh_token": {refreshToken},
	}, refreshToken)
}

func (c *Client) token(ctx context.Context, operation string, values url.Values, retainedRefreshToken string) (socialhub.Token, error) {
	if !validCredential(c.clientID) || !validCredential(c.clientSecret) || c.httpClient == nil || c.clock == nil ||
		!validEndpoint(c.tokenURL) || !validUserAgent(c.userAgent) {
		return socialhub.Token{}, invalidArgument(operation, "OAuth client is incomplete")
	}
	values.Set("client_id", c.clientID)
	values.Set("client_secret", c.clientSecret)
	var payload struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
		Scope        string `json:"scope"`
	}
	if err := c.oauthForm(ctx, c.tokenURL, values, operation, &payload); err != nil {
		return socialhub.Token{}, err
	}
	if !validCredential(payload.AccessToken) || payload.ExpiresIn <= 0 ||
		payload.ExpiresIn > int64((366*24*time.Hour)/time.Second) ||
		(payload.TokenType != "" && !strings.EqualFold(payload.TokenType, "bearer")) {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	refreshToken := payload.RefreshToken
	if refreshToken == "" {
		refreshToken = retainedRefreshToken
	} else if !validCredential(refreshToken) {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return socialhub.Token{
		AccessToken: payload.AccessToken, RefreshToken: refreshToken, TokenType: "Bearer",
		ExpiresAt: c.clock.Now().Add(time.Duration(payload.ExpiresIn) * time.Second), Scopes: strings.Fields(payload.Scope),
	}, nil
}

func (c *Client) Revoke(ctx context.Context, token, tokenTypeHint string) error {
	if !validOpaque(token, maxCredentialLength) ||
		(tokenTypeHint != "access_token" && tokenTypeHint != "refresh_token") || !validEndpoint(c.revokeURL) {
		return invalidArgument("oauth_revoke", "token and a valid token type hint are required")
	}
	return c.oauthForm(ctx, c.revokeURL, url.Values{
		"token": {token}, "token_type_hint": {tokenTypeHint},
	}, "oauth_revoke", nil)
}

func (c *Client) oauthForm(ctx context.Context, endpoint string, values url.Values, operation string, output any) error {
	if c.httpClient == nil || !validCredential(c.clientID) || !validCredential(c.clientSecret) || !validUserAgent(c.userAgent) {
		return invalidArgument(operation, "OAuth client is incomplete")
	}
	values.Set("client_id", c.clientID)
	values.Set("client_secret", c.clientSecret)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("User-Agent", c.userAgent)
	response, err := c.httpClient.Do(request)
	if err != nil {
		return platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, sanitizeTransportError(err))
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxOAuthResponseBytes+1))
	if err != nil {
		return platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(body)) > maxOAuthResponseBytes {
		return platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		decoded := decodeHTTPError(response.StatusCode, response.Header, body)
		if hubError, ok := decoded.(*socialhub.Error); ok {
			hubError.Op = operation
		}
		return decoded
	}
	if output == nil || len(body) == 0 {
		return nil
	}
	if err := json.Unmarshal(body, output); err != nil {
		return platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return nil
}

func sanitizeTransportError(err error) error {
	for {
		var urlError *url.Error
		if !errors.As(err, &urlError) || urlError.Err == nil {
			return err
		}
		err = urlError.Err
	}
}

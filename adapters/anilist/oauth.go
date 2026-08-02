package anilist

import (
	"bytes"
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

func (c *Client) AuthorizationURL(input AuthorizationRequest) (string, error) {
	if err := c.requireOAuthApp("oauth_authorize", false); err != nil {
		return "", err
	}
	if !validEndpoint(c.authURL) || !validRedirectURI(input.RedirectURI) || !validState(input.State) {
		return "", invalidArgument("oauth_authorize", "redirect URI or state is invalid")
	}
	parsed, err := url.Parse(c.authURL)
	if err != nil {
		return "", platformError("oauth_authorize", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	query := parsed.Query()
	query.Set("client_id", c.clientID)
	query.Set("redirect_uri", input.RedirectURI)
	query.Set("response_type", "code")
	query.Set("state", input.State)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (c *Client) ImplicitAuthorizationURL(input ImplicitAuthorizationRequest) (string, error) {
	if err := c.requireOAuthApp("oauth_implicit_authorize", false); err != nil {
		return "", err
	}
	if !validEndpoint(c.authURL) || !validState(input.State) {
		return "", invalidArgument("oauth_implicit_authorize", "state is invalid")
	}
	parsed, err := url.Parse(c.authURL)
	if err != nil {
		return "", platformError("oauth_implicit_authorize", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	query := parsed.Query()
	query.Set("client_id", c.clientID)
	query.Set("response_type", "token")
	query.Set("state", input.State)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (c *Client) Exchange(ctx context.Context, code, redirectURI string) (socialhub.Token, error) {
	if err := c.requireOAuthApp("oauth_exchange", true); err != nil {
		return socialhub.Token{}, err
	}
	if !validOpaque(code, maxCredentialLength) || !validRedirectURI(redirectURI) || c.httpClient == nil ||
		c.clock == nil || !validEndpoint(c.tokenURL) || !validUserAgent(c.userAgent) {
		return socialhub.Token{}, invalidArgument("oauth_exchange", "authorization code, redirect URI, or OAuth client is invalid")
	}
	payload := map[string]string{
		"grant_type": "authorization_code", "client_id": c.clientID, "client_secret": c.clientSecret,
		"redirect_uri": redirectURI, "code": code,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return socialhub.Token{}, platformError("oauth_exchange", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, bytes.NewReader(encoded))
	if err != nil {
		return socialhub.Token{}, platformError("oauth_exchange", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", c.userAgent)
	response, err := c.httpClient.Do(request)
	if err != nil {
		return socialhub.Token{}, platformError("oauth_exchange", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, sanitizeTransportError(err))
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxOAuthResponseBytes+1))
	if err != nil {
		return socialhub.Token{}, platformError("oauth_exchange", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(body)) > maxOAuthResponseBytes {
		return socialhub.Token{}, platformError("oauth_exchange", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		decoded := decodeHTTPError(response.StatusCode, response.Header, body)
		if platformErr, ok := decoded.(*socialhub.Error); ok {
			platformErr.Op = "oauth_exchange"
		}
		return socialhub.Token{}, decoded
	}
	var token struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &token); err != nil {
		return socialhub.Token{}, platformError("oauth_exchange", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if !validCredential(token.AccessToken) || token.ExpiresIn <= 0 ||
		token.ExpiresIn > int64((366*24*time.Hour)/time.Second) ||
		(token.TokenType != "" && !strings.EqualFold(token.TokenType, "bearer")) {
		return socialhub.Token{}, platformError("oauth_exchange", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return socialhub.Token{
		AccessToken: token.AccessToken, TokenType: "Bearer",
		ExpiresAt: c.clock.Now().Add(time.Duration(token.ExpiresIn) * time.Second),
	}, nil
}

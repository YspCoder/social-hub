package myanimelist

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

const maxOAuthResponseBytes int64 = 1 << 20

// PKCE contains the verifier and identical plain challenge required by MAL.
type PKCE struct {
	Verifier  string
	Challenge string
}

// NewPKCE creates a cryptographically random plain PKCE pair.
func NewPKCE() (PKCE, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return PKCE{}, fmt.Errorf("myanimelist: generate PKCE: %w", err)
	}
	verifier := base64.RawURLEncoding.EncodeToString(random)
	return PKCE{Verifier: verifier, Challenge: verifier}, nil
}

func (c *Client) AuthorizationURL(input AuthorizationRequest) (string, error) {
	if !validCredential(c.clientID) || !validEndpoint(c.authURL) || !validRedirectURI(input.RedirectURI) ||
		!validOpaque(input.State, 2048) || !validPKCEValue(input.PKCE.Verifier) ||
		input.PKCE.Verifier != input.PKCE.Challenge {
		return "", invalidArgument("oauth_authorize", "client ID, redirect URI, state, and an identical plain PKCE pair are required")
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
	query.Set("code_challenge", input.PKCE.Challenge)
	query.Set("code_challenge_method", "plain")
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (c *Client) Exchange(ctx context.Context, code, redirectURI, verifier string) (socialhub.Token, error) {
	if !validOpaque(code, maxCredentialLength) || !validRedirectURI(redirectURI) || !validPKCEValue(verifier) {
		return socialhub.Token{}, invalidArgument("oauth_exchange", "authorization code, redirect URI, or PKCE verifier is invalid")
	}
	return c.token(ctx, "oauth_exchange", url.Values{
		"grant_type": {"authorization_code"}, "code": {code},
		"redirect_uri": {redirectURI}, "code_verifier": {verifier},
	}, "", true)
}

func (c *Client) Refresh(ctx context.Context, refreshToken string) (socialhub.Token, error) {
	if !validOpaque(refreshToken, maxCredentialLength) {
		return socialhub.Token{}, invalidArgument("oauth_refresh", "refresh token is invalid")
	}
	return c.token(ctx, "oauth_refresh", url.Values{
		"grant_type": {"refresh_token"}, "refresh_token": {refreshToken},
	}, refreshToken, false)
}

func (c *Client) token(ctx context.Context, operation string, values url.Values, retainedRefreshToken string, requireRefresh bool) (socialhub.Token, error) {
	if !validCredential(c.clientID) || c.httpClient == nil || c.clock == nil ||
		!validEndpoint(c.tokenURL) || !validUserAgent(c.userAgent) {
		return socialhub.Token{}, invalidArgument(operation, "OAuth client is incomplete")
	}
	values.Set("client_id", c.clientID)
	if c.clientSecret != "" {
		values.Set("client_secret", c.clientSecret)
	}
	var payload struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := c.oauthForm(ctx, values, operation, &payload); err != nil {
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
	}
	if (requireRefresh && refreshToken == "") || (refreshToken != "" && !validCredential(refreshToken)) {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return socialhub.Token{
		AccessToken: payload.AccessToken, RefreshToken: refreshToken, TokenType: "Bearer",
		ExpiresAt: c.clock.Now().Add(time.Duration(payload.ExpiresIn) * time.Second),
	}, nil
}

func (c *Client) oauthForm(ctx context.Context, values url.Values, operation string, output any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, strings.NewReader(values.Encode()))
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
		if platformErr, ok := decoded.(*socialhub.Error); ok {
			platformErr.Op = operation
		}
		return decoded
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

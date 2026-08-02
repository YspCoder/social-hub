package simkl

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
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

const (
	maxOAuthResponseBytes int64 = 1 << 20
	simklTokenLifetime          = 157680000 * time.Second
)

// PKCE contains an RFC 7636 verifier and its S256 challenge.
type PKCE struct {
	Verifier  string
	Challenge string
}

// PINAuthorization contains the values needed to display and poll Simkl's PIN flow.
type PINAuthorization struct {
	DeviceCode      string
	UserCode        string
	VerificationURI string
	ExpiresAt       time.Time
	Interval        time.Duration
}

// NewPKCE creates a cryptographically random S256 PKCE pair.
func NewPKCE() (PKCE, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return PKCE{}, fmt.Errorf("simkl: generate PKCE: %w", err)
	}
	verifier := base64.RawURLEncoding.EncodeToString(random)
	digest := sha256.Sum256([]byte(verifier))
	return PKCE{Verifier: verifier, Challenge: base64.RawURLEncoding.EncodeToString(digest[:])}, nil
}

func (c *Client) AuthorizationURL(input AuthorizationRequest) (string, error) {
	if err := c.requireSecret("oauth_authorize"); err != nil {
		return "", err
	}
	if !validRedirectURI(input.RedirectURI) || !validOpaque(input.State, 2048) || !validEndpoint(c.authURL) {
		return "", invalidArgument("oauth_authorize", "redirect URI and state are required")
	}
	return c.authorizationURL(input.RedirectURI, input.State, PKCE{})
}

func (c *Client) AuthorizationURLPKCE(input PKCEAuthorizationRequest) (string, error) {
	if err := c.requireClientID("oauth_authorize_pkce"); err != nil {
		return "", err
	}
	if (input.RedirectURI != "" && !validRedirectURI(input.RedirectURI)) || !validOpaque(input.State, 2048) ||
		!validPKCE(input.PKCE) || !validEndpoint(c.authURL) {
		return "", invalidArgument("oauth_authorize_pkce", "state, optional redirect URI, and an S256 PKCE pair are required")
	}
	return c.authorizationURL(input.RedirectURI, input.State, input.PKCE)
}

func (c *Client) authorizationURL(redirectURI, state string, pkce PKCE) (string, error) {
	parsed, err := url.Parse(c.authURL)
	if err != nil {
		return "", platformError("oauth_authorize", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	query := parsed.Query()
	query.Set("response_type", "code")
	query.Set("client_id", c.clientID)
	query.Set("state", state)
	if redirectURI != "" {
		query.Set("redirect_uri", redirectURI)
	}
	if pkce.Challenge != "" {
		query.Set("code_challenge", pkce.Challenge)
		query.Set("code_challenge_method", "S256")
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (c *Client) Exchange(ctx context.Context, code, redirectURI string) (socialhub.Token, error) {
	if err := c.requireSecret("oauth_exchange"); err != nil {
		return socialhub.Token{}, err
	}
	if !validOpaque(code, maxCredentialLength) || !validRedirectURI(redirectURI) {
		return socialhub.Token{}, invalidArgument("oauth_exchange", "authorization code and redirect URI are required")
	}
	return c.exchangeToken(ctx, "oauth_exchange", url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "client_id": {c.clientID},
		"client_secret": {c.clientSecret}, "redirect_uri": {redirectURI},
	})
}

func (c *Client) ExchangePKCE(ctx context.Context, code, verifier, redirectURI string) (socialhub.Token, error) {
	if err := c.requireClientID("oauth_exchange_pkce"); err != nil {
		return socialhub.Token{}, err
	}
	if !validOpaque(code, maxCredentialLength) || !validPKCEVerifier(verifier) ||
		(redirectURI != "" && !validRedirectURI(redirectURI)) {
		return socialhub.Token{}, invalidArgument("oauth_exchange_pkce", "authorization code, PKCE verifier, or optional redirect URI is invalid")
	}
	values := url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "client_id": {c.clientID}, "code_verifier": {verifier},
	}
	if redirectURI != "" {
		values.Set("redirect_uri", redirectURI)
	}
	return c.exchangeToken(ctx, "oauth_exchange_pkce", values)
}

func (c *Client) exchangeToken(ctx context.Context, operation string, values url.Values) (socialhub.Token, error) {
	if c.httpClient == nil || c.clock == nil || !validEndpoint(c.tokenURL) || !validUserAgent(c.userAgent) {
		return socialhub.Token{}, invalidArgument(operation, "OAuth client is incomplete")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("User-Agent", c.userAgent)
	response, err := c.httpClient.Do(request)
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, sanitizeTransportError(err))
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxOAuthResponseBytes+1))
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(body)) > maxOAuthResponseBytes {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		decoded := decodeHTTPError(response.StatusCode, response.Header, body)
		if platformErr, ok := decoded.(*socialhub.Error); ok {
			platformErr.Op = operation
		}
		return socialhub.Token{}, decoded
	}
	var payload struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Scope       string `json:"scope"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if !validCredential(payload.AccessToken) || !strings.EqualFold(payload.TokenType, "bearer") ||
		payload.Scope != "public" || payload.ExpiresIn <= 0 || payload.ExpiresIn > int64((6*365*24*time.Hour)/time.Second) {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return socialhub.Token{
		AccessToken: payload.AccessToken, TokenType: "Bearer", Scopes: []string{"public"},
		ExpiresAt: c.clock.Now().Add(time.Duration(payload.ExpiresIn) * time.Second),
	}, nil
}

func (c *Client) RequestPIN(ctx context.Context, options ...socialhub.CallOption) (*PINAuthorization, error) {
	if err := c.requireClientID("oauth_pin_request"); err != nil {
		return nil, err
	}
	var payload struct {
		Result          string `json:"result"`
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURI string `json:"verification_uri"`
		VerificationURL string `json:"verification_url"`
		ExpiresIn       int64  `json:"expires_in"`
		Interval        int64  `json:"interval"`
	}
	if _, err := requestJSON(ctx, c.api, "oauth_pin_request", http.MethodGet, "/oauth/pin", nil, nil, &payload, options...); err != nil {
		return nil, err
	}
	verificationURI := firstNonEmpty(payload.VerificationURI, payload.VerificationURL)
	if payload.Result != "OK" || !validOpaque(payload.DeviceCode, maxCredentialLength) || !validPINCode(payload.UserCode) ||
		!validEndpoint(verificationURI) || payload.ExpiresIn <= 0 || payload.ExpiresIn > int64(time.Hour/time.Second) ||
		payload.Interval <= 0 || payload.Interval > int64(time.Minute/time.Second) {
		return nil, platformError("oauth_pin_request", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &PINAuthorization{
		DeviceCode: payload.DeviceCode, UserCode: payload.UserCode, VerificationURI: verificationURI,
		ExpiresAt: c.clock.Now().Add(time.Duration(payload.ExpiresIn) * time.Second),
		Interval:  time.Duration(payload.Interval) * time.Second,
	}, nil
}

func (c *Client) PollPIN(ctx context.Context, authorization PINAuthorization, options ...socialhub.CallOption) (socialhub.Token, error) {
	if err := c.requireClientID("oauth_pin_poll"); err != nil {
		return socialhub.Token{}, err
	}
	if !validPINCode(authorization.UserCode) || authorization.ExpiresAt.IsZero() || authorization.Interval <= 0 || authorization.Interval > time.Minute {
		return socialhub.Token{}, invalidArgument("oauth_pin_poll", "PIN authorization is invalid")
	}
	if !c.clock.Now().Before(authorization.ExpiresAt) {
		return socialhub.Token{}, &socialhub.Error{
			Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction,
			Platform: "simkl", Product: productName, Op: "oauth_pin_poll", PlatformCode: "expired_token",
		}
	}
	var payload struct {
		Result      string `json:"result"`
		AccessToken string `json:"access_token"`
		Message     string `json:"message"`
		DeviceCode  string `json:"device_code"`
		UserCode    string `json:"user_code"`
	}
	if _, err := requestJSON(ctx, c.api, "oauth_pin_poll", http.MethodGet, "/oauth/pin/"+escaped(authorization.UserCode), nil, nil, &payload, options...); err != nil {
		return socialhub.Token{}, err
	}
	if payload.Result == "OK" && validCredential(payload.AccessToken) {
		return socialhub.Token{
			AccessToken: payload.AccessToken, TokenType: "Bearer", Scopes: []string{"public"},
			ExpiresAt: c.clock.Now().Add(simklTokenLifetime),
		}, nil
	}
	if payload.Result == "KO" && strings.EqualFold(payload.Message, "Authorization pending") {
		return socialhub.Token{}, &socialhub.Error{
			Code: socialhub.CodeTemporarilyUnavailable, Class: socialhub.ClassRetryable,
			Platform: "simkl", Product: productName, Op: "oauth_pin_poll", PlatformCode: "authorization_pending",
			PlatformMessage: bounded(payload.Message, 512), RetryAfter: authorization.Interval,
		}
	}
	if payload.DeviceCode != "" || payload.UserCode != "" {
		return socialhub.Token{}, &socialhub.Error{
			Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction,
			Platform: "simkl", Product: productName, Op: "oauth_pin_poll", PlatformCode: "pin_code_gone",
			PlatformMessage: "the PIN was consumed, expired, or unknown; request a new code",
		}
	}
	return socialhub.Token{}, platformError("oauth_pin_poll", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
}

func validPKCE(input PKCE) bool {
	if !validPKCEVerifier(input.Verifier) {
		return false
	}
	digest := sha256.Sum256([]byte(input.Verifier))
	return input.Challenge == base64.RawURLEncoding.EncodeToString(digest[:])
}

func validPKCEVerifier(value string) bool {
	if len(value) < 43 || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if !(character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || strings.ContainsRune("-._~", character)) {
			return false
		}
	}
	return true
}

func validPINCode(value string) bool {
	if len(value) < 4 || len(value) > 16 {
		return false
	}
	for _, character := range value {
		if !(character >= 'A' && character <= 'Z' || character >= '0' && character <= '9') {
			return false
		}
	}
	return true
}

func sanitizeTransportError(err error) error {
	for err != nil {
		var urlError *url.Error
		if !errors.As(err, &urlError) || urlError.Err == nil {
			return err
		}
		err = urlError.Err
	}
	return nil
}

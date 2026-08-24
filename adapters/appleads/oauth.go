package appleads

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
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
	clientSecretTTL             = 5 * time.Minute
)

// OAuthClient implements Apple's OAuth 2 client-credentials flow with an
// ES256 client-secret JWT.
type OAuthClient struct {
	ClientID   string
	TeamID     string
	KeyID      string
	PrivateKey *ecdsa.PrivateKey
	TokenURL   string
	HTTPClient *http.Client
	Clock      socialhub.Clock
}

func (client *OAuthClient) ClientSecret() (string, error) {
	if !validOpaque(client.ClientID, 1024) || !validOpaque(client.TeamID, 1024) || !validOpaque(client.KeyID, 1024) ||
		client.PrivateKey == nil || client.PrivateKey.Curve != elliptic.P256() || client.Clock == nil {
		return "", invalidArgument("oauth_client_secret", "OAuth credentials are incomplete or the private key is not P-256")
	}
	now := client.Clock.Now().UTC()
	header := struct {
		Algorithm string `json:"alg"`
		KeyID     string `json:"kid"`
	}{Algorithm: "ES256", KeyID: client.KeyID}
	payload := struct {
		Issuer   string `json:"iss"`
		IssuedAt int64  `json:"iat"`
		Expires  int64  `json:"exp"`
		Audience string `json:"aud"`
		Subject  string `json:"sub"`
	}{
		Issuer: client.TeamID, IssuedAt: now.Unix(), Expires: now.Add(clientSecretTTL).Unix(),
		Audience: "https://appleid.apple.com", Subject: client.ClientID,
	}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", platformError("oauth_client_secret", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", platformError("oauth_client_secret", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	encodedHeader := base64.RawURLEncoding.EncodeToString(headerJSON)
	encodedPayload := base64.RawURLEncoding.EncodeToString(payloadJSON)
	signingInput := encodedHeader + "." + encodedPayload
	digest := sha256.Sum256([]byte(signingInput))
	r, s, err := ecdsa.Sign(rand.Reader, client.PrivateKey, digest[:])
	if err != nil {
		return "", platformError("oauth_client_secret", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	signature := make([]byte, 64)
	r.FillBytes(signature[:32])
	s.FillBytes(signature[32:])
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func (client *OAuthClient) Token(ctx context.Context) (socialhub.Token, error) {
	if client.HTTPClient == nil || client.Clock == nil || !validEndpoint(client.TokenURL) {
		return socialhub.Token{}, invalidArgument("oauth_token", "OAuth client or token endpoint is incomplete")
	}
	secret, err := client.ClientSecret()
	if err != nil {
		return socialhub.Token{}, err
	}
	values := url.Values{
		"grant_type": {"client_credentials"}, "client_id": {client.ClientID},
		"client_secret": {secret}, "scope": {oauthScope},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.TokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return socialhub.Token{}, platformError("oauth_token", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := client.HTTPClient.Do(request)
	if err != nil {
		return socialhub.Token{}, platformError("oauth_token", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxOAuthResponseBytes+1))
	if err != nil {
		return socialhub.Token{}, platformError("oauth_token", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(body)) > maxOAuthResponseBytes {
		return socialhub.Token{}, platformError("oauth_token", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("OAuth response exceeded size limit"))
	}
	var payload struct {
		AccessToken      string `json:"access_token"`
		TokenType        string `json:"token_type"`
		ExpiresIn        int64  `json:"expires_in"`
		Scope            string `json:"scope"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	decodeErr := json.Unmarshal(body, &payload)
	if response.StatusCode < 200 || response.StatusCode >= 300 || payload.Error != "" {
		if decodeErr != nil {
			return socialhub.Token{}, decodeHTTPError(response.StatusCode, response.Header, body)
		}
		return socialhub.Token{}, oauthError(response.StatusCode, response.Header, payload.Error, payload.ErrorDescription)
	}
	if decodeErr != nil || !validOpaque(payload.AccessToken, 16384) || payload.ExpiresIn <= 0 || payload.ExpiresIn > int64((2*time.Hour)/time.Second) ||
		payload.Scope != oauthScope || !strings.EqualFold(payload.TokenType, "bearer") {
		return socialhub.Token{}, platformError("oauth_token", socialhub.CodePlatformError, socialhub.ClassPermanent, decodeErr)
	}
	return socialhub.Token{
		AccessToken: payload.AccessToken, TokenType: "Bearer", Scopes: []string{oauthScope},
		ExpiresAt: client.Clock.Now().Add(time.Duration(payload.ExpiresIn) * time.Second),
	}, nil
}

func oauthError(status int, header http.Header, platformCode, message string) error {
	code, class := classifyError(status)
	switch platformCode {
	case "invalid_client", "unauthorized_client", "access_denied":
		code, class = socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "invalid_request", "unsupported_grant_type", "invalid_scope":
		code, class = socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case "temporarily_unavailable", "server_error":
		code, class = socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: "oauth_token", HTTPStatus: status,
		PlatformCode: boundedMessage(redactSensitive(platformCode), 256), PlatformMessage: boundedMessage(redactSensitive(message), 512),
		RequestID:  boundedMessage(firstNonEmpty(header.Get("x-request-id"), header.Get("request-id")), 256),
		RetryAfter: parseRetryAfter(header.Get("Retry-After")),
	}
}

func parsePrivateKey(value []byte) (*ecdsa.PrivateKey, error) {
	block, rest := pem.Decode(value)
	if block == nil || len(strings.TrimSpace(string(rest))) != 0 {
		return nil, fmt.Errorf("invalid PEM")
	}
	if parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		key, ok := parsed.(*ecdsa.PrivateKey)
		if !ok || key.Curve != elliptic.P256() {
			return nil, fmt.Errorf("private key is not P-256 ECDSA")
		}
		return key, nil
	}
	key, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil || key.Curve != elliptic.P256() {
		return nil, fmt.Errorf("private key is not P-256 ECDSA")
	}
	return key, nil
}

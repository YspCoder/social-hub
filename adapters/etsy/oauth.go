package etsy

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
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

type PKCEPair struct {
	Verifier  string
	Challenge string
}

// GeneratePKCE creates the RFC 7636 S256 verifier and challenge required by Etsy.
func GeneratePKCE() (PKCEPair, error) {
	random := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, random); err != nil {
		return PKCEPair{}, platformError("oauth_pkce", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	verifier := base64.RawURLEncoding.EncodeToString(random)
	digest := sha256.Sum256([]byte(verifier))
	return PKCEPair{Verifier: verifier, Challenge: base64.RawURLEncoding.EncodeToString(digest[:])}, nil
}

type AuthorizationRequest struct {
	RedirectURI   string
	State         string
	Scopes        []string
	CodeChallenge string
}

// OAuthClient implements Etsy's OAuth 2.0 authorization-code grant with PKCE
// and its refresh-token grant.
type OAuthClient struct {
	ClientID   string
	AuthURL    string
	TokenURL   string
	HTTPClient *http.Client
	Clock      socialhub.Clock
}

func (client *OAuthClient) AuthorizationURL(input AuthorizationRequest) (string, error) {
	if !validOpaque(client.ClientID, 1024) || !validEndpoint(client.AuthURL) ||
		!validCallbackURL(input.RedirectURI) || !validOpaque(input.State, 1024) ||
		!validScopeSet(input.Scopes) || !validPKCEValue(input.CodeChallenge) {
		return "", invalidArgument("oauth_authorize", "client ID, HTTPS redirect URI, one-time state, scopes, and PKCE challenge are required")
	}
	parsed, _ := url.Parse(client.AuthURL)
	query := parsed.Query()
	query.Set("response_type", "code")
	query.Set("client_id", client.ClientID)
	query.Set("redirect_uri", input.RedirectURI)
	query.Set("scope", strings.Join(input.Scopes, " "))
	query.Set("state", input.State)
	query.Set("code_challenge", input.CodeChallenge)
	query.Set("code_challenge_method", "S256")
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (client *OAuthClient) Exchange(ctx context.Context, code, redirectURI, codeVerifier string) (socialhub.Token, error) {
	if !validOpaque(code, 4096) || !validCallbackURL(redirectURI) || !validPKCEValue(codeVerifier) {
		return socialhub.Token{}, invalidArgument("oauth_exchange", "authorization code, HTTPS redirect URI, and PKCE verifier are required")
	}
	return client.token(ctx, "oauth_exchange", url.Values{
		"grant_type": {"authorization_code"}, "client_id": {client.ClientID},
		"redirect_uri": {redirectURI}, "code": {code}, "code_verifier": {codeVerifier},
	}, code, codeVerifier)
}

func (client *OAuthClient) Refresh(ctx context.Context, refreshToken string) (socialhub.Token, error) {
	if !validOpaque(refreshToken, 16_384) {
		return socialhub.Token{}, invalidArgument("oauth_refresh", "refresh token is required")
	}
	return client.token(ctx, "oauth_refresh", url.Values{
		"grant_type": {"refresh_token"}, "client_id": {client.ClientID}, "refresh_token": {refreshToken},
	}, refreshToken)
}

func (client *OAuthClient) token(ctx context.Context, operation string, values url.Values, secrets ...string) (socialhub.Token, error) {
	if !validOpaque(client.ClientID, 1024) || !validEndpoint(client.TokenURL) || client.HTTPClient == nil || client.Clock == nil {
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
		return socialhub.Token{}, platformContractError(operation, "Etsy OAuth response exceeded 1 MiB")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return socialhub.Token{}, withOperation(decodeHTTPError(response.StatusCode, response.Header, body, client.Clock.Now(), secrets...), operation)
	}
	if !validJSONContentType(response.Header.Get("Content-Type")) {
		return socialhub.Token{}, platformContractError(operation, "Etsy returned a non-JSON OAuth success response")
	}
	var payload struct {
		AccessToken      string `json:"access_token"`
		TokenType        string `json:"token_type"`
		ExpiresIn        int64  `json:"expires_in"`
		RefreshToken     string `json:"refresh_token"`
		Scope            string `json:"scope"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if payload.Error != "" {
		return socialhub.Token{}, oauthResponseError(operation, response.StatusCode, response.Header, payload.Error, payload.ErrorDescription, body, client.Clock.Now(), secrets...)
	}
	scopes := strings.Fields(payload.Scope)
	if !validOpaque(payload.AccessToken, 16_384) || !validOpaque(payload.RefreshToken, 16_384) ||
		payload.ExpiresIn <= 0 || payload.ExpiresIn > int64((24*time.Hour)/time.Second) ||
		!strings.EqualFold(payload.TokenType, "bearer") || !validScopeSet(scopes) {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("invalid OAuth token response fields"))
	}
	return socialhub.Token{
		AccessToken: payload.AccessToken, RefreshToken: payload.RefreshToken, TokenType: "Bearer",
		ExpiresAt: client.Clock.Now().Add(time.Duration(payload.ExpiresIn) * time.Second), Scopes: scopes,
	}, nil
}

func validPKCEValue(value string) bool {
	if len(value) < 43 || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if (character < 'A' || character > 'Z') && (character < 'a' || character > 'z') &&
			(character < '0' || character > '9') && !strings.ContainsRune("-._~", character) {
			return false
		}
	}
	return true
}

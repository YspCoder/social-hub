package mixcloud

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

const maxOAuthResponseBytes int64 = 1 << 20

// OAuthClient implements Mixcloud's browser-only OAuth 2 authorization-code
// flow. The official token endpoint uses query parameters and returns a
// form-encoded access_token without a refresh token or expiry.
type OAuthClient struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	UserAgent    string
	HTTPClient   *http.Client
}

// AuthorizationURL builds the Mixcloud browser authorization URL. RedirectURI
// may be empty for the documented out-of-band flow.
func (client *OAuthClient) AuthorizationURL(redirectURI, state string) (string, error) {
	if !client.valid() || !validOAuthRedirect(redirectURI) || !validOpaque(state, 1024) {
		return "", invalidArgument("oauth_authorize", "OAuth client, redirect URI, or state is invalid")
	}
	parsed, _ := url.Parse(client.AuthURL)
	query := parsed.Query()
	query.Set("client_id", client.ClientID)
	query.Set("state", state)
	if redirectURI != "" {
		query.Set("redirect_uri", redirectURI)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

// Exchange trades an authorization code for Mixcloud's non-expiring token.
// RedirectURI must exactly match the value used for authorization, including
// being empty for the out-of-band flow.
func (client *OAuthClient) Exchange(ctx context.Context, code, redirectURI string) (socialhub.Token, error) {
	if !client.valid() || !validOpaque(code, maxOpaqueLength) || !validOAuthRedirect(redirectURI) {
		return socialhub.Token{}, invalidArgument("oauth_exchange", "OAuth client, code, or redirect URI is invalid")
	}
	parsed, _ := url.Parse(client.TokenURL)
	query := parsed.Query()
	query.Set("client_id", client.ClientID)
	query.Set("client_secret", client.ClientSecret)
	query.Set("code", code)
	query.Set("redirect_uri", redirectURI)
	parsed.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return socialhub.Token{}, platformError("oauth_exchange", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/x-www-form-urlencoded, application/json")
	request.Header.Set("User-Agent", client.UserAgent)
	response, err := client.HTTPClient.Do(request)
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
		return socialhub.Token{}, decodeOAuthError(response.StatusCode, response.Header, body)
	}
	values, parseErr := url.ParseQuery(strings.TrimSpace(string(body)))
	accessToken := values.Get("access_token")
	if accessToken == "" {
		var payload struct {
			AccessToken string `json:"access_token"`
		}
		if err := json.Unmarshal(body, &payload); err == nil {
			accessToken = payload.AccessToken
		}
	}
	if parseErr != nil || !validOpaque(accessToken, maxOpaqueLength) {
		return socialhub.Token{}, platformError("oauth_exchange", socialhub.CodePlatformError, socialhub.ClassPermanent, parseErr)
	}
	return socialhub.Token{AccessToken: accessToken, TokenType: "OAuth"}, nil
}

func (client *OAuthClient) valid() bool {
	return validOpaque(client.ClientID, 512) && validOpaque(client.ClientSecret, maxOpaqueLength) &&
		validEndpoint(client.AuthURL) && validEndpoint(client.TokenURL) && validUserAgent(client.UserAgent) && client.HTTPClient != nil
}

func decodeOAuthError(status int, header http.Header, body []byte) error {
	errorValue := decodeHTTPError(status, header, body)
	var platformErr *socialhub.Error
	if !errors.As(errorValue, &platformErr) {
		return errorValue
	}
	values, _ := url.ParseQuery(strings.TrimSpace(string(body)))
	if platformErr.PlatformCode == "" {
		platformErr.PlatformCode = boundedMessage(values.Get("error"), 128)
	}
	if platformErr.PlatformMessage == "" {
		platformErr.PlatformMessage = boundedMessage(values.Get("error_description"), 512)
	}
	switch platformErr.PlatformCode {
	case "invalid_client", "invalid_grant":
		platformErr.Code, platformErr.Class = socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "access_denied":
		platformErr.Code, platformErr.Class = socialhub.CodePermissionDenied, socialhub.ClassUserAction
	}
	platformErr.Op = "oauth_exchange"
	return platformErr
}

func sanitizeTransportError(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}

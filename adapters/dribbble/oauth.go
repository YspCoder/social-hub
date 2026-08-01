package dribbble

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

const maxOAuthResponseBytes int64 = 1 << 20

// OAuthClient implements Dribbble's web authorization-code flow. Dribbble
// does not document refresh tokens or non-web grants.
type OAuthClient struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	HTTPClient   *http.Client
}

func (client *OAuthClient) AuthorizationURL(redirectURI, state string, scopes []string) (string, error) {
	if !validOpaque(client.ClientID, 512) || !validCallbackURL(redirectURI) || !validOpaque(state, 1024) || len(scopes) == 0 {
		return "", invalidArgument("oauth_authorize", "client ID, callback URL, state, and scopes are required")
	}
	for _, scope := range scopes {
		if scope != "public" && scope != "upload" {
			return "", invalidArgument("oauth_authorize", "scope must be public or upload")
		}
	}
	parsed, err := url.Parse(client.AuthURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", invalidArgument("oauth_authorize", "authorization URL is invalid")
	}
	query := parsed.Query()
	query.Set("client_id", client.ClientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("scope", strings.Join(scopes, " "))
	query.Set("state", state)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (client *OAuthClient) Exchange(ctx context.Context, code, redirectURI string) (socialhub.Token, error) {
	if !validOpaque(code, 4096) || !validCallbackURL(redirectURI) {
		return socialhub.Token{}, invalidArgument("oauth_exchange", "authorization code and callback URL are required")
	}
	if !validOpaque(client.ClientID, 512) || !validOpaque(client.ClientSecret, 4096) || client.HTTPClient == nil {
		return socialhub.Token{}, invalidArgument("oauth_exchange", "OAuth client is incomplete")
	}
	values := url.Values{
		"client_id": {client.ClientID}, "client_secret": {client.ClientSecret},
		"code": {code}, "redirect_uri": {redirectURI},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.TokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return socialhub.Token{}, platformError("oauth_exchange", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := client.HTTPClient.Do(request)
	if err != nil {
		return socialhub.Token{}, platformError("oauth_exchange", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxOAuthResponseBytes+1))
	if err != nil {
		return socialhub.Token{}, platformError("oauth_exchange", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(body)) > maxOAuthResponseBytes {
		return socialhub.Token{}, platformError("oauth_exchange", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("OAuth response exceeded size limit"))
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return socialhub.Token{}, decodeHTTPError(response.StatusCode, response.Header, body)
	}
	var payload struct {
		AccessToken      string `json:"access_token"`
		TokenType        string `json:"token_type"`
		Scope            string `json:"scope"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return socialhub.Token{}, platformError("oauth_exchange", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if payload.Error != "" {
		return socialhub.Token{}, &socialhub.Error{
			Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction, Platform: "dribbble", Product: productName,
			Op: "oauth_exchange", PlatformCode: payload.Error, PlatformMessage: boundedMessage(payload.ErrorDescription, 512),
		}
	}
	if !validOpaque(payload.AccessToken, 4096) {
		return socialhub.Token{}, platformError("oauth_exchange", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	tokenType := payload.TokenType
	if tokenType == "" || strings.EqualFold(tokenType, "bearer") {
		tokenType = "Bearer"
	}
	return socialhub.Token{AccessToken: payload.AccessToken, TokenType: tokenType, Scopes: strings.Fields(payload.Scope)}, nil
}

func validCallbackURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme != "" && parsed.Host != "" && parsed.User == nil && parsed.Fragment == ""
}

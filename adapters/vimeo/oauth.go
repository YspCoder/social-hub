package vimeo

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

const maxOAuthResponseSize int64 = 1 << 20

// OAuthClient implements Vimeo's authorization-code and client-credentials grants.
type OAuthClient struct {
	ClientID       string
	ClientSecret   string
	AuthURL        string
	TokenURL       string
	ClientTokenURL string
	HTTPClient     *http.Client
	Clock          socialhub.Clock
}

func (c *OAuthClient) AuthorizationURL(redirectURI, state string, scopes []string) (string, error) {
	if strings.TrimSpace(c.ClientID) == "" || !validRedirectURI(redirectURI) || strings.TrimSpace(state) == "" || !validOAuthScopes(scopes) {
		return "", invalidArgument("oauth_authorize", "client ID, valid redirect URI, state, and scopes are required")
	}
	parsed, err := url.Parse(c.AuthURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", invalidArgument("oauth_authorize", "authorization URL is invalid")
	}
	query := parsed.Query()
	query.Set("response_type", "code")
	query.Set("client_id", c.ClientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("scope", strings.Join(scopes, " "))
	query.Set("state", state)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (c *OAuthClient) Exchange(ctx context.Context, code, redirectURI string) (socialhub.Token, error) {
	if strings.TrimSpace(code) == "" || !validRedirectURI(redirectURI) {
		return socialhub.Token{}, invalidArgument("oauth_exchange", "code and valid redirect URI are required")
	}
	return c.token(ctx, c.TokenURL, url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {redirectURI},
	}, "oauth_exchange")
}

func (c *OAuthClient) ClientCredentials(ctx context.Context, scopes []string) (socialhub.Token, error) {
	if !validOAuthScopes(scopes) {
		return socialhub.Token{}, invalidArgument("oauth_client_credentials", "valid scopes are required")
	}
	return c.token(ctx, c.ClientTokenURL, url.Values{
		"grant_type": {"client_credentials"}, "scope": {strings.Join(scopes, " ")},
	}, "oauth_client_credentials")
}

func (c *OAuthClient) token(ctx context.Context, endpoint string, values url.Values, operation string) (socialhub.Token, error) {
	if strings.TrimSpace(c.ClientID) == "" || strings.TrimSpace(c.ClientSecret) == "" || c.HTTPClient == nil || c.Clock == nil || !validEndpoint(endpoint) {
		return socialhub.Token{}, invalidArgument(operation, "OAuth client is incomplete")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.SetBasicAuth(c.ClientID, c.ClientSecret)
	request.Header.Set("Accept", vimeoAccept)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, sanitizeUploadError(err))
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxOAuthResponseSize+1))
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(body)) > maxOAuthResponseSize {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var oauthError struct {
			Error       string `json:"error"`
			Description string `json:"error_description"`
		}
		_ = json.Unmarshal(body, &oauthError)
		code := socialhub.CodeUnauthenticated
		class := socialhub.ClassUserAction
		if oauthError.Error == "invalid_request" || oauthError.Error == "invalid_scope" {
			code, class = socialhub.CodeInvalidArgument, socialhub.ClassPermanent
		}
		return socialhub.Token{}, &socialhub.Error{
			Code: code, Class: class, Platform: "vimeo", Product: productName, Op: operation,
			HTTPStatus: response.StatusCode, PlatformCode: boundedMessage(oauthError.Error, 128),
			PlatformMessage: boundedMessage(oauthError.Description, 512),
		}
	}
	var payload struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Scope       string `json:"scope"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || strings.TrimSpace(payload.AccessToken) == "" {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if payload.ExpiresIn < 0 || payload.ExpiresIn > int64((365*24*time.Hour)/time.Second) {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	var expiresAt time.Time
	if payload.ExpiresIn > 0 {
		expiresAt = c.Clock.Now().Add(time.Duration(payload.ExpiresIn) * time.Second)
	}
	tokenType := payload.TokenType
	if tokenType == "" || strings.EqualFold(tokenType, "bearer") {
		tokenType = "Bearer"
	}
	return socialhub.Token{AccessToken: payload.AccessToken, TokenType: tokenType, ExpiresAt: expiresAt, Scopes: strings.Fields(payload.Scope)}, nil
}

func validRedirectURI(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil && parsed.Fragment == ""
}

func validOAuthScopes(scopes []string) bool {
	if len(scopes) == 0 {
		return false
	}
	seen := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		switch scope {
		case "public", "private", "create", "edit", "delete", "interact", "stats", "upload", "video_files", "purchased":
		default:
			return false
		}
		if _, exists := seen[scope]; exists {
			return false
		}
		seen[scope] = struct{}{}
	}
	return true
}

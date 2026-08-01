package wordpresscom

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

const maxOAuthResponseBytes int64 = 1 << 20

var oauthScopes = map[string]struct{}{
	"users": {}, "sites": {}, "posts": {}, "comments": {}, "taxonomy": {}, "follow": {},
	"sharing": {}, "freshly-pressed": {}, "notifications": {}, "insights": {}, "read": {},
	"stats": {}, "media": {}, "menus": {}, "batch": {}, "videos": {}, "global": {}, "auth": {},
}

// OAuthToken preserves the site identity returned with a WordPress.com token.
type OAuthToken struct {
	Token   socialhub.Token
	BlogID  string
	BlogURL string
}

// OAuthClient implements WordPress.com's OAuth2 authorization-code flow.
// WordPress.com does not document refresh-token issuance for this flow.
type OAuthClient struct {
	ClientID     string
	ClientSecret string
	Site         string
	AuthURL      string
	TokenURL     string
	HTTPClient   *http.Client
}

func (client *OAuthClient) AuthorizationURL(redirectURI, state string, scopes []string) (string, error) {
	if !validOpaque(client.ClientID, 512) || !validSite(client.Site) || !validCallbackURL(redirectURI) || !validOpaque(state, 1024) || !validEndpoint(client.AuthURL) {
		return "", invalidArgument("oauth_authorize", "client ID, site, redirect URI, state, or authorization endpoint is invalid")
	}
	for _, scope := range scopes {
		if _, found := oauthScopes[scope]; !found {
			return "", invalidArgument("oauth_authorize", "scope is not documented by WordPress.com")
		}
	}
	parsed, _ := url.Parse(client.AuthURL)
	query := parsed.Query()
	query.Set("client_id", client.ClientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("response_type", "code")
	query.Set("blog", client.Site)
	query.Set("state", state)
	if len(scopes) > 0 {
		query.Set("scope", strings.Join(scopes, " "))
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (client *OAuthClient) Exchange(ctx context.Context, code, redirectURI string) (OAuthToken, error) {
	if !validOpaque(code, 4096) || !validCallbackURL(redirectURI) {
		return OAuthToken{}, invalidArgument("oauth_exchange", "authorization code and redirect URI are required")
	}
	if !validOpaque(client.ClientID, 512) || !validOpaque(client.ClientSecret, 4096) || !validSite(client.Site) || client.HTTPClient == nil || !validEndpoint(client.TokenURL) {
		return OAuthToken{}, invalidArgument("oauth_exchange", "OAuth client is incomplete")
	}
	values := url.Values{
		"client_id": {client.ClientID}, "client_secret": {client.ClientSecret}, "code": {code},
		"grant_type": {"authorization_code"}, "redirect_uri": {redirectURI},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.TokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return OAuthToken{}, platformError("oauth_exchange", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := client.HTTPClient.Do(request)
	if err != nil {
		return OAuthToken{}, platformError("oauth_exchange", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxOAuthResponseBytes+1))
	if err != nil {
		return OAuthToken{}, platformError("oauth_exchange", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(body)) > maxOAuthResponseBytes {
		return OAuthToken{}, platformError("oauth_exchange", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("OAuth response exceeded size limit"))
	}
	var payload struct {
		AccessToken      string          `json:"access_token"`
		BlogID           json.RawMessage `json:"blog_id"`
		BlogURL          string          `json:"blog_url"`
		TokenType        string          `json:"token_type"`
		Scope            string          `json:"scope"`
		Error            string          `json:"error"`
		ErrorDescription string          `json:"error_description"`
		Message          string          `json:"message"`
	}
	decodeErr := json.Unmarshal(body, &payload)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if decodeErr != nil {
			return OAuthToken{}, decodeHTTPError(response.StatusCode, response.Header, body)
		}
		return OAuthToken{}, oauthResponseError(response.StatusCode, response.Header, payload.Error, firstNonEmpty(payload.ErrorDescription, payload.Message))
	}
	if decodeErr != nil {
		return OAuthToken{}, platformError("oauth_exchange", socialhub.CodePlatformError, socialhub.ClassPermanent, decodeErr)
	}
	if payload.Error != "" {
		return OAuthToken{}, oauthResponseError(response.StatusCode, response.Header, payload.Error, firstNonEmpty(payload.ErrorDescription, payload.Message))
	}
	blogID := parseBlogID(payload.BlogID)
	if !validOpaque(payload.AccessToken, 4096) || !validID(blogID) || !validBlogURL(payload.BlogURL) {
		return OAuthToken{}, platformError("oauth_exchange", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if validID(client.Site) && blogID != client.Site {
		return OAuthToken{}, platformError("oauth_exchange", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("OAuth response site does not match configured site"))
	}
	tokenType := payload.TokenType
	if tokenType == "" || strings.EqualFold(tokenType, "bearer") {
		tokenType = "Bearer"
	}
	return OAuthToken{
		Token:  socialhub.Token{AccessToken: payload.AccessToken, TokenType: tokenType, Scopes: splitOAuthScopes(payload.Scope)},
		BlogID: blogID, BlogURL: payload.BlogURL,
	}, nil
}

func oauthResponseError(status int, header http.Header, code, message string) error {
	errorCode, class := classifyError(status)
	switch code {
	case "invalid_client", "invalid_grant", "unauthorized_client", "access_denied":
		errorCode, class = socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "invalid_request", "unsupported_grant_type", "unsupported_response_type", "invalid_scope":
		errorCode = socialhub.CodeInvalidArgument
	case "temporarily_unavailable", "server_error":
		errorCode, class = socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	default:
		if status == http.StatusTooManyRequests {
			errorCode, class = socialhub.CodeRateLimited, socialhub.ClassRetryable
		} else if status >= 500 {
			errorCode, class = socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
		} else if status == http.StatusUnauthorized {
			errorCode, class = socialhub.CodeUnauthenticated, socialhub.ClassUserAction
		}
	}
	return &socialhub.Error{
		Code: errorCode, Class: class, Platform: "wordpress.com", Product: productName, Op: "oauth_exchange",
		HTTPStatus: status, PlatformCode: boundedMessage(code, 256), PlatformMessage: boundedMessage(message, 512),
		RequestID:  boundedMessage(firstNonEmpty(header.Get("X-Request-Id"), header.Get("X-Automattic-Request-Id"), header.Get("X-Trace-Id")), 256),
		RetryAfter: parseRetryAfter(header.Get("Retry-After")),
	}
}

func parseBlogID(raw json.RawMessage) string {
	var value string
	if json.Unmarshal(raw, &value) == nil && validID(value) {
		return value
	}
	var number json.Number
	if json.Unmarshal(raw, &number) == nil {
		value = number.String()
		if parsed, err := strconv.ParseInt(value, 10, 64); err == nil && parsed > 0 {
			return value
		}
	}
	return ""
}

func splitOAuthScopes(value string) []string {
	return strings.FieldsFunc(value, func(character rune) bool { return character == ' ' || character == ',' })
}

func validCallbackURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil && parsed.Fragment == ""
}

func validBlogURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil
}

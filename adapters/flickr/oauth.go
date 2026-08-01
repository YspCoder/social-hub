package flickr

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/dghubble/oauth1"

	"social-hub/pkg/socialhub"
)

const maxOAuthResponseSize int64 = 1 << 20

// OAuthClient implements Flickr's OAuth 1.0a authorization flow.
type OAuthClient struct {
	ConsumerKey     string
	ConsumerSecret  string
	RequestTokenURL string
	AuthorizeURL    string
	AccessTokenURL  string
	HTTPClient      *http.Client
	noncer          oauth1.Noncer
}

// OAuthRequestToken is the temporary credential returned before user approval.
type OAuthRequestToken struct {
	Token            string
	Secret           string
	Permission       string
	AuthorizationURL string
}

// OAuthAccessToken is Flickr's non-expiring token credential pair.
type OAuthAccessToken struct {
	Token      string
	Secret     string
	UserID     string
	Username   string
	FullName   string
	Permission string
}

// BeginAuthorization obtains temporary credentials and builds the approval URL.
func (c *OAuthClient) BeginAuthorization(ctx context.Context, callbackURL, permission string) (*OAuthRequestToken, error) {
	if !c.valid() || !validCallbackURL(callbackURL) || permissionRank(permission) == 0 {
		return nil, invalidArgument("oauth_begin", "complete OAuth client, valid callback URL, and read/write/delete permission are required")
	}
	config, capture := c.config(ctx, callbackURL)
	token, secret, err := config.RequestToken()
	if err != nil {
		return nil, c.oauthError("oauth_begin", capture, err)
	}
	authorizationURL, err := config.AuthorizationURL(token)
	if err != nil {
		return nil, platformError("oauth_begin", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	query := authorizationURL.Query()
	query.Set("perms", permission)
	authorizationURL.RawQuery = query.Encode()
	return &OAuthRequestToken{Token: token, Secret: secret, Permission: permission, AuthorizationURL: authorizationURL.String()}, nil
}

// Exchange trades approved temporary credentials for a durable token pair.
// Flickr does not issue refresh tokens or publish an expiry for this token.
func (c *OAuthClient) Exchange(ctx context.Context, requestToken OAuthRequestToken, verifier string) (*OAuthAccessToken, error) {
	if !c.valid() || !validOAuthCredential(requestToken.Token) || !validOAuthCredential(requestToken.Secret) || !validOAuthCredential(verifier) || permissionRank(requestToken.Permission) == 0 {
		return nil, invalidArgument("oauth_exchange", "request token, request secret, verifier, and permission are required")
	}
	config, capture := c.config(ctx, "")
	token, secret, err := config.AccessToken(requestToken.Token, requestToken.Secret, verifier)
	if err != nil {
		return nil, c.oauthError("oauth_exchange", capture, err)
	}
	values, err := url.ParseQuery(strings.TrimSpace(string(capture.body)))
	if err != nil {
		return nil, platformError("oauth_exchange", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	userID, username := values.Get("user_nsid"), values.Get("username")
	if !validResourceID(userID) || strings.TrimSpace(username) == "" {
		return nil, platformError("oauth_exchange", socialhub.CodePlatformError, socialhub.ClassPermanent, errors.New("Flickr access token response is missing user identity"))
	}
	return &OAuthAccessToken{
		Token: token, Secret: secret, UserID: userID, Username: username,
		FullName: values.Get("fullname"), Permission: requestToken.Permission,
	}, nil
}

func (c *OAuthClient) valid() bool {
	return strings.TrimSpace(c.ConsumerKey) != "" && strings.TrimSpace(c.ConsumerSecret) != "" && c.HTTPClient != nil &&
		validEndpoint(c.RequestTokenURL) && validEndpoint(c.AuthorizeURL) && validEndpoint(c.AccessTokenURL)
}

func (c *OAuthClient) config(ctx context.Context, callbackURL string) (*oauth1.Config, *capturedResponseTransport) {
	base := c.HTTPClient.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	capture := &capturedResponseTransport{ctx: ctx, base: base, maximum: maxOAuthResponseSize}
	httpClient := &http.Client{Transport: capture, Timeout: c.HTTPClient.Timeout, CheckRedirect: rejectRedirect}
	config := &oauth1.Config{
		ConsumerKey: c.ConsumerKey, ConsumerSecret: c.ConsumerSecret, CallbackURL: callbackURL,
		Endpoint:   oauth1.Endpoint{RequestTokenURL: c.RequestTokenURL, AuthorizeURL: c.AuthorizeURL, AccessTokenURL: c.AccessTokenURL},
		HTTPClient: httpClient, Noncer: c.noncer,
	}
	return config, capture
}

func (c *OAuthClient) oauthError(operation string, capture *capturedResponseTransport, cause error) error {
	if capture.status != 0 && (capture.status < 200 || capture.status >= 300) {
		values, _ := url.ParseQuery(strings.TrimSpace(string(capture.body)))
		problem := firstNonEmpty(values.Get("oauth_problem"), values.Get("error"))
		message := firstNonEmpty(values.Get("oauth_problem_advice"), values.Get("error_description"), cause.Error())
		code, class := classifyError(capture.status, 0)
		if problem == "permission_denied" || problem == "token_rejected" || problem == "signature_invalid" {
			code, class = socialhub.CodeUnauthenticated, socialhub.ClassUserAction
		}
		return &socialhub.Error{
			Code: code, Class: class, Platform: "flickr", Product: productName, Op: operation,
			HTTPStatus: capture.status, PlatformCode: boundedMessage(problem, 128), PlatformMessage: boundedMessage(message, 512),
			RequestID: boundedMessage(firstNonEmpty(capture.header.Get("X-Request-ID"), capture.header.Get("X-Correlation-ID")), 512),
		}
	}
	return platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, cause)
}

func validOAuthCredential(value string) bool {
	return strings.TrimSpace(value) == value && value != "" && len(value) <= 4096 && !strings.ContainsAny(value, "\r\n")
}

func validCallbackURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil && parsed.Fragment == ""
}

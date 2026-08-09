package ads

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/dghubble/oauth1"

	"social-hub/pkg/socialhub"
)

const maxOAuthResponseBytes int64 = 1 << 20

// OAuthClient implements X's three-legged OAuth 1.0a flow.
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
	AuthorizationURL string
}

// OAuthAccessToken is X's durable OAuth 1.0a credential pair. X does not issue
// a refresh token or publish an expiry for this credential.
type OAuthAccessToken struct {
	Token      string
	Secret     string
	UserID     string
	ScreenName string
}

func (client *OAuthClient) BeginAuthorization(ctx context.Context, callbackURL string) (*OAuthRequestToken, error) {
	if !client.valid() || !validCallbackURL(callbackURL) {
		return nil, invalidArgument("oauth_begin", "complete OAuth client and valid callback URL are required")
	}
	config, capture := client.config(ctx, callbackURL)
	token, secret, err := config.RequestToken()
	if err != nil {
		return nil, client.oauthError("oauth_begin", capture, err)
	}
	authorizationURL, err := config.AuthorizationURL(token)
	if err != nil {
		return nil, platformError("oauth_begin", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return &OAuthRequestToken{Token: token, Secret: secret, AuthorizationURL: authorizationURL.String()}, nil
}

func (client *OAuthClient) Exchange(ctx context.Context, requestToken OAuthRequestToken, verifier string) (*OAuthAccessToken, error) {
	if !client.valid() || !validOAuthCredential(requestToken.Token) || !validOAuthCredential(requestToken.Secret) || !validOAuthCredential(verifier) {
		return nil, invalidArgument("oauth_exchange", "request token, request secret, and verifier are required")
	}
	config, capture := client.config(ctx, "")
	token, secret, err := config.AccessToken(requestToken.Token, requestToken.Secret, verifier)
	if err != nil {
		return nil, client.oauthError("oauth_exchange", capture, err)
	}
	values, err := url.ParseQuery(strings.TrimSpace(string(capture.body)))
	if err != nil {
		return nil, platformError("oauth_exchange", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if !validOAuthCredential(token) || !validOAuthCredential(secret) || !validTweetID(values.Get("user_id")) {
		return nil, platformContractError("oauth_exchange", "X access-token response omitted credentials or user identity")
	}
	return &OAuthAccessToken{
		Token: token, Secret: secret, UserID: values.Get("user_id"), ScreenName: values.Get("screen_name"),
	}, nil
}

func (client *OAuthClient) valid() bool {
	return validOpaque(client.ConsumerKey, 1024) && validOpaque(client.ConsumerSecret, 4096) && client.HTTPClient != nil &&
		validEndpoint(client.RequestTokenURL) && validEndpoint(client.AuthorizeURL) && validEndpoint(client.AccessTokenURL)
}

func (client *OAuthClient) config(ctx context.Context, callbackURL string) (*oauth1.Config, *capturedResponseTransport) {
	base := client.HTTPClient.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	capture := &capturedResponseTransport{ctx: ctx, base: base, maximum: maxOAuthResponseBytes}
	httpClient := &http.Client{Transport: capture, Timeout: client.HTTPClient.Timeout, CheckRedirect: rejectRedirect}
	config := &oauth1.Config{
		ConsumerKey: client.ConsumerKey, ConsumerSecret: client.ConsumerSecret, CallbackURL: callbackURL,
		Endpoint: oauth1.Endpoint{
			RequestTokenURL: client.RequestTokenURL, AuthorizeURL: client.AuthorizeURL, AccessTokenURL: client.AccessTokenURL,
		},
		HTTPClient: httpClient, Noncer: client.noncer,
	}
	return config, capture
}

func (client *OAuthClient) oauthError(operation string, capture *capturedResponseTransport, cause error) error {
	if capture.status != 0 && (capture.status < 200 || capture.status >= 300) {
		values, _ := url.ParseQuery(strings.TrimSpace(string(capture.body)))
		platformCode := firstNonEmpty(values.Get("oauth_problem"), values.Get("error"))
		message := firstNonEmpty(values.Get("oauth_problem_advice"), values.Get("error_description"))
		if platformCode == "" {
			var envelope errorEnvelope
			_ = json.Unmarshal(capture.body, &envelope)
			platformCode = envelope.Error
			if len(envelope.Errors) > 0 {
				platformCode = firstNonEmpty(envelope.Errors[0].Code, platformCode)
				message = firstNonEmpty(envelope.Errors[0].Message, envelope.Errors[0].Details, message)
			}
		}
		code, class := classifyError(capture.status, platformCode)
		return &socialhub.Error{
			Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
			HTTPStatus: capture.status, PlatformCode: boundedMessage(platformCode, 256),
			PlatformMessage: boundedMessage(redactSensitive(message), 512),
			RequestID:       boundedMessage(firstNonEmpty(capture.header.Get("x-request-id"), capture.header.Get("x-transaction-id")), 256),
		}
	}
	return platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, cause)
}

func validOAuthCredential(value string) bool {
	return strings.TrimSpace(value) == value && value != "" && len(value) <= 4096 && !strings.ContainsAny(value, "\r\n")
}

type capturedResponseTransport struct {
	ctx     context.Context
	base    http.RoundTripper
	body    []byte
	status  int
	header  http.Header
	maximum int64
}

func (transport *capturedResponseTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := transport.base.RoundTrip(request.Clone(transport.ctx))
	if err != nil {
		return nil, err
	}
	body, readErr := io.ReadAll(io.LimitReader(response.Body, transport.maximum+1))
	_ = response.Body.Close()
	if readErr != nil {
		return nil, readErr
	}
	if int64(len(body)) > transport.maximum {
		return nil, errors.New("OAuth response exceeded size limit")
	}
	transport.body = append([]byte(nil), body...)
	transport.status = response.StatusCode
	transport.header = response.Header.Clone()
	response.Body = io.NopCloser(bytes.NewReader(body))
	return response, nil
}

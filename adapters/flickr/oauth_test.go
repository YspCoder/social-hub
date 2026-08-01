package flickr

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/dghubble/oauth1"

	"social-hub/pkg/socialhub"
)

type fixedNoncer string

func (noncer fixedNoncer) Nonce() string { return string(noncer) }

func TestOAuthAuthorizationAndExchange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/request_token":
			verifyOAuthSignature(t, request, "consumer-secret", "", false)
			parameters, err := parseOAuthHeader(request.Header.Get("Authorization"))
			if err != nil || parameters["oauth_callback"] != "https://client.example/callback?tenant=one" || parameters["oauth_consumer_key"] != "consumer-key" {
				t.Errorf("request token OAuth parameters=%v err=%v", parameters, err)
			}
			writer.Header().Set("Content-Type", "application/x-www-form-urlencoded")
			_, _ = writer.Write([]byte("oauth_token=request-token&oauth_token_secret=request-secret&oauth_callback_confirmed=true"))
		case "/access_token":
			verifyOAuthSignature(t, request, "consumer-secret", "request-secret", false)
			parameters, err := parseOAuthHeader(request.Header.Get("Authorization"))
			if err != nil || parameters["oauth_token"] != "request-token" || parameters["oauth_verifier"] != "verifier-1" {
				t.Errorf("access token OAuth parameters=%v err=%v", parameters, err)
			}
			writer.Header().Set("Content-Type", "application/x-www-form-urlencoded")
			_, _ = writer.Write([]byte("oauth_token=access-token&oauth_token_secret=access-secret&user_nsid=owner%40N01&username=alice&fullname=Alice+Example"))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	client := &OAuthClient{
		ConsumerKey: "consumer-key", ConsumerSecret: "consumer-secret", HTTPClient: server.Client(),
		RequestTokenURL: server.URL + "/request_token", AuthorizeURL: server.URL + "/authorize", AccessTokenURL: server.URL + "/access_token",
		noncer: fixedNoncer("fixed-nonce"),
	}
	temporary, err := client.BeginAuthorization(context.Background(), "https://client.example/callback?tenant=one", PermissionWrite)
	if err != nil {
		t.Fatal(err)
	}
	authorizationURL, err := url.Parse(temporary.AuthorizationURL)
	if err != nil || temporary.Token != "request-token" || temporary.Secret != "request-secret" || temporary.Permission != PermissionWrite || authorizationURL.Query().Get("oauth_token") != "request-token" || authorizationURL.Query().Get("perms") != PermissionWrite {
		t.Fatalf("temporary=%#v URL=%#v err=%v", temporary, authorizationURL, err)
	}
	access, err := client.Exchange(context.Background(), *temporary, "verifier-1")
	if err != nil || access.Token != "access-token" || access.Secret != "access-secret" || access.UserID != "owner@N01" || access.Username != "alice" || access.FullName != "Alice Example" || access.Permission != PermissionWrite {
		t.Fatalf("access=%#v err=%v", access, err)
	}
}

func TestOAuthErrorsAndValidation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/x-www-form-urlencoded")
		writer.Header().Set("X-Request-ID", "oauth-request")
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = writer.Write([]byte("oauth_problem=permission_denied&oauth_problem_advice=approval+was+denied"))
	}))
	defer server.Close()
	client := &OAuthClient{
		ConsumerKey: "consumer-key", ConsumerSecret: "consumer-secret", HTTPClient: server.Client(),
		RequestTokenURL: server.URL, AuthorizeURL: server.URL + "/authorize", AccessTokenURL: server.URL,
		noncer: fixedNoncer("fixed-nonce"),
	}
	_, err := client.BeginAuthorization(context.Background(), "https://client.example/callback", PermissionRead)
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodeUnauthenticated || platformErr.PlatformCode != "permission_denied" || platformErr.RequestID != "oauth-request" {
		t.Fatalf("OAuth denial=%#v", err)
	}

	invalid := []struct {
		name string
		run  func() error
	}{
		{"begin callback", func() error {
			_, err := client.BeginAuthorization(context.Background(), "javascript:alert(1)", PermissionRead)
			return err
		}},
		{"begin permission", func() error {
			_, err := client.BeginAuthorization(context.Background(), "https://client.example/callback", "admin")
			return err
		}},
		{"exchange token", func() error {
			_, err := client.Exchange(context.Background(), OAuthRequestToken{Token: " bad", Secret: "secret", Permission: PermissionRead}, "verifier")
			return err
		}},
		{"exchange verifier", func() error {
			_, err := client.Exchange(context.Background(), OAuthRequestToken{Token: "token", Secret: "secret", Permission: PermissionRead}, "bad\nverifier")
			return err
		}},
	}
	for _, test := range invalid {
		t.Run(test.name, func(t *testing.T) {
			if code := errorCode(test.run()); code != socialhub.CodeInvalidArgument {
				t.Fatalf("code=%q", code)
			}
		})
	}
}

func TestOAuthExchangeRejectsMissingIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/x-www-form-urlencoded")
		_, _ = writer.Write([]byte("oauth_token=access-token&oauth_token_secret=access-secret"))
	}))
	defer server.Close()
	client := &OAuthClient{
		ConsumerKey: "consumer-key", ConsumerSecret: "consumer-secret", HTTPClient: server.Client(),
		RequestTokenURL: server.URL, AuthorizeURL: server.URL, AccessTokenURL: server.URL,
		noncer: fixedNoncer("fixed-nonce"),
	}
	_, err := client.Exchange(context.Background(), OAuthRequestToken{Token: "request-token", Secret: "request-secret", Permission: PermissionRead}, "verifier")
	if errorCode(err) != socialhub.CodePlatformError || !strings.Contains(err.Error(), "platform_error") {
		t.Fatalf("error=%v", err)
	}
}

var _ oauth1.Noncer = fixedNoncer("")

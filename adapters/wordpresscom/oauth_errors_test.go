package wordpresscom

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestOAuthAuthorizationAndExchange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/oauth2/token" || request.ParseForm() != nil ||
			request.Header.Get("Accept") != "application/json" || request.Header.Get("Content-Type") != "application/x-www-form-urlencoded" ||
			request.PostForm.Get("client_id") != "client-id" || request.PostForm.Get("client_secret") != "client-secret" ||
			request.PostForm.Get("code") != "auth-code" || request.PostForm.Get("grant_type") != "authorization_code" ||
			request.PostForm.Get("redirect_uri") != "https://app.example/callback" {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		writeJSON(writer, http.StatusOK, `{"access_token":"oauth-token","blog_id":"123","blog_url":"https://example.wordpress.com","token_type":"bearer","scope":"posts,media users"}`)
	}))
	defer server.Close()
	adapter, _ := newTestClient(t, server, true, []string{"global"})
	oauth, err := adapter.OAuth(context.Background(), "blog")
	if err != nil {
		t.Fatal(err)
	}
	authorizationURL, err := oauth.AuthorizationURL("https://app.example/callback", "state-value", []string{"posts", "media"})
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(authorizationURL)
	query := parsed.Query()
	if parsed.Path != "/oauth2/authorize" || query.Get("client_id") != "client-id" || query.Get("redirect_uri") != "https://app.example/callback" || query.Get("response_type") != "code" || query.Get("blog") != "123" || query.Get("state") != "state-value" || query.Get("scope") != "posts media" {
		t.Fatalf("authorization URL=%s", authorizationURL)
	}
	result, err := oauth.Exchange(context.Background(), "auth-code", "https://app.example/callback")
	if err != nil || result.BlogID != "123" || result.BlogURL != "https://example.wordpress.com" || result.Token.AccessToken != "oauth-token" || result.Token.TokenType != "Bearer" || len(result.Token.Scopes) != 3 || result.Token.RefreshToken != "" || !result.Token.ExpiresAt.IsZero() {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestOAuthValidationAndResponseErrors(t *testing.T) {
	client := &OAuthClient{}
	if _, err := client.AuthorizationURL("bad", "", nil); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("authorize validation=%v", err)
	}
	client.ClientID, client.Site, client.AuthURL = "id", "example.wordpress.com", "https://public-api.wordpress.com/oauth2/authorize"
	if _, err := client.AuthorizationURL("https://app.example/callback", "state", []string{"unknown"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("scope validation=%v", err)
	}
	if authorizationURL, err := client.AuthorizationURL("https://app.example/callback", "state", nil); err != nil {
		t.Fatal(err)
	} else if parsed, _ := url.Parse(authorizationURL); parsed.Query().Has("scope") {
		t.Fatalf("unexpected scope in %s", authorizationURL)
	}
	if _, err := client.Exchange(context.Background(), "", "bad"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("exchange validation=%v", err)
	}

	tests := []struct {
		name   string
		status int
		body   string
		code   socialhub.ErrorCode
		class  socialhub.ErrorClass
	}{
		{"wrapper", http.StatusOK, `{"error":"invalid_grant","error_description":"expired"}`, socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{"scope", http.StatusBadRequest, `{"error":"invalid_scope","error_description":"bad scope"}`, socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{"rate", http.StatusTooManyRequests, `{}`, socialhub.CodeRateLimited, socialhub.ClassRetryable},
		{"server", http.StatusServiceUnavailable, `{}`, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{"non-json server", http.StatusBadGateway, `upstream unavailable`, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.Header().Set("Retry-After", "1.5")
				writer.Header().Set("X-Automattic-Request-Id", "request-1")
				writeJSON(writer, test.status, test.body)
			}))
			defer server.Close()
			oauth := &OAuthClient{ClientID: "id", ClientSecret: "secret", Site: "123", TokenURL: server.URL, HTTPClient: server.Client()}
			_, err := oauth.Exchange(context.Background(), "code", "https://app.example/callback")
			var platformErr *socialhub.Error
			if !errors.As(err, &platformErr) || platformErr.Code != test.code || platformErr.Class != test.class || platformErr.HTTPStatus != test.status || platformErr.RequestID != "request-1" || platformErr.RetryAfter != 1500*time.Millisecond {
				t.Fatalf("error=%#v", err)
			}
		})
	}

	badBodies := []string{
		`not-json`,
		`{"access_token":"token","blog_id":124,"blog_url":"https://example.wordpress.com"}`,
		`{"access_token":"token","blog_id":123,"blog_url":"bad"}`,
		`{"blog_id":123,"blog_url":"https://example.wordpress.com"}`,
	}
	for index, body := range badBodies {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writeJSON(writer, http.StatusOK, body)
		}))
		oauth := &OAuthClient{ClientID: "id", ClientSecret: "secret", Site: "123", TokenURL: server.URL, HTTPClient: server.Client()}
		_, err := oauth.Exchange(context.Background(), "code", "https://app.example/callback")
		server.Close()
		if !isPlatformError(err) {
			t.Fatalf("bad body %d error=%v", index, err)
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, http.StatusOK, `{"access_token":"`+strings.Repeat("x", int(maxOAuthResponseBytes))+`"}`)
	}))
	defer server.Close()
	oauth := &OAuthClient{ClientID: "id", ClientSecret: "secret", Site: "123", TokenURL: server.URL, HTTPClient: server.Client()}
	if _, err := oauth.Exchange(context.Background(), "code", "https://app.example/callback"); !isPlatformError(err) {
		t.Fatalf("oversized response=%v", err)
	}
}

func TestRedirectRefusal(t *testing.T) {
	var followed bool
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { followed = true }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, target.URL, http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	adapter, client := newTestClient(t, server, true, []string{"global"})
	if _, err := client.GetPost(context.Background(), "1"); err == nil || followed {
		t.Fatalf("API error=%v followed=%v", err, followed)
	}
	oauth, err := adapter.OAuth(context.Background(), "blog")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := oauth.Exchange(context.Background(), "code", "https://app.example/callback"); err == nil || followed {
		t.Fatalf("OAuth error=%v followed=%v", err, followed)
	}
}

func TestHTTPErrorClassificationAndHelpers(t *testing.T) {
	tests := []struct {
		status int
		code   socialhub.ErrorCode
		class  socialhub.ErrorClass
	}{
		{http.StatusBadRequest, socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{http.StatusUnprocessableEntity, socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{http.StatusRequestEntityTooLarge, socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{http.StatusUnauthorized, socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{http.StatusForbidden, socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{http.StatusNotFound, socialhub.CodeNotFound, socialhub.ClassPermanent},
		{http.StatusGone, socialhub.CodeNotFound, socialhub.ClassPermanent},
		{http.StatusConflict, socialhub.CodeConflict, socialhub.ClassPermanent},
		{http.StatusLocked, socialhub.CodeConflict, socialhub.ClassPermanent},
		{http.StatusTooManyRequests, socialhub.CodeRateLimited, socialhub.ClassRetryable},
		{http.StatusServiceUnavailable, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{http.StatusTeapot, socialhub.CodePlatformError, socialhub.ClassPermanent},
	}
	header := http.Header{"Retry-After": {"2.5"}, "X-Request-Id": {"request-1"}}
	for _, test := range tests {
		err := decodeHTTPError(test.status, header, []byte(`{"error":"failure","message":"failed"}`))
		var platformErr *socialhub.Error
		if !errors.As(err, &platformErr) || platformErr.Code != test.code || platformErr.Class != test.class || platformErr.PlatformCode != "failure" || platformErr.PlatformMessage != "failed" || platformErr.RequestID != "request-1" || platformErr.RetryAfter != 2500*time.Millisecond {
			t.Fatalf("status=%d error=%#v", test.status, err)
		}
	}
	if parseRetryAfter("bad") != 0 || parseRetryAfter("86401") != 0 || boundedMessage(strings.Repeat("界", 520), 512) != strings.Repeat("界", 512) || firstNonEmpty("", "value") != "value" {
		t.Fatal("error helper contract failed")
	}
	if parseBlogID([]byte(`"123"`)) != "123" || parseBlogID([]byte(`124`)) != "124" || parseBlogID([]byte(`0`)) != "" || len(splitOAuthScopes("posts,media users")) != 3 {
		t.Fatal("OAuth helper contract failed")
	}
	if !validCallbackURL("https://app.example/callback?x=1") || validCallbackURL("https://user:pass@app.example/callback") || !validBlogURL("http://example.com") || validBlogURL("relative") {
		t.Fatal("URL helper contract failed")
	}
}

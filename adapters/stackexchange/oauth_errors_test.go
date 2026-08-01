package stackexchange

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

const (
	testCodeChallenge = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQ"
	testCodeVerifier  = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQ"
)

func TestOAuthAuthorizationAndExchange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/oauth/access_token/json" || request.Method != http.MethodPost || request.ParseForm() != nil || request.UserAgent() != defaultUserAgent ||
			request.PostForm.Get("client_id") != "12345" || request.PostForm.Get("client_secret") != "client-secret" ||
			(request.PostForm.Get("code") != "auth-code" && request.PostForm.Get("code") != "pkce-code") || request.PostForm.Get("redirect_uri") != "https://app.example/callback" {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		if request.PostForm.Get("code") == "pkce-code" && request.PostForm.Get("code_verifier") != testCodeVerifier {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		writeJSON(writer, http.StatusOK, `{"access_token":"oauth-token","expires":3600}`)
	}))
	defer server.Close()
	clock := &mutableClock{now: time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)}
	adapter, _ := newTestClient(t, server, true, []string{"write_access"}, clock)
	oauth, err := adapter.OAuth(context.Background(), "stackoverflow")
	if err != nil {
		t.Fatal(err)
	}
	authorizationURL, err := oauth.AuthorizationURL("https://app.example/callback", "state-value", []string{"write_access", "no_expiry"})
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(authorizationURL)
	if parsed.Path != "/oauth" || parsed.Query().Get("client_id") != "12345" || parsed.Query().Get("redirect_uri") != "https://app.example/callback" || parsed.Query().Get("state") != "state-value" || parsed.Query().Get("scope") != "write_access no_expiry" {
		t.Fatalf("authorization URL=%s", authorizationURL)
	}
	pkceURL, err := oauth.AuthorizationURLPKCE("https://app.example/callback", "pkce-state", []string{"write_access"}, testCodeChallenge)
	if err != nil {
		t.Fatal(err)
	}
	parsedPKCE, _ := url.Parse(pkceURL)
	if parsedPKCE.Query().Get("code_challenge") != testCodeChallenge || parsedPKCE.Query().Get("code_challenge_method") != "S256" {
		t.Fatalf("PKCE URL=%s", pkceURL)
	}
	token, err := oauth.Exchange(context.Background(), "auth-code", "https://app.example/callback")
	if err != nil || token.AccessToken != "oauth-token" || token.RefreshToken != "" || token.TokenType != "Bearer" || !token.ExpiresAt.Equal(clock.Now().Add(time.Hour)) {
		t.Fatalf("token=%#v err=%v", token, err)
	}
	if _, err := oauth.ExchangeWithVerifier(context.Background(), "pkce-code", "https://app.example/callback", testCodeVerifier); err != nil {
		t.Fatal(err)
	}
}

func TestOAuthAndErrorBoundaries(t *testing.T) {
	t.Run("public PKCE no expiry", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writeJSON(writer, http.StatusOK, `{"access_token":"permanent-token"}`)
		}))
		defer server.Close()
		clock := &mutableClock{now: time.Now()}
		client := &OAuthClient{ClientID: "1", AuthURL: server.URL, TokenURL: server.URL, UserAgent: defaultUserAgent, HTTPClient: server.Client(), Clock: clock}
		token, err := client.ExchangeWithVerifier(context.Background(), "code", "https://app.example/callback", testCodeVerifier)
		if err != nil || !token.ExpiresAt.IsZero() {
			t.Fatalf("token=%#v err=%v", token, err)
		}
	})
	t.Run("wrapper error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writeJSON(writer, http.StatusOK, `{"error":{"type":"invalid_request","message":"bad code"}}`)
		}))
		defer server.Close()
		clock := &mutableClock{now: time.Now()}
		client := &OAuthClient{ClientID: "1", ClientSecret: "secret", AuthURL: server.URL, TokenURL: server.URL, UserAgent: defaultUserAgent, HTTPClient: server.Client(), Clock: clock}
		if _, err := client.Exchange(context.Background(), "code", "https://app.example/callback"); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("error=%v", err)
		}
	})
	t.Run("HTTP OAuth error", func(t *testing.T) {
		err := decodeHTTPError(http.StatusBadRequest, nil, []byte(`{"error":{"type":"access_denied","message":"user denied"}}`))
		if !errors.Is(err, socialhub.ErrPermissionDenied) {
			t.Fatalf("error=%v", err)
		}
	})
	t.Run("redirect", func(t *testing.T) {
		var followed bool
		target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { followed = true }))
		defer target.Close()
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			http.Redirect(writer, request, target.URL, http.StatusTemporaryRedirect)
		}))
		defer server.Close()
		clock := &mutableClock{now: time.Now()}
		base := server.Client()
		clone := *base
		clone.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
		client := &OAuthClient{ClientID: "1", ClientSecret: "secret", AuthURL: server.URL, TokenURL: server.URL, UserAgent: defaultUserAgent, HTTPClient: &clone, Clock: clock}
		if _, err := client.Exchange(context.Background(), "code", "https://app.example/callback"); err == nil || followed {
			t.Fatalf("error=%v followed=%v", err, followed)
		}
	})

	client := &OAuthClient{}
	if _, err := client.AuthorizationURL("bad", "", nil); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("authorize validation=%v", err)
	}
	if _, err := client.Exchange(context.Background(), "", "bad"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("exchange validation=%v", err)
	}
	if _, err := client.AuthorizationURLPKCE("https://app.example/callback", "state", nil, "short"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("PKCE authorize validation=%v", err)
	}
	if _, err := client.ExchangeWithVerifier(context.Background(), "code", "https://app.example/callback", "short"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("PKCE exchange validation=%v", err)
	}

	classifications := map[string]socialhub.ErrorCode{
		"bad_parameter": socialhub.CodeInvalidArgument, "access_denied": socialhub.CodePermissionDenied,
		"duplicate_request": socialhub.CodeConflict, "too_many_ips": socialhub.CodeRateLimited,
		"internal_error": socialhub.CodeTemporarilyUnavailable, "no_method": socialhub.CodeNotFound,
	}
	for name, expected := range classifications {
		code, _ := classifyError(http.StatusOK, name)
		if code != expected {
			t.Fatalf("name=%s code=%s expected=%s", name, code, expected)
		}
	}
	if parseRetryAfter("61") != 61*time.Second || parseRetryAfter("bad") != 0 || !strings.HasPrefix(boundedMessage(strings.Repeat("界", 600), 512), strings.Repeat("界", 10)) {
		t.Fatal("error helper contract failed")
	}
}

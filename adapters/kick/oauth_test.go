package kick

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

func TestOAuthFlows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/oauth/token":
			if request.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
				http.Error(writer, "content type", http.StatusBadRequest)
				return
			}
			if err := request.ParseForm(); err != nil || request.Form.Get("client_id") != "client-id" || request.Form.Get("client_secret") != "client-secret" {
				http.Error(writer, "credentials", http.StatusBadRequest)
				return
			}
			switch request.Form.Get("grant_type") {
			case "authorization_code":
				if request.Form.Get("code") == "rate" {
					writer.Header().Set("Retry-After", "2")
					writer.WriteHeader(http.StatusTooManyRequests)
					_, _ = writer.Write([]byte(`{"message":"slow down"}`))
					return
				}
				writeJSON(t, writer, map[string]any{"access_token": "user-access", "refresh_token": "user-refresh", "token_type": "Bearer", "expires_in": 3600, "scope": "user:read chat:write"})
			case "client_credentials":
				writeJSON(t, writer, map[string]any{"access_token": "app-access", "token_type": "bearer", "expires_in": 7200})
			case "refresh_token":
				writeJSON(t, writer, map[string]any{"access_token": "refreshed-access", "refresh_token": "refreshed-refresh", "token_type": "Bearer", "expires_in": 1800, "scope": "user:read"})
			default:
				http.Error(writer, "grant", http.StatusBadRequest)
			}
		case "/oauth/revoke":
			if request.URL.Query().Get("token") != "user-access" || request.URL.Query().Get("token_hint_type") != "access_token" {
				http.Error(writer, "revoke query", http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte("OK"))
		case "/oauth/token/introspect":
			if request.Header.Get("Authorization") != "Bearer user-access" {
				http.Error(writer, "auth", http.StatusUnauthorized)
				return
			}
			writeJSON(t, writer, map[string]any{"data": map[string]any{"active": true, "client_id": "client-id", "token_type": "user", "scope": "user:read", "exp": 1785680000}})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	client := &OAuthClient{
		ClientID: "client-id", ClientSecret: "client-secret", AuthURL: server.URL + "/oauth/authorize",
		TokenURL: server.URL + "/oauth/token", RevokeURL: server.URL + "/oauth/revoke", IntrospectURL: server.URL + "/oauth/token/introspect",
		HTTPClient: server.Client(), Clock: fixedClock{now: testNow},
	}
	pkce, err := NewPKCE()
	if err != nil || !validPKCEValue(pkce.Verifier) || !validPKCEValue(pkce.Challenge) {
		t.Fatalf("PKCE: %#v %v", pkce, err)
	}
	authorizationURL, err := client.AuthorizationURL("https://app.test/callback", "state-1", pkce, []string{"user:read", "chat:write"})
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(authorizationURL)
	if parsed.Query().Get("code_challenge_method") != "S256" || parsed.Query().Get("scope") != "user:read chat:write" || parsed.Query().Get("state") != "state-1" {
		t.Fatalf("authorization URL: %s", authorizationURL)
	}
	userToken, err := client.Exchange(context.Background(), "code", "https://app.test/callback", pkce.Verifier)
	if err != nil || userToken.AccessToken != "user-access" || userToken.RefreshToken != "user-refresh" || !userToken.ExpiresAt.Equal(testNow.Add(time.Hour)) || len(userToken.Scopes) != 2 {
		t.Fatalf("exchange: %#v %v", userToken, err)
	}
	appToken, err := client.ClientCredentials(context.Background())
	if err != nil || appToken.AccessToken != "app-access" || !appToken.ExpiresAt.Equal(testNow.Add(2*time.Hour)) {
		t.Fatalf("client credentials: %#v %v", appToken, err)
	}
	refreshed, err := client.Refresh(context.Background(), "user-refresh")
	if err != nil || refreshed.RefreshToken != "refreshed-refresh" || !refreshed.ExpiresAt.Equal(testNow.Add(30*time.Minute)) {
		t.Fatalf("refresh: %#v %v", refreshed, err)
	}
	if err := client.Revoke(context.Background(), "user-access", "access_token"); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	introspection, err := client.Introspect(context.Background(), "user-access")
	if err != nil || !introspection.Active || introspection.TokenType != "user" {
		t.Fatalf("introspect: %#v %v", introspection, err)
	}
	_, err = client.Exchange(context.Background(), "rate", "https://app.test/callback", pkce.Verifier)
	if !errors.Is(err, socialhub.ErrRateLimited) {
		t.Fatalf("rate limit: %v", err)
	}
	var hubError *socialhub.Error
	if !errors.As(err, &hubError) || hubError.RetryAfter != 2*time.Second || hubError.PlatformMessage != "slow down" {
		t.Fatalf("rate details: %#v", hubError)
	}
}

func TestOAuthValidation(t *testing.T) {
	pkce := PKCE{Verifier: strings.Repeat("a", 43), Challenge: strings.Repeat("b", 43)}
	client := &OAuthClient{ClientID: "id", ClientSecret: "secret", AuthURL: "https://id.kick.com/oauth/authorize"}
	if _, err := client.AuthorizationURL("bad", "state", pkce, []string{"user:read"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("redirect: %v", err)
	}
	if _, err := client.AuthorizationURL("https://app.test/callback", "state", pkce, []string{"unknown"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("scope: %v", err)
	}
	if _, err := client.AuthorizationURL("https://app.test/callback", "state", pkce, []string{"user:read", "user:read"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("duplicate scope: %v", err)
	}
	if _, err := client.Exchange(context.Background(), "", "https://app.test/callback", pkce.Verifier); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("exchange: %v", err)
	}
	if _, err := client.Refresh(context.Background(), ""); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("refresh: %v", err)
	}
	if err := client.Revoke(context.Background(), "token", "bad"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("revoke: %v", err)
	}
	if _, err := client.Introspect(context.Background(), "bad token"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("introspect: %v", err)
	}
	if validRedirectURI("ftp://app.test") || validPKCEValue("short") || validPKCEValue(strings.Repeat("?", 43)) || !validRedirectURI("http://localhost/callback") {
		t.Fatal("OAuth validators mismatch")
	}
}

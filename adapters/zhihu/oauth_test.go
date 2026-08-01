package zhihu

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestOAuthAuthorizationAndExchange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/access_token" || request.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		if err := request.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if request.Form.Get("app_id") != "app-id" || request.Form.Get("app_key") != "app-key" || request.Form.Get("grant_type") != "authorization_code" || request.Form.Get("redirect_uri") != "https://app.example/callback" || request.Form.Get("code") != "code-1" {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = writer.Write([]byte(`{"access_token":"oauth-token","token_type":"Bearer","expires_in":3600}`))
	}))
	defer server.Close()
	adapter, _ := newTestAdapter(t, server, true, false, true)
	oauth, err := adapter.OAuth(context.Background(), "primary")
	if err != nil {
		t.Fatal(err)
	}
	authorizationURL, err := oauth.AuthorizationURL("https://app.example/callback")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(authorizationURL)
	if err != nil || parsed.Path != "/authorize" || parsed.Query().Get("app_id") != "app-id" || parsed.Query().Get("redirect_uri") != "https://app.example/callback" || parsed.Query().Get("response_type") != "code" {
		t.Fatalf("authorization URL=%q err=%v", authorizationURL, err)
	}
	token, err := oauth.Exchange(context.Background(), "code-1", "https://app.example/callback")
	if err != nil {
		t.Fatal(err)
	}
	wantExpiry := time.Date(2026, 8, 1, 1, 0, 0, 0, time.UTC)
	if token.AccessToken != "oauth-token" || token.TokenType != "Bearer" || !token.ExpiresAt.Equal(wantExpiry) {
		t.Fatalf("token=%#v", token)
	}
}

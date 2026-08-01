package weibo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestOAuthAuthorizationURLAndExchange(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/oauth2/access_token" || request.Method != http.MethodPost {
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		if err := request.ParseForm(); err != nil {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		if request.Form.Get("client_id") != "client-id" || request.Form.Get("client_secret") != "client-secret" || request.Form.Get("code") != "code-1" || request.Form.Get("grant_type") != "authorization_code" {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = writer.Write([]byte(`{"access_token":"token-1","expires_in":"7200","uid":"42"}`))
	}))
	defer server.Close()
	adapter, _ := newTestAdapter(t, server)
	oauth, err := adapter.OAuth(context.Background(), "primary")
	if err != nil {
		t.Fatal(err)
	}
	authorizationURL, err := oauth.AuthorizationURL("https://app.example/callback", "state-1", []string{"all"})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(authorizationURL)
	if err != nil || parsed.Query().Get("state") != "state-1" || parsed.Query().Get("scope") != "all" {
		t.Fatalf("authorization URL=%q err=%v", authorizationURL, err)
	}
	token, err := oauth.Exchange(context.Background(), "code-1", "https://app.example/callback")
	if err != nil || token.AccessToken != "token-1" {
		t.Fatalf("token=%#v err=%v", token, err)
	}
	wantExpiry := time.Date(2026, 8, 1, 2, 0, 0, 0, time.UTC)
	if !token.ExpiresAt.Equal(wantExpiry) {
		t.Fatalf("expiry=%s want=%s", token.ExpiresAt, wantExpiry)
	}
}

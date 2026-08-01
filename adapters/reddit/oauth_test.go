package reddit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestRedditOAuthFlows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		clientID, secret, ok := request.BasicAuth()
		if request.URL.Path != "/api/v1/access_token" || request.Method != http.MethodPost || !ok || clientID != "client-id" || secret != "client-secret" || request.Header.Get("User-Agent") != testUserAgent || request.ParseForm() != nil {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		switch request.Form.Get("grant_type") {
		case "authorization_code":
			writeJSON(writer, `{"access_token":"access","refresh_token":"refresh","token_type":"bearer","expires_in":3600,"scope":"identity read history"}`)
		case "refresh_token":
			writeJSON(writer, `{"access_token":"renewed","token_type":"bearer","expires_in":3600,"scope":"identity read"}`)
		default:
			writer.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()
	adapter, _ := newTestAdapter(t, server, nil)
	oauth, err := adapter.OAuth(context.Background(), "redditor")
	if err != nil {
		t.Fatal(err)
	}
	authorizationURL, err := oauth.AuthorizationURL("https://app.example/callback", "state", []string{"identity", "read", "history"})
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(authorizationURL)
	if parsed.Query().Get("duration") != "permanent" || parsed.Query().Get("scope") != "identity read history" {
		t.Fatalf("authorization URL=%s", authorizationURL)
	}
	token, err := oauth.Exchange(context.Background(), "code", "https://app.example/callback")
	if err != nil || token.AccessToken != "access" || token.RefreshToken != "refresh" || time.Until(token.ExpiresAt) < 59*time.Minute {
		t.Fatalf("token=%#v err=%v", token, err)
	}
	refreshed, err := oauth.Refresh(context.Background(), token.RefreshToken)
	if err != nil || refreshed.AccessToken != "renewed" || refreshed.RefreshToken != "refresh" {
		t.Fatalf("refreshed=%#v err=%v", refreshed, err)
	}
}

package tiktok

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestTikTokOAuthFlows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/oauth/token" || request.Method != http.MethodPost || request.ParseForm() != nil || request.Form.Get("client_key") != "client-key" || request.Form.Get("client_secret") != "client-secret" {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		switch request.Form.Get("grant_type") {
		case "authorization_code":
			writeJSON(writer, `{"access_token":"access","expires_in":86400,"open_id":"open-id","refresh_expires_in":31536000,"refresh_token":"refresh","scope":"user.info.basic,video.list","token_type":"Bearer"}`)
		case "refresh_token":
			writeJSON(writer, `{"access_token":"renewed","expires_in":86400,"open_id":"open-id","refresh_expires_in":31536000,"refresh_token":"refresh-2","scope":"user.info.basic,video.list"}`)
		default:
			writer.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()
	adapter, _ := newTestAdapter(t, server, nil, true)
	oauth, err := adapter.OAuth(context.Background(), "creator")
	if err != nil {
		t.Fatal(err)
	}
	authorizationURL, err := oauth.AuthorizationURLPKCE("https://app.example/callback", "state", []string{"user.info.basic", "video.list"}, "challenge")
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(authorizationURL)
	if parsed.Query().Get("scope") != "user.info.basic,video.list" || parsed.Query().Get("code_challenge_method") != "S256" {
		t.Fatalf("authorization URL=%s", authorizationURL)
	}
	result, err := oauth.ExchangeWithVerifier(context.Background(), "code", "https://app.example/callback", "verifier")
	if err != nil || result.OpenID != "open-id" || result.Token.AccessToken != "access" || time.Until(result.Token.ExpiresAt) < 23*time.Hour {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	refreshed, err := oauth.Refresh(context.Background(), result.Token.RefreshToken)
	if err != nil || refreshed.Token.AccessToken != "renewed" || refreshed.Token.RefreshToken != "refresh-2" {
		t.Fatalf("refreshed=%#v err=%v", refreshed, err)
	}
}

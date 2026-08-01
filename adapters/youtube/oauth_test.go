package youtube

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestYouTubeOAuthFlows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/oauth/token" || request.Method != http.MethodPost || request.ParseForm() != nil || request.Form.Get("client_secret") != "client-secret" {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		switch request.Form.Get("grant_type") {
		case "authorization_code":
			writeJSON(writer, `{"access_token":"access","expires_in":3600,"refresh_token":"refresh","scope":"youtube.readonly youtube.upload","token_type":"Bearer"}`)
		case "refresh_token":
			writeJSON(writer, `{"access_token":"renewed","expires_in":3600,"scope":"youtube.readonly"}`)
		default:
			writer.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()
	adapter, _ := newTestAdapter(t, server, nil)
	oauth, err := adapter.OAuth(context.Background(), "channel")
	if err != nil {
		t.Fatal(err)
	}
	authorizationURL, err := oauth.AuthorizationURL("https://app.example/callback", "state", []string{"youtube.readonly", "youtube.upload"})
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(authorizationURL)
	if parsed.Query().Get("access_type") != "offline" || parsed.Query().Get("include_granted_scopes") != "true" {
		t.Fatalf("authorization URL=%s", authorizationURL)
	}
	token, err := oauth.Exchange(context.Background(), "code", "https://app.example/callback")
	if err != nil || token.AccessToken != "access" || token.RefreshToken != "refresh" || time.Until(token.ExpiresAt) < 59*time.Minute {
		t.Fatalf("token=%#v err=%v", token, err)
	}
	refreshed, err := oauth.Refresh(context.Background(), token.RefreshToken)
	if err != nil || refreshed.AccessToken != "renewed" {
		t.Fatalf("refreshed=%#v err=%v", refreshed, err)
	}
}

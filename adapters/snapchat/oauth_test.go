package snapchat

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestSnapchatOAuthFlows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/access_token" || request.Method != http.MethodPost || request.ParseForm() != nil ||
			request.Form.Get("client_id") != "client-id" || request.Form.Get("client_secret") != "client-secret" {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		switch request.Form.Get("grant_type") {
		case "authorization_code":
			if request.Form.Get("redirect_uri") != "https://app.example/callback" || request.Form.Get("code") != "code" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, `{"access_token":"access","refresh_token":"refresh-1","token_type":"Bearer","expires_in":3600,"scope":"snapchat-profile-api"}`)
		case "refresh_token":
			if request.Form.Get("refresh_token") != "refresh-1" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, `{"access_token":"renewed","refresh_token":"refresh-2","token_type":"bearer","expires_in":3600,"scope":"snapchat-profile-api"}`)
		default:
			writer.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()
	adapter, _ := newTestAdapter(t, server, nil)
	oauth, err := adapter.OAuth(context.Background(), "creator")
	if err != nil {
		t.Fatal(err)
	}
	authorizationURL, err := oauth.AuthorizationURL("https://app.example/callback", "state", []string{requiredScope, "snapchat-marketing-api"})
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(authorizationURL)
	if parsed.Query().Get("scope") != "snapchat-profile-api snapchat-marketing-api" || parsed.Query().Get("state") != "state" {
		t.Fatalf("authorization URL=%s", authorizationURL)
	}
	token, err := oauth.Exchange(context.Background(), "code", "https://app.example/callback")
	if err != nil || token.AccessToken != "access" || token.RefreshToken != "refresh-1" || time.Until(token.ExpiresAt) < 59*time.Minute {
		t.Fatalf("token=%#v err=%v", token, err)
	}
	refreshed, err := oauth.Refresh(context.Background(), token.RefreshToken)
	if err != nil || refreshed.AccessToken != "renewed" || refreshed.RefreshToken != "refresh-2" || refreshed.TokenType != "Bearer" {
		t.Fatalf("refreshed=%#v err=%v", refreshed, err)
	}
}

func TestOAuthValidation(t *testing.T) {
	client := &OAuthClient{ClientID: "client-id", AuthURL: "://invalid"}
	if _, err := client.AuthorizationURL("https://app.example/callback", "state", []string{requiredScope}); err == nil {
		t.Fatal("expected invalid authorization URL error")
	}
	client.AuthURL = "https://accounts.snapchat.com/login/oauth2/authorize"
	if _, err := client.AuthorizationURL("https://app.example/callback", "state", nil); err == nil {
		t.Fatal("expected missing scope error")
	}
	if _, err := client.Exchange(context.Background(), "", ""); err == nil {
		t.Fatal("expected exchange validation error")
	}
}

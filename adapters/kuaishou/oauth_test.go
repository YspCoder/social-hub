package kuaishou

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestOAuthExchangeAndRefresh(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Query().Get("app_id") != "app-id" || request.URL.Query().Get("app_secret") != "app-secret" {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		switch request.URL.Path {
		case "/oauth2/access_token":
			if request.URL.Query().Get("code") != "code-1" || request.URL.Query().Get("grant_type") != "authorization_code" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"result":1,"access_token":"token-1","expires_in":172800,"refresh_token":"refresh-1","refresh_token_expires_in":15552000,"open_id":"open-id-1","scopes":["user_info","user_video_publish"]}`))
		case "/oauth2/refresh_token":
			if request.URL.Query().Get("refresh_token") != "refresh-1" || request.URL.Query().Get("grant_type") != "refresh_token" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"result":1,"access_token":"token-2","expires_in":172800,"refresh_token":"refresh-2","refresh_token_expires_in":1000,"scopes":["user_info"]}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	adapter, _ := newTestAdapter(t, server)
	oauth, err := adapter.OAuth(context.Background(), "primary")
	if err != nil {
		t.Fatal(err)
	}
	authorizationURL, err := oauth.AuthorizationURL("https://app.example/callback", "state-1", []string{"user_info", "user_video_publish"})
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(authorizationURL)
	if parsed.Query().Get("scope") != "user_info,user_video_publish" || parsed.Query().Get("state") != "state-1" || parsed.Query().Get("app_id") != "app-id" {
		t.Fatalf("authorization URL=%q", authorizationURL)
	}
	token, err := oauth.Exchange(context.Background(), "code-1")
	if err != nil || token.OpenID != "open-id-1" || token.Token.AccessToken != "token-1" || len(token.Token.Scopes) != 2 {
		t.Fatalf("token=%#v err=%v", token, err)
	}
	if !token.Token.ExpiresAt.Equal(time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("access expiry=%s", token.Token.ExpiresAt)
	}
	refreshed, err := oauth.Refresh(context.Background(), "refresh-1")
	if err != nil || refreshed.Token.AccessToken != "token-2" || refreshed.Token.RefreshToken != "refresh-2" {
		t.Fatalf("refreshed=%#v err=%v", refreshed, err)
	}
}

package bilibili

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestOAuthExchangeAndSingleUseRefresh(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.Header.Get("Content-Type") != "application/x-www-form-urlencoded" || request.URL.Query().Get("client_id") != "client-id" || request.URL.Query().Get("client_secret") != "app-secret" {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		switch request.URL.Path {
		case "/x/account-oauth2/v1/token":
			if request.URL.Query().Get("code") != "code-1" || request.URL.Query().Get("grant_type") != "authorization_code" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"code":0,"message":"0","data":{"access_token":"token-1","refresh_token":"refresh-1","expires_in":1785715200,"scopes":["USER_INFO","ARC_BASE"]}}`))
		case "/x/account-oauth2/v1/refresh_token":
			if request.URL.Query().Get("refresh_token") != "refresh-1" || request.URL.Query().Get("grant_type") != "refresh_token" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"code":0,"message":"0","data":{"access_token":"token-2","refresh_token":"refresh-2","expires_in":1785801600}}`))
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
	authorizationURL, err := oauth.AuthorizationURL("https://app.example/callback", "state-1")
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(authorizationURL)
	if parsed.Query().Get("client_id") != "client-id" || parsed.Query().Get("gourl") != "https://app.example/callback" || parsed.Query().Get("state") != "state-1" {
		t.Fatalf("authorization URL=%q", authorizationURL)
	}
	token, err := oauth.Exchange(context.Background(), "code-1")
	if err != nil || token.AccessToken != "token-1" || len(token.Scopes) != 2 || !token.ExpiresAt.Equal(time.Unix(1785715200, 0)) {
		t.Fatalf("token=%#v err=%v", token, err)
	}
	refreshed, err := oauth.Refresh(context.Background(), "refresh-1")
	if err != nil || refreshed.AccessToken != "token-2" || refreshed.RefreshToken != "refresh-2" {
		t.Fatalf("refreshed=%#v err=%v", refreshed, err)
	}
}

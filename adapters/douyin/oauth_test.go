package douyin

import (
	"context"
	"mime"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestOAuthUserRefreshAndClientTokens(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		mediaType, _, _ := mime.ParseMediaType(request.Header.Get("Content-Type"))
		if mediaType == "multipart/form-data" {
			_ = request.ParseMultipartForm(1 << 20)
		} else {
			_ = request.ParseForm()
		}
		if request.Form.Get("client_key") != "client-key" {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		switch request.URL.Path {
		case "/oauth/access_token/":
			if mediaType != "application/x-www-form-urlencoded" || request.Form.Get("code") != "code-1" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"data":{"access_token":"user-token-1","refresh_token":"refresh-1","open_id":"open-id-1","expires_in":"1296000","refresh_expires_in":"2592000","scope":"user_info,video.list","error_code":"0"}}`))
		case "/oauth/refresh_token/":
			if !strings.HasPrefix(mediaType, "multipart/") || request.Form.Get("refresh_token") != "refresh-1" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"data":{"access_token":"user-token-2","refresh_token":"refresh-1","open_id":"open-id-1","expires_in":1296000,"refresh_expires_in":2592000,"scope":"user_info","error_code":0}}`))
		case "/oauth/client_token/":
			if request.Form.Get("grant_type") != "client_credential" || request.Form.Get("client_secret") != "client-secret" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"data":{"access_token":"client-token-1","expires_in":"7200","error_code":"0"}}`))
		case "/oauth/renew_refresh_token/":
			if request.Form.Get("refresh_token") != "refresh-1" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"data":{"refresh_token":"refresh-2","expires_in":"2592000","error_code":"0"}}`))
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
	authorizationURL, err := oauth.AuthorizationURL("https://app.example/callback", "state-1", []string{"user_info", "video.list"})
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(authorizationURL)
	if parsed.Query().Get("scope") != "user_info,video.list" || parsed.Query().Get("state") != "state-1" {
		t.Fatalf("authorization URL=%q", authorizationURL)
	}
	userToken, err := oauth.Exchange(context.Background(), "code-1")
	if err != nil || userToken.OpenID != "open-id-1" || userToken.Token.TokenType != "DouyinUser" || len(userToken.Token.Scopes) != 2 {
		t.Fatalf("user token=%#v err=%v", userToken, err)
	}
	refreshed, err := oauth.Refresh(context.Background(), "refresh-1")
	if err != nil || refreshed.Token.AccessToken != "user-token-2" {
		t.Fatalf("refreshed=%#v err=%v", refreshed, err)
	}
	clientToken, err := oauth.ClientToken(context.Background())
	if err != nil || clientToken.AccessToken != "client-token-1" || clientToken.TokenType != "DouyinClient" {
		t.Fatalf("client token=%#v err=%v", clientToken, err)
	}
	wantExpiry := time.Date(2026, 8, 1, 2, 0, 0, 0, time.UTC)
	if !clientToken.ExpiresAt.Equal(wantExpiry) {
		t.Fatalf("client expiry=%s", clientToken.ExpiresAt)
	}
	renewed, err := oauth.RenewRefreshToken(context.Background(), "refresh-1")
	if err != nil || renewed.Value != "refresh-2" {
		t.Fatalf("renewed refresh token=%#v err=%v", renewed, err)
	}
}

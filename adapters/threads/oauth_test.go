package threads

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestThreadsOAuthLifecycle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/oauth/access_token":
			if request.Method != http.MethodPost || request.ParseForm() != nil || request.Form.Get("client_id") != "threads-app-id" || request.Form.Get("client_secret") != "app-secret" || request.Form.Get("code") != "code-1" || request.Form.Get("redirect_uri") != "https://app.test/callback" || request.Form.Get("grant_type") != "authorization_code" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeTestJSON(t, writer, map[string]any{"access_token": "short-token", "user_id": 12345})
		case "/access_token":
			query := request.URL.Query()
			if request.Method != http.MethodGet || query.Get("grant_type") != "th_exchange_token" || query.Get("client_secret") != "app-secret" || query.Get("access_token") != "short-token" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeTestJSON(t, writer, map[string]any{"access_token": "long-token", "token_type": "bearer", "expires_in": 5_184_000})
		case "/refresh_access_token":
			query := request.URL.Query()
			if query.Get("grant_type") != "th_refresh_token" || query.Get("access_token") != "long-token" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeTestJSON(t, writer, map[string]any{"access_token": "refreshed-token", "expires_in": 5_184_000})
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := &OAuthClient{
		ClientID: "threads-app-id", ClientSecret: "app-secret", AuthURL: server.URL + "/oauth/authorize",
		TokenURL: server.URL + "/oauth/access_token", LongTokenURL: server.URL + "/access_token",
		RefreshURL: server.URL + "/refresh_access_token", HTTPClient: server.Client(), Clock: fixedClock{now: testNow},
	}

	authorizationURL, err := client.AuthorizationURL("https://app.test/callback", "state-1", []string{"threads_basic", "threads_content_publish"})
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(authorizationURL)
	query := parsed.Query()
	if query.Get("client_id") != "threads-app-id" || query.Get("redirect_uri") != "https://app.test/callback" || query.Get("state") != "state-1" || query.Get("scope") != "threads_basic,threads_content_publish" || query.Get("response_type") != "code" {
		t.Fatalf("authorization query=%v", query)
	}
	exchanged, err := client.Exchange(context.Background(), "code-1", "https://app.test/callback")
	if err != nil || exchanged.Token.AccessToken != "short-token" || exchanged.UserID != "12345" {
		t.Fatalf("exchange=%#v error=%v", exchanged, err)
	}
	longLived, err := client.ExchangeLongLived(context.Background(), exchanged.Token.AccessToken)
	if err != nil || longLived.AccessToken != "long-token" || longLived.TokenType != "bearer" || !longLived.ExpiresAt.Equal(testNow.Add(60*24*time.Hour)) {
		t.Fatalf("long-lived token=%#v error=%v", longLived, err)
	}
	refreshed, err := client.Refresh(context.Background(), longLived.AccessToken)
	if err != nil || refreshed.AccessToken != "refreshed-token" || refreshed.TokenType != "Bearer" || !refreshed.ExpiresAt.Equal(testNow.Add(60*24*time.Hour)) {
		t.Fatalf("refreshed token=%#v error=%v", refreshed, err)
	}
}

func TestOAuthValidationAndFailures(t *testing.T) {
	client := &OAuthClient{}
	if _, err := client.AuthorizationURL("", "", nil); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("authorization error=%v", err)
	}
	if _, err := client.Exchange(context.Background(), "", ""); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("exchange error=%v", err)
	}
	if _, err := client.ExchangeLongLived(context.Background(), ""); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("long-lived error=%v", err)
	}
	if _, err := client.Refresh(context.Background(), ""); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("refresh error=%v", err)
	}
	client = &OAuthClient{ClientID: "id", ClientSecret: "secret", HTTPClient: http.DefaultClient, Clock: fixedClock{now: testNow}}
	if _, err := client.Exchange(context.Background(), "code", "redirect"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("invalid endpoint error=%v", err)
	}
	if _, err := client.decodeToken([]byte(`{"expires_in":999999999}`), "test"); err == nil {
		t.Fatal("malformed token should fail")
	}
}

package instagram

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestInstagramOAuthFlows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/oauth/access_token":
			if request.Method != http.MethodPost || request.ParseForm() != nil || request.Form.Get("grant_type") != "authorization_code" || request.Form.Get("client_secret") != "app-secret" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, `{"access_token":"short","user_id":178,"permissions":["instagram_business_basic"]}`)
		case "/access_token":
			if request.URL.Query().Get("grant_type") != "ig_exchange_token" || request.URL.Query().Get("access_token") != "short" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, `{"access_token":"long","token_type":"bearer","expires_in":5184000}`)
		case "/refresh_access_token":
			if request.URL.Query().Get("grant_type") != "ig_refresh_token" || request.URL.Query().Get("access_token") != "long" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, `{"access_token":"refreshed","token_type":"bearer","expires_in":5184000}`)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	adapter, _ := newTestAdapter(t, server, nil, false)
	oauth, err := adapter.OAuth(context.Background(), "brand")
	if err != nil {
		t.Fatal(err)
	}
	authURL, err := oauth.AuthorizationURL("https://app.example/callback", "state", []string{"instagram_business_basic"})
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(authURL)
	if parsed.Query().Get("state") != "state" || parsed.Query().Get("scope") != "instagram_business_basic" {
		t.Fatalf("authorization URL=%s", authURL)
	}
	result, err := oauth.Exchange(context.Background(), "code", "https://app.example/callback")
	if err != nil || result.UserID != "178" || result.Token.AccessToken != "short" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	long, err := oauth.ExchangeLongLived(context.Background(), result.Token.AccessToken)
	if err != nil || long.AccessToken != "long" || time.Until(long.ExpiresAt) < 59*24*time.Hour {
		t.Fatalf("long=%#v err=%v", long, err)
	}
	refreshed, err := oauth.Refresh(context.Background(), long.AccessToken)
	if err != nil || refreshed.AccessToken != "refreshed" {
		t.Fatalf("refreshed=%#v err=%v", refreshed, err)
	}
}

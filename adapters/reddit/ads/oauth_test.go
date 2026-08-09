package ads

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestOAuthAuthorizationExchangeAndRefresh(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/token" || request.Header.Get("User-Agent") != testUserAgent {
			t.Fatalf("request=%s %s User-Agent=%q", request.Method, request.URL, request.Header.Get("User-Agent"))
		}
		wantBasic := "Basic " + base64.StdEncoding.EncodeToString([]byte("client-id:client-secret"))
		if request.Header.Get("Authorization") != wantBasic {
			t.Fatalf("Authorization=%q", request.Header.Get("Authorization"))
		}
		if err := request.ParseForm(); err != nil {
			t.Fatal(err)
		}
		switch request.Form.Get("grant_type") {
		case "authorization_code":
			if request.Form.Get("code") != "code" || request.Form.Get("redirect_uri") != "https://app.example.test/callback" {
				t.Fatalf("form=%v", request.Form)
			}
			writeJSON(t, writer, http.StatusOK, map[string]any{
				"access_token": "access-1", "refresh_token": "refresh-1", "token_type": "bearer", "expires_in": 3600, "scope": "adsread adsedit",
			})
		case "refresh_token":
			if request.Form.Get("refresh_token") != "refresh-1" {
				t.Fatalf("form=%v", request.Form)
			}
			writeJSON(t, writer, http.StatusOK, map[string]any{"access_token": "access-2", "token_type": "Bearer", "expires_in": 1800, "scope": "adsread"})
		default:
			t.Fatalf("form=%v", request.Form)
		}
	}))
	defer server.Close()
	adapter, _ := newTestAdapter(t, server)
	client, err := adapter.OAuth(context.Background(), "paid-social")
	if err != nil {
		t.Fatal(err)
	}
	authorize, err := client.AuthorizationURL("https://app.example.test/callback", "state", []string{readScope, editScope})
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(authorize)
	if parsed.Path != "/authorize" || parsed.Query().Get("duration") != "permanent" || parsed.Query().Get("scope") != "adsread adsedit" || parsed.Query().Get("state") != "state" {
		t.Fatalf("authorize=%s", authorize)
	}
	token, err := client.Exchange(context.Background(), "code", "https://app.example.test/callback")
	if err != nil || token.AccessToken != "access-1" || token.RefreshToken != "refresh-1" || token.TokenType != "Bearer" ||
		!token.ExpiresAt.Equal(testNow.Add(time.Hour)) || len(token.Scopes) != 2 {
		t.Fatalf("token=%#v err=%v", token, err)
	}
	token, err = client.Refresh(context.Background(), "refresh-1")
	if err != nil || token.AccessToken != "access-2" || token.RefreshToken != "refresh-1" || !token.ExpiresAt.Equal(testNow.Add(30*time.Minute)) {
		t.Fatalf("refreshed=%#v err=%v", token, err)
	}
}

func TestOAuthValidationErrorsAndRedaction(t *testing.T) {
	client := &OAuthClient{}
	if _, err := client.AuthorizationURL("bad", "", nil); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("authorization error=%v", err)
	}
	if _, err := client.Exchange(context.Background(), "", "bad"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("exchange error=%v", err)
	}
	if _, err := client.Refresh(context.Background(), " bad "); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("refresh error=%v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("x-request-id", "oauth-request")
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte(`{"error":"invalid_grant","error_description":"access_token=secret-token"}`))
	}))
	defer server.Close()
	client = &OAuthClient{
		ClientID: "client-id", ClientSecret: "client-secret", AuthURL: server.URL, TokenURL: server.URL,
		UserAgent: testUserAgent, HTTPClient: server.Client(), Clock: fixedClock{value: testNow},
	}
	_, err := client.Refresh(context.Background(), "refresh-token")
	hub := hubError(t, err)
	if !errors.Is(err, socialhub.ErrUnauthenticated) || hub.PlatformCode != "invalid_grant" || hub.RequestID != "oauth-request" ||
		strings.Contains(hub.PlatformMessage, "secret-token") || !strings.Contains(hub.PlatformMessage, "[REDACTED]") {
		t.Fatalf("OAuth error=%#v", hub)
	}
}

package marketing

import (
	"context"
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
		if request.Method != http.MethodPost || request.URL.Path != "/token" || request.ParseForm() != nil {
			t.Fatalf("token request=%s %s", request.Method, request.URL)
		}
		if request.Form.Get("client_id") != "snap-client-id" || request.Form.Get("client_secret") != "client-secret" ||
			request.Header.Get("Content-Type") != "application/x-www-form-urlencoded" || request.Header.Get("Accept") != "application/json" {
			t.Fatalf("form=%v headers=%v", request.Form, request.Header)
		}
		switch request.Form.Get("grant_type") {
		case "authorization_code":
			if request.Form.Get("code") != "auth-code" || request.Form.Get("redirect_uri") != "https://app.example/callback" {
				t.Fatalf("exchange form=%v", request.Form)
			}
			writeValue(t, writer, http.StatusOK, map[string]any{
				"access_token": "access-1", "refresh_token": "refresh-1", "token_type": "bearer",
				"expires_in": 3600, "scope": marketingScope,
			})
		case "refresh_token":
			if request.Form.Get("refresh_token") != "refresh-1" {
				t.Fatalf("refresh form=%v", request.Form)
			}
			writeValue(t, writer, http.StatusOK, map[string]any{
				"access_token": "access-2", "token_type": "Bearer", "expires_in": 1800, "scope": marketingScope,
			})
		default:
			t.Fatalf("form=%v", request.Form)
		}
	}))
	defer server.Close()
	adapter, _ := newTestAdapter(t, server)
	oauth, err := adapter.OAuth(context.Background(), "paid-social")
	if err != nil {
		t.Fatal(err)
	}
	authorize, err := oauth.AuthorizationURL("https://app.example/callback", "state-value")
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(authorize)
	query := parsed.Query()
	if query.Get("client_id") != "snap-client-id" || query.Get("redirect_uri") != "https://app.example/callback" ||
		query.Get("response_type") != "code" || query.Get("scope") != marketingScope || query.Get("state") != "state-value" {
		t.Fatalf("authorization URL=%s", authorize)
	}
	exchanged, err := oauth.Exchange(context.Background(), "auth-code", "https://app.example/callback")
	if err != nil || exchanged.AccessToken != "access-1" || exchanged.RefreshToken != "refresh-1" || exchanged.TokenType != "Bearer" ||
		!exchanged.ExpiresAt.Equal(testNow.Add(time.Hour)) || len(exchanged.Scopes) != 1 || exchanged.Scopes[0] != marketingScope {
		t.Fatalf("exchange=%#v err=%v", exchanged, err)
	}
	refreshed, err := oauth.Refresh(context.Background(), "refresh-1")
	if err != nil || refreshed.AccessToken != "access-2" || refreshed.RefreshToken != "refresh-1" ||
		!refreshed.ExpiresAt.Equal(testNow.Add(30*time.Minute)) {
		t.Fatalf("refresh=%#v err=%v", refreshed, err)
	}
}

func TestOAuthValidationAndFailures(t *testing.T) {
	client := &OAuthClient{}
	invalidCalls := []func() error{
		func() error { _, err := client.AuthorizationURL("bad", ""); return err },
		func() error { _, err := client.Exchange(context.Background(), "", "bad"); return err },
		func() error { _, err := client.Refresh(context.Background(), ""); return err },
		func() error {
			client.ClientID, client.ClientSecret = "client", "secret"
			client.AuthURL, client.TokenURL = defaultAuthURL, defaultTokenURL
			_, err := client.Exchange(context.Background(), "code", "https://app.example/callback")
			return err
		},
	}
	for index, call := range invalidCalls {
		if err := call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Errorf("call %d error=%v", index, err)
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("X-Request-ID", "oauth-request")
		writeValue(t, writer, http.StatusUnauthorized, map[string]any{
			"error": "invalid_token", "error_description": "access_token=secret-value expired",
		})
	}))
	defer server.Close()
	adapter, _ := newTestAdapter(t, server)
	oauth, err := adapter.OAuth(context.Background(), "paid-social")
	if err != nil {
		t.Fatal(err)
	}
	_, err = oauth.Exchange(context.Background(), "code", "https://app.example/callback")
	hub := hubError(t, err)
	if !errors.Is(err, socialhub.ErrUnauthenticated) || hub.RequestID != "oauth-request" ||
		strings.Contains(hub.PlatformMessage, "secret-value") || !strings.Contains(hub.PlatformMessage, "[REDACTED]") {
		t.Fatalf("OAuth HTTP error=%#v", hub)
	}
}

func TestOAuthSuccessPayloadErrorsAndContracts(t *testing.T) {
	responses := []map[string]any{
		{"error": "invalid_grant", "error_description": "authorization failed"},
		{"access_token": "", "expires_in": 3600},
		{"access_token": "access", "refresh_token": " bad", "expires_in": 3600},
		{"access_token": "access", "expires_in": 90000},
	}
	for index, payload := range responses {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writeValue(t, writer, http.StatusOK, payload)
		}))
		adapter, _ := newTestAdapter(t, server)
		oauth, err := adapter.OAuth(context.Background(), "paid-social")
		if err != nil {
			t.Fatal(err)
		}
		_, err = oauth.Exchange(context.Background(), "code", "https://app.example/callback")
		server.Close()
		if err == nil {
			t.Errorf("payload %d accepted", index)
		}
	}
}

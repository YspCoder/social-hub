package marketing

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

func TestOAuthAuthorizationExchangeAndPartnerRefresh(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/token" || request.ParseForm() != nil {
			t.Fatalf("token request=%s %s", request.Method, request.URL)
		}
		if request.Header.Get("Accept") != "application/json" || request.Header.Get("Content-Type") != "application/x-www-form-urlencoded" ||
			request.Form.Get("client_id") != "linkedin-client-id" || request.Form.Get("client_secret") != "client-secret" {
			t.Fatalf("headers=%v form=%v", request.Header, request.Form)
		}
		switch request.Form.Get("grant_type") {
		case "authorization_code":
			if request.Form.Get("code") != "auth-code" || request.Form.Get("redirect_uri") != "https://app.example/callback" {
				t.Fatalf("exchange form=%v", request.Form)
			}
			writeValue(t, writer, http.StatusOK, map[string]any{
				"access_token": "access-1", "refresh_token": "refresh-1", "token_type": "bearer",
				"expires_in": 3600, "scope": "r_ads rw_ads r_ads_reporting",
			})
		case "refresh_token":
			if request.Form.Get("refresh_token") != "refresh-1" {
				t.Fatalf("refresh form=%v", request.Form)
			}
			writeValue(t, writer, http.StatusOK, map[string]any{
				"access_token": "access-2", "expires_in": 1800, "scope": "r_ads,r_ads_reporting",
			})
		default:
			t.Fatalf("form=%v", request.Form)
		}
	}))
	defer server.Close()

	adapter, _ := newTestAdapter(t, server)
	oauth, err := adapter.OAuth(context.Background(), "b2b-demand")
	if err != nil {
		t.Fatal(err)
	}
	authorize, err := oauth.AuthorizationURL("https://app.example/callback", "state-value", []string{readAdsScope, writeAdsScope, reportingAdsScope})
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(authorize)
	query := parsed.Query()
	if query.Get("client_id") != "linkedin-client-id" || query.Get("redirect_uri") != "https://app.example/callback" ||
		query.Get("response_type") != "code" || query.Get("scope") != "r_ads rw_ads r_ads_reporting" || query.Get("state") != "state-value" {
		t.Fatalf("authorization URL=%s", authorize)
	}
	token, err := oauth.Exchange(context.Background(), "auth-code", "https://app.example/callback")
	if err != nil || token.AccessToken != "access-1" || token.RefreshToken != "refresh-1" || token.TokenType != "Bearer" ||
		!token.ExpiresAt.Equal(testNow.Add(time.Hour)) || len(token.Scopes) != 3 {
		t.Fatalf("exchange=%#v err=%v", token, err)
	}
	refreshed, err := oauth.Refresh(context.Background(), token.RefreshToken)
	if err != nil || refreshed.AccessToken != "access-2" || refreshed.RefreshToken != "refresh-1" ||
		!refreshed.ExpiresAt.Equal(testNow.Add(30*time.Minute)) || len(refreshed.Scopes) != 2 {
		t.Fatalf("refresh=%#v err=%v", refreshed, err)
	}
}

func TestOAuthValidationAndErrorRedaction(t *testing.T) {
	broken := &OAuthClient{}
	invalidCalls := []func() error{
		func() error { _, err := broken.AuthorizationURL("bad", "", nil); return err },
		func() error { _, err := broken.Exchange(context.Background(), "", "bad"); return err },
		func() error { _, err := broken.Refresh(context.Background(), ""); return err },
		func() error {
			_, err := broken.AuthorizationURL("https://app.example/callback", "state", []string{"openid"})
			return err
		},
		func() error {
			_, err := broken.Exchange(context.Background(), "code", "https://app.example/callback")
			return err
		},
	}
	for index, call := range invalidCalls {
		if err := call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Errorf("call %d error=%v", index, err)
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("x-li-uuid", "oauth-request")
		writeValue(t, writer, http.StatusUnauthorized, map[string]any{
			"error": "invalid_grant", "error_description": "authorization access_token=secret-value expired",
		})
	}))
	defer server.Close()
	adapter, _ := newTestAdapter(t, server)
	oauth, err := adapter.OAuth(context.Background(), "b2b-demand")
	if err != nil {
		t.Fatal(err)
	}
	_, err = oauth.Exchange(context.Background(), "code", "https://app.example/callback")
	hub := hubError(t, err)
	if !errors.Is(err, socialhub.ErrUnauthenticated) || hub.RequestID != "oauth-request" ||
		hub.PlatformMessage == "" || hub.PlatformMessage == "authorization access_token=secret-value expired" {
		t.Fatalf("OAuth HTTP error=%#v", hub)
	}
}

package ads

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

func TestOAuthAuthorizationAndTokenGrants(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/token" || request.ParseForm() != nil {
			t.Fatalf("token request=%s %s", request.Method, request.URL)
		}
		clientID, secret, ok := request.BasicAuth()
		if !ok || clientID != "pinterest-app-id" || secret != "client-secret" {
			t.Fatalf("basic auth=%q %q %t", clientID, secret, ok)
		}
		if request.Header.Get("Accept") != "application/json" || request.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Fatalf("headers=%v", request.Header)
		}
		switch request.Form.Get("grant_type") {
		case "authorization_code":
			if request.Form.Get("code") != "auth-code" || request.Form.Get("redirect_uri") != "https://app.example/callback" {
				t.Fatalf("exchange form=%v", request.Form)
			}
			writeJSON(writer, http.StatusOK, `{"access_token":"access-1","refresh_token":"refresh-1","token_type":"bearer","response_type":"authorization_code","expires_in":3600,"refresh_token_expires_in":7200,"scope":"ads:read,ads:write"}`)
		case "refresh_token":
			if request.Form.Get("refresh_token") != "refresh-1" || request.Form.Get("scope") != "ads:read" {
				t.Fatalf("refresh form=%v", request.Form)
			}
			writeJSON(writer, http.StatusOK, `{"access_token":"access-2","expires_in":1800,"refresh_token_expires_at":2000000000,"scope":"ads:read"}`)
		case "client_credentials":
			if request.Form.Get("scope") != "ads:read,ads:write" {
				t.Fatalf("client credentials form=%v", request.Form)
			}
			writeJSON(writer, http.StatusOK, `{"access_token":"access-3","token_type":"Bearer","expires_in":900,"scope":"ads:read ads:write"}`)
		default:
			t.Fatalf("form=%v", request.Form)
		}
	}))
	defer server.Close()

	adapter, _ := newTestAdapter(t, server)
	oauth, err := adapter.OAuth(context.Background(), "visual-commerce")
	if err != nil {
		t.Fatal(err)
	}
	authorize, err := oauth.AuthorizationURL("https://app.example/callback", "state-value", []string{adsReadScope, adsWriteScope})
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(authorize)
	query := parsed.Query()
	if query.Get("client_id") != "pinterest-app-id" || query.Get("redirect_uri") != "https://app.example/callback" ||
		query.Get("response_type") != "code" || query.Get("scope") != "ads:read,ads:write" || query.Get("state") != "state-value" {
		t.Fatalf("authorization URL=%s", authorize)
	}

	exchanged, err := oauth.Exchange(context.Background(), "auth-code", "https://app.example/callback")
	if err != nil || exchanged.Token.AccessToken != "access-1" || exchanged.Token.RefreshToken != "refresh-1" ||
		exchanged.Token.TokenType != "Bearer" || !exchanged.Token.ExpiresAt.Equal(testNow.Add(time.Hour)) ||
		!exchanged.RefreshExpiresAt.Equal(testNow.Add(2*time.Hour)) || exchanged.ResponseType != "authorization_code" {
		t.Fatalf("exchange=%#v err=%v", exchanged, err)
	}
	refreshed, err := oauth.Refresh(context.Background(), "refresh-1", []string{adsReadScope})
	if err != nil || refreshed.Token.AccessToken != "access-2" || refreshed.Token.RefreshToken != "refresh-1" ||
		!refreshed.RefreshExpiresAt.Equal(time.Unix(2000000000, 0)) {
		t.Fatalf("refresh=%#v err=%v", refreshed, err)
	}
	service, err := oauth.ClientCredentials(context.Background(), []string{adsReadScope, adsWriteScope})
	if err != nil || service.Token.AccessToken != "access-3" || len(service.Token.Scopes) != 2 || !service.RefreshExpiresAt.IsZero() {
		t.Fatalf("client credentials=%#v err=%v", service, err)
	}
}

func TestOAuthValidationAndFailures(t *testing.T) {
	broken := &OAuthClient{}
	invalidCalls := []func() error{
		func() error { _, err := broken.AuthorizationURL("bad", "", nil); return err },
		func() error { _, err := broken.Exchange(context.Background(), "", "bad"); return err },
		func() error { _, err := broken.Refresh(context.Background(), "", nil); return err },
		func() error {
			_, err := broken.Refresh(context.Background(), "refresh", []string{"pins:read"})
			return err
		},
		func() error {
			_, err := broken.ClientCredentials(context.Background(), []string{adsReadScope, adsReadScope})
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
		writer.Header().Set("x-pinterest-rid", "oauth-request")
		writeJSON(writer, http.StatusUnauthorized, `{"code":29,"message":"authorization access_token=secret-value expired"}`)
	}))
	defer server.Close()
	adapter, _ := newTestAdapter(t, server)
	oauth, err := adapter.OAuth(context.Background(), "visual-commerce")
	if err != nil {
		t.Fatal(err)
	}
	_, err = oauth.Exchange(context.Background(), "code", "https://app.example/callback")
	hub := hubError(t, err)
	if !errors.Is(err, socialhub.ErrUnauthenticated) || hub.RequestID != "oauth-request" || hub.PlatformMessage == "" || hub.PlatformMessage == "authorization access_token=secret-value expired" {
		t.Fatalf("OAuth HTTP error=%#v", hub)
	}
}

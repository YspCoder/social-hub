package amazonads

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

func TestOAuthAuthorizationExchangeAndRefresh(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.Header.Get("Content-Type") != "application/x-www-form-urlencoded" || request.ParseForm() != nil {
			t.Fatalf("token request=%s %s", request.Method, request.URL)
		}
		if request.Form.Get("client_id") != "amzn1.application-oa2-client.test" || request.Form.Get("client_secret") != "client-secret" {
			t.Errorf("credentials=%v", request.Form)
		}
		switch request.Form.Get("grant_type") {
		case "authorization_code":
			writeJSON(writer, http.StatusOK, `{"access_token":"access-1","expires_in":3600,"refresh_token":"refresh-1","scope":"advertising::campaign_management","token_type":"bearer"}`)
		case "refresh_token":
			writeJSON(writer, http.StatusOK, `{"access_token":"access-2","expires_in":3600,"token_type":"Bearer"}`)
		default:
			t.Fatalf("form=%v", request.Form)
		}
	}))
	defer server.Close()
	adapter, _ := newTestAdapter(t, server)
	oauth, err := adapter.OAuth(context.Background(), "retail-us")
	if err != nil {
		t.Fatal(err)
	}
	authorize, err := oauth.AuthorizationURL("https://app.example/callback", "state-value")
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(authorize)
	query := parsed.Query()
	if query.Get("client_id") == "" || query.Get("scope") != managementScope || query.Get("response_type") != "code" || query.Get("state") != "state-value" {
		t.Fatalf("authorize=%s", authorize)
	}
	token, err := oauth.Exchange(context.Background(), "auth-code", "https://app.example/callback")
	if err != nil || token.AccessToken != "access-1" || token.RefreshToken != "refresh-1" || !token.ExpiresAt.Equal(testNow.Add(time.Hour)) {
		t.Fatalf("exchange=%#v err=%v", token, err)
	}
	token, err = oauth.Refresh(context.Background(), "refresh-1")
	if err != nil || token.AccessToken != "access-2" || token.RefreshToken != "refresh-1" {
		t.Fatalf("refresh=%#v err=%v", token, err)
	}
}

func TestOAuthErrorsAndValidation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writeJSON(writer, http.StatusBadRequest, `{"error":"invalid_grant","error_description":"expired"}`)
	}))
	defer server.Close()
	adapter, _ := newTestAdapter(t, server)
	oauth, err := adapter.OAuth(context.Background(), "retail-us")
	if err != nil {
		t.Fatal(err)
	}
	_, err = oauth.Exchange(context.Background(), "code", "https://app.example/callback")
	if !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("OAuth error=%v", err)
	}
	broken := &OAuthClient{}
	for _, invoke := range []func() error{
		func() error { _, err := broken.AuthorizationURL("bad", ""); return err },
		func() error { _, err := broken.Exchange(context.Background(), "", "bad"); return err },
		func() error { _, err := broken.Refresh(context.Background(), ""); return err },
		func() error {
			_, err := broken.Exchange(context.Background(), "code", "https://app.example/callback")
			return err
		},
	} {
		if err := invoke(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("validation=%v", err)
		}
	}
}

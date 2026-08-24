package googleads

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
		if request.Method != http.MethodPost || request.Header.Get("Content-Type") != "application/x-www-form-urlencoded" || request.ParseForm() != nil {
			t.Fatalf("token request=%s %s", request.Method, request.URL)
		}
		if request.Form.Get("client_id") != "client-id" || request.Form.Get("client_secret") != "client-secret" {
			t.Errorf("credentials=%v", request.Form)
		}
		switch request.Form.Get("grant_type") {
		case "authorization_code":
			writeJSON(writer, http.StatusOK, `{"access_token":"access-1","expires_in":3600,"refresh_token":"refresh-1","scope":"`+adwordsScope+`","token_type":"Bearer"}`)
		case "refresh_token":
			writeJSON(writer, http.StatusOK, `{"access_token":"access-2","expires_in":3600,"scope":"`+adwordsScope+`","token_type":"bearer"}`)
		default:
			t.Fatalf("form=%v", request.Form)
		}
	}))
	defer server.Close()
	adapter, _ := newTestAdapter(t, server)
	oauth, err := adapter.OAuth(context.Background(), "brand-search")
	if err != nil {
		t.Fatal(err)
	}
	authorize, err := oauth.AuthorizationURL("https://app.example/callback", "state-value")
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(authorize)
	query := parsed.Query()
	if query.Get("client_id") != "client-id" || query.Get("scope") != adwordsScope || query.Get("state") != "state-value" ||
		query.Get("access_type") != "offline" || query.Get("include_granted_scopes") != "true" || query.Get("prompt") != "consent" {
		t.Fatalf("authorize=%s", authorize)
	}
	token, err := oauth.Exchange(context.Background(), "auth-code", "https://app.example/callback")
	if err != nil || token.AccessToken != "access-1" || token.RefreshToken != "refresh-1" ||
		!token.ExpiresAt.Equal(testNow.Add(time.Hour)) || len(token.Scopes) != 1 {
		t.Fatalf("exchange=%#v err=%v", token, err)
	}
	token, err = oauth.Refresh(context.Background(), "refresh-1")
	if err != nil || token.AccessToken != "access-2" || token.RefreshToken != "refresh-1" || token.TokenType != "Bearer" {
		t.Fatalf("refresh=%#v err=%v", token, err)
	}
}

func TestOAuthErrorsAndMalformedResponses(t *testing.T) {
	responses := []struct {
		status int
		body   string
		code   socialhub.ErrorCode
	}{
		{http.StatusBadRequest, `{"error":"invalid_grant","error_description":"expired"}`, socialhub.CodeUnauthenticated},
		{http.StatusServiceUnavailable, `{"error":"temporarily_unavailable","error_description":"retry"}`, socialhub.CodeTemporarilyUnavailable},
		{http.StatusOK, `not-json`, socialhub.CodePlatformError},
		{http.StatusOK, `{"access_token":"a","refresh_token":"r","expires_in":0}`, socialhub.CodePlatformError},
		{http.StatusBadRequest, `not-json`, socialhub.CodeInvalidArgument},
	}
	for index, test := range responses {
		t.Run(string(rune('a'+index)), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				writeJSON(writer, test.status, test.body)
			}))
			defer server.Close()
			adapter, _ := newTestAdapter(t, server)
			oauth, err := adapter.OAuth(context.Background(), "brand-search")
			if err != nil {
				t.Fatal(err)
			}
			_, err = oauth.Exchange(context.Background(), "code", "https://app.example/callback")
			if hubError(t, err).Code != test.code {
				t.Fatalf("error=%v", err)
			}
		})
	}

	client := &OAuthClient{}
	for _, invoke := range []func() error{
		func() error { _, err := client.AuthorizationURL("bad", ""); return err },
		func() error { _, err := client.Exchange(context.Background(), "", "bad"); return err },
		func() error { _, err := client.Refresh(context.Background(), ""); return err },
		func() error {
			_, err := client.Exchange(context.Background(), "code", "https://app.example/callback")
			return err
		},
	} {
		if err := invoke(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("validation error=%v", err)
		}
	}
	if validCallbackURL("ftp://example.com/callback") != true || validCallbackURL("https://user:pass@example.com") {
		t.Fatal("callback URL contract failed")
	}
	if boundedMessage(strings.Repeat("界", 30), 20) != strings.Repeat("界", 20) {
		t.Fatal("bounded message failed")
	}
}

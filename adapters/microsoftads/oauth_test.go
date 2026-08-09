package microsoftads

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

func TestOAuthAuthorizationExchangeRefreshAndScope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/token" || request.Method != http.MethodPost || request.ParseForm() != nil {
			t.Fatalf("token request=%s %s", request.Method, request.URL)
		}
		if request.Form.Get("client_id") != "client-id" || request.Form.Get("client_secret") != "client-secret" || request.Form.Get("scope") != oauthScopes {
			t.Fatalf("token form=%v", request.Form)
		}
		switch request.Form.Get("grant_type") {
		case "authorization_code":
			writeValue(t, writer, http.StatusOK, map[string]any{"access_token": "access-1", "expires_in": 3600, "refresh_token": "refresh-1", "scope": oauthScopes, "token_type": "Bearer"})
		case "refresh_token":
			writeValue(t, writer, http.StatusOK, map[string]any{"access_token": "access-2", "expires_in": 3600, "scope": oauthScopes, "token_type": "bearer"})
		default:
			t.Fatalf("grant=%q", request.Form.Get("grant_type"))
		}
	}))
	defer server.Close()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server.URL),
		socialhub.WithHTTPClient(server.Client()), socialhub.WithClock(fixedClock{now: testNow}),
		socialhub.WithSecretResolver(mapResolver{"test://client-secret": "client-secret"}),
	); err != nil {
		t.Fatal(err)
	}
	oauth, err := adapter.OAuth(context.Background(), "brand-search")
	if err != nil {
		t.Fatal(err)
	}
	authorize, err := oauth.AuthorizationURL("https://app.example/callback", "state-value")
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(authorize)
	if parsed.Query().Get("scope") != oauthScopes || parsed.Query().Get("state") != "state-value" || parsed.Query().Get("response_mode") != "query" {
		t.Fatalf("authorize=%s", authorize)
	}
	token, err := oauth.Exchange(context.Background(), "auth-code", "https://app.example/callback")
	if err != nil || token.AccessToken != "access-1" || token.RefreshToken != "refresh-1" || !token.ExpiresAt.Equal(testNow.Add(time.Hour)) {
		t.Fatalf("exchange=%#v err=%v", token, err)
	}
	token, err = oauth.Refresh(context.Background(), "refresh-1")
	if err != nil || token.AccessToken != "access-2" || token.RefreshToken != "refresh-1" || token.TokenType != "Bearer" {
		t.Fatalf("refresh=%#v err=%v", token, err)
	}
}

func TestOAuthErrorsValidationAndRedirect(t *testing.T) {
	tests := []struct {
		status int
		body   any
		code   socialhub.ErrorCode
	}{
		{http.StatusBadRequest, map[string]any{"error": "invalid_grant", "error_description": "refresh_token: secret"}, socialhub.CodeUnauthenticated},
		{http.StatusServiceUnavailable, map[string]any{"error": "temporarily_unavailable", "error_description": "retry"}, socialhub.CodeTemporarilyUnavailable},
		{http.StatusOK, "not-json", socialhub.CodePlatformError},
		{http.StatusOK, map[string]any{"access_token": "a", "refresh_token": "r", "expires_in": 0}, socialhub.CodePlatformError},
	}
	for _, test := range tests {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if text, ok := test.body.(string); ok {
				writer.WriteHeader(test.status)
				_, _ = writer.Write([]byte(text))
				return
			}
			writeValue(t, writer, test.status, test.body)
		}))
		adapter := &Adapter{}
		if err := adapter.Init(context.Background(), testConfig(server.URL),
			socialhub.WithHTTPClient(server.Client()), socialhub.WithClock(fixedClock{now: testNow}),
			socialhub.WithSecretResolver(mapResolver{"test://client-secret": "client-secret"}),
		); err != nil {
			t.Fatal(err)
		}
		oauth, err := adapter.OAuth(context.Background(), "brand-search")
		if err != nil {
			t.Fatal(err)
		}
		_, err = oauth.Exchange(context.Background(), "code", "https://app.example/callback")
		if hubError(t, err).Code != test.code {
			t.Errorf("error=%v", err)
		}
		server.Close()
	}

	client := &OAuthClient{}
	for _, call := range []func() error{
		func() error { _, err := client.AuthorizationURL("bad", ""); return err },
		func() error { _, err := client.Exchange(context.Background(), "", "bad"); return err },
		func() error { _, err := client.Refresh(context.Background(), ""); return err },
		func() error {
			_, err := client.Exchange(context.Background(), "code", "https://app.example/callback")
			return err
		},
	} {
		if err := call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Errorf("validation error=%v", err)
		}
	}

	var targetHits int
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { targetHits++ }))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, target.URL, http.StatusFound)
	}))
	defer redirect.Close()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(redirect.URL),
		socialhub.WithHTTPClient(redirect.Client()), socialhub.WithClock(fixedClock{now: testNow}),
		socialhub.WithSecretResolver(mapResolver{"test://client-secret": "client-secret"}),
	); err != nil {
		t.Fatal(err)
	}
	oauth, _ := adapter.OAuth(context.Background(), "brand-search")
	if _, err := oauth.Exchange(context.Background(), "code", "https://app.example/callback"); err == nil || targetHits != 0 {
		t.Fatalf("redirect error=%v target hits=%d", err, targetHits)
	}
}

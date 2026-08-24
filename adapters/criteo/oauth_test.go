package criteo

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestOAuthClientCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/oauth2/token" {
			t.Fatalf("request=%s %s", request.Method, request.URL.Path)
		}
		if err := request.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if request.Form.Get("client_id") != "client-id" || request.Form.Get("client_secret") != "client-secret" ||
			request.Form.Get("grant_type") != "client_credentials" || len(request.Form) != 3 {
			t.Fatalf("form=%v", request.Form)
		}
		if request.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Fatalf("Content-Type=%q", request.Header.Get("Content-Type"))
		}
		writeJSON(t, writer, http.StatusOK, map[string]any{
			"access_token": "managed-token", "token_type": "bearer", "expires_in": 900,
		})
	}))
	defer server.Close()
	clock := &mutableClock{value: testNow}
	client := OAuthClient{
		ClientID: "client-id", ClientSecret: "client-secret", TokenURL: server.URL + "/oauth2/token",
		HTTPClient: server.Client(), Clock: clock,
	}
	token, err := client.ClientCredentials(context.Background())
	if err != nil || token.AccessToken != "managed-token" || token.TokenType != "Bearer" ||
		!token.ExpiresAt.Equal(testNow.Add(15*time.Minute)) || len(token.Scopes) != len(managedScopes) {
		t.Fatalf("token=%#v err=%v", token, err)
	}
}

func TestManagedTokenCacheAndOAuthHelper(t *testing.T) {
	var tokenCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/oauth2/token":
			call := tokenCalls.Add(1)
			writeJSON(t, writer, http.StatusOK, map[string]any{
				"access_token": "managed-" + string(rune('0'+call)), "token_type": "Bearer", "expires_in": 900,
			})
		case advertisersMePath:
			if request.Header.Get("Authorization") == "" {
				t.Error("missing authorization")
			}
			writeJSON(t, writer, http.StatusOK, successEnvelope([]any{map[string]any{
				"type": "advertiser", "id": testAdvertiserID,
				"attributes": map[string]any{"advertiserName": "Example"},
			}}))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	clock := &mutableClock{value: testNow}
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), managedConfig(server.URL),
		socialhub.WithHTTPClient(server.Client()), socialhub.WithClock(clock),
		socialhub.WithSecretResolver(mapResolver{"test://client-secret": "client-secret"}),
		socialhub.WithTokenStore(socialhub.NewMemoryTokenStore()),
	); err != nil {
		t.Fatal(err)
	}
	oauth, err := adapter.OAuth(context.Background(), testAccountID)
	if err != nil || oauth.ClientID != "client-id" || oauth.ClientSecret != "client-secret" {
		t.Fatalf("oauth=%#v err=%v", oauth, err)
	}
	common, err := adapter.Client(context.Background(), testAccountID)
	if err != nil {
		t.Fatal(err)
	}
	client := common.(*Client)
	var wait sync.WaitGroup
	for range 12 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			if _, err := client.ValidateConfiguredAdvertiser(context.Background()); err != nil {
				t.Errorf("validate: %v", err)
			}
		}()
	}
	wait.Wait()
	if tokenCalls.Load() != 1 {
		t.Fatalf("token calls=%d", tokenCalls.Load())
	}
	clock.Set(testNow.Add(14 * time.Minute))
	if _, err := client.ValidateConfiguredAdvertiser(context.Background()); err != nil {
		t.Fatal(err)
	}
	if tokenCalls.Load() != 2 {
		t.Fatalf("token refresh calls=%d", tokenCalls.Load())
	}
}

func TestOAuthErrorsAndValidation(t *testing.T) {
	tests := []struct {
		name   string
		body   any
		status int
		want   error
	}{
		{"invalid client", map[string]any{"error": "invalid_client", "error_description": "bad secret"}, http.StatusUnauthorized, socialhub.ErrUnauthenticated},
		{"rate", map[string]any{"error": "slow_down"}, http.StatusTooManyRequests, socialhub.ErrRateLimited},
		{"malformed token", map[string]any{"access_token": "", "expires_in": 900}, http.StatusOK, nil},
		{"long expiry", map[string]any{"access_token": "token", "expires_in": 999999}, http.StatusOK, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writeJSON(t, writer, test.status, test.body)
			}))
			defer server.Close()
			client := OAuthClient{ClientID: "id", ClientSecret: "secret", TokenURL: server.URL, HTTPClient: server.Client(), Clock: &mutableClock{value: testNow}}
			_, err := client.ClientCredentials(context.Background())
			if test.want != nil && !errors.Is(err, test.want) {
				t.Fatalf("error=%v", err)
			}
			if test.want == nil && !errors.Is(err, socialhub.ErrUnavailable) && requireHubError(t, err).Code != socialhub.CodePlatformError {
				t.Fatalf("error=%v", err)
			}
		})
	}
	invalid := OAuthClient{}
	if _, err := invalid.ClientCredentials(context.Background()); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("validation error=%v", err)
	}
}

func TestOAuthRedirectIsRejectedByAdapter(t *testing.T) {
	receiver := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		t.Errorf("credential-bearing redirect followed: %s", request.URL)
	}))
	defer receiver.Close()
	redirector := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Location", receiver.URL)
		writer.WriteHeader(http.StatusFound)
	}))
	defer redirector.Close()
	config := managedConfig(redirector.URL)
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), config,
		socialhub.WithHTTPClient(redirector.Client()),
		socialhub.WithSecretResolver(mapResolver{"test://client-secret": "client-secret"}),
	); err != nil {
		t.Fatal(err)
	}
	oauth, err := adapter.OAuth(context.Background(), testAccountID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = oauth.ClientCredentials(context.Background())
	if hub := requireHubError(t, err); hub.HTTPStatus != http.StatusFound {
		t.Fatalf("error=%#v", hub)
	}
}

package taboola

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestOAuthClientCredentialsWireContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/backstage/oauth/token" || request.URL.RawQuery != "" {
			t.Fatalf("request=%s %s", request.Method, request.URL)
		}
		if request.Header.Get("Content-Type") != "application/x-www-form-urlencoded" || request.Header.Get("Authorization") != "" {
			t.Fatalf("headers=%v", request.Header)
		}
		if err := request.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if request.Form.Get("client_id") != "client-id" || request.Form.Get("client_secret") != "client-secret" || request.Form.Get("grant_type") != "client_credentials" || len(request.Form) != 3 {
			t.Fatalf("form=%v", request.Form)
		}
		writeJSON(t, writer, http.StatusOK, map[string]any{"access_token": "issued-token", "token_type": "bearer", "expires_in": 43200})
	}))
	defer server.Close()
	client := OAuthClient{
		ClientID: "client-id", ClientSecret: "client-secret", TokenURL: server.URL + "/backstage/oauth/token",
		HTTPClient: cloneHTTPClient(server.Client()), Clock: fixedClock{value: testNow},
	}
	token, err := client.ClientCredentials(context.Background())
	if err != nil || token.AccessToken != "issued-token" || token.TokenType != "Bearer" || token.RefreshToken != "" || !token.ExpiresAt.Equal(testNow.Add(12*time.Hour)) {
		t.Fatalf("token=%#v err=%v", token, err)
	}
}

func TestOAuthErrorsValidationAndRedirects(t *testing.T) {
	for index, client := range []OAuthClient{
		{},
		{ClientID: "id", ClientSecret: "secret", TokenURL: "https://example.test/token/", HTTPClient: http.DefaultClient, Clock: fixedClock{value: testNow}},
		{ClientID: "id\n", ClientSecret: "secret", TokenURL: "https://example.test/token", HTTPClient: http.DefaultClient, Clock: fixedClock{value: testNow}},
	} {
		if _, err := client.ClientCredentials(context.Background()); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Errorf("case %d error=%v", index, err)
		}
	}

	tests := []struct {
		name        string
		status      int
		contentType string
		body        string
		code        socialhub.ErrorCode
	}{
		{"XML invalid client", http.StatusBadRequest, "application/xml", `<BadClientCredentialsException><error>invalid_client</error><error_description>Bad client credentials client_secret=hidden</error_description></BadClientCredentialsException>`, socialhub.CodeUnauthenticated},
		{"JSON rate limit", http.StatusTooManyRequests, "application/json", `{"error":"slow_down","error_description":"access_token=hidden"}`, socialhub.CodeRateLimited},
		{"HTML forbidden", http.StatusForbidden, "text/html", `<html><title>authorization Bearer hidden</title></html>`, socialhub.CodePermissionDenied},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				writer.Header().Set("Content-Type", test.contentType)
				writer.Header().Set("Retry-After", "9")
				writer.WriteHeader(test.status)
				_, _ = io.WriteString(writer, test.body)
			}))
			defer server.Close()
			client := OAuthClient{ClientID: "id", ClientSecret: "secret", TokenURL: server.URL, HTTPClient: cloneHTTPClient(server.Client()), Clock: fixedClock{value: testNow}}
			_, err := client.ClientCredentials(context.Background())
			hub := hubError(t, err)
			if hub.Code != test.code || strings.Contains(strings.ToLower(hub.PlatformMessage), "hidden") || strings.Contains(strings.ToLower(err.Error()), "hidden") {
				t.Fatalf("hub=%#v error=%v", hub, err)
			}
			if test.status == http.StatusTooManyRequests && hub.RetryAfter != 9*time.Second {
				t.Fatalf("RetryAfter=%s", hub.RetryAfter)
			}
		})
	}

	targetCalls := atomic.Int32{}
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { targetCalls.Add(1) }))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, target.URL, http.StatusFound)
	}))
	defer redirect.Close()
	client := OAuthClient{ClientID: "id", ClientSecret: "secret", TokenURL: redirect.URL, HTTPClient: cloneHTTPClient(redirect.Client()), Clock: fixedClock{value: testNow}}
	if _, err := client.ClientCredentials(context.Background()); err == nil || targetCalls.Load() != 0 {
		t.Fatalf("redirect error=%v target calls=%d", err, targetCalls.Load())
	}
}

func TestOAuthResponseValidationAndBounds(t *testing.T) {
	bodies := []string{
		`not-json`,
		`{"access_token":"","token_type":"bearer","expires_in":43200}`,
		`{"access_token":"token","token_type":"mac","expires_in":43200}`,
		`{"access_token":"token","token_type":"bearer","expires_in":0}`,
		`{"access_token":"token","token_type":"bearer","expires_in":90000}`,
		strings.Repeat("x", int(maxOAuthResponseBytes)+1),
	}
	for index, body := range bodies {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(writer, body)
		}))
		client := OAuthClient{ClientID: "id", ClientSecret: "secret", TokenURL: server.URL, HTTPClient: cloneHTTPClient(server.Client()), Clock: fixedClock{value: testNow}}
		if _, err := client.ClientCredentials(context.Background()); err == nil {
			t.Errorf("case %d unexpectedly succeeded", index)
		}
		server.Close()
	}
}

func TestAutomaticTokenCachingAndConcurrency(t *testing.T) {
	var tokenCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/backstage/oauth/token":
			tokenCalls.Add(1)
			writeJSON(t, writer, http.StatusOK, map[string]any{"access_token": "automatic-token", "token_type": "bearer", "expires_in": 43200})
		case "/backstage/api/1.0/users/current/account":
			if request.Header.Get("Authorization") != "Bearer automatic-token" {
				t.Fatalf("Authorization=%q", request.Header.Get("Authorization"))
			}
			writeJSON(t, writer, http.StatusOK, Account{ID: 1, AccountID: testAdvertiserID})
		default:
			t.Fatalf("unexpected request=%s", request.URL)
		}
	}))
	defer server.Close()
	config := testConfig(server.URL)
	config.Accounts[0].AccessTokenRef = ""
	config.Accounts[0].ClientID = "client-id"
	config.Accounts[0].SecretRef = "secret://client"
	store := socialhub.NewMemoryTokenStore()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), config,
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{"secret://client": "client-secret"}),
		socialhub.WithClock(fixedClock{value: testNow}), socialhub.WithTokenStore(store),
	); err != nil {
		t.Fatal(err)
	}
	value, err := adapter.Client(context.Background(), testAccountID)
	if err != nil {
		t.Fatal(err)
	}
	client := value.(*Client)
	var wait sync.WaitGroup
	for range 12 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			if _, err := client.CurrentAccount(context.Background()); err != nil {
				t.Errorf("CurrentAccount error=%v", err)
			}
		}()
	}
	wait.Wait()
	if tokenCalls.Load() != 1 {
		t.Fatalf("token calls=%d", tokenCalls.Load())
	}

	secondValue, err := adapter.Client(context.Background(), testAccountID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := secondValue.(*Client).CurrentAccount(context.Background()); err != nil {
		t.Fatal(err)
	}
	if tokenCalls.Load() != 1 {
		t.Fatalf("stored token was not reused; token calls=%d", tokenCalls.Load())
	}
}

type failingTokenStore struct{ getErr, putErr error }

func (store failingTokenStore) Get(context.Context, socialhub.TokenKey) (socialhub.Token, error) {
	return socialhub.Token{}, store.getErr
}
func (store failingTokenStore) Put(context.Context, socialhub.TokenKey, socialhub.Token) error {
	return store.putErr
}
func (store failingTokenStore) Delete(context.Context, socialhub.TokenKey) error { return nil }

func TestTokenStoreFailuresAreRetryable(t *testing.T) {
	oauth := OAuthClient{ClientID: "id", ClientSecret: "secret", TokenURL: "https://example.test/token", HTTPClient: http.DefaultClient, Clock: fixedClock{value: testNow}}
	source := &clientTokenSource{oauth: oauth, store: failingTokenStore{getErr: errors.New("store unavailable")}}
	if hub := hubError(t, func() error { _, err := source.Token(context.Background()); return err }()); hub.Code != socialhub.CodeTemporarilyUnavailable || !hub.Retryable() {
		t.Fatalf("hub=%#v", hub)
	}
}

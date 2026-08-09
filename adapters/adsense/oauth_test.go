package adsense

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestOAuthAuthorizationExchangeAndRefresh(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/token" || request.Method != http.MethodPost || request.ParseForm() != nil {
			t.Fatalf("token request=%s %s", request.Method, request.URL)
		}
		if request.Form.Get("client_id") != "client-id" || request.Form.Get("client_secret") != "client-secret" {
			t.Errorf("credentials=%v", request.Form)
		}
		switch request.Form.Get("grant_type") {
		case "authorization_code":
			writeJSON(t, writer, http.StatusOK, map[string]any{"access_token": "access-1", "expires_in": 3600, "refresh_token": "refresh-1", "scope": fullScope, "token_type": "Bearer"})
		case "refresh_token":
			if request.Form.Get("refresh_token") != "refresh-1" {
				t.Errorf("refresh form=%v", request.Form)
			}
			writeJSON(t, writer, http.StatusOK, map[string]any{"access_token": "access-2", "expires_in": 3600, "token_type": "bearer"})
		default:
			t.Fatalf("form=%v", request.Form)
		}
	}))
	defer server.Close()
	adapter, _ := newStaticClient(t, server)
	oauth, err := adapter.OAuth(context.Background(), testAccountID)
	if err != nil {
		t.Fatal(err)
	}
	authorize, err := oauth.AuthorizationURL("https://app.example/callback", "state-value")
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(authorize)
	query := parsed.Query()
	if query.Get("client_id") != "client-id" || query.Get("scope") != fullScope || query.Get("state") != "state-value" ||
		query.Get("access_type") != "offline" || query.Get("prompt") != "consent" || query.Get("include_granted_scopes") != "true" {
		t.Fatalf("authorize=%s", authorize)
	}
	token, err := oauth.Exchange(context.Background(), "auth-code", "https://app.example/callback")
	if err != nil || token.AccessToken != "access-1" || token.RefreshToken != "refresh-1" || !token.ExpiresAt.Equal(testNow.Add(time.Hour)) {
		t.Fatalf("exchange=%#v err=%v", token, err)
	}
	token, err = oauth.Refresh(context.Background(), "refresh-1")
	if err != nil || token.AccessToken != "access-2" || token.RefreshToken != "refresh-1" || token.TokenType != "Bearer" || len(token.Scopes) != 1 {
		t.Fatalf("refresh=%#v err=%v", token, err)
	}
}

func TestManagedRefreshTokenSourceCachesAndRefreshes(t *testing.T) {
	var tokenCalls atomic.Int32
	clock := &mutableClock{value: testNow}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/token":
			if request.ParseForm() != nil || request.Form.Get("refresh_token") != "refresh-secret" {
				t.Fatalf("refresh request=%v", request.Form)
			}
			call := tokenCalls.Add(1)
			writeJSON(t, writer, http.StatusOK, map[string]any{"access_token": "managed-" + string(rune('0'+call)), "expires_in": 3600, "scope": readOnlyScope, "token_type": "Bearer"})
		case "/v2/" + accountName():
			writeJSON(t, writer, http.StatusOK, Account{Name: accountName()})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	config := managedConfig(server.URL)
	config.Accounts[0].Approval.Scopes = nil
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), config,
		socialhub.WithHTTPClient(server.Client()), socialhub.WithClock(clock), socialhub.WithTokenStore(socialhub.NewMemoryTokenStore()),
		socialhub.WithSecretResolver(mapResolver{"test://client-secret": "client-secret", "test://refresh-token": "refresh-secret"}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), testAccountID)
	if err != nil {
		t.Fatal(err)
	}
	client := common.(*Client)
	if len(client.scopes) != 1 || client.scopes[0] != readOnlyScope {
		t.Fatalf("managed scopes=%v", client.scopes)
	}
	if _, err := client.GetAccount(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetAccount(context.Background()); err != nil || tokenCalls.Load() != 1 {
		t.Fatalf("cached request err=%v tokenCalls=%d", err, tokenCalls.Load())
	}
	clock.Set(testNow.Add(2 * time.Hour))
	if _, err := client.GetAccount(context.Background()); err != nil || tokenCalls.Load() != 2 {
		t.Fatalf("refreshed request err=%v tokenCalls=%d", err, tokenCalls.Load())
	}
}

func TestOAuthErrorsAndValidation(t *testing.T) {
	tests := []struct {
		status int
		body   string
		code   socialhub.ErrorCode
	}{
		{400, `{"error":"invalid_grant","error_description":"expired"}`, socialhub.CodeUnauthenticated},
		{503, `{"error":"temporarily_unavailable","error_description":"retry"}`, socialhub.CodeTemporarilyUnavailable},
		{200, `not-json`, socialhub.CodePlatformError},
		{200, `{"access_token":"a","refresh_token":"r","expires_in":0}`, socialhub.CodePlatformError},
		{200, `{"access_token":"a","expires_in":3600}`, socialhub.CodePlatformError},
		{400, `not-json`, socialhub.CodeInvalidArgument},
	}
	for _, test := range tests {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(test.status)
			_, _ = writer.Write([]byte(test.body))
		}))
		oauth := &OAuthClient{ClientID: "client-id", ClientSecret: "client-secret", AuthURL: server.URL + "/authorize", TokenURL: server.URL, HTTPClient: server.Client(), Clock: &mutableClock{value: testNow}, Scopes: []string{readOnlyScope}}
		_, err := oauth.Exchange(context.Background(), "code", "https://app.example/callback")
		if requireHubError(t, err).Code != test.code {
			t.Errorf("status=%d error=%v", test.status, err)
		}
		server.Close()
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
	if validCallbackURL("ftp://example.com/callback") || validCallbackURL("https://user:pass@example.com") ||
		!validCallbackURL("http://localhost/callback") || validOAuthScopes(nil) || validOAuthScopes([]string{"openid"}) ||
		validOAuthScopes([]string{fullScope, fullScope}) || boundedMessage(strings.Repeat("x", 30), 20) != strings.Repeat("x", 20) {
		t.Fatal("OAuth validation contract failed")
	}
}

type failingTokenStore struct{}

func (failingTokenStore) Get(context.Context, socialhub.TokenKey) (socialhub.Token, error) {
	return socialhub.Token{}, errors.New("store unavailable")
}
func (failingTokenStore) Put(context.Context, socialhub.TokenKey, socialhub.Token) error {
	return errors.New("store unavailable")
}
func (failingTokenStore) Delete(context.Context, socialhub.TokenKey) error { return nil }

func TestRefreshTokenSourceStoreFailuresAndMemoryHit(t *testing.T) {
	source := &refreshTokenSource{oauth: OAuthClient{Clock: &mutableClock{value: testNow}}, store: failingTokenStore{}}
	if _, err := source.Token(context.Background()); !errors.Is(err, socialhub.ErrUnavailable) {
		t.Fatalf("cache get error=%v", err)
	}
	source = &refreshTokenSource{oauth: OAuthClient{ClientID: "client", ClientSecret: "secret", TokenURL: "https://example.com/token", HTTPClient: http.DefaultClient, Clock: &mutableClock{value: testNow}, Scopes: []string{readOnlyScope}}, refreshToken: "refresh", token: socialhub.Token{AccessToken: "cached", ExpiresAt: testNow.Add(time.Hour)}}
	if token, err := source.Token(context.Background()); err != nil || token.AccessToken != "cached" {
		t.Fatalf("memory token=%#v err=%v", token, err)
	}
}

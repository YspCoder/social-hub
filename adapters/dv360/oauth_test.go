package dv360

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
		if request.Method != http.MethodPost || request.Header.Get("Content-Type") != "application/x-www-form-urlencoded" || request.ParseForm() != nil {
			t.Fatalf("token request=%s %s", request.Method, request.URL)
		}
		if request.Form.Get("client_id") != "client-id" || request.Form.Get("client_secret") != "client-secret" {
			t.Errorf("credentials=%v", request.Form)
		}
		switch request.Form.Get("grant_type") {
		case "authorization_code":
			writeJSON(t, writer, http.StatusOK, map[string]any{
				"access_token": "access-1", "expires_in": 3600, "refresh_token": "refresh-1",
				"scope": displayVideoScope, "token_type": "Bearer",
			})
		case "refresh_token":
			if request.Form.Get("refresh_token") != "refresh-1" {
				t.Errorf("refresh form=%v", request.Form)
			}
			writeJSON(t, writer, http.StatusOK, map[string]any{
				"access_token": "access-2", "expires_in": 3600, "token_type": "bearer",
			})
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
	if query.Get("client_id") != "client-id" || query.Get("scope") != displayVideoScope ||
		query.Get("state") != "state-value" || query.Get("access_type") != "offline" ||
		query.Get("include_granted_scopes") != "true" || query.Get("prompt") != "consent" {
		t.Fatalf("authorize=%s", authorize)
	}
	token, err := oauth.Exchange(context.Background(), "auth-code", "https://app.example/callback")
	if err != nil || token.AccessToken != "access-1" || token.RefreshToken != "refresh-1" ||
		!token.ExpiresAt.Equal(testNow.Add(time.Hour)) || len(token.Scopes) != 1 {
		t.Fatalf("exchange=%#v err=%v", token, err)
	}
	token, err = oauth.Refresh(context.Background(), "refresh-1")
	if err != nil || token.AccessToken != "access-2" || token.RefreshToken != "refresh-1" || token.TokenType != "Bearer" ||
		len(token.Scopes) != 1 || token.Scopes[0] != displayVideoScope {
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
			writeJSON(t, writer, http.StatusOK, map[string]any{
				"access_token": "managed-" + string(rune('0'+call)), "expires_in": 3600,
				"scope": displayVideoScope, "token_type": "Bearer",
			})
		case "/v4/advertisers/" + testAdvertiserID:
			if request.Header.Get("Authorization") != "Bearer managed-"+string(rune('0'+tokenCalls.Load())) {
				t.Errorf("Authorization=%q", request.Header.Get("Authorization"))
			}
			writeJSON(t, writer, http.StatusOK, advertiserResource(EntityStatusActive))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), managedConfig(server.URL),
		socialhub.WithHTTPClient(server.Client()), socialhub.WithClock(clock),
		socialhub.WithTokenStore(socialhub.NewMemoryTokenStore()),
		socialhub.WithSecretResolver(mapResolver{
			"test://client-secret": "client-secret", "test://refresh-token": "refresh-secret",
		}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), testAccountID)
	if err != nil {
		t.Fatal(err)
	}
	client := common.(*Client)
	if _, err := client.GetAdvertiser(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetAdvertiser(context.Background()); err != nil || tokenCalls.Load() != 1 {
		t.Fatalf("cached request err=%v tokenCalls=%d", err, tokenCalls.Load())
	}
	clock.Set(testNow.Add(2 * time.Hour))
	if _, err := client.GetAdvertiser(context.Background()); err != nil || tokenCalls.Load() != 2 {
		t.Fatalf("refreshed request err=%v tokenCalls=%d", err, tokenCalls.Load())
	}
}

func TestOAuthErrorsAndValidation(t *testing.T) {
	responses := []struct {
		status int
		body   string
		code   socialhub.ErrorCode
	}{
		{http.StatusBadRequest, `{"error":"invalid_grant","error_description":"expired"}`, socialhub.CodeUnauthenticated},
		{http.StatusServiceUnavailable, `{"error":"temporarily_unavailable","error_description":"retry"}`, socialhub.CodeTemporarilyUnavailable},
		{http.StatusOK, `not-json`, socialhub.CodePlatformError},
		{http.StatusOK, `{"access_token":"a","refresh_token":"r","expires_in":0}`, socialhub.CodePlatformError},
		{http.StatusOK, `{"access_token":"a","expires_in":3600}`, socialhub.CodePlatformError},
		{http.StatusBadRequest, `not-json`, socialhub.CodeInvalidArgument},
	}
	for index, test := range responses {
		t.Run(string(rune('a'+index)), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.Header().Set("Content-Type", "application/json")
				writer.WriteHeader(test.status)
				_, _ = writer.Write([]byte(test.body))
			}))
			defer server.Close()
			oauth := &OAuthClient{
				ClientID: "client-id", ClientSecret: "client-secret", AuthURL: server.URL + "/authorize",
				TokenURL: server.URL, HTTPClient: server.Client(), Clock: &mutableClock{value: testNow},
			}
			_, err := oauth.Exchange(context.Background(), "code", "https://app.example/callback")
			if requireHubError(t, err).Code != test.code {
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
	if validCallbackURL("ftp://example.com/callback") || validCallbackURL("https://user:pass@example.com") ||
		!validCallbackURL("http://localhost/callback") {
		t.Fatal("callback URL contract failed")
	}
	if boundedMessage(strings.Repeat("界", 30), 20) != strings.Repeat("界", 20) {
		t.Fatal("bounded message failed")
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

func TestRefreshTokenSourceStoreFailures(t *testing.T) {
	source := &refreshTokenSource{
		oauth: OAuthClient{Clock: &mutableClock{value: testNow}}, store: failingTokenStore{},
	}
	if _, err := source.Token(context.Background()); !errors.Is(err, socialhub.ErrUnavailable) {
		t.Fatalf("cache get error=%v", err)
	}
	source = &refreshTokenSource{
		oauth: OAuthClient{
			ClientID: "client", ClientSecret: "secret", TokenURL: "https://example.com/token",
			HTTPClient: http.DefaultClient, Clock: &mutableClock{value: testNow},
		},
		refreshToken: "refresh", token: socialhub.Token{AccessToken: "cached", ExpiresAt: testNow.Add(time.Hour)},
	}
	if token, err := source.Token(context.Background()); err != nil || token.AccessToken != "cached" {
		t.Fatalf("memory cache token=%#v err=%v", token, err)
	}
}

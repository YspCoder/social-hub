package youtubereporting

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
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
			if request.Form.Get("code") != "auth-code" || request.Form.Get("redirect_uri") != "https://app.example/callback" {
				t.Errorf("exchange form=%v", request.Form)
			}
			writeJSON(t, writer, http.StatusOK, map[string]any{
				"access_token": "access-1", "expires_in": 3600, "refresh_token": "refresh-1",
				"scope": analyticsReadScope + " " + analyticsRevenueScope, "token_type": "Bearer",
			})
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
	adapter := &Adapter{}
	config := managedConfig(server.URL)
	config.Accounts[0].Approval.Scopes = append([]string(nil), supportedScopes...)
	if err := adapter.Init(context.Background(), config,
		socialhub.WithHTTPClient(server.Client()), socialhub.WithClock(&mutableClock{value: testNow}),
		socialhub.WithSecretResolver(mapResolver{"test://client-secret": "client-secret", "test://refresh-token": "refresh-1"}),
	); err != nil {
		t.Fatal(err)
	}
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
	if query.Get("client_id") != "client-id" || query.Get("scope") != analyticsReadScope+" "+analyticsRevenueScope ||
		query.Get("state") != "state-value" || query.Get("access_type") != "offline" || query.Get("prompt") != "consent" ||
		query.Get("include_granted_scopes") != "true" {
		t.Fatalf("authorize=%s", authorize)
	}
	token, err := oauth.Exchange(context.Background(), "auth-code", "https://app.example/callback")
	if err != nil || token.AccessToken != "access-1" || token.RefreshToken != "refresh-1" || !token.ExpiresAt.Equal(testNow.Add(time.Hour)) || len(token.Scopes) != 2 {
		t.Fatalf("exchange=%#v err=%v", token, err)
	}
	token, err = oauth.Refresh(context.Background(), "refresh-1")
	if err != nil || token.AccessToken != "access-2" || token.RefreshToken != "refresh-1" || token.TokenType != "Bearer" || len(token.Scopes) != 2 {
		t.Fatalf("refresh=%#v err=%v", token, err)
	}
}

func TestManagedRefreshTokenSourceCaches(t *testing.T) {
	var tokenCalls atomic.Int32
	clock := &mutableClock{value: testNow}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/token":
			call := tokenCalls.Add(1)
			writeJSON(t, writer, http.StatusOK, map[string]any{
				"access_token": "managed-" + string(rune('0'+call)), "expires_in": 3600,
				"scope": analyticsReadScope, "token_type": "Bearer",
			})
		case "/v1/jobs":
			writeJSON(t, writer, http.StatusOK, map[string]any{"jobs": []any{}})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), managedConfig(server.URL),
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
	if _, err := client.ListJobs(context.Background(), ListRequest{}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ListJobs(context.Background(), ListRequest{}); err != nil || tokenCalls.Load() != 1 {
		t.Fatalf("cached request err=%v tokenCalls=%d", err, tokenCalls.Load())
	}
	clock.Set(testNow.Add(2 * time.Hour))
	if _, err := client.ListJobs(context.Background(), ListRequest{}); err != nil || tokenCalls.Load() != 2 {
		t.Fatalf("refreshed request err=%v tokenCalls=%d", err, tokenCalls.Load())
	}
}

func TestContentOwnerServiceAccountJWTAndCache(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	privateKeyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: mustPKCS8(t, privateKey)}))
	var tokenCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/token":
			if request.ParseForm() != nil || request.Form.Get("grant_type") != jwtBearerGrantType {
				t.Fatalf("token form=%v", request.Form)
			}
			verifyJWTAssertion(t, request.Form.Get("assertion"), &privateKey.PublicKey, "http://"+request.Host+"/token")
			tokenCalls.Add(1)
			writeJSON(t, writer, http.StatusOK, map[string]any{"access_token": "service-access", "expires_in": 3600, "token_type": "Bearer"})
		case "/v1/jobs":
			if request.URL.Query().Get("onBehalfOfContentOwner") != testOwnerID || request.Header.Get("Authorization") != "Bearer service-access" {
				t.Errorf("owner request=%v auth=%q", request.URL.Query(), request.Header.Get("Authorization"))
			}
			writeJSON(t, writer, http.StatusOK, map[string]any{"jobs": []any{}})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), serviceAccountConfig(server.URL),
		socialhub.WithHTTPClient(server.Client()), socialhub.WithClock(&mutableClock{value: testNow}), socialhub.WithTokenStore(socialhub.NewMemoryTokenStore()),
		socialhub.WithSecretResolver(mapResolver{"test://private-key": privateKeyPEM}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), testAccountID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := common.(*Client).ListJobs(context.Background(), ListRequest{}); err != nil {
		t.Fatal(err)
	}
	second, err := adapter.Client(context.Background(), testAccountID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := second.(*Client).ListJobs(context.Background(), ListRequest{}); err != nil || tokenCalls.Load() != 1 {
		t.Fatalf("service cache err=%v calls=%d", err, tokenCalls.Load())
	}
}

func mustPKCS8(t *testing.T, key *rsa.PrivateKey) []byte {
	t.Helper()
	encoded, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func verifyJWTAssertion(t *testing.T, assertion string, publicKey *rsa.PublicKey, audience string) {
	t.Helper()
	parts := strings.Split(assertion, ".")
	if len(parts) != 3 {
		t.Fatalf("assertion parts=%d", len(parts))
	}
	var header map[string]any
	var claims map[string]any
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatal(err)
	}
	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	if json.Unmarshal(headerJSON, &header) != nil || json.Unmarshal(claimsJSON, &claims) != nil {
		t.Fatal("invalid JWT JSON")
	}
	if header["alg"] != "RS256" || claims["iss"] != "reports@test-project.iam.gserviceaccount.com" || claims["aud"] != audience ||
		claims["scope"] != analyticsReadScope || int64(claims["exp"].(float64)-claims["iat"].(float64)) != int64(time.Hour/time.Second) {
		t.Fatalf("header=%v claims=%v", header, claims)
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, digest[:], signature); err != nil {
		t.Fatalf("signature error=%v", err)
	}
}

func TestOAuthCredentialAndTokenStoreErrors(t *testing.T) {
	tests := []struct {
		status int
		body   string
		code   socialhub.ErrorCode
	}{
		{400, `{"error":"invalid_grant","error_description":"expired"}`, socialhub.CodeUnauthenticated},
		{503, `{"error":"temporarily_unavailable","error_description":"retry"}`, socialhub.CodeTemporarilyUnavailable},
		{200, `not-json`, socialhub.CodePlatformError},
		{200, `{"access_token":"a","refresh_token":"r","expires_in":0}`, socialhub.CodePlatformError},
		{400, `not-json`, socialhub.CodeInvalidArgument},
	}
	for _, test := range tests {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(test.status)
			_, _ = writer.Write([]byte(test.body))
		}))
		oauth := &OAuthClient{ClientID: "client-id", ClientSecret: "client-secret", AuthURL: server.URL + "/authorize", TokenURL: server.URL, HTTPClient: server.Client(), Clock: &mutableClock{value: testNow}, Scopes: []string{analyticsReadScope}}
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
	if _, err := parseRSAPrivateKey("not-pem"); err == nil {
		t.Fatal("invalid key accepted")
	}
	source := &refreshTokenSource{oauth: OAuthClient{Clock: &mutableClock{value: testNow}}, store: failingTokenStore{}}
	if _, err := source.Token(context.Background()); !errors.Is(err, socialhub.ErrUnavailable) {
		t.Fatalf("cache get error=%v", err)
	}
	source = &refreshTokenSource{oauth: OAuthClient{Clock: &mutableClock{value: testNow}}, token: socialhub.Token{AccessToken: "cached", ExpiresAt: testNow.Add(time.Hour)}}
	if token, err := source.Token(context.Background()); err != nil || token.AccessToken != "cached" {
		t.Fatalf("memory token=%#v err=%v", token, err)
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

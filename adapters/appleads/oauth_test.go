package appleads

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestOAuthClientSecretClaimsAndSignature(t *testing.T) {
	key, _ := generatePrivateKey(t)
	client := OAuthClient{ClientID: "client-id", TeamID: "team-id", KeyID: "key-id", PrivateKey: key, Clock: fixedClock{now: testNow}}
	secret, err := client.ClientSecret()
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(secret, ".")
	if len(parts) != 3 {
		t.Fatalf("JWT parts=%d", len(parts))
	}
	decodePart := func(value string, target any) {
		decoded, decodeErr := base64.RawURLEncoding.DecodeString(value)
		if decodeErr != nil || json.Unmarshal(decoded, target) != nil {
			t.Fatalf("decode JWT part: %v", decodeErr)
		}
	}
	var header map[string]any
	var claims struct {
		Issuer   string `json:"iss"`
		IssuedAt int64  `json:"iat"`
		Expires  int64  `json:"exp"`
		Audience string `json:"aud"`
		Subject  string `json:"sub"`
	}
	decodePart(parts[0], &header)
	decodePart(parts[1], &claims)
	if header["alg"] != "ES256" || header["kid"] != "key-id" || claims.Issuer != "team-id" || claims.Subject != "client-id" ||
		claims.Audience != "https://appleid.apple.com" || claims.IssuedAt != testNow.Unix() || claims.Expires != testNow.Add(clientSecretTTL).Unix() {
		t.Fatalf("header=%v claims=%#v", header, claims)
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(signature) != 64 {
		t.Fatalf("signature length=%d err=%v", len(signature), err)
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	r := new(big.Int).SetBytes(signature[:32])
	s := new(big.Int).SetBytes(signature[32:])
	if !ecdsa.Verify(&key.PublicKey, digest[:], r, s) {
		t.Fatal("ES256 signature verification failed")
	}
}

func TestOAuthClientSecretValidationAndPrivateKeyParsing(t *testing.T) {
	key, pemValue := generatePrivateKey(t)
	parsed, err := parsePrivateKey([]byte(pemValue))
	if err != nil || parsed.D.Cmp(key.D) != 0 {
		t.Fatalf("PKCS8 key=%v err=%v", parsed, err)
	}
	sec1, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err = parsePrivateKey(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: sec1}))
	if err != nil || parsed.D.Cmp(key.D) != 0 {
		t.Fatalf("SEC1 key=%v err=%v", parsed, err)
	}
	wrongCurve, err := ecdsa.GenerateKey(elliptic.P384(), strings.NewReader(strings.Repeat("x", 1024)))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []OAuthClient{
		{},
		{ClientID: "client", TeamID: "team", KeyID: "key", PrivateKey: wrongCurve, Clock: fixedClock{now: testNow}},
		{ClientID: "bad\nclient", TeamID: "team", KeyID: "key", PrivateKey: key, Clock: fixedClock{now: testNow}},
	} {
		if _, err := test.ClientSecret(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("ClientSecret error=%v", err)
		}
	}
	for _, value := range [][]byte{
		[]byte("not pem"),
		append([]byte(pemValue), []byte("trailing")...),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("bad")}),
	} {
		if _, err := parsePrivateKey(value); err == nil {
			t.Fatalf("key %q was accepted", value)
		}
	}
}

func TestOAuthTokenRequestAndResponse(t *testing.T) {
	key, _ := generatePrivateKey(t)
	requestSeen := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestSeen = true
		if request.Method != http.MethodPost || request.URL.Path != "/token" || request.Header.Get("Content-Type") != "application/x-www-form-urlencoded" || request.ParseForm() != nil {
			t.Errorf("request=%s %s headers=%v", request.Method, request.URL, request.Header)
		}
		if request.Form.Get("grant_type") != "client_credentials" || request.Form.Get("client_id") != "client-id" ||
			request.Form.Get("scope") != oauthScope || len(strings.Split(request.Form.Get("client_secret"), ".")) != 3 {
			t.Errorf("form=%v", request.Form)
		}
		writeJSON(t, writer, http.StatusOK, map[string]any{
			"access_token": "managed-token", "token_type": "bearer", "expires_in": 3600, "scope": oauthScope,
		})
	}))
	defer server.Close()
	client := OAuthClient{
		ClientID: "client-id", TeamID: "team-id", KeyID: "key-id", PrivateKey: key,
		TokenURL: server.URL + "/token", HTTPClient: server.Client(), Clock: fixedClock{now: testNow},
	}
	token, err := client.Token(context.Background())
	if err != nil || !requestSeen || token.AccessToken != "managed-token" || token.TokenType != "Bearer" ||
		!token.ExpiresAt.Equal(testNow.Add(time.Hour)) || len(token.Scopes) != 1 || token.Scopes[0] != oauthScope {
		t.Fatalf("token=%#v requestSeen=%v err=%v", token, requestSeen, err)
	}
	if _, err := (&OAuthClient{}).Token(context.Background()); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("invalid OAuth client error=%v", err)
	}
}

func TestOAuthTokenErrorsAndBounds(t *testing.T) {
	key, _ := generatePrivateKey(t)
	tests := []struct {
		name   string
		status int
		body   string
		code   socialhub.ErrorCode
	}{
		{"invalid client", http.StatusBadRequest, `{"error":"invalid_client","error_description":"bad"}`, socialhub.CodeUnauthenticated},
		{"invalid scope", http.StatusBadRequest, `{"error":"invalid_scope","error_description":"bad"}`, socialhub.CodeInvalidArgument},
		{"temporary", http.StatusServiceUnavailable, `{"error":"temporarily_unavailable","error_description":"retry"}`, socialhub.CodeTemporarilyUnavailable},
		{"malformed success", http.StatusOK, `not-json`, socialhub.CodePlatformError},
		{"missing fields", http.StatusOK, `{"access_token":"a","expires_in":0}`, socialhub.CodePlatformError},
		{"wrong scope", http.StatusOK, `{"access_token":"a","expires_in":3600,"scope":"wrong","token_type":"Bearer"}`, socialhub.CodePlatformError},
		{"malformed failure", http.StatusBadRequest, `not-json`, socialhub.CodeInvalidArgument},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				writer.Header().Set("Retry-After", "2")
				writer.Header().Set("Content-Type", "application/json")
				writer.WriteHeader(test.status)
				_, _ = writer.Write([]byte(test.body))
			}))
			defer server.Close()
			client := OAuthClient{
				ClientID: "client-id", TeamID: "team-id", KeyID: "key-id", PrivateKey: key,
				TokenURL: server.URL, HTTPClient: server.Client(), Clock: fixedClock{now: testNow},
			}
			_, err := client.Token(context.Background())
			hub := requireHubError(t, err)
			if hub.Code != test.code {
				t.Fatalf("error=%#v", hub)
			}
		})
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(strings.Repeat("x", int(maxOAuthResponseBytes)+1)))
	}))
	defer server.Close()
	client := OAuthClient{
		ClientID: "client-id", TeamID: "team-id", KeyID: "key-id", PrivateKey: key,
		TokenURL: server.URL, HTTPClient: server.Client(), Clock: fixedClock{now: testNow},
	}
	if hub := requireHubError(t, func() error { _, err := client.Token(context.Background()); return err }()); hub.Code != socialhub.CodePlatformError {
		t.Fatalf("bounded response error=%#v", hub)
	}
}

func TestManagedOAuthAdapterAndConcurrentTokenCache(t *testing.T) {
	_, privateKeyPEM := generatePrivateKey(t)
	var tokenCalls atomic.Int64
	var apiCalls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/token":
			tokenCalls.Add(1)
			writeJSON(t, writer, http.StatusOK, map[string]any{
				"access_token": "managed-token", "token_type": "Bearer", "expires_in": 3600, "scope": oauthScope,
			})
		case "/api/v5/acls":
			apiCalls.Add(1)
			if request.Header.Get("Authorization") != "Bearer managed-token" || request.Header.Get("X-AP-Context") != "orgId=12345" {
				t.Errorf("headers=%v", request.Header)
			}
			writeJSON(t, writer, http.StatusOK, pagedEnvelope([]UserACL{{OrgID: testOrgID, OrgName: "Org"}}, 1))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	store := socialhub.NewMemoryTokenStore()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), managedConfig(server.URL),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithClock(fixedClock{now: testNow}),
		socialhub.WithTokenStore(store),
		socialhub.WithSecretResolver(mapResolver{"test://private-key": privateKeyPEM}),
	); err != nil {
		t.Fatal(err)
	}
	oauth, err := adapter.OAuth(context.Background(), "search-us")
	if err != nil || oauth.ClientID != "client-id" || oauth.TeamID != "team-id" || oauth.KeyID != "key-id" {
		t.Fatalf("OAuth=%#v err=%v", oauth, err)
	}
	common, err := adapter.Client(context.Background(), "search-us")
	if err != nil {
		t.Fatal(err)
	}
	client := common.(*Client)
	const workers = 12
	var wait sync.WaitGroup
	errorsSeen := make(chan error, workers)
	for range workers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, callErr := client.ListACL(context.Background(), Pagination{Limit: 1})
			errorsSeen <- callErr
		}()
	}
	wait.Wait()
	close(errorsSeen)
	for callErr := range errorsSeen {
		if callErr != nil {
			t.Fatal(callErr)
		}
	}
	if tokenCalls.Load() != 1 || apiCalls.Load() != workers {
		t.Fatalf("tokenCalls=%d apiCalls=%d", tokenCalls.Load(), apiCalls.Load())
	}

	key := socialhub.TokenKey{
		Platform: platformName, Product: productName, Tenant: "team-id", Account: "search-us", Subject: "client-id", Scopes: oauthScope,
	}
	cached, err := store.Get(context.Background(), key)
	if err != nil || cached.AccessToken != "managed-token" {
		t.Fatalf("cached=%#v err=%v", cached, err)
	}
}

func TestManagedOAuthUsesStoredTokenAndRefreshLead(t *testing.T) {
	_, privateKeyPEM := generatePrivateKey(t)
	var tokenCalls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/token" {
			tokenCalls.Add(1)
			writeJSON(t, writer, http.StatusOK, map[string]any{
				"access_token": "fresh", "token_type": "Bearer", "expires_in": 3600, "scope": oauthScope,
			})
			return
		}
		if request.Header.Get("Authorization") != "Bearer cached" && request.Header.Get("Authorization") != "Bearer fresh" {
			t.Errorf("Authorization=%q", request.Header.Get("Authorization"))
		}
		writeJSON(t, writer, http.StatusOK, pagedEnvelope([]UserACL{{OrgID: testOrgID, OrgName: "Org"}}, 1))
	}))
	defer server.Close()
	key := socialhub.TokenKey{
		Platform: platformName, Product: productName, Tenant: "team-id", Account: "search-us", Subject: "client-id", Scopes: oauthScope,
	}
	store := socialhub.NewMemoryTokenStore()
	if err := store.Put(context.Background(), key, socialhub.Token{AccessToken: "cached", TokenType: "Bearer", ExpiresAt: testNow.Add(10 * time.Minute)}); err != nil {
		t.Fatal(err)
	}
	newClient := func() *Client {
		adapter := &Adapter{}
		if err := adapter.Init(context.Background(), managedConfig(server.URL),
			socialhub.WithHTTPClient(server.Client()), socialhub.WithClock(fixedClock{now: testNow}),
			socialhub.WithTokenStore(store), socialhub.WithSecretResolver(mapResolver{"test://private-key": privateKeyPEM}),
		); err != nil {
			t.Fatal(err)
		}
		common, err := adapter.Client(context.Background(), "search-us")
		if err != nil {
			t.Fatal(err)
		}
		return common.(*Client)
	}
	if _, err := newClient().ListACL(context.Background(), Pagination{Limit: 1}); err != nil || tokenCalls.Load() != 0 {
		t.Fatalf("cached request err=%v tokenCalls=%d", err, tokenCalls.Load())
	}
	if err := store.Put(context.Background(), key, socialhub.Token{AccessToken: "cached", TokenType: "Bearer", ExpiresAt: testNow.Add(4 * time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if _, err := newClient().ListACL(context.Background(), Pagination{Limit: 1}); err != nil || tokenCalls.Load() != 1 {
		t.Fatalf("refresh request err=%v tokenCalls=%d", err, tokenCalls.Load())
	}
}

type failingTokenStore struct {
	getErr error
	putErr error
}

func (store failingTokenStore) Get(context.Context, socialhub.TokenKey) (socialhub.Token, error) {
	return socialhub.Token{}, store.getErr
}

func (store failingTokenStore) Put(context.Context, socialhub.TokenKey, socialhub.Token) error {
	return store.putErr
}

func (failingTokenStore) Delete(context.Context, socialhub.TokenKey) error { return nil }

func TestTokenStoreErrorsAreRetryable(t *testing.T) {
	key, _ := generatePrivateKey(t)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writeJSON(t, writer, http.StatusOK, map[string]any{
			"access_token": "fresh", "token_type": "Bearer", "expires_in": 3600, "scope": oauthScope,
		})
	}))
	defer server.Close()
	oauth := OAuthClient{
		ClientID: "client", TeamID: "team", KeyID: "key", PrivateKey: key,
		TokenURL: server.URL, HTTPClient: server.Client(), Clock: fixedClock{now: testNow},
	}
	for _, store := range []failingTokenStore{
		{getErr: errors.New("get failed")},
		{getErr: socialhub.ErrNotFound, putErr: errors.New("put failed")},
	} {
		source := &clientTokenSource{oauth: oauth, store: store}
		_, err := source.Token(context.Background())
		hub := requireHubError(t, err)
		if !hub.Retryable() || hub.Code != socialhub.CodeTemporarilyUnavailable {
			t.Fatalf("error=%#v", hub)
		}
	}
}

package tmdb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	testBearerToken    = "test-read-access-token"
	testAPIKey         = "test-api-key"
	testSessionID      = "test-session-id"
	testGuestSessionID = "test-guest-session-id"
)

var testNow = time.Date(2026, time.August, 2, 8, 9, 10, 0, time.UTC)

type mapResolver map[string]string

func (resolver mapResolver) Resolve(_ context.Context, reference string) (string, error) {
	value, ok := resolver[reference]
	if !ok {
		return "", errors.New("missing test secret")
	}
	return value, nil
}

type fixedClock struct{ now time.Time }

func (clock fixedClock) Now() time.Time { return clock.now }

func testConfig(server *httptest.Server, bearer, session, guest bool) socialhub.AdapterConfig {
	account := socialhub.AccountConfig{ID: "viewer", Settings: map[string]any{"account_id": int64(42)}}
	if bearer {
		account.SecretRef = "test://bearer"
	} else {
		account.ClientID = testAPIKey
	}
	if session {
		account.AccessTokenRef = "test://session"
	}
	if guest {
		account.Settings["guest_session_ref"] = "test://guest"
	}
	return socialhub.AdapterConfig{
		Adapter: adapterName,
		Settings: map[string]any{
			"base_url": server.URL, "auth_url": server.URL + "/authenticate", "user_agent": "social-hub-tests/1.0",
		},
		Accounts: []socialhub.AccountConfig{account},
	}
}

func newTestClient(t *testing.T, server *httptest.Server, bearer, session, guest bool) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server, bearer, session, guest),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{
			"test://bearer": testBearerToken, "test://session": testSessionID, "test://guest": testGuestSessionID,
		}),
		socialhub.WithClock(fixedClock{now: testNow}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "viewer")
	if err != nil {
		t.Fatal(err)
	}
	return adapter, common.(*Client)
}

func TestAdapterRegistrationCapabilitiesAndLifecycle(t *testing.T) {
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatalf("registered adapters=%v", socialhub.Adapters())
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newTestClient(t, server, true, true, true)
	metadata := adapter.Metadata()
	if adapter.Name() != adapterName || metadata.Product != productName || metadata.APIVersion != apiVersion || metadata.DocURL != documentationURL {
		t.Fatalf("metadata=%#v", metadata)
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range []socialhub.Capability{CapabilityAuth, CapabilityCatalog, CapabilityAccount, CapabilityLibrary, CapabilityRating} {
		if !capabilities.Has(capability) {
			t.Fatalf("capability %s=%#v", capability, capabilities[capability])
		}
	}
	for _, capability := range []socialhub.Capability{socialhub.CapPublish, socialhub.CapFetch, socialhub.CapMedia, socialhub.CapReact, socialhub.CapMessage, socialhub.CapWebhook} {
		if capabilities.Has(capability) {
			t.Fatalf("common capability %s must be unsupported", capability)
		}
	}
	if client.Platform() != "tmdb" || client.Account() != "viewer" || client.Close() != nil ||
		client.AuthWorkflow() == nil || client.CatalogWorkflow() == nil || client.AccountWorkflow() == nil || client.LibraryWorkflow() == nil {
		t.Fatalf("client=%#v", client)
	}
	if _, ok := client.Publisher(); ok {
		t.Fatal("publisher must not be exposed")
	}
	if _, ok := client.Fetcher(); ok {
		t.Fatal("fetcher must not be exposed")
	}
	if _, ok := client.MediaUploader(); ok {
		t.Fatal("media uploader must not be exposed")
	}
	if _, ok := client.Reactor(); ok {
		t.Fatal("reactor must not be exposed")
	}
	if _, ok := client.Messenger(); ok {
		t.Fatal("messenger must not be exposed")
	}
	if _, ok := client.WebhookHandler(); ok {
		t.Fatal("webhook handler must not be exposed")
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "viewer"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close=%v", err)
	}
	if err := adapter.Init(context.Background(), testConfig(server, true, true, true)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("reinit=%v", err)
	}
}

func TestAdapterValidationAndCredentialGates(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	valid := testConfig(server, true, true, true)
	tests := []struct {
		name   string
		mutate func(*socialhub.AdapterConfig)
	}{
		{"adapter", func(config *socialhub.AdapterConfig) { config.Adapter = "other" }},
		{"base URL", func(config *socialhub.AdapterConfig) { config.Settings["base_url"] = "ftp://example.test" }},
		{"auth URL", func(config *socialhub.AdapterConfig) { config.Settings["auth_url"] = "https://user:pass@example.test" }},
		{"user agent", func(config *socialhub.AdapterConfig) { config.Settings["user_agent"] = "bad\nagent" }},
		{"credential", func(config *socialhub.AdapterConfig) { config.Accounts[0].SecretRef = "" }},
		{"secret ref", func(config *socialhub.AdapterConfig) { config.Accounts[0].SecretRef = "bad\nref" }},
		{"session ref", func(config *socialhub.AdapterConfig) { config.Accounts[0].AccessTokenRef = "bad\nref" }},
		{"account ID", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["account_id"] = -1 }},
		{"guest ref", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["guest_session_ref"] = "bad\nref" }},
		{"adapter setting", func(config *socialhub.AdapterConfig) { config.Settings["other"] = true }},
		{"account setting", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["other"] = true }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := cloneConfig(valid)
			test.mutate(&config)
			if err := (&Adapter{}).Init(context.Background(), config); !errors.Is(err, socialhub.ErrInvalidArgument) {
				t.Fatalf("error=%v", err)
			}
		})
	}

	_, public := newTestClient(t, server, true, false, false)
	capabilities, _ := public.Capabilities(context.Background())
	for _, capability := range []socialhub.Capability{CapabilityAccount, CapabilityLibrary, CapabilityRating} {
		if capabilities.Has(capability) || capabilities[capability].Approval != socialhub.ApprovalRequired {
			t.Fatalf("gated capability %s=%#v", capability, capabilities[capability])
		}
	}
	if _, err := public.GetAccount(context.Background()); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("account gate=%v", err)
	}
	if _, err := public.SetRating(context.Background(), RatingRequest{Target: MediaTarget{MediaType: MediaMovie, MediaID: 1}, Value: 8}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("rating gate=%v", err)
	}

	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), valid, socialhub.WithHTTPClient(server.Client()), socialhub.WithSecretResolver(mapResolver{})); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "viewer"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("unresolved credential=%v", err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing account=%v", err)
	}
}

func TestAuthenticationWorkflows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer "+testBearerToken || request.Header.Get("User-Agent") != "social-hub-tests/1.0" {
			http.Error(writer, "bad application auth", http.StatusUnauthorized)
			return
		}
		switch request.Method + " " + request.URL.Path {
		case "GET /authentication/token/new":
			writeJSON(writer, http.StatusOK, `{"success":true,"expires_at":"2026-08-02 09:09:10 UTC","request_token":"request-token"}`)
		case "POST /authentication/session/new":
			if requestBody(request) != `{"request_token":"request-token"}` {
				http.Error(writer, "bad token", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"success":true,"session_id":"new-session"}`)
		case "DELETE /authentication/session":
			if requestBody(request) != `{"session_id":"new-session"}` {
				http.Error(writer, "bad session", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"success":true}`)
		case "GET /authentication/guest_session/new":
			writeJSON(writer, http.StatusOK, `{"success":true,"expires_at":"2026-08-02T10:09:10Z","guest_session_id":"guest-session"}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, true, false, false)
	token, err := client.RequestToken(context.Background())
	if err != nil || token.Token != "request-token" || !token.ExpiresAt.Equal(testNow.Add(time.Hour)) {
		t.Fatalf("token=%#v err=%v", token, err)
	}
	approvalURL, err := client.ApprovalURL(token.Token, "https://app.example/callback?source=tmdb")
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(approvalURL)
	if parsed.Path != "/authenticate/request-token" || parsed.Query().Get("redirect_to") != "https://app.example/callback?source=tmdb" {
		t.Fatalf("approval URL=%s", approvalURL)
	}
	sessionID, err := client.CreateSession(context.Background(), token.Token)
	if err != nil || sessionID != "new-session" {
		t.Fatalf("session=%q err=%v", sessionID, err)
	}
	if err := client.DeleteSession(context.Background(), sessionID); err != nil {
		t.Fatal(err)
	}
	guest, err := client.CreateGuestSession(context.Background())
	if err != nil || guest.ID != "guest-session" || !guest.ExpiresAt.Equal(testNow.Add(2*time.Hour)) {
		t.Fatalf("guest=%#v err=%v", guest, err)
	}
}

func TestAuthenticationValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, true, false, false)
	tests := []func() error{
		func() error { _, err := client.ApprovalURL("bad/token", ""); return err },
		func() error { _, err := client.ApprovalURL("token", "ftp://example.test"); return err },
		func() error { _, err := client.CreateSession(context.Background(), ""); return err },
		func() error { return client.DeleteSession(context.Background(), "") },
	}
	for _, call := range tests {
		if err := call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("error=%v", err)
		}
	}
}

func cloneConfig(input socialhub.AdapterConfig) socialhub.AdapterConfig {
	output := input
	output.Settings = cloneMap(input.Settings)
	output.Accounts = append([]socialhub.AccountConfig(nil), input.Accounts...)
	output.Accounts[0].Settings = cloneMap(input.Accounts[0].Settings)
	return output
}

func cloneMap(input map[string]any) map[string]any {
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func writeJSON(writer http.ResponseWriter, status int, body string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_, _ = writer.Write([]byte(body))
}

func requestBody(request *http.Request) string {
	body, _ := io.ReadAll(request.Body)
	var compact bytes.Buffer
	if json.Compact(&compact, body) == nil {
		return compact.String()
	}
	return strings.TrimSpace(string(body))
}

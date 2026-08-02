package applemusic

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

type mapResolver map[string]string

func (r mapResolver) Resolve(_ context.Context, reference string) (string, error) {
	value, ok := r[reference]
	if !ok {
		return "", errors.New("missing test secret")
	}
	return value, nil
}

type testClock struct{ now time.Time }

func (c *testClock) Now() time.Time { return c.now }

var testNow = time.Date(2026, time.August, 2, 1, 2, 3, 0, time.UTC)

func newTestAdapter(t *testing.T, server *httptest.Server, withUserToken bool) (*Adapter, *Client) {
	t.Helper()
	settings := map[string]any{"storefront": "US"}
	secrets := mapResolver{"test://developer": "developer-token"}
	if withUserToken {
		settings["music_user_token_ref"] = "test://user"
		secrets["test://user"] = "music-user-token"
	}
	config := socialhub.AdapterConfig{
		Adapter:  adapterName,
		Settings: map[string]any{"base_url": server.URL + "/v1", "developer_token_ttl": "1h"},
		Accounts: []socialhub.AccountConfig{{
			ID: "listener", AccessTokenRef: "test://developer", Settings: settings,
		}},
	}
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), config,
		socialhub.WithHTTPClient(server.Client()), socialhub.WithSecretResolver(secrets),
		socialhub.WithClock(&testClock{now: testNow}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "listener")
	if err != nil {
		t.Fatal(err)
	}
	return adapter, common.(*Client)
}

func TestAdapterRegistrationMetadataAndCapabilities(t *testing.T) {
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatalf("registered adapters=%v", socialhub.Adapters())
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newTestAdapter(t, server, true)
	metadata := adapter.Metadata()
	if adapter.Name() != adapterName || metadata.Product != productName || metadata.APIVersion != apiVersion || metadata.DocURL != documentationURL {
		t.Fatalf("metadata=%#v", metadata)
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range []socialhub.Capability{CapabilityStorefront, CapabilityCatalog, CapabilityLibrary, CapabilityPlaylist, CapabilityHistory} {
		if !capabilities.Has(capability) {
			t.Fatalf("capability %q=%#v", capability, capabilities[capability])
		}
	}
	for _, capability := range []socialhub.Capability{socialhub.CapPublish, socialhub.CapFetch, socialhub.CapMedia, socialhub.CapReact, socialhub.CapMessage, socialhub.CapWebhook} {
		if capabilities.Has(capability) {
			t.Fatalf("common capability %q must be unsupported", capability)
		}
	}
	if client.Platform() != "applemusic" || client.Account() != "listener" || client.StorefrontWorkflow() == nil ||
		client.CatalogWorkflow() == nil || client.LibraryWorkflow() == nil || client.PlaylistWorkflow() == nil || client.HistoryWorkflow() == nil {
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
	if client.Close() != nil || adapter.Close() != nil {
		t.Fatal("close failed")
	}
	if _, err := adapter.Client(context.Background(), "listener"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close error=%v", err)
	}
}

func TestAdapterValidationAndCredentialFailures(t *testing.T) {
	valid := socialhub.AccountConfig{ID: "listener", AccessTokenRef: "test://developer", Settings: map[string]any{"storefront": "US"}}
	tests := []struct {
		name   string
		config socialhub.AdapterConfig
	}{
		{"wrong adapter", socialhub.AdapterConfig{Adapter: "other", Accounts: []socialhub.AccountConfig{valid}}},
		{"bad endpoint", socialhub.AdapterConfig{Adapter: adapterName, Settings: map[string]any{"base_url": "ftp://api.example"}, Accounts: []socialhub.AccountConfig{valid}}},
		{"bad ttl", socialhub.AdapterConfig{Adapter: adapterName, Settings: map[string]any{"developer_token_ttl": "5000h"}, Accounts: []socialhub.AccountConfig{valid}}},
		{"bad storefront", socialhub.AdapterConfig{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "listener", AccessTokenRef: "ref", Settings: map[string]any{"storefront": "USA"}}}}},
		{"missing signing config", socialhub.AdapterConfig{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "listener"}}}},
		{"bad signing ID", socialhub.AdapterConfig{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "listener", SecretRef: "ref", Settings: map[string]any{"team_id": "bad", "key_id": "ABC123DEFG"}}}}},
		{"unknown account setting", socialhub.AdapterConfig{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "listener", AccessTokenRef: "ref", Settings: map[string]any{"unknown": true}}}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := (&Adapter{}).Init(context.Background(), test.config); !errors.Is(err, socialhub.ErrInvalidArgument) {
				t.Fatalf("error=%v", err)
			}
		})
	}

	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, _ := newTestAdapter(t, server, false)
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing account error=%v", err)
	}
	adapter.options.Secrets = mapResolver{}
	if _, err := adapter.Client(context.Background(), "listener"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("missing developer token error=%v", err)
	}

	privateKey := testPrivateKeyPEM(t, elliptic.P256())
	dynamic := &Adapter{}
	err := dynamic.Init(context.Background(), socialhub.AdapterConfig{
		Adapter: adapterName, Settings: map[string]any{"base_url": server.URL + "/v1"},
		Accounts: []socialhub.AccountConfig{{ID: "signed", SecretRef: "test://key", Settings: map[string]any{"team_id": "TEAM123456", "key_id": "KEY1234567"}}},
	}, socialhub.WithHTTPClient(server.Client()), socialhub.WithSecretResolver(mapResolver{"test://key": privateKey}), socialhub.WithClock(&testClock{now: testNow}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dynamic.Client(context.Background(), "signed"); err != nil {
		t.Fatalf("signed client error=%v", err)
	}
	dynamic.options.Secrets = mapResolver{"test://key": "not a key"}
	if _, err := dynamic.Client(context.Background(), "signed"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("invalid private key error=%v", err)
	}

	_, publicOnly := newTestAdapter(t, server, false)
	capabilities, _ := publicOnly.Capabilities(context.Background())
	if capabilities.Has(CapabilityLibrary) || capabilities[CapabilityLibrary].Approval != socialhub.ApprovalRequired {
		t.Fatalf("library capability=%#v", capabilities[CapabilityLibrary])
	}
	if _, err := publicOnly.ListLibrarySongs(context.Background(), PaginationRequest{}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("library gate error=%v", err)
	}
}

func testPrivateKeyPEM(t *testing.T, curve elliptic.Curve) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

func errorCode(err error) socialhub.ErrorCode {
	var hubError *socialhub.Error
	if errors.As(err, &hubError) {
		return hubError.Code
	}
	return ""
}

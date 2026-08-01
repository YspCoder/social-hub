package vimeo

import (
	"context"
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

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

var testNow = time.Date(2026, time.August, 2, 1, 2, 3, 0, time.UTC)

func newTestAdapter(t *testing.T, server *httptest.Server, scopes []string) (*Adapter, *Client) {
	t.Helper()
	config := socialhub.AdapterConfig{
		Adapter: adapterName, Product: productName,
		Settings: map[string]any{
			"base_url": server.URL + "/api", "auth_url": server.URL + "/authorize",
			"token_url": server.URL + "/oauth/access_token", "client_token_url": server.URL + "/oauth/authorize/client",
		},
		Accounts: []socialhub.AccountConfig{{
			ID: "account", ClientID: "client-id", SecretRef: "test://secret", AccessTokenRef: "test://token",
			Settings: map[string]any{"user_id": "user-1"}, Approval: socialhub.ApprovalConfig{Scopes: scopes},
		}},
	}
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), config,
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{"test://secret": "client-secret", "test://token": "access-token"}),
		socialhub.WithClock(fixedClock{now: testNow}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "account")
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
	adapter, client := newTestAdapter(t, server, []string{"public", "interact", "upload", "edit", "delete"})
	metadata := adapter.Metadata()
	if adapter.Name() != adapterName || metadata.APIVersion != "3.4" || metadata.Product != productName {
		t.Fatalf("metadata=%#v", metadata)
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range []socialhub.Capability{socialhub.CapFetch, socialhub.CapReact, CapabilityHomeFeed, CapabilityVideoUpload} {
		if !capabilities.Has(capability) {
			t.Fatalf("capability %q=%#v", capability, capabilities[capability])
		}
	}
	for _, capability := range []socialhub.Capability{socialhub.CapPublish, socialhub.CapMedia, socialhub.CapMessage, socialhub.CapWebhook} {
		if capabilities.Has(capability) {
			t.Fatalf("unsupported capability %q=%#v", capability, capabilities[capability])
		}
	}
	if client.Platform() != "vimeo" || client.Account() != "account" || client.VideoUploadWorkflow() == nil || client.FeedWorkflow() == nil {
		t.Fatalf("client=%#v", client)
	}
	if _, ok := client.Publisher(); ok {
		t.Fatal("publisher must not be exposed")
	}
	if _, ok := client.Fetcher(); !ok {
		t.Fatal("fetcher must be exposed")
	}
	if _, ok := client.MediaUploader(); ok {
		t.Fatal("media uploader must not be exposed")
	}
	if _, ok := client.Reactor(); !ok {
		t.Fatal("reactor must be exposed")
	}
	if _, ok := client.Messenger(); ok {
		t.Fatal("messenger must not be exposed")
	}
	if _, ok := client.WebhookHandler(); ok {
		t.Fatal("webhook handler must not be exposed")
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "account"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close error=%v", err)
	}
	if err := adapter.Init(context.Background(), socialhub.AdapterConfig{}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("re-init error=%v", err)
	}
}

func TestAdapterValidationAndClientFailures(t *testing.T) {
	validAccount := socialhub.AccountConfig{ID: "account", AccessTokenRef: "test://token"}
	tests := []struct {
		name   string
		config socialhub.AdapterConfig
	}{
		{"wrong adapter", socialhub.AdapterConfig{Adapter: "other", Accounts: []socialhub.AccountConfig{validAccount}}},
		{"bad endpoint", socialhub.AdapterConfig{Adapter: adapterName, Settings: map[string]any{"base_url": "ftp://api.example"}, Accounts: []socialhub.AccountConfig{validAccount}}},
		{"missing token", socialhub.AdapterConfig{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "account"}}}},
		{"bad account setting", socialhub.AdapterConfig{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "account", AccessTokenRef: "test://token", Settings: map[string]any{"user_id": "bad/id"}}}}},
		{"unknown setting", socialhub.AdapterConfig{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "account", AccessTokenRef: "test://token", Settings: map[string]any{"unknown": true}}}}},
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
	adapter, _ := newTestAdapter(t, server, nil)
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing account error=%v", err)
	}
	adapter.options.Secrets = mapResolver{"test://secret": "client-secret"}
	if _, err := adapter.Client(context.Background(), "account"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("missing token error=%v", err)
	}
}

func TestScopeGating(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, []string{"public"})
	capabilities, _ := client.Capabilities(context.Background())
	if capabilities.Has(socialhub.CapReact) || capabilities.Has(CapabilityVideoUpload) || !capabilities.Has(socialhub.CapFetch) {
		t.Fatalf("capabilities=%#v", capabilities)
	}
	reaction := socialhub.ReactionRequest{TargetID: "video-1", Kind: socialhub.ReactionLike}
	if err := client.React(context.Background(), reaction); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("reaction scope error=%v", err)
	}
	if _, err := client.VideoUploadWorkflow().Initialize(context.Background(), VideoUploadRequest{Name: "video", Size: 4}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("upload scope error=%v", err)
	}
}

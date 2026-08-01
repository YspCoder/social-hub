package tiktok

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

func newTestAdapter(t *testing.T, server *httptest.Server, scopes []string, withSecret bool) (*Adapter, *Client) {
	t.Helper()
	account := socialhub.AccountConfig{
		ID: "creator", ClientID: "client-key", AccessTokenRef: "test://access-token",
		Settings: map[string]any{"open_id": "open-id"}, Approval: socialhub.ApprovalConfig{Scopes: scopes},
	}
	if withSecret {
		account.SecretRef = "test://client-secret"
	}
	config := socialhub.AdapterConfig{
		Adapter: adapterName, Product: "tiktok-for-developers",
		Settings: map[string]any{"base_url": server.URL, "auth_url": server.URL + "/authorize", "token_url": server.URL + "/oauth/token"},
		Accounts: []socialhub.AccountConfig{account},
	}
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), config,
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{"test://access-token": "access-token", "test://client-secret": "client-secret"}),
		socialhub.WithClock(fixedClock{now: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "creator")
	if err != nil {
		t.Fatal(err)
	}
	return adapter, common.(*Client)
}

func TestAdapterRegistrationAndCapabilities(t *testing.T) {
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatalf("registered adapters=%v", socialhub.Adapters())
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newTestAdapter(t, server, []string{"user.info.basic", "video.list", "video.publish"}, true)
	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range []socialhub.Capability{CapabilityContentPosting, socialhub.CapFetch, socialhub.CapWebhook} {
		if !capabilities.Has(capability) {
			t.Fatalf("capability %q=%#v", capability, capabilities[capability])
		}
	}
	for _, capability := range []socialhub.Capability{socialhub.CapPublish, socialhub.CapMedia, socialhub.CapReact, socialhub.CapMessage} {
		if capabilities.Has(capability) {
			t.Fatalf("unexpected capability %q", capability)
		}
	}
	if _, ok := client.Publisher(); ok || client.ContentWorkflow() == nil {
		t.Fatal("common publisher should be unavailable and content workflow available")
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "creator"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close error=%v", err)
	}
}

func TestAdapterValidationAndScopeGating(t *testing.T) {
	invalid := socialhub.AdapterConfig{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "creator", AccessTokenRef: "test://token"}}}
	if err := (&Adapter{}).Init(context.Background(), invalid); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("validation error=%v", err)
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, []string{"video.list"}, false)
	if _, err := client.ContentWorkflow().CreatorInfo(context.Background()); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("scope error=%v", err)
	}
}

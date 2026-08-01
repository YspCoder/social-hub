package snapchat

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

func newTestAdapter(t *testing.T, server *httptest.Server, scopes []string) (*Adapter, *Client) {
	t.Helper()
	config := socialhub.AdapterConfig{
		Adapter: adapterName, Product: "snapchat-public-profile",
		Settings: map[string]any{
			"base_url": server.URL, "auth_url": server.URL + "/authorize", "token_url": server.URL + "/access_token",
		},
		Accounts: []socialhub.AccountConfig{{
			ID: "creator", ClientID: "client-id", SecretRef: "test://client-secret", AccessTokenRef: "test://access-token",
			Settings: map[string]any{"profile_id": "profile-1"}, Approval: socialhub.ApprovalConfig{Scopes: scopes},
		}},
	}
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), config,
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{"test://client-secret": "client-secret", "test://access-token": "access-token"}),
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
	adapter, client := newTestAdapter(t, server, []string{requiredScope})
	capabilities, err := client.Capabilities(context.Background())
	if err != nil || !capabilities.Has(CapabilityPublicProfileWorkflow) {
		t.Fatalf("capabilities=%#v err=%v", capabilities, err)
	}
	for _, capability := range []socialhub.Capability{
		socialhub.CapPublish, socialhub.CapFetch, socialhub.CapMedia,
		socialhub.CapReact, socialhub.CapMessage, socialhub.CapWebhook,
	} {
		if capabilities.Has(capability) {
			t.Fatalf("common capability %s should be unavailable", capability)
		}
	}
	if _, ok := client.Fetcher(); ok || client.PublicProfileWorkflow() == nil {
		t.Fatal("common fetcher should be unavailable and typed workflow available")
	}
	metadata := adapter.Metadata()
	if metadata.APIVersion != "v1" || metadata.Product != "snapchat-public-profile" {
		t.Fatalf("metadata=%#v", metadata)
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "creator"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close error=%v", err)
	}
}

func TestAdapterValidationAndScopeGating(t *testing.T) {
	invalid := socialhub.AdapterConfig{
		Adapter:  adapterName,
		Accounts: []socialhub.AccountConfig{{ID: "creator", AccessTokenRef: "test://token"}},
	}
	if err := (&Adapter{}).Init(context.Background(), invalid); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("validation error=%v", err)
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, []string{"snapchat-marketing-api"})
	_, err := client.PublicProfileWorkflow().Profile(context.Background(), "profile-1")
	if !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("scope error=%v", err)
	}
}

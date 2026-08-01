package linkedin

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
		Adapter: adapterName, Product: "linkedin-rest",
		Settings: map[string]any{"base_url": server.URL, "auth_url": server.URL + "/authorize", "token_url": server.URL + "/oauth/token"},
		Accounts: []socialhub.AccountConfig{{
			ID: "company", ClientID: "client-id", SecretRef: "test://client-secret", AccessTokenRef: "test://access-token",
			Settings: map[string]any{"author_urn": "urn:li:organization:123"}, Approval: socialhub.ApprovalConfig{Scopes: scopes},
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
	common, err := adapter.Client(context.Background(), "company")
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
	adapter, client := newTestAdapter(t, server, []string{"openid", "profile", "r_organization_social", "w_organization_social"})
	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range []socialhub.Capability{socialhub.CapPublish, socialhub.CapFetch, socialhub.CapMedia, socialhub.CapReact} {
		if !capabilities.Has(capability) {
			t.Fatalf("capability %q=%#v", capability, capabilities[capability])
		}
	}
	if capabilities.Has(socialhub.CapMessage) || capabilities.Has(socialhub.CapWebhook) {
		t.Fatalf("capabilities=%#v", capabilities)
	}
	if _, ok := client.Publisher(); !ok {
		t.Fatal("publisher unavailable")
	}
	if _, ok := client.WebhookHandler(); ok {
		t.Fatal("webhook should be unavailable")
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "company"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close error=%v", err)
	}
}

func TestAdapterValidationAndScopeGating(t *testing.T) {
	invalid := socialhub.AdapterConfig{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "company", AccessTokenRef: "test://token", Settings: map[string]any{"author_urn": "invalid"}}}}
	if err := (&Adapter{}).Init(context.Background(), invalid); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("validation error=%v", err)
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, []string{"openid", "profile"})
	text := "blocked"
	if _, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("scope error=%v", err)
	}
}

package instagram

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

func newTestAdapter(t *testing.T, server *httptest.Server, scopes []string, webhook bool) (*Adapter, *Client) {
	t.Helper()
	account := socialhub.AccountConfig{
		ID: "brand", ClientID: "client-id", SecretRef: "test://app-secret", AccessTokenRef: "test://access-token",
		Settings: map[string]any{"user_id": "178"}, Approval: socialhub.ApprovalConfig{Scopes: scopes},
	}
	if webhook {
		account.Webhook.SecretRef = "test://webhook-secret"
		account.Webhook.TokenRef = "test://webhook-token"
	}
	config := socialhub.AdapterConfig{
		Adapter: adapterName, Product: "instagram-login",
		Settings: map[string]any{
			"base_url": server.URL, "auth_url": server.URL + "/authorize", "token_url": server.URL + "/oauth/access_token",
			"long_token_url": server.URL + "/access_token", "refresh_url": server.URL + "/refresh_access_token",
		},
		Accounts: []socialhub.AccountConfig{account},
	}
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), config,
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{
			"test://app-secret": "app-secret", "test://access-token": "access-token",
			"test://webhook-secret": "webhook-secret", "test://webhook-token": "verify-token",
		}),
		socialhub.WithClock(fixedClock{now: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "brand")
	if err != nil {
		t.Fatal(err)
	}
	return adapter, common.(*Client)
}

func TestAdapterRegistrationAndCapabilitySurface(t *testing.T) {
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatalf("registered adapters=%v", socialhub.Adapters())
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newTestAdapter(t, server, []string{
		"instagram_business_basic", "instagram_business_content_publish", "instagram_business_manage_comments", messagingScope,
	}, true)
	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range []socialhub.Capability{CapabilityContainerPublish, socialhub.CapFetch, socialhub.CapReact, socialhub.CapMessage, socialhub.CapWebhook} {
		if !capabilities.Has(capability) {
			t.Fatalf("capability %q=%#v", capability, capabilities[capability])
		}
	}
	if capabilities.Has(socialhub.CapPublish) || capabilities.Has(socialhub.CapMedia) {
		t.Fatalf("capabilities=%#v", capabilities)
	}
	if _, ok := client.Publisher(); ok {
		t.Fatal("publisher should be unavailable")
	}
	if _, ok := client.Fetcher(); !ok {
		t.Fatal("fetcher unavailable")
	}
	if _, ok := client.Reactor(); !ok {
		t.Fatal("reactor unavailable")
	}
	if _, ok := client.Messenger(); !ok {
		t.Fatal("messenger unavailable")
	}
	if _, ok := client.WebhookHandler(); !ok {
		t.Fatal("webhook unavailable")
	}
	if client.ContainerWorkflow() == nil || client.MessagingWorkflow() == nil || client.MessagingProfileWorkflow() == nil {
		t.Fatal("typed workflow unavailable")
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "brand"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close error=%v", err)
	}
}

func TestExplicitScopesFailBeforeNetwork(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, []string{"instagram_business_basic"}, false)
	_, err := client.ContainerWorkflow().Create(context.Background(), ContainerRequest{Type: ContainerImage, MediaURL: "https://cdn.example/image.jpg"})
	if !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("error=%v", err)
	}
}

func TestAdapterRequiresProfessionalUserID(t *testing.T) {
	config := socialhub.AdapterConfig{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "brand", AccessTokenRef: "test://token"}}}
	if err := (&Adapter{}).Init(context.Background(), config); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("error=%v", err)
	}
}

package bilibili

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"social-hub/extensions/video"
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

func newTestAdapter(t *testing.T, server *httptest.Server) (*Adapter, *Client) {
	t.Helper()
	config := socialhub.AdapterConfig{
		Adapter: adapterName,
		Product: "open-platform",
		Settings: map[string]any{
			"base_url": server.URL, "upload_base_url": server.URL,
			"auth_url": server.URL + "/pc/account-pc/auth/oauth", "oauth_base_url": server.URL,
		},
		Accounts: []socialhub.AccountConfig{{
			ID: "primary", ClientID: "client-id", SecretRef: "test://app-secret", AccessTokenRef: "test://access-token",
			Settings: map[string]any{
				"open_id": "open-id-1", "default_tid": 229, "default_tags": []string{"social-hub"},
				"default_copyright": 1, "no_reprint": true,
			},
			Approval: socialhub.ApprovalConfig{Scopes: []string{"USER_INFO", "ARC_BASE"}},
		}},
	}
	adapter := &Adapter{}
	err := adapter.Init(context.Background(), config,
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{"test://app-secret": "app-secret", "test://access-token": "access-token"}),
		socialhub.WithClock(fixedClock{now: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)}),
	)
	if err != nil {
		t.Fatal(err)
	}
	client, err := adapter.Client(context.Background(), "primary")
	if err != nil {
		t.Fatal(err)
	}
	return adapter, client.(*Client)
}

func TestAdapterRegistrationAndCapabilitySurface(t *testing.T) {
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatalf("registered adapters=%v", socialhub.Adapters())
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newTestAdapter(t, server)
	capabilities, err := client.Capabilities(context.Background())
	if err != nil || !capabilities.Has(socialhub.CapPublish) || !capabilities.Has(socialhub.CapFetch) || !capabilities.Has(socialhub.CapMedia) {
		t.Fatalf("capabilities=%#v err=%v", capabilities, err)
	}
	if _, ok := client.Reactor(); ok {
		t.Fatal("reactor should be unavailable")
	}
	if client.SubmissionWorkflow() == nil || client.VideoWorkflow() == nil {
		t.Fatal("typed workflows unavailable")
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "primary"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close error=%v", err)
	}
}

func TestRepostDefaultsRequireSource(t *testing.T) {
	config := socialhub.AdapterConfig{
		Adapter: adapterName, Product: "open-platform",
		Accounts: []socialhub.AccountConfig{{ID: "primary", ClientID: "client-id", SecretRef: "test://secret", Settings: map[string]any{"default_copyright": 2}}},
	}
	err := (&Adapter{}).Init(context.Background(), config)
	if !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("error=%v", err)
	}
}

var _ video.Provider = (*Client)(nil)

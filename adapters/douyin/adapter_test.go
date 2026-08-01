package douyin

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
		Product: "openapi",
		Settings: map[string]any{
			"base_url":       server.URL,
			"auth_url":       server.URL + "/platform/oauth/connect/",
			"oauth_base_url": server.URL,
		},
		Accounts: []socialhub.AccountConfig{{
			ID:             "primary",
			ClientID:       "client-key",
			SecretRef:      "test://client-secret",
			AccessTokenRef: "test://user-token",
			Settings:       map[string]any{"open_id": "open-id-1"},
			Approval: socialhub.ApprovalConfig{Scopes: []string{
				"user_info", "video.list", "video.data", "video.create", "video.delete", "item.comment",
			}},
		}},
	}
	adapter := &Adapter{}
	err := adapter.Init(context.Background(), config,
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{
			"test://client-secret": "client-secret",
			"test://user-token":    "user-token",
		}),
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
	if err != nil || !capabilities.Has(socialhub.CapPublish) || !capabilities.Has(socialhub.CapFetch) || !capabilities.Has(socialhub.CapMedia) || !capabilities.Has(socialhub.CapReact) || !capabilities.Has(socialhub.CapWebhook) {
		t.Fatalf("capabilities=%#v err=%v", capabilities, err)
	}
	if _, ok := client.Messenger(); ok {
		t.Fatal("messenger should be unavailable")
	}
	if workflow := client.VideoWorkflow(); workflow == nil {
		t.Fatal("video workflow unavailable")
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "primary"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close error=%v", err)
	}
}

var _ video.Provider = (*Client)(nil)

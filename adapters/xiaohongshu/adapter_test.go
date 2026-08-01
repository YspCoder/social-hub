package xiaohongshu

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
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

type steppingClock struct {
	mu   sync.Mutex
	now  time.Time
	step time.Duration
}

func (c *steppingClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	value := c.now
	c.now = c.now.Add(c.step)
	return value
}

func newTestAdapter(t *testing.T, server *httptest.Server, approved bool, accessTokenRef string, store socialhub.TokenStore, clock socialhub.Clock) (*Adapter, *Client) {
	t.Helper()
	config := socialhub.AdapterConfig{
		Adapter: adapterName,
		Product: "share-js",
		Settings: map[string]any{
			"base_url": server.URL,
		},
		Accounts: []socialhub.AccountConfig{{
			ID:             "primary",
			ClientID:       "app-key",
			SecretRef:      "test://app-secret",
			AccessTokenRef: accessTokenRef,
			Settings:       map[string]any{"approved": approved},
		}},
	}
	options := []socialhub.Option{
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{
			"test://app-secret":  "app-secret",
			"test://share-token": "static-share-token",
		}),
		socialhub.WithClock(clock),
	}
	if store != nil {
		options = append(options, socialhub.WithTokenStore(store))
	}
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), config, options...); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "primary")
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
	clock := &steppingClock{now: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)}
	adapter, client := newTestAdapter(t, server, false, "", nil, clock)

	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	share := capabilities[CapabilityShare]
	if !share.Supported || share.Approval != socialhub.ApprovalRequired || capabilities.Has(CapabilityShare) {
		t.Fatalf("share capability=%#v", share)
	}
	for _, capability := range []socialhub.Capability{
		socialhub.CapPublish,
		socialhub.CapFetch,
		socialhub.CapMedia,
		socialhub.CapReact,
		socialhub.CapMessage,
		socialhub.CapWebhook,
	} {
		if capabilities[capability].Supported {
			t.Fatalf("capability %q must be unavailable", capability)
		}
	}
	if publisher, ok := client.Publisher(); ok || publisher != nil {
		t.Fatal("publisher should be unavailable")
	}
	if fetcher, ok := client.Fetcher(); ok || fetcher != nil {
		t.Fatal("fetcher should be unavailable")
	}
	if uploader, ok := client.MediaUploader(); ok || uploader != nil {
		t.Fatal("media uploader should be unavailable")
	}
	if reactor, ok := client.Reactor(); ok || reactor != nil {
		t.Fatal("reactor should be unavailable")
	}
	if messenger, ok := client.Messenger(); ok || messenger != nil {
		t.Fatal("messenger should be unavailable")
	}
	if webhook, ok := client.WebhookHandler(); ok || webhook != nil {
		t.Fatal("webhook handler should be unavailable")
	}
	if client.ShareWorkflow() == nil {
		t.Fatal("share workflow unavailable")
	}
	if _, err := client.ShareWorkflow().Prepare(context.Background(), ShareRequest{Type: ShareTypeNormal, Images: []string{"https://cdn.example/note.jpg"}}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("unapproved share error=%v", err)
	}

	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "primary"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close error=%v", err)
	}
}

func TestAdapterRejectsMissingCredentials(t *testing.T) {
	config := socialhub.AdapterConfig{
		Adapter:  adapterName,
		Product:  "share-js",
		Accounts: []socialhub.AccountConfig{{ID: "primary"}},
	}
	if err := (&Adapter{}).Init(context.Background(), config); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("error=%v", err)
	}
}

package youtube

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
		Adapter: adapterName, Product: "youtube-data",
		Settings: map[string]any{
			"base_url": server.URL + "/youtube/v3", "upload_url": server.URL + "/upload/youtube/v3",
			"auth_url": server.URL + "/authorize", "token_url": server.URL + "/oauth/token",
		},
		Accounts: []socialhub.AccountConfig{{
			ID: "channel", ClientID: "client-id", SecretRef: "test://client-secret", AccessTokenRef: "test://access-token",
			Settings: map[string]any{"channel_id": "channel-1"}, Approval: socialhub.ApprovalConfig{Scopes: scopes},
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
	common, err := adapter.Client(context.Background(), "channel")
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
	adapter, client := newTestAdapter(t, server, []string{
		"https://www.googleapis.com/auth/youtube.readonly", "https://www.googleapis.com/auth/youtube.upload", "https://www.googleapis.com/auth/youtube.force-ssl",
	})
	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range []socialhub.Capability{CapabilityVideoUpload, socialhub.CapFetch, socialhub.CapReact} {
		if !capabilities.Has(capability) {
			t.Fatalf("capability %q=%#v", capability, capabilities[capability])
		}
	}
	if capabilities.Has(socialhub.CapPublish) || capabilities.Has(socialhub.CapMedia) || capabilities.Has(socialhub.CapMessage) || capabilities.Has(socialhub.CapWebhook) {
		t.Fatalf("capabilities=%#v", capabilities)
	}
	if _, ok := client.Publisher(); ok || client.VideoUploadWorkflow() == nil {
		t.Fatal("common publisher should be unavailable and upload workflow available")
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "channel"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close error=%v", err)
	}
}

func TestAdapterValidationAndScopeGating(t *testing.T) {
	invalid := socialhub.AdapterConfig{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "channel", AccessTokenRef: "test://token"}}}
	if err := (&Adapter{}).Init(context.Background(), invalid); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("validation error=%v", err)
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, []string{"https://www.googleapis.com/auth/youtube.readonly"})
	if _, err := client.VideoUploadWorkflow().Initialize(context.Background(), VideoUploadRequest{Title: "video", MIME: "video/mp4", Size: 4}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("scope error=%v", err)
	}
}

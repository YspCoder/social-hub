package soundcloud

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

const (
	testUserURN  = "soundcloud:users:123"
	testTrackURN = "soundcloud:tracks:456"
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

func newTestAdapter(t *testing.T, server *httptest.Server) (*Adapter, *Client) {
	t.Helper()
	config := socialhub.AdapterConfig{
		Adapter: adapterName, Product: productName,
		Settings: map[string]any{
			"base_url": server.URL + "/api", "auth_url": server.URL + "/authorize", "token_url": server.URL + "/oauth/token",
		},
		Accounts: []socialhub.AccountConfig{{
			ID: "artist", ClientID: "client-id", SecretRef: "test://secret", AccessTokenRef: "test://token",
			Approval: socialhub.ApprovalConfig{AccountType: "artist_pro"}, Settings: map[string]any{"user_urn": testUserURN},
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
	common, err := adapter.Client(context.Background(), "artist")
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
	adapter, client := newTestAdapter(t, server)
	metadata := adapter.Metadata()
	if adapter.Name() != adapterName || metadata.APIVersion != apiVersion || metadata.Product != productName || metadata.DocURL != docURL {
		t.Fatalf("metadata=%#v", metadata)
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range []socialhub.Capability{socialhub.CapFetch, socialhub.CapReact, CapabilityTrackUpload, CapabilityActivityFeed} {
		if !capabilities.Has(capability) {
			t.Fatalf("capability %q=%#v", capability, capabilities[capability])
		}
	}
	for _, capability := range []socialhub.Capability{socialhub.CapPublish, socialhub.CapMedia, socialhub.CapMessage, socialhub.CapWebhook} {
		if capabilities.Has(capability) {
			t.Fatalf("unsupported capability %q=%#v", capability, capabilities[capability])
		}
	}
	if client.Platform() != "soundcloud" || client.Account() != "artist" || client.TrackUploadWorkflow() == nil || client.ActivityWorkflow() == nil {
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
	if _, err := adapter.Client(context.Background(), "artist"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close error=%v", err)
	}
}

func TestAdapterValidationAndClientFailures(t *testing.T) {
	validAccount := socialhub.AccountConfig{ID: "artist", AccessTokenRef: "test://token"}
	tests := []struct {
		name   string
		config socialhub.AdapterConfig
	}{
		{"wrong adapter", socialhub.AdapterConfig{Adapter: "other", Accounts: []socialhub.AccountConfig{validAccount}}},
		{"bad endpoint", socialhub.AdapterConfig{Adapter: adapterName, Settings: map[string]any{"base_url": "ftp://api.example"}, Accounts: []socialhub.AccountConfig{validAccount}}},
		{"missing token", socialhub.AdapterConfig{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "artist"}}}},
		{"bad user urn", socialhub.AdapterConfig{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "artist", AccessTokenRef: "test://token", Settings: map[string]any{"user_urn": "123"}}}}},
		{"unknown setting", socialhub.AdapterConfig{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "artist", AccessTokenRef: "test://token", Settings: map[string]any{"unknown": true}}}}},
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
	adapter, _ := newTestAdapter(t, server)
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing account error=%v", err)
	}
	adapter.options.Secrets = mapResolver{"test://secret": "client-secret"}
	if _, err := adapter.Client(context.Background(), "artist"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("missing token error=%v", err)
	}
	adapter.config.Accounts[0].ClientID = ""
	if _, err := adapter.OAuth(context.Background(), "artist"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("OAuth validation error=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing OAuth account error=%v", err)
	}
}

func writeJSON(writer http.ResponseWriter, value string) {
	writer.Header().Set("Content-Type", "application/json")
	_, _ = writer.Write([]byte(value))
}

func trackJSON(urn string) string {
	return `{"urn":"` + urn + `","title":"Track title","description":"Track description","artwork_url":"https://cdn.example/art.jpg","duration":61000,"created_at":"2026/08/01 00:00:00 +0000","sharing":"public","access":"playable","metadata_artist":"Artist","genre":"Electronic","license":"cc-by","tag_list":"demo","user":{"urn":"` + testUserURN + `"},"comment_count":2,"favoritings_count":3,"playback_count":4,"reposts_count":5,"download_count":6,"streamable":true,"downloadable":false,"commentable":true,"uri":"https://api.soundcloud.com/tracks/456","permalink_url":"https://soundcloud.com/artist/track"}`
}

func errorCode(err error) socialhub.ErrorCode {
	var hubError *socialhub.Error
	if errors.As(err, &hubError) {
		return hubError.Code
	}
	return ""
}

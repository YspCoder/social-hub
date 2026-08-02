package toutiao

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

var testNow = time.Date(2026, 8, 2, 3, 4, 5, 0, time.UTC)

func newTestAdapter(t *testing.T, apiServer, oauthServer *httptest.Server) (*Adapter, *Client) {
	t.Helper()
	config := testConfig(apiServer.URL, oauthServer.URL)
	adapter := &Adapter{}
	err := adapter.Init(context.Background(), config,
		socialhub.WithHTTPClient(apiServer.Client()),
		socialhub.WithSecretResolver(mapResolver{
			"test://client-secret": "client-secret",
			"test://user-token":    "user-token",
		}),
		socialhub.WithClock(fixedClock{now: testNow}),
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

func testConfig(apiURL, oauthURL string) socialhub.AdapterConfig {
	return socialhub.AdapterConfig{
		Adapter: adapterName,
		Product: productName,
		Settings: map[string]any{
			"base_url": apiURL, "oauth_base_url": oauthURL,
			"auth_url": oauthURL + "/oauth/authorize/",
		},
		Accounts: []socialhub.AccountConfig{{
			ID: "primary", ClientID: "client-key", SecretRef: "test://client-secret",
			AccessTokenRef: "test://user-token", Settings: map[string]any{"open_id": "open-id-1"},
			Approval: socialhub.ApprovalConfig{Scopes: []string{
				"user_info", "toutiao.video.create", "toutiao.video.data",
			}},
		}},
	}
}

func TestAdapterRegistrationMetadataAndSurface(t *testing.T) {
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatalf("registered adapters=%v", socialhub.Adapters())
	}
	apiServer := httptest.NewServer(http.NotFoundHandler())
	defer apiServer.Close()
	oauthServer := httptest.NewServer(http.NotFoundHandler())
	defer oauthServer.Close()
	adapter, client := newTestAdapter(t, apiServer, oauthServer)

	metadata := adapter.Metadata()
	if adapter.Name() != adapterName || metadata.Name != adapterName || metadata.Product != productName || metadata.APIVersion != apiVersion || metadata.DocURL == "" || metadata.VerifiedAt.IsZero() {
		t.Fatalf("metadata=%#v", metadata)
	}
	if client.Platform() != platformName || client.Account() != "primary" {
		t.Fatalf("identity=%s/%s", client.Platform(), client.Account())
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range []socialhub.Capability{socialhub.CapPublish, socialhub.CapFetch, socialhub.CapMedia} {
		if !capabilities.Has(capability) || capabilities[capability].Approval != socialhub.ApprovalGranted {
			t.Fatalf("capability %s=%#v", capability, capabilities[capability])
		}
	}
	for _, capability := range []socialhub.Capability{socialhub.CapReact, socialhub.CapMessage, socialhub.CapWebhook} {
		if capabilities.Has(capability) {
			t.Fatalf("capability %s should be unsupported", capability)
		}
	}
	if _, ok := client.Publisher(); !ok {
		t.Fatal("publisher unavailable")
	}
	if _, ok := client.Fetcher(); !ok {
		t.Fatal("fetcher unavailable")
	}
	if _, ok := client.MediaUploader(); !ok {
		t.Fatal("media uploader unavailable")
	}
	if _, ok := client.Reactor(); ok {
		t.Fatal("reactor should be unavailable")
	}
	if _, ok := client.Messenger(); ok {
		t.Fatal("messenger should be unavailable")
	}
	if _, ok := client.WebhookHandler(); ok {
		t.Fatal("webhook should be unavailable")
	}
	if client.VideoWorkflow() == nil {
		t.Fatal("video workflow unavailable")
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "primary"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close error=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "primary"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("oauth after close error=%v", err)
	}
	if err := adapter.Init(context.Background(), testConfig(apiServer.URL, oauthServer.URL)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("init after close error=%v", err)
	}
}

func TestAdapterOpenAndValidation(t *testing.T) {
	t.Parallel()
	apiServer := httptest.NewServer(http.NotFoundHandler())
	defer apiServer.Close()
	oauthServer := httptest.NewServer(http.NotFoundHandler())
	defer oauthServer.Close()
	config := testConfig(apiServer.URL, oauthServer.URL)
	opened, err := socialhub.Open(context.Background(), adapterName, config,
		socialhub.WithHTTPClient(apiServer.Client()),
		socialhub.WithSecretResolver(mapResolver{"test://client-secret": "secret", "test://user-token": "token"}),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()

	tests := []struct {
		name   string
		mutate func(*socialhub.AdapterConfig)
	}{
		{"adapter mismatch", func(c *socialhub.AdapterConfig) { c.Adapter = "douyin/openapi" }},
		{"product mismatch", func(c *socialhub.AdapterConfig) { c.Product = "other" }},
		{"bad base URL", func(c *socialhub.AdapterConfig) { c.Settings["base_url"] = "file:///tmp/api" }},
		{"unknown setting", func(c *socialhub.AdapterConfig) { c.Settings["unknown"] = true }},
		{"missing credentials", func(c *socialhub.AdapterConfig) { c.Accounts[0].AccessTokenRef, c.Accounts[0].ClientID = "", "" }},
		{"bad open ID", func(c *socialhub.AdapterConfig) { c.Accounts[0].Settings["open_id"] = " bad " }},
		{"webhook", func(c *socialhub.AdapterConfig) { c.Accounts[0].Webhook.SecretRef = "secret" }},
		{"bad scope", func(c *socialhub.AdapterConfig) { c.Accounts[0].Approval.Scopes = []string{"bad scope"} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := testConfig(apiServer.URL, oauthServer.URL)
			test.mutate(&candidate)
			if err := (&Adapter{}).Init(context.Background(), candidate); !errors.Is(err, socialhub.ErrInvalidArgument) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestAdapterClientAndOAuthCredentialRequirements(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	config := testConfig(server.URL, server.URL)
	config.Accounts[0].AccessTokenRef = ""
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), config, socialhub.WithHTTPClient(server.Client()), socialhub.WithSecretResolver(mapResolver{"test://client-secret": "secret"})); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "primary"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("client error=%v", err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing client error=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing oauth error=%v", err)
	}

	config = testConfig(server.URL, server.URL)
	config.Accounts[0].ClientID = ""
	config.Accounts[0].SecretRef = ""
	adapter = &Adapter{}
	if err := adapter.Init(context.Background(), config, socialhub.WithHTTPClient(server.Client()), socialhub.WithSecretResolver(mapResolver{"test://user-token": "token"})); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.OAuth(context.Background(), "primary"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("oauth error=%v", err)
	}
}

var _ video.Provider = (*Client)(nil)

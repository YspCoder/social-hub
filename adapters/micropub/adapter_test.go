package micropub

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

func (resolver mapResolver) Resolve(_ context.Context, reference string) (string, error) {
	value, found := resolver[reference]
	if !found {
		return "", errors.New("missing test secret")
	}
	return value, nil
}

type fixedClock struct{ now time.Time }

func (clock fixedClock) Now() time.Time { return clock.now }

var testNow = time.Date(2026, time.August, 3, 1, 2, 3, 0, time.UTC)

func testConfig(server *httptest.Server, scopes []string, update, delete, undelete bool) socialhub.AdapterConfig {
	return socialhub.AdapterConfig{
		Adapter: adapterName, Product: productName,
		Accounts: []socialhub.AccountConfig{{
			ID: "site", AccessTokenRef: "test://token", Approval: socialhub.ApprovalConfig{Scopes: scopes},
			Settings: map[string]any{
				"endpoint": server.URL + "/micropub?tenant=one", "site_url": server.URL + "/",
				"supports_update": update, "supports_delete": delete, "supports_undelete": undelete,
			},
		}},
	}
}

func newTestClient(t *testing.T, server *httptest.Server, scopes []string, update, delete, undelete bool) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server, scopes, update, delete, undelete),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{"test://token": "access-token"}),
		socialhub.WithClock(fixedClock{now: testNow}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "site")
	if err != nil {
		t.Fatal(err)
	}
	return adapter, common.(*Client)
}

func TestAdapterRegistrationMetadataCapabilitiesAndLifecycle(t *testing.T) {
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatalf("registered adapters=%v", socialhub.Adapters())
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newTestClient(t, server, []string{"create", "update", "delete"}, true, true, true)
	metadata := adapter.Metadata()
	if adapter.Name() != adapterName || metadata.Product != productName || metadata.APIVersion != apiVersion || metadata.DocURL != documentationURL || metadata.VerifiedAt.IsZero() {
		t.Fatalf("metadata=%#v", metadata)
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range []socialhub.Capability{socialhub.CapPublish, socialhub.CapFetch, CapabilityEntries, CapabilityQueries, CapabilityMediaEndpoint} {
		if !capabilities.Has(capability) {
			t.Fatalf("capability %q=%#v", capability, capabilities[capability])
		}
	}
	for _, capability := range []socialhub.Capability{socialhub.CapMedia, socialhub.CapReact, socialhub.CapMessage, socialhub.CapWebhook} {
		if capabilities.Has(capability) {
			t.Fatalf("unsupported capability %q=%#v", capability, capabilities[capability])
		}
	}
	if client.Platform() != platformName || client.Account() != "site" || client.EntryWorkflow() == nil || client.QueryWorkflow() == nil || client.MediaWorkflow() == nil {
		t.Fatalf("client=%#v", client)
	}
	if _, ok := client.Publisher(); !ok {
		t.Fatal("Publisher must be exposed")
	}
	if _, ok := client.Fetcher(); !ok {
		t.Fatal("Fetcher must be exposed")
	}
	if _, ok := client.MediaUploader(); ok {
		t.Fatal("common MediaUploader must not be exposed")
	}
	if _, ok := client.Reactor(); ok {
		t.Fatal("Reactor must not be exposed")
	}
	if _, ok := client.Messenger(); ok {
		t.Fatal("Messenger must not be exposed")
	}
	if _, ok := client.WebhookHandler(); ok {
		t.Fatal("WebhookHandler must not be exposed")
	}
	if client.Close() != nil || adapter.Close() != nil {
		t.Fatal("close failed")
	}
	if _, err := adapter.Client(context.Background(), "site"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close=%v", err)
	}
	if err := adapter.Init(context.Background(), testConfig(server, nil, true, true, true)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("reinit=%v", err)
	}
}

func TestAdapterValidationClientFailuresAndScopeGating(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	valid := testConfig(server, nil, true, true, false)
	tests := []struct {
		name   string
		mutate func(*socialhub.AdapterConfig)
	}{
		{"adapter", func(config *socialhub.AdapterConfig) { config.Adapter = "other" }},
		{"adapter settings", func(config *socialhub.AdapterConfig) { config.Settings = map[string]any{"unknown": true} }},
		{"endpoint", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["endpoint"] = "ftp://example.test" }},
		{"endpoint user", func(config *socialhub.AdapterConfig) {
			config.Accounts[0].Settings["endpoint"] = "https://user:pass@example.test/micropub"
		}},
		{"endpoint fragment", func(config *socialhub.AdapterConfig) {
			config.Accounts[0].Settings["endpoint"] = "https://example.test/micropub#fragment"
		}},
		{"site", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["site_url"] = "/relative" }},
		{"undelete", func(config *socialhub.AdapterConfig) {
			config.Accounts[0].Settings["supports_undelete"] = true
			config.Accounts[0].Settings["supports_delete"] = false
		}},
		{"token ref", func(config *socialhub.AdapterConfig) { config.Accounts[0].AccessTokenRef = "" }},
		{"unknown", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["unknown"] = true }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := cloneConfig(valid)
			test.mutate(&config)
			if err := (&Adapter{}).Init(context.Background(), config); !errors.Is(err, socialhub.ErrInvalidArgument) {
				t.Fatalf("error=%v", err)
			}
		})
	}

	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), valid, socialhub.WithHTTPClient(server.Client()), socialhub.WithSecretResolver(mapResolver{})); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing client=%v", err)
	}
	if _, err := adapter.Client(context.Background(), "site"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("missing token=%v", err)
	}
	adapter.options.Secrets = mapResolver{"test://token": "bad\ntoken"}
	if _, err := adapter.Client(context.Background(), "site"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("invalid token=%v", err)
	}

	_, client := newTestClient(t, server, []string{"create"}, true, true, false)
	capabilities, _ := client.Capabilities(context.Background())
	if capabilities.Has(socialhub.CapFetch) || capabilities[socialhub.CapFetch].Approval != socialhub.ApprovalRequired {
		t.Fatalf("fetch capability=%#v", capabilities[socialhub.CapFetch])
	}
	if err := client.requireScope("test", "update"); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("scope error=%v", err)
	}
	if !scopeGranted([]string{" CREATE "}, "create") || scopeGranted([]string{"read"}, "create") {
		t.Fatal("scope comparison failed")
	}
}

func TestClientWithoutUpdateDoesNotExposeFetcher(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, nil, false, false, false)
	capabilities, _ := client.Capabilities(context.Background())
	if capabilities.Has(socialhub.CapFetch) {
		t.Fatalf("fetch=%#v", capabilities[socialhub.CapFetch])
	}
	if _, ok := client.Fetcher(); ok {
		t.Fatal("Fetcher must not be exposed")
	}
	if _, err := client.GetUser(context.Background(), "me"); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("user error=%v", err)
	}
	if _, err := client.PublishStatus(context.Background(), "https://example.test/post"); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("status error=%v", err)
	}
	if err := client.DeletePost(context.Background(), "https://example.test/post"); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("delete error=%v", err)
	}
	if _, err := client.UndeleteEntry(context.Background(), "https://example.test/post"); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("undelete error=%v", err)
	}
}

func cloneConfig(input socialhub.AdapterConfig) socialhub.AdapterConfig {
	output := input
	output.Settings = cloneMap(input.Settings)
	output.Accounts = append([]socialhub.AccountConfig(nil), input.Accounts...)
	output.Accounts[0].Settings = cloneMap(input.Accounts[0].Settings)
	return output
}

func cloneMap(input map[string]any) map[string]any {
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func errorCode(err error) socialhub.ErrorCode {
	var hubError *socialhub.Error
	if errors.As(err, &hubError) {
		return hubError.Code
	}
	return ""
}

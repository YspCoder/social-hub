package mixcloud

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

var testNow = time.Date(2026, 8, 2, 3, 4, 5, 0, time.UTC)

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

func testConfig(server *httptest.Server, accountType string) socialhub.AdapterConfig {
	return socialhub.AdapterConfig{
		Adapter: adapterName, Product: productName,
		Settings: map[string]any{
			"base_url": server.URL, "auth_url": server.URL + "/oauth/authorize",
			"token_url": server.URL + "/oauth/access_token", "user_agent": "social-hub-tests/1.0",
		},
		Accounts: []socialhub.AccountConfig{{
			ID: "creator", ClientID: "client-id", SecretRef: "test://client-secret", AccessTokenRef: "test://access-token",
			Settings: map[string]any{"username": "sample-dj"}, Approval: socialhub.ApprovalConfig{AccountType: accountType},
		}},
	}
}

func newTestClient(t *testing.T, server *httptest.Server, accountType string) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server, accountType),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{"test://access-token": "access-token", "test://client-secret": "client-secret"}),
		socialhub.WithClock(fixedClock{now: testNow}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "creator")
	if err != nil {
		t.Fatal(err)
	}
	return adapter, common.(*Client)
}

func TestAdapterRegistrationCapabilitiesAndLifecycle(t *testing.T) {
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatalf("registered adapters=%v", socialhub.Adapters())
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newTestClient(t, server, "pro")
	metadata := adapter.Metadata()
	if adapter.Name() != adapterName || metadata.Product != productName || metadata.APIVersion != apiVersion || metadata.DocURL != documentationURL {
		t.Fatalf("metadata=%#v", metadata)
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range []socialhub.Capability{
		socialhub.CapFetch, socialhub.CapReact, CapabilityIdentity, CapabilityCloudcasts,
		CapabilityDiscovery, CapabilityUpload, CapabilityLibrary, CapabilityProUpload,
	} {
		if !capabilities.Has(capability) {
			t.Fatalf("capability %s=%#v", capability, capabilities[capability])
		}
	}
	if capabilities[CapabilityProUpload].Approval != socialhub.ApprovalGranted {
		t.Fatalf("Pro capability=%#v", capabilities[CapabilityProUpload])
	}
	for _, capability := range []socialhub.Capability{socialhub.CapPublish, socialhub.CapMedia, socialhub.CapMessage, socialhub.CapWebhook} {
		if capabilities.Has(capability) {
			t.Fatalf("unexpected capability %s", capability)
		}
	}
	if client.Platform() != "mixcloud" || client.Account() != "creator" || client.Close() != nil ||
		client.UserWorkflow() == nil || client.CloudcastWorkflow() == nil || client.DiscoveryWorkflow() == nil ||
		client.UploadWorkflow() == nil || client.LibraryWorkflow() == nil {
		t.Fatalf("client=%#v", client)
	}
	if _, ok := client.Fetcher(); !ok {
		t.Fatal("Fetcher must be exposed")
	}
	if _, ok := client.Reactor(); !ok {
		t.Fatal("Reactor must be exposed")
	}
	if _, ok := client.Publisher(); ok {
		t.Fatal("Publisher must not be exposed")
	}
	if _, ok := client.MediaUploader(); ok {
		t.Fatal("MediaUploader must not be exposed")
	}
	if _, ok := client.Messenger(); ok {
		t.Fatal("Messenger must not be exposed")
	}
	if _, ok := client.WebhookHandler(); ok {
		t.Fatal("WebhookHandler must not be exposed")
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "creator"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "creator"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("OAuth after close=%v", err)
	}
	if err := adapter.Init(context.Background(), testConfig(server, "pro")); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("reinit=%v", err)
	}
}

func TestAdapterValidationAndCredentialErrors(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	valid := testConfig(server, "")
	tests := []struct {
		name   string
		mutate func(*socialhub.AdapterConfig)
	}{
		{"adapter", func(config *socialhub.AdapterConfig) { config.Adapter = "other" }},
		{"base URL", func(config *socialhub.AdapterConfig) { config.Settings["base_url"] = "https://user:pass@example.test" }},
		{"token URL query", func(config *socialhub.AdapterConfig) { config.Settings["token_url"] = "https://example.test/token?x=1" }},
		{"user agent", func(config *socialhub.AdapterConfig) { config.Settings["user_agent"] = "bad\nagent" }},
		{"username", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["username"] = "bad/name" }},
		{"unknown adapter setting", func(config *socialhub.AdapterConfig) { config.Settings["other"] = true }},
		{"unknown account setting", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["other"] = true }},
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
	if _, err := adapter.OAuth(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing OAuth=%v", err)
	}
	if _, err := adapter.Client(context.Background(), "creator"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("unresolved token=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "creator"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("unresolved secret=%v", err)
	}

	missingReference := cloneConfig(valid)
	missingReference.Accounts[0].AccessTokenRef = ""
	missingAdapter := &Adapter{}
	if err := missingAdapter.Init(context.Background(), missingReference); err != nil {
		t.Fatal(err)
	}
	if _, err := missingAdapter.Client(context.Background(), "creator"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("missing token reference=%v", err)
	}
	missingReference = cloneConfig(valid)
	missingReference.Accounts[0].ClientID = ""
	missingAdapter = &Adapter{}
	if err := missingAdapter.Init(context.Background(), missingReference); err != nil {
		t.Fatal(err)
	}
	if _, err := missingAdapter.OAuth(context.Background(), "creator"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("missing client ID=%v", err)
	}
}

func TestUnknownProStatusAndPerClientOptions(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newTestClient(t, server, "")
	capabilities, _ := client.Capabilities(context.Background())
	if capabilities[CapabilityProUpload].Approval != socialhub.ApprovalUnknown {
		t.Fatalf("Pro approval=%s", capabilities[CapabilityProUpload].Approval)
	}
	common, err := adapter.Client(context.Background(), "creator", socialhub.WithClock(fixedClock{now: testNow.Add(time.Hour)}))
	if err != nil || !common.(*Client).clock.Now().Equal(testNow.Add(time.Hour)) {
		t.Fatalf("client=%#v err=%v", common, err)
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

func writeJSON(writer http.ResponseWriter, status int, body string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_, _ = writer.Write([]byte(body))
}

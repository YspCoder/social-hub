package listenbrainz

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"social-hub/pkg/socialhub"
)

type mapResolver map[string]string

func (resolver mapResolver) Resolve(_ context.Context, reference string) (string, error) {
	value, ok := resolver[reference]
	if !ok {
		return "", errors.New("missing test secret")
	}
	return value, nil
}

func testConfig(server *httptest.Server) socialhub.AdapterConfig {
	return socialhub.AdapterConfig{
		Adapter:  adapterName,
		Settings: map[string]any{"base_url": server.URL + "/api"},
		Accounts: []socialhub.AccountConfig{
			{ID: "public", Settings: map[string]any{"username": "rob"}},
			{ID: "private", AccessTokenRef: "listenbrainz-token", Settings: map[string]any{"username": "private-user"}},
		},
	}
}

func newTestClients(t *testing.T, server *httptest.Server) (*Adapter, *Client, *Client) {
	t.Helper()
	adapter := &Adapter{}
	err := adapter.Init(
		context.Background(), testConfig(server), socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{"listenbrainz-token": "test-token"}),
	)
	if err != nil {
		t.Fatal(err)
	}
	publicCommon, err := adapter.Client(context.Background(), "public")
	if err != nil {
		t.Fatal(err)
	}
	privateCommon, err := adapter.Client(context.Background(), "private")
	if err != nil {
		t.Fatal(err)
	}
	return adapter, publicCommon.(*Client), privateCommon.(*Client)
}

func writeJSON(writer http.ResponseWriter, status int, body string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_, _ = writer.Write([]byte(body))
}

func TestAdapterLifecycleRegistrationAndCapabilities(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, public, private := newTestClients(t, server)

	metadata := adapter.Metadata()
	if adapter.Name() != adapterName || metadata.Name != adapterName || metadata.Product != productName ||
		metadata.APIVersion != apiVersion || metadata.DocURL != documentationURL || metadata.VerifiedAt.IsZero() {
		t.Fatalf("unexpected metadata: %#v", metadata)
	}
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatal("ListenBrainz adapter was not registered")
	}
	publicCapabilities, err := public.Capabilities(context.Background())
	if err != nil || !publicCapabilities.Has(CapabilityListening) || !publicCapabilities.Has(CapabilityFeedback) ||
		publicCapabilities.Has(CapabilitySubmission) || publicCapabilities[CapabilitySubmission].Approval != socialhub.ApprovalRequired {
		t.Fatalf("unexpected public capabilities: %#v, %v", publicCapabilities, err)
	}
	privateCapabilities, err := private.Capabilities(context.Background())
	if err != nil || !privateCapabilities.Has(CapabilityAuth) || !privateCapabilities.Has(CapabilitySubmission) ||
		!privateCapabilities.Has(CapabilityFeedbackWrite) || !privateCapabilities.Has(CapabilityPlaylist) {
		t.Fatalf("unexpected private capabilities: %#v, %v", privateCapabilities, err)
	}
	if public.Platform() != "listenbrainz" || public.Account() != "public" || public.AuthWorkflow() == nil ||
		public.ListeningWorkflow() == nil || public.FeedbackWorkflow() == nil || public.PlaylistWorkflow() == nil {
		t.Fatal("unexpected client identity or workflows")
	}
	if _, ok := public.Publisher(); ok {
		t.Fatal("publisher unexpectedly exposed")
	}
	if _, ok := public.Fetcher(); ok {
		t.Fatal("fetcher unexpectedly exposed")
	}
	if _, ok := public.MediaUploader(); ok {
		t.Fatal("media uploader unexpectedly exposed")
	}
	if _, ok := public.Reactor(); ok {
		t.Fatal("reactor unexpectedly exposed")
	}
	if _, ok := public.Messenger(); ok {
		t.Fatal("messenger unexpectedly exposed")
	}
	if _, ok := public.WebhookHandler(); ok {
		t.Fatal("webhook unexpectedly exposed")
	}
	if err := public.Close(); err != nil {
		t.Fatal(err)
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "public"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("expected closed adapter error, got %v", err)
	}
	if err := adapter.Init(context.Background(), testConfig(server)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("expected closed init error, got %v", err)
	}
}

func TestConfigurationAndClientValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	if _, err := (&Adapter{}).Client(context.Background(), "public"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("expected uninitialized adapter error, got %v", err)
	}
	base := testConfig(server)
	tests := []struct {
		name   string
		mutate func(*socialhub.AdapterConfig)
	}{
		{"adapter mismatch", func(config *socialhub.AdapterConfig) { config.Adapter = "other" }},
		{"bad base", func(config *socialhub.AdapterConfig) { config.Settings["base_url"] = "relative" }},
		{"unknown setting", func(config *socialhub.AdapterConfig) { config.Settings["unknown"] = true }},
		{"client id", func(config *socialhub.AdapterConfig) { config.Accounts[0].ClientID = "client" }},
		{"secret ref", func(config *socialhub.AdapterConfig) { config.Accounts[0].SecretRef = "secret" }},
		{"webhook", func(config *socialhub.AdapterConfig) { config.Accounts[0].Webhook.SecretRef = "secret" }},
		{"scope", func(config *socialhub.AdapterConfig) { config.Accounts[0].Approval.Scopes = []string{"write"} }},
		{"bad username", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["username"] = "bad/name" }},
		{"unknown account setting", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["unknown"] = true }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := cloneConfig(base)
			test.mutate(&config)
			if err := (&Adapter{}).Init(context.Background(), config); !errors.Is(err, socialhub.ErrInvalidArgument) {
				t.Fatalf("expected invalid argument, got %v", err)
			}
		})
	}

	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), base, socialhub.WithSecretResolver(mapResolver{})); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("expected missing account, got %v", err)
	}
	if _, err := adapter.Client(context.Background(), "private"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("expected missing secret error, got %v", err)
	}
	badTokenAdapter := &Adapter{}
	if err := badTokenAdapter.Init(context.Background(), base, socialhub.WithSecretResolver(mapResolver{"listenbrainz-token": " bad "})); err != nil {
		t.Fatal(err)
	}
	if _, err := badTokenAdapter.Client(context.Background(), "private"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("expected invalid token error, got %v", err)
	}
}

func cloneConfig(config socialhub.AdapterConfig) socialhub.AdapterConfig {
	copyConfig := config
	copyConfig.Settings = make(map[string]any, len(config.Settings))
	for key, value := range config.Settings {
		copyConfig.Settings[key] = value
	}
	copyConfig.Accounts = append([]socialhub.AccountConfig(nil), config.Accounts...)
	for index := range copyConfig.Accounts {
		settings := make(map[string]any, len(config.Accounts[index].Settings))
		for key, value := range config.Accounts[index].Settings {
			settings[key] = value
		}
		copyConfig.Accounts[index].Settings = settings
		copyConfig.Accounts[index].Approval.Scopes = append([]string(nil), config.Accounts[index].Approval.Scopes...)
	}
	return copyConfig
}

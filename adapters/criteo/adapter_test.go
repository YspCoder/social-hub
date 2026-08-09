package criteo

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestAdapterMetadataRegistryAndCapabilities(t *testing.T) {
	metadata := (&Adapter{}).Metadata()
	if metadata.Name != adapterName || metadata.Product != productName || metadata.APIVersion != apiVersion || metadata.DocURL == "" || metadata.VerifiedAt.IsZero() {
		t.Fatalf("metadata=%#v", metadata)
	}
	found := false
	for _, name := range socialhub.Adapters() {
		found = found || name == adapterName
	}
	if !found {
		t.Fatalf("adapter %q not registered", adapterName)
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newStaticClient(t, server)
	capabilities, err := client.Capabilities(context.Background())
	if err != nil || !capabilities[CapabilityCampaignManagement].Supported || !capabilities[CapabilityReporting].Supported {
		t.Fatalf("capabilities=%#v err=%v", capabilities, err)
	}
	for _, capability := range []socialhub.Capability{socialhub.CapPublish, socialhub.CapFetch, socialhub.CapMedia, socialhub.CapReact, socialhub.CapMessage, socialhub.CapWebhook} {
		if capabilities[capability].Supported {
			t.Errorf("capability %s unexpectedly supported", capability)
		}
	}
	if client.Platform() != platformName || client.Account() != testAccountID || client.Close() != nil {
		t.Fatalf("client identity is invalid")
	}
	if _, ok := client.Publisher(); ok {
		t.Fatal("Publisher unexpectedly supported")
	}
	if _, ok := client.Fetcher(); ok {
		t.Fatal("Fetcher unexpectedly supported")
	}
	if _, ok := client.MediaUploader(); ok {
		t.Fatal("MediaUploader unexpectedly supported")
	}
	if _, ok := client.Reactor(); ok {
		t.Fatal("Reactor unexpectedly supported")
	}
	if _, ok := client.Messenger(); ok {
		t.Fatal("Messenger unexpectedly supported")
	}
	if _, ok := client.WebhookHandler(); ok {
		t.Fatal("Webhook unexpectedly supported")
	}
	if client.Advertisers() == nil || client.Campaigns() == nil || client.AdSets() == nil || client.Statistics() == nil {
		t.Fatal("typed workflows are unavailable")
	}
	if _, err := adapter.OAuth(context.Background(), testAccountID); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("static OAuth error=%v", err)
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), testAccountID); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("closed client error=%v", err)
	}
	if err := adapter.Init(context.Background(), staticConfig(server.URL)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("closed init error=%v", err)
	}
}

func TestAdapterValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	valid := staticConfig(server.URL)
	tests := []struct {
		name   string
		mutate func(*socialhub.AdapterConfig)
	}{
		{"missing accounts", func(config *socialhub.AdapterConfig) { config.Accounts = nil }},
		{"adapter", func(config *socialhub.AdapterConfig) { config.Adapter = "criteo/wrong" }},
		{"product", func(config *socialhub.AdapterConfig) { config.Product = "wrong" }},
		{"base URL", func(config *socialhub.AdapterConfig) {
			config.Settings["base_url"] = "https://user:secret@example.test/"
		}},
		{"token URL", func(config *socialhub.AdapterConfig) { config.Settings["token_url"] = server.URL + "/token/" }},
		{"settings field", func(config *socialhub.AdapterConfig) { config.Settings["unknown"] = true }},
		{"advertiser", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["advertiser_id"] = "abc" }},
		{"account field", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["unknown"] = true }},
		{"no auth", func(config *socialhub.AdapterConfig) { config.Accounts[0].AccessTokenRef = "" }},
		{"mixed auth", func(config *socialhub.AdapterConfig) {
			config.Accounts[0].ClientID, config.Accounts[0].SecretRef = "id", "secret://ref"
		}},
		{"partial managed auth", func(config *socialhub.AdapterConfig) {
			config.Accounts[0].AccessTokenRef, config.Accounts[0].ClientID = "", "id"
		}},
		{"webhook", func(config *socialhub.AdapterConfig) { config.Accounts[0].Webhook.SecretRef = "secret://webhook" }},
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
}

func TestAdapterClientValidationAndOpen(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assertAPIRequest(t, request)
		writeJSON(t, writer, http.StatusOK, successEnvelope([]any{map[string]any{
			"type": "advertiser", "id": testAdvertiserID,
			"attributes": map[string]any{"advertiserName": "Example"},
		}}))
	}))
	defer server.Close()
	config := staticConfig(server.URL)
	adapter, err := socialhub.Open(context.Background(), adapterName, config,
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{"test://access-token": testAccessToken}),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer adapter.Close()
	client, err := adapter.Client(context.Background(), testAccountID)
	if err != nil {
		t.Fatal(err)
	}
	advertiser, err := client.(*Client).ValidateConfiguredAdvertiser(context.Background())
	if err != nil || advertiser.ID != testAdvertiserID {
		t.Fatalf("advertiser=%#v err=%v", advertiser, err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing account error=%v", err)
	}
	badResolver := &Adapter{}
	if err := badResolver.Init(context.Background(), config, socialhub.WithSecretResolver(mapResolver{})); err != nil {
		t.Fatal(err)
	}
	if _, err := badResolver.Client(context.Background(), testAccountID); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("secret error=%v", err)
	}
}

func cloneConfig(input socialhub.AdapterConfig) socialhub.AdapterConfig {
	result := input
	result.Settings = make(map[string]any, len(input.Settings))
	for key, value := range input.Settings {
		result.Settings[key] = value
	}
	result.Accounts = append([]socialhub.AccountConfig(nil), input.Accounts...)
	for index := range result.Accounts {
		settings := make(map[string]any, len(input.Accounts[index].Settings))
		for key, value := range input.Accounts[index].Settings {
			settings[key] = value
		}
		result.Accounts[index].Settings = settings
	}
	return result
}

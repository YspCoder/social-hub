package amazonads

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestRegistrationMetadataCapabilitiesAndLifecycle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatalf("adapter not registered: %v", socialhub.Adapters())
	}
	adapter, client := newTestAdapter(t, server)
	metadata := adapter.Metadata()
	if adapter.Name() != adapterName || metadata.APIVersion != apiVersion || metadata.Product != productName || metadata.DocURL == "" {
		t.Fatalf("metadata=%#v", metadata)
	}
	if client.Platform() != platformName || client.Account() != "retail-us" || client.profileID != testProfileID || client.region != RegionNA {
		t.Fatalf("client=%#v", client)
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil || !capabilities.Has(CapabilitySponsoredProducts) || !capabilities.Has(CapabilityReportingV3) {
		t.Fatalf("capabilities=%#v err=%v", capabilities, err)
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
	if client.Profiles() == nil || client.Campaigns() == nil || client.AdGroups() == nil || client.ProductAds() == nil || client.Keywords() == nil || client.Reports() == nil {
		t.Fatal("typed workflow missing")
	}
	if client.Close() != nil || adapter.Close() != nil || adapter.Close() != nil {
		t.Fatal("close failed")
	}
	if _, err := adapter.Client(context.Background(), "retail-us"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close=%v", err)
	}
	if err := adapter.Init(context.Background(), testConfig(server.URL)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("init after close=%v", err)
	}
}

func TestAdapterValidationAndApproval(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("unexpected request") }))
	defer server.Close()
	base := testConfig(server.URL)
	cases := []func(*socialhub.AdapterConfig){
		func(config *socialhub.AdapterConfig) { config.Adapter = "wrong" },
		func(config *socialhub.AdapterConfig) { config.Product = "wrong" },
		func(config *socialhub.AdapterConfig) { config.Settings["base_url"] = "https://user:secret@example.com" },
		func(config *socialhub.AdapterConfig) { config.Accounts[0].ClientID = "" },
		func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["profile_id"] = "bad" },
		func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["region"] = "AP" },
		func(config *socialhub.AdapterConfig) { config.Accounts[0].Webhook.SecretRef = "test://hook" },
	}
	for index, mutate := range cases {
		config := base
		config.Settings = cloneMap(base.Settings)
		config.Accounts = append([]socialhub.AccountConfig(nil), base.Accounts...)
		config.Accounts[0].Settings = cloneMap(base.Accounts[0].Settings)
		mutate(&config)
		if err := (&Adapter{}).Init(context.Background(), config); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Errorf("case %d error=%v", index, err)
		}
	}

	config := testConfig(server.URL)
	config.Accounts[0].Approval.Scopes = []string{"wrong"}
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), config, socialhub.WithHTTPClient(server.Client()), socialhub.WithSecretResolver(mapResolver{"test://access-token": "access-token"})); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "retail-us")
	if err != nil {
		t.Fatal(err)
	}
	client := common.(*Client)
	capabilities, _ := client.Capabilities(context.Background())
	if capabilities.Has(CapabilitySponsoredProducts) || capabilities[CapabilitySponsoredProducts].Approval != socialhub.ApprovalRequired {
		t.Fatalf("approval=%#v", capabilities[CapabilitySponsoredProducts])
	}
	_, err = client.ListProfiles(context.Background())
	if !errors.Is(err, socialhub.ErrApprovalRequired) || hubError(t, err).RequiredScopes[0] != managementScope {
		t.Fatalf("approval error=%v", err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing account=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing OAuth account=%v", err)
	}
}

func cloneMap(input map[string]any) map[string]any {
	result := make(map[string]any, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

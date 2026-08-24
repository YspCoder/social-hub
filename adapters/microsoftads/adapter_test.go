package microsoftads

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestAdapterRegistrationMetadataCapabilitiesAndLifecycle(t *testing.T) {
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatalf("registered adapters=%v", socialhub.Adapters())
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newTestAdapter(t, server)
	metadata := adapter.Metadata()
	if adapter.Name() != adapterName || metadata.APIVersion != apiVersion || metadata.Product != productName || metadata.DocURL != documentationURL || metadata.VerifiedAt.IsZero() {
		t.Fatalf("metadata=%#v", metadata)
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil || !capabilities.Has(CapabilityAdsManagement) || !capabilities.Has(CapabilityReports) {
		t.Fatalf("capabilities=%#v err=%v", capabilities, err)
	}
	for _, capability := range []socialhub.Capability{socialhub.CapPublish, socialhub.CapFetch, socialhub.CapMedia, socialhub.CapReact, socialhub.CapMessage, socialhub.CapWebhook} {
		if capabilities.Has(capability) {
			t.Fatalf("unsupported capability %q=%#v", capability, capabilities[capability])
		}
	}
	if client.Accounts() == nil || client.Campaigns() == nil || client.AdGroups() == nil || client.Ads() == nil || client.Keywords() == nil || client.Reports() == nil ||
		client.Platform() != platformName || client.Account() != "brand-search" {
		t.Fatalf("client=%#v", client)
	}
	if _, ok := client.Publisher(); ok {
		t.Fatal("publisher exposed")
	}
	if _, ok := client.Fetcher(); ok {
		t.Fatal("fetcher exposed")
	}
	if _, ok := client.MediaUploader(); ok {
		t.Fatal("media exposed")
	}
	if _, ok := client.Reactor(); ok {
		t.Fatal("reactor exposed")
	}
	if _, ok := client.Messenger(); ok {
		t.Fatal("messenger exposed")
	}
	if _, ok := client.WebhookHandler(); ok {
		t.Fatal("webhook exposed")
	}
	if client.Close() != nil || adapter.Close() != nil {
		t.Fatal("close failed")
	}
	if _, err := adapter.Client(context.Background(), "brand-search"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "brand-search"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("OAuth after close=%v", err)
	}
	if err := adapter.Init(context.Background(), testConfig(server.URL)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("reinit=%v", err)
	}
}

func TestAdapterValidationScopeAndSecretFailures(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	base := testConfig(server.URL)
	tests := []struct {
		name   string
		mutate func(*socialhub.AdapterConfig)
	}{
		{"config", func(config *socialhub.AdapterConfig) { config.Accounts = nil }},
		{"name", func(config *socialhub.AdapterConfig) { config.Adapter = "wrong" }},
		{"product", func(config *socialhub.AdapterConfig) { config.Product = "organic" }},
		{"campaign endpoint", func(config *socialhub.AdapterConfig) {
			config.Settings["campaign_base_url"] = "https://user:pass@example.com"
		}},
		{"customer endpoint", func(config *socialhub.AdapterConfig) { config.Settings["customer_base_url"] = "relative" }},
		{"report endpoint", func(config *socialhub.AdapterConfig) { config.Settings["reporting_base_url"] = "ftp://example.com" }},
		{"report bytes", func(config *socialhub.AdapterConfig) { config.Settings["max_report_bytes"] = 0 }},
		{"global unknown", func(config *socialhub.AdapterConfig) { config.Settings["unknown"] = true }},
		{"access token", func(config *socialhub.AdapterConfig) { config.Accounts[0].AccessTokenRef = "" }},
		{"customer", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["customer_id"] = "bad" }},
		{"account", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["customer_account_id"] = "0" }},
		{"developer ref", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["developer_token_ref"] = "" }},
		{"account unknown", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["unknown"] = true }},
		{"webhook", func(config *socialhub.AdapterConfig) { config.Accounts[0].Webhook.SecretRef = "secret://webhook" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := base
			config.Settings = cloneMap(base.Settings)
			config.Accounts = append([]socialhub.AccountConfig(nil), base.Accounts...)
			config.Accounts[0].Settings = cloneMap(base.Accounts[0].Settings)
			test.mutate(&config)
			if err := (&Adapter{}).Init(context.Background(), config); !errors.Is(err, socialhub.ErrInvalidArgument) {
				t.Fatalf("error=%v", err)
			}
		})
	}
	adapter, client := newTestAdapter(t, server)
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing client=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing OAuth=%v", err)
	}
	client.scopes = []string{"openid"}
	if _, err := client.GetAccount(context.Background()); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("scope gate=%v", err)
	}
	adapter.options.Secrets = mapResolver{"test://access-token": "access-token"}
	if _, err := adapter.Client(context.Background(), "brand-search"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("developer secret=%v", err)
	}
	adapter.options.Secrets = mapResolver{"test://developer-token": testDeveloperToken}
	if _, err := adapter.Client(context.Background(), "brand-search"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("access secret=%v", err)
	}
	adapter.config.Accounts[0].ClientID = ""
	if _, err := adapter.OAuth(context.Background(), "brand-search"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("OAuth config=%v", err)
	}
	if _, err := (&Adapter{}).Client(context.Background(), "brand-search"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("uninitialized client=%v", err)
	}
	if _, err := (&Adapter{}).OAuth(context.Background(), "brand-search"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("uninitialized OAuth=%v", err)
	}
}

package tencentads

import (
	"context"
	"net/http/httptest"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestAdapterMetadataCapabilitiesAndLifecycle(t *testing.T) {
	server := httptest.NewServer(nil)
	defer server.Close()
	adapter, client := newTestAdapter(t, server)
	metadata := adapter.Metadata()
	if adapter.Name() != adapterName || metadata.Name != adapterName || metadata.APIVersion != apiVersion || metadata.VerifiedAt.IsZero() {
		t.Fatalf("unexpected metadata: %#v", metadata)
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil || !capabilities.Has(CapabilityMarketingManagement) || !capabilities.Has(CapabilityMarketingReports) || capabilities.Has(socialhub.CapPublish) {
		t.Fatalf("capabilities=%#v err=%v", capabilities, err)
	}
	if client.Platform() != platformName || client.Account() != "ads-primary" || client.Advertisers() == nil ||
		client.Campaigns() == nil || client.AdGroups() == nil || client.AdCreatives() == nil || client.Reports() == nil {
		t.Fatal("client identity or typed workflows are invalid")
	}
	if publisher, ok := client.Publisher(); ok || publisher != nil {
		t.Fatal("publisher must be unavailable")
	}
	if fetcher, ok := client.Fetcher(); ok || fetcher != nil {
		t.Fatal("fetcher must be unavailable")
	}
	if uploader, ok := client.MediaUploader(); ok || uploader != nil {
		t.Fatal("media uploader must be unavailable")
	}
	if reactor, ok := client.Reactor(); ok || reactor != nil {
		t.Fatal("reactor must be unavailable")
	}
	if messenger, ok := client.Messenger(); ok || messenger != nil {
		t.Fatal("messenger must be unavailable")
	}
	if webhook, ok := client.WebhookHandler(); ok || webhook != nil {
		t.Fatal("webhook must be unavailable")
	}
	if err := client.Close(); err != nil || adapter.Close() != nil {
		t.Fatal("close failed")
	}
	if _, err := adapter.Client(context.Background(), "ads-primary"); hubError(t, err).Code != socialhub.CodeConflict {
		t.Fatalf("closed client error=%v", err)
	}
	if err := adapter.Init(context.Background(), testConfig(server.URL)); hubError(t, err).Code != socialhub.CodeConflict {
		t.Fatalf("reinit error=%v", err)
	}
}

func TestAdapterValidation(t *testing.T) {
	server := httptest.NewServer(nil)
	defer server.Close()
	base := testConfig(server.URL)
	tests := []struct {
		name   string
		mutate func(*socialhub.AdapterConfig)
	}{
		{"name", func(config *socialhub.AdapterConfig) { config.Adapter = "wrong" }},
		{"product", func(config *socialhub.AdapterConfig) { config.Product = "organic" }},
		{"api endpoint", func(config *socialhub.AdapterConfig) { config.Settings["base_url"] = "https://user:pass@example.com" }},
		{"auth endpoint", func(config *socialhub.AdapterConfig) { config.Settings["authorization_base_url"] = "relative" }},
		{"token endpoint", func(config *socialhub.AdapterConfig) { config.Settings["token_base_url"] = "https://example.com?q=1" }},
		{"token", func(config *socialhub.AdapterConfig) { config.Accounts[0].AccessTokenRef = "" }},
		{"account", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["account_id"] = 0 }},
		{"webhook", func(config *socialhub.AdapterConfig) { config.Accounts[0].Webhook.SecretRef = "secret://webhook" }},
		{"unknown setting", func(config *socialhub.AdapterConfig) { config.Settings["unknown"] = true }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := base
			config.Settings = cloneMap(base.Settings)
			config.Accounts = append([]socialhub.AccountConfig(nil), base.Accounts...)
			config.Accounts[0].Settings = cloneMap(base.Accounts[0].Settings)
			test.mutate(&config)
			if err := (&Adapter{}).Init(context.Background(), config); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestAdapterClientAndOAuthFailures(t *testing.T) {
	server := httptest.NewServer(nil)
	defer server.Close()
	adapter, _ := newTestAdapter(t, server)
	if _, err := adapter.Client(context.Background(), "missing"); hubError(t, err).Code != socialhub.CodeNotFound {
		t.Fatalf("unknown account error=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "missing"); hubError(t, err).Code != socialhub.CodeNotFound {
		t.Fatalf("unknown OAuth account error=%v", err)
	}
	config := testConfig(server.URL)
	config.Accounts[0].AppID = "not-numeric"
	broken := &Adapter{}
	if err := broken.Init(context.Background(), config,
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{"test://access-token": "access-token", "test://app-secret": "secret"}),
	); err != nil {
		t.Fatal(err)
	}
	if _, err := broken.OAuth(context.Background(), "ads-primary"); hubError(t, err).Code != socialhub.CodeInvalidArgument {
		t.Fatalf("OAuth validation error=%v", err)
	}
	adapter.Close()
	if _, err := adapter.OAuth(context.Background(), "ads-primary"); hubError(t, err).Code != socialhub.CodeConflict {
		t.Fatalf("closed OAuth error=%v", err)
	}
}

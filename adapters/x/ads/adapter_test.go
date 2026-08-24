package ads

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestAdapterRegistrationMetadataCapabilitiesAndLifecycle(t *testing.T) {
	found := false
	for _, name := range socialhub.Adapters() {
		found = found || name == adapterName
	}
	if !found {
		t.Fatal("adapter was not registered")
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newTestAdapter(t, server)
	metadata := adapter.Metadata()
	if adapter.Name() != adapterName || metadata.Product != productName || metadata.APIVersion != apiVersion || metadata.DocURL == "" || metadata.VerifiedAt.IsZero() {
		t.Fatalf("metadata=%#v", metadata)
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil || !capabilities[CapabilityAdsManagement].Supported || capabilities[CapabilityAdsManagement].Approval != socialhub.ApprovalGranted ||
		!capabilities[CapabilityAdsReporting].Supported || capabilities.Has(socialhub.CapPublish) {
		t.Fatalf("capabilities=%#v err=%v", capabilities, err)
	}
	if client.Platform() != platformName || client.Account() != "paid-social" || client.Accounts() == nil || client.Campaigns() == nil ||
		client.LineItems() == nil || client.PromotedTweets() == nil || client.Stats() == nil || client.Close() != nil {
		t.Fatal("client identity or typed workflows are unavailable")
	}
	if value, ok := client.Publisher(); ok || value != nil {
		t.Fatal("Publisher unexpectedly supported")
	}
	if value, ok := client.Fetcher(); ok || value != nil {
		t.Fatal("Fetcher unexpectedly supported")
	}
	if value, ok := client.MediaUploader(); ok || value != nil {
		t.Fatal("MediaUploader unexpectedly supported")
	}
	if value, ok := client.Reactor(); ok || value != nil {
		t.Fatal("Reactor unexpectedly supported")
	}
	if value, ok := client.Messenger(); ok || value != nil {
		t.Fatal("Messenger unexpectedly supported")
	}
	if value, ok := client.WebhookHandler(); ok || value != nil {
		t.Fatal("WebhookHandler unexpectedly supported")
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "paid-social"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close error=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "paid-social"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("OAuth after close error=%v", err)
	}
	if err := adapter.Init(context.Background(), testConfig(server.URL)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("re-init after close error=%v", err)
	}
}

func TestAdapterConfigurationAndSecretValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	invalid := []socialhub.AdapterConfig{
		{},
		func() socialhub.AdapterConfig {
			value := testConfig(server.URL)
			value.Adapter = "x/wrong"
			return value
		}(),
		func() socialhub.AdapterConfig { value := testConfig(server.URL); value.Product = "api"; return value }(),
		func() socialhub.AdapterConfig {
			value := testConfig(server.URL)
			value.Settings["base_url"] = "https://user:pass@example.com"
			return value
		}(),
		func() socialhub.AdapterConfig {
			value := testConfig(server.URL)
			value.Accounts[0].ClientID = ""
			return value
		}(),
		func() socialhub.AdapterConfig {
			value := testConfig(server.URL)
			value.Accounts[0].SecretRef = ""
			return value
		}(),
		func() socialhub.AdapterConfig {
			value := testConfig(server.URL)
			value.Accounts[0].AccessTokenRef = ""
			return value
		}(),
		func() socialhub.AdapterConfig {
			value := testConfig(server.URL)
			value.Accounts[0].Settings["ads_account_id"] = "INVALID"
			return value
		}(),
		func() socialhub.AdapterConfig {
			value := testConfig(server.URL)
			value.Accounts[0].Settings["access_token_secret_ref"] = ""
			return value
		}(),
		func() socialhub.AdapterConfig {
			value := testConfig(server.URL)
			value.Accounts[0].Webhook.SecretRef = "secret"
			return value
		}(),
	}
	for index, config := range invalid {
		if err := (&Adapter{}).Init(context.Background(), config); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Errorf("config %d error=%v", index, err)
		}
	}
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server.URL),
		socialhub.WithHTTPClient(server.Client()), socialhub.WithSecretResolver(mapResolver{})); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "paid-social"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("client secret error=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "paid-social"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("OAuth secret error=%v", err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing client error=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing OAuth error=%v", err)
	}
}

func TestCapabilitiesExposeUnknownAndRequiredAccess(t *testing.T) {
	for _, accountType := range []string{"", "basic_access"} {
		server := httptest.NewServer(http.NotFoundHandler())
		config := testConfig(server.URL)
		config.Accounts[0].Approval.AccountType = accountType
		adapter := &Adapter{}
		if err := adapter.Init(context.Background(), config,
			socialhub.WithHTTPClient(server.Client()),
			socialhub.WithSecretResolver(mapResolver{
				"test://consumer-secret": "consumer-secret", "test://access-token": "access-token", "test://access-token-secret": "access-token-secret",
			})); err != nil {
			t.Fatal(err)
		}
		common, err := adapter.Client(context.Background(), "paid-social")
		if err != nil {
			t.Fatal(err)
		}
		capabilities, _ := common.Capabilities(context.Background())
		want := socialhub.ApprovalUnknown
		if accountType != "" {
			want = socialhub.ApprovalRequired
		}
		if capabilities[CapabilityAdsManagement].Approval != want {
			t.Fatalf("accountType=%q capabilities=%#v", accountType, capabilities)
		}
		server.Close()
	}
}

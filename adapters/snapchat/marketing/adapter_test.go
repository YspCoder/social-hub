package marketing

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
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writeValue(t, writer, http.StatusOK, successEnvelope("adaccounts", "adaccount", map[string]any{"id": testAdAccountID}))
	}))
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
	if client.Platform() != platformName || client.Account() != "paid-social" {
		t.Fatalf("client identity=%s/%s", client.Platform(), client.Account())
	}
	if publisher, ok := client.Publisher(); ok || publisher != nil {
		t.Fatal("common Publisher unexpectedly supported")
	}
	if fetcher, ok := client.Fetcher(); ok || fetcher != nil {
		t.Fatal("common Fetcher unexpectedly supported")
	}
	if uploader, ok := client.MediaUploader(); ok || uploader != nil {
		t.Fatal("common MediaUploader unexpectedly supported")
	}
	if reactor, ok := client.Reactor(); ok || reactor != nil {
		t.Fatal("common Reactor unexpectedly supported")
	}
	if messenger, ok := client.Messenger(); ok || messenger != nil {
		t.Fatal("common Messenger unexpectedly supported")
	}
	if webhook, ok := client.WebhookHandler(); ok || webhook != nil {
		t.Fatal("common WebhookHandler unexpectedly supported")
	}
	if client.AdAccounts() == nil || client.Campaigns() == nil || client.AdSquads() == nil || client.Ads() == nil || client.Stats() == nil || client.Close() != nil {
		t.Fatal("typed workflows unavailable")
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "paid-social"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close error=%v", err)
	}
	if err := adapter.Init(context.Background(), testConfig(server.URL)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("re-init after close error=%v", err)
	}
}

func TestAdapterValidationAndScopeGate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("scope gate should reject before network access")
	}))
	defer server.Close()

	invalidConfigs := []socialhub.AdapterConfig{
		{},
		func() socialhub.AdapterConfig {
			value := testConfig(server.URL)
			value.Adapter = "snapchat/wrong"
			return value
		}(),
		func() socialhub.AdapterConfig {
			value := testConfig(server.URL)
			value.Product = "organic"
			return value
		}(),
		func() socialhub.AdapterConfig {
			value := testConfig(server.URL)
			value.Settings["base_url"] = "https://user:pass@example.com"
			return value
		}(),
		func() socialhub.AdapterConfig {
			value := testConfig(server.URL)
			value.Accounts[0].AccessTokenRef = ""
			return value
		}(),
		func() socialhub.AdapterConfig {
			value := testConfig(server.URL)
			value.Accounts[0].Settings["ad_account_id"] = "bad"
			return value
		}(),
		func() socialhub.AdapterConfig {
			value := testConfig(server.URL)
			value.Accounts[0].Webhook.SecretRef = "secret"
			return value
		}(),
	}
	for index, config := range invalidConfigs {
		if err := (&Adapter{}).Init(context.Background(), config); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Errorf("config %d error=%v", index, err)
		}
	}

	config := testConfig(server.URL)
	config.Accounts[0].Approval.Scopes = []string{"snapchat-profile-api"}
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), config,
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{"test://access-token": "access-token", "test://client-secret": "client-secret"}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "paid-social")
	if err != nil {
		t.Fatal(err)
	}
	client := common.(*Client)
	if _, err := client.GetAdAccount(context.Background()); !errors.Is(err, socialhub.ErrApprovalRequired) || len(hubError(t, err).RequiredScopes) != 1 {
		t.Fatalf("scope error=%v", err)
	}
	capabilities, _ := client.Capabilities(context.Background())
	if capabilities[CapabilityAdsManagement].Approval != socialhub.ApprovalRequired {
		t.Fatalf("capabilities=%#v", capabilities)
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing account error=%v", err)
	}
}

func TestAdapterSecretAndOAuthValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	config := testConfig(server.URL)
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), config,
		socialhub.WithHTTPClient(server.Client()), socialhub.WithSecretResolver(mapResolver{})); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "paid-social"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("secret error=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "paid-social"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("OAuth secret error=%v", err)
	}
	config.Accounts[0].ClientID = ""
	adapter = &Adapter{}
	if err := adapter.Init(context.Background(), config,
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{"test://access-token": "access-token", "test://client-secret": "client-secret"}),
	); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.OAuth(context.Background(), "paid-social"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("OAuth config error=%v", err)
	}
}

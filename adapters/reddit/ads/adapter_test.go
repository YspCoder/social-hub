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
	if client.Platform() != platformName || client.Account() != "paid-social" || client.AdAccounts() == nil || client.FundingInstruments() == nil ||
		client.Campaigns() == nil || client.AdGroups() == nil || client.Ads() == nil || client.Reports() == nil || client.Close() != nil {
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
			value.Adapter = "reddit/wrong"
			return value
		}(),
		func() socialhub.AdapterConfig {
			value := testConfig(server.URL)
			value.Product = "data-api"
			return value
		}(),
		func() socialhub.AdapterConfig {
			value := testConfig(server.URL)
			value.Settings["base_url"] = "https://user:pass@example.com"
			return value
		}(),
		func() socialhub.AdapterConfig {
			value := testConfig(server.URL)
			value.Settings["user_agent"] = "Go-http-client/1.1"
			return value
		}(),
		func() socialhub.AdapterConfig {
			value := testConfig(server.URL)
			value.Accounts[0].AccessTokenRef = ""
			return value
		}(),
		func() socialhub.AdapterConfig {
			value := testConfig(server.URL)
			value.Accounts[0].Settings["ad_account_id"] = "wrong"
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

func TestCapabilitiesUnknownRequiredAndScopePrecheck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodGet && request.URL.Path == "/api/v3/campaigns/"+testCampaignID {
			writeJSON(t, writer, http.StatusOK, singleResponse[Campaign]{Data: campaignFixture("current", StatusPaused)})
			return
		}
		t.Fatalf("scope failure reached write network: %s %s", request.Method, request.URL)
	}))
	defer server.Close()
	for _, scopes := range [][]string{nil, {readScope}} {
		config := testConfig(server.URL)
		config.Accounts[0].Approval.Scopes = scopes
		adapter := &Adapter{}
		if err := adapter.Init(context.Background(), config,
			socialhub.WithHTTPClient(server.Client()),
			socialhub.WithSecretResolver(mapResolver{"test://client-secret": "secret", "test://access-token": "token"})); err != nil {
			t.Fatal(err)
		}
		common, err := adapter.Client(context.Background(), "paid-social")
		if err != nil {
			t.Fatal(err)
		}
		client := common.(*Client)
		capabilities, _ := client.Capabilities(context.Background())
		want := socialhub.ApprovalUnknown
		if scopes != nil {
			want = socialhub.ApprovalRequired
		}
		if capabilities[CapabilityAdsManagement].Approval != want {
			t.Fatalf("scopes=%v capabilities=%#v", scopes, capabilities)
		}
		if scopes != nil {
			_, err := client.UpdateCampaign(context.Background(), testCampaignID, UpdateCampaignRequest{Name: stringPointer("new")})
			if !errors.Is(err, socialhub.ErrApprovalRequired) || len(hubError(t, err).RequiredScopes) != 1 {
				t.Fatalf("scope error=%v", err)
			}
		}
	}
}

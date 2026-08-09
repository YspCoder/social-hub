package googleads

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
	if adapter.Name() != adapterName || metadata.APIVersion != apiVersion || metadata.Product != productName ||
		metadata.DocURL != documentationURL || metadata.VerifiedAt.IsZero() {
		t.Fatalf("metadata=%#v", metadata)
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil || !capabilities.Has(CapabilityAdsManagement) || !capabilities.Has(CapabilityGAQLReports) {
		t.Fatalf("capabilities=%#v err=%v", capabilities, err)
	}
	for _, capability := range []socialhub.Capability{
		socialhub.CapPublish, socialhub.CapFetch, socialhub.CapMedia,
		socialhub.CapReact, socialhub.CapMessage, socialhub.CapWebhook,
	} {
		if capabilities.Has(capability) {
			t.Fatalf("unsupported capability %q=%#v", capability, capabilities[capability])
		}
	}
	if client.Platform() != platformName || client.Account() != "brand-search" ||
		client.Customers() == nil || client.CampaignBudgets() == nil || client.Campaigns() == nil ||
		client.AdGroups() == nil || client.Ads() == nil || client.Reports() == nil {
		t.Fatalf("client=%#v", client)
	}
	if _, ok := client.Publisher(); ok {
		t.Fatal("publisher must not be exposed")
	}
	if _, ok := client.Fetcher(); ok {
		t.Fatal("fetcher must not be exposed")
	}
	if _, ok := client.MediaUploader(); ok {
		t.Fatal("media uploader must not be exposed")
	}
	if _, ok := client.Reactor(); ok {
		t.Fatal("reactor must not be exposed")
	}
	if _, ok := client.Messenger(); ok {
		t.Fatal("messenger must not be exposed")
	}
	if _, ok := client.WebhookHandler(); ok {
		t.Fatal("webhook handler must not be exposed")
	}
	if client.Close() != nil || adapter.Close() != nil {
		t.Fatal("close failed")
	}
	if _, err := adapter.Client(context.Background(), "brand-search"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close error=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "brand-search"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("OAuth after close error=%v", err)
	}
	if err := adapter.Init(context.Background(), testConfig(server.URL)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("reinit error=%v", err)
	}
}

func TestAdapterValidationAndClientFailures(t *testing.T) {
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
		{"api endpoint", func(config *socialhub.AdapterConfig) { config.Settings["base_url"] = "https://user:pass@example.com" }},
		{"auth endpoint", func(config *socialhub.AdapterConfig) { config.Settings["auth_url"] = "relative" }},
		{"token endpoint", func(config *socialhub.AdapterConfig) { config.Settings["token_url"] = "ftp://example.com" }},
		{"global unknown", func(config *socialhub.AdapterConfig) { config.Settings["unknown"] = true }},
		{"access token", func(config *socialhub.AdapterConfig) { config.Accounts[0].AccessTokenRef = "" }},
		{"customer", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["customer_id"] = "123-456-7890" }},
		{"manager", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["login_customer_id"] = "manager" }},
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

	adapter, _ := newTestAdapter(t, server)
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing account error=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing OAuth error=%v", err)
	}
	adapter.options.Secrets = mapResolver{"test://access-token": "access-token", "test://developer-token": "short"}
	if _, err := adapter.Client(context.Background(), "brand-search"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("developer token error=%v", err)
	}
	adapter.options.Secrets = mapResolver{"test://developer-token": testDeveloperToken}
	if _, err := adapter.Client(context.Background(), "brand-search"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("access token error=%v", err)
	}
	adapter.config.Accounts[0].ClientID = ""
	if _, err := adapter.OAuth(context.Background(), "brand-search"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("OAuth config error=%v", err)
	}
	if _, err := (&Adapter{}).Client(context.Background(), "brand-search"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("uninitialized client error=%v", err)
	}
	if _, err := (&Adapter{}).OAuth(context.Background(), "brand-search"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("uninitialized OAuth error=%v", err)
	}
}

func TestScopeGateAndDirectCustomerAuthentication(t *testing.T) {
	requestSeen := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestSeen = true
		if request.Header.Get("login-customer-id") != "" {
			t.Errorf("unexpected manager header=%q", request.Header.Get("login-customer-id"))
		}
		writeJSON(writer, http.StatusOK, `{"resourceNames":["customers/1234567890"]}`)
	}))
	defer server.Close()
	config := testConfig(server.URL)
	delete(config.Accounts[0].Settings, "login_customer_id")
	config.Accounts[0].Approval.Scopes = []string{"openid"}
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), config,
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{
			"test://access-token": "access-token", "test://developer-token": testDeveloperToken,
		}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "brand-search")
	if err != nil {
		t.Fatal(err)
	}
	client := common.(*Client)
	if _, err := client.ListAccessibleCustomers(context.Background()); !errors.Is(err, socialhub.ErrApprovalRequired) || requestSeen {
		t.Fatalf("scope error=%v requestSeen=%v", err, requestSeen)
	}
	client.scopes = []string{adwordsScope}
	if _, err := client.ListAccessibleCustomers(context.Background()); err != nil || !requestSeen {
		t.Fatalf("direct request err=%v requestSeen=%v", err, requestSeen)
	}
}

package cm360

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
		t.Fatalf("adapter %q is not registered", adapterName)
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newStaticClient(t, server)
	metadata := adapter.Metadata()
	if adapter.Name() != adapterName || metadata.Product != productName || metadata.APIVersion != apiVersion ||
		metadata.DocURL != documentationURL || metadata.VerifiedAt.IsZero() || discoveryRevision != "20260721" {
		t.Fatalf("metadata=%#v", metadata)
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil || !capabilities.Has(CapabilityCampaignManager360) {
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
	if client.Platform() != platformName || client.Account() != testAccountID || client.Profiles() == nil ||
		client.Advertisers() == nil || client.Campaigns() == nil || client.Placements() == nil ||
		client.Ads() == nil || client.Reporting() == nil {
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
	if _, err := adapter.Client(context.Background(), testAccountID); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close error=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), testAccountID); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("OAuth after close error=%v", err)
	}
	if err := adapter.Init(context.Background(), staticConfig(server.URL)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("reinit error=%v", err)
	}
}

func TestAdapterValidationAndFailures(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	base := staticConfig(server.URL)
	tests := []struct {
		name   string
		mutate func(*socialhub.AdapterConfig)
	}{
		{"config", func(config *socialhub.AdapterConfig) { config.Accounts = nil }},
		{"name", func(config *socialhub.AdapterConfig) { config.Adapter = "wrong" }},
		{"product", func(config *socialhub.AdapterConfig) { config.Product = "organic" }},
		{"api endpoint", func(config *socialhub.AdapterConfig) { config.Settings["base_url"] = "https://user:pass@example.com" }},
		{"auth endpoint", func(config *socialhub.AdapterConfig) { config.Settings["auth_url"] = "relative" }},
		{"token endpoint", func(config *socialhub.AdapterConfig) {
			config.Settings["token_url"] = "https://example.com/token?secret=x"
		}},
		{"global unknown", func(config *socialhub.AdapterConfig) { config.Settings["unknown"] = true }},
		{"profile", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["profile_id"] = "0" }},
		{"advertiser", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["advertiser_id"] = "bad" }},
		{"account unknown", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["unknown"] = true }},
		{"no auth", func(config *socialhub.AdapterConfig) {
			config.Accounts[0].AccessTokenRef, config.Accounts[0].ClientID, config.Accounts[0].SecretRef = "", "", ""
		}},
		{"static refresh", func(config *socialhub.AdapterConfig) {
			config.Accounts[0].Settings["refresh_token_ref"] = "test://refresh"
		}},
		{"partial OAuth", func(config *socialhub.AdapterConfig) { config.Accounts[0].SecretRef = "" }},
		{"webhook", func(config *socialhub.AdapterConfig) { config.Accounts[0].Webhook.SecretRef = "test://webhook" }},
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

	adapter, _ := newStaticClient(t, server)
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing account error=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing OAuth error=%v", err)
	}
	adapter.options.Secrets = mapResolver{"test://client-secret": "client-secret"}
	if _, err := adapter.Client(context.Background(), testAccountID); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("missing access token error=%v", err)
	}
	adapter.options.Secrets = mapResolver{"test://access-token": testAccessToken}
	if _, err := adapter.OAuth(context.Background(), testAccountID); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("missing client secret error=%v", err)
	}
	adapter.config.Accounts[0].ClientID = ""
	if _, err := adapter.OAuth(context.Background(), testAccountID); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("OAuth config error=%v", err)
	}
	if _, err := (&Adapter{}).Client(context.Background(), testAccountID); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("uninitialized client error=%v", err)
	}
	if _, err := (&Adapter{}).OAuth(context.Background(), testAccountID); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("uninitialized OAuth error=%v", err)
	}
}

func TestScopeGatesBeforeNetwork(t *testing.T) {
	requestSeen := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestSeen = true
		writeJSON(t, writer, http.StatusOK, profileResource())
	}))
	defer server.Close()
	config := staticConfig(server.URL)
	config.Accounts[0].Approval.Scopes = []string{"openid"}
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), config,
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{"test://access-token": testAccessToken, "test://client-secret": "secret"}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), testAccountID)
	if err != nil {
		t.Fatal(err)
	}
	client := common.(*Client)
	if _, err := client.GetProfile(context.Background()); !errors.Is(err, socialhub.ErrApprovalRequired) || requestSeen {
		t.Fatalf("profile scope error=%v requestSeen=%v", err, requestSeen)
	}
	if _, err := client.GetAdvertiser(context.Background()); !errors.Is(err, socialhub.ErrApprovalRequired) || requestSeen {
		t.Fatalf("advertiser scope error=%v requestSeen=%v", err, requestSeen)
	}
	if _, err := client.QueryReportData(context.Background(), validReportQuery()); !errors.Is(err, socialhub.ErrApprovalRequired) || requestSeen {
		t.Fatalf("report scope error=%v requestSeen=%v", err, requestSeen)
	}
}

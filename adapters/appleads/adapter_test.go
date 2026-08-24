package appleads

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
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newStaticClient(t, server)
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatalf("adapter %q is not registered", adapterName)
	}
	metadata := adapter.Metadata()
	if adapter.Name() != adapterName || metadata.Product != productName || metadata.APIVersion != apiVersion ||
		metadata.DocURL != documentationURL || metadata.VerifiedAt.IsZero() {
		t.Fatalf("metadata=%#v", metadata)
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil || !capabilities.Has(CapabilityAccountAccess) || !capabilities.Has(CapabilityCampaignManagement) ||
		!capabilities.Has(CapabilityCreativeManagement) || !capabilities.Has(CapabilityReporting) {
		t.Fatalf("capabilities=%#v err=%v", capabilities, err)
	}
	for _, capability := range []socialhub.Capability{
		socialhub.CapPublish, socialhub.CapFetch, socialhub.CapMedia,
		socialhub.CapReact, socialhub.CapMessage, socialhub.CapWebhook,
	} {
		if capabilities.Has(capability) {
			t.Fatalf("unsupported capability %q is enabled", capability)
		}
	}
	if client.Platform() != platformName || client.Account() != "search-us" || client.ACL() == nil || client.Campaigns() == nil ||
		client.AdGroups() == nil || client.Keywords() == nil || client.Creatives() == nil || client.Ads() == nil || client.Reports() == nil {
		t.Fatalf("client=%#v", client)
	}
	if _, ok := client.Publisher(); ok {
		t.Fatal("Publisher must not be exposed")
	}
	if _, ok := client.Fetcher(); ok {
		t.Fatal("Fetcher must not be exposed")
	}
	if _, ok := client.MediaUploader(); ok {
		t.Fatal("MediaUploader must not be exposed")
	}
	if _, ok := client.Reactor(); ok {
		t.Fatal("Reactor must not be exposed")
	}
	if _, ok := client.Messenger(); ok {
		t.Fatal("Messenger must not be exposed")
	}
	if _, ok := client.WebhookHandler(); ok {
		t.Fatal("WebhookHandler must not be exposed")
	}
	if client.Close() != nil || adapter.Close() != nil {
		t.Fatal("close failed")
	}
	if _, err := adapter.Client(context.Background(), "search-us"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("Client after close error=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "search-us"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("OAuth after close error=%v", err)
	}
	if err := adapter.Init(context.Background(), staticConfig(server.URL)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("reinitialize error=%v", err)
	}
}

func TestAdapterConfigurationValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	base := staticConfig(server.URL)
	tests := []struct {
		name   string
		mutate func(*socialhub.AdapterConfig)
	}{
		{"shared config", func(config *socialhub.AdapterConfig) { config.Accounts = nil }},
		{"adapter", func(config *socialhub.AdapterConfig) { config.Adapter = "wrong" }},
		{"product", func(config *socialhub.AdapterConfig) { config.Product = "organic" }},
		{"base endpoint", func(config *socialhub.AdapterConfig) { config.Settings["base_url"] = "https://user:pass@example.com" }},
		{"token endpoint", func(config *socialhub.AdapterConfig) { config.Settings["token_url"] = "https://example.com/" }},
		{"unknown global", func(config *socialhub.AdapterConfig) { config.Settings["unknown"] = true }},
		{"organization", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["org_id"] = 0 }},
		{"unknown account", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["unknown"] = true }},
		{"no credential", func(config *socialhub.AdapterConfig) { config.Accounts[0].AccessTokenRef = "" }},
		{"mixed credential", func(config *socialhub.AdapterConfig) {
			config.Accounts[0].ClientID = "client"
			config.Accounts[0].Settings["team_id"] = "team"
			config.Accounts[0].Settings["key_id"] = "key"
			config.Accounts[0].Settings["private_key_ref"] = "secret://key"
		}},
		{"static with managed field", func(config *socialhub.AdapterConfig) { config.Accounts[0].ClientID = "client" }},
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

	managed := managedConfig(server.URL)
	managed.Accounts[0].Settings["team_id"] = ""
	if err := (&Adapter{}).Init(context.Background(), managed); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("partial managed OAuth error=%v", err)
	}
	if _, err := (&Adapter{}).Client(context.Background(), "search-us"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("uninitialized Client error=%v", err)
	}
	if _, err := (&Adapter{}).OAuth(context.Background(), "search-us"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("uninitialized OAuth error=%v", err)
	}
}

func TestAdapterClientAuthenticationAndFailures(t *testing.T) {
	requestSeen := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestSeen = true
		assertAPIRequest(t, request)
		if request.URL.Path != "/api/v5/acls" || request.URL.Query().Get("offset") != "0" || request.URL.Query().Get("limit") != "20" ||
			request.Header.Get("X-Request-ID") != "request-1" {
			t.Errorf("request=%s %s headers=%v", request.Method, request.URL, request.Header)
		}
		writeJSON(t, writer, http.StatusOK, pagedEnvelope([]UserACL{{OrgID: testOrgID, OrgName: "Org"}}, 1))
	}))
	defer server.Close()
	adapter, client := newStaticClient(t, server)
	page, err := client.ListACL(context.Background(), Pagination{Limit: 20}, socialhub.WithRequestID("request-1"))
	if err != nil || !requestSeen || len(page.Items) != 1 || page.HasMore {
		t.Fatalf("page=%#v requestSeen=%v err=%v", page, requestSeen, err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing Client error=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing OAuth error=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "search-us"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("static OAuth error=%v", err)
	}
	adapter.options.Secrets = mapResolver{}
	if _, err := adapter.Client(context.Background(), "search-us"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("missing token error=%v", err)
	}
	adapter.options.Secrets = mapResolver{"test://access-token": " "}
	if _, err := adapter.Client(context.Background(), "search-us"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("blank token error=%v", err)
	}
}

func TestRedirectsAreRejected(t *testing.T) {
	redirectFollowed := false
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { redirectFollowed = true }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, target.URL, http.StatusFound)
	}))
	defer server.Close()
	_, client := newStaticClient(t, server)
	_, err := client.ListACL(context.Background(), Pagination{Limit: 1})
	if err == nil || redirectFollowed {
		t.Fatalf("error=%v redirectFollowed=%v", err, redirectFollowed)
	}
}

package simkl

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	testClientSecret = "simkl-client-secret"
	testAccessToken  = "simkl-access-token"
)

var testNow = time.Date(2026, time.August, 2, 8, 9, 10, 0, time.UTC)

type mapResolver map[string]string

func (r mapResolver) Resolve(_ context.Context, reference string) (string, error) {
	value, ok := r[reference]
	if !ok {
		return "", errors.New("missing fixture secret")
	}
	return value, nil
}

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

func testConfig(server *httptest.Server, withSecret, withToken bool) socialhub.AdapterConfig {
	account := socialhub.AccountConfig{ID: "media", ClientID: "client-id"}
	if withSecret {
		account.SecretRef = "test://client-secret"
	}
	if withToken {
		account.AccessTokenRef = "test://access-token"
	}
	return socialhub.AdapterConfig{
		Adapter: adapterName,
		Settings: map[string]any{
			"api_url": server.URL + "/api", "cdn_url": server.URL + "/cdn",
			"auth_url": server.URL + "/oauth/authorize", "token_url": server.URL + "/oauth/token",
			"app_name": "social-hub-tests", "app_version": "1.2.3", "user_agent": "social-hub-simkl-tests/1.0",
		},
		Accounts: []socialhub.AccountConfig{account},
	}
}

func newTestClient(t *testing.T, server *httptest.Server, withSecret, withToken bool) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server, withSecret, withToken),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{
			"test://client-secret": testClientSecret, "test://access-token": testAccessToken,
		}),
		socialhub.WithClock(fixedClock{now: testNow}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "media")
	if err != nil {
		t.Fatal(err)
	}
	client, ok := common.(*Client)
	if !ok {
		t.Fatalf("unexpected client type %T", common)
	}
	return adapter, client
}

func TestAdapterLifecycleAndCapabilities(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newTestClient(t, server, true, true)
	if adapter.Name() != adapterName || adapter.Metadata().APIVersion != apiVersion || adapter.Metadata().DocURL != documentationURL {
		t.Fatal("unexpected adapter metadata")
	}
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatal("adapter was not registered")
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil || capabilities[CapabilitySync].Approval != socialhub.ApprovalGranted ||
		!capabilities[CapabilityTrending].Supported || capabilities[socialhub.CapFetch].Supported {
		t.Fatalf("unexpected capabilities: %#v, %v", capabilities, err)
	}
	if client.Platform() != "simkl" || client.Account() != "media" {
		t.Fatal("unexpected client identity")
	}
	if _, ok := client.Publisher(); ok {
		t.Fatal("publisher unexpectedly exposed")
	}
	if _, ok := client.Fetcher(); ok {
		t.Fatal("fetcher unexpectedly exposed")
	}
	if _, ok := client.MediaUploader(); ok {
		t.Fatal("media uploader unexpectedly exposed")
	}
	if _, ok := client.Reactor(); ok {
		t.Fatal("reactor unexpectedly exposed")
	}
	if _, ok := client.Messenger(); ok {
		t.Fatal("messenger unexpectedly exposed")
	}
	if _, ok := client.WebhookHandler(); ok {
		t.Fatal("webhook unexpectedly exposed")
	}
	if client.OAuthWorkflow() == nil || client.CatalogWorkflow() == nil || client.TrendingWorkflow() == nil ||
		client.UserWorkflow() == nil || client.SyncWorkflow() == nil || client.ScrobbleWorkflow() == nil {
		t.Fatal("typed workflows are missing")
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "media"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("expected closed adapter error, got %v", err)
	}
	if err := adapter.Init(context.Background(), testConfig(server, true, true)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("expected closed init error, got %v", err)
	}
}

func TestPublicTrendingOnlyClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/cdn/discover/trending/movies/today_100.json" || request.URL.Query().Get("client_id") != "" ||
			request.URL.Query().Get("app-name") != "social-hub-tests" {
			t.Errorf("unexpected public CDN request: %s", request.URL.String())
		}
		_, _ = writer.Write([]byte(`[{"title":"Dune","rank":1,"url":"/movies/1/dune","ids":{"simkl_id":1}}]`))
	}))
	defer server.Close()
	config := testConfig(server, false, false)
	config.Accounts[0].ClientID = ""
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), config, socialhub.WithHTTPClient(server.Client())); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "media")
	if err != nil {
		t.Fatal(err)
	}
	client := common.(*Client)
	capabilities, _ := client.Capabilities(context.Background())
	if capabilities[CapabilityCatalog].Approval != socialhub.ApprovalRequired || capabilities[CapabilityTrending].Approval != socialhub.ApprovalGranted {
		t.Fatalf("unexpected public capabilities: %#v", capabilities)
	}
	items, err := client.ListTrending(context.Background(), TrendingRequest{Type: MediaMovie, Period: TrendingToday, Limit: 100})
	if err != nil || len(items) != 1 || items[0].IDs.Simkl != 1 {
		t.Fatalf("unexpected trending result: %#v, %v", items, err)
	}
	if _, err := client.Search(context.Background(), SearchRequest{Type: MediaMovie, Query: "Dune"}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("expected client ID approval error, got %v", err)
	}
}

func TestConfigurationAndSecretValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	tests := []struct {
		name   string
		mutate func(*socialhub.AdapterConfig)
	}{
		{"adapter", func(c *socialhub.AdapterConfig) { c.Adapter = "simkl" }},
		{"api URL", func(c *socialhub.AdapterConfig) { c.Settings["api_url"] = "javascript:bad" }},
		{"app name", func(c *socialhub.AdapterConfig) { c.Settings["app_name"] = " bad " }},
		{"scope", func(c *socialhub.AdapterConfig) { c.Accounts[0].Approval.Scopes = []string{"public"} }},
		{"account settings", func(c *socialhub.AdapterConfig) { c.Accounts[0].Settings = map[string]any{"x": true} }},
		{"credential without client", func(c *socialhub.AdapterConfig) { c.Accounts[0].ClientID = "" }},
		{"unknown global", func(c *socialhub.AdapterConfig) { c.Settings["unknown"] = true }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := testConfig(server, true, true)
			test.mutate(&config)
			if err := (&Adapter{}).Init(context.Background(), config); !errors.Is(err, socialhub.ErrInvalidArgument) {
				t.Fatalf("expected invalid argument, got %v", err)
			}
		})
	}
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server, true, true),
		socialhub.WithHTTPClient(server.Client()), socialhub.WithSecretResolver(mapResolver{})); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "media"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("expected secret resolution error, got %v", err)
	}
	adapter, _ = newTestClient(t, server, true, true)
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("expected missing account error, got %v", err)
	}
}

func TestRedirectsAreRejected(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("redirect target must not be reached")
	}))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, target.URL, http.StatusFound)
	}))
	defer server.Close()
	_, client := newTestClient(t, server, true, true)
	if _, err := client.GetMovie(context.Background(), 1); err == nil {
		t.Fatal("expected API redirect failure")
	}
	if _, err := client.Exchange(context.Background(), "code", "https://app.example/callback"); err == nil {
		t.Fatal("expected OAuth redirect failure")
	}
}

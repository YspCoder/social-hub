package giphy

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

var testNow = time.Date(2026, time.August, 2, 8, 0, 0, 0, time.UTC)

type fixedClock struct{ now time.Time }

func (clock fixedClock) Now() time.Time { return clock.now }

func testConfig(baseURL string) socialhub.AdapterConfig {
	return socialhub.AdapterConfig{
		Adapter: adapterName,
		Settings: map[string]any{
			"base_url": baseURL + "/v1", "upload_url": baseURL + "/upload/v1", "analytics_origin": baseURL,
		},
		Accounts: []socialhub.AccountConfig{{ID: "main", ClientID: "api-key"}},
	}
}

func newTestClient(t *testing.T, server *httptest.Server) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(
		context.Background(), testConfig(server.URL), socialhub.WithHTTPClient(server.Client()), socialhub.WithClock(fixedClock{testNow}),
	); err != nil {
		t.Fatalf("Init: %v", err)
	}
	common, err := adapter.Client(context.Background(), "main")
	if err != nil {
		t.Fatalf("Client: %v", err)
	}
	client, ok := common.(*Client)
	if !ok {
		t.Fatalf("client type %T", common)
	}
	return adapter, client
}

func TestAdapterLifecycleCapabilitiesAndRegistration(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newTestClient(t, server)
	metadata := adapter.Metadata()
	if adapter.Name() != adapterName || metadata.Product != productName || metadata.APIVersion != apiVersion || metadata.DocURL != documentationURL || metadata.VerifiedAt.IsZero() {
		t.Fatalf("metadata=%#v", metadata)
	}
	if !containsString(socialhub.Adapters(), adapterName) {
		t.Fatalf("adapter %q is not registered", adapterName)
	}
	if client.Platform() != "giphy" || client.Account() != "main" || client.DiscoveryWorkflow() == nil || client.UploadWorkflow() == nil || client.AnalyticsWorkflow() == nil {
		t.Fatal("client identity or typed workflows are missing")
	}
	if _, ok := client.Publisher(); ok {
		t.Fatal("publisher must be unavailable")
	}
	if _, ok := client.Fetcher(); ok {
		t.Fatal("fetcher must be unavailable")
	}
	if _, ok := client.MediaUploader(); ok {
		t.Fatal("common media uploader must be unavailable")
	}
	if _, ok := client.Reactor(); ok {
		t.Fatal("reactor must be unavailable")
	}
	if _, ok := client.Messenger(); ok {
		t.Fatal("messenger must be unavailable")
	}
	if _, ok := client.WebhookHandler(); ok {
		t.Fatal("webhook handler must be unavailable")
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil || !capabilities.Has(CapabilityDiscovery) || !capabilities.Has(CapabilityUpload) || !capabilities.Has(CapabilityAnalytics) || capabilities[socialhub.CapFetch].Supported {
		t.Fatalf("capabilities=%#v err=%v", capabilities, err)
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "main"); errorCode(err) != socialhub.CodeConflict {
		t.Fatalf("client after close=%v", err)
	}
	if err := adapter.Init(context.Background(), testConfig(server.URL)); errorCode(err) != socialhub.CodeConflict {
		t.Fatalf("init after close=%v", err)
	}
}

func TestAdapterValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	tests := []struct {
		name   string
		mutate func(*socialhub.AdapterConfig)
	}{
		{"missing adapter", func(config *socialhub.AdapterConfig) { config.Adapter = "" }},
		{"wrong adapter", func(config *socialhub.AdapterConfig) { config.Adapter = "wrong" }},
		{"api key", func(config *socialhub.AdapterConfig) { config.Accounts[0].ClientID = "" }},
		{"base URL", func(config *socialhub.AdapterConfig) { config.Settings["base_url"] = "://bad" }},
		{"upload query", func(config *socialhub.AdapterConfig) {
			config.Settings["upload_url"] = server.URL + "/upload?key=value"
		}},
		{"analytics path", func(config *socialhub.AdapterConfig) { config.Settings["analytics_origin"] = server.URL + "/analytics" }},
		{"unknown setting", func(config *socialhub.AdapterConfig) { config.Settings["unknown"] = true }},
		{"account setting", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings = map[string]any{"unknown": true} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := testConfig(server.URL)
			test.mutate(&config)
			if err := (&Adapter{}).Init(context.Background(), config); errorCode(err) != socialhub.CodeInvalidArgument {
				t.Fatalf("error=%v", err)
			}
		})
	}
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server.URL), socialhub.WithHTTPClient(server.Client())); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); errorCode(err) != socialhub.CodeNotFound {
		t.Fatalf("missing account=%v", err)
	}
}

func errorCode(err error) socialhub.ErrorCode {
	var platformErr *socialhub.Error
	if errors.As(err, &platformErr) {
		return platformErr.Code
	}
	return ""
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func writeJSON(writer http.ResponseWriter, status int, body string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_, _ = writer.Write([]byte(body))
}

var _ socialhub.Clock = fixedClock{}

package unitystatistics

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestAdapterRegistrationMetadataCapabilitiesAndAuthentication(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()
	adapter, err := socialhub.Open(context.Background(), adapterName, basicConfig(server.URL), testOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	if adapter.Name() != adapterName {
		t.Fatalf("adapter=%v", adapter)
	}
	metadata := adapter.Metadata()
	if metadata.Name != adapterName || metadata.Product != productName || metadata.APIVersion != apiVersion || metadata.DocURL != documentationURL || metadata.VerifiedAt.IsZero() || contractVersion != "v2.0 latest" {
		t.Fatalf("metadata=%#v", metadata)
	}
	common, err := adapter.Client(context.Background(), testAccountID)
	if err != nil {
		t.Fatal(err)
	}
	client := common.(*Client)
	request, err := client.api.NewRequest(context.Background(), http.MethodGet, client.reportPath("acquisitions"), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	wantBasic := "Basic " + base64.StdEncoding.EncodeToString([]byte(testKeyID+":"+testSecretKey))
	if request.Header.Get("Authorization") != wantBasic || client.Platform() != platformName || client.Account() != testAccountID || client.Reports() == nil {
		t.Fatalf("authorization=%q client=%#v", request.Header.Get("Authorization"), client)
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil || !capabilities[CapabilityAdvertisingStatistics].Supported || capabilities[CapabilityAdvertisingStatistics].Approval != socialhub.ApprovalRequired ||
		capabilities[socialhub.CapFetch].Supported || capabilities[socialhub.CapPublish].Supported || capabilities[socialhub.CapMedia].Supported ||
		capabilities[socialhub.CapReact].Supported || capabilities[socialhub.CapMessage].Supported || capabilities[socialhub.CapWebhook].Supported {
		t.Fatalf("capabilities=%#v err=%v", capabilities, err)
	}
	if _, ok := client.Publisher(); ok {
		t.Fatal("publisher unexpectedly supported")
	}
	if _, ok := client.Fetcher(); ok {
		t.Fatal("fetcher unexpectedly supported")
	}
	if _, ok := client.MediaUploader(); ok {
		t.Fatal("media unexpectedly supported")
	}
	if _, ok := client.Reactor(); ok {
		t.Fatal("reactor unexpectedly supported")
	}
	if _, ok := client.Messenger(); ok {
		t.Fatal("messenger unexpectedly supported")
	}
	if _, ok := client.WebhookHandler(); ok {
		t.Fatal("webhook unexpectedly supported")
	}
	if client.Close() != nil || adapter.Close() != nil {
		t.Fatal("close failed")
	}
	if _, err := adapter.Client(context.Background(), testAccountID); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("closed client error=%v", err)
	}
	if err := adapter.Init(context.Background(), basicConfig(server.URL)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("re-init error=%v", err)
	}
}

func TestBearerAuthenticationAndClientOverrides(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server.URL), testOptions()...); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), testAccountID, socialhub.WithSecretResolver(mapResolver{"secret://unity-stats-bearer": "override-bearer"}))
	if err != nil {
		t.Fatal(err)
	}
	client := common.(*Client)
	request, err := client.api.NewRequest(context.Background(), http.MethodGet, client.reportPath("skan"), nil, nil)
	if err != nil || request.Header.Get("Authorization") != "Bearer override-bearer" {
		t.Fatalf("authorization=%q err=%v", request.Header.Get("Authorization"), err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing account error=%v", err)
	}
}

func TestAdapterConfigurationValidation(t *testing.T) {
	base := testConfig("https://example.test")
	tests := []struct {
		name string
		edit func(*socialhub.AdapterConfig)
	}{
		{"missing account", func(config *socialhub.AdapterConfig) { config.Accounts = nil }},
		{"adapter mismatch", func(config *socialhub.AdapterConfig) { config.Adapter = "other" }},
		{"product mismatch", func(config *socialhub.AdapterConfig) { config.Product = "other" }},
		{"unknown setting", func(config *socialhub.AdapterConfig) { config.Settings["unknown"] = true }},
		{"bad base URL", func(config *socialhub.AdapterConfig) { config.Settings["base_url"] = "https://user:pass@example.test" }},
		{"base query", func(config *socialhub.AdapterConfig) { config.Settings["base_url"] = "https://example.test?x=1" }},
		{"base trailing slash", func(config *socialhub.AdapterConfig) { config.Settings["base_url"] = "https://example.test/" }},
		{"bad organization", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["organization_id"] = "0" }},
		{"overflow organization", func(config *socialhub.AdapterConfig) {
			config.Accounts[0].Settings["organization_id"] = "9223372036854775808"
		}},
		{"unknown account setting", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["unknown"] = true }},
		{"missing credentials", func(config *socialhub.AdapterConfig) { config.Accounts[0].AccessTokenRef = "" }},
		{"partial basic", func(config *socialhub.AdapterConfig) {
			config.Accounts[0].AccessTokenRef, config.Accounts[0].ClientID = "", testKeyID
		}},
		{"bad basic key ID", func(config *socialhub.AdapterConfig) {
			config.Accounts[0].AccessTokenRef, config.Accounts[0].ClientID, config.Accounts[0].SecretRef = "", "bad:key", "secret://key"
		}},
		{"both credentials", func(config *socialhub.AdapterConfig) {
			config.Accounts[0].ClientID, config.Accounts[0].SecretRef = testKeyID, "secret://key"
		}},
		{"bearer with secret", func(config *socialhub.AdapterConfig) { config.Accounts[0].SecretRef = "secret://key" }},
		{"webhook", func(config *socialhub.AdapterConfig) { config.Accounts[0].Webhook.SecretRef = "secret://webhook" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := base
			config.Settings = cloneMap(base.Settings)
			config.Accounts = append([]socialhub.AccountConfig(nil), base.Accounts...)
			config.Accounts[0].Settings = cloneMap(base.Accounts[0].Settings)
			test.edit(&config)
			adapter := &Adapter{}
			if err := adapter.Init(context.Background(), config, testOptions()...); !errors.Is(err, socialhub.ErrInvalidArgument) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestClientSecretResolutionFailures(t *testing.T) {
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig("https://example.test"), socialhub.WithSecretResolver(mapResolver{})); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), testAccountID); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("bearer resolution error=%v", err)
	}
	basic := &Adapter{}
	if err := basic.Init(context.Background(), basicConfig("https://example.test"), socialhub.WithSecretResolver(mapResolver{})); err != nil {
		t.Fatal(err)
	}
	if _, err := basic.Client(context.Background(), testAccountID); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("basic resolution error=%v", err)
	}
}

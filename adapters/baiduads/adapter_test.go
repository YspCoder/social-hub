package baiduads

import (
	"context"
	"errors"
	"net/http/httptest"
	"slices"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestAdapterLifecycleAndCapabilities(t *testing.T) {
	server := httptest.NewServer(nil)
	defer server.Close()
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatal("Baidu Ads adapter is not registered")
	}
	adapter, err := socialhub.Open(context.Background(), adapterName, testConfig(server.URL),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{"test://access": testAccessToken, "test://secret": testSecretKey}),
		socialhub.WithClock(fixedClock{value: testNow}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if adapter.Name() != adapterName {
		t.Fatalf("name=%q", adapter.Name())
	}
	metadata := adapter.Metadata()
	if metadata.Product != productName || metadata.APIVersion != apiVersion || metadata.DocURL != documentationURL || metadata.VerifiedAt.IsZero() {
		t.Fatalf("metadata=%+v", metadata)
	}
	common, err := adapter.Client(context.Background(), testAccountID)
	if err != nil {
		t.Fatal(err)
	}
	client := common.(*Client)
	if client.Platform() != platformName || client.Account() != testAccountID {
		t.Fatalf("client platform=%q account=%q", client.Platform(), client.Account())
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil || !capabilities[CapabilitySearchAdsManagement].Supported || !capabilities[CapabilityMarketingReports].Supported ||
		capabilities[socialhub.CapPublish].Supported {
		t.Fatalf("capabilities=%+v err=%v", capabilities, err)
	}
	if publisher, ok := client.Publisher(); ok || publisher != nil {
		t.Fatal("publisher unexpectedly supported")
	}
	if fetcher, ok := client.Fetcher(); ok || fetcher != nil {
		t.Fatal("fetcher unexpectedly supported")
	}
	if media, ok := client.MediaUploader(); ok || media != nil {
		t.Fatal("media unexpectedly supported")
	}
	if reactor, ok := client.Reactor(); ok || reactor != nil {
		t.Fatal("reactor unexpectedly supported")
	}
	if messenger, ok := client.Messenger(); ok || messenger != nil {
		t.Fatal("messenger unexpectedly supported")
	}
	if webhook, ok := client.WebhookHandler(); ok || webhook != nil {
		t.Fatal("webhook unexpectedly supported")
	}
	if client.Accounts() != client || client.Campaigns() != client || client.AdGroups() != client || client.Creatives() != client || client.Reports() != client {
		t.Fatal("typed workflow accessor returned another client")
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	typedAdapter := adapter.(*Adapter)
	oauth, err := typedAdapter.OAuth(context.Background(), testAccountID)
	if err != nil || oauth.AppID != "baidu-app-id" || oauth.SecretKey != testSecretKey || oauth.BaseURL != server.URL {
		t.Fatalf("oauth=%+v err=%v", oauth, err)
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), testAccountID); requireHubError(t, err).Code != socialhub.CodeConflict {
		t.Fatalf("client after close err=%v", err)
	}
	if _, err := typedAdapter.OAuth(context.Background(), testAccountID); requireHubError(t, err).Code != socialhub.CodeConflict {
		t.Fatalf("oauth after close err=%v", err)
	}
}

func TestAdapterValidationFailures(t *testing.T) {
	server := httptest.NewServer(nil)
	defer server.Close()
	valid := testConfig(server.URL)
	tests := []struct {
		name   string
		mutate func(*socialhub.AdapterConfig)
	}{
		{"shared config", func(config *socialhub.AdapterConfig) { config.Accounts = nil }},
		{"adapter mismatch", func(config *socialhub.AdapterConfig) { config.Adapter = "other" }},
		{"product mismatch", func(config *socialhub.AdapterConfig) { config.Product = "other" }},
		{"invalid base URL", func(config *socialhub.AdapterConfig) { config.Settings["base_url"] = "ftp://example.com" }},
		{"unknown setting", func(config *socialhub.AdapterConfig) { config.Settings["unknown"] = true }},
		{"missing token ref", func(config *socialhub.AdapterConfig) { config.Accounts[0].AccessTokenRef = "" }},
		{"missing username", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["user_name"] = "" }},
		{"unknown account setting", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["unknown"] = true }},
		{"webhook", func(config *socialhub.AdapterConfig) { config.Accounts[0].Webhook.SecretRef = "test://webhook" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := cloneConfig(valid)
			test.mutate(&config)
			err := (&Adapter{}).Init(context.Background(), config,
				socialhub.WithHTTPClient(server.Client()), socialhub.WithSecretResolver(mapResolver{}))
			if requireHubError(t, err).Code != socialhub.CodeInvalidArgument {
				t.Fatalf("err=%v", err)
			}
		})
	}
	adapter := &Adapter{}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := adapter.Init(context.Background(), valid, socialhub.WithHTTPClient(server.Client()), socialhub.WithSecretResolver(mapResolver{})); requireHubError(t, err).Code != socialhub.CodeConflict {
		t.Fatalf("reinit err=%v", err)
	}
	optionError := errors.New("option failure")
	err := (&Adapter{}).Init(context.Background(), valid, func(*socialhub.Options) error { return optionError })
	if !errors.Is(err, optionError) {
		t.Fatalf("option err=%v", err)
	}
}

func TestAdapterClientAndOAuthFailures(t *testing.T) {
	server := httptest.NewServer(nil)
	defer server.Close()
	if _, err := (&Adapter{}).Client(context.Background(), testAccountID); requireHubError(t, err).Code != socialhub.CodeConflict {
		t.Fatalf("uninitialized client err=%v", err)
	}
	if _, err := (&Adapter{}).OAuth(context.Background(), testAccountID); requireHubError(t, err).Code != socialhub.CodeConflict {
		t.Fatalf("uninitialized oauth err=%v", err)
	}
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server.URL),
		socialhub.WithHTTPClient(server.Client()), socialhub.WithSecretResolver(mapResolver{})); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); requireHubError(t, err).Code != socialhub.CodeNotFound {
		t.Fatalf("missing account err=%v", err)
	}
	if _, err := adapter.Client(context.Background(), testAccountID); requireHubError(t, err).Code != socialhub.CodeUnauthenticated {
		t.Fatalf("missing access token err=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "missing"); requireHubError(t, err).Code != socialhub.CodeNotFound {
		t.Fatalf("missing OAuth account err=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), testAccountID); requireHubError(t, err).Code != socialhub.CodeUnauthenticated {
		t.Fatalf("missing OAuth secret err=%v", err)
	}

	config := testConfig(server.URL)
	config.Accounts[0].AppID = ""
	config.Accounts[0].SecretRef = ""
	incomplete := &Adapter{}
	if err := incomplete.Init(context.Background(), config,
		socialhub.WithHTTPClient(server.Client()), socialhub.WithSecretResolver(mapResolver{"test://access": testAccessToken})); err != nil {
		t.Fatal(err)
	}
	if _, err := incomplete.OAuth(context.Background(), testAccountID); requireHubError(t, err).Code != socialhub.CodeInvalidArgument {
		t.Fatalf("incomplete OAuth err=%v", err)
	}
}

func cloneConfig(input socialhub.AdapterConfig) socialhub.AdapterConfig {
	output := input
	output.Settings = make(map[string]any, len(input.Settings))
	for key, value := range input.Settings {
		output.Settings[key] = value
	}
	output.Accounts = append([]socialhub.AccountConfig(nil), input.Accounts...)
	for index := range output.Accounts {
		settings := make(map[string]any, len(input.Accounts[index].Settings))
		for key, value := range input.Accounts[index].Settings {
			settings[key] = value
		}
		output.Accounts[index].Settings = settings
	}
	return output
}

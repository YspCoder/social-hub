package zalo

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

type testResolver map[string]string

func (r testResolver) Resolve(_ context.Context, reference string) (string, error) {
	value, ok := r[reference]
	if !ok {
		return "", errors.New("missing test secret")
	}
	return value, nil
}

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

var testNow = time.Date(2026, time.August, 2, 12, 0, 0, 0, time.UTC)

func testConfig(serverURL string, withOAuth, withWebhook bool) socialhub.AdapterConfig {
	account := socialhub.AccountConfig{
		ID: "main", AppID: "360846524940903967", AccessTokenRef: "test://access-token",
		Settings: map[string]any{"oa_id": "388613280878808645"},
	}
	if withOAuth {
		account.SecretRef = "test://app-secret"
	}
	if withWebhook {
		account.Webhook.SecretRef = "test://oa-secret"
	}
	return socialhub.AdapterConfig{
		Adapter:  adapterName,
		Settings: map[string]any{"base_url": serverURL, "oauth_base_url": serverURL},
		Accounts: []socialhub.AccountConfig{account},
	}
}

func newTestClient(t *testing.T, server *httptest.Server, withOAuth, withWebhook bool) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server.URL, withOAuth, withWebhook),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(testResolver{
			"test://access-token": "oa-access-token", "test://app-secret": "app-secret", "test://oa-secret": "oa-secret",
		}),
		socialhub.WithClock(fixedClock{now: testNow}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	return adapter, common.(*Client)
}

func writeTestJSON(t *testing.T, writer http.ResponseWriter, value any) {
	t.Helper()
	writer.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(writer).Encode(value); err != nil {
		t.Errorf("encode response: %v", err)
	}
}

func errorCode(err error) socialhub.ErrorCode {
	var platformErr *socialhub.Error
	if errors.As(err, &platformErr) {
		return platformErr.Code
	}
	return ""
}

func TestAdapterRegistrationCapabilitiesAndLifecycle(t *testing.T) {
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatalf("registered adapters=%v", socialhub.Adapters())
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newTestClient(t, server, true, true)
	if adapter.Name() != adapterName || client.Platform() != "zalo" || client.Account() != "main" {
		t.Fatalf("identity=%s %s/%s", adapter.Name(), client.Platform(), client.Account())
	}
	metadata := adapter.Metadata()
	if metadata.Product != productName || metadata.APIVersion != apiVersion || metadata.DocURL != docURL || metadata.VerifiedAt.IsZero() {
		t.Fatalf("metadata=%#v", metadata)
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []socialhub.Capability{
		socialhub.CapFetch, socialhub.CapMessage, socialhub.CapWebhook,
		CapabilityConsultationMessages, CapabilityOAProfile, CapabilityUserProfiles,
	} {
		if !capabilities.Has(name) || capabilities[name].DocURL == "" {
			t.Fatalf("capability %s=%#v", name, capabilities[name])
		}
	}
	for _, name := range []socialhub.Capability{socialhub.CapPublish, socialhub.CapMedia, socialhub.CapReact} {
		if capabilities.Has(name) {
			t.Fatalf("unsupported capability %s=%#v", name, capabilities[name])
		}
	}
	if _, ok := client.Fetcher(); !ok {
		t.Fatal("fetcher should be available")
	}
	if _, ok := client.Messenger(); !ok {
		t.Fatal("messenger should be available")
	}
	if _, ok := client.WebhookHandler(); !ok {
		t.Fatal("webhook should be available")
	}
	if _, ok := client.Publisher(); ok {
		t.Fatal("publisher should be unavailable")
	}
	if _, ok := client.MediaUploader(); ok {
		t.Fatal("media uploader should be unavailable")
	}
	if _, ok := client.Reactor(); ok {
		t.Fatal("reactor should be unavailable")
	}
	if client.MessageWorkflow() == nil || client.ProfileWorkflow() == nil || client.Close() != nil {
		t.Fatal("typed workflows or close unavailable")
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "main"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close=%v", err)
	}
	if err := adapter.Init(context.Background(), testConfig(server.URL, false, false)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("init after close=%v", err)
	}
}

func TestAdapterValidationAndOptionalWebhook(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, false, false)
	if _, ok := client.WebhookHandler(); ok {
		t.Fatal("webhook should require an OA secret")
	}
	capabilities, _ := client.Capabilities(context.Background())
	if capabilities.Has(socialhub.CapWebhook) {
		t.Fatalf("webhook capability=%#v", capabilities[socialhub.CapWebhook])
	}

	tests := []struct {
		name   string
		mutate func(*socialhub.AdapterConfig)
	}{
		{name: "adapter mismatch", mutate: func(c *socialhub.AdapterConfig) { c.Adapter = "wrong" }},
		{name: "missing token", mutate: func(c *socialhub.AdapterConfig) { c.Accounts[0].AccessTokenRef = "" }},
		{name: "bad app id", mutate: func(c *socialhub.AdapterConfig) { c.Accounts[0].AppID = "not-numeric" }},
		{name: "secret without app", mutate: func(c *socialhub.AdapterConfig) { c.Accounts[0].AppID = ""; c.Accounts[0].SecretRef = "secret" }},
		{name: "webhook without app", mutate: func(c *socialhub.AdapterConfig) { c.Accounts[0].AppID = ""; c.Accounts[0].Webhook.SecretRef = "secret" }},
		{name: "bad oa id", mutate: func(c *socialhub.AdapterConfig) { c.Accounts[0].Settings["oa_id"] = "bad" }},
		{name: "bad endpoint", mutate: func(c *socialhub.AdapterConfig) { c.Settings["base_url"] = "ftp://example.test" }},
		{name: "unknown setting", mutate: func(c *socialhub.AdapterConfig) { c.Settings["unknown"] = true }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := testConfig(server.URL, false, false)
			test.mutate(&config)
			adapter := &Adapter{}
			if err := adapter.Init(context.Background(), config); !errors.Is(err, socialhub.ErrInvalidArgument) {
				t.Fatalf("error=%v", err)
			}
		})
	}

	adapter := &Adapter{}
	if _, err := adapter.Client(context.Background(), "main"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client before init=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "main"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("oauth before init=%v", err)
	}
	initialized, _ := newTestClient(t, server, false, false)
	if _, err := initialized.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing account=%v", err)
	}
	if _, err := initialized.OAuth(context.Background(), "main"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("incomplete oauth=%v", err)
	}
}

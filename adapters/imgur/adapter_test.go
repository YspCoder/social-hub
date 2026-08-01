package imgur

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

type testSecrets map[string]string

func (secrets testSecrets) Resolve(_ context.Context, reference string) (string, error) {
	value, ok := secrets[reference]
	if !ok {
		return "", errors.New("missing test secret")
	}
	return value, nil
}

func testConfig(baseURL string, bearer bool) socialhub.AdapterConfig {
	account := socialhub.AccountConfig{
		ID: "main", ClientID: "client-id", SecretRef: "secret://client",
		Settings: map[string]any{"username": "alice"},
	}
	if bearer {
		account.AccessTokenRef = "secret://access"
	}
	return socialhub.AdapterConfig{
		Adapter: adapterName,
		Settings: map[string]any{
			"base_url": baseURL + "/3", "auth_url": baseURL + "/oauth2/authorize", "token_url": baseURL + "/oauth2/token",
		},
		Accounts: []socialhub.AccountConfig{account},
	}
}

func newTestClient(t *testing.T, server *httptest.Server, bearer bool) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	err := adapter.Init(
		context.Background(), testConfig(server.URL, bearer), socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(testSecrets{"secret://client": "client-secret", "secret://access": "access-token"}),
		socialhub.WithClock(fixedClock{testNow}),
	)
	if err != nil {
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
	adapter, client := newTestClient(t, server, true)
	metadata := adapter.Metadata()
	if adapter.Name() != adapterName || metadata.Product != productName || metadata.APIVersion != apiVersion || metadata.DocURL != documentationURL || metadata.VerifiedAt.IsZero() {
		t.Fatalf("metadata=%#v", metadata)
	}
	if !containsString(socialhub.Adapters(), adapterName) {
		t.Fatalf("adapter %q is not registered", adapterName)
	}
	if client.Platform() != "imgur" || client.Account() != "main" || client.ImageWorkflow() == nil || client.AlbumWorkflow() == nil || client.GalleryWorkflow() == nil || client.CreditWorkflow() == nil {
		t.Fatal("client identity or typed workflows are missing")
	}
	if _, ok := client.Publisher(); !ok {
		t.Fatal("publisher is unavailable with Bearer auth")
	}
	if _, ok := client.Fetcher(); !ok {
		t.Fatal("fetcher is unavailable")
	}
	if _, ok := client.Reactor(); !ok {
		t.Fatal("reactor is unavailable with Bearer auth")
	}
	if _, ok := client.MediaUploader(); ok {
		t.Fatal("common media uploader must be unavailable")
	}
	if _, ok := client.Messenger(); ok {
		t.Fatal("messenger must be unavailable")
	}
	if _, ok := client.WebhookHandler(); ok {
		t.Fatal("webhook handler must be unavailable")
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil || !capabilities.Has(socialhub.CapPublish) || !capabilities.Has(socialhub.CapFetch) || !capabilities.Has(CapabilityImages) || !capabilities.Has(CapabilityAlbums) || !capabilities.Has(CapabilityGallery) || !capabilities.Has(CapabilityCredits) || capabilities[socialhub.CapMessage].Supported {
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
	if _, err := adapter.OAuth(context.Background(), "main"); errorCode(err) != socialhub.CodeConflict {
		t.Fatalf("OAuth after close=%v", err)
	}
	if err := adapter.Init(context.Background(), testConfig(server.URL, true)); errorCode(err) != socialhub.CodeConflict {
		t.Fatalf("init after close=%v", err)
	}
}

func TestPublicClientCapabilitiesAndValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, false)
	if _, ok := client.Publisher(); ok {
		t.Fatal("public client must not publish")
	}
	if _, ok := client.Reactor(); ok {
		t.Fatal("public client must not react")
	}
	capabilities, _ := client.Capabilities(context.Background())
	if capabilities[socialhub.CapPublish].Supported || capabilities[CapabilityGallery].Supported || !capabilities[CapabilityImages].Supported {
		t.Fatalf("capabilities=%#v", capabilities)
	}
	if _, err := client.requireUser("test"); errorCode(err) != socialhub.CodeUnauthenticated {
		t.Fatalf("require user=%v", err)
	}

	tests := []struct {
		name   string
		mutate func(*socialhub.AdapterConfig)
	}{
		{"missing adapter", func(config *socialhub.AdapterConfig) { config.Adapter = "" }},
		{"wrong adapter", func(config *socialhub.AdapterConfig) { config.Adapter = "wrong" }},
		{"client id", func(config *socialhub.AdapterConfig) { config.Accounts[0].ClientID = "" }},
		{"endpoint", func(config *socialhub.AdapterConfig) { config.Settings["base_url"] = "://bad" }},
		{"endpoint credentials", func(config *socialhub.AdapterConfig) { config.Settings["token_url"] = "https://user@example.com/token" }},
		{"username", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["username"] = "bad/name" }},
		{"unknown setting", func(config *socialhub.AdapterConfig) { config.Settings["unknown"] = true }},
		{"unknown account setting", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["unknown"] = true }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := testConfig(server.URL, false)
			test.mutate(&config)
			if err := (&Adapter{}).Init(context.Background(), config); errorCode(err) != socialhub.CodeInvalidArgument {
				t.Fatalf("error=%v", err)
			}
		})
	}

	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server.URL, false), socialhub.WithHTTPClient(server.Client())); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); errorCode(err) != socialhub.CodeNotFound {
		t.Fatalf("missing account=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "missing"); errorCode(err) != socialhub.CodeNotFound {
		t.Fatalf("missing OAuth account=%v", err)
	}
	config := testConfig(server.URL, false)
	config.Accounts[0].SecretRef = ""
	withoutSecret := &Adapter{}
	if err := withoutSecret.Init(context.Background(), config); err != nil {
		t.Fatal(err)
	}
	if _, err := withoutSecret.OAuth(context.Background(), "main"); errorCode(err) != socialhub.CodeInvalidArgument {
		t.Fatalf("OAuth without secret=%v", err)
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
var _ socialhub.SecretResolver = testSecrets{}

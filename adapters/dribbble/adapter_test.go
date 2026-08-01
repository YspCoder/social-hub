package dribbble

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

type mapResolver map[string]string

func (resolver mapResolver) Resolve(_ context.Context, reference string) (string, error) {
	value, exists := resolver[reference]
	if !exists {
		return "", errors.New("missing test secret")
	}
	return value, nil
}

type fixedClock struct{ now time.Time }

func (clock fixedClock) Now() time.Time { return clock.now }

func testConfig(server *httptest.Server, scopes []string) socialhub.AdapterConfig {
	return socialhub.AdapterConfig{
		Adapter: adapterName, Product: productName,
		Settings: map[string]any{"base_url": server.URL + "/v2", "auth_url": server.URL + "/oauth/authorize", "token_url": server.URL + "/oauth/token"},
		Accounts: []socialhub.AccountConfig{{
			ID: "designer", ClientID: "client-id", SecretRef: "test://client-secret", AccessTokenRef: "test://access-token",
			Settings: map[string]any{"user_id": "1"}, Approval: socialhub.ApprovalConfig{Scopes: scopes},
		}},
	}
}

func newTestClient(t *testing.T, server *httptest.Server, scopes []string) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server, scopes),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{"test://client-secret": "client-secret", "test://access-token": "access-token"}),
		socialhub.WithClock(fixedClock{now: time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "designer")
	if err != nil {
		t.Fatal(err)
	}
	return adapter, common.(*Client)
}

func TestAdapterRegistrationCapabilitiesAndLifecycle(t *testing.T) {
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatalf("registered adapters=%v", socialhub.Adapters())
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newTestClient(t, server, []string{"public", "upload"})
	metadata := adapter.Metadata()
	if adapter.Name() != adapterName || metadata.Product != productName || metadata.APIVersion != apiVersion || metadata.DocURL != documentationURL {
		t.Fatalf("metadata=%#v", metadata)
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil || !capabilities.Has(socialhub.CapFetch) || !capabilities.Has(CapabilityShots) || !capabilities.Has(CapabilityProjects) || !capabilities.Has(CapabilityAttachments) {
		t.Fatalf("capabilities=%#v err=%v", capabilities, err)
	}
	for _, capability := range []socialhub.Capability{socialhub.CapPublish, socialhub.CapMedia, socialhub.CapReact, socialhub.CapMessage, socialhub.CapWebhook} {
		if capabilities.Has(capability) {
			t.Fatalf("unexpected capability %s", capability)
		}
	}
	if client.Platform() != "dribbble" || client.Account() != "designer" || client.ShotWorkflow() == nil || client.ProjectWorkflow() == nil || client.AttachmentWorkflow() == nil || client.Close() != nil {
		t.Fatalf("client=%#v", client)
	}
	if _, ok := client.Fetcher(); !ok {
		t.Fatal("Fetcher must be exposed")
	}
	if _, ok := client.Publisher(); ok {
		t.Fatal("Publisher must not be exposed")
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
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "designer"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close=%v", err)
	}
	if err := adapter.Init(context.Background(), testConfig(server, nil)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("reinit=%v", err)
	}
}

func TestAdapterValidationAndScopeGating(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	valid := testConfig(server, nil)
	tests := []struct {
		name   string
		mutate func(*socialhub.AdapterConfig)
	}{
		{"adapter", func(config *socialhub.AdapterConfig) { config.Adapter = "other" }},
		{"token", func(config *socialhub.AdapterConfig) { config.Accounts[0].AccessTokenRef = "" }},
		{"user", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["user_id"] = "bad" }},
		{"endpoint", func(config *socialhub.AdapterConfig) {
			config.Settings["base_url"] = "https://user:pass@example.com/v2"
		}},
		{"unknown setting", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["other"] = true }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := valid
			config.Settings = cloneMap(valid.Settings)
			config.Accounts = append([]socialhub.AccountConfig(nil), valid.Accounts...)
			config.Accounts[0].Settings = cloneMap(valid.Accounts[0].Settings)
			test.mutate(&config)
			if err := (&Adapter{}).Init(context.Background(), config); !errors.Is(err, socialhub.ErrInvalidArgument) {
				t.Fatalf("error=%v", err)
			}
		})
	}
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), valid, socialhub.WithHTTPClient(server.Client()), socialhub.WithSecretResolver(mapResolver{"test://access-token": "token"})); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing client=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing oauth=%v", err)
	}

	_, readOnly := newTestClient(t, server, []string{"public"})
	_, err := readOnly.CreateShot(context.Background(), CreateShotRequest{Filename: "shot.png", MIME: "image/png", Size: 1, Title: "Shot"}, strings.NewReader("x"))
	if !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("scope error=%v", err)
	}
}

func cloneMap(input map[string]any) map[string]any {
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func writeJSON(writer http.ResponseWriter, status int, body string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_, _ = writer.Write([]byte(body))
}

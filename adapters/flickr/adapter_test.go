package flickr

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

func testConfig(baseURL string) socialhub.AdapterConfig {
	return socialhub.AdapterConfig{
		Adapter: adapterName,
		Settings: map[string]any{
			"base_url": baseURL + "/rest", "upload_url": baseURL + "/upload",
			"request_token_url": baseURL + "/request_token", "authorize_url": baseURL + "/authorize",
			"access_token_url": baseURL + "/access_token",
		},
		Accounts: []socialhub.AccountConfig{{
			ID: "main", ClientID: "api-key", SecretRef: "secret://consumer",
			AccessTokenRef: "secret://access", Approval: socialhub.ApprovalConfig{Scopes: []string{PermissionDelete}},
			Settings: map[string]any{"user_id": "owner@N01", "token_secret_ref": "secret://token"},
		}},
	}
}

func newTestClient(t *testing.T, server *httptest.Server) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	err := adapter.Init(
		context.Background(), testConfig(server.URL), socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(testSecrets{
			"secret://consumer": "consumer-secret", "secret://access": "access-token", "secret://token": "token-secret",
		}), socialhub.WithClock(fixedClock{testNow}),
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
	adapter, client := newTestClient(t, server)
	metadata := adapter.Metadata()
	if adapter.Name() != adapterName || metadata.Product != productName || metadata.APIVersion != apiVersion || metadata.VerifiedAt.IsZero() {
		t.Fatalf("metadata=%#v", metadata)
	}
	if !containsString(socialhub.Adapters(), adapterName) {
		t.Fatalf("adapter %q is not registered", adapterName)
	}
	if client.Platform() != "flickr" || client.Account() != "main" || client.PhotoWorkflow() == nil || client.PhotoUploadWorkflow() == nil || client.AlbumWorkflow() == nil {
		t.Fatal("client identity or typed workflows are missing")
	}
	if _, ok := client.Fetcher(); !ok {
		t.Fatal("fetcher is unavailable")
	}
	if _, ok := client.Reactor(); !ok {
		t.Fatal("reactor is unavailable for delete permission")
	}
	if _, ok := client.Publisher(); ok {
		t.Fatal("publisher must be unavailable")
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
	if err != nil || !capabilities.Has(socialhub.CapFetch) || !capabilities.Has(CapabilityPhotoUpload) || capabilities[socialhub.CapWebhook].Supported {
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

func TestAdapterValidationAndOAuthFactory(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	tests := []struct {
		name   string
		mutate func(*socialhub.AdapterConfig)
	}{
		{"adapter", func(config *socialhub.AdapterConfig) { config.Adapter = "wrong" }},
		{"api key", func(config *socialhub.AdapterConfig) { config.Accounts[0].ClientID = "" }},
		{"endpoint", func(config *socialhub.AdapterConfig) { config.Settings["base_url"] = "://bad" }},
		{"user", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["user_id"] = "bad/id" }},
		{"consumer secret", func(config *socialhub.AdapterConfig) { config.Accounts[0].SecretRef = "" }},
		{"token secret", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["token_secret_ref"] = "" }},
		{"scope", func(config *socialhub.AdapterConfig) { config.Accounts[0].Approval.Scopes = []string{"admin"} }},
		{"settings", func(config *socialhub.AdapterConfig) { config.Settings["unknown"] = true }},
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
	if err := adapter.Init(context.Background(), testConfig(server.URL), socialhub.WithHTTPClient(server.Client()), socialhub.WithSecretResolver(testSecrets{"secret://consumer": "consumer-secret"})); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); errorCode(err) != socialhub.CodeNotFound {
		t.Fatalf("missing account=%v", err)
	}
	oauth, err := adapter.OAuth(context.Background(), "main")
	if err != nil || oauth.ConsumerKey != "api-key" || oauth.ConsumerSecret != "consumer-secret" || oauth.AccessTokenURL != server.URL+"/access_token" {
		t.Fatalf("oauth=%#v err=%v", oauth, err)
	}

	withoutSecret := testConfig(server.URL)
	withoutSecret.Accounts[0].AccessTokenRef = ""
	withoutSecret.Accounts[0].SecretRef = ""
	withoutSecret.Accounts[0].Approval.Scopes = nil
	withoutSecret.Accounts[0].Settings = map[string]any{"user_id": "owner@N01"}
	publicAdapter := &Adapter{}
	if err := publicAdapter.Init(context.Background(), withoutSecret); err != nil {
		t.Fatal(err)
	}
	if _, err := publicAdapter.OAuth(context.Background(), "main"); errorCode(err) != socialhub.CodeInvalidArgument {
		t.Fatalf("OAuth without secret=%v", err)
	}
}

func TestPermissionHierarchy(t *testing.T) {
	if !permissionAtLeast(PermissionDelete, PermissionWrite) || !permissionAtLeast(PermissionWrite, PermissionRead) || permissionAtLeast(PermissionRead, PermissionWrite) || permissionAtLeast("", PermissionRead) {
		t.Fatal("permission hierarchy is invalid")
	}
	client := &Client{permission: PermissionRead}
	if err := client.requirePermission("test", PermissionWrite); errorCode(err) != socialhub.CodeApprovalRequired {
		t.Fatalf("permission error=%v", err)
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

func boolPointer(value bool) *bool { return &value }

var _ socialhub.Clock = fixedClock{}
var _ socialhub.SecretResolver = testSecrets{}

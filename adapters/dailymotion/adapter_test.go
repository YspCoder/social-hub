package dailymotion

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

var testNow = time.Date(2026, time.August, 2, 3, 0, 0, 0, time.UTC)

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
		Adapter:  adapterName,
		Settings: map[string]any{"base_url": baseURL + "/v2", "token_url": baseURL + "/token"},
		Accounts: []socialhub.AccountConfig{{
			ID: "main", AccessTokenRef: "secret://access", Approval: socialhub.ApprovalConfig{Scopes: []string{BundleOrganization}},
			Settings: map[string]any{"profile_id": "profile-1"},
		}},
	}
}

func newTestClient(t *testing.T, server *httptest.Server) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	err := adapter.Init(context.Background(), testConfig(server.URL), socialhub.WithHTTPClient(server.Client()), socialhub.WithSecretResolver(testSecrets{"secret://access": "access-token"}), socialhub.WithClock(fixedClock{testNow}))
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

func TestAdapterLifecycleMetadataAndCapabilities(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newTestClient(t, server)
	metadata := adapter.Metadata()
	if adapter.Name() != adapterName || metadata.Product != productName || metadata.APIVersion != apiVersion || metadata.VerifiedAt.IsZero() {
		t.Fatalf("metadata=%#v", metadata)
	}
	if !containsString(socialhub.Adapters(), adapterName) {
		t.Fatalf("registered adapters missing %q", adapterName)
	}
	if client.Platform() != "dailymotion" || client.Account() != "main" || client.ProfileWorkflow() == nil || client.VideoWorkflow() == nil || client.VideoUploadWorkflow() == nil || client.PlaylistWorkflow() == nil {
		t.Fatalf("client workflows missing")
	}
	if _, ok := client.Fetcher(); !ok {
		t.Fatal("fetcher unavailable")
	}
	if _, ok := client.Publisher(); ok {
		t.Fatal("publisher must be unavailable")
	}
	if _, ok := client.MediaUploader(); ok {
		t.Fatal("common uploader must be unavailable")
	}
	if _, ok := client.Reactor(); ok {
		t.Fatal("reactor must be unavailable")
	}
	if _, ok := client.Messenger(); ok {
		t.Fatal("messenger must be unavailable")
	}
	if _, ok := client.WebhookHandler(); ok {
		t.Fatal("webhook verifier must be unavailable")
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil || !capabilities.Has(CapabilityVideoUpload) || capabilities[socialhub.CapWebhook].Supported {
		t.Fatalf("capabilities=%#v err=%v", capabilities, err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("client close: %v", err)
	}
	if err := adapter.Close(); err != nil {
		t.Fatalf("adapter close: %v", err)
	}
	if _, err := adapter.Client(context.Background(), "main"); errorCode(err) != socialhub.CodeConflict {
		t.Fatalf("client after close=%v", err)
	}
	if err := adapter.Init(context.Background(), testConfig(server.URL)); errorCode(err) != socialhub.CodeConflict {
		t.Fatalf("init after close=%v", err)
	}
}

func TestAdapterValidationAndOAuthAccess(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	valid := testConfig(server.URL)
	tests := []struct {
		name   string
		mutate func(*socialhub.AdapterConfig)
	}{
		{"name", func(c *socialhub.AdapterConfig) { c.Adapter = "wrong" }},
		{"endpoint", func(c *socialhub.AdapterConfig) { c.Settings["base_url"] = "://bad" }},
		{"credentials", func(c *socialhub.AdapterConfig) { c.Accounts[0].AccessTokenRef = "" }},
		{"scopes", func(c *socialhub.AdapterConfig) { c.Accounts[0].Approval.Scopes = []string{"unknown"} }},
		{"profile", func(c *socialhub.AdapterConfig) { c.Accounts[0].Settings["profile_id"] = "bad/id" }},
		{"settings", func(c *socialhub.AdapterConfig) { c.Settings["unknown"] = true }},
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
	if err := adapter.Init(context.Background(), valid, socialhub.WithSecretResolver(testSecrets{"secret://access": "token"})); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); errorCode(err) != socialhub.CodeNotFound {
		t.Fatalf("missing account=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "main"); errorCode(err) != socialhub.CodeInvalidArgument {
		t.Fatalf("oauth static account=%v", err)
	}

	credentials := testConfig(server.URL)
	credentials.Accounts[0].AccessTokenRef = ""
	credentials.Accounts[0].ClientID = "client-id"
	credentials.Accounts[0].SecretRef = "secret://client"
	credentialAdapter := &Adapter{}
	if err := credentialAdapter.Init(context.Background(), credentials, socialhub.WithSecretResolver(testSecrets{"secret://client": "client-secret"}), socialhub.WithHTTPClient(server.Client()), socialhub.WithClock(fixedClock{testNow})); err != nil {
		t.Fatal(err)
	}
	oauth, err := credentialAdapter.OAuth(context.Background(), "main")
	if err != nil || oauth.ClientID != "client-id" || oauth.ClientSecret != "client-secret" {
		t.Fatalf("oauth=%#v err=%v", oauth, err)
	}
}

func TestCapabilityScopeExpansion(t *testing.T) {
	client := &Client{scopes: []string{BundlePublic}}
	if !client.hasScope(ScopeVideoRead) || client.hasScope(ScopeVideoManage) || client.hasScope(ScopeAccountRead) {
		t.Fatal("bundle.public expansion is wrong")
	}
	client.scopes = []string{BundleUser}
	if !client.hasScope(ScopeAccountRead) || !client.hasScope(ScopePlaylistManage) || client.hasScope(ScopePlayerManage) {
		t.Fatal("bundle.user expansion is wrong")
	}
	client.scopes = []string{ScopeVideoManage}
	if !client.hasScope(ScopeVideoRead) {
		t.Fatal("manage must imply read")
	}
	client.scopes = []string{ScopeProfileRead}
	if err := client.requireScopes("test", ScopeVideoRead); errorCode(err) != socialhub.CodeApprovalRequired {
		t.Fatalf("scope error=%v", err)
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

func writeJSON(writer http.ResponseWriter, status int, value string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_, _ = writer.Write([]byte(value))
}

func boolPointer(value bool) *bool     { return &value }
func textPointer(value string) *string { return &value }

var _ socialhub.Clock = fixedClock{}
var _ socialhub.SecretResolver = testSecrets{}

package peertube

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

var testNow = time.Date(2026, time.August, 2, 8, 0, 0, 0, time.UTC)

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

type testSecrets map[string]string

func (s testSecrets) Resolve(_ context.Context, reference string) (string, error) {
	value, ok := s[reference]
	if !ok {
		return "", errors.New("missing test secret")
	}
	return value, nil
}

func testConfig(instanceURL string) socialhub.AdapterConfig {
	return socialhub.AdapterConfig{
		Adapter: adapterName,
		Accounts: []socialhub.AccountConfig{{
			ID: "main", AccessTokenRef: "test://access", Approval: socialhub.ApprovalConfig{Scopes: []string{"user"}},
			Settings: map[string]any{"instance_url": instanceURL, "account_name": "creator"},
		}},
	}
}

func newTestClient(t *testing.T, server *httptest.Server) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server.URL),
		socialhub.WithHTTPClient(server.Client()), socialhub.WithSecretResolver(testSecrets{"test://access": "access-token"}),
		socialhub.WithClock(fixedClock{testNow})); err != nil {
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
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatalf("registered adapters=%v", socialhub.Adapters())
	}
	if client.Platform() != "peertube" || client.Account() != "main" || client.VideoWorkflow() == nil || client.ChannelWorkflow() == nil || client.CommentWorkflow() == nil {
		t.Fatal("client identity or typed workflows missing")
	}
	if _, ok := client.Fetcher(); !ok {
		t.Fatal("fetcher unavailable")
	}
	if _, ok := client.Reactor(); !ok {
		t.Fatal("reactor unavailable")
	}
	if _, ok := client.Publisher(); ok {
		t.Fatal("publisher must be unavailable")
	}
	if _, ok := client.MediaUploader(); ok {
		t.Fatal("media uploader must be unavailable")
	}
	if _, ok := client.Messenger(); ok {
		t.Fatal("messenger must be unavailable")
	}
	if _, ok := client.WebhookHandler(); ok {
		t.Fatal("webhook handler must be unavailable")
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil || !capabilities.Has(socialhub.CapFetch) || !capabilities.Has(socialhub.CapReact) || !capabilities.Has(CapabilityVideoWorkflow) || !capabilities.Has(CapabilityChannelWorkflow) {
		t.Fatalf("capabilities=%#v err=%v", capabilities, err)
	}
	if capabilities[socialhub.CapPublish].Supported || capabilities[socialhub.CapWebhook].Supported {
		t.Fatalf("unsupported capabilities=%#v", capabilities)
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
	if _, err := adapter.OAuth("main"); errorCode(err) != socialhub.CodeConflict {
		t.Fatalf("oauth after close=%v", err)
	}
	if err := adapter.Init(context.Background(), testConfig(server.URL)); errorCode(err) != socialhub.CodeConflict {
		t.Fatalf("init after close=%v", err)
	}
}

func TestAdapterValidationAndOverrides(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	tests := []struct {
		name   string
		mutate func(*socialhub.AdapterConfig)
	}{
		{"adapter", func(c *socialhub.AdapterConfig) { c.Adapter = "wrong" }},
		{"adapter settings", func(c *socialhub.AdapterConfig) { c.Settings = map[string]any{"unknown": true} }},
		{"instance", func(c *socialhub.AdapterConfig) { c.Accounts[0].Settings["instance_url"] = "https://example.test/path" }},
		{"handle", func(c *socialhub.AdapterConfig) { c.Accounts[0].Settings["account_name"] = "bad/name" }},
		{"role", func(c *socialhub.AdapterConfig) { c.Accounts[0].Approval.Scopes = []string{"owner"} }},
		{"duplicate role", func(c *socialhub.AdapterConfig) { c.Accounts[0].Approval.Scopes = []string{"user", "user"} }},
		{"account settings", func(c *socialhub.AdapterConfig) { c.Accounts[0].Settings["unknown"] = true }},
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
	if err := adapter.Init(context.Background(), testConfig(server.URL), socialhub.WithHTTPClient(server.Client()), socialhub.WithSecretResolver(testSecrets{}), socialhub.WithClock(fixedClock{testNow})); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); errorCode(err) != socialhub.CodeNotFound {
		t.Fatalf("missing account=%v", err)
	}
	if _, err := adapter.Client(context.Background(), "main"); errorCode(err) != socialhub.CodeUnauthenticated {
		t.Fatalf("missing token secret=%v", err)
	}
	oauth, err := adapter.OAuth("main")
	if err != nil || oauth.InstanceURL != server.URL || oauth.HTTPClient == nil || oauth.Clock == nil {
		t.Fatalf("oauth=%#v err=%v", oauth, err)
	}
	if _, err := adapter.OAuth("missing"); errorCode(err) != socialhub.CodeNotFound {
		t.Fatalf("missing oauth account=%v", err)
	}
	bootstrapConfig := testConfig(server.URL)
	bootstrapConfig.Accounts[0].AccessTokenRef = ""
	bootstrap := &Adapter{}
	if err := bootstrap.Init(context.Background(), bootstrapConfig, socialhub.WithHTTPClient(server.Client()), socialhub.WithClock(fixedClock{testNow})); err != nil {
		t.Fatalf("OAuth bootstrap init=%v", err)
	}
	if _, err := bootstrap.OAuth("main"); err != nil {
		t.Fatalf("OAuth bootstrap helper=%v", err)
	}
	if _, err := bootstrap.Client(context.Background(), "main"); errorCode(err) != socialhub.CodeUnauthenticated {
		t.Fatalf("bootstrap client without token=%v", err)
	}
}

func TestRoleSemantics(t *testing.T) {
	if !roleGranted([]string{"admin"}, "user") || !roleGranted([]string{"moderator"}, "user") || roleGranted([]string{"user"}, "moderator") {
		t.Fatal("role hierarchy is wrong")
	}
	client := &Client{roles: []string{"moderator"}, instanceURL: "https://video.example"}
	if err := client.requireUser("test"); err != nil {
		t.Fatal(err)
	}
	client.roles = []string{"admin"}
	state := capabilityState(socialhub.CapFetch, client.roles, "reason", "docs")
	if state.Approval != socialhub.ApprovalGranted {
		t.Fatalf("state=%#v", state)
	}
}

func errorCode(err error) socialhub.ErrorCode {
	var platformErr *socialhub.Error
	if errors.As(err, &platformErr) {
		return platformErr.Code
	}
	return ""
}

func boolPointer(value bool) *bool     { return &value }
func intPointer(value int) *int        { return &value }
func int64Pointer(value int64) *int64  { return &value }
func textPointer(value string) *string { return &value }

var _ socialhub.Clock = fixedClock{}
var _ socialhub.SecretResolver = testSecrets{}

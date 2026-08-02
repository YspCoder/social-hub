package kick

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

var testNow = time.Date(2026, time.August, 2, 10, 0, 0, 0, time.UTC)

type mapResolver map[string]string

func (resolver mapResolver) Resolve(_ context.Context, reference string) (string, error) {
	value, found := resolver[reference]
	if !found {
		return "", fmt.Errorf("secret not found")
	}
	return value, nil
}

type fixedClock struct{ now time.Time }

func (clock fixedClock) Now() time.Time { return clock.now }

func testConfig(baseURL, tokenType string, scopes []string) socialhub.AdapterConfig {
	return socialhub.AdapterConfig{
		Adapter: adapterName, Product: productName,
		Settings: map[string]any{
			"base_url": baseURL, "auth_url": baseURL + "/oauth/authorize", "token_url": baseURL + "/oauth/token",
			"revoke_url": baseURL + "/oauth/revoke", "introspect_url": baseURL + "/oauth/token/introspect",
		},
		Accounts: []socialhub.AccountConfig{{
			ID: "main", ClientID: "client-id", SecretRef: "secret://client", AccessTokenRef: "secret://token",
			Approval: socialhub.ApprovalConfig{Scopes: scopes},
			Settings: map[string]any{
				"broadcaster_user_id": "123", "channel_slug": "streamer", "token_type": tokenType,
			},
		}},
	}
}

func newTestClient(t *testing.T, server *httptest.Server, tokenType string, scopes []string) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	err := adapter.Init(context.Background(), testConfig(server.URL, tokenType, scopes),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{"secret://token": "access-token", "secret://client": "client-secret"}),
		socialhub.WithClock(fixedClock{now: testNow}),
	)
	if err != nil {
		t.Fatalf("init adapter: %v", err)
	}
	common, err := adapter.Client(context.Background(), "main")
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	return adapter, common.(*Client)
}

func TestAdapterLifecycleAndCapabilities(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newTestClient(t, server, "user", []string{"user:read", "channel:read", "chat:write", "events:subscribe"})
	if adapter.Name() != adapterName || adapter.Metadata().APIVersion != "v2" || client.Platform() != "kick" || client.Account() != "main" {
		t.Fatal("adapter metadata or client identity mismatch")
	}
	found := false
	for _, name := range socialhub.Adapters() {
		found = found || name == adapterName
	}
	if !found {
		t.Fatal("Kick adapter was not registered")
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil || !capabilities[socialhub.CapWebhook].Supported || !capabilities[CapabilityLivestreams].Supported || !capabilities[CapabilityChat].Supported {
		t.Fatalf("capabilities: %#v %v", capabilities, err)
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
	if _, ok := client.WebhookHandler(); !ok {
		t.Fatal("webhook handler not exposed")
	}
	if client.UserWorkflow() == nil || client.ChannelWorkflow() == nil || client.LivestreamWorkflow() == nil ||
		client.CategoryWorkflow() == nil || client.ChatWorkflow() == nil || client.SubscriptionWorkflow() == nil {
		t.Fatal("typed workflows not exposed")
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing client: %v", err)
	}
	oauth, err := adapter.OAuth(context.Background(), "main")
	if err != nil || oauth.ClientSecret != "client-secret" {
		t.Fatalf("OAuth helper: %#v %v", oauth, err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("client close: %v", err)
	}
	if err := adapter.Close(); err != nil {
		t.Fatalf("adapter close: %v", err)
	}
	if _, err := adapter.Client(context.Background(), "main"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close: %v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "main"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("oauth after close: %v", err)
	}
	if err := adapter.Init(context.Background(), testConfig(server.URL, "user", nil)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("init after close: %v", err)
	}
}

func TestAdapterValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	valid := testConfig(server.URL, "user", nil)
	cases := []func(*socialhub.AdapterConfig){
		func(config *socialhub.AdapterConfig) { config.Adapter = "kick/wrong" },
		func(config *socialhub.AdapterConfig) { config.Settings["base_url"] = "ftp://bad" },
		func(config *socialhub.AdapterConfig) { config.Accounts[0].AccessTokenRef = "" },
		func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["token_type"] = "bot" },
		func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["broadcaster_user_id"] = "zero" },
		func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["channel_slug"] = "bad/slug" },
		func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["webhook_public_key"] = "bad key" },
		func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["unknown"] = true },
	}
	for index, mutate := range cases {
		config := testConfig(server.URL, "user", nil)
		mutate(&config)
		if err := (&Adapter{}).Init(context.Background(), config); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("validation case %d: %v", index, err)
		}
	}
	missingAccount := valid
	missingAccount.Accounts = nil
	if err := (&Adapter{}).Init(context.Background(), missingAccount); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("missing account: %v", err)
	}
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), valid,
		socialhub.WithHTTPClient(server.Client()), socialhub.WithSecretResolver(mapResolver{})); err != nil {
		t.Fatalf("valid init: %v", err)
	}
	if _, err := adapter.Client(context.Background(), "main"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("missing token: %v", err)
	}
	config := testConfig(server.URL, "user", nil)
	config.Accounts[0].ClientID = ""
	config.Accounts[0].SecretRef = ""
	adapter = &Adapter{}
	if err := adapter.Init(context.Background(), config, socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{"secret://token": "token"})); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.OAuth(context.Background(), "main"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("incomplete OAuth config: %v", err)
	}
}

func TestRejectCrossOriginRedirect(t *testing.T) {
	origin := httptest.NewRequest(http.MethodGet, "https://api.kick.com/public/v1/users", nil)
	sameOrigin := httptest.NewRequest(http.MethodGet, "https://api.kick.com/public/v1/channels", nil)
	if err := rejectCrossOriginRedirect(sameOrigin, []*http.Request{origin}); err != nil {
		t.Fatalf("same-origin redirect: %v", err)
	}
	crossOrigin := httptest.NewRequest(http.MethodGet, "https://other.test/token", nil)
	if err := rejectCrossOriginRedirect(crossOrigin, []*http.Request{origin}); !errors.Is(err, http.ErrUseLastResponse) {
		t.Fatalf("cross-origin redirect: %v", err)
	}
	via := make([]*http.Request, 10)
	for index := range via {
		via[index] = origin
	}
	if err := rejectCrossOriginRedirect(sameOrigin, via); err == nil {
		t.Fatal("redirect loop accepted")
	}
	if err := rejectCrossOriginRedirect(sameOrigin, nil); err != nil {
		t.Fatalf("initial request: %v", err)
	}
}

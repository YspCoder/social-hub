package kitsu

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

const (
	testAccessToken  = "kitsu-access-token"
	testClientSecret = "kitsu-client-secret"
)

var testNow = time.Date(2026, time.August, 2, 8, 9, 10, 0, time.UTC)

type mapResolver map[string]string

func (r mapResolver) Resolve(_ context.Context, reference string) (string, error) {
	value, ok := r[reference]
	if !ok {
		return "", errors.New("missing fixture secret")
	}
	return value, nil
}

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

func testConfig(server *httptest.Server, withToken, withUser bool) socialhub.AdapterConfig {
	account := socialhub.AccountConfig{ID: "fan", ClientID: "client-id", SecretRef: "test://client-secret"}
	if withToken {
		account.AccessTokenRef = "test://access-token"
	}
	if withUser {
		account.Settings = map[string]any{"user_id": "42"}
	}
	return socialhub.AdapterConfig{
		Adapter: adapterName,
		Settings: map[string]any{
			"api_url": server.URL + "/api/edge", "token_url": server.URL + "/oauth/token",
			"user_agent": "social-hub-kitsu-tests/1.0",
		},
		Accounts: []socialhub.AccountConfig{account},
	}
}

func newTestClient(t *testing.T, server *httptest.Server, withToken, withUser bool) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server, withToken, withUser),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{
			"test://client-secret": testClientSecret, "test://access-token": testAccessToken,
		}),
		socialhub.WithClock(fixedClock{now: testNow}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "fan")
	if err != nil {
		t.Fatal(err)
	}
	client, ok := common.(*Client)
	if !ok {
		t.Fatalf("unexpected client type %T", common)
	}
	return adapter, client
}

func TestAdapterLifecycleAndCapabilities(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newTestClient(t, server, true, true)
	if adapter.Name() != adapterName || adapter.Metadata().APIVersion != apiVersion || adapter.Metadata().DocURL != documentationURL {
		t.Fatal("unexpected adapter metadata")
	}
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatal("adapter was not registered")
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil || !capabilities[CapabilityLibrary].Supported || capabilities[CapabilityLibrary].Approval != socialhub.ApprovalGranted {
		t.Fatalf("unexpected capabilities: %#v, %v", capabilities, err)
	}
	if capabilities[socialhub.CapFetch].Supported {
		t.Fatal("common fetcher must remain unsupported")
	}
	if _, ok := client.Fetcher(); ok {
		t.Fatal("common fetcher unexpectedly exposed")
	}
	if client.Platform() != "kitsu" || client.Account() != "fan" {
		t.Fatal("unexpected client identity")
	}
	if _, ok := client.Publisher(); ok {
		t.Fatal("publisher unexpectedly exposed")
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
	if _, ok := client.WebhookHandler(); ok {
		t.Fatal("webhook unexpectedly exposed")
	}
	if client.TokenWorkflow() == nil || client.AnimeWorkflow() == nil || client.MangaWorkflow() == nil ||
		client.UserWorkflow() == nil || client.LibraryWorkflow() == nil || client.PostWorkflow() == nil || client.CommentWorkflow() == nil {
		t.Fatal("typed workflows are missing")
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "fan"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("expected closed adapter error, got %v", err)
	}
}

func TestPublicClientAndConfigurationValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, false, false)
	capabilities, _ := client.Capabilities(context.Background())
	if capabilities[CapabilityLibrary].Approval != socialhub.ApprovalRequired {
		t.Fatal("unauthenticated mutation capability must require approval")
	}
	if _, err := client.CreatePost(context.Background(), CreatePostRequest{Content: "hello"}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("expected approval error, got %v", err)
	}
	if _, err := client.GetCurrentUser(context.Background()); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("expected current-user approval error, got %v", err)
	}
	_, tokenOnly := newTestClient(t, server, true, false)
	if _, err := tokenOnly.CreatePost(context.Background(), CreatePostRequest{Content: "hello"}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("expected user ID approval error, got %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*socialhub.AdapterConfig)
	}{
		{"adapter", func(c *socialhub.AdapterConfig) { c.Adapter = "kitsu" }},
		{"endpoint", func(c *socialhub.AdapterConfig) { c.Settings["api_url"] = "javascript:bad" }},
		{"scope", func(c *socialhub.AdapterConfig) { c.Accounts[0].Approval.Scopes = []string{"write"} }},
		{"user", func(c *socialhub.AdapterConfig) { c.Accounts[0].Settings = map[string]any{"user_id": "01"} }},
		{"unknown", func(c *socialhub.AdapterConfig) { c.Accounts[0].Settings = map[string]any{"unknown": true} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := testConfig(server, false, false)
			test.mutate(&config)
			if err := (&Adapter{}).Init(context.Background(), config); !errors.Is(err, socialhub.ErrInvalidArgument) {
				t.Fatalf("expected invalid argument, got %v", err)
			}
		})
	}
}

func TestClientSecretResolutionAndAccountLookup(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter := &Adapter{}
	config := testConfig(server, true, true)
	if err := adapter.Init(context.Background(), config,
		socialhub.WithHTTPClient(server.Client()), socialhub.WithSecretResolver(mapResolver{})); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "fan"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("expected secret resolution failure, got %v", err)
	}
	adapter, _ = newTestClient(t, server, true, true)
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("expected account not found, got %v", err)
	}
}

func TestRedirectsAreRejected(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("redirect target must not be reached")
	}))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, target.URL, http.StatusFound)
	}))
	defer server.Close()
	_, client := newTestClient(t, server, true, true)
	if _, err := client.GetAnime(context.Background(), "1"); err == nil {
		t.Fatal("expected redirect response to fail")
	}
}

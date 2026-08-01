package zhihu

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

type mapResolver map[string]string

func (r mapResolver) Resolve(_ context.Context, reference string) (string, error) {
	value, ok := r[reference]
	if !ok {
		return "", errors.New("missing test secret")
	}
	return value, nil
}

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

func newTestAdapter(t *testing.T, server *httptest.Server, approved, withOAuthToken, withOAuthApp bool) (*Adapter, *Client) {
	t.Helper()
	settings := map[string]any{"approved": approved}
	if withOAuthToken {
		settings["oauth_token_ref"] = "test://oauth-token"
	}
	account := socialhub.AccountConfig{
		ID:             "primary",
		AccessTokenRef: "test://access-secret",
		Settings:       settings,
	}
	if withOAuthApp {
		account.ClientID = "app-id"
		account.SecretRef = "test://app-key"
	}
	config := socialhub.AdapterConfig{
		Adapter: adapterName,
		Product: "data-api",
		Settings: map[string]any{
			"base_url":  server.URL,
			"auth_url":  server.URL + "/authorize",
			"token_url": server.URL + "/access_token",
		},
		Accounts: []socialhub.AccountConfig{account},
	}
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), config,
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{
			"test://access-secret": "access-secret",
			"test://oauth-token":   "oauth-user-token",
			"test://app-key":       "app-key",
		}),
		socialhub.WithClock(fixedClock{now: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "primary")
	if err != nil {
		t.Fatal(err)
	}
	return adapter, common.(*Client)
}

func TestAdapterRegistrationAndCapabilitySurface(t *testing.T) {
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatalf("registered adapters=%v", socialhub.Adapters())
	}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests++ }))
	defer server.Close()
	adapter, client := newTestAdapter(t, server, false, false, false)

	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range []socialhub.Capability{socialhub.CapFetch, CapabilitySearch, CapabilityHotList} {
		state := capabilities[capability]
		if !state.Supported || state.Approval != socialhub.ApprovalRequired || capabilities.Has(capability) {
			t.Fatalf("capability %q=%#v", capability, state)
		}
	}
	if fetcher, ok := client.Fetcher(); !ok || fetcher == nil {
		t.Fatal("fetcher unavailable")
	}
	if publisher, ok := client.Publisher(); ok || publisher != nil {
		t.Fatal("publisher should be unavailable")
	}
	if uploader, ok := client.MediaUploader(); ok || uploader != nil {
		t.Fatal("media uploader should be unavailable")
	}
	if reactor, ok := client.Reactor(); ok || reactor != nil {
		t.Fatal("reactor should be unavailable")
	}
	if messenger, ok := client.Messenger(); ok || messenger != nil {
		t.Fatal("messenger should be unavailable")
	}
	if webhook, ok := client.WebhookHandler(); ok || webhook != nil {
		t.Fatal("webhook handler should be unavailable")
	}
	if client.SearchWorkflow() == nil {
		t.Fatal("search workflow unavailable")
	}
	if _, err := client.SearchWorkflow().Search(context.Background(), SearchRequest{Query: "Go"}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("unapproved search error=%v", err)
	}
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("unapproved list error=%v", err)
	}
	if requests != 0 {
		t.Fatalf("unapproved requests=%d", requests)
	}
	if _, err := adapter.OAuth(context.Background(), "primary"); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("OAuth without approved app error=%v", err)
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "primary"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close error=%v", err)
	}
}

func TestAdapterValidatesCredentials(t *testing.T) {
	tests := []socialhub.AccountConfig{
		{ID: "primary"},
		{ID: "primary", AccessTokenRef: "test://access-secret", ClientID: "app-id"},
		{ID: "primary", AccessTokenRef: "test://access-secret", SecretRef: "test://app-key"},
	}
	for _, account := range tests {
		config := socialhub.AdapterConfig{Adapter: adapterName, Product: "data-api", Accounts: []socialhub.AccountConfig{account}}
		if err := (&Adapter{}).Init(context.Background(), config); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("account=%#v error=%v", account, err)
		}
	}
}

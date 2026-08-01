package line

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
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

var (
	testNow     = time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)
	testUserID  = "U" + strings.Repeat("a", 32)
	testUserID2 = "U" + strings.Repeat("b", 32)
	testGroupID = "C" + strings.Repeat("c", 32)
	testRoomID  = "R" + strings.Repeat("d", 32)
	testBotID   = "U" + strings.Repeat("e", 32)
)

func testConfig(serverURL string, withSecret bool) socialhub.AdapterConfig {
	account := socialhub.AccountConfig{
		ID: "main", AccessTokenRef: "test://channel-token",
		Settings: map[string]any{"bot_user_id": testBotID},
	}
	if withSecret {
		account.ClientID = "1234567890"
		account.SecretRef = "test://channel-secret"
	}
	return socialhub.AdapterConfig{
		Adapter:  adapterName,
		Settings: map[string]any{"base_url": serverURL, "data_base_url": serverURL, "token_base_url": serverURL},
		Accounts: []socialhub.AccountConfig{account},
	}
}

func newTestAdapter(t *testing.T, server *httptest.Server, withSecret bool) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server.URL, withSecret),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(testResolver{
			"test://channel-token": "channel-token", "test://channel-secret": "channel-secret",
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

func hasErrorCode(err error, code socialhub.ErrorCode) bool {
	var platformErr *socialhub.Error
	return errors.As(err, &platformErr) && platformErr.Code == code
}

func TestAdapterRegistrationCapabilitiesAndLifecycle(t *testing.T) {
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatalf("registered adapters=%v", socialhub.Adapters())
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newTestAdapter(t, server, true)
	if adapter.Name() != adapterName || client.Platform() != "line" || client.Account() != "main" {
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
	for _, name := range []socialhub.Capability{socialhub.CapMessage, socialhub.CapWebhook, CapabilityTypedMessages, CapabilityProfiles, CapabilityContent, CapabilityQuota} {
		if !capabilities.Has(name) || capabilities[name].Capability != name {
			t.Fatalf("capability %s=%#v", name, capabilities[name])
		}
	}
	for _, name := range []socialhub.Capability{socialhub.CapPublish, socialhub.CapFetch, socialhub.CapMedia, socialhub.CapReact} {
		if capabilities.Has(name) {
			t.Fatalf("unsupported capability %s=%#v", name, capabilities[name])
		}
	}
	if _, ok := client.Publisher(); ok {
		t.Fatal("publisher should be unavailable")
	}
	if _, ok := client.Fetcher(); ok {
		t.Fatal("fetcher should be unavailable")
	}
	if _, ok := client.MediaUploader(); ok {
		t.Fatal("media uploader should be unavailable")
	}
	if _, ok := client.Reactor(); ok {
		t.Fatal("reactor should be unavailable")
	}
	if _, ok := client.Messenger(); !ok {
		t.Fatal("messenger should be available")
	}
	if _, ok := client.WebhookHandler(); !ok {
		t.Fatal("webhook should be available")
	}
	if client.MessageWorkflow() == nil || client.ProfileWorkflow() == nil || client.ContentWorkflow() == nil || client.QuotaWorkflow() == nil {
		t.Fatal("typed workflows should be available")
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	tokens, err := adapter.Tokens(context.Background(), "main")
	if err != nil || tokens.ChannelID != "1234567890" || tokens.ChannelSecret != "channel-secret" || tokens.Clock == nil {
		t.Fatalf("tokens=%#v error=%v", tokens, err)
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "main"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close=%v", err)
	}
	if _, err := adapter.Tokens(context.Background(), "main"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("tokens after close=%v", err)
	}
	if err := adapter.Init(context.Background(), testConfig(server.URL, true)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("init after close=%v", err)
	}
}

func TestAdapterValidationAndTokenOnlyClient(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	invalid := []socialhub.AdapterConfig{
		{Adapter: "line", Accounts: []socialhub.AccountConfig{{ID: "main", AccessTokenRef: "token"}}},
		{Adapter: adapterName, Settings: map[string]any{"base_url": "https://user:pass@example.test"}, Accounts: []socialhub.AccountConfig{{ID: "main", AccessTokenRef: "token"}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main"}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", AccessTokenRef: "token", ClientID: "channel"}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", AccessTokenRef: "token", Settings: map[string]any{"bot_user_id": "bad"}}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", AccessTokenRef: "token", Settings: map[string]any{"unknown": true}}}},
	}
	for index, config := range invalid {
		if err := (&Adapter{}).Init(context.Background(), config); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("invalid config %d=%v", index, err)
		}
	}
	adapter, client := newTestAdapter(t, server, false)
	if _, ok := client.WebhookHandler(); ok {
		t.Fatal("token-only client exposes webhook")
	}
	capabilities, _ := client.Capabilities(context.Background())
	if capabilities.Has(socialhub.CapWebhook) {
		t.Fatalf("webhook capability=%#v", capabilities[socialhub.CapWebhook])
	}
	if _, err := adapter.Tokens(context.Background(), "main"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("token helper without credentials=%v", err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing client=%v", err)
	}
	if _, err := adapter.Tokens(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing token account=%v", err)
	}
}

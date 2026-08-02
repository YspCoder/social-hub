package deviantart

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
	testUserID      = "11111111-2222-3333-4444-555555555555"
	testDeviationID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	testCommentID   = "99999999-8888-7777-6666-555555555555"
)

var testNow = time.Date(2026, 8, 2, 3, 4, 5, 0, time.UTC)

type mapResolver map[string]string

func (resolver mapResolver) Resolve(_ context.Context, reference string) (string, error) {
	value, found := resolver[reference]
	if !found {
		return "", errors.New("missing test secret")
	}
	return value, nil
}

type fixedClock struct{ now time.Time }

func (clock fixedClock) Now() time.Time { return clock.now }

func testConfig(server *httptest.Server, scopes []string, secret bool) socialhub.AdapterConfig {
	account := socialhub.AccountConfig{
		ID: "artist", ClientID: "12345", AccessTokenRef: "test://access-token",
		Settings: map[string]any{"username": "sample-artist", "user_id": testUserID},
		Approval: socialhub.ApprovalConfig{Scopes: scopes},
	}
	if secret {
		account.SecretRef = "test://client-secret"
	}
	return socialhub.AdapterConfig{
		Adapter: adapterName, Product: productName,
		Settings: map[string]any{
			"base_url": server.URL + "/api/v1/oauth2", "auth_url": server.URL + "/oauth2/authorize",
			"token_url": server.URL + "/oauth2/token", "revoke_url": server.URL + "/oauth2/revoke",
			"user_agent": "social-hub-tests/1.0",
		},
		Accounts: []socialhub.AccountConfig{account},
	}
}

func newTestClient(t *testing.T, server *httptest.Server, scopes []string) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server, scopes, true),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{"test://access-token": "access-token", "test://client-secret": "client-secret"}),
		socialhub.WithClock(fixedClock{now: testNow}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "artist")
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
	scopes := []string{"basic", "user", "browse", "user.manage", "comment.post", "collection"}
	adapter, client := newTestClient(t, server, scopes)
	metadata := adapter.Metadata()
	if adapter.Name() != adapterName || metadata.Product != productName || metadata.APIVersion != apiVersion || metadata.DocURL != documentationURL {
		t.Fatalf("metadata=%#v", metadata)
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range []socialhub.Capability{
		socialhub.CapPublish, socialhub.CapFetch, socialhub.CapReact, CapabilityIdentity,
		CapabilityDeviations, CapabilityStatuses, CapabilityComments, CapabilityCollections,
	} {
		if !capabilities.Has(capability) || capabilities[capability].Approval != socialhub.ApprovalGranted {
			t.Fatalf("capability %s=%#v", capability, capabilities[capability])
		}
	}
	for _, capability := range []socialhub.Capability{socialhub.CapMedia, socialhub.CapMessage, socialhub.CapWebhook} {
		if capabilities.Has(capability) {
			t.Fatalf("unexpected capability %s", capability)
		}
	}
	if client.Platform() != "deviantart" || client.Account() != "artist" || client.Close() != nil ||
		client.UserWorkflow() == nil || client.DeviationWorkflow() == nil || client.GalleryWorkflow() == nil ||
		client.StatusWorkflow() == nil || client.CommentWorkflow() == nil || client.CollectionWorkflow() == nil {
		t.Fatalf("client=%#v", client)
	}
	if _, ok := client.Publisher(); !ok {
		t.Fatal("Publisher must be exposed")
	}
	if _, ok := client.Fetcher(); !ok {
		t.Fatal("Fetcher must be exposed")
	}
	if _, ok := client.Reactor(); !ok {
		t.Fatal("Reactor must be exposed")
	}
	if _, ok := client.MediaUploader(); ok {
		t.Fatal("MediaUploader must not be exposed")
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
	if _, err := adapter.Client(context.Background(), "artist"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "artist"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("oauth after close=%v", err)
	}
	if err := adapter.Init(context.Background(), testConfig(server, nil, true)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("reinit=%v", err)
	}
}

func TestAdapterValidationAndClientErrors(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	valid := testConfig(server, nil, true)
	tests := []struct {
		name   string
		mutate func(*socialhub.AdapterConfig)
	}{
		{"adapter", func(config *socialhub.AdapterConfig) { config.Adapter = "other" }},
		{"endpoint", func(config *socialhub.AdapterConfig) { config.Settings["base_url"] = "https://user:pass@example.test" }},
		{"endpoint query", func(config *socialhub.AdapterConfig) { config.Settings["token_url"] = "https://example.test/token?x=1" }},
		{"user agent", func(config *socialhub.AdapterConfig) { config.Settings["user_agent"] = "bad\nagent" }},
		{"username", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["username"] = "bad/name" }},
		{"user id", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["user_id"] = "bad id" }},
		{"scope", func(config *socialhub.AdapterConfig) { config.Accounts[0].Approval.Scopes = []string{"unknown"} }},
		{"duplicate scope", func(config *socialhub.AdapterConfig) {
			config.Accounts[0].Approval.Scopes = []string{"browse", " browse "}
		}},
		{"unknown setting", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["other"] = true }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := cloneConfig(valid)
			test.mutate(&config)
			if err := (&Adapter{}).Init(context.Background(), config); !errors.Is(err, socialhub.ErrInvalidArgument) {
				t.Fatalf("error=%v", err)
			}
		})
	}

	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), valid, socialhub.WithHTTPClient(server.Client()), socialhub.WithSecretResolver(mapResolver{})); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing client=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing oauth=%v", err)
	}
	if _, err := adapter.Client(context.Background(), "artist"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("unresolved token=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "artist"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("unresolved secret=%v", err)
	}

	public := cloneConfig(valid)
	public.Accounts[0].SecretRef = ""
	publicAdapter := &Adapter{}
	if err := publicAdapter.Init(context.Background(), public, socialhub.WithHTTPClient(server.Client())); err != nil {
		t.Fatal(err)
	}
	oauth, err := publicAdapter.OAuth(context.Background(), "artist")
	if err != nil || oauth.ClientSecret != "" {
		t.Fatalf("public OAuth=%#v err=%v", oauth, err)
	}
	public.Accounts[0].ClientID = ""
	publicAdapter = &Adapter{}
	if err := publicAdapter.Init(context.Background(), public); err != nil {
		t.Fatal(err)
	}
	if _, err := publicAdapter.OAuth(context.Background(), "artist"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("missing client ID=%v", err)
	}
}

func TestScopeGatingAndHelpers(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, []string{"browse"})
	if err := client.requireScopes("test", "collection"); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("scope error=%v", err)
	}
	if !client.validActor("") || !client.validActor(testUserID) || client.validActor("other") {
		t.Fatal("actor validation failed")
	}
	if !validResourceID(testDeviationID) || validResourceID("bad/path") || !validUsername("sample-artist") || validUsername("bad user") || validUsername(" sample") ||
		!validUserAgent("sdk/1") || validUserAgent("bad\nagent") || !validCursor("opaque:cursor") || validCursor("bad\n") {
		t.Fatal("validation helper failed")
	}
	query, offset, err := offsetQuery("page", "12", 99, 24, 0, 50000)
	if !errors.Is(err, socialhub.ErrInvalidArgument) || offset != 0 || query != nil {
		t.Fatalf("invalid max query=%v offset=%d err=%v", query, offset, err)
	}
	if _, _, err := offsetQuery("page", "bad", 10, 24, 0, 50000); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("invalid cursor=%v", err)
	}
	next := 20
	n, p, err := pageCursors(&next, true, 10, 10, 0, 50000)
	if err != nil || dereference(n) != "20" || dereference(p) != "0" {
		t.Fatalf("cursors next=%v previous=%v err=%v", n, p, err)
	}
	if _, _, err := pageCursors(nil, true, 0, 10, 0, 50000); err == nil {
		t.Fatal("missing next offset must fail")
	} else {
		var platformErr *socialhub.Error
		if !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodePlatformError {
			t.Fatalf("missing next offset=%v", err)
		}
	}
}

func cloneConfig(input socialhub.AdapterConfig) socialhub.AdapterConfig {
	output := input
	output.Settings = cloneMap(input.Settings)
	output.Accounts = append([]socialhub.AccountConfig(nil), input.Accounts...)
	output.Accounts[0].Settings = cloneMap(input.Accounts[0].Settings)
	output.Accounts[0].Approval.Scopes = append([]string(nil), input.Accounts[0].Approval.Scopes...)
	return output
}

func cloneMap(input map[string]any) map[string]any {
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func dereference(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func writeJSON(writer http.ResponseWriter, status int, body string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_, _ = writer.Write([]byte(body))
}

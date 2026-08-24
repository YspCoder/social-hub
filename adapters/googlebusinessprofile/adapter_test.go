package googlebusinessprofile

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
	testGoogleAccountID = "1001"
	testLocationID      = "2002"
	testPostID          = "post-1"
	testReviewID        = "review-1"
)

var testNow = time.Date(2026, time.August, 3, 1, 2, 3, 0, time.UTC)

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

func newTestAdapter(t *testing.T, server *httptest.Server, scopes []string) (*Adapter, *Client) {
	t.Helper()
	config := socialhub.AdapterConfig{
		Adapter: adapterName, Product: productName,
		Settings: map[string]any{
			"base_url": server.URL + "/v4", "auth_url": server.URL + "/authorize", "token_url": server.URL + "/token",
		},
		Accounts: []socialhub.AccountConfig{{
			ID: "store", ClientID: "client-id.apps.googleusercontent.com", SecretRef: "test://client-secret",
			AccessTokenRef: "test://access-token", Approval: socialhub.ApprovalConfig{Scopes: scopes},
			Settings: map[string]any{"google_account_id": testGoogleAccountID, "location_id": testLocationID, "language_code": "en-US"},
		}},
	}
	adapter := &Adapter{}
	secrets := mapResolver{"test://client-secret": "client-secret", "test://access-token": "access-token"}
	if err := adapter.Init(context.Background(), config,
		socialhub.WithHTTPClient(server.Client()), socialhub.WithSecretResolver(secrets), socialhub.WithClock(fixedClock{now: testNow}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "store")
	if err != nil {
		t.Fatal(err)
	}
	return adapter, common.(*Client)
}

func TestAdapterRegistrationMetadataAndCapabilities(t *testing.T) {
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatalf("registered adapters=%v", socialhub.Adapters())
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newTestAdapter(t, server, []string{businessScope})
	metadata := adapter.Metadata()
	if adapter.Name() != adapterName || metadata.APIVersion != apiVersion || metadata.Product != productName || metadata.DocURL != documentationURL || metadata.VerifiedAt.IsZero() {
		t.Fatalf("metadata=%#v", metadata)
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range []socialhub.Capability{socialhub.CapPublish, socialhub.CapFetch, CapabilityLocalPostWorkflow, CapabilityReviewWorkflow} {
		if !capabilities.Has(capability) {
			t.Fatalf("capability %q=%#v", capability, capabilities[capability])
		}
	}
	for _, capability := range []socialhub.Capability{socialhub.CapMedia, socialhub.CapReact, socialhub.CapMessage, socialhub.CapWebhook} {
		if capabilities.Has(capability) {
			t.Fatalf("unsupported capability %q=%#v", capability, capabilities[capability])
		}
	}
	if client.Platform() != platformName || client.Account() != "store" || client.LocalPostWorkflow() == nil || client.ReviewWorkflow() == nil {
		t.Fatalf("client=%#v", client)
	}
	if _, ok := client.Publisher(); !ok {
		t.Fatal("publisher must be exposed")
	}
	if _, ok := client.Fetcher(); !ok {
		t.Fatal("fetcher must be exposed")
	}
	if _, ok := client.MediaUploader(); ok {
		t.Fatal("media uploader must not be exposed")
	}
	if _, ok := client.Reactor(); ok {
		t.Fatal("reactor must not be exposed")
	}
	if _, ok := client.Messenger(); ok {
		t.Fatal("messenger must not be exposed")
	}
	if _, ok := client.WebhookHandler(); ok {
		t.Fatal("webhook handler must not be exposed")
	}
	if client.Close() != nil || adapter.Close() != nil {
		t.Fatal("close failed")
	}
	if _, err := adapter.Client(context.Background(), "store"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close error=%v", err)
	}
}

func TestAdapterValidationAndClientFailures(t *testing.T) {
	valid := socialhub.AccountConfig{
		ID: "store", AccessTokenRef: "test://token",
		Settings: map[string]any{"google_account_id": testGoogleAccountID, "location_id": testLocationID},
	}
	tests := []struct {
		name   string
		config socialhub.AdapterConfig
	}{
		{"wrong adapter", socialhub.AdapterConfig{Adapter: "other", Accounts: []socialhub.AccountConfig{valid}}},
		{"bad endpoint", socialhub.AdapterConfig{Adapter: adapterName, Settings: map[string]any{"base_url": "ftp://api.example"}, Accounts: []socialhub.AccountConfig{valid}}},
		{"missing token", socialhub.AdapterConfig{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "store", Settings: valid.Settings}}}},
		{"bad account", socialhub.AdapterConfig{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "store", AccessTokenRef: "test://token", Settings: map[string]any{"google_account_id": "a/b", "location_id": testLocationID}}}}},
		{"bad location", socialhub.AdapterConfig{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "store", AccessTokenRef: "test://token", Settings: map[string]any{"google_account_id": testGoogleAccountID, "location_id": "\n"}}}}},
		{"bad language", socialhub.AdapterConfig{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "store", AccessTokenRef: "test://token", Settings: map[string]any{"google_account_id": testGoogleAccountID, "location_id": testLocationID, "language_code": "en US"}}}}},
		{"unknown setting", socialhub.AdapterConfig{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "store", AccessTokenRef: "test://token", Settings: map[string]any{"google_account_id": testGoogleAccountID, "location_id": testLocationID, "unknown": true}}}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := (&Adapter{}).Init(context.Background(), test.config); !errors.Is(err, socialhub.ErrInvalidArgument) {
				t.Fatalf("error=%v", err)
			}
		})
	}

	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newTestAdapter(t, server, []string{"openid"})
	capabilities, _ := client.Capabilities(context.Background())
	if capabilities.Has(socialhub.CapPublish) || capabilities[socialhub.CapPublish].Approval != socialhub.ApprovalRequired {
		t.Fatalf("publish capability=%#v", capabilities[socialhub.CapPublish])
	}
	if _, err := client.GetUser(context.Background(), "me"); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("scope error=%v", err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing client error=%v", err)
	}
	adapter.options.Secrets = mapResolver{"test://client-secret": "client-secret"}
	if _, err := adapter.Client(context.Background(), "store"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("missing access token error=%v", err)
	}
	adapter.config.Accounts[0].ClientID = ""
	if _, err := adapter.OAuth(context.Background(), "store"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("OAuth validation error=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing OAuth account error=%v", err)
	}
}

func writeJSON(writer http.ResponseWriter, status int, value string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_, _ = writer.Write([]byte(value))
}

func errorCode(err error) socialhub.ErrorCode {
	var hubError *socialhub.Error
	if errors.As(err, &hubError) {
		return hubError.Code
	}
	return ""
}

func localPostJSON(id, topic, state string) string {
	return `{"name":"accounts/` + testGoogleAccountID + `/locations/` + testLocationID + `/localPosts/` + id + `","languageCode":"en-US","summary":"Hello customers","createTime":"2026-08-03T01:00:00Z","updateTime":"2026-08-03T01:01:00Z","state":"` + state + `","media":[{"name":"accounts/1001/locations/2002/media/media-1","mediaFormat":"PHOTO","googleUrl":"https://google.example/photo.jpg","sourceUrl":"https://cdn.example/photo.jpg"}],"searchUrl":"https://google.example/post/` + id + `","topicType":"` + topic + `"}`
}

func reviewJSON(id string) string {
	return `{"name":"accounts/` + testGoogleAccountID + `/locations/` + testLocationID + `/reviews/` + id + `","reviewId":"` + id + `","reviewer":{"profilePhotoUrl":"https://google.example/avatar.jpg","displayName":"Ada","isAnonymous":false},"starRating":"FIVE","comment":"Excellent","createTime":"2026-08-01T01:00:00Z","updateTime":"2026-08-01T02:00:00Z","reviewReply":{"comment":"Thank you","updateTime":"2026-08-01T03:00:00Z","reviewReplyState":"APPROVED"},"reviewMediaItems":[{"thumbnailUrl":"https://google.example/review.jpg","thumbnailLabel":"Storefront"}]}`
}

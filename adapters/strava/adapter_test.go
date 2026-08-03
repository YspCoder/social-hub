package strava

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
	testAthleteID  = "123456789012345"
	testActivityID = "789012345678901"
	testUploadID   = "456789012345678"
)

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

var testNow = time.Date(2026, time.August, 3, 1, 2, 3, 0, time.UTC)

func newTestAdapter(t *testing.T, server *httptest.Server, webhook bool, scopes []string) (*Adapter, *Client) {
	t.Helper()
	account := socialhub.AccountConfig{
		ID: "athlete", ClientID: "12345", SecretRef: "test://client-secret", AccessTokenRef: "test://access-token",
		Approval: socialhub.ApprovalConfig{Scopes: scopes}, Settings: map[string]any{"athlete_id": testAthleteID},
	}
	secrets := mapResolver{"test://client-secret": "client-secret", "test://access-token": "access-token"}
	if webhook {
		account.Settings["subscription_id"] = int64(2468)
		account.Webhook.TokenRef = "test://verify-token"
		secrets["test://verify-token"] = "verify-token"
	}
	config := socialhub.AdapterConfig{
		Adapter: adapterName, Product: productName,
		Settings: map[string]any{
			"base_url": server.URL + "/api", "auth_url": server.URL + "/authorize", "token_url": server.URL + "/oauth/token",
		},
		Accounts: []socialhub.AccountConfig{account},
	}
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), config,
		socialhub.WithHTTPClient(server.Client()), socialhub.WithSecretResolver(secrets), socialhub.WithClock(fixedClock{now: testNow}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "athlete")
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
	adapter, client := newTestAdapter(t, server, true, []string{"read", "activity:read_all", "activity:write"})
	metadata := adapter.Metadata()
	if adapter.Name() != adapterName || metadata.APIVersion != apiVersion || metadata.Product != productName || metadata.DocURL != documentationURL || metadata.VerifiedAt.IsZero() {
		t.Fatalf("metadata=%#v", metadata)
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range []socialhub.Capability{socialhub.CapFetch, socialhub.CapWebhook, CapabilityActivityWorkflow, CapabilityActivityUpload} {
		if !capabilities.Has(capability) {
			t.Fatalf("capability %q=%#v", capability, capabilities[capability])
		}
	}
	for _, capability := range []socialhub.Capability{socialhub.CapPublish, socialhub.CapMedia, socialhub.CapReact, socialhub.CapMessage} {
		if capabilities.Has(capability) {
			t.Fatalf("unsupported capability %q=%#v", capability, capabilities[capability])
		}
	}
	if client.Platform() != "strava" || client.Account() != "athlete" || client.ActivityWorkflow() == nil || client.ActivityUploadWorkflow() == nil {
		t.Fatalf("client=%#v", client)
	}
	if _, ok := client.Publisher(); ok {
		t.Fatal("publisher must not be exposed")
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
	if _, ok := client.WebhookHandler(); !ok {
		t.Fatal("webhook handler must be exposed")
	}
	if err := client.Close(); err != nil || adapter.Close() != nil {
		t.Fatalf("close client=%v adapter=%v", err, adapter.Close())
	}
	if _, err := adapter.Client(context.Background(), "athlete"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close error=%v", err)
	}
}

func TestAdapterValidationAndClientFailures(t *testing.T) {
	valid := socialhub.AccountConfig{
		ID: "athlete", AccessTokenRef: "test://token", Settings: map[string]any{"athlete_id": testAthleteID},
	}
	tests := []struct {
		name   string
		config socialhub.AdapterConfig
	}{
		{"wrong adapter", socialhub.AdapterConfig{Adapter: "other", Accounts: []socialhub.AccountConfig{valid}}},
		{"bad endpoint", socialhub.AdapterConfig{Adapter: adapterName, Settings: map[string]any{"base_url": "ftp://api.example"}, Accounts: []socialhub.AccountConfig{valid}}},
		{"missing token", socialhub.AdapterConfig{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "athlete", Settings: valid.Settings}}}},
		{"bad athlete", socialhub.AdapterConfig{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "athlete", AccessTokenRef: "test://token", Settings: map[string]any{"athlete_id": "1.5"}}}}},
		{"unknown setting", socialhub.AdapterConfig{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "athlete", AccessTokenRef: "test://token", Settings: map[string]any{"athlete_id": testAthleteID, "unknown": true}}}}},
		{"subscription only", socialhub.AdapterConfig{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "athlete", AccessTokenRef: "test://token", Settings: map[string]any{"athlete_id": testAthleteID, "subscription_id": 1}}}}},
		{"token only", socialhub.AdapterConfig{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "athlete", AccessTokenRef: "test://token", Settings: valid.Settings, Webhook: socialhub.WebhookConfig{TokenRef: "test://verify"}}}}},
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
	adapter, client := newTestAdapter(t, server, false, []string{"read", "activity:read"})
	if _, ok := client.WebhookHandler(); ok {
		t.Fatal("webhook handler must not be exposed without config")
	}
	capabilities, _ := client.Capabilities(context.Background())
	if capabilities.Has(CapabilityActivityWorkflow) || capabilities[CapabilityActivityWorkflow].Approval != socialhub.ApprovalRequired {
		t.Fatalf("write capability=%#v", capabilities[CapabilityActivityWorkflow])
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing client error=%v", err)
	}
	adapter.options.Secrets = mapResolver{"test://client-secret": "client-secret"}
	if _, err := adapter.Client(context.Background(), "athlete"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("missing access token error=%v", err)
	}
	adapter.config.Accounts[0].ClientID = "not-decimal"
	if _, err := adapter.OAuth(context.Background(), "athlete"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("OAuth validation error=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing OAuth account error=%v", err)
	}
}

func activityJSON(id, athleteID string) string {
	return `{"id":` + id + `,"athlete":{"id":` + athleteID + `},"upload_id":456789012345678,"external_id":"device-1","name":"Morning Ride","description":"Steady effort","sport_type":"Ride","distance":24931.4,"moving_time":4500,"elapsed_time":4600,"total_elevation_gain":123.4,"kudos_count":3,"comment_count":2,"start_date":"2026-08-02T12:15:09Z","start_date_local":"2026-08-02T20:15:09+08:00","timezone":"(GMT+08:00) Asia/Shanghai","private":false,"trainer":false,"commute":true,"manual":true,"hide_from_home":false,"device_name":"Garmin Edge","gear_id":"b1"}`
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

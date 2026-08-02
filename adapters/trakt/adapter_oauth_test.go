package trakt

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	testClientID     = "test-client-id"
	testClientSecret = "test-client-secret"
	testAccessToken  = "test-access-token"
)

var testNow = time.Date(2026, time.August, 2, 8, 9, 10, 0, time.UTC)

type mapResolver map[string]string

func (resolver mapResolver) Resolve(_ context.Context, reference string) (string, error) {
	value, ok := resolver[reference]
	if !ok {
		return "", errors.New("missing test secret")
	}
	return value, nil
}

type fixedClock struct{ now time.Time }

func (clock fixedClock) Now() time.Time { return clock.now }

func testConfig(server *httptest.Server, withSecret, withToken bool) socialhub.AdapterConfig {
	account := socialhub.AccountConfig{
		ID: "viewer", ClientID: testClientID, Settings: map[string]any{"username": "test-user"},
	}
	if withSecret {
		account.SecretRef = "test://secret"
	}
	if withToken {
		account.AccessTokenRef = "test://token"
	}
	return socialhub.AdapterConfig{
		Adapter:  adapterName,
		Settings: map[string]any{"base_url": server.URL, "auth_url": server.URL, "user_agent": "social-hub-tests/1.0"},
		Accounts: []socialhub.AccountConfig{account},
	}
}

func newTestClient(t *testing.T, server *httptest.Server, withSecret, withToken bool) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server, withSecret, withToken),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{"test://secret": testClientSecret, "test://token": testAccessToken}),
		socialhub.WithClock(fixedClock{now: testNow}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "viewer")
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
	adapter, client := newTestClient(t, server, true, true)
	metadata := adapter.Metadata()
	if adapter.Name() != adapterName || metadata.Product != productName || metadata.APIVersion != apiVersion || metadata.DocURL != documentationURL {
		t.Fatalf("metadata=%#v", metadata)
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range []socialhub.Capability{CapabilityAuth, CapabilityCatalog, CapabilityProfile, CapabilitySync, CapabilityScrobble, CapabilityComments} {
		if !capabilities.Has(capability) {
			t.Fatalf("capability %s=%#v", capability, capabilities[capability])
		}
	}
	for _, capability := range []socialhub.Capability{socialhub.CapPublish, socialhub.CapFetch, socialhub.CapMedia, socialhub.CapReact, socialhub.CapMessage, socialhub.CapWebhook} {
		if capabilities.Has(capability) {
			t.Fatalf("common capability %s must be unsupported", capability)
		}
	}
	if client.Platform() != "trakt" || client.Account() != "viewer" || client.Close() != nil ||
		client.OAuthWorkflow() == nil || client.CatalogWorkflow() == nil || client.UserWorkflow() == nil ||
		client.SyncWorkflow() == nil || client.ScrobbleWorkflow() == nil || client.CommentWorkflow() == nil {
		t.Fatalf("client=%#v", client)
	}
	if _, ok := client.Publisher(); ok {
		t.Fatal("publisher must not be exposed")
	}
	if _, ok := client.Fetcher(); ok {
		t.Fatal("fetcher must not be exposed")
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
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "viewer"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close=%v", err)
	}
	if err := adapter.Init(context.Background(), testConfig(server, true, true)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("reinit=%v", err)
	}
}

func TestAdapterValidationAndCredentialGates(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	valid := testConfig(server, false, false)
	tests := []struct {
		name   string
		mutate func(*socialhub.AdapterConfig)
	}{
		{"adapter", func(config *socialhub.AdapterConfig) { config.Adapter = "other" }},
		{"base URL", func(config *socialhub.AdapterConfig) { config.Settings["base_url"] = "ftp://example.test" }},
		{"auth URL", func(config *socialhub.AdapterConfig) { config.Settings["auth_url"] = "https://user:pass@example.test" }},
		{"user agent", func(config *socialhub.AdapterConfig) { config.Settings["user_agent"] = "bad\nagent" }},
		{"client ID", func(config *socialhub.AdapterConfig) { config.Accounts[0].ClientID = "" }},
		{"username", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["username"] = "bad/name" }},
		{"adapter setting", func(config *socialhub.AdapterConfig) { config.Settings["other"] = true }},
		{"account setting", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["other"] = true }},
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
	adapter, public := newTestClient(t, server, false, false)
	capabilities, _ := public.Capabilities(context.Background())
	for _, capability := range []socialhub.Capability{CapabilityAuth, CapabilitySync, CapabilityScrobble} {
		if capabilities.Has(capability) || capabilities[capability].Approval != socialhub.ApprovalRequired {
			t.Fatalf("gated capability %s=%#v", capability, capabilities[capability])
		}
	}
	if _, err := public.Exchange(context.Background(), "code", "https://app.example/callback"); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("secret gate=%v", err)
	}
	if _, err := public.GetSettings(context.Background()); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("OAuth gate=%v", err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing account=%v", err)
	}

	broken := &Adapter{}
	config := testConfig(server, true, true)
	if err := broken.Init(context.Background(), config, socialhub.WithHTTPClient(server.Client()), socialhub.WithSecretResolver(mapResolver{})); err != nil {
		t.Fatal(err)
	}
	if _, err := broken.Client(context.Background(), "viewer"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("unresolved credential=%v", err)
	}
}

func TestOAuthBrowserDeviceRefreshAndRevoke(t *testing.T) {
	polls := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("client_secret") != "" || request.Header.Get("trakt-api-key") != testClientID ||
			request.Header.Get("trakt-api-version") != apiVersion || request.Header.Get("User-Agent") != "social-hub-tests/1.0" {
			http.Error(writer, "bad OAuth headers", http.StatusBadRequest)
			return
		}
		body, _ := io.ReadAll(request.Body)
		var payload map[string]any
		_ = json.Unmarshal(body, &payload)
		switch request.URL.Path {
		case "/oauth/token":
			if payload["client_secret"] != testClientSecret {
				http.Error(writer, "missing secret", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"access_token":"new-access","token_type":"Bearer","expires_in":604800,"refresh_token":"new-refresh","scope":"public","created_at":1785658150}`)
		case "/oauth/device/code":
			writeJSON(writer, http.StatusOK, `{"device_code":"device-code","user_code":"ABCD1234","verification_url":"https://auth.trakt.tv/activate","expires_in":600,"interval":5}`)
		case "/oauth/device/token":
			polls++
			if polls == 1 {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"access_token":"device-access","token_type":"Bearer","expires_in":604800,"refresh_token":"device-refresh","scope":"public","created_at":1785658150}`)
		case "/oauth/revoke":
			if payload["token"] != "new-access" {
				http.Error(writer, "bad token", http.StatusBadRequest)
				return
			}
			writer.WriteHeader(http.StatusOK)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, true, false)
	authorizationURL, err := client.AuthorizationURL(AuthorizationRequest{
		RedirectURI: "https://app.example/callback", State: "state-value", Signup: true, ForceLogin: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(authorizationURL)
	if parsed.Path != "/oauth/authorize" || parsed.Query().Get("response_type") != "code" || parsed.Query().Get("signup") != "true" || parsed.Query().Get("prompt") != "login" {
		t.Fatalf("authorization URL=%s", authorizationURL)
	}
	token, err := client.Exchange(context.Background(), "auth-code", "https://app.example/callback")
	if err != nil || token.AccessToken != "new-access" || token.RefreshToken != "new-refresh" || token.ExpiresAt.IsZero() {
		t.Fatalf("exchange token=%#v err=%v", token, err)
	}
	refreshed, err := client.Refresh(context.Background(), "old-refresh", "https://app.example/callback")
	if err != nil || refreshed.RefreshToken != "new-refresh" {
		t.Fatalf("refresh token=%#v err=%v", refreshed, err)
	}
	device, err := client.RequestDeviceCode(context.Background())
	if err != nil || device.DeviceCode != "device-code" || device.Interval != 5*time.Second || !device.ExpiresAt.Equal(testNow.Add(10*time.Minute)) {
		t.Fatalf("device=%#v err=%v", device, err)
	}
	if _, err := client.PollDevice(context.Background(), *device); !errors.Is(err, socialhub.ErrUnavailable) {
		t.Fatalf("pending poll=%v", err)
	} else {
		var platformErr *socialhub.Error
		if !errors.As(err, &platformErr) || platformErr.RetryAfter != 5*time.Second || platformErr.PlatformCode != "authorization_pending" {
			t.Fatalf("pending platform error=%#v", platformErr)
		}
	}
	deviceToken, err := client.PollDevice(context.Background(), *device)
	if err != nil || deviceToken.AccessToken != "device-access" {
		t.Fatalf("device token=%#v err=%v", deviceToken, err)
	}
	if err := client.Revoke(context.Background(), "new-access"); err != nil {
		t.Fatal(err)
	}
}

func TestOAuthValidationAndExpiredDevice(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, true, false)
	tests := []struct {
		name string
		call func() error
	}{
		{"authorize", func() error { _, err := client.AuthorizationURL(AuthorizationRequest{}); return err }},
		{"exchange", func() error { _, err := client.Exchange(context.Background(), "", "bad"); return err }},
		{"refresh", func() error { _, err := client.Refresh(context.Background(), "", "bad"); return err }},
		{"poll", func() error { _, err := client.PollDevice(context.Background(), DeviceAuthorization{}); return err }},
		{"revoke", func() error { return client.Revoke(context.Background(), "") }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
				t.Fatalf("error=%v", err)
			}
		})
	}
	_, err := client.PollDevice(context.Background(), DeviceAuthorization{
		DeviceCode: "device", ExpiresAt: testNow.Add(-time.Second), Interval: time.Second,
	})
	if !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("expired device=%v", err)
	}
}

func cloneConfig(input socialhub.AdapterConfig) socialhub.AdapterConfig {
	output := input
	output.Settings = cloneMap(input.Settings)
	output.Accounts = append([]socialhub.AccountConfig(nil), input.Accounts...)
	output.Accounts[0].Settings = cloneMap(input.Accounts[0].Settings)
	return output
}

func cloneMap(input map[string]any) map[string]any {
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func writeJSON(writer http.ResponseWriter, status int, body string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_, _ = writer.Write([]byte(body))
}

func requestBody(request *http.Request) string {
	body, _ := io.ReadAll(request.Body)
	return strings.TrimSpace(string(body))
}

package letterboxd

import (
	"bytes"
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
	testClientID     = "letterboxd-client"
	testClientSecret = "letterboxd-secret"
	testAccessToken  = "letterboxd-access"
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

func testConfig(server *httptest.Server, kind TokenKind, withToken bool, scopes []string) socialhub.AdapterConfig {
	account := socialhub.AccountConfig{
		ID: "cinephile", ClientID: testClientID, SecretRef: "test://client-secret",
		Approval: socialhub.ApprovalConfig{Scopes: scopes}, Settings: map[string]any{"token_kind": kind},
	}
	if withToken {
		account.AccessTokenRef = "test://access-token"
	}
	return socialhub.AdapterConfig{
		Adapter: adapterName,
		Settings: map[string]any{
			"base_url": server.URL, "auth_url": server.URL + "/authorize", "token_url": server.URL + "/token",
			"revoke_url": server.URL + "/revoke", "user_agent": "social-hub-letterboxd-tests/1.0",
		},
		Accounts: []socialhub.AccountConfig{account},
	}
}

func newTestClient(t *testing.T, server *httptest.Server, kind TokenKind, withToken bool, scopes []string) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server, kind, withToken, scopes),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{
			"test://client-secret": testClientSecret, "test://access-token": testAccessToken,
		}),
		socialhub.WithClock(fixedClock{now: testNow}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "cinephile")
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

	adapter, client := newTestClient(t, server, TokenUser, true, []string{"content:modify", "oauth:refresh"})
	metadata := adapter.Metadata()
	if adapter.Name() != adapterName || metadata.Product != productName || metadata.APIVersion != apiVersion ||
		metadata.DocURL != documentationURL || metadata.VerifiedAt.IsZero() {
		t.Fatalf("metadata=%#v", metadata)
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range []socialhub.Capability{CapabilityOAuth, CapabilityCatalog, CapabilityMembers, CapabilityLogEntries, CapabilityRelationships} {
		if !capabilities.Has(capability) {
			t.Fatalf("capability %s=%#v", capability, capabilities[capability])
		}
	}
	for _, capability := range []socialhub.Capability{socialhub.CapPublish, socialhub.CapFetch, socialhub.CapMedia, socialhub.CapReact, socialhub.CapMessage, socialhub.CapWebhook} {
		if capabilities.Has(capability) {
			t.Fatalf("common capability %s must be unsupported", capability)
		}
	}
	if client.Platform() != "letterboxd" || client.Account() != "cinephile" || client.Close() != nil ||
		client.OAuthWorkflow() == nil || client.CatalogWorkflow() == nil || client.MemberWorkflow() == nil ||
		client.LogEntryWorkflow() == nil || client.RelationshipWorkflow() == nil {
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
	if _, err := adapter.Client(context.Background(), "cinephile"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close=%v", err)
	}
	if err := adapter.Init(context.Background(), testConfig(server, TokenUser, true, nil)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("reinit=%v", err)
	}
}

func TestCapabilityTokenGates(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, none := newTestClient(t, server, TokenClient, false, nil)
	capabilities, _ := none.Capabilities(context.Background())
	if capabilities[CapabilityCatalog].Approval != socialhub.ApprovalRequired || capabilities.Has(CapabilityCatalog) {
		t.Fatalf("no-token capabilities=%#v", capabilities)
	}
	if _, err := none.Search(context.Background(), SearchRequest{Input: "Alien"}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("no-token gate=%v", err)
	}

	_, app := newTestClient(t, server, TokenClient, true, nil)
	capabilities, _ = app.Capabilities(context.Background())
	if !capabilities.Has(CapabilityCatalog) || capabilities.Has(CapabilityRelationships) {
		t.Fatalf("client-token capabilities=%#v", capabilities)
	}
	if _, err := app.GetMe(context.Background()); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("client-token user gate=%v", err)
	}
	if err := app.SetLike(context.Background(), "film-1", true); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("client-token write gate=%v", err)
	}

	_, missingScope := newTestClient(t, server, TokenUser, true, []string{"profile"})
	if err := missingScope.SetWatchlist(context.Background(), "film-1", true); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("scope gate=%v", err)
	}
}

func TestAdapterValidationAndCredentials(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	valid := testConfig(server, TokenUser, true, []string{"content:modify"})
	tests := []struct {
		name   string
		mutate func(*socialhub.AdapterConfig)
	}{
		{"adapter", func(c *socialhub.AdapterConfig) { c.Adapter = "other" }},
		{"base URL", func(c *socialhub.AdapterConfig) { c.Settings["base_url"] = "ftp://example.test" }},
		{"auth URL", func(c *socialhub.AdapterConfig) { c.Settings["auth_url"] = "https://u:p@example.test" }},
		{"user agent", func(c *socialhub.AdapterConfig) { c.Settings["user_agent"] = "bad\nagent" }},
		{"client ID", func(c *socialhub.AdapterConfig) { c.Accounts[0].ClientID = "" }},
		{"secret ref", func(c *socialhub.AdapterConfig) { c.Accounts[0].SecretRef = "" }},
		{"access ref", func(c *socialhub.AdapterConfig) { c.Accounts[0].AccessTokenRef = "bad\nref" }},
		{"token kind", func(c *socialhub.AdapterConfig) { c.Accounts[0].Settings["token_kind"] = "bot" }},
		{"user without token", func(c *socialhub.AdapterConfig) { c.Accounts[0].AccessTokenRef = "" }},
		{"first party scope", func(c *socialhub.AdapterConfig) { c.Accounts[0].Approval.Scopes = []string{"client:firstparty"} }},
		{"duplicate scope", func(c *socialhub.AdapterConfig) { c.Accounts[0].Approval.Scopes = []string{"profile", "profile"} }},
		{"adapter setting", func(c *socialhub.AdapterConfig) { c.Settings["unknown"] = true }},
		{"account setting", func(c *socialhub.AdapterConfig) { c.Accounts[0].Settings["unknown"] = true }},
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
	if _, err := adapter.Client(context.Background(), "cinephile"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("unresolved secret=%v", err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing account=%v", err)
	}
	if _, err := (&Adapter{}).Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("uninitialized adapter=%v", err)
	}
}

func TestOAuthFlows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Accept") != "application/json" || request.Header.Get("Content-Type") != "application/x-www-form-urlencoded" ||
			request.Header.Get("User-Agent") != "social-hub-letterboxd-tests/1.0" {
			http.Error(writer, "bad headers", http.StatusBadRequest)
			return
		}
		if err := request.ParseForm(); err != nil || request.Form.Get("client_id") != testClientID || request.Form.Get("client_secret") != testClientSecret {
			http.Error(writer, "bad credentials", http.StatusBadRequest)
			return
		}
		switch request.URL.Path {
		case "/token":
			switch request.Form.Get("grant_type") {
			case "client_credentials":
				if request.Form.Get("scope") != "profile email" {
					http.Error(writer, "bad scope", http.StatusBadRequest)
					return
				}
				writeJSON(writer, http.StatusOK, `{"access_token":"app-token","token_type":"Bearer","expires_in":3600,"scope":"profile email"}`)
			case "authorization_code":
				if request.Form.Get("code") != "code-1" || request.Form.Get("redirect_uri") != "https://app.example/callback" {
					http.Error(writer, "bad exchange", http.StatusBadRequest)
					return
				}
				writeJSON(writer, http.StatusOK, `{"access_token":"user-token","refresh_token":"refresh-1","token_type":"bearer","expires_in":1800,"scope":"content:modify oauth:refresh"}`)
			case "refresh_token":
				if request.Form.Get("refresh_token") != "refresh-1" {
					http.Error(writer, "bad refresh", http.StatusBadRequest)
					return
				}
				writeJSON(writer, http.StatusOK, `{"access_token":"user-token-2","token_type":"Bearer","expires_in":900,"scope":"content:modify"}`)
			default:
				http.Error(writer, "bad grant", http.StatusBadRequest)
			}
		case "/revoke":
			if request.Form.Get("token") != "refresh-1" || request.Form.Get("token_type_hint") != "refresh_token" {
				http.Error(writer, "bad revoke", http.StatusBadRequest)
				return
			}
			writer.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, TokenClient, false, nil)

	authorize, err := client.AuthorizationURL(AuthorizationRequest{
		RedirectURI: "https://app.example/callback", State: "state-1", Scopes: []string{"openid", "profile"},
	})
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(authorize)
	if parsed.Path != "/authorize" || parsed.Query().Get("client_id") != testClientID || parsed.Query().Get("response_type") != "code" ||
		parsed.Query().Get("redirect_uri") != "https://app.example/callback" || parsed.Query().Get("state") != "state-1" || parsed.Query().Get("scope") != "openid profile" {
		t.Fatalf("authorization URL=%s", authorize)
	}
	app, err := client.ClientCredentials(context.Background(), []string{"profile", "email"})
	if err != nil || app.AccessToken != "app-token" || !app.ExpiresAt.Equal(testNow.Add(time.Hour)) || len(app.Scopes) != 2 {
		t.Fatalf("app token=%#v err=%v", app, err)
	}
	user, err := client.Exchange(context.Background(), "code-1", "https://app.example/callback")
	if err != nil || user.RefreshToken != "refresh-1" || !user.ExpiresAt.Equal(testNow.Add(30*time.Minute)) {
		t.Fatalf("user token=%#v err=%v", user, err)
	}
	refreshed, err := client.Refresh(context.Background(), user.RefreshToken)
	if err != nil || refreshed.AccessToken != "user-token-2" || refreshed.RefreshToken != "refresh-1" || !refreshed.ExpiresAt.Equal(testNow.Add(15*time.Minute)) {
		t.Fatalf("refreshed token=%#v err=%v", refreshed, err)
	}
	if err := client.Revoke(context.Background(), refreshed.RefreshToken, "refresh_token"); err != nil {
		t.Fatal(err)
	}
}

func TestOAuthValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, TokenClient, false, nil)
	tests := []func() error{
		func() error { _, err := client.AuthorizationURL(AuthorizationRequest{}); return err },
		func() error {
			_, err := client.AuthorizationURL(AuthorizationRequest{RedirectURI: "https://app.example/cb", State: "state", Scopes: []string{"client:firstparty"}})
			return err
		},
		func() error {
			_, err := client.ClientCredentials(context.Background(), []string{"profile", "profile"})
			return err
		},
		func() error {
			_, err := client.Exchange(context.Background(), "", "https://app.example/cb")
			return err
		},
		func() error { _, err := client.Refresh(context.Background(), ""); return err },
		func() error { return client.Revoke(context.Background(), "token", "bad") },
	}
	for _, call := range tests {
		if err := call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("error=%v", err)
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

func writeJSON(writer http.ResponseWriter, status int, body string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_, _ = writer.Write([]byte(body))
}

func requestBody(request *http.Request) string {
	body, _ := io.ReadAll(request.Body)
	var compact bytes.Buffer
	if json.Compact(&compact, body) == nil {
		return compact.String()
	}
	return strings.TrimSpace(string(body))
}

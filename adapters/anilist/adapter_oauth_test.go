package anilist

import (
	"context"
	"encoding/json"
	"errors"
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
	testClientID     = "12345"
	testClientSecret = "anilist-client-secret"
	testAccessToken  = "anilist-access-token"
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

func testConfig(server *httptest.Server, withApp, withSecret, withToken bool) socialhub.AdapterConfig {
	account := socialhub.AccountConfig{ID: "anime-fan"}
	if withApp {
		account.ClientID = testClientID
	}
	if withSecret {
		account.SecretRef = "test://client-secret"
	}
	if withToken {
		account.AccessTokenRef = "test://access-token"
	}
	return socialhub.AdapterConfig{
		Adapter: adapterName,
		Settings: map[string]any{
			"graphql_url": server.URL + "/graphql", "auth_url": server.URL + "/authorize",
			"token_url": server.URL + "/token", "user_agent": "social-hub-anilist-tests/1.0",
		},
		Accounts: []socialhub.AccountConfig{account},
	}
}

func newTestClient(t *testing.T, server *httptest.Server, withApp, withSecret, withToken bool) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server, withApp, withSecret, withToken),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{
			"test://client-secret": testClientSecret, "test://access-token": testAccessToken,
		}),
		socialhub.WithClock(fixedClock{now: testNow}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "anime-fan")
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
	adapter, client := newTestClient(t, server, true, true, true)
	metadata := adapter.Metadata()
	if adapter.Name() != adapterName || metadata.Product != productName || metadata.APIVersion != apiVersion ||
		metadata.DocURL != documentationURL || metadata.VerifiedAt.IsZero() {
		t.Fatalf("metadata=%#v", metadata)
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range []socialhub.Capability{
		CapabilityOAuth, CapabilityMedia, CapabilityUser, CapabilityMediaList, CapabilityActivity,
	} {
		if !capabilities.Has(capability) {
			t.Fatalf("capability %s=%#v", capability, capabilities[capability])
		}
	}
	for _, capability := range []socialhub.Capability{
		socialhub.CapPublish, socialhub.CapFetch, socialhub.CapMedia,
		socialhub.CapReact, socialhub.CapMessage, socialhub.CapWebhook,
	} {
		if capabilities.Has(capability) {
			t.Fatalf("common capability %s must be unsupported", capability)
		}
	}
	if client.Platform() != "anilist" || client.Account() != "anime-fan" || client.Close() != nil ||
		client.OAuthWorkflow() == nil || client.MediaWorkflow() == nil || client.UserWorkflow() == nil ||
		client.MediaListWorkflow() == nil || client.ActivityWorkflow() == nil {
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
	if _, err := adapter.Client(context.Background(), "anime-fan"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close=%v", err)
	}
	if err := adapter.Init(context.Background(), testConfig(server, true, true, true)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("reinit=%v", err)
	}
}

func TestPublicCapabilitiesAndUserGates(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, false, false, false)
	capabilities, _ := client.Capabilities(context.Background())
	if capabilities.Has(CapabilityOAuth) || !capabilities.Has(CapabilityMedia) ||
		!capabilities.Has(CapabilityMediaList) || !capabilities.Has(CapabilityActivity) {
		t.Fatalf("capabilities=%#v", capabilities)
	}
	if _, err := client.GetViewer(context.Background()); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("viewer gate=%v", err)
	}
	if _, err := client.SaveMediaListEntry(context.Background(), SaveMediaListEntryRequest{MediaID: 1}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("list gate=%v", err)
	}
	if _, err := client.SaveTextActivity(context.Background(), SaveTextActivityRequest{Text: "hello"}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("activity gate=%v", err)
	}
	if _, err := client.ListActivities(context.Background(), ListActivitiesRequest{Following: true}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("following gate=%v", err)
	}
}

func TestAdapterValidationAndCredentials(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	valid := testConfig(server, true, true, true)
	tests := []struct {
		name   string
		mutate func(*socialhub.AdapterConfig)
	}{
		{"adapter", func(c *socialhub.AdapterConfig) { c.Adapter = "other" }},
		{"GraphQL URL", func(c *socialhub.AdapterConfig) { c.Settings["graphql_url"] = "ftp://example.test" }},
		{"auth URL", func(c *socialhub.AdapterConfig) { c.Settings["auth_url"] = "https://u:p@example.test" }},
		{"token URL", func(c *socialhub.AdapterConfig) { c.Settings["token_url"] = "not-a-url" }},
		{"user agent", func(c *socialhub.AdapterConfig) { c.Settings["user_agent"] = "bad\nagent" }},
		{"client ID", func(c *socialhub.AdapterConfig) { c.Accounts[0].ClientID = " bad" }},
		{"secret without client", func(c *socialhub.AdapterConfig) { c.Accounts[0].ClientID = "" }},
		{"secret ref", func(c *socialhub.AdapterConfig) { c.Accounts[0].SecretRef = "bad\nref" }},
		{"access ref", func(c *socialhub.AdapterConfig) { c.Accounts[0].AccessTokenRef = "bad\nref" }},
		{"scope", func(c *socialhub.AdapterConfig) { c.Accounts[0].Approval.Scopes = []string{"read"} }},
		{"adapter setting", func(c *socialhub.AdapterConfig) { c.Settings["unknown"] = true }},
		{"account setting", func(c *socialhub.AdapterConfig) { c.Accounts[0].Settings = map[string]any{"unknown": true} }},
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

	for _, mutate := range []func(*socialhub.AdapterConfig){
		func(c *socialhub.AdapterConfig) { c.Accounts[0].SecretRef = "test://missing" },
		func(c *socialhub.AdapterConfig) { c.Accounts[0].AccessTokenRef = "test://missing" },
	} {
		config := cloneConfig(valid)
		mutate(&config)
		adapter := &Adapter{}
		if err := adapter.Init(context.Background(), config, socialhub.WithHTTPClient(server.Client()), socialhub.WithSecretResolver(mapResolver{})); err != nil {
			t.Fatal(err)
		}
		if _, err := adapter.Client(context.Background(), "anime-fan"); !errors.Is(err, socialhub.ErrUnauthenticated) {
			t.Fatalf("unresolved credential=%v", err)
		}
	}
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), valid); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing account=%v", err)
	}
	if _, err := (&Adapter{}).Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("uninitialized adapter=%v", err)
	}
}

func TestGraphQLAuthenticationModes(t *testing.T) {
	var publicSeen, userSeen bool
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body := readGraphQLRequest(t, writer, request)
		name, _ := body.Variables["name"].(string)
		switch name {
		case "public":
			publicSeen = request.Header.Get("Authorization") == ""
		case "user":
			userSeen = request.Header.Get("Authorization") == "Bearer "+testAccessToken
		}
		writeJSON(writer, http.StatusOK, `{"data":{"User":{"id":1,"name":"`+name+`"}}}`)
	}))
	defer server.Close()
	_, public := newTestClient(t, server, false, false, false)
	_, user := newTestClient(t, server, false, false, true)
	if _, err := public.GetUser(context.Background(), UserLookup{Name: "public"}); err != nil {
		t.Fatal(err)
	}
	if _, err := user.GetUser(context.Background(), UserLookup{Name: "user"}); err != nil {
		t.Fatal(err)
	}
	if !publicSeen || !userSeen {
		t.Fatalf("public=%v user=%v", publicSeen, userSeen)
	}
}

func TestOAuthFlows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/token" || request.Header.Get("Accept") != "application/json" ||
			request.Header.Get("Content-Type") != "application/json" ||
			request.Header.Get("User-Agent") != "social-hub-anilist-tests/1.0" || request.Header.Get("Authorization") != "" {
			http.Error(writer, "bad request", http.StatusBadRequest)
			return
		}
		var body map[string]string
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil || body["grant_type"] != "authorization_code" ||
			body["client_id"] != testClientID || body["client_secret"] != testClientSecret ||
			body["redirect_uri"] != "myapp://oauth/callback" || body["code"] != "code-1" {
			http.Error(writer, "bad token body", http.StatusBadRequest)
			return
		}
		writeJSON(writer, http.StatusOK, `{"access_token":"user-token","token_type":"Bearer","expires_in":31536000}`)
	}))
	defer server.Close()
	_, client := newTestClient(t, server, true, true, false)

	authorize, err := client.AuthorizationURL(AuthorizationRequest{RedirectURI: "myapp://oauth/callback", State: "state-1"})
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(authorize)
	if parsed.Path != "/authorize" || parsed.Query().Get("client_id") != testClientID ||
		parsed.Query().Get("redirect_uri") != "myapp://oauth/callback" || parsed.Query().Get("response_type") != "code" ||
		parsed.Query().Get("state") != "state-1" || parsed.Query().Get("scope") != "" {
		t.Fatalf("authorization URL=%s", authorize)
	}
	implicit, err := client.ImplicitAuthorizationURL(ImplicitAuthorizationRequest{State: "state-2"})
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ = url.Parse(implicit)
	if parsed.Query().Get("client_id") != testClientID || parsed.Query().Get("response_type") != "token" ||
		parsed.Query().Get("state") != "state-2" || parsed.Query().Get("redirect_uri") != "" {
		t.Fatalf("implicit URL=%s", implicit)
	}
	token, err := client.Exchange(context.Background(), "code-1", "myapp://oauth/callback")
	if err != nil || token.AccessToken != "user-token" || token.RefreshToken != "" || token.TokenType != "Bearer" ||
		!token.ExpiresAt.Equal(testNow.Add(365*24*time.Hour)) {
		t.Fatalf("token=%#v err=%v", token, err)
	}
}

func TestOAuthValidationAndApproval(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, public := newTestClient(t, server, false, false, false)
	if _, err := public.AuthorizationURL(AuthorizationRequest{RedirectURI: "https://app.example/cb", State: "state"}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("app gate=%v", err)
	}
	_, implicit := newTestClient(t, server, true, false, false)
	if _, err := implicit.ImplicitAuthorizationURL(ImplicitAuthorizationRequest{State: "state"}); err != nil {
		t.Fatal(err)
	}
	if _, err := implicit.Exchange(context.Background(), "code", "https://app.example/cb"); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("secret gate=%v", err)
	}
	_, client := newTestClient(t, server, true, true, false)
	tests := []func() error{
		func() error { _, err := client.AuthorizationURL(AuthorizationRequest{}); return err },
		func() error {
			_, err := client.AuthorizationURL(AuthorizationRequest{RedirectURI: "javascript:alert(1)", State: "state"})
			return err
		},
		func() error { _, err := client.ImplicitAuthorizationURL(ImplicitAuthorizationRequest{}); return err },
		func() error {
			_, err := client.Exchange(context.Background(), "", "https://app.example/cb")
			return err
		},
		func() error {
			_, err := client.Exchange(context.Background(), "code", "data:text/plain,bad")
			return err
		},
	}
	for _, call := range tests {
		if err := call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("error=%v", err)
		}
	}
}

func cloneConfig(input socialhub.AdapterConfig) socialhub.AdapterConfig {
	output := input
	output.Settings = make(map[string]any, len(input.Settings))
	for key, value := range input.Settings {
		output.Settings[key] = value
	}
	output.Accounts = append([]socialhub.AccountConfig(nil), input.Accounts...)
	for index := range output.Accounts {
		output.Accounts[index].Approval.Scopes = append([]string(nil), input.Accounts[index].Approval.Scopes...)
		if input.Accounts[index].Settings != nil {
			output.Accounts[index].Settings = make(map[string]any, len(input.Accounts[index].Settings))
			for key, value := range input.Accounts[index].Settings {
				output.Accounts[index].Settings[key] = value
			}
		}
	}
	return output
}

func writeJSON(writer http.ResponseWriter, status int, body string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_, _ = writer.Write([]byte(body))
}

type capturedGraphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

func readGraphQLRequest(t *testing.T, writer http.ResponseWriter, request *http.Request) capturedGraphQLRequest {
	t.Helper()
	if request.Method != http.MethodPost || request.URL.Path != "/graphql/" || request.Header.Get("Content-Type") != "application/json" ||
		request.Header.Get("User-Agent") != "social-hub-anilist-tests/1.0" {
		http.Error(writer, "bad GraphQL request", http.StatusBadRequest)
		t.Errorf("method=%s path=%s content-type=%s user-agent=%s", request.Method, request.URL.Path, request.Header.Get("Content-Type"), request.Header.Get("User-Agent"))
		return capturedGraphQLRequest{}
	}
	var body capturedGraphQLRequest
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		http.Error(writer, "bad JSON", http.StatusBadRequest)
		t.Error(err)
	}
	return body
}

func operationIs(body capturedGraphQLRequest, name string) bool {
	return strings.Contains(body.Query, "query "+name+"(") || strings.Contains(body.Query, "query "+name+" {") ||
		strings.Contains(body.Query, "mutation "+name+"(")
}

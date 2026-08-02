package myanimelist

import (
	"context"
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
	testClientID     = "12345678901234567890123456789012"
	testClientSecret = "myanimelist-client-secret"
	testAccessToken  = "myanimelist-access-token"
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

func testConfig(server *httptest.Server, withSecret, withToken bool, scopes []string) socialhub.AdapterConfig {
	account := socialhub.AccountConfig{
		ID: "anime-fan", ClientID: testClientID,
		Approval: socialhub.ApprovalConfig{Scopes: scopes},
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
			"base_url": server.URL, "auth_url": server.URL + "/authorize",
			"token_url": server.URL + "/token", "user_agent": "social-hub-myanimelist-tests/1.0",
		},
		Accounts: []socialhub.AccountConfig{account},
	}
}

func newTestClient(t *testing.T, server *httptest.Server, withSecret, withToken bool, scopes []string) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server, withSecret, withToken, scopes),
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

	adapter, client := newTestClient(t, server, true, true, []string{scopeWriteUsers})
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
		CapabilityOAuth, CapabilityAnime, CapabilityManga, CapabilityUser, CapabilityAnimeList, CapabilityMangaList,
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
	if client.Platform() != "myanimelist" || client.Account() != "anime-fan" || client.Close() != nil ||
		client.OAuthWorkflow() == nil || client.AnimeWorkflow() == nil || client.MangaWorkflow() == nil ||
		client.UserWorkflow() == nil || client.AnimeListWorkflow() == nil || client.MangaListWorkflow() == nil {
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
	if err := adapter.Init(context.Background(), testConfig(server, true, true, nil)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("reinit=%v", err)
	}
}

func TestPublicCapabilitiesAndUserGates(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, false, false, nil)
	capabilities, _ := client.Capabilities(context.Background())
	if capabilities.Has(CapabilityUser) || !capabilities.Has(CapabilityAnime) || !capabilities.Has(CapabilityAnimeList) {
		t.Fatalf("capabilities=%#v", capabilities)
	}
	if _, err := client.GetMe(context.Background()); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("get me gate=%v", err)
	}
	if _, err := client.ListAnimeSuggestions(context.Background(), PageRequest{}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("suggestions gate=%v", err)
	}
	if _, err := client.ListAnimeList(context.Background(), AnimeListRequest{Username: "@me"}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("@me list gate=%v", err)
	}
	if _, err := client.UpdateAnimeListStatus(context.Background(), UpdateAnimeListStatusRequest{AnimeID: 1}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("write gate=%v", err)
	}
}

func TestAdapterValidationAndCredentials(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	valid := testConfig(server, true, true, []string{scopeWriteUsers})
	tests := []struct {
		name   string
		mutate func(*socialhub.AdapterConfig)
	}{
		{"adapter", func(c *socialhub.AdapterConfig) { c.Adapter = "other" }},
		{"base URL", func(c *socialhub.AdapterConfig) { c.Settings["base_url"] = "ftp://example.test" }},
		{"auth URL", func(c *socialhub.AdapterConfig) { c.Settings["auth_url"] = "https://u:p@example.test" }},
		{"token URL", func(c *socialhub.AdapterConfig) { c.Settings["token_url"] = "not-a-url" }},
		{"user agent", func(c *socialhub.AdapterConfig) { c.Settings["user_agent"] = "bad\nagent" }},
		{"client ID", func(c *socialhub.AdapterConfig) { c.Accounts[0].ClientID = "" }},
		{"secret ref", func(c *socialhub.AdapterConfig) { c.Accounts[0].SecretRef = "bad\nref" }},
		{"access ref", func(c *socialhub.AdapterConfig) { c.Accounts[0].AccessTokenRef = "bad\nref" }},
		{"scope", func(c *socialhub.AdapterConfig) { c.Accounts[0].Approval.Scopes = []string{"forum:write"} }},
		{"duplicate scope", func(c *socialhub.AdapterConfig) {
			c.Accounts[0].Approval.Scopes = []string{scopeWriteUsers, scopeWriteUsers}
		}},
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

func TestAPIAuthenticationModes(t *testing.T) {
	var publicSeen, userSeen bool
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Query().Get("q") {
		case "public":
			publicSeen = request.Header.Get("X-MAL-CLIENT-ID") == testClientID && request.Header.Get("Authorization") == ""
		case "user":
			userSeen = request.Header.Get("Authorization") == "Bearer "+testAccessToken && request.Header.Get("X-MAL-CLIENT-ID") == ""
		}
		writeJSON(writer, http.StatusOK, `{"data":[],"paging":{}}`)
	}))
	defer server.Close()
	_, public := newTestClient(t, server, false, false, nil)
	_, user := newTestClient(t, server, false, true, nil)
	if _, err := public.SearchAnime(context.Background(), SearchRequest{Query: "public"}); err != nil {
		t.Fatal(err)
	}
	if _, err := user.SearchAnime(context.Background(), SearchRequest{Query: "user"}); err != nil {
		t.Fatal(err)
	}
	if !publicSeen || !userSeen {
		t.Fatalf("public=%v user=%v", publicSeen, userSeen)
	}
}

func TestOAuthFlows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/token" || request.Header.Get("Accept") != "application/json" ||
			request.Header.Get("Content-Type") != "application/x-www-form-urlencoded" ||
			request.Header.Get("User-Agent") != "social-hub-myanimelist-tests/1.0" || request.Header.Get("Authorization") != "" {
			http.Error(writer, "bad request", http.StatusBadRequest)
			return
		}
		if err := request.ParseForm(); err != nil || request.Form.Get("client_id") != testClientID || request.Form.Get("client_secret") != testClientSecret {
			http.Error(writer, "bad credentials", http.StatusBadRequest)
			return
		}
		switch request.Form.Get("grant_type") {
		case "authorization_code":
			if request.Form.Get("code") != "code-1" || request.Form.Get("redirect_uri") != "https://app.example/callback" ||
				request.Form.Get("code_verifier") != strings.Repeat("a", 43) {
				http.Error(writer, "bad exchange", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"access_token":"user-token","refresh_token":"refresh-1","token_type":"Bearer","expires_in":3600}`)
		case "refresh_token":
			if request.Form.Get("refresh_token") != "refresh-1" {
				http.Error(writer, "bad refresh", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"access_token":"user-token-2","token_type":"bearer","expires_in":1800}`)
		default:
			http.Error(writer, "bad grant", http.StatusBadRequest)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, true, false, nil)

	pkce := PKCE{Verifier: strings.Repeat("a", 43), Challenge: strings.Repeat("a", 43)}
	authorize, err := client.AuthorizationURL(AuthorizationRequest{RedirectURI: "https://app.example/callback", State: "state-1", PKCE: pkce})
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(authorize)
	if parsed.Path != "/authorize" || parsed.Query().Get("response_type") != "code" ||
		parsed.Query().Get("client_id") != testClientID || parsed.Query().Get("redirect_uri") != "https://app.example/callback" ||
		parsed.Query().Get("state") != "state-1" || parsed.Query().Get("code_challenge") != pkce.Challenge ||
		parsed.Query().Get("code_challenge_method") != "plain" || parsed.Query().Get("scope") != "" {
		t.Fatalf("authorization URL=%s", authorize)
	}
	token, err := client.Exchange(context.Background(), "code-1", "https://app.example/callback", pkce.Verifier)
	if err != nil || token.AccessToken != "user-token" || token.RefreshToken != "refresh-1" ||
		!token.ExpiresAt.Equal(testNow.Add(time.Hour)) {
		t.Fatalf("token=%#v err=%v", token, err)
	}
	refreshed, err := client.Refresh(context.Background(), token.RefreshToken)
	if err != nil || refreshed.AccessToken != "user-token-2" || refreshed.RefreshToken != "refresh-1" ||
		!refreshed.ExpiresAt.Equal(testNow.Add(30*time.Minute)) {
		t.Fatalf("refreshed=%#v err=%v", refreshed, err)
	}
	generated, err := NewPKCE()
	if err != nil || generated.Verifier != generated.Challenge || !validPKCEValue(generated.Verifier) {
		t.Fatalf("generated=%#v err=%v", generated, err)
	}
}

func TestOAuthPublicClientAndValidation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_ = request.ParseForm()
		if request.Form.Get("client_id") != testClientID || request.Form.Get("client_secret") != "" {
			http.Error(writer, "bad public client", http.StatusBadRequest)
			return
		}
		writeJSON(writer, http.StatusOK, `{"access_token":"public-user-token","refresh_token":"refresh","expires_in":3600}`)
	}))
	defer server.Close()
	_, client := newTestClient(t, server, false, false, nil)
	verifier := strings.Repeat("b", 43)
	if _, err := client.Exchange(context.Background(), "code", "http://127.0.0.1/callback", verifier); err != nil {
		t.Fatal(err)
	}
	tests := []func() error{
		func() error { _, err := client.AuthorizationURL(AuthorizationRequest{}); return err },
		func() error {
			_, err := client.AuthorizationURL(AuthorizationRequest{RedirectURI: "https://app.example/cb", State: "state", PKCE: PKCE{Verifier: verifier, Challenge: strings.Repeat("c", 43)}})
			return err
		},
		func() error {
			_, err := client.Exchange(context.Background(), "", "https://app.example/cb", verifier)
			return err
		},
		func() error {
			_, err := client.Exchange(context.Background(), "code", "ftp://app.example/cb", verifier)
			return err
		},
		func() error {
			_, err := client.Exchange(context.Background(), "code", "https://app.example/cb", "short")
			return err
		},
		func() error { _, err := client.Refresh(context.Background(), ""); return err },
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

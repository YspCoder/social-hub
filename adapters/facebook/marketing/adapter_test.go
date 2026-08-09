package marketing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestAdapterRegistrationMetadataCapabilitiesAndAuthentication(t *testing.T) {
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatalf("registered adapters=%v", socialhub.Adapters())
	}
	digest := hmac.New(sha256.New, []byte("app-secret"))
	_, _ = digest.Write([]byte("access-token"))
	wantProof := hex.EncodeToString(digest.Sum(nil))
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/v25.0/act_123" {
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		if request.Header.Get("Authorization") != "Bearer access-token" || request.URL.Query().Get("appsecret_proof") != wantProof {
			writeJSON(writer, http.StatusUnauthorized, `{"error":{"code":190,"message":"bad auth"}}`)
			return
		}
		writeJSON(writer, http.StatusOK, `{"id":"act_123","name":"Example Ads","currency":"USD"}`)
	}))
	defer server.Close()

	adapter, client := newTestAdapter(t, server, []string{managementScope})
	metadata := adapter.Metadata()
	if adapter.Name() != adapterName || metadata.Product != productName || metadata.APIVersion != graphVersion || metadata.DocURL != documentationURL || metadata.VerifiedAt.IsZero() {
		t.Fatalf("metadata=%#v", metadata)
	}
	if client.Platform() != platformName || client.Account() != "ads-primary" || client.Management() == nil || client.Insights() == nil {
		t.Fatalf("client=%#v", client)
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil || !capabilities.Has(CapabilityMarketingManagement) || !capabilities.Has(CapabilityMarketingInsights) {
		t.Fatalf("capabilities=%#v err=%v", capabilities, err)
	}
	for _, capability := range []socialhub.Capability{
		socialhub.CapPublish, socialhub.CapFetch, socialhub.CapMedia,
		socialhub.CapReact, socialhub.CapMessage, socialhub.CapWebhook,
	} {
		if capabilities.Has(capability) {
			t.Fatalf("common capability %q must be unavailable", capability)
		}
	}
	if publisher, ok := client.Publisher(); ok || publisher != nil {
		t.Fatal("publisher must be unavailable")
	}
	if fetcher, ok := client.Fetcher(); ok || fetcher != nil {
		t.Fatal("fetcher must be unavailable")
	}
	if uploader, ok := client.MediaUploader(); ok || uploader != nil {
		t.Fatal("media uploader must be unavailable")
	}
	if reactor, ok := client.Reactor(); ok || reactor != nil {
		t.Fatal("reactor must be unavailable")
	}
	if messenger, ok := client.Messenger(); ok || messenger != nil {
		t.Fatal("messenger must be unavailable")
	}
	if webhook, ok := client.WebhookHandler(); ok || webhook != nil {
		t.Fatal("webhook must be unavailable")
	}
	account, err := client.GetAdAccount(context.Background())
	if err != nil || account.ID != "act_123" || len(account.Raw) == 0 {
		t.Fatalf("account=%#v err=%v", account, err)
	}
	oauth, err := adapter.OAuth(context.Background(), "ads-primary")
	if err != nil || oauth.ClientSecret != "app-secret" || oauth.Clock.Now() != testNow {
		t.Fatalf("oauth=%#v err=%v", oauth, err)
	}
	if client.Close() != nil || adapter.Close() != nil {
		t.Fatal("close failed")
	}
	if _, err := adapter.Client(context.Background(), "ads-primary"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close error=%v", err)
	}
}

func TestAdapterValidationScopeAndOptionalProof(t *testing.T) {
	valid := socialhub.AccountConfig{
		ID: "ads", AccessTokenRef: "test://token", Settings: map[string]any{"ad_account_id": testAdAccountID},
	}
	tests := []struct {
		name   string
		config socialhub.AdapterConfig
	}{
		{"wrong adapter", socialhub.AdapterConfig{Adapter: "other", Accounts: []socialhub.AccountConfig{valid}}},
		{"bad endpoint", socialhub.AdapterConfig{Adapter: adapterName, Settings: map[string]any{"base_url": "ftp://graph.example"}, Accounts: []socialhub.AccountConfig{valid}}},
		{"endpoint query", socialhub.AdapterConfig{Adapter: adapterName, Settings: map[string]any{"base_url": "https://graph.example?v=25"}, Accounts: []socialhub.AccountConfig{valid}}},
		{"missing token", socialhub.AdapterConfig{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "ads", Settings: valid.Settings}}}},
		{"prefixed account", socialhub.AdapterConfig{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "ads", AccessTokenRef: "test://token", Settings: map[string]any{"ad_account_id": "act_123"}}}}},
		{"unknown setting", socialhub.AdapterConfig{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "ads", AccessTokenRef: "test://token", Settings: map[string]any{"ad_account_id": testAdAccountID, "unknown": true}}}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := (&Adapter{}).Init(context.Background(), test.config); !errors.Is(err, socialhub.ErrInvalidArgument) {
				t.Fatalf("error=%v", err)
			}
		})
	}

	var proofSeen string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		proofSeen = request.URL.Query().Get("appsecret_proof")
		writeJSON(writer, http.StatusOK, `{"id":"act_123"}`)
	}))
	defer server.Close()
	config := socialhub.AdapterConfig{
		Adapter: adapterName, Settings: map[string]any{
			"base_url": server.URL, "auth_url": server.URL + "/oauth", "token_url": server.URL + "/token",
		},
		Accounts: []socialhub.AccountConfig{{ID: "ads", AccessTokenRef: "test://token", Approval: socialhub.ApprovalConfig{Scopes: []string{"pages_read_engagement"}}, Settings: valid.Settings}},
	}
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), config, socialhub.WithHTTPClient(server.Client()), socialhub.WithSecretResolver(mapResolver{"test://token": "token"})); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "ads")
	if err != nil {
		t.Fatal(err)
	}
	client := common.(*Client)
	capabilities, _ := client.Capabilities(context.Background())
	if capabilities.Has(CapabilityMarketingManagement) || capabilities[CapabilityMarketingManagement].Approval != socialhub.ApprovalRequired {
		t.Fatalf("management capability=%#v", capabilities[CapabilityMarketingManagement])
	}
	if _, err := client.GetAdAccount(context.Background()); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("scope error=%v", err)
	}
	client.scopes = nil
	if _, err := client.GetAdAccount(context.Background()); err != nil || proofSeen != "" {
		t.Fatalf("caller-managed token request err=%v proof=%q", err, proofSeen)
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing client error=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "ads"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("incomplete OAuth error=%v", err)
	}
	adapter.options.Secrets = mapResolver{}
	if _, err := adapter.Client(context.Background(), "ads"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("missing token error=%v", err)
	}
}

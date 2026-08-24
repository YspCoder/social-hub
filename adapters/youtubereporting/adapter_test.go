package youtubereporting

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestAdapterRegistrationMetadataCapabilitiesAndClose(t *testing.T) {
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatalf("adapter %q not registered", adapterName)
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newStaticClient(t, server, staticConfig(server.URL))
	metadata := adapter.Metadata()
	if adapter.Name() != adapterName || metadata.Name != adapterName || metadata.Product != productName ||
		metadata.APIVersion != apiVersion || metadata.DocURL != documentationURL || metadata.VerifiedAt.IsZero() || discoveryRevision == "" {
		t.Fatalf("metadata=%#v", metadata)
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil || !capabilities[CapabilityYouTubeBulkReporting].Supported || capabilities[socialhub.CapFetch].Supported {
		t.Fatalf("capabilities=%#v err=%v", capabilities, err)
	}
	if client.Platform() != platformName || client.Account() != testAccountID || client.Reporting() != client || client.contentOwnerID != "" {
		t.Fatal("client identity or typed workflow failed")
	}
	for name, unsupported := range map[string]bool{
		"publish": func() bool { value, ok := client.Publisher(); return value == nil && !ok }(),
		"fetch":   func() bool { value, ok := client.Fetcher(); return value == nil && !ok }(),
		"media":   func() bool { value, ok := client.MediaUploader(); return value == nil && !ok }(),
		"react":   func() bool { value, ok := client.Reactor(); return value == nil && !ok }(),
		"message": func() bool { value, ok := client.Messenger(); return value == nil && !ok }(),
		"webhook": func() bool { value, ok := client.WebhookHandler(); return value == nil && !ok }(),
	} {
		if !unsupported {
			t.Errorf("%s should be unsupported", name)
		}
	}
	if client.Close() != nil || adapter.Close() != nil {
		t.Fatal("close failed")
	}
	if _, err := adapter.Client(context.Background(), testAccountID); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("closed client error=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), testAccountID); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("closed OAuth error=%v", err)
	}
	if err := adapter.Init(context.Background(), staticConfig(server.URL)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("closed init error=%v", err)
	}
}

func TestAdapterInitValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	base := staticConfig(server.URL)
	tests := []func(socialhub.AdapterConfig) socialhub.AdapterConfig{
		func(value socialhub.AdapterConfig) socialhub.AdapterConfig { value.Adapter = "wrong"; return value },
		func(value socialhub.AdapterConfig) socialhub.AdapterConfig { value.Product = "wrong"; return value },
		func(value socialhub.AdapterConfig) socialhub.AdapterConfig {
			value.Settings = map[string]any{"base_url": "https://user:pass@example.com"}
			return value
		},
		func(value socialhub.AdapterConfig) socialhub.AdapterConfig {
			value.Settings["unknown"] = true
			return value
		},
		func(value socialhub.AdapterConfig) socialhub.AdapterConfig {
			value.Accounts[0].Settings["content_owner_id"] = "bad/id"
			return value
		},
		func(value socialhub.AdapterConfig) socialhub.AdapterConfig {
			value.Accounts[0].AccessTokenRef = ""
			return value
		},
		func(value socialhub.AdapterConfig) socialhub.AdapterConfig {
			value.Accounts[0].Settings["refresh_token_ref"] = "test://refresh"
			return value
		},
		func(value socialhub.AdapterConfig) socialhub.AdapterConfig {
			value.Accounts[0].Approval.Scopes = []string{"openid"}
			return value
		},
		func(value socialhub.AdapterConfig) socialhub.AdapterConfig {
			value.Accounts[0].Approval.Scopes = []string{analyticsReadScope, analyticsReadScope}
			return value
		},
		func(value socialhub.AdapterConfig) socialhub.AdapterConfig {
			value.Accounts[0].ClientID = ""
			return value
		},
		func(value socialhub.AdapterConfig) socialhub.AdapterConfig {
			value.Accounts[0].SecretRef = ""
			return value
		},
		func(value socialhub.AdapterConfig) socialhub.AdapterConfig {
			value.Accounts[0].Settings["service_account_email"] = "reports@test.iam.gserviceaccount.com"
			value.Accounts[0].Settings["private_key_ref"] = "test://key"
			return value
		},
		func(value socialhub.AdapterConfig) socialhub.AdapterConfig {
			value.Accounts[0].AccessTokenRef = ""
			value.Accounts[0].ClientID = ""
			value.Accounts[0].SecretRef = ""
			value.Accounts[0].Settings["service_account_email"] = "reports@test.iam.gserviceaccount.com"
			value.Accounts[0].Settings["private_key_ref"] = "test://key"
			return value
		},
		func(value socialhub.AdapterConfig) socialhub.AdapterConfig {
			value.Accounts[0].Webhook.SecretRef = "test://webhook"
			return value
		},
	}
	for index, mutate := range tests {
		config := base
		config.Settings = cloneMap(base.Settings)
		config.Accounts = append([]socialhub.AccountConfig(nil), base.Accounts...)
		config.Accounts[0].Settings = cloneMap(base.Accounts[0].Settings)
		config.Accounts[0].Approval.Scopes = append([]string(nil), base.Accounts[0].Approval.Scopes...)
		if err := (&Adapter{}).Init(context.Background(), mutate(config)); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Errorf("case %d error=%v", index, err)
		}
	}
}

func TestAdapterClientSecretsOverridesAndScopeGate(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), staticConfig(server.URL), socialhub.WithHTTPClient(server.Client()), socialhub.WithSecretResolver(mapResolver{})); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing account error=%v", err)
	}
	if _, err := adapter.Client(context.Background(), testAccountID); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("missing access token error=%v", err)
	}
	adapter.options.Secrets = mapResolver{"test://access-token": testAccessToken}
	if _, err := adapter.OAuth(context.Background(), testAccountID); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("missing client secret error=%v", err)
	}
	if _, err := (&Adapter{}).Client(context.Background(), testAccountID); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("uninitialized client error=%v", err)
	}

	requestSeen := false
	scopeServer := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requestSeen = true }))
	defer scopeServer.Close()
	config := staticConfig(scopeServer.URL)
	config.Accounts[0].Approval.Scopes = nil
	_, client := newStaticClient(t, scopeServer, config)
	client.scopes = []string{"openid"}
	if _, err := client.ListJobs(context.Background(), ListRequest{}); !errors.Is(err, socialhub.ErrApprovalRequired) || requestSeen {
		t.Fatalf("scope error=%v requestSeen=%v", err, requestSeen)
	}
}

func TestClientRejectsCredentialRedirect(t *testing.T) {
	redirected := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/v1/jobs" {
			http.Redirect(writer, request, "/credential-target", http.StatusFound)
			return
		}
		redirected = true
	}))
	defer server.Close()
	_, client := newStaticClient(t, server, staticConfig(server.URL))
	if _, err := client.ListJobs(context.Background(), ListRequest{}); err == nil || redirected {
		t.Fatalf("redirect error=%v redirected=%v", err, redirected)
	}
}

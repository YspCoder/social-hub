package admob

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
	adapter, client := newStaticClient(t, server)
	metadata := adapter.Metadata()
	if adapter.Name() != adapterName || metadata.Name != adapterName || metadata.Product != productName ||
		metadata.APIVersion != apiVersion || metadata.DocURL != documentationURL || metadata.VerifiedAt.IsZero() || discoveryRevision == "" {
		t.Fatalf("metadata=%#v", metadata)
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil || !capabilities[CapabilityAdMob].Supported || capabilities[socialhub.CapFetch].Supported {
		t.Fatalf("capabilities=%#v err=%v", capabilities, err)
	}
	if client.Platform() != platformName || client.Account() != testAccountID || client.Accounts() != client ||
		client.Inventory() != client || client.Reporting() != client {
		t.Fatal("client identity or typed workflows failed")
	}
	if value, ok := client.Publisher(); ok || value != nil {
		t.Fatal("publisher should be unsupported")
	}
	if value, ok := client.Fetcher(); ok || value != nil {
		t.Fatal("fetcher should be unsupported")
	}
	if value, ok := client.MediaUploader(); ok || value != nil {
		t.Fatal("media should be unsupported")
	}
	if value, ok := client.Reactor(); ok || value != nil {
		t.Fatal("reactor should be unsupported")
	}
	if value, ok := client.Messenger(); ok || value != nil {
		t.Fatal("messenger should be unsupported")
	}
	if value, ok := client.WebhookHandler(); ok || value != nil {
		t.Fatal("webhook should be unsupported")
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
			value.Settings = map[string]any{"unknown": true}
			return value
		},
		func(value socialhub.AdapterConfig) socialhub.AdapterConfig {
			value.Accounts[0].Settings = map[string]any{"publisher_id": "123"}
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
			value.Accounts[0].ClientID = ""
			return value
		},
		func(value socialhub.AdapterConfig) socialhub.AdapterConfig {
			value.Accounts[0].SecretRef = ""
			return value
		},
		func(value socialhub.AdapterConfig) socialhub.AdapterConfig {
			value.Accounts[0].AccessTokenRef = ""
			value.Accounts[0].Settings["refresh_token_ref"] = "test://refresh"
			value.Accounts[0].Approval.Scopes = []string{"openid"}
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
		if err := (&Adapter{}).Init(context.Background(), mutate(config)); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Errorf("case %d error=%v", index, err)
		}
	}
}

func TestAdapterClientScopeSecretsAndRedirects(t *testing.T) {
	redirected := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/v1/"+accountName() {
			http.Redirect(writer, request, "/credential-target", http.StatusFound)
			return
		}
		redirected = true
		writeJSON(t, writer, http.StatusOK, accountFixture())
	}))
	defer server.Close()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), staticConfig(server.URL),
		socialhub.WithHTTPClient(server.Client()), socialhub.WithSecretResolver(mapResolver{"test://client-secret": "secret"}),
	); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing account error=%v", err)
	}
	if _, err := adapter.Client(context.Background(), testAccountID); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("missing token error=%v", err)
	}
	adapter.options.Secrets = mapResolver{"test://access-token": testAccessToken}
	if _, err := adapter.OAuth(context.Background(), testAccountID); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("missing client secret error=%v", err)
	}
	adapter.config.Accounts[0].ClientID = ""
	if _, err := adapter.OAuth(context.Background(), testAccountID); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("OAuth config error=%v", err)
	}
	if _, err := (&Adapter{}).Client(context.Background(), testAccountID); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("uninitialized client error=%v", err)
	}
	_, client := newStaticClient(t, server)
	if _, err := client.GetAccount(context.Background()); err == nil || redirected {
		t.Fatalf("redirect error=%v redirected=%v", err, redirected)
	}
}

func TestReportOnlyScopeGatesInventory(t *testing.T) {
	requestSeen := false
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requestSeen = true }))
	defer server.Close()
	_, client := newStaticClient(t, server, func(config *socialhub.AdapterConfig) {
		config.Accounts[0].Approval.Scopes = []string{reportScope}
	})
	if _, err := client.ListApps(context.Background(), ListRequest{}); !errors.Is(err, socialhub.ErrApprovalRequired) || requestSeen {
		t.Fatalf("inventory scope error=%v requestSeen=%v", err, requestSeen)
	}
}

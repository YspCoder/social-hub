package weibo

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

type mapResolver map[string]string

func (r mapResolver) Resolve(_ context.Context, reference string) (string, error) {
	value, ok := r[reference]
	if !ok {
		return "", errors.New("missing test secret")
	}
	return value, nil
}

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

func newTestAdapter(t *testing.T, server *httptest.Server) (*Adapter, *Client) {
	t.Helper()
	config := socialhub.AdapterConfig{
		Adapter: adapterName,
		Product: "open-api",
		Settings: map[string]any{
			"base_url":  server.URL,
			"auth_url":  server.URL + "/oauth2/authorize",
			"token_url": server.URL + "/oauth2/access_token",
		},
		Accounts: []socialhub.AccountConfig{{
			ID:             "primary",
			ClientID:       "client-id",
			SecretRef:      "test://client-secret",
			AccessTokenRef: "test://access-token",
			Settings:       map[string]any{"source_ip": "203.0.113.10"},
		}},
	}
	adapter := &Adapter{}
	err := adapter.Init(context.Background(), config,
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{
			"test://client-secret": "client-secret",
			"test://access-token":  "access-token",
		}),
		socialhub.WithClock(fixedClock{now: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)}),
	)
	if err != nil {
		t.Fatal(err)
	}
	client, err := adapter.Client(context.Background(), "primary")
	if err != nil {
		t.Fatal(err)
	}
	return adapter, client.(*Client)
}

func TestAdapterRegistrationAndSurface(t *testing.T) {
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatalf("registered adapters = %v", socialhub.Adapters())
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newTestAdapter(t, server)
	if adapter.Metadata().APIVersion != "2" || client.Platform() != "weibo" || client.Account() != "primary" {
		t.Fatalf("metadata=%#v identity=%s/%s", adapter.Metadata(), client.Platform(), client.Account())
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil || !capabilities.Has(socialhub.CapPublish) || !capabilities.Has(socialhub.CapFetch) || !capabilities.Has(socialhub.CapMedia) || !capabilities.Has(socialhub.CapReact) {
		t.Fatalf("capabilities=%#v err=%v", capabilities, err)
	}
	if _, ok := client.Messenger(); ok {
		t.Fatal("messenger should be unavailable")
	}
	if _, ok := client.WebhookHandler(); ok {
		t.Fatal("webhook should be unavailable")
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "primary"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close error=%v", err)
	}
}

func TestGetUserUsesQueryTokenAndMapsProfile(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/2/users/show.json" || request.URL.Query().Get("access_token") != "access-token" || request.URL.Query().Get("uid") != "42" {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = writer.Write([]byte(`{"idstr":"42","screen_name":"operator","name":"Operator","avatar_large":"https://img.example/avatar.jpg","verified":true,"verified_type":2}`))
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	user, err := client.GetUser(context.Background(), "42")
	if err != nil || user.ID != "42" || user.Username == nil || *user.Username != "operator" || user.AccountType == nil || *user.AccountType != "verified" {
		t.Fatalf("user=%#v err=%v", user, err)
	}
}

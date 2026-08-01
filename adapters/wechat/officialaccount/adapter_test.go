package officialaccount

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync/atomic"
	"testing"

	"social-hub/extensions/material"
	"social-hub/pkg/socialhub"
)

const testAESKey = "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"

type mapResolver map[string]string

func (r mapResolver) Resolve(_ context.Context, reference string) (string, error) {
	value, ok := r[reference]
	if !ok {
		return "", errors.New("missing test secret")
	}
	return value, nil
}

func newTestClient(t *testing.T, server *httptest.Server) (*Adapter, *Client) {
	t.Helper()
	config := socialhub.AdapterConfig{
		Adapter:  adapterName,
		Product:  "official-account",
		Settings: map[string]any{"base_url": server.URL},
		Accounts: []socialhub.AccountConfig{{
			ID:        "primary",
			AppID:     "wx-app-id",
			SecretRef: "test://app-secret",
			Webhook: socialhub.WebhookConfig{
				TokenRef:  "test://webhook-token",
				AESKeyRef: "test://aes-key",
			},
		}},
	}
	adapter := &Adapter{}
	err := adapter.Init(context.Background(), config,
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{
			"test://app-secret":    "app-secret",
			"test://webhook-token": "webhook-token",
			"test://aes-key":       testAESKey,
		}),
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
	adapter, client := newTestClient(t, server)
	if adapter.Name() != adapterName || adapter.Metadata().Product != "official-account" {
		t.Fatalf("metadata = %#v", adapter.Metadata())
	}
	if client.Platform() != "wechat" || client.Account() != "primary" {
		t.Fatalf("identity = %s/%s", client.Platform(), client.Account())
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil || !capabilities.Has(socialhub.CapFetch) || !capabilities.Has(socialhub.CapMedia) || !capabilities.Has(socialhub.CapMessage) || !capabilities.Has(socialhub.CapWebhook) {
		t.Fatalf("capabilities = %#v, err = %v", capabilities, err)
	}
	if capabilities.Has(socialhub.CapPublish) {
		t.Fatal("common publisher should not be advertised")
	}
	if _, ok := client.Publisher(); ok {
		t.Fatal("common publisher should be unavailable")
	}
	if _, ok := client.Fetcher(); !ok {
		t.Fatal("fetcher unavailable")
	}
	if _, ok := client.MediaUploader(); !ok {
		t.Fatal("uploader unavailable")
	}
	if _, ok := client.Messenger(); !ok {
		t.Fatal("messenger unavailable")
	}
	if _, ok := client.WebhookHandler(); !ok {
		t.Fatal("webhook unavailable")
	}
	if manager := client.MaterialManager(); manager == nil {
		t.Fatal("material manager unavailable")
	}
	if client.Drafts() == nil {
		t.Fatal("draft service unavailable")
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "primary"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close error = %v", err)
	}
}

func TestAppTokenCachingUserAndMessage(t *testing.T) {
	t.Parallel()
	var tokenCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/cgi-bin/token":
			tokenCalls.Add(1)
			if request.URL.Query().Get("appid") != "wx-app-id" || request.URL.Query().Get("secret") != "app-secret" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"access_token":"wechat-token","expires_in":7200}`))
		case "/cgi-bin/user/info":
			assertAccessToken(t, writer, request)
			_, _ = writer.Write([]byte(`{"subscribe":1,"openid":"openid-1","nickname":"Reader","headimgurl":"https://cdn.example/avatar.jpg","unionid":"union-1","subscribe_time":1785542400}`))
		case "/cgi-bin/message/custom/send":
			assertAccessToken(t, writer, request)
			_, _ = writer.Write([]byte(`{"errcode":0,"errmsg":"ok","msgid":123456789}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server)
	user, err := client.GetUser(context.Background(), "openid-1")
	if err != nil || user.DisplayName == nil || *user.DisplayName != "Reader" {
		t.Fatalf("user = %#v, err = %v", user, err)
	}
	text := "hello"
	message, err := client.SendMessage(context.Background(), socialhub.SendMessageRequest{RecipientIDs: []string{"openid-1"}, Text: &text})
	if err != nil || message.ID != "123456789" || message.Direction != socialhub.DirectionOutbound {
		t.Fatalf("message = %#v, err = %v", message, err)
	}
	if tokenCalls.Load() != 1 {
		t.Fatalf("token calls = %d", tokenCalls.Load())
	}
}

func assertAccessToken(t *testing.T, writer http.ResponseWriter, request *http.Request) {
	t.Helper()
	if request.URL.Query().Get("access_token") != "wechat-token" || request.Header.Get("Authorization") != "" {
		writer.WriteHeader(http.StatusUnauthorized)
		t.Fatalf("token placement query=%q header=%q", request.URL.Query().Get("access_token"), request.Header.Get("Authorization"))
	}
}

var _ material.Provider = (*Client)(nil)

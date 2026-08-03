package messenger

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

type testResolver map[string]string

func (r testResolver) Resolve(_ context.Context, reference string) (string, error) {
	value, ok := r[reference]
	if !ok {
		return "", errors.New("missing test secret")
	}
	return value, nil
}

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

var testNow = time.Date(2026, time.August, 3, 12, 0, 0, 0, time.UTC)

func testConfig(serverURL string, withWebhook bool) socialhub.AdapterConfig {
	account := socialhub.AccountConfig{
		ID: "main", AccessTokenRef: "test://page-token", Settings: map[string]any{"page_id": "123456789"},
	}
	if withWebhook {
		account.Webhook = socialhub.WebhookConfig{
			SecretRef: "test://app-secret", TokenRef: "test://verify-token",
		}
	}
	return socialhub.AdapterConfig{
		Adapter: adapterName, Product: productName,
		Settings: map[string]any{"base_url": serverURL + "/v26.0"}, Accounts: []socialhub.AccountConfig{account},
	}
}

func newTestClient(t *testing.T, server *httptest.Server, withWebhook bool) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server.URL, withWebhook),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(testResolver{
			"test://page-token": "page-token", "test://app-secret": "app-secret", "test://verify-token": "verify-token",
		}),
		socialhub.WithClock(fixedClock{now: testNow}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	return adapter, common.(*Client)
}

func writeTestJSON(t *testing.T, writer http.ResponseWriter, value any) {
	t.Helper()
	writer.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(writer).Encode(value); err != nil {
		t.Errorf("encode response: %v", err)
	}
}

func errorCode(err error) socialhub.ErrorCode {
	var platformErr *socialhub.Error
	if errors.As(err, &platformErr) {
		return platformErr.Code
	}
	return ""
}

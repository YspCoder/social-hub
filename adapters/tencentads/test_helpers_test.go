package tencentads

import (
	"context"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

const testAdvertiserID int64 = 123456

var testNow = time.Date(2026, time.August, 9, 1, 2, 3, 0, time.UTC)

type mapResolver map[string]string

func (resolver mapResolver) Resolve(_ context.Context, reference string) (string, error) {
	value, found := resolver[reference]
	if !found {
		return "", errors.New("missing test secret")
	}
	return value, nil
}

type fixedClock struct{ now time.Time }

func (clock fixedClock) Now() time.Time { return clock.now }

func testConfig(baseURL string) socialhub.AdapterConfig {
	return socialhub.AdapterConfig{
		Adapter: adapterName, Product: productName,
		Settings: map[string]any{
			"base_url": baseURL, "authorization_base_url": baseURL, "token_base_url": baseURL,
		},
		Accounts: []socialhub.AccountConfig{{
			ID: "ads-primary", AppID: "789", SecretRef: "test://app-secret",
			AccessTokenRef: "test://access-token", Settings: map[string]any{"account_id": testAdvertiserID},
		}},
	}
}

func newTestAdapter(t *testing.T, server *httptest.Server) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server.URL),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{
			"test://app-secret": "app-secret", "test://access-token": "access-token",
		}),
		socialhub.WithClock(fixedClock{now: testNow}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "ads-primary")
	if err != nil {
		t.Fatal(err)
	}
	return adapter, common.(*Client)
}

func assertAPIAuth(t *testing.T, request *http.Request) {
	t.Helper()
	query := request.URL.Query()
	if query.Get("access_token") != "access-token" || query.Get("timestamp") != strconv.FormatInt(testNow.Unix(), 10) {
		t.Errorf("unexpected API auth query: %v", query)
	}
	nonce := query.Get("nonce")
	decoded, err := hex.DecodeString(nonce)
	if err != nil || len(decoded) != 8 {
		t.Errorf("nonce=%q err=%v", nonce, err)
	}
}

func writeJSON(writer http.ResponseWriter, status int, value string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_, _ = writer.Write([]byte(value))
}

func hubError(t *testing.T, err error) *socialhub.Error {
	t.Helper()
	var result *socialhub.Error
	if !errors.As(err, &result) {
		t.Fatalf("expected *socialhub.Error, got %T: %v", err, err)
	}
	return result
}

func cloneMap(input map[string]any) map[string]any {
	result := make(map[string]any, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

package baiduads

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

const (
	testAccountID   socialhub.AccountID = "baidu-main"
	testAccessToken                     = "baidu-access-token"
	testSecretKey                       = "baidu-secret-key"
	testUserName                        = "search-user"
)

var testNow = time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC)

type mapResolver map[string]string

func (resolver mapResolver) Resolve(_ context.Context, reference string) (string, error) {
	value, found := resolver[reference]
	if !found {
		return "", errors.New("secret not found")
	}
	return value, nil
}

type fixedClock struct{ value time.Time }

func (clock fixedClock) Now() time.Time { return clock.value }

func testConfig(baseURL string) socialhub.AdapterConfig {
	return socialhub.AdapterConfig{
		Adapter: adapterName, Product: productName,
		Settings: map[string]any{"base_url": baseURL, "oauth_base_url": baseURL},
		Accounts: []socialhub.AccountConfig{{
			ID: testAccountID, AppID: "baidu-app-id", SecretRef: "test://secret",
			AccessTokenRef: "test://access", Settings: map[string]any{"user_name": testUserName},
		}},
	}
}

func newTestClient(t *testing.T, server *httptest.Server) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server.URL),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{"test://access": testAccessToken, "test://secret": testSecretKey}),
		socialhub.WithClock(fixedClock{value: testNow}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), testAccountID)
	if err != nil {
		t.Fatal(err)
	}
	return adapter, common.(*Client)
}

func decodeRequest(t *testing.T, request *http.Request) map[string]json.RawMessage {
	t.Helper()
	defer request.Body.Close()
	if request.Method != http.MethodPost || request.Header.Get("Content-Type") != "application/json;charset=UTF-8" {
		t.Fatalf("method=%s content-type=%q", request.Method, request.Header.Get("Content-Type"))
	}
	var envelope struct {
		Header requestHeader              `json:"header"`
		Body   map[string]json.RawMessage `json:"body"`
	}
	if err := json.NewDecoder(request.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Header.UserName != testUserName || envelope.Header.AccessToken != testAccessToken {
		t.Fatalf("request header=%+v", envelope.Header)
	}
	if request.URL.Query().Get("accessToken") != "" {
		t.Fatal("access token leaked into query")
	}
	return envelope.Body
}

func decodeRaw[T any](t *testing.T, value json.RawMessage) T {
	t.Helper()
	var decoded T
	if err := json.Unmarshal(value, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded
}

func writeSuccess(t *testing.T, writer http.ResponseWriter, data any) {
	t.Helper()
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("X-B3-Traceid", "trace-success")
	if err := json.NewEncoder(writer).Encode(map[string]any{
		"header": map[string]any{"status": 0, "desc": "success", "failures": []any{}, "traceid": "body-trace"},
		"body":   map[string]any{"data": data},
	}); err != nil {
		t.Fatal(err)
	}
}

func requireHubError(t *testing.T, err error) *socialhub.Error {
	t.Helper()
	if err == nil {
		t.Fatal("expected error")
	}
	var hub *socialhub.Error
	if !errors.As(err, &hub) {
		t.Fatalf("error type=%T value=%v", err, err)
	}
	return hub
}

func stringPointer(value string) *string  { return &value }
func floatPointer(value float64) *float64 { return &value }
func boolPointer(value bool) *bool        { return &value }

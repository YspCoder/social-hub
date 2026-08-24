package googleads

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

var testNow = time.Date(2026, time.August, 9, 1, 2, 3, 0, time.UTC)

type fixedClock struct{ now time.Time }

func (clock fixedClock) Now() time.Time { return clock.now }

const (
	testCustomerID      = "1234567890"
	testLoginCustomerID = "0987654321"
	testDeveloperToken  = "1234567890123456789012"
	testBudget          = "customers/1234567890/campaignBudgets/101"
	testCampaign        = "customers/1234567890/campaigns/201"
	testAdGroup         = "customers/1234567890/adGroups/301"
	testAd              = "customers/1234567890/ads/401"
	testAdGroupAd       = "customers/1234567890/adGroupAds/301~401"
)

type mapResolver map[string]string

func (resolver mapResolver) Resolve(_ context.Context, reference string) (string, error) {
	value, found := resolver[reference]
	if !found {
		return "", errors.New("missing test secret")
	}
	return value, nil
}

func testConfig(baseURL string) socialhub.AdapterConfig {
	return socialhub.AdapterConfig{
		Adapter: adapterName, Product: productName,
		Settings: map[string]any{"base_url": baseURL, "auth_url": baseURL, "token_url": baseURL},
		Accounts: []socialhub.AccountConfig{{
			ID: "brand-search", ClientID: "client-id", SecretRef: "test://client-secret",
			AccessTokenRef: "test://access-token",
			Settings: map[string]any{
				"customer_id": testCustomerID, "login_customer_id": testLoginCustomerID,
				"developer_token_ref": "test://developer-token",
			},
		}},
	}
}

func newTestAdapter(t *testing.T, server *httptest.Server) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server.URL),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithClock(fixedClock{now: testNow}),
		socialhub.WithSecretResolver(mapResolver{
			"test://client-secret": "client-secret", "test://access-token": "access-token",
			"test://developer-token": testDeveloperToken,
		}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "brand-search")
	if err != nil {
		t.Fatal(err)
	}
	return adapter, common.(*Client)
}

func assertAPIRequest(t *testing.T, request *http.Request) {
	t.Helper()
	if request.Header.Get("Authorization") != "Bearer access-token" ||
		request.Header.Get("developer-token") != testDeveloperToken ||
		request.Header.Get("login-customer-id") != testLoginCustomerID ||
		request.URL.Query().Get("access_token") != "" {
		t.Fatalf("unexpected API authentication: %s %s headers=%v", request.Method, request.URL, request.Header)
	}
}

func decodeBody(t *testing.T, request *http.Request) map[string]any {
	t.Helper()
	if request.Method != http.MethodPost || request.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("unexpected request: %s %s headers=%v", request.Method, request.URL, request.Header)
	}
	var body map[string]any
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	return body
}

func firstOperation(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	operations, ok := body["operations"].([]any)
	if !ok || len(operations) != 1 || body["responseContentType"] != "MUTABLE_RESOURCE" {
		t.Fatalf("mutate body=%v", body)
	}
	operation, ok := operations[0].(map[string]any)
	if !ok {
		t.Fatalf("operation=%v", operations[0])
	}
	return operation
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

func stringPointer(value string) *string { return &value }
func int64Pointer(value int64) *int64    { return &value }

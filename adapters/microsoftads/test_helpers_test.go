package microsoftads

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

const (
	testCustomerID     = "1001"
	testAccountID      = "2001"
	testCampaignID     = "3001"
	testAdGroupID      = "4001"
	testAdID           = "5001"
	testKeywordID      = "6001"
	testDeveloperToken = "developer-token-value"
)

type fixedClock struct{ now time.Time }

func (clock fixedClock) Now() time.Time { return clock.now }

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
		Settings: map[string]any{
			"campaign_base_url": baseURL, "customer_base_url": baseURL,
			"reporting_base_url": baseURL, "auth_url": baseURL + "/authorize",
			"token_url": baseURL + "/token", "max_report_bytes": int64(16),
		},
		Accounts: []socialhub.AccountConfig{{
			ID: "brand-search", ClientID: "client-id", SecretRef: "test://client-secret",
			AccessTokenRef: "test://access-token",
			Settings: map[string]any{
				"customer_id": testCustomerID, "customer_account_id": testAccountID,
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
		request.Header.Get("DeveloperToken") != testDeveloperToken ||
		request.URL.Query().Get("access_token") != "" {
		t.Fatalf("unexpected API authentication: %s %s headers=%v", request.Method, request.URL, request.Header)
	}
	if request.URL.Path == "/Account/Query" {
		if request.Header.Get("CustomerId") != "" || request.Header.Get("CustomerAccountId") != "" {
			t.Fatalf("customer management received account headers: %v", request.Header)
		}
		return
	}
	if request.Header.Get("CustomerId") != testCustomerID || request.Header.Get("CustomerAccountId") != testAccountID {
		t.Fatalf("missing campaign/reporting account headers: %v", request.Header)
	}
}

func decodeBody(t *testing.T, request *http.Request) map[string]any {
	t.Helper()
	if request.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("content type=%q", request.Header.Get("Content-Type"))
	}
	var body map[string]any
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	return body
}

func writeValue(t *testing.T, writer http.ResponseWriter, status int, value any) {
	t.Helper()
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	if err := json.NewEncoder(writer).Encode(value); err != nil {
		t.Fatal(err)
	}
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

func floatPointer(value float64) *float64         { return &value }
func matchTypePointer(value MatchType) *MatchType { return &value }
func networkPointer(value Network) *Network       { return &value }

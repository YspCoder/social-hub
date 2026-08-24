package analyticsdata

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	testAccountID   socialhub.AccountID = "analytics-main"
	testPropertyID                      = "123456789"
	testAccessToken                     = "analytics-access-token"
)

var testNow = time.Date(2026, time.August, 10, 12, 0, 0, 0, time.UTC)

type mapResolver map[string]string

func (resolver mapResolver) Resolve(_ context.Context, reference string) (string, error) {
	value, found := resolver[reference]
	if !found {
		return "", errors.New("secret not found")
	}
	return value, nil
}

type mutableClock struct {
	mu    sync.RWMutex
	value time.Time
}

func (clock *mutableClock) Now() time.Time {
	clock.mu.RLock()
	defer clock.mu.RUnlock()
	return clock.value
}

func (clock *mutableClock) Set(value time.Time) {
	clock.mu.Lock()
	clock.value = value
	clock.mu.Unlock()
}

func staticConfig(baseURL string) socialhub.AdapterConfig {
	return socialhub.AdapterConfig{
		Adapter: adapterName, Product: productName,
		Settings: map[string]any{"base_url": baseURL, "auth_url": baseURL + "/authorize", "token_url": baseURL + "/token"},
		Accounts: []socialhub.AccountConfig{{
			ID: testAccountID, ClientID: "client-id", SecretRef: "test://client-secret", AccessTokenRef: "test://access-token",
			Approval: socialhub.ApprovalConfig{Scopes: []string{readOnlyScope}},
			Settings: map[string]any{"property_id": testPropertyID},
		}},
	}
}

func managedConfig(baseURL string) socialhub.AdapterConfig {
	config := staticConfig(baseURL)
	config.Accounts[0].AccessTokenRef = ""
	config.Accounts[0].Settings["refresh_token_ref"] = "test://refresh-token"
	return config
}

func serviceAccountConfig(baseURL string) socialhub.AdapterConfig {
	config := staticConfig(baseURL)
	config.Accounts[0].AccessTokenRef = ""
	config.Accounts[0].ClientID = ""
	config.Accounts[0].SecretRef = ""
	config.Accounts[0].Settings["service_account_email"] = "reports@test-project.iam.gserviceaccount.com"
	config.Accounts[0].Settings["private_key_ref"] = "test://private-key"
	return config
}

func newStaticClient(t *testing.T, server *httptest.Server) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), staticConfig(server.URL),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{"test://access-token": testAccessToken, "test://client-secret": "client-secret"}),
		socialhub.WithClock(&mutableClock{value: testNow}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), testAccountID)
	if err != nil {
		t.Fatal(err)
	}
	return adapter, common.(*Client)
}

func assertAPIRequest(t *testing.T, request *http.Request, method, path string) {
	t.Helper()
	if request.Method != method || request.URL.Path != path {
		t.Fatalf("request=%s %s", request.Method, request.URL.Path)
	}
	if request.Header.Get("Authorization") != "Bearer "+testAccessToken {
		t.Errorf("Authorization=%q", request.Header.Get("Authorization"))
	}
	if request.URL.Query().Get("access_token") != "" {
		t.Errorf("access token leaked into query")
	}
}

func writeJSON(t *testing.T, writer http.ResponseWriter, status int, value any) {
	t.Helper()
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	if err := json.NewEncoder(writer).Encode(value); err != nil {
		t.Fatal(err)
	}
}

func propertyName() string { return "properties/" + testPropertyID }

func reportRequest() RunReportRequest {
	return RunReportRequest{
		DateRanges: []DateRange{{StartDate: "7daysAgo", EndDate: "yesterday"}},
		Dimensions: []Dimension{{Name: "country"}}, Metrics: []Metric{{Name: "eventCount"}},
		Limit: 100, ReturnPropertyQuota: true,
	}
}

func reportFixture(kind string, dimensions ...string) map[string]any {
	dimensionHeaders := make([]map[string]string, len(dimensions))
	dimensionValues := make([]map[string]string, len(dimensions))
	for index, name := range dimensions {
		dimensionHeaders[index] = map[string]string{"name": name}
		dimensionValues[index] = map[string]string{"value": "value-" + name}
	}
	return map[string]any{
		"dimensionHeaders": dimensionHeaders,
		"metricHeaders":    []any{map[string]any{"name": "eventCount", "type": MetricTypeInteger}},
		"rows":             []any{map[string]any{"dimensionValues": dimensionValues, "metricValues": []any{map[string]string{"value": "1"}}}},
		"rowCount":         1,
		"metadata":         map[string]any{"currencyCode": "USD", "timeZone": "Asia/Shanghai"},
		"propertyQuota":    map[string]any{"tokensPerDay": map[string]int{"consumed": 1, "remaining": 199999}},
		"kind":             kind,
	}
}

func pivotRequest() RunPivotReportRequest {
	return RunPivotReportRequest{
		DateRanges: []DateRange{{StartDate: "2026-08-01", EndDate: "2026-08-09"}},
		Dimensions: []Dimension{{Name: "country"}, {Name: "eventName"}}, Metrics: []Metric{{Name: "eventCount"}},
		Pivots:              []Pivot{{FieldNames: []string{"country"}, Limit: 10}, {FieldNames: []string{"eventName"}, Limit: 10}},
		ReturnPropertyQuota: true,
	}
}

func pivotFixture() map[string]any {
	return map[string]any{
		"pivotHeaders": []any{
			map[string]any{"pivotDimensionHeaders": []any{map[string]any{"dimensionValues": []any{map[string]string{"value": "US"}}}}, "rowCount": 1},
			map[string]any{"pivotDimensionHeaders": []any{map[string]any{"dimensionValues": []any{map[string]string{"value": "page_view"}}}}, "rowCount": 1},
		},
		"dimensionHeaders": []any{map[string]string{"name": "country"}, map[string]string{"name": "eventName"}},
		"metricHeaders":    []any{map[string]any{"name": "eventCount", "type": MetricTypeInteger}},
		"rows": []any{map[string]any{
			"dimensionValues": []any{map[string]string{"value": "US"}, map[string]string{"value": "page_view"}},
			"metricValues":    []any{map[string]string{"value": "1"}},
		}},
		"metadata":      map[string]any{"currencyCode": "USD"},
		"propertyQuota": map[string]any{"tokensPerHour": map[string]int{"consumed": 1, "remaining": 39999}},
		"kind":          "analyticsData#runPivotReport",
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

func cloneMap(input map[string]any) map[string]any {
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

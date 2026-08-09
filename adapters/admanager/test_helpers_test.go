package admanager

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
	testAccountID   socialhub.AccountID = "ad-manager-main"
	testNetworkCode                     = "123456"
	testCompanyID                       = "101"
	testAdUnitID                        = "202"
	testOrderID                         = "303"
	testLineItemID                      = "404"
	testReportID                        = "505"
	testOperationID                     = "606"
	testResultID                        = "707"
	testAccessToken                     = "ad-manager-access-token"
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
			ID: testAccountID, ClientID: "client-id", SecretRef: "test://client-secret",
			AccessTokenRef: "test://access-token", Approval: socialhub.ApprovalConfig{Scopes: []string{fullScope}},
			Settings: map[string]any{"network_code": testNetworkCode},
		}},
	}
}

func managedConfig(baseURL string) socialhub.AdapterConfig {
	config := staticConfig(baseURL)
	config.Accounts[0].AccessTokenRef = ""
	config.Accounts[0].Settings["refresh_token_ref"] = "test://refresh-token"
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

func assertAPIRequest(t *testing.T, request *http.Request) {
	t.Helper()
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

func decodeJSONBody(t *testing.T, request *http.Request, target any) {
	t.Helper()
	defer request.Body.Close()
	if request.Header.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type=%q", request.Header.Get("Content-Type"))
	}
	if err := json.NewDecoder(request.Body).Decode(target); err != nil {
		t.Fatal(err)
	}
}

func networkName() string                     { return "networks/" + testNetworkCode }
func resourceName(resource, id string) string { return networkName() + "/" + resource + "/" + id }
func operationName() string                   { return networkName() + "/operations/reports/runs/" + testOperationID }
func resultName() string                      { return resourceName("reports", testReportID) + "/results/" + testResultID }

func networkResource() Network {
	return Network{Name: networkName(), NetworkCode: testNetworkCode, DisplayName: "Publisher", CurrencyCode: "USD", TimeZone: "America/New_York"}
}

func companyResource() Company {
	return Company{Name: resourceName("companies", testCompanyID), CompanyID: testCompanyID, DisplayName: "Advertiser", Type: CompanyAdvertiser}
}

func adUnitResource() AdUnit {
	return AdUnit{Name: resourceName("adUnits", testAdUnitID), AdUnitID: testAdUnitID, DisplayName: "Homepage", ParentAdUnit: resourceName("adUnits", "1"), Status: AdUnitActive}
}

func orderResource() Order {
	return Order{Name: resourceName("orders", testOrderID), OrderID: testOrderID, DisplayName: "Autumn order", Advertiser: resourceName("companies", testCompanyID), Status: OrderApproved}
}

func lineItemResource() LineItem {
	return LineItem{Name: resourceName("lineItems", testLineItemID), Order: resourceName("orders", testOrderID), DisplayName: "Homepage standard", Status: "DELIVERING"}
}

func reportDefinition() ReportDefinition {
	return ReportDefinition{
		Dimensions: []Dimension{DimensionLineItemName, DimensionLineItemID},
		Metrics:    []Metric{MetricAdServerImpressions},
		DateRange:  DateRange{Relative: RelativeYesterday}, ReportType: ReportHistorical,
	}
}

func reportResource() Report {
	return Report{Name: resourceName("reports", testReportID), ReportID: testReportID, DisplayName: "Yesterday delivery", ReportDefinition: reportDefinition(), Visibility: ReportHidden}
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

package admob

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
	testAccountID      socialhub.AccountID = "admob-main"
	testPublisherID                        = "pub-123456"
	testAppFragment                        = "111"
	testAdUnitFragment                     = "222"
	testAccessToken                        = "admob-access-token"
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
			ID: testAccountID, ClientID: "client-id", SecretRef: "test://client-secret",
			AccessTokenRef: "test://access-token", Approval: socialhub.ApprovalConfig{Scopes: []string{readOnlyScope}},
			Settings: map[string]any{"publisher_id": testPublisherID},
		}},
	}
}

func managedConfig(baseURL string) socialhub.AdapterConfig {
	config := staticConfig(baseURL)
	config.Accounts[0].AccessTokenRef = ""
	config.Accounts[0].Settings["refresh_token_ref"] = "test://refresh-token"
	return config
}

func newStaticClient(t *testing.T, server *httptest.Server, mutate ...func(*socialhub.AdapterConfig)) (*Adapter, *Client) {
	t.Helper()
	config := staticConfig(server.URL)
	for _, change := range mutate {
		change(&config)
	}
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), config,
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
		t.Errorf("request=%s %s want=%s %s", request.Method, request.URL.Path, method, path)
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

func accountName() string { return "accounts/" + testPublisherID }
func appID() string       { return "ca-app-" + testPublisherID + "~" + testAppFragment }
func adUnitID() string    { return "ca-app-" + testPublisherID + "/" + testAdUnitFragment }
func appName() string     { return accountName() + "/apps/" + testAppFragment }
func adUnitName() string  { return accountName() + "/adUnits/" + testAdUnitFragment }

func validDateFixture() Date { return Date{Year: 2026, Month: 8, Day: 9} }

func accountFixture() PublisherAccount {
	return PublisherAccount{Name: accountName(), PublisherID: testPublisherID, ReportingTimeZone: "Asia/Shanghai", CurrencyCode: "USD"}
}

func appFixture() App {
	return App{
		Name: appName(), AppID: appID(), Platform: AppPlatformAndroid,
		ManualAppInfo: &ManualAppInfo{DisplayName: "Social Hub"}, AppApprovalState: AppApprovalApproved,
	}
}

func adUnitFixture() AdUnit {
	return AdUnit{
		Name: adUnitName(), AdUnitID: adUnitID(), AppID: appID(), DisplayName: "Home Banner",
		AdFormat: AdFormatBanner, AdTypes: []AdType{AdTypeRichMedia},
	}
}

func validNetworkSpec() NetworkReportSpec {
	return NetworkReportSpec{
		DateRange:  DateRange{StartDate: validDateFixture(), EndDate: validDateFixture()},
		Dimensions: []Dimension{DimensionDate}, Metrics: []Metric{MetricClicks, MetricEstimatedEarnings},
		LocalizationSettings: &LocalizationSettings{CurrencyCode: "USD", LanguageCode: "en-US"},
		TimeZone:             "America/Los_Angeles", MaxReportRows: 10,
	}
}

func validMediationSpec() MediationReportSpec {
	return MediationReportSpec{
		DateRange:  DateRange{StartDate: validDateFixture(), EndDate: validDateFixture()},
		Dimensions: []Dimension{DimensionAdSource}, Metrics: []Metric{MetricImpressions, MetricObservedECPM},
		DimensionFilters: []DimensionFilter{{Dimension: DimensionCountry, MatchesAny: StringList{Values: []string{"US"}}}},
		SortConditions:   []SortCondition{{Metric: MetricObservedECPM, Order: SortDescending}},
		MaxReportRows:    10,
	}
}

func reportResponse(dimensions []Dimension, metrics []Metric) []any {
	dimensionValues := make(map[string]any, len(expectedDimensions(dimensions)))
	for _, dimension := range expectedDimensions(dimensions) {
		dimensionValues[string(dimension)] = map[string]any{"value": "value-1", "displayLabel": "Label"}
	}
	metricValues := make(map[string]any, len(metrics))
	for _, metric := range metrics {
		switch metricKind(metric) {
		case "integer":
			metricValues[string(metric)] = map[string]any{"integerValue": "5"}
		case "micros":
			metricValues[string(metric)] = map[string]any{"microsValue": "6500000"}
		case "double":
			metricValues[string(metric)] = map[string]any{"doubleValue": 0.5}
		}
	}
	return []any{
		map[string]any{"header": map[string]any{
			"dateRange":            DateRange{StartDate: validDateFixture(), EndDate: validDateFixture()},
			"localizationSettings": LocalizationSettings{CurrencyCode: "USD", LanguageCode: "en-US"},
			"reportingTimeZone":    "America/Los_Angeles",
		}},
		map[string]any{"row": map[string]any{"dimensionValues": dimensionValues, "metricValues": metricValues}},
		map[string]any{"footer": map[string]any{"matchingRowCount": "1"}},
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

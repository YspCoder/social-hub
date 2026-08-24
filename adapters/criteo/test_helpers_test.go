package criteo

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
	testAccountID    socialhub.AccountID = "criteo-primary"
	testAdvertiserID                     = "12345"
	testCampaignID                       = "2001"
	testAdSetID                          = "3001"
	testDatasetID                        = "4001"
	testAccessToken                      = "criteo-access-token"
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
		Settings: map[string]any{"base_url": baseURL, "token_url": baseURL + "/oauth2/token"},
		Accounts: []socialhub.AccountConfig{{
			ID: testAccountID, AccessTokenRef: "test://access-token",
			Approval: socialhub.ApprovalConfig{Scopes: []string{"MarketingSolutions_Campaign_Read"}},
			Settings: map[string]any{"advertiser_id": testAdvertiserID},
		}},
	}
}

func managedConfig(baseURL string) socialhub.AdapterConfig {
	config := staticConfig(baseURL)
	config.Accounts[0].AccessTokenRef = ""
	config.Accounts[0].ClientID = "client-id"
	config.Accounts[0].SecretRef = "test://client-secret"
	return config
}

func newStaticClient(t *testing.T, server *httptest.Server) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), staticConfig(server.URL),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{"test://access-token": testAccessToken}),
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
	if request.Header.Get("Accept") != "application/json" {
		t.Errorf("Accept=%q", request.Header.Get("Accept"))
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

func campaignResource(id, advertiserID, name string) map[string]any {
	return map[string]any{
		"type": "Campaign", "id": id,
		"attributes": map[string]any{
			"id": id, "advertiserId": advertiserID, "name": name, "goal": "acquisition",
			"spendLimit": map[string]any{
				"spendLimitType": "capped", "spendLimitAmount": map[string]any{"value": 100.0},
				"spendLimitRenewal": "monthly",
			},
			"scheduledSpendLimits": []any{},
			"budgetAutomation": map[string]any{
				"enabled":                      true,
				"automatedBudgetConfiguration": map[string]any{"adSetOptimizationObjective": "conversions"},
			},
		},
	}
}

func adSetResource(id, advertiserID, campaignID, name string, activation ActivationStatus, delivery DeliveryStatus) map[string]any {
	return map[string]any{
		"type": "AdSet", "id": id,
		"attributes": map[string]any{
			"advertiserId": advertiserID, "campaignId": campaignID, "datasetId": testDatasetID,
			"name": name, "objective": "visits", "mediaType": "display",
			"attributionConfiguration": map[string]any{"attributionMethod": "criteoAttribution", "lookbackWindow": "30D"},
			"bidding":                  map[string]any{"costController": "maxCPC", "bidAmount": 1.25},
			"budget": map[string]any{
				"budgetStrategy": "capped", "budgetAmount": 50.0, "budgetRenewal": "daily",
				"budgetDeliverySmoothing": "standard", "budgetDeliveryWeek": "undefined",
			},
			"schedule": map[string]any{
				"activationStatus": activation, "deliveryStatus": delivery,
				"startDate": map[string]any{"value": "2026-08-10T00:00:00Z"}, "endDate": map[string]any{"value": nil},
			},
			"targeting": map[string]any{
				"deliveryLimitations": map[string]any{"devices": []string{"desktop"}},
				"frequencyCapping":    map[string]any{"frequency": "daily", "maximumImpressions": 3},
			},
		},
	}
}

func successEnvelope(data any) map[string]any {
	return map[string]any{"data": data, "errors": []any{}, "warnings": []any{}}
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

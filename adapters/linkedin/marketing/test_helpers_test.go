package marketing

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
	testAdAccountID     = "511183580"
	testCampaignGroupID = "603407684"
	testCampaignID      = "145282384"
	testCreativeID      = "urn:li:sponsoredCreative:120493375"
)

var testNow = time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC)

type mapResolver map[string]string

func (resolver mapResolver) Resolve(_ context.Context, reference string) (string, error) {
	value, found := resolver[reference]
	if !found {
		return "", errors.New("missing test secret")
	}
	return value, nil
}

type fixedClock struct{ value time.Time }

func (clock fixedClock) Now() time.Time { return clock.value }

func testConfig(baseURL string) socialhub.AdapterConfig {
	return socialhub.AdapterConfig{
		Adapter: adapterName, Product: productName,
		Settings: map[string]any{"base_url": baseURL, "auth_url": baseURL + "/authorize", "token_url": baseURL + "/token"},
		Accounts: []socialhub.AccountConfig{{
			ID: "b2b-demand", ClientID: "linkedin-client-id",
			SecretRef: "test://client-secret", AccessTokenRef: "test://access-token",
			Approval: socialhub.ApprovalConfig{Scopes: []string{readAdsScope, writeAdsScope, reportingAdsScope}},
			Settings: map[string]any{"ad_account_id": testAdAccountID},
		}},
	}
}

func newTestAdapter(t *testing.T, server *httptest.Server) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server.URL),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{
			"test://client-secret": "client-secret", "test://access-token": "access-token",
		}),
		socialhub.WithClock(fixedClock{value: testNow}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "b2b-demand")
	if err != nil {
		t.Fatal(err)
	}
	return adapter, common.(*Client)
}

func assertAPIRequest(t *testing.T, request *http.Request) {
	t.Helper()
	if request.Header.Get("Authorization") != "Bearer access-token" || request.URL.Query().Get("access_token") != "" {
		t.Fatalf("authentication=%v url=%s", request.Header, request.URL)
	}
	if request.Header.Get("Linkedin-Version") != marketingVersion || request.Header.Get("X-Restli-Protocol-Version") != restliVersion {
		t.Fatalf("LinkedIn headers=%v", request.Header)
	}
	if request.Method == http.MethodPost && request.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("Content-Type=%q", request.Header.Get("Content-Type"))
	}
}

func decodeJSONMap(t *testing.T, request *http.Request) map[string]any {
	t.Helper()
	decoder := json.NewDecoder(request.Body)
	decoder.UseNumber()
	var value map[string]any
	if err := decoder.Decode(&value); err != nil {
		t.Fatal(err)
	}
	return value
}

func writeValue(t *testing.T, writer http.ResponseWriter, status int, value any) {
	t.Helper()
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	if value != nil {
		if err := json.NewEncoder(writer).Encode(value); err != nil {
			t.Fatal(err)
		}
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

func validCampaignRequest() CreateCampaignRequest {
	return CreateCampaignRequest{
		CampaignGroupID: testCampaignGroupID, AssociatedEntityURN: "urn:li:organization:2414183",
		Name: "Launch campaign", Objective: ObjectiveBrandAwareness, CostType: CostCPM,
		DailyBudget: Money{Amount: "50", CurrencyCode: "USD"},
		TotalBudget: &Money{Amount: "500", CurrencyCode: "USD"},
		UnitCost:    Money{Amount: "15", CurrencyCode: "USD"},
		Locale:      Locale{Language: "en", Country: "US"},
		RunSchedule: RunSchedule{Start: 1786276800000, End: 1788955200000},
		TargetingCriteria: TargetingCriteria{Include: TargetingConjunction{And: []TargetingClause{
			{Or: map[string][]string{facetURNPrefix + "interfaceLocales": {"urn:li:locale:en_US"}}},
			{Or: map[string][]string{facetURNPrefix + "locations": {"urn:li:geo:103644278"}}},
		}}},
	}
}

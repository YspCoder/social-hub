package dv360

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
	testAccountID           socialhub.AccountID = "dv360-main"
	testAdvertiserID                            = "123"
	testPartnerID                               = "456"
	testCampaignID                              = "1001"
	testInsertionOrderID                        = "2001"
	testLineItemID                              = "3001"
	testDuplicateLineItemID                     = "3002"
	testAccessToken                             = "dv360-access-token"
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
		Settings: map[string]any{
			"base_url": baseURL, "auth_url": baseURL + "/authorize", "token_url": baseURL + "/token",
		},
		Accounts: []socialhub.AccountConfig{{
			ID: testAccountID, ClientID: "client-id", SecretRef: "test://client-secret",
			AccessTokenRef: "test://access-token",
			Approval:       socialhub.ApprovalConfig{Scopes: []string{displayVideoScope}},
			Settings: map[string]any{
				"advertiser_id": testAdvertiserID, "partner_id": testPartnerID,
			},
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
		socialhub.WithSecretResolver(mapResolver{
			"test://access-token": testAccessToken, "test://client-secret": "client-secret",
		}),
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

func advertiserResource(status EntityStatus) Advertiser {
	return Advertiser{
		Name: "advertisers/" + testAdvertiserID, AdvertiserID: testAdvertiserID,
		PartnerID: testPartnerID, DisplayName: "Brand", EntityStatus: status,
	}
}

func campaignResource(id, name string, status EntityStatus) Campaign {
	end := Date{Year: 2026, Month: 12, Day: 31}
	return Campaign{
		Name:         "advertisers/" + testAdvertiserID + "/campaigns/" + id,
		AdvertiserID: testAdvertiserID, CampaignID: id, DisplayName: name, EntityStatus: status,
		CampaignGoal: validTestCampaignGoal(),
		CampaignFlight: CampaignFlight{
			PlannedSpendAmountMicros: "500000000",
			PlannedDates:             DateRange{StartDate: Date{Year: 2026, Month: 8, Day: 10}, EndDate: &end},
		},
		FrequencyCap: FrequencyCap{Unlimited: true},
	}
}

func insertionOrderResource(id, campaignID, name string, status EntityStatus) InsertionOrder {
	end := Date{Year: 2026, Month: 12, Day: 31}
	return InsertionOrder{
		Name:         "advertisers/" + testAdvertiserID + "/insertionOrders/" + id,
		AdvertiserID: testAdvertiserID, CampaignID: campaignID, InsertionOrderID: id,
		DisplayName: name, EntityStatus: status, InsertionOrderType: InsertionOrderRTB,
		Pacing:       Pacing{Type: PacingEven, Period: PacingPeriodFlight},
		FrequencyCap: FrequencyCap{Unlimited: true},
		Budget: InsertionOrderBudget{
			BudgetUnit: BudgetUnitCurrency, AutomationType: InsertionOrderAutomationNone,
			BudgetSegments: []InsertionOrderBudgetSegment{{
				BudgetAmountMicros: "100000000",
				DateRange:          DateRange{StartDate: Date{Year: 2026, Month: 8, Day: 10}, EndDate: &end},
			}},
		},
		KPI: KPI{Type: KPICPM, AmountMicros: "1000000"}, OptimizationObjective: OptimizationBrandAwareness,
	}
}

func lineItemResource(id, name string, status EntityStatus) LineItem {
	return LineItem{
		Name:         "advertisers/" + testAdvertiserID + "/lineItems/" + id,
		AdvertiserID: testAdvertiserID, CampaignID: testCampaignID,
		InsertionOrderID: testInsertionOrderID, LineItemID: id, DisplayName: name,
		LineItemType: LineItemDisplayDefault, EntityStatus: status,
		Flight:              LineItemFlight{Type: LineItemFlightInherited},
		Budget:              LineItemBudget{AllocationType: LineItemBudgetFixed, BudgetUnit: BudgetUnitCurrency, MaxAmount: "50000000"},
		Pacing:              Pacing{Type: PacingEven, Period: PacingPeriodFlight},
		PartnerRevenueModel: PartnerRevenueModel{MarkupType: PartnerRevenueMediaCost, MarkupAmount: "0"},
		BidStrategy:         BiddingStrategy{FixedBid: &FixedBidStrategy{BidAmountMicros: "1000000"}},
		FrequencyCap:        FrequencyCap{Unlimited: true}, ContainsEUPoliticalAds: DoesNotContainEUPoliticalAdvertising,
	}
}

func validTestCampaignGoal() CampaignGoal {
	return CampaignGoal{
		Type:            CampaignGoalBrandAwareness,
		PerformanceGoal: PerformanceGoal{Type: PerformanceGoalCPM, AmountMicros: "1000000"},
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

func stringPointer(value string) *string { return &value }

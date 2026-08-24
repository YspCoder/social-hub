package ads

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
	testAdAccountID         = "a2_123456"
	testFundingInstrumentID = "604212"
	testCampaignID          = "579922433862993631"
	testAdGroupID           = "579922433862993632"
	testAdID                = "579922433862993633"
	testPostID              = "t3_abc123"
	testPixelID             = "p2_123456"
	testUserAgent           = "windows:social-hub:test (by /u/socialhubtest)"
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
		Settings: map[string]any{
			"base_url": baseURL + "/api/v3", "auth_url": baseURL + "/authorize",
			"token_url": baseURL + "/token", "user_agent": testUserAgent,
		},
		Accounts: []socialhub.AccountConfig{{
			ID: "paid-social", ClientID: "client-id", SecretRef: "test://client-secret",
			AccessTokenRef: "test://access-token",
			Approval:       socialhub.ApprovalConfig{Scopes: []string{readScope, editScope}},
			Settings:       map[string]any{"ad_account_id": testAdAccountID},
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
	common, err := adapter.Client(context.Background(), "paid-social")
	if err != nil {
		t.Fatal(err)
	}
	return adapter, common.(*Client)
}

func assertAPIRequest(t *testing.T, request *http.Request) {
	t.Helper()
	if request.Header.Get("Authorization") != "Bearer access-token" {
		t.Fatalf("Authorization=%q", request.Header.Get("Authorization"))
	}
	if request.Header.Get("User-Agent") != testUserAgent {
		t.Fatalf("User-Agent=%q", request.Header.Get("User-Agent"))
	}
	if request.URL.Query().Get("access_token") != "" {
		t.Fatalf("credential leaked in URL: %s", request.URL)
	}
}

func writeJSON(t *testing.T, writer http.ResponseWriter, status int, value any) {
	t.Helper()
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	if value != nil {
		if err := json.NewEncoder(writer).Encode(value); err != nil {
			t.Fatal(err)
		}
	}
}

func decodeJSONBody(t *testing.T, request *http.Request, target any) {
	t.Helper()
	defer request.Body.Close()
	if err := json.NewDecoder(request.Body).Decode(target); err != nil {
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

func int64Pointer(value int64) *int64 { return &value }

func boolPointer(value bool) *bool { return &value }

func statusPointer(value ConfiguredStatus) *ConfiguredStatus { return &value }

func bidStrategyPointer(value BidStrategy) *BidStrategy { return &value }

func stringPointer(value string) *string { return &value }

func campaignFixture(name string, status ConfiguredStatus) Campaign {
	return Campaign{
		ID: testCampaignID, AdAccountID: testAdAccountID, FundingInstrumentID: testFundingInstrumentID,
		Name: name, ConfiguredStatus: status, Objective: ObjectiveClicks,
		IsCampaignBudgetOptimization: boolPointer(false),
	}
}

func cboCampaignFixture() Campaign {
	value := campaignFixture("CBO Campaign", StatusPaused)
	value.IsCampaignBudgetOptimization = boolPointer(true)
	value.GoalType, value.GoalValue = GoalLifetimeSpend, int64Pointer(10_000_000)
	value.ConversionPixelID = testPixelID
	return value
}

func adGroupFixture(name string, status ConfiguredStatus) AdGroup {
	return AdGroup{
		ID: testAdGroupID, AdAccountID: testAdAccountID, CampaignID: testCampaignID,
		Name: name, ConfiguredStatus: status, BidType: BidTypeCPC,
		ConversionPixelID: testPixelID,
	}
}

func adFixture(name string, status ConfiguredStatus) Ad {
	return Ad{
		ID: testAdID, AdAccountID: testAdAccountID, AdGroupID: testAdGroupID, CampaignID: testCampaignID,
		Name: name, PostID: testPostID, ConfiguredStatus: status,
	}
}

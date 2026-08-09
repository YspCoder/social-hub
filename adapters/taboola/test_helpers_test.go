package taboola

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
	testAccountID    socialhub.AccountID = "taboola-primary"
	testAdvertiserID                     = "demo-advertiser"
	testCampaignID                       = "1001"
	testItemID                           = "2001"
	testAccessToken                      = "taboola-access-token"
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
		Settings: map[string]any{"base_url": baseURL + "/backstage/api/1.0", "token_url": baseURL + "/backstage/oauth/token"},
		Accounts: []socialhub.AccountConfig{{
			ID: testAccountID, AccessTokenRef: "secret://taboola-token",
			Settings: map[string]any{"advertiser_account_id": testAdvertiserID},
		}},
	}
}

func newTestAdapter(t *testing.T, server *httptest.Server) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server.URL),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{"secret://taboola-token": testAccessToken}),
		socialhub.WithClock(fixedClock{value: testNow}),
		socialhub.WithTokenStore(socialhub.NewMemoryTokenStore()),
	); err != nil {
		t.Fatal(err)
	}
	value, err := adapter.Client(context.Background(), testAccountID)
	if err != nil {
		t.Fatal(err)
	}
	client, ok := value.(*Client)
	if !ok {
		t.Fatalf("client type=%T", value)
	}
	return adapter, client
}

func assertAPIRequest(t *testing.T, request *http.Request) {
	t.Helper()
	if request.Header.Get("Authorization") != "Bearer "+testAccessToken {
		t.Fatalf("Authorization=%q", request.Header.Get("Authorization"))
	}
	if request.Header.Get("Accept") != "application/json" {
		t.Fatalf("Accept=%q", request.Header.Get("Accept"))
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

func decodeJSONBody(t *testing.T, request *http.Request) map[string]any {
	t.Helper()
	defer request.Body.Close()
	var value map[string]any
	if err := json.NewDecoder(request.Body).Decode(&value); err != nil {
		t.Fatal(err)
	}
	return value
}

func hubError(t *testing.T, err error) *socialhub.Error {
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

func boolPointer(value bool) *bool        { return &value }
func floatPointer(value float64) *float64 { return &value }
func stringPointer(value string) *string  { return &value }

func campaignFixture(active bool, status CampaignStatus) Campaign {
	return Campaign{
		ID: testCampaignID, AdvertiserID: testAdvertiserID, Name: "Campaign", BrandingText: "Brand",
		BidStrategy: BidStrategyFixed, MarketingObjective: "DRIVE_WEBSITE_TRAFFIC", CPC: floatPointer(0.25),
		SpendingLimit: floatPointer(100), SpendingLimitModel: SpendingMonthly,
		IsActive: boolPointer(active), Status: status,
	}
}

func itemFixture(active bool, status ItemStatus) CampaignItem {
	return CampaignItem{
		ID: testItemID, CampaignID: testCampaignID, Type: "ITEM", URL: "https://example.test/article",
		Title: "Item", ApprovalState: "APPROVED", IsActive: boolPointer(active), Status: status,
	}
}

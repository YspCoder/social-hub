package unityadvertising

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
	testAccountID      socialhub.AccountID = "unity-advertising-primary"
	testOrganizationID                     = "5772562874846"
	testCampaignSetID                      = "651bffb0c4466ba162a56531"
	testCampaignID                         = "651bffb0c4466ba162a56532"
	testCreativeID                         = "651bffb0c4466ba162a56533"
	testCreativePackID                     = "651bffb0c4466ba162a56534"
	testSourceAppID                        = "AbCdEf123456"
	testBearerToken                        = "unity-long-lived-bearer"
	testKeyID                              = "7e0f1152-e0dd-4b14-8e37-04cab07efeb0"
	testSecretKey                          = "unity-service-account-secret"
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

func testConfig(baseURL string) socialhub.AdapterConfig {
	return socialhub.AdapterConfig{
		Adapter: adapterName, Product: productName,
		Settings: map[string]any{"base_url": baseURL},
		Accounts: []socialhub.AccountConfig{{
			ID: testAccountID, AccessTokenRef: "secret://unity-bearer",
			Settings: map[string]any{"organization_id": testOrganizationID},
		}},
	}
}

func basicConfig(baseURL string) socialhub.AdapterConfig {
	config := testConfig(baseURL)
	config.Accounts[0].AccessTokenRef = ""
	config.Accounts[0].ClientID = testKeyID
	config.Accounts[0].SecretRef = "secret://unity-key"
	return config
}

func newTestAdapter(t *testing.T, server *httptest.Server) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server.URL),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{"secret://unity-bearer": testBearerToken}),
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

func assertBearerRequest(t *testing.T, request *http.Request) {
	t.Helper()
	if request.Header.Get("Authorization") != "Bearer "+testBearerToken {
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

func decodeJSONBody(t *testing.T, request *http.Request, target any) {
	t.Helper()
	defer request.Body.Close()
	if err := json.NewDecoder(request.Body).Decode(target); err != nil {
		t.Fatal(err)
	}
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

func cloneMap(input map[string]any) map[string]any {
	result := make(map[string]any, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

func stringPointer(value string) *string                                  { return &value }
func boolPointer(value bool) *bool                                        { return &value }
func moneyPointer(value Money) *Money                                     { return &value }
func bidPointer(value BidAmount) *BidAmount                               { return &value }
func roasGoalPointer(value ROASGoal) *ROASGoal                            { return &value }
func autoStartPointer(value AutoStart) *AutoStart                         { return &value }
func stringSlicePointer(value []string) *[]string                         { return &value }
func countryRegionPointer(value []RegionalTargeting) *[]RegionalTargeting { return &value }

func appFixture() App {
	storeID := "1358236"
	adomain := "example.test"
	gameID := int64(500269499)
	clickURL := "https://example.test/click"
	startURL := "https://example.test/start"
	return App{
		ID: testCampaignSetID, Name: "My Game", Store: StoreApple, StoreID: &storeID, ADomain: &adomain,
		GameID: &gameID, CreatedAt: &testNow, UpdatedAt: &testNow,
		AppAttributionClickURL: &clickURL, AppAttributionStartURL: &startURL,
	}
}

func creativeFixture() Creative {
	return Creative{
		ID: testCreativeID, Name: "Launch video", Language: LanguageEnglish, Type: CreativeLandscapeVideo,
		Files: []CreativePreviewFile{{Name: "launch.mp4", URL: "https://example.test/launch.mp4"}}, CreatedAt: &testNow, Status: CreativeApproved,
	}
}

func creativePackFixture() CreativePack {
	return CreativePack{
		ID: testCreativePackID, Name: "Launch pack", CreativeIDs: []string{testCreativeID},
		Type: CreativePackVideo, CampaignIDs: []string{testCampaignID},
	}
}

func campaignFixture() Campaign {
	daily := Money("500.00")
	return Campaign{
		ID: testCampaignID, Name: "Launch campaign", Goal: CampaignGoalInstalls, BillingType: BillingCPI,
		Enabled: true, ScheduleStart: "2026-08-10", CreatedAt: &testNow, UpdatedAt: &testNow,
		BiddingStrategy: BiddingManual, Budget: &CampaignBudget{Daily: &daily},
	}
}

func budgetFixture() CampaignBudget {
	total, dailySpent, spent, daily := Money("2500.00"), Money("25.00"), Money("50.00"), Money("500.00")
	return CampaignBudget{Total: &total, DailySpent: &dailySpent, Spent: &spent, Daily: &daily}
}

func targetingFixture() Targeting {
	apps := []string{testSourceAppID}
	limited := []LimitedAdTracking{UsersAllowingAdTracking}
	connections := []ConnectionType{ConnectionWiFi}
	sizes := []ScreenSize{ScreenNormal}
	densities := []ScreenDensity{DensityXHDPI}
	regions := []RegionalTargeting{{Country: "US", Subdivisions: []string{"5332921"}}}
	return Targeting{
		AppTargeting: &AppTargetingOptions{AllowList: &apps},
		DeviceTargeting: &DeviceTargetingOptions{
			LimitedAdTracking: &limited, ConnectionType: &connections, ScreenSize: &sizes, ScreenDensity: &densities,
		},
		RegionalTargeting: &regions,
	}
}

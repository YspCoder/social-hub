package unitypublisher

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
	testAccountID      socialhub.AccountID = "unity-publisher-primary"
	testOrganizationID                     = "3573617062594"
	testApplicationID                      = "5a8591dd-4039-49df-9202-96385ba3eff8"
	testProjectID                          = "68ea4233-d529-49d0-a484-0bb0525f1f91"
	testPlacementID                        = "550e8400-e29b-41d4-a716-446655440000"
	testDeviceID                           = "9b360ca8-fbf3-4da1-a73d-2df5e30a93f2"
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

func stringPointer(value string) *string { return &value }
func boolPointer(value bool) *bool       { return &value }

func applicationFixture() Application {
	gameID := int64(12345)
	iconURL := "https://example.test/icon.png"
	storeID := "com.example.game"
	store := StoreGooglePlay
	mode := TestModeForceAll
	coppa, mixed := false, false
	return Application{
		ID: testApplicationID, Name: "My Game", GameID: &gameID, Platform: PlatformAndroid,
		IconURL: &iconURL, ProjectID: stringPointer(testProjectID), StoreID: &storeID, Store: &store,
		TestMode: &mode, KidsSettings: false, COPPA: &coppa, MixedAudience: &mixed,
	}
}

func applicationTestModeFixture() ApplicationTestMode {
	gameID := int64(12345)
	mode := TestModeForceAll
	return ApplicationTestMode{ID: testApplicationID, GameID: &gameID, TestMode: &mode}
}

func placementFixture() Placement {
	gameID := int64(12345)
	return Placement{
		ID: testPlacementID, Key: "Rewarded_Placement", Name: "Rewarded Placement", GameID: &gameID,
		AdFormat: AdFormatRewarded, Status: PlacementActive,
		AdFormatConfigurations: json.RawMessage(`{"name":"coins","value":100}`),
		ApplicationID:          testApplicationID, CreatedAt: testNow, UpdatedAt: testNow,
	}
}

func organizationPlacementFixture() OrganizationPlacement {
	gameID := int64(12345)
	storeID := "com.example.game"
	return OrganizationPlacement{
		PlacementID: "Rewarded_Placement", Name: "Rewarded Placement", PlacementType: "bidding",
		GameID: &gameID, AdFormat: AdFormatRewarded, Platform: OrganizationPlatformAndroid, StoreID: &storeID,
	}
}

func testDeviceFixture() TestDevice {
	platform := PlatformIOS
	return TestDevice{ID: testDeviceID, Platform: &platform, Name: "QA iPhone 15", AdvertisingID: "AEBE52E7-03EE-455A-B3C4-E57283966239"}
}

func cloneMap(input map[string]any) map[string]any {
	result := make(map[string]any, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

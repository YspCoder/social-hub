package cm360

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
	testAccountID    socialhub.AccountID = "cm360-main"
	testProfileID                        = "111"
	testAdvertiserID                     = "222"
	testCampaignID                       = "333"
	testPlacementID                      = "444"
	testAdID                             = "555"
	testReportID                         = "666"
	testFileID                           = "777"
	testAccessToken                      = "cm360-access-token"
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
			Approval:       socialhub.ApprovalConfig{Scopes: []string{traffickingScope, reportingScope}},
			Settings: map[string]any{
				"profile_id": testProfileID, "advertiser_id": testAdvertiserID,
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

func profileResource() UserProfile {
	return UserProfile{ProfileID: testProfileID, AccountID: "888", AccountName: "Agency", UserName: "operator@example.com"}
}

func advertiserResource() Advertiser {
	return Advertiser{ID: testAdvertiserID, AccountID: "888", Name: "Brand", Status: "APPROVED", FloodlightConfigurationID: "999"}
}

func campaignResource(id, name string, archived bool) Campaign {
	return Campaign{
		ID: id, AccountID: "888", AdvertiserID: testAdvertiserID, Name: name, Archived: archived,
		StartDate: "2026-08-10", EndDate: "2026-12-31", Comment: "launch",
	}
}

func placementResource() Placement {
	return Placement{
		ID: testPlacementID, AccountID: "888", AdvertiserID: testAdvertiserID, CampaignID: testCampaignID,
		SiteID: "1234", Name: "Homepage", ActiveStatus: PlacementActive,
		StartDate: "2026-08-10", EndDate: "2026-12-31", Size: &Size{ID: "1", Width: 300, Height: 250},
	}
}

func adResource() Ad {
	return Ad{
		ID: testAdID, AccountID: "888", AdvertiserID: testAdvertiserID, CampaignID: testCampaignID,
		Name: "Standard ad", Type: AdStandard, Active: true,
	}
}

func reportResource() Report {
	return Report{ID: testReportID, AccountID: "888", OwnerProfileID: testProfileID, Name: "Daily delivery", Type: "STANDARD", Format: "CSV"}
}

func reportFileResource(status ReportFileStatus) ReportFile {
	return ReportFile{
		ID: testFileID, ReportID: testReportID, FileName: "daily.csv", Format: "CSV", Status: status,
		DateRange: DateRange{StartDate: "2026-08-01", EndDate: "2026-08-08"},
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

package appleads

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	testOrgID      int64 = 12345
	testCampaignID int64 = 2001
	testAdGroupID  int64 = 3001
	testKeywordID  int64 = 4001
	testCreativeID int64 = 5001
	testAdID       int64 = 6001
	testAdamID     int64 = 987654321
)

var testNow = time.Date(2026, time.August, 9, 1, 2, 3, 0, time.UTC)

type fixedClock struct{ now time.Time }

func (clock fixedClock) Now() time.Time { return clock.now }

type mapResolver map[string]string

func (resolver mapResolver) Resolve(_ context.Context, reference string) (string, error) {
	value, found := resolver[reference]
	if !found {
		return "", errors.New("missing test secret")
	}
	return value, nil
}

func staticConfig(serverURL string) socialhub.AdapterConfig {
	return socialhub.AdapterConfig{
		Adapter: adapterName,
		Product: productName,
		Settings: map[string]any{
			"base_url":  serverURL + "/api/v5",
			"token_url": serverURL + "/token",
		},
		Accounts: []socialhub.AccountConfig{{
			ID:             "search-us",
			AccessTokenRef: "test://access-token",
			Settings:       map[string]any{"org_id": testOrgID},
		}},
	}
}

func managedConfig(serverURL string) socialhub.AdapterConfig {
	config := staticConfig(serverURL)
	config.Accounts[0].AccessTokenRef = ""
	config.Accounts[0].ClientID = "client-id"
	config.Accounts[0].Settings = map[string]any{
		"org_id": testOrgID, "team_id": "team-id", "key_id": "key-id",
		"private_key_ref": "test://private-key",
	}
	return config
}

func newStaticClient(t *testing.T, server *httptest.Server) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), staticConfig(server.URL),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithClock(fixedClock{now: testNow}),
		socialhub.WithSecretResolver(mapResolver{"test://access-token": "access-token"}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "search-us")
	if err != nil {
		t.Fatal(err)
	}
	return adapter, common.(*Client)
}

func generatePrivateKey(t *testing.T) (*ecdsa.PrivateKey, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return key, string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded}))
}

func assertAPIRequest(t *testing.T, request *http.Request) {
	t.Helper()
	if request.Header.Get("Authorization") != "Bearer access-token" ||
		request.Header.Get("X-AP-Context") != "orgId=12345" ||
		request.URL.Query().Get("access_token") != "" {
		t.Errorf("unexpected API authentication: %s %s headers=%v", request.Method, request.URL, request.Header)
	}
}

func decodeRequest(t *testing.T, request *http.Request, target any) {
	t.Helper()
	if request.Header.Get("Content-Type") != "application/json" {
		t.Errorf("content type=%q", request.Header.Get("Content-Type"))
	}
	if err := json.NewDecoder(request.Body).Decode(target); err != nil {
		t.Errorf("decode request: %v", err)
	}
}

func writeJSON(t *testing.T, writer http.ResponseWriter, status int, value any) {
	t.Helper()
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	if err := json.NewEncoder(writer).Encode(value); err != nil {
		t.Errorf("encode response: %v", err)
	}
}

func envelope(data any) map[string]any {
	return map[string]any{"data": data, "pagination": PageDetail{}, "error": nil}
}

func pagedEnvelope(data any, total int64) map[string]any {
	return map[string]any{
		"data":       data,
		"pagination": PageDetail{TotalResults: total, StartIndex: 0, ItemsPerPage: 1},
		"error":      nil,
	}
}

func testCampaign(status CampaignStatus) Campaign {
	return Campaign{ID: testCampaignID, OrgID: testOrgID, AdamID: testAdamID, Name: "Search", Status: status}
}

func testAdGroup(status AdGroupStatus) AdGroup {
	return AdGroup{ID: testAdGroupID, OrgID: testOrgID, CampaignID: testCampaignID, Name: "Core", Status: status}
}

func testKeyword(status KeywordStatus) Keyword {
	return Keyword{ID: testKeywordID, CampaignID: testCampaignID, AdGroupID: testAdGroupID, Text: "travel", MatchType: MatchBroad, Status: status}
}

func testCreative(state string) Creative {
	return Creative{
		ID: testCreativeID, OrgID: testOrgID, AdamID: testAdamID, Name: "Product page",
		Type: CreativeCustomProductPage, ProductPageID: "45812c9b-c296-43d3-c6a0-c5a02f74bf6e", State: state,
	}
}

func testAd(status AdStatus) Ad {
	return Ad{
		ID: testAdID, OrgID: testOrgID, CampaignID: testCampaignID, AdGroupID: testAdGroupID,
		CreativeID: testCreativeID, Name: "Product page ad", CreativeType: CreativeCustomProductPage, Status: status,
	}
}

func requireHubError(t *testing.T, err error) *socialhub.Error {
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

func stringPointer(value string) *string { return &value }

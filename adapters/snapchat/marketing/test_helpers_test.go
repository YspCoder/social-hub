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
	testAdAccountID = "11111111-1111-4111-8111-111111111111"
	testCampaignID  = "22222222-2222-4222-8222-222222222222"
	testAdSquadID   = "33333333-3333-4333-8333-333333333333"
	testAdID        = "44444444-4444-4444-8444-444444444444"
	testCreativeID  = "55555555-5555-4555-8555-555555555555"
	otherAccountID  = "99999999-9999-4999-8999-999999999999"
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
			ID: "paid-social", ClientID: "snap-client-id",
			SecretRef: "test://client-secret", AccessTokenRef: "test://access-token",
			Approval: socialhub.ApprovalConfig{Scopes: []string{marketingScope}},
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
	common, err := adapter.Client(context.Background(), "paid-social")
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
	if request.Method == http.MethodPost && request.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("POST Content-Type=%q", request.Header.Get("Content-Type"))
	}
	if request.Method == http.MethodPatch && request.Header.Get("Content-Type") != "application/json-patch+json" {
		t.Fatalf("PATCH Content-Type=%q", request.Header.Get("Content-Type"))
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

func decodePatch(t *testing.T, request *http.Request) []map[string]any {
	t.Helper()
	var value []map[string]any
	if err := json.NewDecoder(request.Body).Decode(&value); err != nil {
		t.Fatal(err)
	}
	if len(value) == 0 || len(value) > 2 {
		t.Fatalf("patch=%v", value)
	}
	for _, operation := range value {
		if operation["op"] != "replace" || operation["path"] != "/name" && operation["path"] != "/status" {
			t.Fatalf("patch operation=%v", operation)
		}
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

func campaignValue(accountID string, status EntityStatus, name string) map[string]any {
	return map[string]any{
		"id": testCampaignID, "ad_account_id": accountID, "name": name, "status": status,
		"buy_model": "AUCTION", "creation_state": "PUBLISHED",
		"objective_v2_properties": map[string]any{"objective_v2_type": ObjectiveAwarenessAndEngagement},
	}
}

func adSquadValue(status EntityStatus, name string) map[string]any {
	return map[string]any{
		"id": testAdSquadID, "campaign_id": testCampaignID, "name": name, "status": status,
		"type": "SNAP_ADS", "placement_v2": map[string]any{"config": "AUTOMATIC"},
	}
}

func adValue(status EntityStatus, name string) map[string]any {
	return map[string]any{
		"id": testAdID, "ad_squad_id": testAdSquadID, "creative_id": testCreativeID,
		"name": name, "status": status, "type": "SNAP_AD",
	}
}

func successEnvelope(resource string, key string, value any) map[string]any {
	return map[string]any{
		"request_status": "SUCCESS", "request_id": "snap-request",
		resource: []any{map[string]any{"sub_request_status": "SUCCESS", key: value}},
	}
}

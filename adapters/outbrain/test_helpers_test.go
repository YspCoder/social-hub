package outbrain

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
	testAccountID      socialhub.AccountID = "outbrain-primary"
	testMarketerID                         = "00f4b02153ee75f3c9dc4fc128ab041962"
	testBudgetID                           = "004177467f7724bacae68b46524e3c6c3b"
	testCampaignID                         = "00f346fb73eefe9cdc93477727c6ee501d"
	testPromotedLinkID                     = "00a9c284d1874746b611a82198aa23983"
	testAccessToken                        = "outbrain-access-token"
	testUsername                           = "api-user@example.test"
	testPassword                           = "correct-horse-battery-staple"
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

func testConfig(baseURL string) socialhub.AdapterConfig {
	return socialhub.AdapterConfig{
		Adapter: adapterName, Product: productName,
		Settings: map[string]any{"base_url": baseURL},
		Accounts: []socialhub.AccountConfig{{
			ID: testAccountID, AccessTokenRef: "secret://outbrain-token",
			Settings: map[string]any{"marketer_id": testMarketerID},
		}},
	}
}

func loginConfig(baseURL string) socialhub.AdapterConfig {
	config := testConfig(baseURL)
	config.Accounts[0].AccessTokenRef = ""
	config.Accounts[0].SecretRef = "secret://outbrain-password"
	config.Accounts[0].Settings["username"] = testUsername
	return config
}

func newTestAdapter(t *testing.T, server *httptest.Server) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server.URL),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{"secret://outbrain-token": testAccessToken}),
		socialhub.WithClock(&mutableClock{value: testNow}),
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
	if request.Header.Get("OB-TOKEN-V1") != testAccessToken {
		t.Fatalf("OB-TOKEN-V1=%q", request.Header.Get("OB-TOKEN-V1"))
	}
	if request.Header.Get("Authorization") != "" {
		t.Fatalf("Authorization unexpectedly set: %q", request.Header.Get("Authorization"))
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

func boolPointer(value bool) *bool               { return &value }
func floatPointer(value float64) *float64        { return &value }
func stringPointer(value string) *string         { return &value }
func pacingPointer(value PacingType) *PacingType { return &value }

func marketerFixture() Marketer {
	return Marketer{ID: testMarketerID, Name: "Demo Marketer", Enabled: true, CreationTime: "2026-01-01 00:00:00", LastModified: "2026-08-09 00:00:00"}
}

func budgetFixture() Budget {
	return Budget{
		ID: testBudgetID, Name: "Campaign Budget", Amount: 100, Currency: "USD", AmountRemaining: 100,
		StartDate: "2026-08-01", EndDate: "2026-08-03", Type: BudgetCampaign, Pacing: PacingSpendASAP,
	}
}

func campaignFixture(enabled bool) Campaign {
	return Campaign{
		ID: testCampaignID, MarketerID: testMarketerID, Name: "Campaign", Enabled: enabled, CPC: 0.25,
		MinimumCPC: 0.02, Currency: "USD", Budget: budgetFixture(), LiveStatus: LiveStatus{CampaignOnAir: false},
	}
}

func promotedLinkFixture(enabled bool, approved bool) PromotedLink {
	status := "Pending"
	wireStatus := "PENDING"
	if approved {
		status, wireStatus = "Approved", "APPROVED"
	}
	return PromotedLink{
		ID: testPromotedLinkID, CampaignID: testCampaignID, Text: "A useful headline",
		URL: "https://example.test/article", Status: wireStatus, ApprovalStatus: ApprovalStatus{Status: status},
		Enabled: enabled, ImageMetadata: ImageMetadata{RequestedImageURL: "https://example.test/image.jpg"},
	}
}

func cloneMap(input map[string]any) map[string]any {
	result := make(map[string]any, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

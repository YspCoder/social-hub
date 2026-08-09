package ads

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	testAdsAccountID        = "18ce54d4x5t"
	testFundingInstrumentID = "lygyi"
	testCampaignID          = "hwtq0"
	testLineItemID          = "8v7jo"
	testPromotedTweetID     = "1e8i2k"
	testTweetID             = "822333526255120384"
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
			"base_url": baseURL + "/12", "request_token_url": baseURL + "/oauth/request_token",
			"authorize_url": baseURL + "/oauth/authorize", "access_token_url": baseURL + "/oauth/access_token",
		},
		Accounts: []socialhub.AccountConfig{{
			ID: "paid-social", ClientID: "consumer-key", SecretRef: "test://consumer-secret",
			AccessTokenRef: "test://access-token", Approval: socialhub.ApprovalConfig{AccountType: standardAccess},
			Settings: map[string]any{
				"ads_account_id": testAdsAccountID, "access_token_secret_ref": "test://access-token-secret",
			},
		}},
	}
}

func newTestAdapter(t *testing.T, server *httptest.Server) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server.URL),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{
			"test://consumer-secret": "consumer-secret", "test://access-token": "access-token",
			"test://access-token-secret": "access-token-secret",
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
	authorization := request.Header.Get("Authorization")
	if !strings.HasPrefix(authorization, "OAuth ") || !strings.Contains(authorization, `oauth_consumer_key="consumer-key"`) ||
		!strings.Contains(authorization, `oauth_token="access-token"`) || strings.Contains(authorization, "consumer-secret") ||
		strings.Contains(authorization, "access-token-secret") {
		t.Fatalf("Authorization=%q", authorization)
	}
	for _, name := range []string{"oauth_token", "access_token", "consumer_secret", "oauth_token_secret"} {
		if request.URL.Query().Get(name) != "" {
			t.Fatalf("credential leaked in URL: %s", request.URL)
		}
	}
	if request.Body != nil && request.ContentLength > 0 {
		t.Fatalf("X Ads parameters must be query encoded: content-length=%d", request.ContentLength)
	}
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

func int64Pointer(value int64) *int64 { return &value }

func statusPointer(value EntityStatus) *EntityStatus { return &value }

func stringPointer(value string) *string { return &value }

func campaignFixture(name string, status EntityStatus) Campaign {
	return Campaign{
		ID: testCampaignID, AccountID: testAdsAccountID, FundingInstrumentID: testFundingInstrumentID,
		Name: name, EntityStatus: status, BudgetOptimization: "LINE_ITEM",
		DailyBudgetAmountLocalMicro: int64Pointer(1000000), TotalBudgetAmountLocalMicro: int64Pointer(10000000),
	}
}

func lineItemFixture(name string, status EntityStatus) LineItem {
	return LineItem{
		ID: testLineItemID, AccountID: testAdsAccountID, CampaignID: testCampaignID, Name: name,
		Objective: ObjectiveEngagements, ProductType: ProductPromotedTweets,
		Placements: []Placement{PlacementAllOnTwitter}, BidStrategy: BidStrategyMax,
		BidAmountLocalMicro: int64Pointer(3210000), DailyBudgetAmountLocalMicro: int64Pointer(1000000),
		TotalBudgetAmountLocalMicro: int64Pointer(10000000), EntityStatus: status,
		StartTime: testNow.Format(time.RFC3339),
	}
}

func promotedTweetFixture() PromotedTweet {
	return PromotedTweet{
		ID: testPromotedTweetID, LineItemID: testLineItemID, TweetID: testTweetID,
		EntityStatus: StatusActive, ApprovalStatus: "ACCEPTED",
	}
}

package ads

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestTypedWorkflowWireContracts(t *testing.T) {
	campaignName, campaignStatus := "Launch", StatusPaused
	lineItemName, lineItemStatus := "Engagements", StatusPaused
	var writes atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assertAPIRequest(t, request)
		if request.Method != http.MethodGet {
			writes.Add(1)
		}
		switch request.Method + " " + request.URL.Path {
		case http.MethodGet + " /12/accounts/" + testAdsAccountID:
			writeValue(t, writer, http.StatusOK, singleResponse[AdAccount]{Data: AdAccount{ID: testAdsAccountID, Name: "Paid Social", Timezone: "America/Los_Angeles"}})
		case http.MethodGet + " /12/accounts/" + testAdsAccountID + "/authenticated_user_access":
			writeValue(t, writer, http.StatusOK, singleResponse[AuthenticatedUserAccess]{Data: AuthenticatedUserAccess{UserID: "2417045708", Permissions: []string{"ACCOUNT_ADMIN", "TWEET_COMPOSER"}}})
		case http.MethodGet + " /12/accounts/" + testAdsAccountID + "/funding_instruments/" + testFundingInstrumentID:
			writeValue(t, writer, http.StatusOK, singleResponse[FundingInstrument]{Data: FundingInstrument{ID: testFundingInstrumentID, AccountID: testAdsAccountID, Currency: "USD", AbleToFund: true}})
		case http.MethodGet + " /12/accounts/" + testAdsAccountID + "/campaigns":
			if request.URL.Query().Get("cursor") != "campaign-cursor" || request.URL.Query().Get("count") != "1000" {
				t.Fatalf("campaign list query=%v", request.URL.Query())
			}
			next := "next-campaign"
			writeValue(t, writer, http.StatusOK, listResponse[Campaign]{Data: []Campaign{campaignFixture(campaignName, campaignStatus)}, NextCursor: &next})
		case http.MethodGet + " /12/accounts/" + testAdsAccountID + "/campaigns/" + testCampaignID:
			writeValue(t, writer, http.StatusOK, singleResponse[Campaign]{Data: campaignFixture(campaignName, campaignStatus)})
		case http.MethodPost + " /12/accounts/" + testAdsAccountID + "/campaigns":
			query := request.URL.Query()
			if query.Get("funding_instrument_id") != testFundingInstrumentID || query.Get("name") != "Created campaign" ||
				query.Get("daily_budget_amount_local_micro") != "1000000" || query.Get("total_budget_amount_local_micro") != "10000000" ||
				query.Get("entity_status") != "PAUSED" || query.Get("budget_optimization") != "LINE_ITEM" {
				t.Fatalf("create Campaign query=%v", query)
			}
			campaignName, campaignStatus = query.Get("name"), StatusPaused
			writeValue(t, writer, http.StatusOK, singleResponse[Campaign]{Data: campaignFixture(campaignName, campaignStatus)})
		case http.MethodPut + " /12/accounts/" + testAdsAccountID + "/campaigns/" + testCampaignID:
			query := request.URL.Query()
			if query.Get("name") != "Updated campaign" || query.Get("daily_budget_amount_local_micro") != "2000000" || query.Get("entity_status") != "" {
				t.Fatalf("update Campaign query=%v", query)
			}
			campaignName = query.Get("name")
			value := campaignFixture(campaignName, campaignStatus)
			value.DailyBudgetAmountLocalMicro = int64Pointer(2000000)
			writeValue(t, writer, http.StatusOK, singleResponse[Campaign]{Data: value})
		case http.MethodGet + " /12/accounts/" + testAdsAccountID + "/line_items":
			if request.URL.Query().Get("cursor") != "line-cursor" || request.URL.Query().Get("count") != "200" {
				t.Fatalf("line list query=%v", request.URL.Query())
			}
			writeValue(t, writer, http.StatusOK, listResponse[LineItem]{Data: []LineItem{lineItemFixture(lineItemName, lineItemStatus)}})
		case http.MethodGet + " /12/accounts/" + testAdsAccountID + "/line_items/" + testLineItemID:
			writeValue(t, writer, http.StatusOK, singleResponse[LineItem]{Data: lineItemFixture(lineItemName, lineItemStatus)})
		case http.MethodPost + " /12/accounts/" + testAdsAccountID + "/line_items":
			query := request.URL.Query()
			if query.Get("campaign_id") != testCampaignID || query.Get("objective") != "ENGAGEMENTS" || query.Get("product_type") != "PROMOTED_TWEETS" ||
				query.Get("placements") != "ALL_ON_TWITTER" || query.Get("bid_strategy") != "MAX" || query.Get("bid_amount_local_micro") != "3210000" ||
				query.Get("entity_status") != "PAUSED" || query.Get("start_time") != testNow.Format(time.RFC3339) || query.Get("end_time") != "" {
				t.Fatalf("create Line Item query=%v", query)
			}
			lineItemName, lineItemStatus = query.Get("name"), StatusPaused
			writeValue(t, writer, http.StatusOK, singleResponse[LineItem]{Data: lineItemFixture(lineItemName, lineItemStatus)})
		case http.MethodPut + " /12/accounts/" + testAdsAccountID + "/line_items/" + testLineItemID:
			query := request.URL.Query()
			if query.Get("name") != "Updated line item" || query.Get("bid_amount_local_micro") != "4000000" || query.Get("entity_status") != "" {
				t.Fatalf("update Line Item query=%v", query)
			}
			lineItemName = query.Get("name")
			value := lineItemFixture(lineItemName, lineItemStatus)
			value.BidAmountLocalMicro = int64Pointer(4000000)
			writeValue(t, writer, http.StatusOK, singleResponse[LineItem]{Data: value})
		case http.MethodGet + " /12/accounts/" + testAdsAccountID + "/promoted_tweets":
			query := request.URL.Query()
			if query.Get("cursor") != "promoted-cursor" || query.Get("count") != "200" || query.Get("line_item_ids") != testLineItemID {
				t.Fatalf("Promoted Tweet list query=%v", query)
			}
			writeValue(t, writer, http.StatusOK, listResponse[PromotedTweet]{Data: []PromotedTweet{promotedTweetFixture()}})
		case http.MethodGet + " /12/accounts/" + testAdsAccountID + "/promoted_tweets/" + testPromotedTweetID:
			writeValue(t, writer, http.StatusOK, singleResponse[PromotedTweet]{Data: promotedTweetFixture()})
		case http.MethodPost + " /12/accounts/" + testAdsAccountID + "/promoted_tweets":
			if request.URL.Query().Get("line_item_id") != testLineItemID || request.URL.Query().Get("tweet_ids") != testTweetID {
				t.Fatalf("associate query=%v", request.URL.Query())
			}
			writeValue(t, writer, http.StatusOK, listResponse[PromotedTweet]{Data: []PromotedTweet{promotedTweetFixture()}})
		case http.MethodGet + " /12/stats/accounts/" + testAdsAccountID:
			query := request.URL.Query()
			if query.Get("entity") != "LINE_ITEM" || query.Get("entity_ids") != testLineItemID || query.Get("granularity") != "HOUR" ||
				query.Get("placement") != "ALL_ON_TWITTER" || query.Get("metric_groups") != "ENGAGEMENT,BILLING" ||
				query.Get("start_time") != testNow.Add(-24*time.Hour).Format(time.RFC3339) || query.Get("end_time") != testNow.Format(time.RFC3339) {
				t.Fatalf("Stats query=%v", query)
			}
			writeValue(t, writer, http.StatusOK, map[string]any{
				"data_type": "stats", "time_series_length": 24,
				"data": []any{map[string]any{"id": testLineItemID, "id_data": []any{map[string]any{
					"segment": nil, "metrics": map[string]any{"impressions": []int{1233}, "likes": []int{1}},
				}}}},
			})
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)

	account, err := client.GetAdAccount(context.Background())
	if err != nil || account.ID != testAdsAccountID {
		t.Fatalf("account=%#v err=%v", account, err)
	}
	access, err := client.GetAuthenticatedUserAccess(context.Background())
	if err != nil || access.UserID != "2417045708" || len(access.Permissions) != 2 {
		t.Fatalf("access=%#v err=%v", access, err)
	}
	campaigns, err := client.ListCampaigns(context.Background(), ListRequest{Cursor: "campaign-cursor", Count: 1000})
	if err != nil || len(campaigns.Items) != 1 || !campaigns.HasMore || *campaigns.NextCursor != "next-campaign" {
		t.Fatalf("campaigns=%#v err=%v", campaigns, err)
	}
	createdCampaign, err := client.CreateCampaign(context.Background(), CreateCampaignRequest{
		FundingInstrumentID: testFundingInstrumentID, Name: "Created campaign", DailyBudgetAmountLocalMicro: 1000000,
		TotalBudgetAmountLocalMicro: int64Pointer(10000000),
	})
	if err != nil || createdCampaign.EntityStatus != StatusPaused {
		t.Fatalf("created Campaign=%#v err=%v", createdCampaign, err)
	}
	updatedCampaign, err := client.UpdateCampaign(context.Background(), testCampaignID, UpdateCampaignRequest{
		Name: stringPointer("Updated campaign"), DailyBudgetAmountLocalMicro: int64Pointer(2000000),
	})
	if err != nil || updatedCampaign.Name != "Updated campaign" {
		t.Fatalf("updated Campaign=%#v err=%v", updatedCampaign, err)
	}
	lineItems, err := client.ListLineItems(context.Background(), ListRequest{Cursor: "line-cursor", Count: 200})
	if err != nil || len(lineItems.Items) != 1 {
		t.Fatalf("Line Items=%#v err=%v", lineItems, err)
	}
	createdLineItem, err := client.CreateLineItem(context.Background(), CreateLineItemRequest{
		CampaignID: testCampaignID, Name: "Created line item", Objective: ObjectiveEngagements, ProductType: ProductPromotedTweets,
		Placements: []Placement{PlacementAllOnTwitter}, BidStrategy: BidStrategyMax, BidAmountLocalMicro: int64Pointer(3210000),
		DailyBudgetAmountLocalMicro: int64Pointer(1000000), StartTime: testNow,
	})
	if err != nil || createdLineItem.EntityStatus != StatusPaused {
		t.Fatalf("created Line Item=%#v err=%v", createdLineItem, err)
	}
	updatedLineItem, err := client.UpdateLineItem(context.Background(), testLineItemID, UpdateLineItemRequest{
		Name: stringPointer("Updated line item"), BidAmountLocalMicro: int64Pointer(4000000),
	})
	if err != nil || updatedLineItem.Name != "Updated line item" {
		t.Fatalf("updated Line Item=%#v err=%v", updatedLineItem, err)
	}
	promoted, err := client.ListPromotedTweets(context.Background(), ListPromotedTweetsRequest{Cursor: "promoted-cursor", Count: 200, LineItemIDs: []string{testLineItemID}})
	if err != nil || len(promoted.Items) != 1 {
		t.Fatalf("Promoted Tweets=%#v err=%v", promoted, err)
	}
	single, err := client.GetPromotedTweet(context.Background(), testPromotedTweetID)
	if err != nil || single.TweetID != testTweetID {
		t.Fatalf("Promoted Tweet=%#v err=%v", single, err)
	}
	associated, err := client.AssociateTweets(context.Background(), AssociateTweetsRequest{LineItemID: testLineItemID, TweetIDs: []string{testTweetID}})
	if err != nil || len(associated) != 1 {
		t.Fatalf("associated=%#v err=%v", associated, err)
	}
	stats, err := client.GetStats(context.Background(), StatsRequest{
		Entity: AnalyticsLineItem, EntityIDs: []string{testLineItemID}, StartTime: testNow.Add(-24 * time.Hour), EndTime: testNow,
		Granularity: GranularityHour, Placement: AnalyticsPlacementAllOnTwitter, MetricGroups: []MetricGroup{MetricGroupEngagement, MetricGroupBilling},
	})
	if err != nil || stats.TimeSeriesLength != 24 || len(stats.Entities) != 1 || string(stats.Entities[0].IDData[0].Metrics["impressions"]) != "[1233]" {
		t.Fatalf("Stats=%#v err=%v", stats, err)
	}
	if writes.Load() != 5 {
		t.Fatalf("writes=%d", writes.Load())
	}
}

func TestMutationsRejectWrongOwnersAndUnsafeParentsBeforeWrite(t *testing.T) {
	validCampaign := CreateCampaignRequest{FundingInstrumentID: testFundingInstrumentID, Name: "Campaign", DailyBudgetAmountLocalMicro: 1000000}
	validLineItem := CreateLineItemRequest{
		CampaignID: testCampaignID, Objective: ObjectiveEngagements, ProductType: ProductPromotedTweets,
		Placements: []Placement{PlacementAllOnTwitter}, BidStrategy: BidStrategyMax,
		BidAmountLocalMicro: int64Pointer(100000), StartTime: testNow,
	}
	tests := []struct {
		name    string
		handler func(http.ResponseWriter, *http.Request)
		call    func(*Client) error
	}{
		{name: "Campaign Funding Instrument owner", handler: func(writer http.ResponseWriter, _ *http.Request) {
			writeValue(t, writer, http.StatusOK, singleResponse[FundingInstrument]{Data: FundingInstrument{ID: testFundingInstrumentID, AccountID: "other"}})
		}, call: func(client *Client) error {
			_, err := client.CreateCampaign(context.Background(), validCampaign)
			return err
		}},
		{name: "Line Item Campaign owner", handler: func(writer http.ResponseWriter, _ *http.Request) {
			value := campaignFixture("wrong owner", StatusPaused)
			value.AccountID = "other"
			writeValue(t, writer, http.StatusOK, singleResponse[Campaign]{Data: value})
		}, call: func(client *Client) error {
			_, err := client.CreateLineItem(context.Background(), validLineItem)
			return err
		}},
		{name: "Line Item owner", handler: func(writer http.ResponseWriter, _ *http.Request) {
			value := lineItemFixture("wrong owner", StatusPaused)
			value.AccountID = "other"
			writeValue(t, writer, http.StatusOK, singleResponse[LineItem]{Data: value})
		}, call: func(client *Client) error {
			_, err := client.UpdateLineItem(context.Background(), testLineItemID, UpdateLineItemRequest{Name: stringPointer("new")})
			return err
		}},
		{name: "active Promoted Tweet parent", handler: func(writer http.ResponseWriter, _ *http.Request) {
			writeValue(t, writer, http.StatusOK, singleResponse[LineItem]{Data: lineItemFixture("active", StatusActive)})
		}, call: func(client *Client) error {
			_, err := client.AssociateTweets(context.Background(), AssociateTweetsRequest{LineItemID: testLineItemID, TweetIDs: []string{testTweetID}})
			return err
		}},
		{name: "wrong product Promoted Tweet parent", handler: func(writer http.ResponseWriter, _ *http.Request) {
			value := lineItemFixture("media", StatusPaused)
			value.ProductType = ProductMedia
			writeValue(t, writer, http.StatusOK, singleResponse[LineItem]{Data: value})
		}, call: func(client *Client) error {
			_, err := client.AssociateTweets(context.Background(), AssociateTweetsRequest{LineItemID: testLineItemID, TweetIDs: []string{testTweetID}})
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var writes atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				assertAPIRequest(t, request)
				if request.Method != http.MethodGet {
					writes.Add(1)
				}
				test.handler(writer, request)
			}))
			defer server.Close()
			_, client := newTestAdapter(t, server)
			err := test.call(client)
			if err == nil || writes.Load() != 0 {
				t.Fatalf("error=%v writes=%d", err, writes.Load())
			}
		})
	}
}

func TestResponseContractRejections(t *testing.T) {
	tests := []struct {
		name string
		body any
		call func(*Client) error
	}{
		{name: "empty cursor", body: map[string]any{"data": []Campaign{campaignFixture("name", StatusPaused)}, "next_cursor": ""}, call: func(client *Client) error {
			_, err := client.ListCampaigns(context.Background(), ListRequest{})
			return err
		}},
		{name: "wrong Campaign owner", body: func() any {
			value := campaignFixture("name", StatusPaused)
			value.AccountID = "other"
			return singleResponse[Campaign]{Data: value}
		}(), call: func(client *Client) error {
			_, err := client.GetCampaign(context.Background(), testCampaignID)
			return err
		}},
		{name: "wrong Stats entity", body: map[string]any{"data_type": "stats", "time_series_length": 1, "data": []any{map[string]any{"id": "other", "id_data": []any{}}}}, call: func(client *Client) error {
			_, err := client.GetStats(context.Background(), StatsRequest{Entity: AnalyticsLineItem, EntityIDs: []string{testLineItemID}, StartTime: testNow.Add(-time.Hour), EndTime: testNow, Granularity: GranularityHour, Placement: AnalyticsPlacementAllOnTwitter, MetricGroups: []MetricGroup{MetricGroupEngagement}})
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				assertAPIRequest(t, request)
				writeValue(t, writer, http.StatusOK, test.body)
			}))
			defer server.Close()
			_, client := newTestAdapter(t, server)
			err := test.call(client)
			if hubError(t, err).Code != socialhub.CodePlatformError {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestStatsMetricJSONPrecision(t *testing.T) {
	var values MetricValues
	if err := json.Unmarshal([]byte(`{"spend":[9007199254740993],"empty":null}`), &values); err != nil {
		t.Fatal(err)
	}
	if string(values["spend"]) != "[9007199254740993]" || string(values["empty"]) != "null" {
		t.Fatalf("metrics=%v", values)
	}
}

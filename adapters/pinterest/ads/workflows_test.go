package ads

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestPinterestAdsWireContracts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assertAPIRequest(t, request)
		switch request.URL.Path {
		case "/ad_accounts":
			if request.URL.Query().Get("include_shared_accounts") != "true" || request.URL.Query().Get("bookmark") != "account-page" || request.URL.Query().Get("page_size") != "25" {
				t.Errorf("accounts query=%v", request.URL.Query())
			}
			writeJSON(writer, http.StatusOK, `{"items":[{"id":"111111111111","name":"US Store","country":"US","currency":"USD"}],"bookmark":"next-account"}`)
		case "/ad_accounts/" + testAdAccountID:
			writeJSON(writer, http.StatusOK, `{"id":"111111111111","name":"US Store","time_zone":"America/Los_Angeles"}`)
		case "/ad_accounts/" + testAdAccountID + "/campaigns":
			handleCampaigns(t, writer, request)
		case "/ad_accounts/" + testAdAccountID + "/campaigns/" + testCampaignID:
			writeJSON(writer, http.StatusOK, campaignJSON(StatusPaused))
		case "/ad_accounts/" + testAdAccountID + "/ad_groups":
			handleAdGroups(t, writer, request)
		case "/ad_accounts/" + testAdAccountID + "/ad_groups/" + testAdGroupID:
			writeJSON(writer, http.StatusOK, adGroupJSON(StatusPaused))
		case "/ad_accounts/" + testAdAccountID + "/ads":
			handleAds(t, writer, request)
		case "/ad_accounts/" + testAdAccountID + "/ads/" + testAdID:
			writeJSON(writer, http.StatusOK, adJSON(StatusPaused))
		case "/ad_accounts/" + testAdAccountID + "/analytics":
			query := request.URL.Query()
			if query.Get("columns") != "IMPRESSION_1,CLICKTHROUGH_1,SPEND_IN_MICRO_DOLLAR" || query.Get("granularity") != "DAY" || query.Get("click_window_days") != "7" || query.Get("reporting_timezone") != "AD_ACCOUNT_TIME_ZONE" {
				t.Errorf("analytics query=%v", query)
			}
			writeJSON(writer, http.StatusOK, `[{"AD_ACCOUNT_ID":"111111111111","DATE":"2026-08-01","IMPRESSION_1":123,"SPEND_IN_MICRO_DOLLAR":456789}]`)
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	ctx := context.Background()
	includeShared := true
	accounts, err := client.ListAdAccounts(ctx, ListAdAccountsRequest{IncludeSharedAccounts: &includeShared, Cursor: "account-page", MaxResults: 25})
	if err != nil || len(accounts.Items) != 1 || accounts.NextCursor == nil || *accounts.NextCursor != "next-account" {
		t.Fatalf("accounts=%#v err=%v", accounts, err)
	}
	account, err := client.GetAdAccount(ctx)
	if err != nil || account.ID != testAdAccountID {
		t.Fatalf("account=%#v err=%v", account, err)
	}

	campaigns, err := client.ListCampaigns(ctx, ListCampaignsRequest{IDs: []string{testCampaignID}, Statuses: []EntityStatus{StatusPaused}, Cursor: "campaign-page", MaxResults: 20}, socialhub.WithRequestID("caller-request"))
	if err != nil || len(campaigns.Items) != 1 || campaigns.NextCursor == nil {
		t.Fatalf("campaigns=%#v err=%v", campaigns, err)
	}
	if _, err = client.GetCampaign(ctx, testCampaignID); err != nil {
		t.Fatal(err)
	}
	campaign, err := client.CreateCampaign(ctx, validCampaignRequest())
	if err != nil || campaign.Status != StatusPaused {
		t.Fatalf("campaign=%#v err=%v", campaign, err)
	}
	name, spend := "Launch 2", int64(25000000)
	if _, err = client.UpdateCampaign(ctx, testCampaignID, UpdateCampaignRequest{Name: &name, DailySpendCap: &spend}); err != nil {
		t.Fatal(err)
	}
	if _, err = client.SetCampaignStatus(ctx, testCampaignID, StatusActive); err != nil {
		t.Fatal(err)
	}
	if err = client.ArchiveCampaign(ctx, testCampaignID); err != nil {
		t.Fatal(err)
	}

	adGroups, err := client.ListAdGroups(ctx, ListAdGroupsRequest{CampaignIDs: []string{testCampaignID}, Statuses: []EntityStatus{StatusPaused}})
	if err != nil || len(adGroups.Items) != 1 {
		t.Fatalf("Ad Groups=%#v err=%v", adGroups, err)
	}
	if _, err = client.GetAdGroup(ctx, testAdGroupID); err != nil {
		t.Fatal(err)
	}
	adGroup, err := client.CreateAdGroup(ctx, validAdGroupRequest())
	if err != nil || adGroup.Status != StatusPaused {
		t.Fatalf("Ad Group=%#v err=%v", adGroup, err)
	}
	bid := int64(1500000)
	if _, err = client.UpdateAdGroup(ctx, testAdGroupID, UpdateAdGroupRequest{BidInMicroCurrency: &bid}); err != nil {
		t.Fatal(err)
	}
	if _, err = client.SetAdGroupStatus(ctx, testAdGroupID, StatusActive); err != nil {
		t.Fatal(err)
	}
	if err = client.ArchiveAdGroup(ctx, testAdGroupID); err != nil {
		t.Fatal(err)
	}

	adsPage, err := client.ListAds(ctx, ListAdsRequest{AdGroupIDs: []string{testAdGroupID}, Statuses: []EntityStatus{StatusPaused}})
	if err != nil || len(adsPage.Items) != 1 {
		t.Fatalf("Ads=%#v err=%v", adsPage, err)
	}
	if _, err = client.GetAd(ctx, testAdID); err != nil {
		t.Fatal(err)
	}
	ad, err := client.CreateAd(ctx, CreateAdRequest{AdGroupID: testAdGroupID, PinID: testPinID, Name: "Product Pin", CreativeType: CreativeRegular, DestinationURL: "https://shop.example/item"})
	if err != nil || ad.Status != StatusPaused {
		t.Fatalf("Ad=%#v err=%v", ad, err)
	}
	destination := "https://shop.example/item-2"
	if _, err = client.UpdateAd(ctx, testAdID, UpdateAdRequest{DestinationURL: &destination}); err != nil {
		t.Fatal(err)
	}
	if _, err = client.SetAdStatus(ctx, testAdID, StatusActive); err != nil {
		t.Fatal(err)
	}
	if err = client.ArchiveAd(ctx, testAdID); err != nil {
		t.Fatal(err)
	}

	clickWindow := 7
	rows, err := client.GetAccountAnalytics(ctx, AnalyticsRequest{
		StartDate: "2026-08-01", EndDate: "2026-08-07",
		Columns:     []string{"IMPRESSION_1", "CLICKTHROUGH_1", "SPEND_IN_MICRO_DOLLAR"},
		Granularity: GranularityDay, ClickWindowDays: &clickWindow, ReportingTimezone: TimezoneAdAccount,
	})
	if err != nil || len(rows) != 1 || rows[0].AdAccountID != testAdAccountID || string(rows[0].Metrics["SPEND_IN_MICRO_DOLLAR"]) != "456789" {
		t.Fatalf("analytics=%#v err=%v", rows, err)
	}
}

func handleCampaigns(t *testing.T, writer http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodGet {
		query := request.URL.Query()
		if query["campaign_ids"][0] != testCampaignID || query["entity_statuses"][0] != "PAUSED" || request.Header.Get("X-Request-ID") != "caller-request" {
			t.Errorf("campaign query=%v headers=%v", query, request.Header)
		}
		writeJSON(writer, http.StatusOK, `{"items":[`+campaignJSON(StatusPaused)+`],"bookmark":"next-campaign"}`)
		return
	}
	resource := decodeBatchResource(t, request)
	status := StatusPaused
	if raw, ok := resource["status"].(string); ok {
		status = EntityStatus(raw)
	}
	if request.Method == http.MethodPost {
		if resource["status"] != "PAUSED" || resource["ad_account_id"] != testAdAccountID || resource["daily_spend_cap"] != json.Number("20000000") || resource["is_campaign_budget_optimization"] != true {
			t.Errorf("campaign create=%v", resource)
		}
	} else if status == StatusActive || status == StatusArchived {
		if resource["daily_spend_cap"] != nil || resource["name"] != nil {
			t.Errorf("campaign status=%v", resource)
		}
	}
	writeJSON(writer, http.StatusOK, `{"items":[{"data":`+campaignJSON(status)+`} ]}`)
}

func handleAdGroups(t *testing.T, writer http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodGet {
		writeJSON(writer, http.StatusOK, `{"items":[`+adGroupJSON(StatusPaused)+`]}`)
		return
	}
	resource := decodeBatchResource(t, request)
	status := StatusPaused
	if raw, ok := resource["status"].(string); ok {
		status = EntityStatus(raw)
	}
	if request.Method == http.MethodPost {
		targeting := resource["targeting_spec"].(map[string]any)
		if resource["status"] != "PAUSED" || resource["campaign_id"] != testCampaignID || resource["budget_in_micro_currency"] != json.Number("5000000") || targeting["LOCATION"] == nil {
			t.Errorf("Ad Group create=%v", resource)
		}
	} else if status == StatusActive || status == StatusArchived {
		if resource["bid_in_micro_currency"] != nil {
			t.Errorf("Ad Group status=%v", resource)
		}
	}
	writeJSON(writer, http.StatusOK, `{"items":[{"data":`+adGroupJSON(status)+`,"exceptions":[]}]}`)
}

func handleAds(t *testing.T, writer http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodGet {
		writeJSON(writer, http.StatusOK, `{"items":[`+adJSON(StatusPaused)+`]}`)
		return
	}
	resource := decodeBatchResource(t, request)
	status := StatusPaused
	if raw, ok := resource["status"].(string); ok {
		status = EntityStatus(raw)
	}
	if request.Method == http.MethodPost && (resource["status"] != "PAUSED" || resource["ad_group_id"] != testAdGroupID || resource["pin_id"] != testPinID) {
		t.Errorf("Ad create=%v", resource)
	}
	if (status == StatusActive || status == StatusArchived) && resource["destination_url"] != nil {
		t.Errorf("Ad status=%v", resource)
	}
	writeJSON(writer, http.StatusOK, `{"items":[{"data":`+adJSON(status)+`}]}`)
}

func campaignJSON(status EntityStatus) string {
	return `{"id":"` + testCampaignID + `","ad_account_id":"` + testAdAccountID + `","name":"Launch","objective_type":"CONSIDERATION","status":"` + string(status) + `"}`
}

func adGroupJSON(status EntityStatus) string {
	return `{"id":"` + testAdGroupID + `","ad_account_id":"` + testAdAccountID + `","campaign_id":"` + testCampaignID + `","name":"US Prospects","billable_event":"CLICKTHROUGH","status":"` + string(status) + `"}`
}

func adJSON(status EntityStatus) string {
	return `{"id":"` + testAdID + `","ad_account_id":"` + testAdAccountID + `","campaign_id":"` + testCampaignID + `","ad_group_id":"` + testAdGroupID + `","pin_id":"` + testPinID + `","creative_type":"REGULAR","status":"` + string(status) + `","rejected_reasons":[],"rejection_labels":[]}`
}

func validCampaignRequest() CreateCampaignRequest {
	return CreateCampaignRequest{Name: "Launch", Objective: ObjectiveConsideration, CampaignBudgetOptimization: true, DailySpendCap: 20000000}
}

func validAdGroupRequest() CreateAdGroupRequest {
	return CreateAdGroupRequest{
		CampaignID: testCampaignID, Name: "US Prospects", BillableEvent: BillableClickthrough,
		BudgetType: BudgetDaily, BudgetInMicroCurrency: 5000000, BidInMicroCurrency: 1000000,
		BidStrategy: BidMaximum, Pacing: PacingStandard, Placement: PlacementAll,
		Targeting: TargetingSpec{"LOCATION": {"US"}, "GENDER": {"female", "male", "unknown"}},
	}
}

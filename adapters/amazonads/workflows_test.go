package amazonads

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestSponsoredProductsAndReportingWireContracts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/v2/profiles":
			assertAPIRequest(t, request, "", false)
			writeJSON(writer, http.StatusOK, `[{"profileId":1234567890,"countryCode":"US","currencyCode":"USD","dailyBudget":100.50,"timezone":"America/Los_Angeles","accountInfo":{"id":"A1","type":"seller","validPaymentMethod":true}}]`)
		case "/sp/campaigns/list":
			assertAPIRequest(t, request, campaignMediaType, false)
			body := decodeBody(t, request)
			if body["maxResults"] != json.Number("10") || body["nextToken"] != "campaign-page" {
				t.Errorf("campaign list=%v", body)
			}
			writeJSON(writer, http.StatusOK, `{"campaigns":[{"campaignId":"2001","name":"Brand","targetingType":"MANUAL","state":"PAUSED","budget":{"budgetType":"DAILY","budget":12.34}}],"nextToken":"next-campaign","totalResults":2}`)
		case "/sp/campaigns":
			handleCampaignMutation(t, writer, request)
		case "/sp/campaigns/delete":
			handleArchive(t, writer, request, campaignMediaType, "campaignIdFilter", "campaigns", "campaignId", testCampaignID)
		case "/sp/adGroups/list":
			assertAPIRequest(t, request, adGroupMediaType, false)
			writeJSON(writer, http.StatusOK, `{"adGroups":[{"adGroupId":"3001","campaignId":"2001","name":"Exact","defaultBid":0.75,"state":"PAUSED"}],"totalResults":1}`)
		case "/sp/adGroups":
			handleAdGroupMutation(t, writer, request)
		case "/sp/adGroups/delete":
			handleArchive(t, writer, request, adGroupMediaType, "adGroupIdFilter", "adGroups", "adGroupId", testAdGroupID)
		case "/sp/productAds/list":
			assertAPIRequest(t, request, productAdMediaType, false)
			writeJSON(writer, http.StatusOK, `{"productAds":[{"adId":"4001","campaignId":"2001","adGroupId":"3001","sku":"SKU-1","state":"PAUSED"}],"totalResults":1}`)
		case "/sp/productAds":
			handleProductAdMutation(t, writer, request)
		case "/sp/productAds/delete":
			handleArchive(t, writer, request, productAdMediaType, "adIdFilter", "productAds", "adId", testProductAdID)
		case "/sp/keywords/list":
			assertAPIRequest(t, request, keywordMediaType, false)
			writeJSON(writer, http.StatusOK, `{"keywords":[{"keywordId":"5001","campaignId":"2001","adGroupId":"3001","keywordText":"soap bar","matchType":"EXACT","bid":0.42,"state":"PAUSED"}],"totalResults":1}`)
		case "/sp/keywords":
			handleKeywordMutation(t, writer, request)
		case "/sp/keywords/delete":
			handleArchive(t, writer, request, keywordMediaType, "keywordIdFilter", "keywords", "keywordId", testKeywordID)
		case "/reporting/reports":
			assertAPIRequest(t, request, reportCreateMediaType, false)
			body := decodeBody(t, request)
			configuration := body["configuration"].(map[string]any)
			if configuration["adProduct"] != "SPONSORED_PRODUCTS" || configuration["reportTypeId"] != "spCampaigns" {
				t.Errorf("report=%v", body)
			}
			writeJSON(writer, http.StatusAccepted, `{"reportId":"report_abc-123","status":"PENDING"}`)
		case "/reporting/reports/report_abc-123":
			assertAPIRequest(t, request, "", false)
			writeJSON(writer, http.StatusOK, `{"reportId":"report_abc-123","status":"COMPLETED","url":"https://reports.example/signed?secret=opaque","urlExpiresAt":"2026-08-09T13:00:00Z"}`)
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	ctx := context.Background()

	profiles, err := client.ListProfiles(ctx)
	if err != nil || len(profiles) != 1 || profiles[0].ID != testProfileID || profiles[0].DailyBudget != "100.50" {
		t.Fatalf("profiles=%#v err=%v", profiles, err)
	}
	profile, err := client.GetProfile(ctx)
	if err != nil || profile.ID != testProfileID {
		t.Fatalf("profile=%#v err=%v", profile, err)
	}

	campaigns, err := client.ListCampaigns(ctx, ListCampaignsRequest{IDs: []string{testCampaignID}, States: []State{StatePaused}, MaxResults: 10, NextToken: "campaign-page"}, socialhub.WithRequestID("caller-request"))
	if err != nil || len(campaigns.Items) != 1 || campaigns.NextToken != "next-campaign" || campaigns.TotalResults != 2 {
		t.Fatalf("campaigns=%#v err=%v", campaigns, err)
	}
	campaign, err := client.CreateCampaign(ctx, CreateCampaignRequest{Name: "Brand", TargetingType: TargetingManual, StartDate: "2026-08-10", EndDate: "2026-08-31", DailyBudget: "12.34", DynamicBidding: DynamicBidding{Strategy: BiddingLegacyForSales}, PortfolioID: "9001"})
	if err != nil || campaign.ID != testCampaignID || campaign.State != StatePaused {
		t.Fatalf("campaign=%#v err=%v", campaign, err)
	}
	name, budget := "Brand 2", Decimal("15.75")
	if _, err = client.UpdateCampaign(ctx, testCampaignID, UpdateCampaignRequest{Name: &name, DailyBudget: &budget}); err != nil {
		t.Fatal(err)
	}
	if _, err = client.SetCampaignState(ctx, testCampaignID, StateEnabled); err != nil {
		t.Fatal(err)
	}
	if err = client.ArchiveCampaign(ctx, testCampaignID); err != nil {
		t.Fatal(err)
	}

	adGroups, err := client.ListAdGroups(ctx, ListAdGroupsRequest{CampaignIDs: []string{testCampaignID}, States: []State{StatePaused}})
	if err != nil || len(adGroups.Items) != 1 {
		t.Fatalf("ad groups=%#v err=%v", adGroups, err)
	}
	adGroup, err := client.CreateAdGroup(ctx, CreateAdGroupRequest{CampaignID: testCampaignID, Name: "Exact", DefaultBid: "0.75"})
	if err != nil || adGroup.ID != testAdGroupID || adGroup.State != StatePaused {
		t.Fatalf("ad group=%#v err=%v", adGroup, err)
	}
	bid := Decimal("0.80")
	if _, err = client.UpdateAdGroup(ctx, testAdGroupID, UpdateAdGroupRequest{DefaultBid: &bid}); err != nil {
		t.Fatal(err)
	}
	if _, err = client.SetAdGroupState(ctx, testAdGroupID, StateEnabled); err != nil {
		t.Fatal(err)
	}
	if err = client.ArchiveAdGroup(ctx, testAdGroupID); err != nil {
		t.Fatal(err)
	}

	productAds, err := client.ListProductAds(ctx, ListProductAdsRequest{CampaignIDs: []string{testCampaignID}, AdGroupIDs: []string{testAdGroupID}})
	if err != nil || len(productAds.Items) != 1 {
		t.Fatalf("product ads=%#v err=%v", productAds, err)
	}
	productAd, err := client.CreateProductAd(ctx, CreateProductAdRequest{CampaignID: testCampaignID, AdGroupID: testAdGroupID, SKU: "SKU-1"})
	if err != nil || productAd.ID != testProductAdID || productAd.State != StatePaused {
		t.Fatalf("product ad=%#v err=%v", productAd, err)
	}
	if _, err = client.SetProductAdState(ctx, testProductAdID, StateEnabled); err != nil {
		t.Fatal(err)
	}
	if err = client.ArchiveProductAd(ctx, testProductAdID); err != nil {
		t.Fatal(err)
	}

	keywords, err := client.ListKeywords(ctx, ListKeywordsRequest{AdGroupIDs: []string{testAdGroupID}, MatchTypes: []MatchType{MatchExact}})
	if err != nil || len(keywords.Items) != 1 {
		t.Fatalf("keywords=%#v err=%v", keywords, err)
	}
	keyword, err := client.CreateKeyword(ctx, CreateKeywordRequest{CampaignID: testCampaignID, AdGroupID: testAdGroupID, Text: "soap bar", MatchType: MatchExact, Bid: "0.42"})
	if err != nil || keyword.ID != testKeywordID || keyword.State != StatePaused {
		t.Fatalf("keyword=%#v err=%v", keyword, err)
	}
	keywordBid := Decimal("0.50")
	if _, err = client.UpdateKeyword(ctx, testKeywordID, UpdateKeywordRequest{Bid: &keywordBid}); err != nil {
		t.Fatal(err)
	}
	if _, err = client.SetKeywordState(ctx, testKeywordID, StateEnabled); err != nil {
		t.Fatal(err)
	}
	if err = client.ArchiveKeyword(ctx, testKeywordID); err != nil {
		t.Fatal(err)
	}

	report, err := client.CreateReport(ctx, CreateReportRequest{Name: "SP campaign report", StartDate: "2026-08-01", EndDate: "2026-08-07", GroupBy: []string{"campaign"}, Columns: []string{"campaignId", "impressions", "clicks", "cost", "date"}, ReportTypeID: "spCampaigns", TimeUnit: ReportTimeDaily, Format: ReportFormatGZIPJSON})
	if err != nil || report.ID != "report_abc-123" {
		t.Fatalf("report=%#v err=%v", report, err)
	}
	report, err = client.GetReport(ctx, report.ID)
	if err != nil || report.Status != "COMPLETED" || report.URL == "" {
		t.Fatalf("report status=%#v err=%v", report, err)
	}
}

func handleCampaignMutation(t *testing.T, writer http.ResponseWriter, request *http.Request) {
	assertAPIRequest(t, request, campaignMediaType, true)
	resource := firstResource(t, decodeBody(t, request), "campaigns")
	state, name := resource["state"], resource["name"]
	if request.Method == http.MethodPost {
		budget := resource["budget"].(map[string]any)
		if state != "PAUSED" || budget["budget"] != json.Number("12.34") || resource["campaignId"] != nil {
			t.Errorf("campaign create=%v", resource)
		}
		writeJSON(writer, http.StatusOK, `{"campaigns":{"success":[{"index":0,"campaignId":"2001","campaign":{"campaignId":"2001","name":"Brand","state":"PAUSED"}}],"error":[]}}`)
		return
	}
	if state == "ENABLED" {
		if resource["budget"] != nil {
			t.Errorf("status-only campaign included budget: %v", resource)
		}
		writeJSON(writer, http.StatusOK, `{"campaigns":{"success":[{"index":0,"campaignId":"2001","campaign":{"campaignId":"2001","state":"ENABLED"}}],"error":[]}}`)
		return
	}
	if name != "Brand 2" {
		t.Errorf("campaign update=%v", resource)
	}
	writeJSON(writer, http.StatusOK, `{"campaigns":{"success":[{"index":0,"campaignId":"2001","campaign":{"campaignId":"2001","name":"Brand 2","state":"PAUSED"}}],"error":[]}}`)
}

func handleAdGroupMutation(t *testing.T, writer http.ResponseWriter, request *http.Request) {
	assertAPIRequest(t, request, adGroupMediaType, true)
	resource := firstResource(t, decodeBody(t, request), "adGroups")
	if request.Method == http.MethodPost {
		if resource["state"] != "PAUSED" || resource["defaultBid"] != json.Number("0.75") {
			t.Errorf("Ad Group create=%v", resource)
		}
		writeJSON(writer, http.StatusOK, `{"adGroups":{"success":[{"index":0,"adGroupId":"3001","adGroup":{"adGroupId":"3001","campaignId":"2001","state":"PAUSED"}}],"error":[]}}`)
		return
	}
	state := resource["state"]
	if state == "ENABLED" && resource["defaultBid"] != nil {
		t.Errorf("Ad Group status=%v", resource)
	}
	writeJSON(writer, http.StatusOK, `{"adGroups":{"success":[{"index":0,"adGroupId":"3001","adGroup":{"adGroupId":"3001","campaignId":"2001","state":"`+func() string {
		if state == nil {
			return "PAUSED"
		}
		return state.(string)
	}()+`"}}],"error":[]}}`)
}

func handleProductAdMutation(t *testing.T, writer http.ResponseWriter, request *http.Request) {
	assertAPIRequest(t, request, productAdMediaType, true)
	resource := firstResource(t, decodeBody(t, request), "productAds")
	if request.Method == http.MethodPost && (resource["state"] != "PAUSED" || resource["sku"] != "SKU-1") {
		t.Errorf("Product Ad create=%v", resource)
	}
	state := "PAUSED"
	if resource["state"] == "ENABLED" {
		state = "ENABLED"
	}
	writeJSON(writer, http.StatusOK, `{"productAds":{"success":[{"index":0,"adId":"4001","productAd":{"adId":"4001","campaignId":"2001","adGroupId":"3001","state":"`+state+`"}}],"error":[]}}`)
}

func handleKeywordMutation(t *testing.T, writer http.ResponseWriter, request *http.Request) {
	assertAPIRequest(t, request, keywordMediaType, true)
	resource := firstResource(t, decodeBody(t, request), "keywords")
	if request.Method == http.MethodPost && (resource["state"] != "PAUSED" || resource["bid"] != json.Number("0.42")) {
		t.Errorf("Keyword create=%v", resource)
	}
	if resource["state"] == "ENABLED" && resource["bid"] != nil {
		t.Errorf("Keyword status=%v", resource)
	}
	state := "PAUSED"
	if resource["state"] == "ENABLED" {
		state = "ENABLED"
	}
	writeJSON(writer, http.StatusOK, `{"keywords":{"success":[{"index":0,"keywordId":"5001","keyword":{"keywordId":"5001","campaignId":"2001","adGroupId":"3001","state":"`+state+`"}}],"error":[]}}`)
}

func handleArchive(t *testing.T, writer http.ResponseWriter, request *http.Request, mediaType, filterKey, envelopeKey, idKey, id string) {
	assertAPIRequest(t, request, mediaType, true)
	body := decodeBody(t, request)
	filter := body[filterKey].(map[string]any)
	include := filter["include"].([]any)
	if len(include) != 1 || include[0] != id {
		t.Errorf("archive=%v", body)
	}
	writeJSON(writer, http.StatusOK, `{"`+envelopeKey+`":{"success":[{"index":0,"`+idKey+`":"`+id+`"}],"error":[]}}`)
}

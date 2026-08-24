package appleads

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestPaidMediaWorkflows(t *testing.T) {
	campaignStatus := CampaignPaused
	adGroupStatus := AdGroupPaused
	keywordStatus := KeywordPaused
	adStatus := AdPaused
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assertAPIRequest(t, request)
		switch request.URL.Path {
		case "/api/v5/acls":
			if request.Method != http.MethodGet || request.URL.Query().Get("offset") != "0" || request.URL.Query().Get("limit") != "20" {
				t.Errorf("ACL request=%s %s", request.Method, request.URL)
			}
			writeJSON(t, writer, http.StatusOK, pagedEnvelope([]UserACL{{OrgID: testOrgID, OrgName: "Org"}}, 1))
		case "/api/v5/campaigns":
			handleCampaignCollection(t, writer, request, campaignStatus)
		case "/api/v5/campaigns/find":
			if request.Method != http.MethodPost {
				t.Errorf("method=%s", request.Method)
			}
			var selector Selector
			decodeRequest(t, request, &selector)
			if selector.Pagination.Limit != 20 {
				t.Errorf("selector=%#v", selector)
			}
			writeJSON(t, writer, http.StatusOK, pagedEnvelope([]Campaign{testCampaign(campaignStatus)}, 1))
		case "/api/v5/campaigns/2001":
			switch request.Method {
			case http.MethodGet:
				writeJSON(t, writer, http.StatusOK, envelope(testCampaign(campaignStatus)))
			case http.MethodPut:
				var payload updateCampaignPayload
				decodeRequest(t, request, &payload)
				if payload.Campaign.Status != nil {
					campaignStatus = *payload.Campaign.Status
				}
				writeJSON(t, writer, http.StatusOK, envelope(testCampaign(campaignStatus)))
			case http.MethodDelete:
				writeJSON(t, writer, http.StatusOK, envelope(nil))
			default:
				t.Errorf("campaign method=%s", request.Method)
			}
		case "/api/v5/campaigns/2001/adgroups":
			handleAdGroupCollection(t, writer, request, adGroupStatus)
		case "/api/v5/campaigns/2001/adgroups/3001":
			switch request.Method {
			case http.MethodGet:
				writeJSON(t, writer, http.StatusOK, envelope(testAdGroup(adGroupStatus)))
			case http.MethodPut:
				var payload adGroupWrite
				decodeRequest(t, request, &payload)
				if payload.Status != nil {
					adGroupStatus = *payload.Status
				}
				writeJSON(t, writer, http.StatusOK, envelope(testAdGroup(adGroupStatus)))
			case http.MethodDelete:
				writeJSON(t, writer, http.StatusOK, envelope(nil))
			default:
				t.Errorf("Ad Group method=%s", request.Method)
			}
		case "/api/v5/campaigns/2001/adgroups/3001/targetingkeywords":
			if request.Method != http.MethodGet {
				t.Errorf("method=%s", request.Method)
			}
			writeJSON(t, writer, http.StatusOK, pagedEnvelope([]Keyword{testKeyword(keywordStatus)}, 1))
		case "/api/v5/campaigns/2001/adgroups/3001/targetingkeywords/4001":
			switch request.Method {
			case http.MethodGet:
				writeJSON(t, writer, http.StatusOK, envelope(testKeyword(keywordStatus)))
			case http.MethodDelete:
				writeJSON(t, writer, http.StatusOK, envelope(nil))
			default:
				t.Errorf("Keyword method=%s", request.Method)
			}
		case "/api/v5/campaigns/2001/adgroups/3001/targetingkeywords/bulk":
			switch request.Method {
			case http.MethodPost:
				var payload []keywordCreate
				decodeRequest(t, request, &payload)
				if len(payload) != 1 || payload[0].Status != KeywordPaused || payload[0].Text != "hotels" {
					t.Errorf("Keyword create payload=%#v", payload)
				}
				created := testKeyword(KeywordPaused)
				created.ID, created.Text = 4002, "hotels"
				writeJSON(t, writer, http.StatusOK, envelope([]Keyword{created}))
			case http.MethodPut:
				var payload []keywordUpdate
				decodeRequest(t, request, &payload)
				if len(payload) != 1 || payload[0].ID != testKeywordID {
					t.Errorf("Keyword update payload=%#v", payload)
				}
				if payload[0].Status != nil {
					keywordStatus = *payload[0].Status
				}
				writeJSON(t, writer, http.StatusOK, envelope([]Keyword{testKeyword(keywordStatus)}))
			default:
				t.Errorf("Keyword bulk method=%s", request.Method)
			}
		case "/api/v5/creatives":
			handleCreativeCollection(t, writer, request)
		case "/api/v5/creatives/5001":
			if request.Method != http.MethodGet {
				t.Errorf("method=%s", request.Method)
			}
			writeJSON(t, writer, http.StatusOK, envelope(testCreative("VALID")))
		case "/api/v5/campaigns/2001/adgroups/3001/ads":
			handleAdCollection(t, writer, request, adStatus)
		case "/api/v5/campaigns/2001/adgroups/3001/ads/6001":
			switch request.Method {
			case http.MethodGet:
				writeJSON(t, writer, http.StatusOK, envelope(testAd(adStatus)))
			case http.MethodPut:
				var payload adWrite
				decodeRequest(t, request, &payload)
				if payload.Status != nil {
					adStatus = *payload.Status
				}
				writeJSON(t, writer, http.StatusOK, envelope(testAd(adStatus)))
			case http.MethodDelete:
				writeJSON(t, writer, http.StatusOK, envelope(nil))
			default:
				t.Errorf("Ad method=%s", request.Method)
			}
		case "/api/v5/reports/campaigns", "/api/v5/reports/campaigns/2001/adgroups",
			"/api/v5/reports/campaigns/2001/keywords", "/api/v5/reports/campaigns/2001/ads":
			if request.Method != http.MethodPost {
				t.Errorf("report method=%s", request.Method)
			}
			var payload ReportingRequest
			decodeRequest(t, request, &payload)
			if payload.StartTime != "2026-08-01" || payload.EndTime != "2026-08-02" {
				t.Errorf("report payload=%#v", payload)
			}
			writeJSON(t, writer, http.StatusOK, map[string]any{
				"data": map[string]any{"reportingDataResponse": map[string]any{
					"row": []ReportRow{{
						Metadata: ReportMetadata{OrgID: testOrgID, CampaignID: testCampaignID, CampaignName: "Search"},
						Total:    &SpendMetrics{Impressions: 10, Taps: 2},
					}},
					"grandTotals": map[string]any{"other": false, "total": SpendMetrics{Impressions: 10, Taps: 2}},
				}},
				"pagination": PageDetail{TotalResults: 1, StartIndex: 0, ItemsPerPage: 1}, "error": nil,
			})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newStaticClient(t, server)
	ctx := context.Background()

	acl, err := client.ListACL(ctx, Pagination{Limit: 20})
	if err != nil || len(acl.Items) != 1 || acl.Items[0].OrgID != testOrgID {
		t.Fatalf("ACL=%#v err=%v", acl, err)
	}
	campaigns, err := client.ListCampaigns(ctx, Pagination{Limit: 20})
	if err != nil || len(campaigns.Items) != 1 || !campaigns.HasMore {
		t.Fatalf("Campaigns=%#v err=%v", campaigns, err)
	}
	found, err := client.FindCampaigns(ctx, Selector{Pagination: Pagination{Limit: 20}})
	if err != nil || len(found.Items) != 1 {
		t.Fatalf("found=%#v err=%v", found, err)
	}
	if campaign, err := client.GetCampaign(ctx, testCampaignID); err != nil || campaign.ID != testCampaignID {
		t.Fatalf("Campaign=%#v err=%v", campaign, err)
	}
	createdCampaign, err := client.CreateCampaign(ctx, CreateCampaignRequest{
		Name: "New search", AdamID: testAdamID, DailyBudgetAmount: Money{Amount: "10", Currency: "USD"},
		BudgetAmount: &Money{Amount: "100", Currency: "USD"}, BillingEvent: "TAPS",
		SupplySources: []string{"APPSTORE_SEARCH_RESULTS"}, CountriesOrRegions: []string{"US"},
		AdChannelType: "SEARCH", BiddingStrategy: "MANUAL_CPT",
	}, socialhub.WithIdempotencyKey("campaign-create-1"))
	if err != nil || createdCampaign.Status != CampaignPaused || createdCampaign.ID != 2002 {
		t.Fatalf("created Campaign=%#v err=%v", createdCampaign, err)
	}
	if campaign, err := client.UpdateCampaign(ctx, testCampaignID, UpdateCampaignRequest{Name: stringPointer("Search 2")}); err != nil || campaign.ID != testCampaignID {
		t.Fatalf("updated Campaign=%#v err=%v", campaign, err)
	}

	groups, err := client.ListAdGroups(ctx, testCampaignID, Pagination{Limit: 20})
	if err != nil || len(groups.Items) != 1 {
		t.Fatalf("Ad Groups=%#v err=%v", groups, err)
	}
	if group, err := client.GetAdGroup(ctx, testCampaignID, testAdGroupID); err != nil || group.ID != testAdGroupID {
		t.Fatalf("Ad Group=%#v err=%v", group, err)
	}
	if _, err := client.CreateAdGroup(ctx, testCampaignID, CreateAdGroupRequest{
		Name: "New group", PricingModel: "CPC", DefaultBidAmount: &Money{Amount: "1", Currency: "USD"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.UpdateAdGroup(ctx, testCampaignID, testAdGroupID, UpdateAdGroupRequest{Name: stringPointer("Core 2")}); err != nil {
		t.Fatal(err)
	}
	if campaign, err := client.SetCampaignEnabled(ctx, testCampaignID, true); err != nil || campaign.Status != CampaignEnabled {
		t.Fatalf("enabled Campaign=%#v err=%v", campaign, err)
	}
	if group, err := client.SetAdGroupEnabled(ctx, testCampaignID, testAdGroupID, true); err != nil || group.Status != AdGroupEnabled {
		t.Fatalf("enabled Ad Group=%#v err=%v", group, err)
	}
	if _, err := client.SetAdGroupEnabled(ctx, testCampaignID, testAdGroupID, false); err != nil {
		t.Fatal(err)
	}

	keywords, err := client.ListKeywords(ctx, testCampaignID, testAdGroupID, Pagination{Limit: 20})
	if err != nil || len(keywords.Items) != 1 {
		t.Fatalf("Keywords=%#v err=%v", keywords, err)
	}
	if keyword, err := client.GetKeyword(ctx, testCampaignID, testAdGroupID, testKeywordID); err != nil || keyword.ID != testKeywordID {
		t.Fatalf("Keyword=%#v err=%v", keyword, err)
	}
	createdKeywords, err := client.CreateKeywords(ctx, testCampaignID, testAdGroupID, []CreateKeywordRequest{{
		Text: "hotels", MatchType: MatchExact, BidAmount: &Money{Amount: "1", Currency: "USD"},
	}})
	if err != nil || len(createdKeywords) != 1 || createdKeywords[0].Status != KeywordPaused {
		t.Fatalf("created Keywords=%#v err=%v", createdKeywords, err)
	}
	if _, err := client.SetAdGroupEnabled(ctx, testCampaignID, testAdGroupID, true); err != nil {
		t.Fatal(err)
	}
	activeKeyword := KeywordActive
	updatedKeywords, err := client.UpdateKeywords(ctx, testCampaignID, testAdGroupID, []UpdateKeywordRequest{{ID: testKeywordID, Status: &activeKeyword}})
	if err != nil || updatedKeywords[0].Status != KeywordActive {
		t.Fatalf("updated Keywords=%#v err=%v", updatedKeywords, err)
	}
	pausedKeyword := KeywordPaused
	if _, err := client.UpdateKeywords(ctx, testCampaignID, testAdGroupID, []UpdateKeywordRequest{{ID: testKeywordID, Status: &pausedKeyword}}); err != nil {
		t.Fatal(err)
	}
	if err := client.DeleteKeyword(ctx, testCampaignID, testAdGroupID, testKeywordID); err != nil {
		t.Fatal(err)
	}

	creatives, err := client.ListCreatives(ctx, Pagination{Limit: 20})
	if err != nil || len(creatives.Items) != 1 {
		t.Fatalf("Creatives=%#v err=%v", creatives, err)
	}
	if creative, err := client.GetCreative(ctx, testCreativeID); err != nil || creative.State != "VALID" {
		t.Fatalf("Creative=%#v err=%v", creative, err)
	}
	createdCreative, err := client.CreateCreative(ctx, CreateCreativeRequest{
		AdamID: testAdamID, Name: "New page", Type: CreativeCustomProductPage,
		ProductPageID: "d433d57c-0f5f-432d-a0ed-9085a088cabd",
	})
	if err != nil || createdCreative.ID != 5002 {
		t.Fatalf("created Creative=%#v err=%v", createdCreative, err)
	}

	ads, err := client.ListAds(ctx, testCampaignID, testAdGroupID, Pagination{Limit: 20})
	if err != nil || len(ads.Items) != 1 {
		t.Fatalf("Ads=%#v err=%v", ads, err)
	}
	if ad, err := client.GetAd(ctx, testCampaignID, testAdGroupID, testAdID); err != nil || ad.ID != testAdID {
		t.Fatalf("Ad=%#v err=%v", ad, err)
	}
	if _, err := client.SetAdGroupEnabled(ctx, testCampaignID, testAdGroupID, false); err != nil {
		t.Fatal(err)
	}
	createdAd, err := client.CreateAd(ctx, testCampaignID, testAdGroupID, CreateAdRequest{CreativeID: testCreativeID, Name: "New ad"})
	if err != nil || createdAd.Status != AdPaused || createdAd.ID != 6002 {
		t.Fatalf("created Ad=%#v err=%v", createdAd, err)
	}
	if _, err := client.UpdateAd(ctx, testCampaignID, testAdGroupID, testAdID, UpdateAdRequest{Name: stringPointer("Ad 2")}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.SetAdGroupEnabled(ctx, testCampaignID, testAdGroupID, true); err != nil {
		t.Fatal(err)
	}
	if ad, err := client.SetAdEnabled(ctx, testCampaignID, testAdGroupID, testAdID, true); err != nil || ad.Status != AdEnabled {
		t.Fatalf("enabled Ad=%#v err=%v", ad, err)
	}
	if _, err := client.SetAdEnabled(ctx, testCampaignID, testAdGroupID, testAdID, false); err != nil {
		t.Fatal(err)
	}
	if err := client.DeleteAd(ctx, testCampaignID, testAdGroupID, testAdID); err != nil {
		t.Fatal(err)
	}

	reportRequest := ReportingRequest{
		StartTime: "2026-08-01", EndTime: "2026-08-02", Selector: Selector{Pagination: Pagination{Limit: 20}},
		ReturnRowTotals: true, ReturnGrandTotals: true,
	}
	for name, invoke := range map[string]func() (*Report, error){
		"campaign": func() (*Report, error) { return client.CampaignReport(ctx, reportRequest) },
		"ad group": func() (*Report, error) { return client.AdGroupReport(ctx, testCampaignID, reportRequest) },
		"keyword":  func() (*Report, error) { return client.KeywordReport(ctx, testCampaignID, reportRequest) },
		"ad": func() (*Report, error) {
			input := reportRequest
			input.Selector.OrderBy = []Sorting{{Field: "adId", SortOrder: SortAscending}}
			return client.AdReport(ctx, testCampaignID, input)
		},
	} {
		t.Run(name, func(t *testing.T) {
			report, err := invoke()
			if err != nil || len(report.Rows) != 1 || report.Rows[0].Total.Impressions != 10 ||
				report.GrandTotals == nil || report.GrandTotals.Taps != 2 || report.Pagination.TotalResults != 1 {
				t.Fatalf("report=%#v err=%v", report, err)
			}
		})
	}

	if _, err := client.SetAdGroupEnabled(ctx, testCampaignID, testAdGroupID, false); err != nil {
		t.Fatal(err)
	}
	if err := client.DeleteAdGroup(ctx, testCampaignID, testAdGroupID); err != nil {
		t.Fatal(err)
	}
	if _, err := client.SetCampaignEnabled(ctx, testCampaignID, false); err != nil {
		t.Fatal(err)
	}
	if err := client.DeleteCampaign(ctx, testCampaignID); err != nil {
		t.Fatal(err)
	}
}

func handleCampaignCollection(t *testing.T, writer http.ResponseWriter, request *http.Request, status CampaignStatus) {
	t.Helper()
	switch request.Method {
	case http.MethodGet:
		writeJSON(t, writer, http.StatusOK, pagedEnvelope([]Campaign{testCampaign(status)}, 2))
	case http.MethodPost:
		var payload campaignCreate
		decodeRequest(t, request, &payload)
		if payload.Status != CampaignPaused || payload.OrgID != testOrgID || request.Header.Get("Idempotency-Key") != "campaign-create-1" {
			t.Errorf("Campaign create payload=%#v headers=%v", payload, request.Header)
		}
		created := testCampaign(CampaignPaused)
		created.ID, created.Name = 2002, payload.Name
		writeJSON(t, writer, http.StatusOK, envelope(created))
	default:
		t.Errorf("Campaign collection method=%s", request.Method)
	}
}

func handleAdGroupCollection(t *testing.T, writer http.ResponseWriter, request *http.Request, status AdGroupStatus) {
	t.Helper()
	switch request.Method {
	case http.MethodGet:
		writeJSON(t, writer, http.StatusOK, pagedEnvelope([]AdGroup{testAdGroup(status)}, 1))
	case http.MethodPost:
		var payload adGroupWrite
		decodeRequest(t, request, &payload)
		if payload.Status == nil || *payload.Status != AdGroupPaused || payload.OrgID != testOrgID || payload.CampaignID != testCampaignID {
			t.Errorf("Ad Group create payload=%#v", payload)
		}
		created := testAdGroup(AdGroupPaused)
		created.ID = 3002
		writeJSON(t, writer, http.StatusOK, envelope(created))
	default:
		t.Errorf("Ad Group collection method=%s", request.Method)
	}
}

func handleCreativeCollection(t *testing.T, writer http.ResponseWriter, request *http.Request) {
	t.Helper()
	switch request.Method {
	case http.MethodGet:
		writeJSON(t, writer, http.StatusOK, pagedEnvelope([]Creative{testCreative("VALID")}, 1))
	case http.MethodPost:
		var payload creativeCreate
		decodeRequest(t, request, &payload)
		creative := testCreative("VALID")
		creative.ID, creative.Name, creative.Type, creative.ProductPageID = 5002, payload.Name, payload.Type, payload.ProductPageID
		writeJSON(t, writer, http.StatusOK, envelope(creative))
	default:
		t.Errorf("Creative collection method=%s", request.Method)
	}
}

func handleAdCollection(t *testing.T, writer http.ResponseWriter, request *http.Request, status AdStatus) {
	t.Helper()
	switch request.Method {
	case http.MethodGet:
		writeJSON(t, writer, http.StatusOK, pagedEnvelope([]Ad{testAd(status)}, 1))
	case http.MethodPost:
		var payload adWrite
		decodeRequest(t, request, &payload)
		if payload.Status == nil || *payload.Status != AdPaused || payload.CreativeID != testCreativeID {
			t.Errorf("Ad create payload=%#v", payload)
		}
		created := testAd(AdPaused)
		created.ID = 6002
		writeJSON(t, writer, http.StatusOK, envelope(created))
	default:
		t.Errorf("Ad collection method=%s", request.Method)
	}
}

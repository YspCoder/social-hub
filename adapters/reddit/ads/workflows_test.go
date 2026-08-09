package ads

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestTypedWorkflowsHappyPath(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assertAPIRequest(t, request)
		if request.Method != http.MethodGet && request.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("Content-Type=%q", request.Header.Get("Content-Type"))
		}
		switch request.Method + " " + request.URL.Path {
		case "GET /api/v3/ad_accounts/" + testAdAccountID:
			writeJSON(t, writer, http.StatusOK, singleResponse[AdAccount]{Data: AdAccount{ID: testAdAccountID, Name: "Brand", Currency: "USD"}})
		case "GET /api/v3/ad_accounts/" + testAdAccountID + "/funding_instruments":
			response := listResponse[FundingInstrument]{Data: []FundingInstrument{{ID: testFundingInstrumentID, Name: "Credit", IsServable: true}}}
			if request.URL.Query().Get("funding_instrument_ids") == "" {
				next := server.URL + request.URL.Path + "?page.token=funding-next&page.size=10"
				response.Pagination.NextURL = &next
			}
			writeJSON(t, writer, http.StatusOK, response)
		case "GET /api/v3/ad_accounts/" + testAdAccountID + "/campaigns":
			writer.Header().Set("RateLimit-Policy", `"ads-campaign-management-read";q=400;w=60`)
			writer.Header().Set("RateLimit", `"ads-campaign-management-read";r=399;t=59`)
			next := server.URL + request.URL.Path + "?page.token=campaign-next&page.size=10"
			writeJSON(t, writer, http.StatusOK, listResponse[Campaign]{
				Data: []Campaign{campaignFixture("Listed", StatusPaused)}, Pagination: pagination{NextURL: &next},
			})
		case "GET /api/v3/campaigns/" + testCampaignID:
			writeJSON(t, writer, http.StatusOK, singleResponse[Campaign]{Data: campaignFixture("Current", StatusPaused)})
		case "POST /api/v3/ad_accounts/" + testAdAccountID + "/campaigns":
			var payload struct {
				Data createCampaignData `json:"data"`
			}
			decodeJSONBody(t, request, &payload)
			if payload.Data.ConfiguredStatus != StatusPaused || payload.Data.FundingInstrumentID != testFundingInstrumentID || payload.Data.Name != "Created Campaign" {
				t.Fatalf("Campaign payload=%#v", payload.Data)
			}
			writeJSON(t, writer, http.StatusCreated, singleResponse[Campaign]{Data: campaignFixture("Created Campaign", StatusPaused)})
		case "PATCH /api/v3/campaigns/" + testCampaignID:
			var payload struct {
				Data map[string]any `json:"data"`
			}
			decodeJSONBody(t, request, &payload)
			if payload.Data["name"] != "Updated Campaign" {
				t.Fatalf("Campaign patch=%#v", payload.Data)
			}
			writeJSON(t, writer, http.StatusOK, singleResponse[Campaign]{Data: campaignFixture("Updated Campaign", StatusPaused)})
		case "GET /api/v3/ad_accounts/" + testAdAccountID + "/ad_groups":
			writeJSON(t, writer, http.StatusOK, listResponse[AdGroup]{Data: []AdGroup{adGroupFixture("Listed", StatusPaused)}})
		case "GET /api/v3/ad_groups/" + testAdGroupID:
			writeJSON(t, writer, http.StatusOK, singleResponse[AdGroup]{Data: adGroupFixture("Current", StatusPaused)})
		case "POST /api/v3/ad_accounts/" + testAdAccountID + "/ad_groups":
			var payload struct {
				Data createAdGroupData `json:"data"`
			}
			decodeJSONBody(t, request, &payload)
			if payload.Data.ConfiguredStatus != StatusPaused || payload.Data.CampaignID != testCampaignID || payload.Data.ConversionPixelID != testPixelID {
				t.Fatalf("Ad Group payload=%#v", payload.Data)
			}
			writeJSON(t, writer, http.StatusCreated, singleResponse[AdGroup]{Data: adGroupFixture("Created Ad Group", StatusPaused)})
		case "PATCH /api/v3/ad_groups/" + testAdGroupID:
			var payload struct {
				Data map[string]any `json:"data"`
			}
			decodeJSONBody(t, request, &payload)
			if payload.Data["name"] != "Updated Ad Group" {
				t.Fatalf("Ad Group patch=%#v", payload.Data)
			}
			writeJSON(t, writer, http.StatusOK, singleResponse[AdGroup]{Data: adGroupFixture("Updated Ad Group", StatusPaused)})
		case "GET /api/v3/ad_accounts/" + testAdAccountID + "/ads":
			writeJSON(t, writer, http.StatusOK, listResponse[Ad]{Data: []Ad{adFixture("Listed", StatusPaused)}})
		case "GET /api/v3/ads/" + testAdID:
			writeJSON(t, writer, http.StatusOK, singleResponse[Ad]{Data: adFixture("Current", StatusPaused)})
		case "POST /api/v3/ad_accounts/" + testAdAccountID + "/ads":
			var payload struct {
				Data createAdData `json:"data"`
			}
			decodeJSONBody(t, request, &payload)
			if payload.Data.ConfiguredStatus != StatusPaused || payload.Data.AdGroupID != testAdGroupID || payload.Data.PostID != testPostID {
				t.Fatalf("Ad payload=%#v", payload.Data)
			}
			writeJSON(t, writer, http.StatusCreated, singleResponse[Ad]{Data: adFixture("Created Ad", StatusPaused)})
		case "PATCH /api/v3/ads/" + testAdID:
			var payload struct {
				Data map[string]any `json:"data"`
			}
			decodeJSONBody(t, request, &payload)
			if payload.Data["click_url"] != "https://example.test/new" {
				t.Fatalf("Ad patch=%#v", payload.Data)
			}
			value := adFixture("Current", StatusPaused)
			value.ClickURL = "https://example.test/new"
			writeJSON(t, writer, http.StatusOK, singleResponse[Ad]{Data: value})
		case "POST /api/v3/ad_accounts/" + testAdAccountID + "/reports":
			if request.URL.Query().Get("page.token") != "report-current" || request.URL.Query().Get("page.size") != "50" {
				t.Fatalf("Report query=%v", request.URL.Query())
			}
			var payload struct {
				Data reportData `json:"data"`
			}
			decodeJSONBody(t, request, &payload)
			if payload.Data.StartsAt != "2026-08-08T12:00:00Z" || len(payload.Data.Fields) != 2 || payload.Data.Fields[1] != FieldClicks {
				t.Fatalf("Report payload=%#v", payload.Data)
			}
			next := server.URL + request.URL.Path + "?page.token=report-next&page.size=50"
			writeJSON(t, writer, http.StatusOK, map[string]any{
				"data": map[string]any{
					"metrics":            []map[string]any{{"campaign_id": testCampaignID, "clicks": 42, "custom_metric": nil}},
					"metrics_updated_at": "2026-08-09T11:00:00Z",
				},
				"pagination": map[string]any{"next_url": next, "page_index": 0, "total_count": 1},
			})
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	ctx := context.Background()

	account, err := client.GetAdAccount(ctx)
	if err != nil || account.ID != testAdAccountID {
		t.Fatalf("account=%#v err=%v", account, err)
	}
	funding, err := client.ListFundingInstruments(ctx, ListRequest{PageSize: 10})
	if err != nil || len(funding.Items) != 1 || funding.NextCursor == nil || *funding.NextCursor != "funding-next" || funding.Items[0].AdAccountID != testAdAccountID {
		t.Fatalf("funding=%#v err=%v", funding, err)
	}
	selected, err := client.GetFundingInstrument(ctx, testFundingInstrumentID)
	if err != nil || selected.ID != testFundingInstrumentID {
		t.Fatalf("selected=%#v err=%v", selected, err)
	}
	campaigns, err := client.ListCampaigns(ctx, ListRequest{PageSize: 10})
	if err != nil || campaigns.NextCursor == nil || *campaigns.NextCursor != "campaign-next" {
		t.Fatalf("campaigns=%#v err=%v", campaigns, err)
	}
	if rate := client.RateLimit(); rate.Policy != "ads-campaign-management-read" || rate.Quota != 400 || rate.Remaining != 399 || rate.Reset != 59*time.Second || !rate.ObservedAt.Equal(testNow) {
		t.Fatalf("rate=%#v", rate)
	}
	if value, err := client.GetCampaign(ctx, testCampaignID); err != nil || value.ID != testCampaignID {
		t.Fatalf("Campaign=%#v err=%v", value, err)
	}
	createdCampaign, err := client.CreateCampaign(ctx, CreateCampaignRequest{
		FundingInstrumentID: testFundingInstrumentID, Name: "Created Campaign", Objective: ObjectiveClicks,
	})
	if err != nil || createdCampaign.ConfiguredStatus != StatusPaused {
		t.Fatalf("created Campaign=%#v err=%v", createdCampaign, err)
	}
	updatedCampaign, err := client.UpdateCampaign(ctx, testCampaignID, UpdateCampaignRequest{Name: stringPointer("Updated Campaign")})
	if err != nil || updatedCampaign.Name != "Updated Campaign" {
		t.Fatalf("updated Campaign=%#v err=%v", updatedCampaign, err)
	}

	groups, err := client.ListAdGroups(ctx, ListRequest{})
	if err != nil || len(groups.Items) != 1 {
		t.Fatalf("groups=%#v err=%v", groups, err)
	}
	if value, err := client.GetAdGroup(ctx, testAdGroupID); err != nil || value.ID != testAdGroupID {
		t.Fatalf("Ad Group=%#v err=%v", value, err)
	}
	createdGroup, err := client.CreateAdGroup(ctx, CreateAdGroupRequest{
		CampaignID: testCampaignID, Name: "Created Ad Group", BidType: BidTypeCPC,
		BidStrategy: bidStrategyPointer(BidStrategyManual), BidValue: int64Pointer(1_000_000),
		GoalType: GoalDailySpend, GoalValue: int64Pointer(10_000_000), StartTime: testNow, ConversionPixelID: testPixelID,
	})
	if err != nil || createdGroup.ConfiguredStatus != StatusPaused {
		t.Fatalf("created Ad Group=%#v err=%v", createdGroup, err)
	}
	updatedGroup, err := client.UpdateAdGroup(ctx, testAdGroupID, UpdateAdGroupRequest{Name: stringPointer("Updated Ad Group")})
	if err != nil || updatedGroup.Name != "Updated Ad Group" {
		t.Fatalf("updated Ad Group=%#v err=%v", updatedGroup, err)
	}

	ads, err := client.ListAds(ctx, ListRequest{})
	if err != nil || len(ads.Items) != 1 {
		t.Fatalf("ads=%#v err=%v", ads, err)
	}
	if value, err := client.GetAd(ctx, testAdID); err != nil || value.ID != testAdID {
		t.Fatalf("Ad=%#v err=%v", value, err)
	}
	createdAd, err := client.CreateAd(ctx, CreateAdRequest{AdGroupID: testAdGroupID, Name: "Created Ad", PostID: testPostID, ClickURL: "https://example.test"})
	if err != nil || createdAd.ConfiguredStatus != StatusPaused {
		t.Fatalf("created Ad=%#v err=%v", createdAd, err)
	}
	updatedAd, err := client.UpdateAd(ctx, testAdID, UpdateAdRequest{ClickURL: stringPointer("https://example.test/new")})
	if err != nil || updatedAd.ClickURL != "https://example.test/new" {
		t.Fatalf("updated Ad=%#v err=%v", updatedAd, err)
	}

	report, err := client.GetReport(ctx, ReportRequest{
		StartsAt: testNow.Add(-24 * time.Hour), EndsAt: testNow,
		Fields: []ReportField{FieldCampaignID, FieldClicks}, Breakdowns: []ReportBreakdown{BreakdownCampaignID},
		Cursor: "report-current", PageSize: 50,
	})
	if err != nil || len(report.Metrics) != 1 || report.NextCursor == nil || *report.NextCursor != "report-next" || report.TotalCount == nil || *report.TotalCount != 1 ||
		string(report.Metrics[0]["clicks"]) != "42" || string(report.Metrics[0]["custom_metric"]) != "null" {
		t.Fatalf("report=%#v err=%v", report, err)
	}
}

func TestMutationOwnershipAndRequiredInputsStopWrites(t *testing.T) {
	var writes atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assertAPIRequest(t, request)
		if request.Method != http.MethodGet {
			writes.Add(1)
			t.Fatalf("unsafe write reached network: %s %s", request.Method, request.URL)
		}
		switch request.URL.Path {
		case "/api/v3/ad_accounts/" + testAdAccountID + "/funding_instruments":
			writeJSON(t, writer, http.StatusOK, listResponse[FundingInstrument]{Data: []FundingInstrument{{ID: "999"}}})
		case "/api/v3/campaigns/" + testCampaignID:
			value := campaignFixture("Wrong owner", StatusPaused)
			value.AdAccountID = "a2_other"
			writeJSON(t, writer, http.StatusOK, singleResponse[Campaign]{Data: value})
		case "/api/v3/ad_groups/" + testAdGroupID:
			value := adGroupFixture("Wrong owner", StatusPaused)
			value.AdAccountID = "a2_other"
			writeJSON(t, writer, http.StatusOK, singleResponse[AdGroup]{Data: value})
		default:
			t.Fatalf("unexpected read: %s", request.URL)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	ctx := context.Background()

	_, err := client.CreateCampaign(ctx, CreateCampaignRequest{FundingInstrumentID: testFundingInstrumentID, Name: "Campaign", Objective: ObjectiveClicks})
	if hubError(t, err).Code != socialhub.CodePlatformError {
		t.Fatalf("Funding error=%v", err)
	}
	_, err = client.UpdateCampaign(ctx, testCampaignID, UpdateCampaignRequest{Name: stringPointer("new")})
	if hubError(t, err).Code != socialhub.CodePlatformError {
		t.Fatalf("Campaign owner error=%v", err)
	}
	_, err = client.CreateAdGroup(ctx, CreateAdGroupRequest{
		CampaignID: testCampaignID, Name: "Group", BidType: BidTypeCPC, BidStrategy: bidStrategyPointer(BidStrategyManual),
		GoalType: GoalDailySpend, GoalValue: int64Pointer(100), StartTime: testNow, ConversionPixelID: testPixelID,
	})
	if hubError(t, err).Code != socialhub.CodePlatformError {
		t.Fatalf("Campaign parent error=%v", err)
	}
	_, err = client.CreateAd(ctx, CreateAdRequest{AdGroupID: testAdGroupID, Name: "Ad", PostID: testPostID})
	if hubError(t, err).Code != socialhub.CodePlatformError {
		t.Fatalf("Ad Group parent error=%v", err)
	}
	if writes.Load() != 0 {
		t.Fatalf("writes=%d", writes.Load())
	}
}

func TestPaginationURLValidation(t *testing.T) {
	values := []func(*httptest.Server, string) string{
		func(_ *httptest.Server, path string) string {
			return "https://evil.example" + path + "?page.token=next"
		},
		func(server *httptest.Server, _ string) string { return server.URL + "/api/v3/ads? page.token=next" },
		func(server *httptest.Server, path string) string { return server.URL + path + "?page.size=10" },
		func(server *httptest.Server, path string) string {
			return server.URL + path + "?page.token=one&page.token=two"
		},
	}
	for index, nextURL := range values {
		var server *httptest.Server
		server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			next := nextURL(server, request.URL.Path)
			writeJSON(t, writer, http.StatusOK, listResponse[Campaign]{
				Data: []Campaign{campaignFixture("Campaign", StatusPaused)}, Pagination: pagination{NextURL: &next},
			})
		}))
		_, client := newTestAdapter(t, server)
		_, err := client.ListCampaigns(context.Background(), ListRequest{})
		if err == nil || hubError(t, err).Code != socialhub.CodePlatformError {
			t.Errorf("case %d error=%v", index, err)
		}
		server.Close()
	}
}

func TestLocalValidationAvoidsNetwork(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("invalid input reached network") }))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	invalidReport := ReportRequest{StartsAt: testNow.Add(-time.Hour), EndsAt: testNow, Fields: []ReportField{FieldClicks}}
	calls := []func() error{
		func() error {
			_, err := client.ListCampaigns(context.Background(), ListRequest{Cursor: " bad"})
			return err
		},
		func() error { _, err := client.GetCampaign(context.Background(), "bad"); return err },
		func() error {
			_, err := client.CreateCampaign(context.Background(), CreateCampaignRequest{})
			return err
		},
		func() error {
			value := CreateCampaignRequest{
				FundingInstrumentID: testFundingInstrumentID, Name: "CBO", Objective: ObjectiveClicks,
				IsCampaignBudgetOptimization: true, GoalType: GoalDailySpend, GoalValue: int64Pointer(100),
				StartTime: &testNow, BidStrategy: BidStrategyManual, BidType: BidTypeCPC, ConversionPixelID: testPixelID,
			}
			_, err := client.CreateCampaign(context.Background(), value)
			return err
		},
		func() error {
			_, err := client.UpdateCampaign(context.Background(), testCampaignID, UpdateCampaignRequest{})
			return err
		},
		func() error {
			_, err := client.UpdateCampaign(context.Background(), testCampaignID, UpdateCampaignRequest{Status: statusPointer(StatusArchived)})
			return err
		},
		func() error { _, err := client.GetFundingInstrument(context.Background(), "bad"); return err },
		func() error { _, err := client.GetAdGroup(context.Background(), "bad"); return err },
		func() error { _, err := client.CreateAdGroup(context.Background(), CreateAdGroupRequest{}); return err },
		func() error {
			_, err := client.UpdateAdGroup(context.Background(), testAdGroupID, UpdateAdGroupRequest{})
			return err
		},
		func() error { _, err := client.GetAd(context.Background(), "bad"); return err },
		func() error { _, err := client.CreateAd(context.Background(), CreateAdRequest{}); return err },
		func() error { _, err := client.UpdateAd(context.Background(), testAdID, UpdateAdRequest{}); return err },
		func() error {
			value := invalidReport
			value.StartsAt = value.StartsAt.Add(time.Minute)
			_, err := client.GetReport(context.Background(), value)
			return err
		},
		func() error {
			value := invalidReport
			value.Fields = []ReportField{FieldClicks, FieldClicks}
			_, err := client.GetReport(context.Background(), value)
			return err
		},
		func() error {
			value := invalidReport
			value.Breakdowns = []ReportBreakdown{BreakdownCampaignID, BreakdownDate, BreakdownHour, BreakdownPlacement}
			_, err := client.GetReport(context.Background(), value)
			return err
		},
	}
	for index, call := range calls {
		if err := call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Errorf("call %d error=%v", index, err)
		}
	}
	client.clock = fixedClock{value: time.Date(2026, time.October, 30, 0, 0, 0, 0, time.UTC)}
	invalidReport.StartsAt = invalidReport.EndsAt.Add(-8 * 24 * time.Hour)
	invalidReport.Breakdowns = []ReportBreakdown{BreakdownHour}
	if _, err := client.GetReport(context.Background(), invalidReport); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("post-cutoff HOUR range error=%v", err)
	}
}

func TestCBOValidationAndPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assertAPIRequest(t, request)
		switch request.Method + " " + request.URL.Path {
		case "GET /api/v3/ad_accounts/" + testAdAccountID + "/funding_instruments":
			writeJSON(t, writer, http.StatusOK, listResponse[FundingInstrument]{Data: []FundingInstrument{{ID: testFundingInstrumentID}}})
		case "POST /api/v3/ad_accounts/" + testAdAccountID + "/campaigns":
			var payload struct {
				Data createCampaignData `json:"data"`
			}
			decodeJSONBody(t, request, &payload)
			if !payload.Data.IsCampaignBudgetOptimization || payload.Data.ConfiguredStatus != StatusPaused || payload.Data.ConversionPixelID != testPixelID {
				t.Fatalf("CBO payload=%#v", payload.Data)
			}
			writeJSON(t, writer, http.StatusCreated, singleResponse[Campaign]{Data: cboCampaignFixture()})
		case "GET /api/v3/campaigns/" + testCampaignID:
			writeJSON(t, writer, http.StatusOK, singleResponse[Campaign]{Data: cboCampaignFixture()})
		case "POST /api/v3/ad_accounts/" + testAdAccountID + "/ad_groups":
			var payload struct {
				Data createAdGroupData `json:"data"`
			}
			decodeJSONBody(t, request, &payload)
			if payload.Data.BidStrategy != nil || payload.Data.BidValue != nil {
				t.Fatalf("CBO Ad Group payload=%#v", payload.Data)
			}
			writeJSON(t, writer, http.StatusCreated, singleResponse[AdGroup]{Data: adGroupFixture("CBO Group", StatusPaused)})
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	end := testNow.Add(24 * time.Hour)
	campaign, err := client.CreateCampaign(context.Background(), CreateCampaignRequest{
		FundingInstrumentID: testFundingInstrumentID, Name: "CBO Campaign", Objective: ObjectiveImpressions,
		IsCampaignBudgetOptimization: true, GoalType: GoalLifetimeSpend, GoalValue: int64Pointer(10_000_000),
		StartTime: &testNow, EndTime: &end, BidStrategy: BidStrategyBidless, BidType: BidTypeCPM, ConversionPixelID: testPixelID,
	})
	if err != nil || !campaignCBO(*campaign) {
		t.Fatalf("Campaign=%#v err=%v", campaign, err)
	}
	group, err := client.CreateAdGroup(context.Background(), CreateAdGroupRequest{
		CampaignID: testCampaignID, Name: "CBO Group", BidType: BidTypeCPM, StartTime: testNow, ConversionPixelID: testPixelID,
	})
	if err != nil || group.ConfiguredStatus != StatusPaused {
		t.Fatalf("Ad Group=%#v err=%v", group, err)
	}
}

func TestMutationResponseSafetyContracts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.Method + " " + request.URL.Path {
		case "GET /api/v3/ad_accounts/" + testAdAccountID + "/funding_instruments":
			writeJSON(t, writer, http.StatusOK, listResponse[FundingInstrument]{Data: []FundingInstrument{{ID: testFundingInstrumentID}}})
		case "POST /api/v3/ad_accounts/" + testAdAccountID + "/campaigns":
			writeJSON(t, writer, http.StatusCreated, singleResponse[Campaign]{Data: campaignFixture("unsafe", StatusActive)})
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	_, err := client.CreateCampaign(context.Background(), CreateCampaignRequest{
		FundingInstrumentID: testFundingInstrumentID, Name: "Campaign", Objective: ObjectiveClicks,
	})
	if err == nil || !strings.Contains(hubError(t, err).PlatformMessage, "PAUSED") {
		t.Fatalf("safety error=%v", err)
	}
}

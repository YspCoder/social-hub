package marketing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestPaidMediaWorkflows(t *testing.T) {
	campaignName, adSquadName, adName := "Launch", "US Prospects", "Opening frame"
	campaignStatus, adSquadStatus, adStatus := StatusPaused, StatusPaused, StatusPaused
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assertAPIRequest(t, request)
		switch {
		case request.URL.Path == "/adaccounts/"+testAdAccountID && request.Method == http.MethodGet:
			writeValue(t, writer, http.StatusOK, successEnvelope("adaccounts", "adaccount", map[string]any{
				"id": testAdAccountID, "name": "Snap Ads", "currency": "USD", "timezone": "America/Los_Angeles",
			}))
		case request.URL.Path == "/adaccounts/"+testAdAccountID+"/campaigns" && request.Method == http.MethodGet:
			if request.URL.Query().Get("cursor") != "incoming" || request.URL.Query().Get("limit") != "50" || request.Header.Get("X-Request-ID") != "caller-request" {
				t.Errorf("campaign list query=%v headers=%v", request.URL.Query(), request.Header)
			}
			response := successEnvelope("campaigns", "campaign", campaignValue(testAdAccountID, campaignStatus, campaignName))
			response["paging"] = map[string]any{"next_link": server.URL + "/adaccounts/" + testAdAccountID + "/campaigns?cursor=next-campaign&limit=50"}
			writeValue(t, writer, http.StatusOK, response)
		case request.URL.Path == "/adaccounts/"+testAdAccountID+"/campaigns" && request.Method == http.MethodPost:
			resource := oneResource(t, decodeJSONMap(t, request), "campaigns")
			objective := resource["objective_v2_properties"].(map[string]any)
			if resource["ad_account_id"] != testAdAccountID || resource["status"] != "PAUSED" || resource["buy_model"] != "AUCTION" ||
				resource["creation_state"] != "PUBLISHED" || objective["objective_v2_type"] != "AWARENESS_AND_ENGAGEMENT" {
				t.Errorf("campaign create=%v", resource)
			}
			campaignName, campaignStatus = resource["name"].(string), StatusPaused
			writeValue(t, writer, http.StatusOK, successEnvelope("campaigns", "campaign", campaignValue(testAdAccountID, campaignStatus, campaignName)))
		case request.URL.Path == "/campaigns/"+testCampaignID && request.Method == http.MethodGet:
			writeValue(t, writer, http.StatusOK, successEnvelope("campaigns", "campaign", campaignValue(testAdAccountID, campaignStatus, campaignName)))
		case request.URL.Path == "/adaccounts/"+testAdAccountID+"/campaigns/"+testCampaignID && request.Method == http.MethodPatch:
			applyEntityPatch(t, decodePatch(t, request), &campaignName, &campaignStatus)
			writeValue(t, writer, http.StatusOK, successEnvelope("campaigns", "campaign", campaignValue(testAdAccountID, campaignStatus, campaignName)))
		case request.URL.Path == "/adaccounts/"+testAdAccountID+"/adsquads" && request.Method == http.MethodGet:
			writeValue(t, writer, http.StatusOK, successEnvelope("adsquads", "adsquad", adSquadValue(adSquadStatus, adSquadName)))
		case request.URL.Path == "/adsquads/"+testAdSquadID && request.Method == http.MethodGet:
			writeValue(t, writer, http.StatusOK, successEnvelope("adsquads", "adsquad", adSquadValue(adSquadStatus, adSquadName)))
		case request.URL.Path == "/campaigns/"+testCampaignID+"/adsquads" && request.Method == http.MethodPost:
			resource := oneResource(t, decodeJSONMap(t, request), "adsquads")
			placement := resource["placement_v2"].(map[string]any)
			targeting := resource["targeting"].(map[string]any)
			geos := targeting["geos"].([]any)
			if resource["campaign_id"] != testCampaignID || resource["status"] != "PAUSED" || resource["type"] != "SNAP_ADS" ||
				placement["config"] != "AUTOMATIC" || resource["optimization_goal"] != "IMPRESSIONS" ||
				resource["billing_event"] != "IMPRESSION" || resource["bid_strategy"] != "LOWEST_COST_WITH_MAX_BID" ||
				resource["delivery_constraint"] != "DAILY_BUDGET" || resource["bid_micro"] != json.Number("1000000") ||
				resource["daily_budget_micro"] != json.Number("50000000") || geos[0].(map[string]any)["country_code"] != "us" {
				t.Errorf("Ad Squad create=%v", resource)
			}
			adSquadName, adSquadStatus = resource["name"].(string), StatusPaused
			writeValue(t, writer, http.StatusOK, successEnvelope("adsquads", "adsquad", adSquadValue(adSquadStatus, adSquadName)))
		case request.URL.Path == "/campaigns/"+testCampaignID+"/adsquads/"+testAdSquadID && request.Method == http.MethodPatch:
			applyEntityPatch(t, decodePatch(t, request), &adSquadName, &adSquadStatus)
			writeValue(t, writer, http.StatusOK, successEnvelope("adsquads", "adsquad", adSquadValue(adSquadStatus, adSquadName)))
		case request.URL.Path == "/adaccounts/"+testAdAccountID+"/ads" && request.Method == http.MethodGet:
			writeValue(t, writer, http.StatusOK, successEnvelope("ads", "ad", adValue(adStatus, adName)))
		case request.URL.Path == "/ads/"+testAdID && request.Method == http.MethodGet:
			writeValue(t, writer, http.StatusOK, successEnvelope("ads", "ad", adValue(adStatus, adName)))
		case request.URL.Path == "/adsquads/"+testAdSquadID+"/ads" && request.Method == http.MethodPost:
			resource := oneResource(t, decodeJSONMap(t, request), "ads")
			if resource["ad_squad_id"] != testAdSquadID || resource["creative_id"] != testCreativeID || resource["status"] != "PAUSED" || resource["type"] != "SNAP_AD" {
				t.Errorf("Ad create=%v", resource)
			}
			adName, adStatus = resource["name"].(string), StatusPaused
			writeValue(t, writer, http.StatusOK, successEnvelope("ads", "ad", adValue(adStatus, adName)))
		case request.URL.Path == "/adsquads/"+testAdSquadID+"/ads/"+testAdID && request.Method == http.MethodPatch:
			applyEntityPatch(t, decodePatch(t, request), &adName, &adStatus)
			writeValue(t, writer, http.StatusOK, successEnvelope("ads", "ad", adValue(adStatus, adName)))
		case request.URL.Path == "/adaccounts/"+testAdAccountID+"/stats" && request.Method == http.MethodGet:
			query := request.URL.Query()
			if query.Get("granularity") != "DAY" || query.Get("fields") != "impressions,spend" || query.Get("limit") != "200" ||
				query.Get("start_time") != "2026-08-01T00:00:00Z" || query.Get("end_time") != "2026-08-02T00:00:00Z" {
				t.Errorf("stats query=%v", query)
			}
			writeValue(t, writer, http.StatusOK, map[string]any{
				"request_status": "SUCCESS", "request_id": "stats-request",
				"total_stats": []any{map[string]any{"sub_request_status": "SUCCESS", "total_stat": map[string]any{
					"id": testAdAccountID, "type": "AD_ACCOUNT", "granularity": "TOTAL", "stats": map[string]any{"impressions": 1200, "spend": 12.5},
				}}},
				"timeseries_stats": []any{map[string]any{"sub_request_status": "SUCCESS", "timeseries_stat": map[string]any{
					"id": testAdAccountID, "type": "AD_ACCOUNT", "granularity": "DAY", "timeseries": []any{map[string]any{
						"start_time": "2026-08-01T00:00:00Z", "end_time": "2026-08-02T00:00:00Z", "stats": map[string]any{"impressions": 1200, "spend": 12.5},
					}},
				}}},
			})
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	ctx := context.Background()

	account, err := client.GetAdAccount(ctx)
	if err != nil || account.ID != testAdAccountID || account.Currency != "USD" {
		t.Fatalf("account=%#v err=%v", account, err)
	}
	campaigns, err := client.ListCampaigns(ctx, ListRequest{Cursor: "incoming", Limit: 50}, socialhub.WithRequestID("caller-request"))
	if err != nil || len(campaigns.Items) != 1 || !campaigns.HasMore || campaigns.NextCursor == nil || *campaigns.NextCursor != "next-campaign" {
		t.Fatalf("campaigns=%#v err=%v", campaigns, err)
	}
	campaign, err := client.CreateCampaign(ctx, CreateCampaignRequest{
		Name: "Launch 2", Objective: ObjectiveAwarenessAndEngagement, StartTime: testNow.Add(time.Hour),
	})
	if err != nil || campaign.Status != StatusPaused {
		t.Fatalf("Campaign=%#v err=%v", campaign, err)
	}
	newCampaignName := "Launch renamed"
	if _, err = client.UpdateCampaign(ctx, testCampaignID, UpdateEntityRequest{Name: &newCampaignName}); err != nil {
		t.Fatal(err)
	}
	if _, err = client.SetCampaignStatus(ctx, testCampaignID, StatusActive); err != nil || campaignStatus != StatusActive {
		t.Fatalf("Campaign status=%s err=%v", campaignStatus, err)
	}

	adSquads, err := client.ListAdSquads(ctx, ListRequest{})
	if err != nil || len(adSquads.Items) != 1 || adSquads.Items[0].AdAccountID != testAdAccountID {
		t.Fatalf("Ad Squads=%#v err=%v", adSquads, err)
	}
	adSquad, err := client.CreateAdSquad(ctx, CreateAdSquadRequest{
		CampaignID: testCampaignID, Name: "US Launch", BidMicro: 1000000,
		DailyBudgetMicro: 50000000, CountryCodes: []string{"US"}, StartTime: testNow.Add(2 * time.Hour),
	})
	if err != nil || adSquad.Status != StatusPaused {
		t.Fatalf("Ad Squad=%#v err=%v", adSquad, err)
	}
	newAdSquadName := "US Launch renamed"
	if _, err = client.UpdateAdSquad(ctx, testAdSquadID, UpdateEntityRequest{Name: &newAdSquadName}); err != nil {
		t.Fatal(err)
	}
	if _, err = client.SetAdSquadStatus(ctx, testAdSquadID, StatusActive); err != nil || adSquadStatus != StatusActive {
		t.Fatalf("Ad Squad status=%s err=%v", adSquadStatus, err)
	}

	ads, err := client.ListAds(ctx, ListRequest{})
	if err != nil || len(ads.Items) != 1 || ads.Items[0].AdAccountID != testAdAccountID {
		t.Fatalf("Ads=%#v err=%v", ads, err)
	}
	ad, err := client.CreateAd(ctx, CreateAdRequest{AdSquadID: testAdSquadID, CreativeID: testCreativeID, Name: "Sponsored Snap"})
	if err != nil || ad.Status != StatusPaused {
		t.Fatalf("Ad=%#v err=%v", ad, err)
	}
	newAdName := "Sponsored Snap renamed"
	if _, err = client.UpdateAd(ctx, testAdID, UpdateEntityRequest{Name: &newAdName}); err != nil {
		t.Fatal(err)
	}
	if _, err = client.SetAdStatus(ctx, testAdID, StatusActive); err != nil || adStatus != StatusActive {
		t.Fatalf("Ad status=%s err=%v", adStatus, err)
	}

	start, end := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)
	stats, err := client.GetAccountStats(ctx, StatsRequest{
		Granularity: GranularityDay, StartTime: &start, EndTime: &end,
		Fields: []string{"impressions", "spend"}, Limit: 200,
	})
	if err != nil || len(stats.Totals) != 1 || len(stats.Timeseries) != 1 ||
		string(stats.Totals[0].Stats["spend"]) != "12.5" || string(stats.Timeseries[0].Timeseries[0].Stats["impressions"]) != "1200" {
		t.Fatalf("stats=%#v err=%v", stats, err)
	}
}

func TestOwnershipRejectedBeforeMutation(t *testing.T) {
	patches := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assertAPIRequest(t, request)
		switch request.URL.Path {
		case "/ads/" + testAdID:
			writeValue(t, writer, http.StatusOK, successEnvelope("ads", "ad", adValue(StatusPaused, "Ad")))
		case "/adsquads/" + testAdSquadID:
			writeValue(t, writer, http.StatusOK, successEnvelope("adsquads", "adsquad", adSquadValue(StatusPaused, "Squad")))
		case "/campaigns/" + testCampaignID:
			writeValue(t, writer, http.StatusOK, successEnvelope("campaigns", "campaign", campaignValue(otherAccountID, StatusPaused, "Foreign")))
		default:
			if request.Method == http.MethodPatch || request.Method == http.MethodPost {
				patches++
			}
			t.Fatalf("unexpected request %s %s", request.Method, request.URL)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	status := StatusActive
	_, err := client.UpdateAd(context.Background(), testAdID, UpdateEntityRequest{Status: &status})
	if hubError(t, err).Code != socialhub.CodePlatformError || patches != 0 {
		t.Fatalf("ownership error=%v mutation requests=%d", err, patches)
	}
}

func TestSubrequestFailureAndHostilePagination(t *testing.T) {
	tests := []struct {
		name     string
		response map[string]any
		code     socialhub.ErrorCode
	}{
		{
			name: "subrequest failure",
			response: map[string]any{
				"request_status": "SUCCESS", "request_id": "sub-request",
				"campaigns": []any{map[string]any{
					"sub_request_status": "RATE_LIMIT_EXCEEDED",
					"errors":             []any{map[string]any{"error_code": "RATE_LIMIT_EXCEEDED", "message": "quota"}},
				}},
			},
			code: socialhub.CodeRateLimited,
		},
		{
			name: "hostile pagination",
			response: func() map[string]any {
				value := successEnvelope("campaigns", "campaign", campaignValue(testAdAccountID, StatusPaused, "Campaign"))
				value["paging"] = map[string]any{"next_link": "https://attacker.example/adaccounts/" + testAdAccountID + "/campaigns?cursor=stolen"}
				return value
			}(),
			code: socialhub.CodePlatformError,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				writeValue(t, writer, http.StatusOK, test.response)
			}))
			defer server.Close()
			_, client := newTestAdapter(t, server)
			_, err := client.ListCampaigns(context.Background(), ListRequest{})
			if hubError(t, err).Code != test.code {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func oneResource(t *testing.T, envelope map[string]any, key string) map[string]any {
	t.Helper()
	items, ok := envelope[key].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("%s envelope=%v", key, envelope)
	}
	return items[0].(map[string]any)
}

func applyEntityPatch(t *testing.T, operations []map[string]any, name *string, status *EntityStatus) {
	t.Helper()
	for _, operation := range operations {
		switch operation["path"] {
		case "/name":
			*name = operation["value"].(string)
		case "/status":
			*status = EntityStatus(operation["value"].(string))
		}
	}
}

func TestMutationRequiresExactlyOneSuccessfulResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPost {
			writeValue(t, writer, http.StatusOK, map[string]any{"request_status": "SUCCESS", "campaigns": []any{}})
			return
		}
		writeValue(t, writer, http.StatusOK, successEnvelope("campaigns", "campaign", campaignValue(testAdAccountID, StatusPaused, "Campaign")))
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	_, err := client.CreateCampaign(context.Background(), CreateCampaignRequest{
		Name: "Campaign", Objective: ObjectiveAwarenessAndEngagement, StartTime: testNow.Add(time.Hour),
	})
	if hubError(t, err).Code != socialhub.CodePlatformError || !strings.Contains(hubError(t, err).PlatformMessage, "exactly one") {
		t.Fatalf("error=%v", err)
	}
}

func TestPaginationPathAndCursorContracts(t *testing.T) {
	nextLinks := []string{
		"PLACEHOLDER/adaccounts/" + testAdAccountID + "/ads?cursor=one&cursor=two",
		"PLACEHOLDER/other?cursor=one",
	}
	for _, link := range nextLinks {
		var server *httptest.Server
		server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			value := successEnvelope("ads", "ad", adValue(StatusPaused, "Ad"))
			value["paging"] = map[string]any{"next_link": strings.Replace(link, "PLACEHOLDER", server.URL, 1)}
			writeValue(t, writer, http.StatusOK, value)
		}))
		_, client := newTestAdapter(t, server)
		_, err := client.ListAds(context.Background(), ListRequest{})
		server.Close()
		if hubError(t, err).Code != socialhub.CodePlatformError {
			t.Fatalf("link=%s error=%v", link, err)
		}
	}
}

func TestStatsOwnershipContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writeValue(t, writer, http.StatusOK, map[string]any{
			"request_status": "SUCCESS",
			"total_stats":    []any{map[string]any{"sub_request_status": "SUCCESS", "total_stat": map[string]any{"id": otherAccountID, "stats": map[string]any{"spend": 1}}}},
		})
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	_, err := client.GetAccountStats(context.Background(), StatsRequest{Granularity: GranularityTotal, Fields: []string{"spend"}})
	if hubError(t, err).Code != socialhub.CodePlatformError {
		t.Fatalf("error=%v", err)
	}
}

func TestRequestRedirectDoesNotForwardBearer(t *testing.T) {
	forwarded := ""
	destination := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		forwarded = request.Header.Get("Authorization")
	}))
	defer destination.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, destination.URL, http.StatusFound)
	}))
	defer origin.Close()
	_, client := newTestAdapter(t, origin)
	_, err := client.GetAdAccount(context.Background())
	if err == nil || forwarded != "" {
		t.Fatalf("redirect error=%v forwarded=%q", err, forwarded)
	}
}

func TestStatsTimeAlignmentUsesExactHours(t *testing.T) {
	if !hourAligned(time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)) || hourAligned(time.Date(2026, 8, 1, 0, 1, 0, 0, time.UTC)) {
		t.Fatal("hour alignment validation mismatch")
	}
}

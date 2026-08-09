package marketing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"testing"
)

func TestPaidMediaWorkflows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assertAPIRequest(t, request)
		switch request.URL.Path {
		case "/v1.3/advertiser/info/":
			if request.Method != http.MethodGet || request.URL.Query().Get("advertiser_ids") != `["123456789"]` {
				t.Errorf("advertiser query=%s", request.URL.RawQuery)
			}
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"list":[{"advertiser_id":"123456789","name":"Brand"}]}}`)
		case "/v1.3/campaign/get/":
			assertJSONQuery(t, request, "filtering", map[string]any{"campaign_ids": []any{"101"}, "campaign_name": "Launch"})
			assertJSONQuery(t, request, "fields", []any{"campaign_id", "campaign_name"})
			writeJSON(writer, http.StatusOK, pageResponse(`[{"campaign_id":"101","campaign_name":"Launch","operation_status":"DISABLE"}]`, 2, true))
		case "/v1.3/campaign/create/":
			body := decodeBody(t, request)
			if body["operation_status"] != "DISABLE" || body["objective_type"] != "TRAFFIC" || body["campaign_name"] != "Launch" || body["is_search_campaign"] != true {
				t.Errorf("campaign create=%v", body)
			}
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"campaign_id":"101","operation_status":"DISABLE"}}`)
		case "/v1.3/campaign/update/":
			body := decodeBody(t, request)
			if body["campaign_id"] != "101" || body["campaign_name"] != "Launch 2" || body["po_number"] != "PO-1" {
				t.Errorf("campaign update=%v", body)
			}
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"campaign_id":"101","campaign_name":"Launch 2"}}`)
		case "/v1.3/campaign/status/update/":
			body := decodeBody(t, request)
			assertBodyIDs(t, body, "campaign_ids", "101")
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"campaign_ids":["101"],"status":"ENABLE"}}`)
		case "/v1.3/adgroup/get/":
			assertJSONQuery(t, request, "filtering", map[string]any{"adgroup_ids": []any{"201"}, "campaign_ids": []any{"101"}})
			writeJSON(writer, http.StatusOK, pageResponse(`[{"adgroup_id":"201","campaign_id":"101","adgroup_name":"Prospecting","operation_status":"DISABLE"}]`, 1, false))
		case "/v1.3/adgroup/create/":
			body := decodeBody(t, request)
			if body["campaign_id"] != "101" || body["operation_status"] != "DISABLE" || body["promotion_type"] != "WEBSITE" || body["age_groups"] == nil {
				t.Errorf("Ad Group create=%v", body)
			}
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"adgroup_id":"201","campaign_id":"101","operation_status":"DISABLE"}}`)
		case "/v1.3/adgroup/update/":
			body := decodeBody(t, request)
			if body["adgroup_id"] != "201" || body["adgroup_name"] != "Prospecting 2" || body["pacing"] != "PACING_MODE_SMOOTH" {
				t.Errorf("Ad Group update=%v", body)
			}
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"adgroup_id":"201","adgroup_name":"Prospecting 2"}}`)
		case "/v1.3/adgroup/status/update/":
			body := decodeBody(t, request)
			assertBodyIDs(t, body, "adgroup_ids", "201")
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"adgroup_ids":["201"],"status":"DISABLE"}}`)
		case "/v1.3/ad/get/":
			assertJSONQuery(t, request, "filtering", map[string]any{"ad_ids": []any{"301"}, "adgroup_ids": []any{"201"}})
			writeJSON(writer, http.StatusOK, pageResponse(`[{"ad_id":"301","adgroup_id":"201","ad_name":"Video A","operation_status":"DISABLE"}]`, 1, false))
		case "/v1.3/ad/create/":
			body := decodeBody(t, request)
			creatives, ok := body["creatives"].([]any)
			if !ok || len(creatives) != 2 || body["adgroup_id"] != "201" {
				t.Fatalf("Ad create=%v", body)
			}
			for _, item := range creatives {
				creative := item.(map[string]any)
				if creative["operation_status"] != "DISABLE" || creative["tracking_pixel_id"] != "pixel-1" {
					t.Errorf("creative=%v", creative)
				}
			}
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"ad_ids":["301","302"]}}`)
		case "/v1.3/ad/status/update/":
			body := decodeBody(t, request)
			assertBodyIDs(t, body, "ad_ids", "301")
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"ad_ids":["301"],"status":"DELETE"}}`)
		case "/v1.3/report/integrated/get/":
			if request.URL.Query().Get("service_type") != "AUCTION" || request.URL.Query().Get("report_type") != "BASIC" {
				t.Errorf("report query=%s", request.URL.RawQuery)
			}
			assertJSONQuery(t, request, "dimensions", []any{"campaign_id", "stat_time_day"})
			assertJSONQuery(t, request, "metrics", []any{"spend", "impressions"})
			writeJSON(writer, http.StatusOK, pageResponse(`[{"dimensions":{"advertiser_id":"123456789","campaign_id":"101","stat_time_day":"2026-08-01"},"metrics":{"spend":"12.5","impressions":"100"}}]`, 1, false))
		default:
			http.Error(writer, "unexpected path", http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	ctx := context.Background()

	advertiser, err := client.GetAdvertiser(ctx)
	if err != nil || advertiser.ID != testAdvertiserID || len(advertiser.Raw) == 0 {
		t.Fatalf("advertiser=%#v err=%v", advertiser, err)
	}
	campaigns, err := client.ListCampaigns(ctx, ListCampaignsRequest{
		IDs: []string{"101"}, Name: "Launch", Fields: []string{"campaign_id", "campaign_name"}, PageSize: 1,
	})
	if err != nil || len(campaigns.Items) != 1 || !campaigns.HasMore || campaigns.Items[0].AdvertiserID != testAdvertiserID {
		t.Fatalf("campaigns=%#v err=%v", campaigns, err)
	}
	campaign, err := client.CreateCampaign(ctx, validCampaignRequest())
	if err != nil || campaign.ID != "101" || campaign.OperationStatus != StatusDisable {
		t.Fatalf("campaign=%#v err=%v", campaign, err)
	}
	name := "Launch 2"
	updatedCampaign, err := client.UpdateCampaign(ctx, "101", UpdateCampaignRequest{Name: &name, Fields: map[string]any{"po_number": "PO-1"}})
	if err != nil || updatedCampaign.Name != name {
		t.Fatalf("updated campaign=%#v err=%v", updatedCampaign, err)
	}
	if result, err := client.SetCampaignStatus(ctx, "101", StatusEnable); err != nil || !containsID(result.SucceededIDs, "101") {
		t.Fatalf("campaign status=%#v err=%v", result, err)
	}

	adGroups, err := client.ListAdGroups(ctx, ListAdGroupsRequest{IDs: []string{"201"}, CampaignIDs: []string{"101"}})
	if err != nil || len(adGroups.Items) != 1 || adGroups.Items[0].AdvertiserID != testAdvertiserID {
		t.Fatalf("Ad Groups=%#v err=%v", adGroups, err)
	}
	adGroup, err := client.CreateAdGroup(ctx, validAdGroupRequest())
	if err != nil || adGroup.ID != "201" || adGroup.OperationStatus != StatusDisable {
		t.Fatalf("Ad Group=%#v err=%v", adGroup, err)
	}
	adGroupName := "Prospecting 2"
	updatedAdGroup, err := client.UpdateAdGroup(ctx, "201", UpdateAdGroupRequest{Name: &adGroupName, Fields: map[string]any{"pacing": "PACING_MODE_SMOOTH"}})
	if err != nil || updatedAdGroup.Name != adGroupName {
		t.Fatalf("updated Ad Group=%#v err=%v", updatedAdGroup, err)
	}
	if _, err := client.SetAdGroupStatus(ctx, "201", StatusDisable); err != nil {
		t.Fatal(err)
	}

	ads, err := client.ListAds(ctx, ListAdsRequest{IDs: []string{"301"}, AdGroupIDs: []string{"201"}})
	if err != nil || len(ads.Items) != 1 || ads.Items[0].AdvertiserID != testAdvertiserID {
		t.Fatalf("Ads=%#v err=%v", ads, err)
	}
	createdAds, err := client.CreateAds(ctx, validAdsRequest())
	if err != nil || len(createdAds) != 2 || createdAds[0].OperationStatus != StatusDisable || createdAds[1].ID != "302" {
		t.Fatalf("created Ads=%#v err=%v", createdAds, err)
	}
	if _, err := client.SetAdStatus(ctx, "301", StatusDelete); err != nil {
		t.Fatal(err)
	}

	report, err := client.GetReport(ctx, ReportRequest{
		DataLevel: ReportLevelCampaign, StartDate: "2026-08-01", EndDate: "2026-08-30",
		Dimensions: []string{"campaign_id", "stat_time_day"}, Metrics: []string{"spend", "impressions"},
	})
	if err != nil || len(report.Items) != 1 || len(report.Items[0].Raw) == 0 {
		t.Fatalf("report=%#v err=%v", report, err)
	}
}

func validCampaignRequest() CreateCampaignRequest {
	return CreateCampaignRequest{
		Name: "Launch", ObjectiveType: "TRAFFIC", CampaignType: "REGULAR_CAMPAIGN",
		BudgetMode: "BUDGET_MODE_DAY", Budget: 100, Fields: map[string]any{"is_search_campaign": true},
	}
}

func validAdGroupRequest() CreateAdGroupRequest {
	return CreateAdGroupRequest{
		CampaignID: "101", Name: "Prospecting", PromotionType: "WEBSITE", PlacementType: "PLACEMENT_TYPE_NORMAL",
		Placements: []string{"PLACEMENT_TIKTOK"}, LocationIDs: []string{"6252001"}, BudgetMode: "BUDGET_MODE_DAY",
		Budget: 50, ScheduleType: "SCHEDULE_START_END", ScheduleStart: "2026-08-10 00:00:00",
		ScheduleEnd: "2026-08-31 23:59:59", OptimizationGoal: "CLICK", BillingEvent: "CPC",
		BidType: "BID_TYPE_CUSTOM", BidPrice: 1, Fields: map[string]any{"age_groups": []string{"AGE_25_34"}},
	}
}

func validAdsRequest() CreateAdsRequest {
	return CreateAdsRequest{AdGroupID: "201", Creatives: []AdCreative{
		{Name: "Video A", IdentityType: "CUSTOMIZED_USER", IdentityID: "identity-1", AdFormat: "SINGLE_VIDEO", VideoID: "video-1", AdText: "Launch", CallToAction: "LEARN_MORE", LandingPageURL: "https://example.com/a?source=tiktok", Fields: map[string]any{"tracking_pixel_id": "pixel-1"}},
		{Name: "Video B", IdentityType: "CUSTOMIZED_USER", IdentityID: "identity-1", AdFormat: "SINGLE_VIDEO", VideoID: "video-2", AdText: "Launch", CallToAction: "LEARN_MORE", LandingPageURL: "https://example.com/b", Fields: map[string]any{"tracking_pixel_id": "pixel-1"}},
	}}
}

func pageResponse(items string, total int, hasMore bool) string {
	encodedMore, _ := json.Marshal(hasMore)
	return `{"code":0,"data":{"list":` + items + `,"page_info":{"page":1,"page_size":1,"total_number":` +
		strconv.Itoa(total) + `,"total_page":2,"has_more":` + string(encodedMore) + `}}}`
}

func assertJSONQuery(t *testing.T, request *http.Request, key string, expected any) {
	t.Helper()
	var actual any
	if err := json.Unmarshal([]byte(request.URL.Query().Get(key)), &actual); err != nil {
		t.Fatalf("%s query=%q: %v", key, request.URL.Query().Get(key), err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("%s=%#v want %#v", key, actual, expected)
	}
}

func assertBodyIDs(t *testing.T, body map[string]any, key, expected string) {
	t.Helper()
	values, ok := body[key].([]any)
	if !ok || len(values) != 1 || values[0] != expected {
		t.Fatalf("%s=%v", key, body[key])
	}
}

package tencentads

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestMarketingWorkflows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assertAPIAuth(t, request)
		switch request.URL.Path {
		case "/advertiser/get":
			assertListQuery(t, request, "account_id")
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"list":[{"account_id":123456,"corporation_name":"Example"}],"page_info":{"page":1,"page_size":1,"total_number":1,"total_page":1}}}`)
		case "/campaigns/get":
			assertListQuery(t, request, "campaign_id")
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"list":[{"campaign_id":101,"campaign_name":"Launch","configured_status":"AD_STATUS_SUSPEND","campaign_type":"CAMPAIGN_TYPE_NORMAL","promoted_object_type":"PROMOTED_OBJECT_TYPE_LINK","extra":true}],"page_info":{"page":1,"page_size":20,"total_number":21,"total_page":2}}}`)
		case "/campaigns/add":
			body := decodeObject(t, request)
			assertBodyValue(t, body, "account_id", float64(testAdvertiserID))
			assertBodyValue(t, body, "configured_status", string(ConfiguredStatusSuspend))
			assertBodyValue(t, body, "speed_mode", "SPEED_MODE_FAST")
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"campaign_id":101}}`)
		case "/campaigns/update":
			body := decodeObject(t, request)
			assertBodyValue(t, body, "campaign_id", float64(101))
			assertBodyValue(t, body, "campaign_name", "Launch v2")
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"campaign_id":101}}`)
		case "/campaigns/update_configured_status":
			body := decodeObject(t, request)
			assertBodyValue(t, body, "account_id", float64(testAdvertiserID))
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"list":[{"code":0,"campaign_id":101}],"fail_id_list":[]}}`)
		case "/adgroups/get":
			assertListQuery(t, request, "adgroup_id")
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"list":[{"adgroup_id":202,"campaign_id":101,"adgroup_name":"Prospecting","configured_status":"AD_STATUS_SUSPEND","billing_event":"BILLINGEVENT_IMPRESSION","optimization_goal":"OPTIMIZATIONGOAL_IMPRESSION"}],"page_info":{"page":1,"page_size":20,"total_number":1,"total_page":1}}}`)
		case "/adgroups/add":
			body := decodeObject(t, request)
			assertBodyValue(t, body, "campaign_id", float64(101))
			assertBodyValue(t, body, "configured_status", string(ConfiguredStatusSuspend))
			assertBodyValue(t, body, "time_series", "1111")
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"adgroup_id":202}}`)
		case "/adgroups/update":
			body := decodeObject(t, request)
			assertBodyValue(t, body, "adgroup_id", float64(202))
			assertBodyValue(t, body, "daily_budget", float64(1000))
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"adgroup_id":202}}`)
		case "/adgroups/update_configured_status":
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"list":[{"code":0,"adgroup_id":202}],"fail_id_list":[]}}`)
		case "/adcreatives/get":
			assertListQuery(t, request, "adcreative_id")
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"list":[{"adcreative_id":303,"campaign_id":101,"adcreative_name":"Creative","adcreative_template_id":404}],"page_info":{"page":1,"page_size":20,"total_number":1,"total_page":1}}}`)
		case "/adcreatives/add":
			body := decodeObject(t, request)
			assertBodyValue(t, body, "adcreative_template_id", float64(404))
			if _, found := body["adcreative_elements"]; !found {
				t.Error("creative extension fields were not sent")
			}
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"adcreative_id":303}}`)
		case "/adcreatives/update":
			body := decodeObject(t, request)
			assertBodyValue(t, body, "adcreative_id", float64(303))
			assertBodyValue(t, body, "deep_link_url", "app://launch")
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"adcreative_id":303}}`)
		case "/daily_reports/get":
			query := request.URL.Query()
			if query.Get("level") != "REPORT_LEVEL_ADGROUP" || query.Get("date_range") != `{"start_date":"2026-08-01","end_date":"2026-08-02"}` {
				t.Errorf("unexpected report query: %v", query)
			}
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"list":[{"account_id":123456,"campaign_id":101,"adgroup_id":202,"date":"2026-08-01","view_count":10,"cost":250}],"page_info":{"page":1,"page_size":20,"total_number":1,"total_page":1}}}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	ctx := context.Background()

	advertiser, err := client.GetAdvertiser(ctx)
	if err != nil || advertiser.AccountID != testAdvertiserID || len(advertiser.Raw) == 0 {
		t.Fatalf("advertiser=%#v err=%v", advertiser, err)
	}
	campaigns, err := client.ListCampaigns(ctx, ListCampaignsRequest{Filtering: []Filtering{{Field: "campaign_id", Operator: "EQUALS", Values: []string{"101"}}}})
	if err != nil || len(campaigns.Items) != 1 || !campaigns.HasMore || campaigns.Items[0].AccountID != testAdvertiserID || len(campaigns.Items[0].Raw) == 0 {
		t.Fatalf("campaigns=%#v err=%v", campaigns, err)
	}
	campaign, err := client.CreateCampaign(ctx, CreateCampaignRequest{
		Name: "Launch", CampaignType: "CAMPAIGN_TYPE_NORMAL", PromotedObjectType: "PROMOTED_OBJECT_TYPE_LINK",
		DailyBudget: 1000, Fields: map[string]any{"speed_mode": "SPEED_MODE_FAST"},
	})
	if err != nil || campaign.ID != 101 || campaign.ConfiguredStatus != ConfiguredStatusSuspend {
		t.Fatalf("campaign=%#v err=%v", campaign, err)
	}
	name := "Launch v2"
	if err := client.UpdateCampaign(ctx, 101, UpdateCampaignRequest{Name: &name}); err != nil {
		t.Fatal(err)
	}
	if err := client.SetCampaignStatus(ctx, 101, ConfiguredStatusNormal); err != nil {
		t.Fatal(err)
	}

	groups, err := client.ListAdGroups(ctx, ListAdGroupsRequest{})
	if err != nil || len(groups.Items) != 1 || groups.HasMore || groups.Items[0].AccountID != testAdvertiserID {
		t.Fatalf("groups=%#v err=%v", groups, err)
	}
	group, err := client.CreateAdGroup(ctx, CreateAdGroupRequest{
		CampaignID: 101, Name: "Prospecting", PromotedObjectType: "PROMOTED_OBJECT_TYPE_LINK",
		BillingEvent: "BILLINGEVENT_IMPRESSION", OptimizationGoal: "OPTIMIZATIONGOAL_IMPRESSION",
		BeginDate: "2026-08-10", EndDate: "2026-08-31", Fields: map[string]any{"time_series": "1111"},
	})
	if err != nil || group.ID != 202 || group.ConfiguredStatus != ConfiguredStatusSuspend {
		t.Fatalf("group=%#v err=%v", group, err)
	}
	dailyBudget := int64(1000)
	if err := client.UpdateAdGroup(ctx, 202, UpdateAdGroupRequest{DailyBudget: &dailyBudget}); err != nil {
		t.Fatal(err)
	}
	if err := client.SetAdGroupStatus(ctx, 202, ConfiguredStatusSuspend); err != nil {
		t.Fatal(err)
	}

	creatives, err := client.ListAdCreatives(ctx, ListAdCreativesRequest{})
	if err != nil || len(creatives.Items) != 1 || creatives.Items[0].AccountID != testAdvertiserID {
		t.Fatalf("creatives=%#v err=%v", creatives, err)
	}
	creative, err := client.CreateAdCreative(ctx, CreateAdCreativeRequest{
		CampaignID: 101, Name: "Creative", PromotedObjectType: "PROMOTED_OBJECT_TYPE_LINK", TemplateID: 404,
		Fields: map[string]any{"adcreative_elements": map[string]any{"title": "Launch"}},
	})
	if err != nil || creative.ID != 303 {
		t.Fatalf("creative=%#v err=%v", creative, err)
	}
	if err := client.UpdateAdCreative(ctx, 303, UpdateAdCreativeRequest{Fields: map[string]any{"deep_link_url": "app://launch"}}); err != nil {
		t.Fatal(err)
	}

	report, err := client.GetReport(ctx, ReportRequest{
		Granularity: ReportDaily, Level: "REPORT_LEVEL_ADGROUP",
		DateRange: ReportDateRange{StartDate: "2026-08-01", EndDate: "2026-08-02"},
	})
	if err != nil || len(report.Items) != 1 || report.Items[0].AdGroupID != 202 || len(report.Items[0].Raw) == 0 {
		t.Fatalf("report=%#v err=%v", report, err)
	}
}

func decodeObject(t *testing.T, request *http.Request) map[string]any {
	t.Helper()
	defer request.Body.Close()
	var body map[string]any
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	return body
}

func assertBodyValue(t *testing.T, body map[string]any, key string, expected any) {
	t.Helper()
	if actual := body[key]; actual != expected {
		t.Errorf("body[%q]=%#v, want %#v", key, actual, expected)
	}
}

func assertListQuery(t *testing.T, request *http.Request, requiredField string) {
	t.Helper()
	query := request.URL.Query()
	if query.Get("account_id") != strconv.FormatInt(testAdvertiserID, 10) || query.Get("page") != "1" {
		t.Errorf("unexpected list query: %v", query)
	}
	var fields []string
	if err := json.Unmarshal([]byte(query.Get("fields")), &fields); err != nil {
		t.Fatal(err)
	}
	for _, field := range fields {
		if field == requiredField {
			return
		}
	}
	t.Errorf("required field %q missing from %v", requiredField, fields)
}

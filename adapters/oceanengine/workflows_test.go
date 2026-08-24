package oceanengine

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestMarketingWorkflows(t *testing.T) {
	projectID, promotionID := int64(111), int64(222)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Access-Token") != "access-token" || request.Header.Get("Authorization") != "" {
			t.Errorf("unexpected auth headers: %#v", request.Header)
		}
		switch request.URL.Path {
		case "/open_api/v3.0/project/create/":
			body := decodeObject(t, request)
			assertBodyValue(t, body, "advertiser_id", float64(testAdvertiserID))
			assertBodyValue(t, body, "operation", "DISABLE")
			if _, found := body["audience"]; !found {
				t.Error("extension field was not sent")
			}
			writeJSON(writer, http.StatusOK, `{"code":0,"message":"OK","request_id":"req-project-create","data":{"project_id":111}}`)
		case "/open_api/v3.0/project/list/":
			assertListQuery(t, request, "project_id")
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"list":[{"project_id":111,"advertiser_id":123456,"name":"Launch","ad_type":"ALL","extra_provider_field":true}],"page_info":{"page":1,"page_size":20,"total_number":21,"total_page":2}}}`)
		case "/open_api/v3.0/project/update/":
			body := decodeObject(t, request)
			assertBodyValue(t, body, "project_id", float64(projectID))
			assertBodyValue(t, body, "name", "Launch v2")
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"project_id":111,"error_list":[]}}`)
		case "/open_api/v3.0/project/status/update/":
			body := decodeObject(t, request)
			assertBodyValue(t, body, "advertiser_id", float64(testAdvertiserID))
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"project_ids":[111],"errors":[]}}`)
		case "/open_api/v3.0/promotion/create/":
			body := decodeObject(t, request)
			assertBodyValue(t, body, "project_id", float64(projectID))
			assertBodyValue(t, body, "operation", "DISABLE")
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"promotion_id":222}}`)
		case "/open_api/v3.0/promotion/list/":
			assertListQuery(t, request, "promotion_id")
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"list":[{"promotion_id":222,"advertiser_id":123456,"project_id":111,"promotion_name":"Creative"}],"page_info":{"page":1,"page_size":20,"total_number":1,"total_page":1}}}`)
		case "/open_api/v3.0/promotion/update/":
			body := decodeObject(t, request)
			assertBodyValue(t, body, "promotion_id", float64(promotionID))
			assertBodyValue(t, body, "budget", float64(500))
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"promotion_id":222,"error_list":[]}}`)
		case "/open_api/v3.0/promotion/status/update/":
			body := decodeObject(t, request)
			assertBodyValue(t, body, "advertiser_id", float64(testAdvertiserID))
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"promotion_ids":[222],"errors":[]}}`)
		case "/open_api/v3.0/report/custom/get/":
			query := request.URL.Query()
			if query.Get("advertiser_id") != strconv.FormatInt(testAdvertiserID, 10) || query.Get("dimensions") != `["stat_time_day"]` || query.Get("metrics") != `["show","cost"]` || query.Get("filters") != `[]` || query.Get("order_by") != `[]` {
				t.Errorf("unexpected report query: %v", query)
			}
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"rows":[{"dimensions":{"stat_time_day":"2026-08-01"},"metrics":{"show":"10","cost":"2.50"}}],"total_metrics":{"show":"10","cost":"2.50"},"page_info":{"page":1,"page_size":20,"total_number":1,"total_page":1}}}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	ctx := context.Background()

	project, err := client.CreateProject(ctx, validProjectRequest())
	if err != nil || project.ID != projectID || project.OptStatus != "DISABLE" {
		t.Fatalf("project=%#v err=%v", project, err)
	}
	projects, err := client.ListProjects(ctx, ListProjectsRequest{Filter: ProjectFilter{IDs: []int64{projectID}}})
	if err != nil || len(projects.Items) != 1 || !projects.HasMore || len(projects.Items[0].Raw) == 0 {
		t.Fatalf("projects=%#v err=%v", projects, err)
	}
	name := "Launch v2"
	if err := client.UpdateProject(ctx, projectID, UpdateProjectRequest{Name: &name}); err != nil {
		t.Fatal(err)
	}
	if err := client.SetProjectStatus(ctx, projectID, OperationEnable); err != nil {
		t.Fatal(err)
	}

	promotion, err := client.CreatePromotion(ctx, CreatePromotionRequest{ProjectID: projectID, Name: "Creative", Fields: map[string]any{"budget": 100}})
	if err != nil || promotion.ID != promotionID || promotion.OptStatus != "DISABLE" {
		t.Fatalf("promotion=%#v err=%v", promotion, err)
	}
	promotions, err := client.ListPromotions(ctx, ListPromotionsRequest{Filter: PromotionFilter{ProjectID: projectID}})
	if err != nil || len(promotions.Items) != 1 || promotions.HasMore || len(promotions.Items[0].Raw) == 0 {
		t.Fatalf("promotions=%#v err=%v", promotions, err)
	}
	if err := client.UpdatePromotion(ctx, promotionID, UpdatePromotionRequest{Name: "Creative v2", Fields: map[string]any{"budget": 500}}); err != nil {
		t.Fatal(err)
	}
	if err := client.SetPromotionStatus(ctx, promotionID, OperationDisable); err != nil {
		t.Fatal(err)
	}

	report, err := client.GetCustomReport(ctx, CustomReportRequest{
		Dimensions: []string{"stat_time_day"}, Metrics: []string{"show", "cost"},
		Filters: []ReportFilter{}, OrderBy: []ReportOrder{},
		StartTime: "2026-08-01", EndTime: "2026-08-02", DataTopic: ReportTopicBasicData,
	})
	if err != nil || len(report.Items) != 1 || report.TotalMetrics["show"] != "10" || report.HasMore {
		t.Fatalf("report=%#v err=%v", report, err)
	}
}

func validProjectRequest() CreateProjectRequest {
	return CreateProjectRequest{
		Name: "Launch", AdType: AdTypeAll, LandingType: LandingTypeLink,
		MarketingGoal:   MarketingGoalVideoAndImage,
		DeliveryRange:   DeliveryRange{InventoryCatalog: InventoryCatalogManual, InventoryType: []string{"INVENTORY_FEED"}},
		DeliverySetting: DeliverySetting{BidType: BidTypeCustom, BudgetMode: BudgetModeDay},
		Fields:          map[string]any{"audience": map[string]any{"district": "ALL"}},
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
	actual := body[key]
	if actual != expected {
		t.Errorf("body[%q]=%#v, want %#v", key, actual, expected)
	}
}

func assertListQuery(t *testing.T, request *http.Request, requiredField string) {
	t.Helper()
	query := request.URL.Query()
	if query.Get("advertiser_id") != strconv.FormatInt(testAdvertiserID, 10) || query.Get("page") != "1" || query.Get("page_size") != "20" {
		t.Errorf("unexpected list query: %v", query)
	}
	var fields []string
	if err := json.Unmarshal([]byte(query.Get("fields")), &fields); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, field := range fields {
		if field == requiredField {
			found = true
		}
	}
	if !found {
		t.Errorf("required field %q missing from %v", requiredField, fields)
	}
}

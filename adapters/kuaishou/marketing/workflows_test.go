package marketing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTypedMarketingWorkflowsAndPausedCreation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body := assertAPIRequest(t, request)
		switch request.URL.Path {
		case "/v1/advertiser/info":
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"advertiser_id":123456,"user_id":9,"user_name":"Brand"},"request_id":"req-1"}`)
		case "/gw/dsp/campaign/list":
			if body["page"] != float64(1) || body["page_size"] != float64(1) {
				t.Errorf("campaign page=%v", body)
			}
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"total_count":2,"details":[{"campaign_id":101,"campaign_name":"Launch","put_status":2}]}}`)
		case "/gw/dsp/campaign/create":
			if body["put_status"] != float64(PutStatusPaused) || body["type"] != float64(5) || body["campaign_name"] != "Launch" {
				t.Errorf("campaign create=%v", body)
			}
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"campaign_id":101}}`)
		case "/gw/dsp/campaign/update":
			if schedule, ok := body["day_budget_schedule"].([]any); !ok || len(schedule) != 0 {
				t.Errorf("campaign update must explicitly clear day_budget_schedule: %v", body)
			}
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"campaign_id":101}}`)
		case "/v1/campaign/update/status":
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"campaign_ids":[101]}}`)
		case "/gw/dsp/unit/list":
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"total_count":1,"details":[{"unit_id":202,"campaign_id":101,"unit_name":"Prospecting","put_status":2}]}}`)
		case "/gw/dsp/unit/create":
			if body["put_status"] != float64(PutStatusPaused) || body["campaign_id"] != float64(101) || body["target"] == nil {
				t.Errorf("unit create=%v", body)
			}
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"unit_id":202}}`)
		case "/v1/ad_unit/update/status":
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"unit_ids":[202],"errors":[]}}`)
		case "/gw/dsp/creative/list":
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"total_count":1,"details":[{"creative_id":303,"campaign_id":101,"unit_id":202,"creative_name":"Video"}]}}`)
		case "/gw/dsp/creative/create":
			if body["unit_id"] != float64(202) || body["photo_id"] != "photo-1" {
				t.Errorf("creative create=%v", body)
			}
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"creative_id":303}}`)
		case "/v1/creative/update/status":
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"creative_ids":[303]}}`)
		case "/v1/report/unit_report":
			if body["temporal_granularity"] != "DAILY" || body["campaign_ids"] == nil {
				t.Errorf("report body=%v", body)
			}
			writeJSON(writer, http.StatusOK, `{"code":0,"data":{"total_count":1,"details":[{"advertiser_id":123456,"campaign_id":101,"unit_id":202,"stat_date":"2026-08-01","charge":12.5}]}}`)
		default:
			http.Error(writer, "unexpected path", http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	ctx := context.Background()
	advertiser, err := client.GetAdvertiser(ctx)
	if err != nil || advertiser.AdvertiserID != testAdvertiserID || len(advertiser.Raw) == 0 {
		t.Fatalf("advertiser=%#v err=%v", advertiser, err)
	}
	status := 4
	campaigns, err := client.ListCampaigns(ctx, ListCampaignsRequest{
		IDs: []int64{101}, Name: "Launch", PageSize: 1, PutStatuses: []PutStatus{PutStatusPaused}, Status: &status,
		StartDate: "2026-08-01", EndDate: "2026-08-02", TimeFilterType: 1,
	})
	if err != nil || len(campaigns.Items) != 1 || !campaigns.HasMore || campaigns.Items[0].AdvertiserID != testAdvertiserID {
		t.Fatalf("campaigns=%#v err=%v", campaigns, err)
	}
	campaign, err := client.CreateCampaign(ctx, validCampaignRequest())
	if err != nil || campaign.ID != 101 || campaign.PutStatus != PutStatusPaused {
		t.Fatalf("campaign=%#v err=%v", campaign, err)
	}
	name := "Launch 2"
	budget := int64(600000)
	emptySchedule := []int64{}
	if err := client.UpdateCampaign(ctx, 101, UpdateCampaignRequest{
		Name: &name, DayBudget: &budget, DayBudgetSchedule: &emptySchedule, Fields: map[string]any{"auto_adjust": 0},
	}); err != nil {
		t.Fatal(err)
	}
	if result, err := client.SetCampaignStatus(ctx, 101, PutStatusDelivering); err != nil || !containsID(result.SucceededIDs, 101) {
		t.Fatalf("campaign status=%#v err=%v", result, err)
	}
	units, err := client.ListUnits(ctx, ListUnitsRequest{
		IDs: []int64{202}, CampaignID: 101, Name: "Prospecting", PutStatuses: []PutStatus{PutStatusPaused},
		StartDate: "2026-08-01", EndDate: "2026-08-02", TimeFilterType: 1,
	})
	if err != nil || len(units.Items) != 1 || units.Items[0].AdvertiserID != testAdvertiserID {
		t.Fatalf("units=%#v err=%v", units, err)
	}
	unit, err := client.CreateUnit(ctx, validUnitRequest())
	if err != nil || unit.ID != 202 || unit.PutStatus != PutStatusPaused || len(unit.Target) == 0 {
		t.Fatalf("unit=%#v err=%v", unit, err)
	}
	if _, err := client.SetUnitStatus(ctx, 202, PutStatusPaused); err != nil {
		t.Fatal(err)
	}
	creatives, err := client.ListCreatives(ctx, ListCreativesRequest{
		IDs: []int64{303}, CampaignID: 101, UnitID: 202, Name: "Video", PutStatuses: []PutStatus{PutStatusPaused},
		StartDate: "2026-08-01", EndDate: "2026-08-02", TimeFilterType: 1,
	})
	if err != nil || len(creatives.Items) != 1 || creatives.Items[0].AdvertiserID != testAdvertiserID {
		t.Fatalf("creatives=%#v err=%v", creatives, err)
	}
	creative, err := client.CreateCreative(ctx, validCreativeRequest())
	if err != nil || creative.ID != 303 || len(creative.Raw) != 0 {
		t.Fatalf("creative=%#v err=%v", creative, err)
	}
	if _, err := client.SetCreativeStatus(ctx, 303, PutStatusPaused); err != nil {
		t.Fatal(err)
	}
	report, err := client.GetReport(ctx, ReportRequest{
		Level: ReportLevelUnit, StartDate: "2026-08-01", EndDate: "2026-08-07", TemporalGranularity: GranularityDaily,
		ReportDimensions: []string{"adScene"}, CampaignType: 5, CampaignIDs: []int64{101}, UnitIDs: []int64{202},
	})
	if err != nil || len(report.Items) != 1 || report.Items[0].UnitID != 202 || report.Items[0].Date != "2026-08-01" || len(report.Items[0].Raw) == 0 {
		t.Fatalf("report=%#v err=%v", report, err)
	}
}

func validCampaignRequest() CreateCampaignRequest {
	return CreateCampaignRequest{Name: "Launch", MarketingGoal: 5, DayBudget: 500000, Fields: map[string]any{"auto_adjust": 0}}
}

func validUnitRequest() CreateUnitRequest {
	return CreateUnitRequest{
		CampaignID: 101, Name: "Prospecting", BeginTime: "2026-08-10", EndTime: "2026-08-31", BidType: BidTypeOCPM,
		CPABid: 10000, SceneIDs: []string{"1"}, UnitType: 4, Target: map[string]any{"platform_os": 0},
		Fields: map[string]any{"ocpx_action_type": 53},
	}
}

func validCreativeRequest() CreateCreativeRequest {
	return CreateCreativeRequest{
		UnitID: 202, Name: "Video", MaterialType: 1, ActionBarText: "Learn more", Description: "Launch now",
		PhotoID: "photo-1", ImageToken: "cover-1", Fields: map[string]any{"creative_category": 1},
	}
}

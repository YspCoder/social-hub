package baiduads

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

const (
	testCampaignID = int64(101)
	testAdGroupID  = int64(201)
	testCreativeID = int64(301)
)

func TestSearchAdvertisingWorkflows(t *testing.T) {
	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body := decodeRequest(t, request)
		switch request.URL.Path {
		case "/json/sms/service/AccountService/getAccountInfo":
			fields := decodeRaw[[]string](t, body["accountFields"])
			if !contains(fields, "userId") {
				t.Fatal("account request omitted userId")
			}
			writeSuccess(t, writer, []any{map[string]any{"userId": 9001, "balance": 125.5, "customField": "kept"}})
		case "/json/sms/service/CampaignService/getCampaign":
			fields := decodeRaw[[]string](t, body["campaignFields"])
			if !contains(fields, "campaignId") {
				t.Fatal("campaign request omitted campaignId")
			}
			ids := decodeRaw[[]int64](t, body["campaignIds"])
			values := []any{campaignWire(testCampaignID, "Existing", true)}
			if len(ids) == 0 {
				values = append(values, campaignWire(102, "Second", false))
			}
			writeSuccess(t, writer, values)
		case "/json/sms/service/CampaignService/addCampaign":
			resources := decodeRaw[[]map[string]any](t, body["campaignTypes"])
			if len(resources) != 1 || resources[0]["pause"] != true || resources[0]["marketingTargetId"] != float64(0) {
				t.Fatalf("campaign create=%v", resources)
			}
			writeSuccess(t, writer, []any{campaignWire(103, resources[0]["campaignName"].(string), true)})
		case "/json/sms/service/CampaignService/updateCampaign":
			resources := decodeRaw[[]map[string]any](t, body["campaignTypes"])
			pause, _ := resources[0]["pause"].(bool)
			writeSuccess(t, writer, []any{campaignWire(testCampaignID, "Renamed", pause)})
		case "/json/sms/service/CampaignService/deleteCampaign":
			ids := decodeRaw[[]int64](t, body["campaignIds"])
			writeSuccess(t, writer, []any{map[string]any{"campaignId": ids[0]}})
		case "/json/sms/service/AdgroupService/getAdgroup":
			fields := decodeRaw[[]string](t, body["adgroupFields"])
			if !contains(fields, "campaignId") || !contains(fields, "adgroupId") {
				t.Fatalf("ad group fields=%v", fields)
			}
			ids := decodeRaw[[]int64](t, body["ids"])
			idType := decodeRaw[AdGroupIDType](t, body["idType"])
			values := []any{adGroupWire(testAdGroupID, testCampaignID, "Existing group", true)}
			if idType == AdGroupByCampaignID && len(ids) == 1 {
				values = append(values, adGroupWire(202, testCampaignID, "Second group", true))
			}
			writeSuccess(t, writer, values)
		case "/json/sms/service/AdgroupService/addAdgroup":
			resources := decodeRaw[[]map[string]any](t, body["adgroupTypes"])
			if len(resources) != 1 || resources[0]["pause"] != true {
				t.Fatalf("ad group create=%v", resources)
			}
			writeSuccess(t, writer, []any{adGroupWire(203, testCampaignID, resources[0]["adgroupName"].(string), true)})
		case "/json/sms/service/AdgroupService/updateAdgroup":
			resources := decodeRaw[[]map[string]any](t, body["adgroupTypes"])
			pause, _ := resources[0]["pause"].(bool)
			writeSuccess(t, writer, []any{adGroupWire(testAdGroupID, testCampaignID, "Updated group", pause)})
		case "/json/sms/service/AdgroupService/deleteAdgroup":
			ids := decodeRaw[[]int64](t, body["adgroupIds"])
			writeSuccess(t, writer, []any{map[string]any{"adgroupId": ids[0]}})
		case "/json/sms/service/CreativeService/getCreative":
			fields := decodeRaw[[]string](t, body["creativeFields"])
			if !contains(fields, "creativeId") || !contains(fields, "adgroupId") {
				t.Fatalf("creative fields=%v", fields)
			}
			writeSuccess(t, writer, []any{creativeWire(testCreativeID, testCampaignID, testAdGroupID, true)})
		case "/json/sms/service/CreativeService/addCreative":
			resources := decodeRaw[[]map[string]any](t, body["creativeTypes"])
			if len(resources) != 1 {
				t.Fatalf("creative create=%v", resources)
			}
			if _, found := resources[0]["pause"]; found {
				t.Fatal("addCreative must not receive pause")
			}
			writeSuccess(t, writer, []any{creativeWire(302, testCampaignID, testAdGroupID, false)})
		case "/json/sms/service/CreativeService/updateCreative":
			resources := decodeRaw[[]map[string]any](t, body["creativeTypes"])
			pause, ok := resources[0]["pause"].(bool)
			if !ok {
				t.Fatalf("creative update omitted pause: %v", resources)
			}
			id := int64(resources[0]["creativeId"].(float64))
			writeSuccess(t, writer, []any{creativeWire(id, testCampaignID, testAdGroupID, pause)})
		case "/json/sms/service/CreativeService/deleteCreative":
			ids := decodeRaw[[]int64](t, body["creativeIds"])
			writeSuccess(t, writer, []any{map[string]any{"creativeId": ids[0]}})
		case "/json/sms/service/OpenApiReportService/getReportData":
			if _, found := body["startRow"]; !found {
				t.Fatal("synchronous report omitted pagination")
			}
			writeSuccess(t, writer, map[string]any{
				"rows":    []any{map[string]any{"campaignId": "101", "click": "12"}},
				"summary": map[string]any{"click": "12"}, "rowCount": 1, "totalRowCount": 1,
			})
		case "/json/sms/service/OpenApiReportService/createReportTask":
			if _, found := body["startRow"]; found {
				t.Fatal("asynchronous report must omit pagination")
			}
			writeSuccess(t, writer, []any{map[string]any{"taskId": "task-1", "taskStatus": "SUBMITTED"}})
		case "/json/sms/service/OpenApiReportService/getTaskStatus":
			writeSuccess(t, writer, []any{map[string]any{
				"taskId": "task-1", "taskStatus": "SUCCESS", "fileUrl": "https://report.example/file.tsv?signature=ok",
				"dataStartRow": 1, "tableHeader": []string{"campaignId", "click"},
			}})
		default:
			http.NotFound(writer, request)
		}
	})
	server := httptest.NewServer(handler)
	defer server.Close()
	adapter, client := newTestClient(t, server)
	defer adapter.Close()
	ctx := context.Background()

	account, err := client.GetAccount(ctx, nil)
	if err != nil || account.UserID != 9001 || !json.Valid(account.Raw) {
		t.Fatalf("account=%+v err=%v", account, err)
	}
	campaigns, err := client.GetCampaigns(ctx, GetCampaignsRequest{})
	if err != nil || len(campaigns) != 2 || !json.Valid(campaigns[0].Raw) {
		t.Fatalf("campaigns=%+v err=%v", campaigns, err)
	}
	if campaign, err := client.GetCampaign(ctx, testCampaignID, nil); err != nil || campaign.ID != testCampaignID {
		t.Fatalf("campaign=%+v err=%v", campaign, err)
	}
	createdCampaign, err := client.CreateCampaign(ctx, CreateCampaignRequest{
		Name: "Search Plan", Budget: 100, MarketingTargetID: 0, Fields: map[string]any{"equipmentType": 3},
	})
	if err != nil || createdCampaign.ID != 103 || !createdCampaign.Pause {
		t.Fatalf("created campaign=%+v err=%v", createdCampaign, err)
	}
	updatedCampaign, err := client.UpdateCampaign(ctx, testCampaignID, UpdateCampaignRequest{
		Name: stringPointer("Renamed"), Budget: floatPointer(150), Pause: boolPointer(false),
	})
	if err != nil || updatedCampaign.Pause {
		t.Fatalf("updated campaign=%+v err=%v", updatedCampaign, err)
	}
	if err := client.DeleteCampaign(ctx, testCampaignID); err != nil {
		t.Fatal(err)
	}

	groups, err := client.GetAdGroups(ctx, GetAdGroupsRequest{IDs: []int64{testCampaignID}, IDType: AdGroupByCampaignID})
	if err != nil || len(groups) != 2 || !json.Valid(groups[0].Raw) {
		t.Fatalf("ad groups=%+v err=%v", groups, err)
	}
	if group, err := client.GetAdGroup(ctx, testAdGroupID, nil); err != nil || group.ID != testAdGroupID {
		t.Fatalf("ad group=%+v err=%v", group, err)
	}
	createdGroup, err := client.CreateAdGroup(ctx, CreateAdGroupRequest{
		CampaignID: testCampaignID, Name: "Search Group", MaxPrice: 6.8,
	})
	if err != nil || createdGroup.ID != 203 || !createdGroup.Pause {
		t.Fatalf("created ad group=%+v err=%v", createdGroup, err)
	}
	updatedGroup, err := client.UpdateAdGroup(ctx, testAdGroupID, UpdateAdGroupRequest{
		Name: stringPointer("Updated group"), MaxPrice: floatPointer(7.2), Pause: boolPointer(false),
	})
	if err != nil || updatedGroup.Pause {
		t.Fatalf("updated ad group=%+v err=%v", updatedGroup, err)
	}
	if err := client.DeleteAdGroup(ctx, testAdGroupID); err != nil {
		t.Fatal(err)
	}

	if creative, err := client.GetCreative(ctx, testCreativeID, nil); err != nil || creative.ID != testCreativeID || !json.Valid(creative.Raw) {
		t.Fatalf("creative=%+v err=%v", creative, err)
	}
	creativeInput := validCreativeInput()
	createdCreative, err := client.CreateCreative(ctx, creativeInput)
	if err != nil || createdCreative.ID != 302 || !createdCreative.Pause {
		t.Fatalf("created creative=%+v err=%v", createdCreative, err)
	}
	update := updateCreativeInput(false)
	updatedCreative, err := client.UpdateCreative(ctx, testCreativeID, update)
	if err != nil || updatedCreative.Pause {
		t.Fatalf("updated creative=%+v err=%v", updatedCreative, err)
	}
	if err := client.DeleteCreative(ctx, testCreativeID); err != nil {
		t.Fatal(err)
	}

	reportInput := validReportInput()
	report, err := client.GetReportData(ctx, reportInput)
	if err != nil || report.RowCount != 1 || string(report.Rows[0]["click"]) != `"12"` {
		t.Fatalf("report=%+v err=%v", report, err)
	}
	task, err := client.CreateReportTask(ctx, reportInput)
	if err != nil || task.TaskID != "task-1" {
		t.Fatalf("task=%+v err=%v", task, err)
	}
	task, err = client.GetReportTask(ctx, task.TaskID)
	if err != nil || task.Status != "SUCCESS" || task.DataStartRow != 1 {
		t.Fatalf("task=%+v err=%v", task, err)
	}

	quota := DefaultQuotaPolicy()
	if quota.DefaultQPS != 10 || quota.CampaignGetQPS != 100 || quota.CreativeAddQPS != 50 {
		t.Fatalf("quota=%+v", quota)
	}
}

func campaignWire(id int64, name string, pause bool) map[string]any {
	return map[string]any{
		"campaignId": id, "campaignName": name, "budget": 100, "pause": pause,
		"status": 21, "adType": 0, "marketingTargetId": 0, "customField": "kept",
	}
}

func adGroupWire(id, campaignID int64, name string, pause bool) map[string]any {
	return map[string]any{
		"adgroupId": id, "campaignId": campaignID, "adgroupName": name, "maxPrice": 6.8,
		"pause": pause, "status": 31, "adType": 0, "customField": "kept",
	}
}

func creativeWire(id, campaignID, adGroupID int64, pause bool) map[string]any {
	input := validCreativeInput()
	return map[string]any{
		"creativeId": id, "campaignId": campaignID, "adgroupId": adGroupID, "title": input.Title,
		"description1": input.Description1, "description2": input.Description2, "pause": pause, "status": 41,
		"mobileDestinationUrl": input.MobileDestinationURL, "mobileDisplayUrl": input.MobileDisplayURL,
		"pcDestinationUrl": input.PCDestinationURL, "pcDisplayUrl": input.PCDisplayURL, "tabs": input.Tabs,
		"customField": "kept",
	}
}

func validCreativeInput() CreateCreativeRequest {
	return CreateCreativeRequest{
		CampaignID: testCampaignID, AdGroupID: testAdGroupID,
		Title: "Search SDK", Description1: "Reliable search ads", Description2: "Paused for review",
		MobileDestinationURL: "https://example.com/mobile", MobileDisplayURL: "example.com",
		PCDestinationURL: "https://example.com/desktop", PCDisplayURL: "example.com", Tabs: []int{1, 2},
		Fields: map[string]any{"pcFinalUrl": "https://example.com/final"},
	}
}

func updateCreativeInput(pause bool) UpdateCreativeRequest {
	input := validCreativeInput()
	return UpdateCreativeRequest{
		Title: input.Title, Description1: input.Description1, Description2: input.Description2, Pause: pause,
		MobileDestinationURL: input.MobileDestinationURL, MobileDisplayURL: input.MobileDisplayURL,
		PCDestinationURL: input.PCDestinationURL, PCDisplayURL: input.PCDisplayURL, Tabs: input.Tabs,
	}
}

func validReportInput() ReportRequest {
	return ReportRequest{
		ReportType: 248654, StartDate: "2026-08-01", EndDate: "2026-08-02", TimeUnit: ReportTimeDay,
		Columns: []string{"date", "campaignId", "click"}, Sorts: []ReportSort{{Column: "date", Rule: "ASC"}},
		Filters:  []ReportFilter{{Column: "campaignId", Operator: "IN", Values: []string{"101"}}},
		StartRow: 0, RowCount: 100, NeedSum: true,
	}
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

package microsoftads

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"
)

func TestTypedWorkflowWireContractsAndPausedCreates(t *testing.T) {
	campaign := Campaign{ID: testCampaignID, Name: "Search", Status: StatusPaused, BudgetType: "DailyBudgetStandard", DailyBudget: 10, TimeZone: "PacificTimeUSCanadaTijuana", CampaignType: "Search", Languages: []string{"English"}}
	adGroup := AdGroup{ID: testAdGroupID, Name: "Core", Status: StatusPaused, CPCBid: &Bid{Amount: 1}, Language: "English", Network: NetworkOwnedAndOperatedAndSyndicatedSearch}
	ad := ResponsiveSearchAd{ID: testAdID, Type: "ResponsiveSearch", Status: StatusPaused, FinalURLs: []string{"https://example.com"}, Headlines: testAssetLinks(3), Descriptions: testAssetLinks(2)}
	keyword := Keyword{ID: testKeywordID, Text: "running shoes", Status: StatusPaused, MatchType: MatchTypePhrase, Bid: &Bid{Amount: 0.5}}
	var writes atomic.Int32
	var reportURL string
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/report.zip" {
			if request.Header.Get("Authorization") != "" || request.Header.Get("DeveloperToken") != "" || request.URL.Query().Get("sig") != "secret" {
				t.Fatalf("download leaked API authentication: %v", request.Header)
			}
			_, _ = writer.Write([]byte("zip-report"))
			return
		}
		assertAPIRequest(t, request)
		body := decodeBody(t, request)
		switch request.Method + " " + request.URL.Path {
		case http.MethodPost + " /Account/Query":
			if !reflect.DeepEqual(body, map[string]any{"AccountId": testAccountID}) {
				t.Fatalf("account body=%v", body)
			}
			writeValue(t, writer, http.StatusOK, map[string]any{"Account": Account{ID: testAccountID, Name: "Paid Search", ParentCustomerID: testCustomerID}})
		case http.MethodPost + " /Campaigns/QueryByAccountId":
			writeValue(t, writer, http.StatusOK, map[string]any{"Campaigns": []Campaign{campaign}})
		case http.MethodPost + " /Campaigns/QueryByIds":
			assertIDs(t, body, "AccountId", testAccountID, "CampaignIds", testCampaignID)
			writeValue(t, writer, http.StatusOK, map[string]any{"Campaigns": []Campaign{campaign}, "PartialErrors": []any{}})
		case http.MethodPost + " /Campaigns":
			writes.Add(1)
			item := firstItem(t, body, "Campaigns")
			if body["AccountId"] != testAccountID || item["Status"] != "Paused" || item["CampaignType"] != "Search" || item["BudgetType"] != "DailyBudgetStandard" {
				t.Fatalf("create campaign body=%v", body)
			}
			campaign.Name = item["Name"].(string)
			writeValue(t, writer, http.StatusOK, map[string]any{"CampaignIds": []string{testCampaignID}, "PartialErrors": []any{}})
		case http.MethodPut + " /Campaigns":
			writes.Add(1)
			item := firstItem(t, body, "Campaigns")
			applyCampaignUpdate(&campaign, item)
			writeValue(t, writer, http.StatusOK, map[string]any{"PartialErrors": []any{}})
		case http.MethodPost + " /AdGroups/QueryByCampaignId":
			if body["CampaignId"] != testCampaignID {
				t.Fatalf("list ad groups body=%v", body)
			}
			writeValue(t, writer, http.StatusOK, map[string]any{"AdGroups": []AdGroup{adGroup}})
		case http.MethodPost + " /AdGroups/QueryByIds":
			assertIDs(t, body, "CampaignId", testCampaignID, "AdGroupIds", testAdGroupID)
			writeValue(t, writer, http.StatusOK, map[string]any{"AdGroups": []AdGroup{adGroup}, "PartialErrors": []any{}})
		case http.MethodPost + " /AdGroups":
			writes.Add(1)
			item := firstItem(t, body, "AdGroups")
			if body["CampaignId"] != testCampaignID || item["Status"] != "Paused" || item["Network"] != string(NetworkOwnedAndOperatedAndSyndicatedSearch) {
				t.Fatalf("create ad group body=%v", body)
			}
			adGroup.Name = item["Name"].(string)
			writeValue(t, writer, http.StatusOK, map[string]any{"AdGroupIds": []string{testAdGroupID}, "PartialErrors": []any{}})
		case http.MethodPut + " /AdGroups":
			writes.Add(1)
			item := firstItem(t, body, "AdGroups")
			applyAdGroupUpdate(&adGroup, item)
			writeValue(t, writer, http.StatusOK, map[string]any{"PartialErrors": []any{}})
		case http.MethodPost + " /Ads/QueryByAdGroupId":
			if body["AdGroupId"] != testAdGroupID || !reflect.DeepEqual(body["AdTypes"], []any{"ResponsiveSearch"}) {
				t.Fatalf("list ads body=%v", body)
			}
			writeValue(t, writer, http.StatusOK, map[string]any{"Ads": []ResponsiveSearchAd{ad}})
		case http.MethodPost + " /Ads/QueryByIds":
			assertIDs(t, body, "AdGroupId", testAdGroupID, "AdIds", testAdID)
			writeValue(t, writer, http.StatusOK, map[string]any{"Ads": []ResponsiveSearchAd{ad}, "PartialErrors": []any{}})
		case http.MethodPost + " /Ads":
			writes.Add(1)
			item := firstItem(t, body, "Ads")
			if body["AdGroupId"] != testAdGroupID || item["Status"] != "Paused" || item["Type"] != "ResponsiveSearch" {
				t.Fatalf("create RSA body=%v", body)
			}
			assertTextAssets(t, item["Headlines"], 3)
			assertTextAssets(t, item["Descriptions"], 2)
			writeValue(t, writer, http.StatusOK, map[string]any{"AdIds": []string{testAdID}, "PartialErrors": []any{}})
		case http.MethodPut + " /Ads":
			writes.Add(1)
			item := firstItem(t, body, "Ads")
			if item["Type"] != "ResponsiveSearch" {
				t.Fatalf("update RSA body=%v", body)
			}
			applyAdUpdate(&ad, item)
			writeValue(t, writer, http.StatusOK, map[string]any{"PartialErrors": []any{}})
		case http.MethodPost + " /Keywords/QueryByAdGroupId":
			writeValue(t, writer, http.StatusOK, map[string]any{"Keywords": []Keyword{keyword}})
		case http.MethodPost + " /Keywords/QueryByIds":
			assertIDs(t, body, "AdGroupId", testAdGroupID, "KeywordIds", testKeywordID)
			writeValue(t, writer, http.StatusOK, map[string]any{"Keywords": []Keyword{keyword}, "PartialErrors": []any{}})
		case http.MethodPost + " /Keywords":
			writes.Add(1)
			item := firstItem(t, body, "Keywords")
			if body["AdGroupId"] != testAdGroupID || item["Status"] != "Paused" || item["MatchType"] != "Phrase" {
				t.Fatalf("create keyword body=%v", body)
			}
			writeValue(t, writer, http.StatusOK, map[string]any{"KeywordIds": []string{testKeywordID}, "PartialErrors": []any{}})
		case http.MethodPut + " /Keywords":
			writes.Add(1)
			item := firstItem(t, body, "Keywords")
			applyKeywordUpdate(&keyword, item)
			writeValue(t, writer, http.StatusOK, map[string]any{"PartialErrors": []any{}})
		case http.MethodPost + " /GenerateReport/Submit":
			report := body["ReportRequest"].(map[string]any)
			if report["Type"] != "CampaignPerformanceReportRequest" || report["Format"] != "Csv" || report["FormatVersion"] != "2.0" {
				t.Fatalf("submit report body=%v", body)
			}
			scope := report["Scope"].(map[string]any)
			campaigns := scope["Campaigns"].([]any)
			if campaigns[0].(map[string]any)["AccountId"] != testAccountID || campaigns[0].(map[string]any)["CampaignId"] != testCampaignID {
				t.Fatalf("report scope=%v", scope)
			}
			writeValue(t, writer, http.StatusOK, map[string]any{"ReportRequestId": "report-request-1"})
		case http.MethodPost + " /GenerateReport/Poll":
			writeValue(t, writer, http.StatusOK, map[string]any{"ReportRequestStatus": ReportRequestStatus{Status: "Success", ReportDownloadURL: reportURL}})
		default:
			t.Fatalf("unexpected request: %s %s body=%v", request.Method, request.URL, body)
		}
	}))
	defer server.Close()
	reportURL = server.URL + "/report.zip?sig=secret"
	_, client := newTestAdapter(t, server)
	ctx := context.Background()

	if account, err := client.GetAccount(ctx); err != nil || account.ID != testAccountID {
		t.Fatalf("account=%#v err=%v", account, err)
	}
	if values, err := client.ListCampaigns(ctx); err != nil || len(values) != 1 {
		t.Fatalf("campaigns=%#v err=%v", values, err)
	}
	if _, err := client.GetCampaign(ctx, testCampaignID); err != nil {
		t.Fatal(err)
	}
	createdCampaign, err := client.CreateCampaign(ctx, CreateCampaignRequest{Name: "Created search", DailyBudget: 20, TimeZone: "PacificTimeUSCanadaTijuana", Languages: []string{"English"}})
	if err != nil || createdCampaign.Status != StatusPaused {
		t.Fatalf("created campaign=%#v err=%v", createdCampaign, err)
	}
	updatedCampaign, err := client.UpdateCampaign(ctx, testCampaignID, UpdateCampaignRequest{Name: stringPointer("Updated search"), DailyBudget: floatPointer(30)})
	if err != nil || updatedCampaign.Name != "Updated search" || updatedCampaign.DailyBudget != 30 {
		t.Fatalf("updated campaign=%#v err=%v", updatedCampaign, err)
	}
	if value, err := client.SetCampaignStatus(ctx, testCampaignID, StatusActive); err != nil || value.Status != StatusActive {
		t.Fatalf("campaign status=%#v err=%v", value, err)
	}

	if values, err := client.ListAdGroups(ctx, testCampaignID); err != nil || len(values) != 1 {
		t.Fatalf("ad groups=%#v err=%v", values, err)
	}
	createdAdGroup, err := client.CreateAdGroup(ctx, testCampaignID, CreateAdGroupRequest{Name: "Created group", CPCBid: floatPointer(2), Language: "English"})
	if err != nil || createdAdGroup.Status != StatusPaused {
		t.Fatalf("created ad group=%#v err=%v", createdAdGroup, err)
	}
	updatedAdGroup, err := client.UpdateAdGroup(ctx, testCampaignID, testAdGroupID, UpdateAdGroupRequest{Name: stringPointer("Updated group"), Network: networkPointer(NetworkOwnedAndOperatedOnly)})
	if err != nil || updatedAdGroup.Name != "Updated group" {
		t.Fatalf("updated ad group=%#v err=%v", updatedAdGroup, err)
	}
	if value, err := client.SetAdGroupStatus(ctx, testCampaignID, testAdGroupID, StatusActive); err != nil || value.Status != StatusActive {
		t.Fatalf("ad group status=%#v err=%v", value, err)
	}

	if values, err := client.ListResponsiveSearchAds(ctx, testCampaignID, testAdGroupID); err != nil || len(values) != 1 {
		t.Fatalf("ads=%#v err=%v", values, err)
	}
	createdAd, err := client.CreateResponsiveSearchAd(ctx, testCampaignID, testAdGroupID, CreateResponsiveSearchAdRequest{
		FinalURLs: []string{"https://example.com/new"}, Headlines: testAdTextAssets(3), Descriptions: testAdTextAssets(2), Path1: "offers",
	})
	if err != nil || createdAd.Status != StatusPaused {
		t.Fatalf("created ad=%#v err=%v", createdAd, err)
	}
	paths := "updated"
	updatedAd, err := client.UpdateResponsiveSearchAd(ctx, testCampaignID, testAdGroupID, testAdID, UpdateResponsiveSearchAdRequest{Path1: &paths})
	if err != nil || updatedAd.Path1 != "updated" {
		t.Fatalf("updated ad=%#v err=%v", updatedAd, err)
	}
	if value, err := client.SetResponsiveSearchAdStatus(ctx, testCampaignID, testAdGroupID, testAdID, StatusActive); err != nil || value.Status != StatusActive {
		t.Fatalf("ad status=%#v err=%v", value, err)
	}

	if values, err := client.ListKeywords(ctx, testCampaignID, testAdGroupID); err != nil || len(values) != 1 {
		t.Fatalf("keywords=%#v err=%v", values, err)
	}
	createdKeyword, err := client.CreateKeyword(ctx, testCampaignID, testAdGroupID, CreateKeywordRequest{Text: "running shoes", MatchType: MatchTypePhrase, Bid: floatPointer(1)})
	if err != nil || createdKeyword.Status != StatusPaused {
		t.Fatalf("created keyword=%#v err=%v", createdKeyword, err)
	}
	updatedKeyword, err := client.UpdateKeyword(ctx, testCampaignID, testAdGroupID, testKeywordID, UpdateKeywordRequest{Text: stringPointer("trail shoes"), MatchType: matchTypePointer(MatchTypeExact)})
	if err != nil || updatedKeyword.Text != "trail shoes" {
		t.Fatalf("updated keyword=%#v err=%v", updatedKeyword, err)
	}
	if value, err := client.SetKeywordStatus(ctx, testCampaignID, testAdGroupID, testKeywordID, StatusActive); err != nil || value.Status != StatusActive {
		t.Fatalf("keyword status=%#v err=%v", value, err)
	}

	requestID, err := client.SubmitCampaignPerformanceReport(ctx, CampaignPerformanceReportRequest{
		ReportName: "Campaign report", Columns: []string{"TimePeriod", "CampaignId", "Impressions"},
		CampaignIDs: []string{testCampaignID}, Time: ReportTime{PredefinedTime: "Yesterday"},
	})
	if err != nil || requestID != "report-request-1" {
		t.Fatalf("report request=%q err=%v", requestID, err)
	}
	status, err := client.PollReport(ctx, requestID)
	if err != nil || status.Status != "Success" {
		t.Fatalf("report status=%#v err=%v", status, err)
	}
	var destination bytes.Buffer
	written, err := client.DownloadReport(ctx, status.ReportDownloadURL, &destination)
	if err != nil || written != 10 || destination.String() != "zip-report" {
		t.Fatalf("download bytes=%d body=%q err=%v", written, destination.String(), err)
	}
	if writes.Load() != 12 {
		t.Fatalf("mutation writes=%d", writes.Load())
	}
}

func firstItem(t *testing.T, body map[string]any, key string) map[string]any {
	t.Helper()
	items, ok := body[key].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("%s=%v", key, body[key])
	}
	item, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("%s item=%v", key, items[0])
	}
	return item
}

func assertIDs(t *testing.T, body map[string]any, parentKey, parentID, idsKey, id string) {
	t.Helper()
	if body[parentKey] != parentID || !reflect.DeepEqual(body[idsKey], []any{id}) {
		t.Fatalf("ID request body=%v", body)
	}
}

func assertTextAssets(t *testing.T, raw any, count int) {
	t.Helper()
	items, ok := raw.([]any)
	if !ok || len(items) != count {
		t.Fatalf("assets=%v", raw)
	}
	asset := items[0].(map[string]any)["Asset"].(map[string]any)
	if asset["Type"] != "TextAsset" || asset["Text"] == "" {
		t.Fatalf("asset=%v", asset)
	}
}

func applyCampaignUpdate(value *Campaign, item map[string]any) {
	if name, ok := item["Name"].(string); ok {
		value.Name = name
	}
	if budget, ok := item["DailyBudget"].(float64); ok {
		value.DailyBudget = budget
	}
	if status, ok := item["Status"].(string); ok {
		value.Status = Status(status)
	}
}

func applyAdGroupUpdate(value *AdGroup, item map[string]any) {
	if name, ok := item["Name"].(string); ok {
		value.Name = name
	}
	if network, ok := item["Network"].(string); ok {
		value.Network = Network(network)
	}
	if status, ok := item["Status"].(string); ok {
		value.Status = Status(status)
	}
}

func applyAdUpdate(value *ResponsiveSearchAd, item map[string]any) {
	if path, ok := item["Path1"].(string); ok {
		value.Path1 = path
	}
	if status, ok := item["Status"].(string); ok {
		value.Status = Status(status)
	}
}

func applyKeywordUpdate(value *Keyword, item map[string]any) {
	if text, ok := item["Text"].(string); ok {
		value.Text = text
	}
	if matchType, ok := item["MatchType"].(string); ok {
		value.MatchType = MatchType(matchType)
	}
	if status, ok := item["Status"].(string); ok {
		value.Status = Status(status)
	}
}

func testAdTextAssets(count int) []AdTextAsset {
	values := make([]AdTextAsset, count)
	for index := range values {
		values[index] = AdTextAsset{Text: "Asset text " + string(rune('A'+index))}
	}
	return values
}

func testAssetLinks(count int) []AssetLink {
	values := make([]AssetLink, count)
	for index, asset := range testAdTextAssets(count) {
		values[index] = AssetLink{Asset: TextAsset{Type: "TextAsset", Text: asset.Text}}
	}
	return values
}

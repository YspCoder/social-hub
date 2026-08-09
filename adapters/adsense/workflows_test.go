package adsense

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestAccountInventoryComplianceAndReportingWorkflows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assertAPIRequest(t, request)
		if request.Method != http.MethodGet {
			t.Fatalf("method=%s path=%s", request.Method, request.URL.Path)
		}
		switch request.URL.Path {
		case "/v2/" + accountName():
			writeJSON(t, writer, http.StatusOK, Account{Name: accountName(), DisplayName: "Publisher", State: AccountReady})
		case "/v2/" + accountName() + ":listChildAccounts":
			assertListQuery(t, request)
			writeJSON(t, writer, http.StatusOK, map[string]any{"accounts": []Account{{Name: "accounts/pub-999", State: AccountReady}}, "nextPageToken": "child-next"})
		case "/v2/" + accountName() + "/adBlockingRecoveryTag":
			writeJSON(t, writer, http.StatusOK, AdBlockingRecoveryTag{Tag: "<script>recovery</script>", ErrorProtectionCode: "code"})
		case "/v2/" + adClientName():
			writeJSON(t, writer, http.StatusOK, AdClient{Name: adClientName(), ProductCode: "AFC", State: AdClientReady})
		case "/v2/" + accountName() + "/adclients":
			assertListQuery(t, request)
			writeJSON(t, writer, http.StatusOK, map[string]any{"adClients": []AdClient{{Name: adClientName()}}, "nextPageToken": "ad-client-next"})
		case "/v2/" + adClientName() + "/adcode":
			writeJSON(t, writer, http.StatusOK, AdClientAdCode{AdCode: "<script>ads</script>", AMPHead: "head", AMPBody: "body"})
		case "/v2/" + nestedName("adunits", testAdUnitID):
			writeJSON(t, writer, http.StatusOK, AdUnit{Name: nestedName("adunits", testAdUnitID), DisplayName: "Homepage", State: AdUnitActive})
		case "/v2/" + adClientName() + "/adunits":
			assertListQuery(t, request)
			writeJSON(t, writer, http.StatusOK, map[string]any{"adUnits": []AdUnit{{Name: nestedName("adunits", testAdUnitID)}}, "nextPageToken": "ad-unit-next"})
		case "/v2/" + nestedName("adunits", testAdUnitID) + "/adcode":
			writeJSON(t, writer, http.StatusOK, AdUnitAdCode{AdCode: "<ins>ad</ins>"})
		case "/v2/" + nestedName("adunits", testAdUnitID) + ":listLinkedCustomChannels":
			assertListQuery(t, request)
			writeJSON(t, writer, http.StatusOK, map[string]any{"customChannels": []CustomChannel{{Name: nestedName("customchannels", testChannelID)}}, "nextPageToken": "linked-channel-next"})
		case "/v2/" + nestedName("customchannels", testChannelID):
			writeJSON(t, writer, http.StatusOK, CustomChannel{Name: nestedName("customchannels", testChannelID), DisplayName: "News", Active: true})
		case "/v2/" + adClientName() + "/customchannels":
			assertListQuery(t, request)
			writeJSON(t, writer, http.StatusOK, map[string]any{"customChannels": []CustomChannel{{Name: nestedName("customchannels", testChannelID)}}, "nextPageToken": "channel-next"})
		case "/v2/" + nestedName("customchannels", testChannelID) + ":listLinkedAdUnits":
			assertListQuery(t, request)
			writeJSON(t, writer, http.StatusOK, map[string]any{"adUnits": []AdUnit{{Name: nestedName("adunits", testAdUnitID)}}, "nextPageToken": "linked-unit-next"})
		case "/v2/" + nestedName("urlchannels", testURLChannelID):
			writeJSON(t, writer, http.StatusOK, URLChannel{Name: nestedName("urlchannels", testURLChannelID), URIPattern: "example.com/news/*"})
		case "/v2/" + adClientName() + "/urlchannels":
			assertListQuery(t, request)
			writeJSON(t, writer, http.StatusOK, map[string]any{"urlChannels": []URLChannel{{Name: nestedName("urlchannels", testURLChannelID)}}, "nextPageToken": "url-next"})
		case "/v2/" + resourceName("sites", testSiteID):
			writeJSON(t, writer, http.StatusOK, Site{Name: resourceName("sites", testSiteID), Domain: "example.com", State: SiteReady})
		case "/v2/" + accountName() + "/sites":
			assertListQuery(t, request)
			writeJSON(t, writer, http.StatusOK, map[string]any{"sites": []Site{{Name: resourceName("sites", testSiteID)}}, "nextPageToken": "site-next"})
		case "/v2/" + accountName() + "/alerts":
			if request.URL.Query().Get("languageCode") != "zh-CN" {
				t.Errorf("alert query=%v", request.URL.Query())
			}
			writeJSON(t, writer, http.StatusOK, map[string]any{"alerts": []Alert{{Name: resourceName("alerts", "alert-1"), Severity: AlertWarning}}})
		case "/v2/" + accountName() + "/payments":
			writeJSON(t, writer, http.StatusOK, map[string]any{"payments": []Payment{{Name: resourceName("payments", "youtube-2026-08-01"), Amount: "$1.00"}}})
		case "/v2/" + resourceName("policyIssues", testIssueID):
			writeJSON(t, writer, http.StatusOK, policyIssueResource())
		case "/v2/" + accountName() + "/policyIssues":
			assertListQuery(t, request)
			writeJSON(t, writer, http.StatusOK, map[string]any{"policyIssues": []PolicyIssue{policyIssueResource()}, "nextPageToken": "policy-next"})
		case "/v2/" + accountName() + "/reports:generate":
			assertAdHocReportQuery(t, request)
			writeJSON(t, writer, http.StatusOK, reportResult([]map[string]any{
				{"name": "DATE", "type": "DIMENSION"},
				{"name": "CLICKS", "type": "METRIC_TALLY"},
				{"name": "ESTIMATED_EARNINGS", "type": "METRIC_CURRENCY", "currencyCode": "USD"},
			}))
		case "/v2/" + resourceName("reports", testReportID) + "/saved":
			writeJSON(t, writer, http.StatusOK, SavedReport{Name: resourceName("reports", testReportID), Title: "Daily revenue"})
		case "/v2/" + accountName() + "/reports/saved":
			assertListQuery(t, request)
			writeJSON(t, writer, http.StatusOK, map[string]any{"savedReports": []SavedReport{{Name: resourceName("reports", testReportID)}}, "nextPageToken": "report-next"})
		case "/v2/" + resourceName("reports", testReportID) + "/saved:generate":
			if request.URL.Query().Get("dateRange") != "YESTERDAY" || request.URL.Query().Get("reportingTimeZone") != "GOOGLE_TIME_ZONE" {
				t.Errorf("saved report query=%v", request.URL.Query())
			}
			writeJSON(t, writer, http.StatusOK, reportResult([]map[string]any{
				{"name": "COUNTRY_CODE", "type": "DIMENSION"}, {"name": "CLICKS", "type": "METRIC_TALLY"},
			}))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newStaticClient(t, server)
	ctx := context.Background()
	list := ListRequest{PageSize: 50, PageToken: "next"}

	if value, err := client.GetAccount(ctx); err != nil || value.State != AccountReady || len(value.Raw) == 0 {
		t.Fatalf("account=%#v err=%v", value, err)
	}
	if page, err := client.ListChildAccounts(ctx, list); err != nil || len(page.Items) != 1 || page.NextPageToken != "child-next" {
		t.Fatalf("children=%#v err=%v", page, err)
	}
	if value, err := client.GetAdBlockingRecoveryTag(ctx); err != nil || value.Tag == "" || len(value.Raw) == 0 {
		t.Fatalf("recovery=%#v err=%v", value, err)
	}
	if value, err := client.GetAdClient(ctx, testAdClientID); err != nil || value.State != AdClientReady || len(value.Raw) == 0 {
		t.Fatalf("ad client=%#v err=%v", value, err)
	}
	if page, err := client.ListAdClients(ctx, list); err != nil || len(page.Items) != 1 || page.NextPageToken != "ad-client-next" {
		t.Fatalf("ad clients=%#v err=%v", page, err)
	}
	if value, err := client.GetAdClientAdCode(ctx, testAdClientID); err != nil || value.AMPHead != "head" || len(value.Raw) == 0 {
		t.Fatalf("client code=%#v err=%v", value, err)
	}
	if value, err := client.GetAdUnit(ctx, testAdClientID, testAdUnitID); err != nil || value.State != AdUnitActive || len(value.Raw) == 0 {
		t.Fatalf("ad unit=%#v err=%v", value, err)
	}
	if page, err := client.ListAdUnits(ctx, testAdClientID, list); err != nil || len(page.Items) != 1 || page.NextPageToken != "ad-unit-next" {
		t.Fatalf("ad units=%#v err=%v", page, err)
	}
	if value, err := client.GetAdUnitAdCode(ctx, testAdClientID, testAdUnitID); err != nil || value.AdCode == "" || len(value.Raw) == 0 {
		t.Fatalf("unit code=%#v err=%v", value, err)
	}
	if page, err := client.ListAdUnitCustomChannels(ctx, testAdClientID, testAdUnitID, list); err != nil || len(page.Items) != 1 || page.NextPageToken != "linked-channel-next" {
		t.Fatalf("linked channels=%#v err=%v", page, err)
	}
	if value, err := client.GetCustomChannel(ctx, testAdClientID, testChannelID); err != nil || !value.Active || len(value.Raw) == 0 {
		t.Fatalf("custom channel=%#v err=%v", value, err)
	}
	if page, err := client.ListCustomChannels(ctx, testAdClientID, list); err != nil || len(page.Items) != 1 || page.NextPageToken != "channel-next" {
		t.Fatalf("custom channels=%#v err=%v", page, err)
	}
	if page, err := client.ListCustomChannelAdUnits(ctx, testAdClientID, testChannelID, list); err != nil || len(page.Items) != 1 || page.NextPageToken != "linked-unit-next" {
		t.Fatalf("linked units=%#v err=%v", page, err)
	}
	if value, err := client.GetURLChannel(ctx, testAdClientID, testURLChannelID); err != nil || value.URIPattern == "" || len(value.Raw) == 0 {
		t.Fatalf("URL channel=%#v err=%v", value, err)
	}
	if page, err := client.ListURLChannels(ctx, testAdClientID, list); err != nil || len(page.Items) != 1 || page.NextPageToken != "url-next" {
		t.Fatalf("URL channels=%#v err=%v", page, err)
	}
	if value, err := client.GetSite(ctx, testSiteID); err != nil || value.State != SiteReady || len(value.Raw) == 0 {
		t.Fatalf("site=%#v err=%v", value, err)
	}
	if page, err := client.ListSites(ctx, list); err != nil || len(page.Items) != 1 || page.NextPageToken != "site-next" {
		t.Fatalf("sites=%#v err=%v", page, err)
	}
	if values, err := client.ListAlerts(ctx, "zh-CN"); err != nil || len(values) != 1 || len(values[0].Raw) == 0 {
		t.Fatalf("alerts=%#v err=%v", values, err)
	}
	if values, err := client.ListPayments(ctx); err != nil || len(values) != 1 || len(values[0].Raw) == 0 {
		t.Fatalf("payments=%#v err=%v", values, err)
	}
	if value, err := client.GetPolicyIssue(ctx, testIssueID); err != nil || value.AdRequestCount != "12" || len(value.Raw) == 0 {
		t.Fatalf("policy=%#v err=%v", value, err)
	}
	if page, err := client.ListPolicyIssues(ctx, list); err != nil || len(page.Items) != 1 || page.NextPageToken != "policy-next" {
		t.Fatalf("policies=%#v err=%v", page, err)
	}

	request := GenerateReportRequest{
		Dimensions: []Dimension{DimensionDate}, Metrics: []Metric{MetricClicks, MetricEstimatedEarnings},
		DateRange: ReportDateCustom, StartDate: validDateFixture(), EndDate: validDateFixture(),
		ReportingTimeZone: ReportingTimeZoneAccount, CurrencyCode: "USD", LanguageCode: "en-US",
		Filters: []string{"COUNTRY_CODE==US"}, OrderBy: []string{"-ESTIMATED_EARNINGS"},
	}
	if value, err := client.GenerateReport(ctx, request, socialhub.WithRequestID("caller-request")); err != nil || value.TotalMatchedRows != 1 || len(value.Raw) == 0 {
		t.Fatalf("report=%#v err=%v", value, err)
	}
	if value, err := client.GetSavedReport(ctx, testReportID); err != nil || value.Title == "" || len(value.Raw) == 0 {
		t.Fatalf("saved report=%#v err=%v", value, err)
	}
	if page, err := client.ListSavedReports(ctx, list); err != nil || len(page.Items) != 1 || page.NextPageToken != "report-next" {
		t.Fatalf("saved reports=%#v err=%v", page, err)
	}
	if value, err := client.GenerateSavedReport(ctx, testReportID, GenerateSavedReportRequest{DateRange: ReportDateYesterday, ReportingTimeZone: ReportingTimeZoneGoogle}); err != nil || value.TotalMatchedRows != 1 {
		t.Fatalf("generated saved report=%#v err=%v", value, err)
	}
}

func assertListQuery(t *testing.T, request *http.Request) {
	t.Helper()
	if request.URL.Query().Get("pageSize") != "50" || request.URL.Query().Get("pageToken") != "next" {
		t.Errorf("list query=%v", request.URL.Query())
	}
}

func assertAdHocReportQuery(t *testing.T, request *http.Request) {
	t.Helper()
	query := request.URL.Query()
	if query.Get("dateRange") != "CUSTOM" || query.Get("startDate.year") != "2026" || query.Get("limit") != "10000" ||
		query.Get("reportingTimeZone") != "ACCOUNT_TIME_ZONE" || query.Get("currencyCode") != "USD" ||
		len(query["dimensions"]) != 1 || len(query["metrics"]) != 2 || len(query["filters"]) != 1 || len(query["orderBy"]) != 1 {
		t.Errorf("report query=%v", query)
	}
	if request.Header.Get("X-Request-ID") != "caller-request" {
		t.Errorf("request ID=%q", request.Header.Get("X-Request-ID"))
	}
}

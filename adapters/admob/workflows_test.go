package admob

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestCompleteStableV1Workflows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/v1/accounts":
			assertAPIRequest(t, request, http.MethodGet, "/v1/accounts")
			assertListQuery(t, request)
			writeJSON(t, writer, http.StatusOK, map[string]any{"account": []PublisherAccount{accountFixture()}, "nextPageToken": "account-next"})
		case "/v1/" + accountName():
			assertAPIRequest(t, request, http.MethodGet, "/v1/"+accountName())
			writeJSON(t, writer, http.StatusOK, accountFixture())
		case "/v1/" + accountName() + "/apps":
			assertAPIRequest(t, request, http.MethodGet, "/v1/"+accountName()+"/apps")
			assertListQuery(t, request)
			writeJSON(t, writer, http.StatusOK, map[string]any{"apps": []App{appFixture()}, "nextPageToken": "app-next"})
		case "/v1/" + accountName() + "/adUnits":
			assertAPIRequest(t, request, http.MethodGet, "/v1/"+accountName()+"/adUnits")
			assertListQuery(t, request)
			writeJSON(t, writer, http.StatusOK, map[string]any{"adUnits": []AdUnit{adUnitFixture()}, "nextPageToken": "unit-next"})
		case "/v1/" + accountName() + "/networkReport:generate":
			assertAPIRequest(t, request, http.MethodPost, "/v1/"+accountName()+"/networkReport:generate")
			if request.Header.Get("Content-Type") != "application/json" || request.Header.Get("X-Request-ID") != "caller-request" {
				t.Errorf("report headers=%v", request.Header)
			}
			var input GenerateNetworkReportRequest
			if err := json.NewDecoder(request.Body).Decode(&input); err != nil || input.ReportSpec.MaxReportRows != 10 ||
				len(input.ReportSpec.Metrics) != 2 || input.ReportSpec.TimeZone != "America/Los_Angeles" {
				t.Fatalf("network input=%#v err=%v", input, err)
			}
			writeJSON(t, writer, http.StatusOK, reportResponse(input.ReportSpec.Dimensions, input.ReportSpec.Metrics))
		case "/v1/" + accountName() + "/mediationReport:generate":
			assertAPIRequest(t, request, http.MethodPost, "/v1/"+accountName()+"/mediationReport:generate")
			var input GenerateMediationReportRequest
			if err := json.NewDecoder(request.Body).Decode(&input); err != nil || len(input.ReportSpec.DimensionFilters) != 1 ||
				len(input.ReportSpec.SortConditions) != 1 {
				t.Fatalf("mediation input=%#v err=%v", input, err)
			}
			response := reportResponse(input.ReportSpec.Dimensions, input.ReportSpec.Metrics)
			response[2] = map[string]any{"footer": map[string]any{
				"matchingRowCount": "9", "warnings": []ReportWarning{{Type: ReportWarningDataDelayed, Description: "Recent data is delayed"}},
			}}
			writeJSON(t, writer, http.StatusOK, response)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newStaticClient(t, server)
	ctx := context.Background()
	list := ListRequest{PageSize: 50, PageToken: "next"}

	if page, err := client.ListAccounts(ctx, list); err != nil || len(page.Items) != 1 || page.NextPageToken != "account-next" || len(page.Items[0].Raw) == 0 {
		t.Fatalf("accounts=%#v err=%v", page, err)
	}
	if account, err := client.GetAccount(ctx); err != nil || account.PublisherID != testPublisherID || len(account.Raw) == 0 {
		t.Fatalf("account=%#v err=%v", account, err)
	}
	if page, err := client.ListApps(ctx, list); err != nil || len(page.Items) != 1 || page.NextPageToken != "app-next" || len(page.Items[0].Raw) == 0 {
		t.Fatalf("apps=%#v err=%v", page, err)
	}
	if page, err := client.ListAdUnits(ctx, list); err != nil || len(page.Items) != 1 || page.NextPageToken != "unit-next" || len(page.Items[0].Raw) == 0 {
		t.Fatalf("ad units=%#v err=%v", page, err)
	}
	network, err := client.GenerateNetworkReport(ctx, validNetworkSpec(), socialhub.WithRequestID("caller-request"))
	if err != nil || len(network.Rows) != 1 || network.Footer.MatchingRowCount != 1 ||
		network.Rows[0].MetricValues[MetricClicks].IntegerValue == nil || network.Rows[0].MetricValues[MetricEstimatedEarnings].MicrosValue == nil {
		t.Fatalf("network=%#v err=%v", network, err)
	}
	mediation, err := client.GenerateMediationReport(ctx, validMediationSpec())
	if err != nil || len(mediation.Rows) != 1 || mediation.Footer.MatchingRowCount != 9 || len(mediation.Footer.Warnings) != 1 {
		t.Fatalf("mediation=%#v err=%v", mediation, err)
	}
	if mediation.Rows[0].MetricValues[MetricObservedECPM].MicrosValue == nil {
		t.Fatal("mediation micros metric was not preserved")
	}
}

func TestAdUnitDimensionAutomaticallyIncludesApp(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var input GenerateNetworkReportRequest
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			t.Fatal(err)
		}
		writeJSON(t, writer, http.StatusOK, reportResponse(input.ReportSpec.Dimensions, input.ReportSpec.Metrics))
	}))
	defer server.Close()
	_, client := newStaticClient(t, server)
	spec := validNetworkSpec()
	spec.Dimensions = []Dimension{DimensionAdUnit}
	spec.Metrics = []Metric{MetricImpressionCTR}
	report, err := client.GenerateNetworkReport(context.Background(), spec)
	if err != nil || len(report.Rows[0].DimensionValues) != 2 || report.Rows[0].DimensionValues[DimensionApp].Value == "" ||
		report.Rows[0].MetricValues[MetricImpressionCTR].DoubleValue == nil {
		t.Fatalf("report=%#v err=%v", report, err)
	}
}

func assertListQuery(t *testing.T, request *http.Request) {
	t.Helper()
	if request.URL.Query().Get("pageSize") != "50" || request.URL.Query().Get("pageToken") != "next" {
		t.Errorf("list query=%v", request.URL.Query())
	}
}

package admanager

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestInventoryDeliveryAndReportingWorkflows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assertAPIRequest(t, request)
		switch request.Method + " " + request.URL.Path {
		case http.MethodGet + " /v1/" + networkName():
			writeJSON(t, writer, http.StatusOK, networkResource())
		case http.MethodGet + " /v1/" + resourceName("companies", testCompanyID):
			writeJSON(t, writer, http.StatusOK, companyResource())
		case http.MethodGet + " /v1/" + networkName() + "/companies":
			assertListQuery(t, request)
			writeJSON(t, writer, http.StatusOK, map[string]any{"companies": []Company{companyResource()}, "nextPageToken": "company-next", "totalSize": 1})
		case http.MethodGet + " /v1/" + resourceName("adUnits", testAdUnitID):
			writeJSON(t, writer, http.StatusOK, adUnitResource())
		case http.MethodGet + " /v1/" + networkName() + "/adUnits":
			assertListQuery(t, request)
			writeJSON(t, writer, http.StatusOK, map[string]any{"adUnits": []AdUnit{adUnitResource()}, "nextPageToken": "unit-next", "totalSize": 1})
		case http.MethodGet + " /v1/" + resourceName("orders", testOrderID):
			writeJSON(t, writer, http.StatusOK, orderResource())
		case http.MethodGet + " /v1/" + networkName() + "/orders":
			assertListQuery(t, request)
			writeJSON(t, writer, http.StatusOK, map[string]any{"orders": []Order{orderResource()}, "nextPageToken": "order-next", "totalSize": 1})
		case http.MethodGet + " /v1/" + resourceName("lineItems", testLineItemID):
			writeJSON(t, writer, http.StatusOK, lineItemResource())
		case http.MethodGet + " /v1/" + networkName() + "/lineItems":
			assertListQuery(t, request)
			writeJSON(t, writer, http.StatusOK, map[string]any{"lineItems": []LineItem{lineItemResource()}, "nextPageToken": "line-next", "totalSize": 1})
		case http.MethodGet + " /v1/" + resourceName("reports", testReportID):
			writeJSON(t, writer, http.StatusOK, reportResource())
		case http.MethodGet + " /v1/" + networkName() + "/reports":
			assertListQuery(t, request)
			writeJSON(t, writer, http.StatusOK, map[string]any{"reports": []Report{reportResource()}, "nextPageToken": "report-next", "totalSize": 1})
		case http.MethodPost + " /v1/" + networkName() + "/reports":
			var payload struct {
				DisplayName      string           `json:"displayName"`
				ReportDefinition ReportDefinition `json:"reportDefinition"`
				Visibility       ReportVisibility `json:"visibility"`
			}
			decodeJSONBody(t, request, &payload)
			if payload.DisplayName != "Yesterday delivery" || payload.Visibility != ReportHidden || payload.ReportDefinition.ReportType != ReportHistorical {
				t.Errorf("report payload=%#v", payload)
			}
			writeJSON(t, writer, http.StatusOK, reportResource())
		case http.MethodPost + " /v1/" + resourceName("reports", testReportID) + ":run":
			var payload map[string]any
			decodeJSONBody(t, request, &payload)
			if len(payload) != 0 {
				t.Errorf("run payload=%v", payload)
			}
			writeJSON(t, writer, http.StatusOK, map[string]any{
				"name": operationName(), "metadata": map[string]any{"@type": "type.googleapis.com/google.ads.admanager.v1.RunReportMetadata", "report": resourceName("reports", testReportID), "percentComplete": 10},
			})
		case http.MethodGet + " /v1/" + operationName():
			writeJSON(t, writer, http.StatusOK, map[string]any{
				"name": operationName(), "done": true,
				"metadata": map[string]any{"report": resourceName("reports", testReportID), "percentComplete": 100},
				"response": map[string]any{"@type": "type.googleapis.com/google.ads.admanager.v1.RunReportResponse", "reportResult": resultName()},
			})
		case http.MethodGet + " /v1/" + resultName() + ":fetchRows":
			if request.URL.Query().Get("pageSize") != "10000" || request.URL.Query().Get("pageToken") != "rows-next" {
				t.Errorf("rows query=%v", request.URL.Query())
			}
			writeJSON(t, writer, http.StatusOK, map[string]any{
				"rows": []any{map[string]any{
					"dimensionValues":   []any{map[string]any{"stringValue": "Homepage standard"}, map[string]any{"intValue": testLineItemID}},
					"metricValueGroups": []any{map[string]any{"primaryValues": []any{map[string]any{"intValue": "42"}}}},
				}},
				"runTime": "2026-08-09T12:00:00Z", "dateRanges": []FixedDateRange{{StartDate: Date{2026, 8, 8}, EndDate: Date{2026, 8, 8}}},
				"totalRowCount": 1, "nextPageToken": "rows-final",
			})
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL)
		}
	}))
	defer server.Close()
	_, client := newStaticClient(t, server)
	ctx := context.Background()
	list := ListRequest{PageSize: 50, PageToken: "next", Filter: "status = ACTIVE", OrderBy: "displayName desc", Skip: 2}

	if value, err := client.GetNetwork(ctx); err != nil || value.NetworkCode != testNetworkCode || len(value.Raw) == 0 {
		t.Fatalf("network=%#v err=%v", value, err)
	}
	if value, err := client.GetCompany(ctx, testCompanyID); err != nil || value.Type != CompanyAdvertiser || len(value.Raw) == 0 {
		t.Fatalf("company=%#v err=%v", value, err)
	}
	if page, err := client.ListCompanies(ctx, list); err != nil || len(page.Items) != 1 || page.NextPageToken != "company-next" {
		t.Fatalf("companies=%#v err=%v", page, err)
	}
	if value, err := client.GetAdUnit(ctx, testAdUnitID); err != nil || value.Status != AdUnitActive || len(value.Raw) == 0 {
		t.Fatalf("ad unit=%#v err=%v", value, err)
	}
	if page, err := client.ListAdUnits(ctx, list); err != nil || len(page.Items) != 1 || page.NextPageToken != "unit-next" {
		t.Fatalf("ad units=%#v err=%v", page, err)
	}
	if value, err := client.GetOrder(ctx, testOrderID); err != nil || value.Status != OrderApproved || len(value.Raw) == 0 {
		t.Fatalf("order=%#v err=%v", value, err)
	}
	if page, err := client.ListOrders(ctx, list); err != nil || len(page.Items) != 1 || page.NextPageToken != "order-next" {
		t.Fatalf("orders=%#v err=%v", page, err)
	}
	if value, err := client.GetLineItem(ctx, testLineItemID); err != nil || value.Status != "DELIVERING" || len(value.Raw) == 0 {
		t.Fatalf("line item=%#v err=%v", value, err)
	}
	if page, err := client.ListLineItems(ctx, list); err != nil || len(page.Items) != 1 || page.NextPageToken != "line-next" {
		t.Fatalf("line items=%#v err=%v", page, err)
	}
	if value, err := client.GetReport(ctx, testReportID); err != nil || value.Visibility != ReportHidden || len(value.Raw) == 0 {
		t.Fatalf("report=%#v err=%v", value, err)
	}
	if page, err := client.ListReports(ctx, list); err != nil || len(page.Items) != 1 || page.NextPageToken != "report-next" {
		t.Fatalf("reports=%#v err=%v", page, err)
	}
	if value, err := client.CreateHiddenReport(ctx, CreateReportRequest{DisplayName: "Yesterday delivery", Definition: reportDefinition()}); err != nil || value.Visibility != ReportHidden {
		t.Fatalf("created=%#v err=%v", value, err)
	}
	started, err := client.RunReport(ctx, testReportID, socialhub.WithRequestID("caller-request"))
	if err != nil || started.Done || started.Metadata.PercentComplete != 10 {
		t.Fatalf("started=%#v err=%v", started, err)
	}
	completed, err := client.GetReportOperation(ctx, operationName())
	if err != nil || !completed.Done || completed.Result == nil || completed.Result.ReportResult != resultName() || len(completed.Raw) == 0 {
		t.Fatalf("completed=%#v err=%v", completed, err)
	}
	rows, err := client.FetchReportRows(ctx, FetchReportRowsRequest{ResultName: resultName(), PageSize: 10000, PageToken: "rows-next"})
	if err != nil || len(rows.Rows) != 1 || rows.TotalRowCount != 1 || rows.NextPageToken != "rows-final" {
		t.Fatalf("rows=%#v err=%v", rows, err)
	}
}

func assertListQuery(t *testing.T, request *http.Request) {
	t.Helper()
	query := request.URL.Query()
	if query.Get("pageSize") != "50" || query.Get("pageToken") != "next" || query.Get("filter") != "status = ACTIVE" ||
		query.Get("orderBy") != "displayName desc" || query.Get("skip") != "2" {
		t.Errorf("list query=%v", query)
	}
	if query.Get("fields") == "" {
		t.Errorf("totalSize field mask missing: %v", query)
	}
}

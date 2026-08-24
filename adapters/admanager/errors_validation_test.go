package admanager

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestGoogleErrorClassificationRetryAndRedaction(t *testing.T) {
	body := []byte(`{"error":{"code":429,"message":"access_token=secret-value exhausted","status":"RESOURCE_EXHAUSTED","details":[{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"RATE_LIMIT_EXCEEDED","domain":"admanager.googleapis.com"}]}}`)
	header := http.Header{"X-Goog-Request-Id": {"request-1"}, "Retry-After": {"2.5"}}
	err := decodeHTTPError(http.StatusTooManyRequests, header, body)
	var api *APIError
	if !errors.As(err, &api) || !api.Retryable() || api.Hub.Code != socialhub.CodeRateLimited || api.Hub.RetryAfter != 2500*time.Millisecond ||
		api.Hub.RequestID != "request-1" || strings.Contains(api.Hub.PlatformMessage, "secret-value") || api.Hub.PlatformCode != "RATE_LIMIT_EXCEEDED" {
		t.Fatalf("error=%#v", api)
	}
	if !errors.Is(err, socialhub.ErrRateLimited) || api.Error() == "" || api.Unwrap() == nil {
		t.Fatalf("wrapped error=%v", err)
	}
	if (&APIError{}).Error() == "" || (*APIError)(nil).Error() == "" || (*APIError)(nil).Unwrap() != nil || (*APIError)(nil).Retryable() {
		t.Fatal("nil API error contract failed")
	}
}

func TestErrorClassificationAndHelpers(t *testing.T) {
	tests := []struct {
		status      int
		rpc, reason string
		code        socialhub.ErrorCode
		class       socialhub.ErrorClass
	}{
		{403, "", "DAILY_LIMIT_EXCEEDED", socialhub.CodeRateLimited, socialhub.ClassUserAction},
		{403, "", "QUOTA_EXCEEDED", socialhub.CodeRateLimited, socialhub.ClassRetryable},
		{401, "", "UNAUTHENTICATED", socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{403, "", "PERMISSION_DENIED", socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{404, "", "NOT_FOUND", socialhub.CodeNotFound, socialhub.ClassPermanent},
		{503, "", "BACKEND_ERROR", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{400, "", "INVALID_ARGUMENT", socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{400, "INVALID_ARGUMENT", "", socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{429, "RESOURCE_EXHAUSTED", "", socialhub.CodeRateLimited, socialhub.ClassRetryable},
		{401, "UNAUTHENTICATED", "", socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{403, "PERMISSION_DENIED", "", socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{404, "NOT_FOUND", "", socialhub.CodeNotFound, socialhub.ClassPermanent},
		{409, "ABORTED", "", socialhub.CodeConflict, socialhub.ClassPermanent},
		{503, "UNAVAILABLE", "", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{422, "", "", socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{401, "", "", socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{403, "", "", socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{410, "", "", socialhub.CodeNotFound, socialhub.ClassPermanent},
		{409, "", "", socialhub.CodeConflict, socialhub.ClassPermanent},
		{429, "", "", socialhub.CodeRateLimited, socialhub.ClassRetryable},
		{500, "", "", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{418, "", "", socialhub.CodePlatformError, socialhub.ClassPermanent},
	}
	for _, test := range tests {
		code, class := classifyError(test.status, test.rpc, test.reason)
		if code != test.code || class != test.class {
			t.Errorf("status=%d rpc=%q reason=%q got=%s/%s", test.status, test.rpc, test.reason, code, class)
		}
	}
	policy := DefaultQuotaPolicy()
	if policy.StandardNetworkRequestsPerSecond != 2 || policy.AdManager360RequestsPerSecond != 8 || policy.InitialQuotaRetryDelay != 5*time.Second || policy.MaximumListPageSize != 1000 || policy.MaximumReportRowsPageSize != 10000 {
		t.Fatalf("policy=%#v", policy)
	}
	if parseRetryAfter("1.5") != 1500*time.Millisecond || parseRetryAfter("bad") != 0 || firstNonEmpty("", "value") != "value" || redactSensitive("client_secret: topsecret") == "client_secret: topsecret" {
		t.Fatal("error helpers failed")
	}
	if !errors.Is(ownershipError("test", "order"), socialhub.ErrPermissionDenied) || requireHubError(t, platformContractError("test", "bad")).Code != socialhub.CodePlatformError {
		t.Fatal("typed errors failed")
	}
}

func TestValidationPrimitivesAndReportDefinitions(t *testing.T) {
	client := &Client{networkCode: testNetworkCode}
	if !validID("123") || validID("0") || validID("bad") || validID(strings.Repeat("1", 21)) || !validName("Report", 10) || validName(" Report ", 10) ||
		!validListRequest(ListRequest{PageSize: 1000}, 1000) || validListRequest(ListRequest{PageSize: 1001}, 1000) ||
		!validEnumName("AD_SERVER_IMPRESSIONS") || validEnumName("bad value") || !validCurrency("USD") || validCurrency("usd") ||
		!client.ownsResource(resourceName("orders", testOrderID), "orders") || client.ownsResource("networks/999/orders/1", "orders") ||
		!client.ownsReportOperation(operationName()) || !client.ownsReportResult(resultName()) || client.ownsReportResult(resourceName("reports", testReportID)) {
		t.Fatal("validation primitives failed")
	}
	if _, err := client.resourceName("test", "orders", "bad"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("resource error=%v", err)
	}
	if !validReportDefinition(reportDefinition()) {
		t.Fatal("valid report rejected")
	}
	fixed := reportDefinition()
	fixed.DateRange = DateRange{Fixed: &FixedDateRange{StartDate: Date{2026, 8, 1}, EndDate: Date{2026, 8, 9}}}
	fixed.TimeZoneSource = TimeZoneProvided
	fixed.TimeZone = "Asia/Shanghai"
	fixed.CurrencyCode = "CNY"
	if !validReportDefinition(fixed) {
		t.Fatal("fixed report rejected")
	}
	invalid := []ReportDefinition{
		{},
		func() ReportDefinition {
			value := reportDefinition()
			value.Metrics = []Metric{"bad metric"}
			return value
		}(),
		func() ReportDefinition {
			value := reportDefinition()
			value.Metrics = []Metric{MetricClicks, MetricClicks}
			return value
		}(),
		func() ReportDefinition {
			value := reportDefinition()
			value.DateRange = DateRange{Relative: "FOREVER"}
			return value
		}(),
		func() ReportDefinition {
			value := reportDefinition()
			value.DateRange = DateRange{Fixed: &FixedDateRange{StartDate: Date{2026, 2, 30}, EndDate: Date{2026, 3, 1}}}
			return value
		}(),
		func() ReportDefinition { value := reportDefinition(); value.ReportType = "UNKNOWN"; return value }(),
		func() ReportDefinition {
			value := reportDefinition()
			value.TimeZoneSource = TimeZoneProvided
			value.TimeZone = "bad/zone"
			return value
		}(),
		func() ReportDefinition {
			value := reportDefinition()
			value.TimeZoneSource = TimeZoneUTC
			value.TimeZone = "UTC"
			return value
		}(),
	}
	for index, value := range invalid {
		if validReportDefinition(value) {
			t.Errorf("invalid report %d accepted: %#v", index, value)
		}
	}
	transportErr := &url.Error{Op: "Get", URL: "https://example.com?token=secret", Err: errors.New("dial failed")}
	if sanitizeCause(transportErr).Error() != "dial failed" || sanitizeCause(nil) != nil {
		t.Fatal("cause sanitization failed")
	}
}

func TestInvalidCallsDoNotReachNetwork(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requestCount++ }))
	defer server.Close()
	_, client := newStaticClient(t, server)
	ctx := context.Background()
	for _, invoke := range []func() error{
		func() error { _, err := client.GetCompany(ctx, "bad"); return err },
		func() error { _, err := client.ListCompanies(ctx, ListRequest{PageSize: 1001}); return err },
		func() error { _, err := client.GetAdUnit(ctx, "bad"); return err },
		func() error { _, err := client.ListAdUnits(ctx, ListRequest{Skip: -1}); return err },
		func() error { _, err := client.GetOrder(ctx, "bad"); return err },
		func() error { _, err := client.ListOrders(ctx, ListRequest{Filter: "bad\nfilter"}); return err },
		func() error { _, err := client.GetLineItem(ctx, "bad"); return err },
		func() error {
			_, err := client.ListLineItems(ctx, ListRequest{OrderBy: strings.Repeat("x", 1025)})
			return err
		},
		func() error { _, err := client.GetReport(ctx, "bad"); return err },
		func() error { _, err := client.ListReports(ctx, ListRequest{PageToken: " bad "}); return err },
		func() error { _, err := client.CreateHiddenReport(ctx, CreateReportRequest{}); return err },
		func() error { _, err := client.RunReport(ctx, "bad"); return err },
		func() error { _, err := client.GetReportOperation(ctx, "bad"); return err },
		func() error {
			_, err := client.FetchReportRows(ctx, FetchReportRowsRequest{ResultName: "bad"})
			return err
		},
		func() error {
			_, err := client.FetchReportRows(ctx, FetchReportRowsRequest{ResultName: resultName(), PageSize: 10001})
			return err
		},
	} {
		if err := invoke(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Errorf("validation error=%v", err)
		}
	}
	if requestCount != 0 {
		t.Fatalf("invalid calls made %d requests", requestCount)
	}
}

func TestOwnershipAndOperationContractFailures(t *testing.T) {
	tests := []struct {
		name     string
		response any
		invoke   func(*Client) error
		code     socialhub.ErrorCode
	}{
		{"network", Network{Name: "networks/999", NetworkCode: "999"}, func(client *Client) error { _, err := client.GetNetwork(context.Background()); return err }, socialhub.CodePermissionDenied},
		{"company", Company{Name: "networks/999/companies/1"}, func(client *Client) error {
			_, err := client.GetCompany(context.Background(), testCompanyID)
			return err
		}, socialhub.CodePermissionDenied},
		{"wrong company", Company{Name: resourceName("companies", "999")}, func(client *Client) error {
			_, err := client.GetCompany(context.Background(), testCompanyID)
			return err
		}, socialhub.CodePermissionDenied},
		{"ad unit parent", AdUnit{Name: resourceName("adUnits", testAdUnitID), ParentPath: []AdUnitParent{{ParentAdUnit: "networks/999/adUnits/1"}}}, func(client *Client) error {
			_, err := client.GetAdUnit(context.Background(), testAdUnitID)
			return err
		}, socialhub.CodePermissionDenied},
		{"order trafficker", Order{Name: resourceName("orders", testOrderID), Trafficker: "networks/999/users/1"}, func(client *Client) error {
			_, err := client.GetOrder(context.Background(), testOrderID)
			return err
		}, socialhub.CodePermissionDenied},
		{"wrong report", Report{Name: resourceName("reports", "999")}, func(client *Client) error {
			_, err := client.GetReport(context.Background(), testReportID)
			return err
		}, socialhub.CodePermissionDenied},
		{"report visibility", Report{Name: resourceName("reports", testReportID), Visibility: ReportVisible}, func(client *Client) error {
			_, err := client.CreateHiddenReport(context.Background(), CreateReportRequest{DisplayName: "Hidden report", Definition: reportDefinition()})
			return err
		}, socialhub.CodePlatformError},
		{"operation owner", map[string]any{"name": "networks/999/operations/reports/runs/1"}, func(client *Client) error { _, err := client.RunReport(context.Background(), testReportID); return err }, socialhub.CodePermissionDenied},
		{"operation report", map[string]any{"name": operationName(), "metadata": map[string]any{"report": resourceName("reports", "999")}}, func(client *Client) error {
			_, err := client.RunReport(context.Background(), testReportID)
			return err
		}, socialhub.CodePermissionDenied},
		{"operation result mismatch", map[string]any{"name": operationName(), "done": true, "metadata": map[string]any{"report": resourceName("reports", testReportID)}, "response": map[string]any{"reportResult": resourceName("reports", "999") + "/results/1"}}, func(client *Client) error {
			_, err := client.GetReportOperation(context.Background(), operationName())
			return err
		}, socialhub.CodePermissionDenied},
		{"operation result", map[string]any{"name": operationName(), "done": true}, func(client *Client) error {
			_, err := client.GetReportOperation(context.Background(), operationName())
			return err
		}, socialhub.CodePlatformError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writeJSON(t, writer, http.StatusOK, test.response) }))
			defer server.Close()
			_, client := newStaticClient(t, server)
			if err := test.invoke(client); requireHubError(t, err).Code != test.code {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

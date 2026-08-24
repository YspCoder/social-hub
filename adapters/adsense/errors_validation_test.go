package adsense

import (
	"context"
	"encoding/json"
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
	body := []byte(`{"error":{"code":429,"message":"access_token=secret-value exhausted","status":"RESOURCE_EXHAUSTED","details":[{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"RATE_LIMIT_EXCEEDED","domain":"adsense.googleapis.com"}]}}`)
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

func TestErrorClassificationAndQuotaHelpers(t *testing.T) {
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
	if policy.RequestsPerMinutePerUserProject != 100 || policy.RequestsPerMinutePerProject != 500 || policy.RequestsPerDay != 10_000 ||
		policy.MaximumListPageSize != 10_000 || policy.MaximumJSONReportRows != 100_000 || policy.MaximumCSVReportRows != 1_000_000 {
		t.Fatalf("policy=%#v", policy)
	}
	if parseRetryAfter("1.5") != 1500*time.Millisecond || parseRetryAfter("bad") != 0 || firstNonEmpty("", "value") != "value" ||
		redactSensitive("client_secret: topsecret") == "client_secret: topsecret" {
		t.Fatal("error helpers failed")
	}
	if !errors.Is(ownershipError("test", "site"), socialhub.ErrPermissionDenied) || requireHubError(t, platformContractError("test", "bad")).Code != socialhub.CodePlatformError {
		t.Fatal("typed errors failed")
	}
}

func TestValidationPrimitivesAndReports(t *testing.T) {
	client := &Client{publisherID: testPublisherID}
	if !validPublisherID(testPublisherID) || validPublisherID("pub-x") || !validResourceID("a-_.~:1") || validResourceID("bad/id") ||
		!validAccountName(accountName()) || validAccountName("accounts/123") || !validAdClientName(adClientName()) || validAdClientName("accounts/pub-1/sites/a") ||
		!client.ownsAdClient(adClientName()) || !client.ownsNested(nestedName("adunits", testAdUnitID), "adunits") ||
		client.ownsNested(nestedName("adunits", testAdUnitID)+"/extra", "adunits") || !validLanguageCode("zh-CN") || validLanguageCode("bad code") ||
		!validCurrency("USD") || validCurrency("usd") || !validEnumName("TOTAL_EARNINGS") || validEnumName("bad value") {
		t.Fatal("validation primitives failed")
	}
	if _, err := client.resourceName("test", accountName(), "sites", "bad/id"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("resource error=%v", err)
	}
	valid := GenerateReportRequest{
		Dimensions: []Dimension{DimensionDate}, Metrics: []Metric{MetricClicks}, DateRange: ReportDateCustom,
		StartDate: validDateFixture(), EndDate: validDateFixture(), Limit: 100_000,
	}
	if !validGenerateReport(valid) {
		t.Fatal("valid report rejected")
	}
	invalid := []GenerateReportRequest{
		{},
		func() GenerateReportRequest { value := valid; value.Metrics = []Metric{"bad metric"}; return value }(),
		func() GenerateReportRequest {
			value := valid
			value.Metrics = []Metric{MetricClicks, MetricClicks}
			return value
		}(),
		func() GenerateReportRequest { value := valid; value.DateRange = "FOREVER"; return value }(),
		func() GenerateReportRequest {
			value := valid
			value.StartDate = Date{Year: 2026, Month: 2, Day: 30}
			return value
		}(),
		func() GenerateReportRequest { value := valid; value.DateRange = ReportDateYesterday; return value }(),
		func() GenerateReportRequest { value := valid; value.ReportingTimeZone = "LOCAL"; return value }(),
		func() GenerateReportRequest { value := valid; value.CurrencyCode = "usd"; return value }(),
		func() GenerateReportRequest { value := valid; value.LanguageCode = "bad code"; return value }(),
		func() GenerateReportRequest { value := valid; value.Filters = []string{"bad\nfilter"}; return value }(),
		func() GenerateReportRequest { value := valid; value.Limit = 100_001; return value }(),
	}
	for index, value := range invalid {
		if validGenerateReport(value) {
			t.Errorf("invalid report %d accepted: %#v", index, value)
		}
	}
	if !validGenerateSavedReport(GenerateSavedReportRequest{DateRange: ReportDateYesterday}) ||
		validGenerateSavedReport(GenerateSavedReportRequest{}) || validGenerateSavedReport(GenerateSavedReportRequest{DateRange: ReportDateYesterday, StartDate: validDateFixture()}) {
		t.Fatal("saved report validation failed")
	}
	transportErr := &url.Error{Op: "Get", URL: "https://example.com?token=secret", Err: errors.New("dial failed")}
	if sanitizeCause(transportErr).Error() != "dial failed" || sanitizeCause(nil) != nil {
		t.Fatal("cause sanitization failed")
	}
	var result ReportResult
	if json.Unmarshal([]byte(`{"totalMatchedRows":"not-a-number"}`), &result) == nil {
		t.Fatal("invalid matched row count accepted")
	}
}

func TestInvalidCallsDoNotReachNetwork(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requestCount++ }))
	defer server.Close()
	_, client := newStaticClient(t, server)
	ctx := context.Background()
	badList := ListRequest{PageSize: 10_001}
	for _, invoke := range []func() error{
		func() error { _, err := client.ListChildAccounts(ctx, badList); return err },
		func() error { _, err := client.GetAdClient(ctx, "bad/id"); return err },
		func() error { _, err := client.ListAdClients(ctx, badList); return err },
		func() error { _, err := client.GetAdClientAdCode(ctx, "bad/id"); return err },
		func() error { _, err := client.GetAdUnit(ctx, "bad/id", "unit"); return err },
		func() error { _, err := client.ListAdUnits(ctx, testAdClientID, badList); return err },
		func() error { _, err := client.GetAdUnitAdCode(ctx, testAdClientID, "bad/id"); return err },
		func() error {
			_, err := client.ListAdUnitCustomChannels(ctx, testAdClientID, "bad/id", ListRequest{})
			return err
		},
		func() error { _, err := client.GetCustomChannel(ctx, testAdClientID, "bad/id"); return err },
		func() error { _, err := client.ListCustomChannels(ctx, "bad/id", ListRequest{}); return err },
		func() error {
			_, err := client.ListCustomChannelAdUnits(ctx, testAdClientID, testChannelID, badList)
			return err
		},
		func() error { _, err := client.GetURLChannel(ctx, testAdClientID, "bad/id"); return err },
		func() error { _, err := client.ListURLChannels(ctx, testAdClientID, badList); return err },
		func() error { _, err := client.GetSite(ctx, "bad/id"); return err },
		func() error { _, err := client.ListSites(ctx, badList); return err },
		func() error { _, err := client.ListAlerts(ctx, "bad code"); return err },
		func() error { _, err := client.GetPolicyIssue(ctx, "bad/id"); return err },
		func() error { _, err := client.ListPolicyIssues(ctx, badList); return err },
		func() error { _, err := client.GenerateReport(ctx, GenerateReportRequest{}); return err },
		func() error { _, err := client.GetSavedReport(ctx, "bad/id"); return err },
		func() error { _, err := client.ListSavedReports(ctx, badList); return err },
		func() error {
			_, err := client.GenerateSavedReport(ctx, testReportID, GenerateSavedReportRequest{})
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

func TestOwnershipAndPlatformContractFailures(t *testing.T) {
	tests := []struct {
		name     string
		response any
		invoke   func(*Client) error
		code     socialhub.ErrorCode
	}{
		{"account", Account{Name: "accounts/pub-999"}, func(client *Client) error { _, err := client.GetAccount(context.Background()); return err }, socialhub.CodePermissionDenied},
		{"children", map[string]any{"accounts": []Account{{Name: accountName()}}}, func(client *Client) error {
			_, err := client.ListChildAccounts(context.Background(), ListRequest{})
			return err
		}, socialhub.CodePlatformError},
		{"ad client", AdClient{Name: "accounts/pub-999/adclients/ca-pub-999"}, func(client *Client) error {
			_, err := client.GetAdClient(context.Background(), testAdClientID)
			return err
		}, socialhub.CodePermissionDenied},
		{"ad unit", AdUnit{Name: nestedName("adunits", "wrong")}, func(client *Client) error {
			_, err := client.GetAdUnit(context.Background(), testAdClientID, testAdUnitID)
			return err
		}, socialhub.CodePermissionDenied},
		{"custom channel", CustomChannel{Name: nestedName("customchannels", "wrong")}, func(client *Client) error {
			_, err := client.GetCustomChannel(context.Background(), testAdClientID, testChannelID)
			return err
		}, socialhub.CodePermissionDenied},
		{"URL channel", URLChannel{Name: nestedName("urlchannels", "wrong")}, func(client *Client) error {
			_, err := client.GetURLChannel(context.Background(), testAdClientID, testURLChannelID)
			return err
		}, socialhub.CodePermissionDenied},
		{"site", Site{Name: "accounts/pub-999/sites/example.com"}, func(client *Client) error { _, err := client.GetSite(context.Background(), testSiteID); return err }, socialhub.CodePermissionDenied},
		{"policy count", func() PolicyIssue { value := policyIssueResource(); value.AdRequestCount = "-1"; return value }(), func(client *Client) error {
			_, err := client.GetPolicyIssue(context.Background(), testIssueID)
			return err
		}, socialhub.CodePlatformError},
		{"saved report", SavedReport{Name: "accounts/pub-999/reports/report-1"}, func(client *Client) error {
			_, err := client.GetSavedReport(context.Background(), testReportID)
			return err
		}, socialhub.CodePermissionDenied},
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

func TestMalformedReportsAreRejected(t *testing.T) {
	validInput := GenerateReportRequest{Dimensions: []Dimension{DimensionDate}, Metrics: []Metric{MetricClicks}, DateRange: ReportDateYesterday}
	base := reportResult([]map[string]any{{"name": "DATE", "type": "DIMENSION"}, {"name": "CLICKS", "type": "METRIC_TALLY"}})
	tests := []func(map[string]any){
		func(value map[string]any) { delete(value, "totalMatchedRows") },
		func(value map[string]any) { value["totalMatchedRows"] = "-1" },
		func(value map[string]any) {
			value["headers"] = []map[string]any{{"name": "CLICKS", "type": "METRIC_TALLY"}, {"name": "DATE", "type": "DIMENSION"}}
		},
		func(value map[string]any) {
			value["headers"] = []map[string]any{{"name": "DATE", "type": "METRIC_TALLY"}, {"name": "CLICKS", "type": "METRIC_TALLY"}}
		},
		func(value map[string]any) {
			value["rows"] = []any{map[string]any{"cells": []any{map[string]string{"value": "1"}}}}
		},
		func(value map[string]any) { value["startDate"] = Date{Year: 2026, Month: 2, Day: 30} },
	}
	for index, mutate := range tests {
		encoded, _ := json.Marshal(base)
		var response map[string]any
		_ = json.Unmarshal(encoded, &response)
		mutate(response)
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writeJSON(t, writer, http.StatusOK, response) }))
		_, client := newStaticClient(t, server)
		_, err := client.GenerateReport(context.Background(), validInput)
		server.Close()
		if requireHubError(t, err).Code != socialhub.CodePlatformError {
			t.Errorf("case %d error=%v", index, err)
		}
	}
}

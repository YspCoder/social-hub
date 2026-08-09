package cm360

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
	body := []byte(`{"error":{"code":429,"message":"access_token=secret-value exhausted","errors":[{"domain":"usageLimits","reason":"userRateLimitExceeded","message":"slow down"}]}}`)
	header := http.Header{"X-Goog-Request-Id": {"request-1"}, "Retry-After": {"2.5"}}
	err := decodeHTTPError(http.StatusTooManyRequests, header, body)
	var api *APIError
	if !errors.As(err, &api) || !api.Retryable() || api.Hub.Code != socialhub.CodeRateLimited ||
		api.Hub.RetryAfter != 2500*time.Millisecond || api.Hub.RequestID != "request-1" ||
		strings.Contains(api.Hub.PlatformMessage, "secret-value") || api.Hub.PlatformCode != "userRateLimitExceeded" {
		t.Fatalf("error=%#v", api)
	}
	if !errors.Is(err, socialhub.ErrRateLimited) || api.Error() == "" || api.Unwrap() == nil {
		t.Fatalf("wrapped error=%v", err)
	}
	if (&APIError{}).Error() == "" || (*APIError)(nil).Error() == "" || (*APIError)(nil).Unwrap() != nil || (*APIError)(nil).Retryable() {
		t.Fatal("nil API error contract failed")
	}
}

func TestErrorClassificationQuotaAndHelpers(t *testing.T) {
	tests := []struct {
		status int
		rpc    string
		reason string
		code   socialhub.ErrorCode
		class  socialhub.ErrorClass
	}{
		{403, "", "dailyLimitExceeded", socialhub.CodeRateLimited, socialhub.ClassUserAction},
		{403, "", "quotaExceeded", socialhub.CodeRateLimited, socialhub.ClassRetryable},
		{401, "", "authError", socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{403, "", "insufficientPermissions", socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{404, "", "notFound", socialhub.CodeNotFound, socialhub.ClassPermanent},
		{503, "", "backendError", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{400, "", "invalidParameter", socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
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
	if policy.ProjectRequestsPerDay != 50000 || policy.ProjectQueriesPerSecond != 1 ||
		policy.ProjectMaximumQueriesPerSecond != 10 || policy.ReportDataRequestsPerMinute != 120 ||
		policy.ReportDataRequestsPerDay != 10000 || policy.ReportDataTimeoutSeconds != 60 ||
		policy.RecommendedWriteConcurrency != 1 {
		t.Fatalf("policy=%#v", policy)
	}
	if parseRetryAfter("1.5") != 1500*time.Millisecond || parseRetryAfter("bad") != 0 ||
		firstNonEmpty("", "value") != "value" || redactSensitive("client_secret: topsecret") == "client_secret: topsecret" {
		t.Fatal("error helpers failed")
	}
	if !errors.Is(ownershipError("test", "campaign"), socialhub.ErrPermissionDenied) ||
		requireHubError(t, platformContractError("test", "bad")).Code != socialhub.CodePlatformError {
		t.Fatal("typed errors failed")
	}
}

func TestValidationAndReportQueryPreparation(t *testing.T) {
	if !validID("123") || validID("0") || validID("01x") || validID(strings.Repeat("1", 21)) ||
		!validName("Campaign", 10) || validName(" name ", 10) || validText("line\nbreak", 20, false) ||
		!validAbsoluteDateRange("2026-08-01", "2026-08-09") || validAbsoluteDateRange("2026-08-10", "2026-08-09") ||
		!validRelativeDateRange("LAST_7_DAYS") || validRelativeDateRange("FOREVER") ||
		!validCMField("impressions") || validCMField("other:field") || validCMField("bad field") || validCMField("1metric") ||
		!validIDs([]string{"1", "2"}, 2) || validIDs([]string{"1", "1"}, 2) ||
		validPlacementStatus("bad") || validAdType("bad") || validReportFileStatus("bad") {
		t.Fatal("validation primitives failed")
	}
	prepared, err := prepareReportDataQuery(validReportQuery(), testAdvertiserID)
	if err != nil || len(prepared.DimensionFilters) != 1 || prepared.DimensionFilters[0].ID != testAdvertiserID {
		t.Fatalf("prepared=%#v err=%v", prepared, err)
	}
	query := validReportQuery()
	query.DateRange = DateRange{RelativeDateRange: "LAST_7_DAYS"}
	query.DimensionFilters = []DimensionValue{{DimensionName: "advertiser", ID: testAdvertiserID}}
	query.SortBys = []SortBy{{Name: "impressions", SortOrder: SortDescending}}
	if _, err := prepareReportDataQuery(query, testAdvertiserID); err != nil {
		t.Fatalf("relative query error=%v", err)
	}
	query.DimensionFilters[0].ID = "999"
	if _, err := prepareReportDataQuery(query, testAdvertiserID); !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("ownership query error=%v", err)
	}
	invalid := []ReportDataQueryRequest{
		{},
		{DateRange: DateRange{StartDate: "bad", EndDate: "bad"}, MetricNames: []string{"impressions"}},
		{DateRange: DateRange{RelativeDateRange: "TODAY", StartDate: "2026-08-01"}, MetricNames: []string{"impressions"}},
		{DateRange: DateRange{RelativeDateRange: "TODAY"}, MetricNames: []string{"bad field"}},
		{DateRange: DateRange{RelativeDateRange: "TODAY"}, MetricNames: []string{"impressions", "impressions"}},
		{DateRange: DateRange{RelativeDateRange: "TODAY"}, MetricNames: []string{"impressions"}, MaxResults: 1001},
		{DateRange: DateRange{RelativeDateRange: "TODAY"}, MetricNames: []string{"impressions"}, SortBys: []SortBy{{Name: "clicks", SortOrder: SortAscending}}},
		{DateRange: DateRange{RelativeDateRange: "TODAY"}, MetricNames: []string{"impressions"}, DimensionFilters: []DimensionValue{{DimensionName: "bad field"}}},
	}
	for _, input := range invalid {
		if _, err := prepareReportDataQuery(input, testAdvertiserID); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Errorf("input=%#v error=%v", input, err)
		}
	}
	transportErr := &url.Error{Op: "Get", URL: "https://example.com?token=secret", Err: errors.New("dial failed")}
	if sanitizeCause(transportErr).Error() != "dial failed" || sanitizeCause(nil) != nil {
		t.Fatal("cause sanitization failed")
	}
}

func TestInvalidWorkflowCallsDoNotReachNetwork(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requestCount++ }))
	defer server.Close()
	_, client := newStaticClient(t, server)
	ctx := context.Background()
	for _, invoke := range []func() error{
		func() error { _, err := client.GetCampaign(ctx, "bad"); return err },
		func() error { _, err := client.ListCampaigns(ctx, CampaignListRequest{MaxResults: 1001}); return err },
		func() error { _, err := client.CreateCampaign(ctx, CreateCampaignRequest{}); return err },
		func() error {
			_, err := client.UpdateCampaign(ctx, testCampaignID, UpdateCampaignRequest{})
			return err
		},
		func() error { _, err := client.GetPlacement(ctx, "bad"); return err },
		func() error {
			_, err := client.ListPlacements(ctx, PlacementListRequest{ActiveStatus: "bad"})
			return err
		},
		func() error { _, err := client.GetAd(ctx, "bad"); return err },
		func() error { _, err := client.ListAds(ctx, AdListRequest{Type: "bad"}); return err },
		func() error { _, err := client.GetReport(ctx, "bad"); return err },
		func() error { _, err := client.ListReports(ctx, ReportListRequest{Scope: "bad"}); return err },
		func() error { _, err := client.RunReport(ctx, "bad", false); return err },
		func() error { _, err := client.GetReportFile(ctx, "bad", "bad"); return err },
		func() error { _, err := client.ListReportFiles(ctx, "bad", ReportFileListRequest{}); return err },
		func() error {
			_, err := client.DownloadReportFileRange(ctx, testReportID, testFileID, ByteRange{Start: 2, End: 1}, nil)
			return err
		},
	} {
		if err := invoke(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("validation error=%v", err)
		}
	}
	if requestCount != 0 {
		t.Fatalf("invalid calls made %d requests", requestCount)
	}
}

package analyticsdata

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestErrorClassificationMatrix(t *testing.T) {
	tests := []struct {
		status int
		api    string
		reason string
		code   socialhub.ErrorCode
		class  socialhub.ErrorClass
	}{
		{400, "", "quotaExceeded", socialhub.CodeRateLimited, socialhub.ClassRetryable},
		{403, "", "dailyLimitExceeded", socialhub.CodeRateLimited, socialhub.ClassUserAction},
		{401, "", "authError", socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{403, "", "insufficientPermissions", socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{404, "", "notFound", socialhub.CodeNotFound, socialhub.ClassPermanent},
		{500, "", "backendError", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{400, "", "invalidArgument", socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{409, "ALREADY_EXISTS", "", socialhub.CodeConflict, socialhub.ClassPermanent},
		{429, "RESOURCE_EXHAUSTED", "", socialhub.CodeRateLimited, socialhub.ClassRetryable},
		{503, "UNAVAILABLE", "", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{410, "", "", socialhub.CodeNotFound, socialhub.ClassPermanent},
		{418, "", "", socialhub.CodePlatformError, socialhub.ClassPermanent},
	}
	for _, test := range tests {
		code, class := classifyError(test.status, test.api, test.reason)
		if code != test.code || class != test.class {
			t.Errorf("status=%d api=%q reason=%q got=%s/%s", test.status, test.api, test.reason, code, class)
		}
	}
}

func TestErrorHelpers(t *testing.T) {
	nilAPI := (*APIError)(nil)
	if nilAPI.Error() == "" || nilAPI.Unwrap() != nil || nilAPI.Retryable() {
		t.Fatal("nil API error contract failed")
	}
	hub := &socialhub.Error{Code: socialhub.CodeRateLimited, Class: socialhub.ClassRetryable}
	api := &APIError{Hub: hub}
	if api.Error() != hub.Error() || api.Unwrap() != hub || !api.Retryable() {
		t.Fatal("API error wrapping failed")
	}
	if parseRetryAfter("1.5") != 1500*time.Millisecond || parseRetryAfter("-1") != 0 || parseRetryAfter("invalid") != 0 {
		t.Fatal("numeric Retry-After parsing failed")
	}
	future := time.Now().Add(5 * time.Minute).UTC().Format(http.TimeFormat)
	if delay := parseRetryAfter(future); delay < 4*time.Minute || delay > 6*time.Minute {
		t.Fatalf("HTTP-date Retry-After=%s", delay)
	}
	secret := "client_secret=visible access_token: token authorization=Bearer-value"
	redacted := redactSensitive(secret)
	if strings.Contains(redacted, "visible") || strings.Contains(redacted, "Bearer-value") || !strings.Contains(redacted, "[REDACTED]") {
		t.Fatalf("redacted=%q", redacted)
	}
	if boundedMessage("短文本", 10) != "短文本" || boundedMessage(strings.Repeat("x", 30), 20) != strings.Repeat("x", 20) ||
		firstNonEmpty("", " value ", "later") != " value " {
		t.Fatal("bounded or first non-empty helper failed")
	}
	cause := errors.New("network")
	if sanitizeCause(cause) != cause || sanitizeCause(nil) != nil {
		t.Fatal("cause sanitization failed")
	}
}

func TestResponseValidationBranches(t *testing.T) {
	response := &ReportResponse{
		DimensionHeaders: []DimensionHeader{{Name: "country"}}, MetricHeaders: []MetricHeader{{Name: "eventCount", Type: MetricTypeInteger}},
		Rows: []Row{{DimensionValues: []DimensionValue{{Value: "US"}}, MetricValues: []MetricValue{{Value: "1"}}}}, RowCount: 1,
		Metadata:      &ResponseMetadata{SamplingMetadatas: []SamplingMetadata{{SamplesReadCount: 5, SamplingSpaceSize: 10}}},
		PropertyQuota: &PropertyQuota{ConcurrentRequests: &QuotaStatus{Consumed: 1, Remaining: 9}}, Kind: "analyticsData#runReport",
	}
	if !validReportResponse(response, []string{"country"}, []string{"eventCount"}, 10, "analyticsData#runReport") {
		t.Fatal("valid response rejected")
	}
	response.Metadata.SamplingMetadatas[0].SamplesReadCount = 11
	if validReportResponse(response, []string{"country"}, []string{"eventCount"}, 10, "analyticsData#runReport") {
		t.Fatal("invalid sampling accepted")
	}
	response.Metadata = nil
	response.PropertyQuota.ConcurrentRequests.Remaining = -1
	if validReportResponse(response, []string{"country"}, []string{"eventCount"}, 10, "analyticsData#runReport") {
		t.Fatal("negative quota accepted")
	}
	if validMetricType("UNKNOWN") || !validMetricType(MetricTypeCurrency) {
		t.Fatal("metric type validation failed")
	}
	pivotInput := pivotRequest()
	pivotInput.Dimensions = append(pivotInput.Dimensions, Dimension{Name: "filterOnly"})
	pivotResponse := &PivotReportResponse{
		PivotHeaders: []PivotHeader{
			{PivotDimensionHeaders: []PivotDimensionHeader{{DimensionValues: []DimensionValue{{Value: "US"}}}}, RowCount: 1},
			{PivotDimensionHeaders: []PivotDimensionHeader{{DimensionValues: []DimensionValue{{Value: "page_view"}}}}, RowCount: 1},
		},
		DimensionHeaders: []DimensionHeader{{Name: "country"}, {Name: "eventName"}},
		MetricHeaders:    []MetricHeader{{Name: "eventCount", Type: MetricTypeInteger}},
		Rows: []Row{{
			DimensionValues: []DimensionValue{{Value: "US"}, {Value: "page_view"}},
			MetricValues:    []MetricValue{{Value: "1"}},
		}},
		Kind: "analyticsData#runPivotReport",
	}
	if !validPivotResponse(pivotResponse, pivotInput) {
		t.Fatal("pivot response with filter-only request dimension rejected")
	}
	pivotInput.DateRanges = append(pivotInput.DateRanges, DateRange{StartDate: "2026-07-01", EndDate: "2026-07-31"})
	if !validPivotResponse(pivotResponse, pivotInput) {
		t.Fatal("pivot response gained an unrequested dateRange column")
	}
}

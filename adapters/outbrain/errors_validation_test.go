package outbrain

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestDecodeHTTPErrorMappingMetadataAndRedaction(t *testing.T) {
	tests := []struct {
		status int
		code   socialhub.ErrorCode
		class  socialhub.ErrorClass
	}{
		{http.StatusBadRequest, socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{http.StatusUnauthorized, socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{http.StatusForbidden, socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{http.StatusNotFound, socialhub.CodeNotFound, socialhub.ClassPermanent},
		{http.StatusConflict, socialhub.CodeConflict, socialhub.ClassPermanent},
		{http.StatusTooManyRequests, socialhub.CodeRateLimited, socialhub.ClassRetryable},
		{http.StatusServiceUnavailable, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{http.StatusTeapot, socialhub.CodePlatformError, socialhub.ClassPermanent},
	}
	for _, test := range tests {
		header := http.Header{"AMPLIFY-REQUEST-ID": {"request-123"}, "rate-limit-msec-left": {"1500"}}
		body := []byte(`{"moreInfo":"get-budget","errorMessage":"password=hunter2 OB-TOKEN-V1: top-secret"}`)
		err := decodeHTTPError(test.status, header, body)
		hub := hubError(t, err)
		if hub.Code != test.code || hub.Class != test.class || hub.RequestID != "request-123" || hub.RetryAfter != 1500*time.Millisecond {
			t.Fatalf("status=%d hub=%#v", test.status, hub)
		}
		if strings.Contains(hub.PlatformMessage, "hunter2") || strings.Contains(hub.PlatformMessage, "top-secret") || !strings.Contains(hub.PlatformMessage, "[REDACTED]") {
			t.Fatalf("message=%q", hub.PlatformMessage)
		}
		var api *APIError
		if !errors.As(err, &api) || api.MoreInfo != "get-budget" || api.ErrorMessage == "" || (test.class == socialhub.ClassRetryable) != api.Retryable() {
			t.Fatalf("api=%#v err=%v", api, err)
		}
	}
}

func TestErrorHelpersAndValidation(t *testing.T) {
	if got := parseRateLimitDelay(http.Header{"Retry-After": {"2.5"}}); got != 2500*time.Millisecond {
		t.Fatalf("Retry-After=%v", got)
	}
	if got := parseRateLimitDelay(http.Header{"rate-limit-msec-left": {"bad"}}); got != 0 {
		t.Fatalf("bad delay=%v", got)
	}
	if len([]rune(boundedMessage(strings.Repeat("界", 600), 512))) != 512 {
		t.Fatal("boundedMessage did not preserve rune boundary")
	}
	if !errors.Is(invalidArgument("test", "bad"), socialhub.ErrInvalidArgument) || !errors.Is(platformError("test", socialhub.CodeConflict, socialhub.ClassPermanent, nil), socialhub.ErrConflict) {
		t.Fatal("common error mapping mismatch")
	}
	if (&APIError{}).Error() == "" || (&APIError{}).Unwrap() != nil || (&APIError{}).Retryable() {
		t.Fatal("nil APIError helpers mismatch")
	}
	if validPathID("bad/id") || !validPathID(testCampaignID) || validDestinationURL("file:///etc/passwd") || !validDestinationURL("https://example.test") {
		t.Fatal("identifier or URL validation mismatch")
	}
	if validTimezone("UTC") || !validTimezone("GMT+12:00") || validDateWindow("2026-08-09", "2026-08-01") {
		t.Fatal("timezone or date validation mismatch")
	}
	if validReportSort("unknown", campaignReportSortFields) || !validReportSort("-ctr", campaignReportSortFields) || validBreakdown("hourOfDay", false) || !validBreakdown("hourOfDay", true) {
		t.Fatal("report enum validation mismatch")
	}
	var metrics Metrics
	if err := json.Unmarshal([]byte(`{"clicks":3,"unknown":"kept"}`), &metrics); err != nil || metrics.Clicks != 3 || !strings.Contains(string(metrics.Raw), "unknown") {
		t.Fatalf("metrics=%#v err=%v", metrics, err)
	}
	if !promotedLinkFixture(false, true).Approved() || promotedLinkFixture(false, false).Approved() {
		t.Fatal("approval normalization mismatch")
	}
}

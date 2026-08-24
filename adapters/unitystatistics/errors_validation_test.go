package unitystatistics

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestCurrentMetricAndBreakdownContract(t *testing.T) {
	base := []AcquisitionMetric{MetricStarts, MetricViews, MetricClicks, MetricInstalls, MetricSpend, MetricCPI, MetricCTR, MetricCVR, MetricECPM}
	for _, metric := range base {
		if !validAcquisitionMetric(metric) {
			t.Errorf("base metric rejected: %s", metric)
		}
	}
	days := []int{0, 1, 3, 7, 14, 21, 28}
	suffixes := []string{
		"AdRevenue", "AdRevenueRoas", "IapRevenue", "IapRoas", "Purchases", "UniquePurchasers", "Retained",
		"RetentionRate", "TotalRoas", "LevelComplete", "CostPerLevelComplete", "LevelCompleteRate", "Payer", "PayerRate", "CostPerPayer",
	}
	count := len(base)
	for _, day := range days {
		for _, suffix := range suffixes {
			metric := AcquisitionMetric(fmt.Sprintf("d%d%s", day, suffix))
			if !validAcquisitionMetric(metric) {
				t.Errorf("post-install metric rejected: %s", metric)
			}
			count++
		}
	}
	if count != 114 || validAcquisitionMetric("d2Payer") || validAcquisitionMetric("d0Unknown") || validAcquisitionMetric("unknown") {
		t.Fatalf("metric count=%d", count)
	}
	for _, metric := range []SKANMetric{SKANMetricStarts, SKANMetricViews, SKANMetricClicks, SKANMetricInstalls, SKANMetricSpend, SKANMetricCPI, SKANMetricCVR} {
		if !validSKANMetric(metric) {
			t.Errorf("SKAN metric rejected: %s", metric)
		}
	}
	for _, breakdown := range []AcquisitionBreakdown{
		BreakdownApp, BreakdownCampaign, BreakdownCountry, BreakdownCreativePack, BreakdownCreativePackType,
		BreakdownOSVersion, BreakdownPlatform, BreakdownSourceAppID, BreakdownStore, BreakdownTargetGame, BreakdownEventType, BreakdownEventName,
	} {
		if !validAcquisitionBreakdown(breakdown) {
			t.Errorf("breakdown rejected: %s", breakdown)
		}
	}
	for _, breakdown := range []SKANBreakdown{SKANBreakdownApp, SKANBreakdownCampaign, SKANBreakdownConversionValue, SKANBreakdownTargetGame} {
		if !validSKANBreakdown(breakdown) {
			t.Errorf("SKAN breakdown rejected: %s", breakdown)
		}
	}
	if validSKANMetric("ctr") || validAcquisitionBreakdown("bad") || validSKANBreakdown("country") {
		t.Fatal("unsupported report enum accepted")
	}
}

func TestRequestPreparationAndValidation(t *testing.T) {
	input := acquisitionRequest(true)
	input.Format = ""
	input.Metrics = []AcquisitionMetric{MetricClicks, MetricD21PayerRate}
	input.Breakdowns = []AcquisitionBreakdown{BreakdownApp}
	input.AppIDs = []string{"app-1"}
	query, format, err := prepareAcquisitionsRequest(input)
	if err != nil || format != FormatCSV || query.Get("format") != "csv" || query.Get("eofMarker") != "true" ||
		query.Get("metrics") != "clicks,d21PayerRate" || query.Get("breakdowns") != "app" || query.Get("appIds") != "app-1" {
		t.Fatalf("format=%s query=%v err=%v", format, query, err)
	}
	start := input.Start
	validSKAN := SKANReportRequest{
		Start: start, End: start.Add(time.Hour), Scale: ScaleHour, Metrics: []SKANMetric{SKANMetricInstalls},
		Breakdowns: []SKANBreakdown{SKANBreakdownConversionValue}, Format: FormatJSON,
	}
	query, format, err = prepareSKANRequest(validSKAN)
	if err != nil || format != FormatJSON || query.Get("breakdowns") != "conversionValue" {
		t.Fatalf("format=%s query=%v err=%v", format, query, err)
	}

	invalidAcquisitions := []AcquisitionsReportRequest{
		{},
		{Start: start, End: start, Scale: ScaleDay, Metrics: []AcquisitionMetric{MetricClicks}},
		{Start: start, End: start.Add(time.Hour), Scale: "bad", Metrics: []AcquisitionMetric{MetricClicks}},
		{Start: start, End: start.Add(time.Hour), Scale: ScaleHour, Format: "xml", Metrics: []AcquisitionMetric{MetricClicks}},
		{Start: start, End: start.Add(time.Hour), Scale: ScaleHour, Format: FormatJSON, EOFMarker: true, Metrics: []AcquisitionMetric{MetricClicks}},
		{Start: start, End: start.Add(time.Hour), Scale: ScaleHour},
		{Start: start, End: start.Add(time.Hour), Scale: ScaleHour, Metrics: []AcquisitionMetric{"bad"}},
		{Start: start, End: start.Add(time.Hour), Scale: ScaleHour, Metrics: []AcquisitionMetric{MetricClicks, MetricClicks}},
		{Start: start, End: start.Add(time.Hour), Scale: ScaleHour, Metrics: []AcquisitionMetric{MetricClicks}, Breakdowns: []AcquisitionBreakdown{"bad"}},
		{Start: start, End: start.Add(time.Hour), Scale: ScaleHour, Metrics: []AcquisitionMetric{MetricClicks}, AppIDs: []string{"bad,id"}},
		{Start: start, End: start.Add(time.Hour), Scale: ScaleHour, Metrics: []AcquisitionMetric{MetricClicks}, CampaignIDs: []string{"duplicate", "duplicate"}},
		{Start: start, End: start.Add(time.Hour), Scale: ScaleHour, Metrics: []AcquisitionMetric{MetricClicks}, CreativePackTypes: []CreativePackType{"bad"}},
		{Start: start, End: start.Add(time.Hour), Scale: ScaleHour, Metrics: []AcquisitionMetric{MetricClicks}, Countries: []CountryCode{"ZZ"}},
		{Start: start, End: start.Add(time.Hour), Scale: ScaleHour, Metrics: []AcquisitionMetric{MetricClicks}, Platforms: []Platform{"windows"}},
	}
	for index, request := range invalidAcquisitions {
		if _, _, err := prepareAcquisitionsRequest(request); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Errorf("invalid acquisition %d accepted: %#v err=%v", index, request, err)
		}
	}
	invalidSKAN := []SKANReportRequest{
		{Start: start, End: start.Add(time.Hour), Scale: ScaleHour, Metrics: []SKANMetric{"bad"}},
		{Start: start, End: start.Add(time.Hour), Scale: ScaleHour, Metrics: []SKANMetric{SKANMetricClicks}, Breakdowns: []SKANBreakdown{"bad"}},
		{Start: start, End: start.Add(time.Hour), Scale: ScaleHour, Metrics: []SKANMetric{SKANMetricClicks}, Format: FormatJSON, EOFMarker: true},
	}
	for index, request := range invalidSKAN {
		if _, _, err := prepareSKANRequest(request); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Errorf("invalid SKAN %d accepted: %#v err=%v", index, request, err)
		}
	}
}

func TestInvalidDownloadsDoNotReachNetwork(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	valid := acquisitionRequest(false)
	for _, invoke := range []func() error{
		func() error {
			_, err := client.DownloadAcquisitionsReport(context.Background(), AcquisitionsReportRequest{}, io.Discard, DownloadOptions{})
			return err
		},
		func() error {
			_, err := client.DownloadAcquisitionsReport(context.Background(), valid, nil, DownloadOptions{})
			return err
		},
		func() error {
			_, err := client.DownloadAcquisitionsReport(context.Background(), valid, io.Discard, DownloadOptions{MaxBytes: -1})
			return err
		},
		func() error {
			_, err := client.DownloadAcquisitionsReport(context.Background(), valid, io.Discard, DownloadOptions{Compression: "brotli"})
			return err
		},
		func() error {
			_, err := client.DownloadSKANReport(context.Background(), SKANReportRequest{}, io.Discard, DownloadOptions{})
			return err
		},
	} {
		if err := invoke(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Errorf("error=%v", err)
		}
	}
	if calls.Load() != 0 {
		t.Fatalf("network calls=%d", calls.Load())
	}
}

func TestErrorClassificationQuotaAndHelpers(t *testing.T) {
	tests := []struct {
		status int
		code   string
		want   socialhub.ErrorCode
		class  socialhub.ErrorClass
	}{
		{403, "63", socialhub.CodeApprovalRequired, socialhub.ClassUserAction},
		{403, "64", socialhub.CodeApprovalRequired, socialhub.ClassUserAction},
		{401, "65", socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{400, "", socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{413, "", socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{415, "", socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{422, "", socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{401, "", socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{402, "", socialhub.CodeApprovalRequired, socialhub.ClassUserAction},
		{403, "", socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{404, "", socialhub.CodeNotFound, socialhub.ClassPermanent},
		{410, "", socialhub.CodeNotFound, socialhub.ClassPermanent},
		{409, "", socialhub.CodeConflict, socialhub.ClassPermanent},
		{424, "", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{429, "", socialhub.CodeRateLimited, socialhub.ClassRetryable},
		{503, "", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{418, "", socialhub.CodePlatformError, socialhub.ClassPermanent},
	}
	for _, test := range tests {
		code, class := classifyError(test.status, test.code)
		if code != test.want || class != test.class {
			t.Errorf("status=%d code=%q got=%s/%s", test.status, test.code, code, class)
		}
	}
	quota := DefaultQuotaPolicy()
	if quota.RequestsPerSecond != 1 || quota.RequestsPer30Minute != 30 || len(quota.Dimensions) != 2 {
		t.Fatalf("quota=%#v", quota)
	}
	if !validEndpoint("https://example.test") || validEndpoint("mailto:test@example.test") || validEndpoint("https://user:pass@example.test") ||
		!validOrganizationID(testOrganizationID) || validOrganizationID("01") || validOrganizationID("9223372036854775808") ||
		!validBasicKeyID(testKeyID) || validBasicKeyID("bad:key") || !validCountry("US") || !validCountry("AQ") ||
		!validCountry("BQ") || !validCountry("SS") || validCountry("ZZ") ||
		!validCreativePackType(CreativePackVideoPlayable) || validCreativePackType("bad") || !validPlatform(PlatformIOS) || validPlatform("bad") ||
		!validFilterValue("event name") || validFilterValue("bad,value") || validPostInstallDay(2) {
		t.Fatal("validation helper mismatch")
	}
	if value, ok := normalizedCompression(""); !ok || value != CompressionIdentity {
		t.Fatal("default compression mismatch")
	}
	if value, ok := normalizedCompression("bad"); ok || value != "" {
		t.Fatal("invalid compression accepted")
	}
	if parseRetryAfter("1.5") != 1500*time.Millisecond || parseRetryAfter("bad") != 0 || firstNonEmpty("", "value") != "value" ||
		boundedMessage(strings.Repeat("x", 10), 5) != "xxxxx" || !strings.Contains(redactSensitive("token: secret-value"), "[REDACTED]") {
		t.Fatal("error helper mismatch")
	}
	future := time.Now().Add(time.Minute).UTC().Format(http.TimeFormat)
	if parseRetryAfter(future) <= 0 || parseRetryAfter("999999") != 0 {
		t.Fatal("Retry-After date parsing failed")
	}
	transportErr := &url.Error{Op: "Get", URL: "https://example.test?token=secret", Err: errors.New("dial failed")}
	if sanitizeCause(transportErr).Error() != "dial failed" || sanitizeCause(nil) != nil {
		t.Fatal("transport error sanitization failed")
	}
	if (&APIError{}).Error() == "" || (*APIError)(nil).Error() == "" || (*APIError)(nil).Unwrap() != nil || (*APIError)(nil).Retryable() {
		t.Fatal("nil API error contract failed")
	}
}

func TestCopyAndEOFHelpers(t *testing.T) {
	var output bytes.Buffer
	written, err := copyBounded(&output, strings.NewReader("1234"), 4)
	if err != nil || written != 4 || output.String() != "1234" {
		t.Fatalf("written=%d output=%q err=%v", written, output.String(), err)
	}
	output.Reset()
	written, err = copyBounded(&output, strings.NewReader("12345"), 4)
	if !errors.Is(err, errReportTooLarge) || written != 4 || output.String() != "1234" {
		t.Fatalf("written=%d output=%q err=%v", written, output.String(), err)
	}
	if !validEOFRecord([]string{"#__EOF__", "rows=2", ""}, 3, 2) || validEOFRecord([]string{"#__EOF__", "rows=3", ""}, 3, 2) ||
		validEOFRecord([]string{"#__EOF__", "bad"}, 2, 0) || validEOFRecord([]string{"#__EOF__", "rows=0", "extra"}, 3, 0) {
		t.Fatal("EOF validation mismatch")
	}
	sourceErr := errors.New("read failed")
	if hub := requireHubError(t, reportCopyError("report", sourceErr)); hub.Code != socialhub.CodeTemporarilyUnavailable || hub.Class != socialhub.ClassRetryable {
		t.Fatalf("source error=%v", hub)
	}
	if expectedMediaType(FormatCSV) != "text/csv" || expectedMediaType(FormatJSON) != "application/json" || normalizedContentEncoding(" identity ") != "" {
		t.Fatal("media helper mismatch")
	}
}

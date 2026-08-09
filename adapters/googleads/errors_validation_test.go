package googleads

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestGoogleAdsFailureAndErrorClassification(t *testing.T) {
	header := http.Header{"Request-Id": {"header-request"}, "Retry-After": {"2.5"}}
	err := decodeHTTPError(http.StatusTooManyRequests, header, []byte(`{
		"error":{"code":429,"status":"RESOURCE_EXHAUSTED","message":"quota",
		"details":[{"@type":"type.googleapis.com/google.ads.googleads.v25.errors.GoogleAdsFailure",
		"requestId":"failure-request","errors":[{"errorCode":{"quotaError":"RESOURCE_TEMPORARILY_EXHAUSTED"},
		"message":"developer_token: secret-value"}]}]}}
	`))
	hub := hubError(t, err)
	if !errors.Is(err, socialhub.ErrRateLimited) || !hub.Retryable() || hub.PlatformCode != "RESOURCE_TEMPORARILY_EXHAUSTED" ||
		hub.PlatformMessage != "developer_token: [REDACTED]" || hub.RequestID != "failure-request" || hub.RetryAfter != 2500*time.Millisecond {
		t.Fatalf("error=%#v", hub)
	}

	cases := []struct {
		status   int
		platform string
		code     socialhub.ErrorCode
		class    socialhub.ErrorClass
	}{
		{400, "INVALID_ARGUMENT", socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{400, "FAILED_PRECONDITION", socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{429, "RESOURCE_EXHAUSTED", socialhub.CodeRateLimited, socialhub.ClassRetryable},
		{401, "UNAUTHENTICATED", socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{403, "PERMISSION_DENIED", socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{404, "NOT_FOUND", socialhub.CodeNotFound, socialhub.ClassPermanent},
		{409, "ALREADY_EXISTS", socialhub.CodeConflict, socialhub.ClassPermanent},
		{503, "UNAVAILABLE", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{400, "", socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{401, "", socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{403, "", socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{404, "", socialhub.CodeNotFound, socialhub.ClassPermanent},
		{409, "", socialhub.CodeConflict, socialhub.ClassPermanent},
		{429, "", socialhub.CodeRateLimited, socialhub.ClassRetryable},
		{500, "", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{418, "", socialhub.CodePlatformError, socialhub.ClassPermanent},
	}
	for _, test := range cases {
		code, class := classifyError(test.status, test.platform)
		if code != test.code || class != test.class {
			t.Errorf("classify(%d,%q)=%s/%s", test.status, test.platform, code, class)
		}
	}
	contract := platformContractError("test", "contract")
	if hubError(t, contract).Code != socialhub.CodePlatformError {
		t.Fatalf("contract=%v", contract)
	}
}

func TestErrorAndValidationHelpers(t *testing.T) {
	future := time.Now().Add(2 * time.Minute).UTC().Format(http.TimeFormat)
	if parseRetryAfter(future) <= 0 || parseRetryAfter("bad") != 0 || parseRetryAfter("-1") != 0 || parseRetryAfter("90000") != 0 {
		t.Fatal("Retry-After parsing failed")
	}
	message := "access_token=one refresh_token:two client_secret three developer_tokenfour"
	redacted := redactSensitive(message)
	if strings.Contains(redacted, "one") || strings.Contains(redacted, "two") || strings.Contains(redacted, "three") || !strings.Contains(redacted, "developer_tokenfour") {
		t.Fatalf("redacted=%q", redacted)
	}
	if firstNonEmpty("", " value ") != " value " || firstNonEmpty("", "") != "" {
		t.Fatal("firstNonEmpty failed")
	}

	falseChecks := []struct {
		name string
		ok   bool
	}{
		{"customer length", validCustomerID("123")},
		{"customer character", validCustomerID("123456789x")},
		{"numeric empty", validNumericID("")},
		{"numeric zero", validNumericID("000")},
		{"numeric character", validNumericID("12x")},
		{"opaque empty", validOpaque("", 10)},
		{"opaque trim", validOpaque(" value ", 10)},
		{"opaque control", validOpaque("a\nb", 10)},
		{"resource collection", validResourceName(testCustomerID, "campaigns", testBudget)},
		{"ad group ad separator", validResourceName(testCustomerID, "adGroupAds", "customers/1234567890/adGroupAds/1")},
		{"customer resource", validCustomerResourceName("customers/123")},
		{"final URL empty", validateFinalURLs(nil)},
		{"final URL scheme", validateFinalURLs([]string{"ftp://example.com"})},
		{"headline count", validateTextAssets(validHeadlines()[:2], 3, 15, 30)},
		{"headline output", validateTextAssets([]AdTextAsset{{Text: "a", AssetPerformanceLabel: "GOOD"}}, 1, 1, 30)},
		{"headline pin", validateTextAssets([]AdTextAsset{{Text: "a", PinnedField: "BAD"}}, 1, 1, 30)},
		{"path slash", validOptionalPath("bad/path")},
		{"GAQL mutation", validGAQL("UPDATE campaign SET name = x")},
		{"GAQL comment", validGAQL("SELECT campaign.id FROM campaign -- comment")},
		{"JSON uppercase", validJSONField("ResourceName")},
		{"JSON punctuation", validJSONField("bad_field")},
		{"optional int sign", validOptionalInt64("-1")},
		{"optional int length", validOptionalInt64(strings.Repeat("1", 21))},
	}
	for _, check := range falseChecks {
		if check.ok {
			t.Errorf("%s unexpectedly valid", check.name)
		}
	}
	for _, pinned := range []string{"HEADLINE_1", "HEADLINE_2", "HEADLINE_3", "DESCRIPTION_1", "DESCRIPTION_2"} {
		if !validPinnedField(pinned) {
			t.Errorf("pinned=%s", pinned)
		}
	}
	if !validOptionalInt64("") || !validOptionalInt64("123") {
		t.Fatal("optional int64 failed")
	}

	if _, err := mergeFields("test", nil, map[string]any{"bad_field": true}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("field name error=%v", err)
	}
	if _, err := mergeFields("test", nil, map[string]any{"status": "ENABLED"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("protected field error=%v", err)
	}
	if _, err := mergeFields("test", nil, map[string]any{"customField": func() {}}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("JSON field error=%v", err)
	}
}

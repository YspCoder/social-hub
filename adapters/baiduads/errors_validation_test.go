package baiduads

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestBusinessEnvelopeAndErrorClassification(t *testing.T) {
	zero := 0
	success := apiEnvelope[[]int]{Header: &responseHeader{Status: &zero}, Body: &responseBody[[]int]{Data: &[]int{1}}}
	data, err := requireEnvelope("test", success, http.Header{"X-B3-Traceid": {"header-trace"}})
	if err != nil || len(*data) != 1 {
		t.Fatalf("data=%v err=%v", data, err)
	}
	missing := []apiEnvelope[[]int]{
		{},
		{Header: &responseHeader{}},
		{Header: &responseHeader{Status: &zero}},
		{Header: &responseHeader{Status: &zero}, Body: &responseBody[[]int]{}},
	}
	for _, envelope := range missing {
		if requireHubError(t, envelopeError(envelope)).Code != socialhub.CodePlatformError {
			t.Fatalf("envelope=%+v", envelope)
		}
	}
	status := 1
	tests := []struct {
		name       string
		failure    apiFailure
		status     int
		want       socialhub.ErrorCode
		class      socialhub.ErrorClass
		retryAfter time.Duration
	}{
		{"auth", apiFailure{Code: 894061, Message: "expired"}, 1, socialhub.CodeUnauthenticated, socialhub.ClassUserAction, 0},
		{"permission", apiFailure{Code: 8406, Message: "denied"}, 1, socialhub.CodePermissionDenied, socialhub.ClassUserAction, 0},
		{"qps", apiFailure{Code: 8501, Message: "slow"}, 1, socialhub.CodeRateLimited, socialhub.ClassRetryable, time.Second},
		{"quota", apiFailure{Code: 901245, Message: "quota"}, 1, socialhub.CodeRateLimited, socialhub.ClassRetryable, time.Minute},
		{"temporary", apiFailure{Code: 1, Message: "retry"}, 3, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, 0},
		{"platform", apiFailure{Code: 1, Message: "failed"}, 1, socialhub.CodePlatformError, socialhub.ClassPermanent, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status = test.status
			envelope := apiEnvelope[[]int]{Header: &responseHeader{
				Status: &status, Desc: "failure", TraceID: "body-trace", Failures: []apiFailure{test.failure},
			}}
			_, err := requireEnvelope("operation", envelope, nil)
			hub := requireHubError(t, err)
			if hub.Code != test.want || hub.Class != test.class || hub.PlatformCode != strings.TrimSpace(strings.TrimPrefix(hub.PlatformCode, "+")) ||
				hub.PlatformMessage != test.failure.Message || hub.RequestID != "body-trace" || hub.RetryAfter != test.retryAfter {
				t.Fatalf("err=%+v", hub)
			}
		})
	}
	status = 1
	_, err = requireEnvelope("desc", apiEnvelope[[]int]{Header: &responseHeader{Status: &status, Desc: "description"}}, nil)
	if requireHubError(t, err).PlatformMessage != "description" {
		t.Fatalf("desc err=%v", err)
	}
}

func TestHTTPErrorMappingAndHelpers(t *testing.T) {
	tests := []struct {
		status int
		want   socialhub.ErrorCode
		class  socialhub.ErrorClass
	}{
		{http.StatusBadRequest, socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{http.StatusUnprocessableEntity, socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{http.StatusUnauthorized, socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{http.StatusForbidden, socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{http.StatusNotFound, socialhub.CodeNotFound, socialhub.ClassPermanent},
		{http.StatusConflict, socialhub.CodeConflict, socialhub.ClassPermanent},
		{http.StatusTooManyRequests, socialhub.CodeRateLimited, socialhub.ClassRetryable},
		{http.StatusServiceUnavailable, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{http.StatusTeapot, socialhub.CodePlatformError, socialhub.ClassPermanent},
	}
	for _, test := range tests {
		header := http.Header{"X-B3-Traceid": {"http-trace"}, "Retry-After": {"2.5"}}
		err := decodeHTTPError(test.status, header, []byte(`{"header":{"failures":[{"code":99,"message":"failed"}]}}`))
		hub := requireHubError(t, err)
		if hub.Code != test.want || hub.Class != test.class || hub.RequestID != "http-trace" || hub.PlatformCode != "99" ||
			hub.PlatformMessage != "failed" || hub.RetryAfter != 2500*time.Millisecond {
			t.Fatalf("status=%d err=%+v", test.status, hub)
		}
	}
	err := decodeHTTPError(http.StatusBadRequest, nil, []byte(`{"code":7,"message":"top-level"}`))
	if hub := requireHubError(t, err); hub.PlatformCode != "7" || hub.PlatformMessage != "top-level" {
		t.Fatalf("top-level err=%+v", hub)
	}
	if parseRetryAfter("bad") != 0 || parseRetryAfter("999999") != 0 || parseRetryAfter("0") != 0 {
		t.Fatal("invalid Retry-After accepted")
	}
	long := strings.Repeat("界", 600)
	if got := boundedMessage(long, 10); len([]rune(got)) != 10 {
		t.Fatalf("bounded length=%d", len([]rune(got)))
	}
	if boundedMessage("short", 10) != "short" || firstNonEmpty("", " value ") != " value " || firstNonEmpty("", "") != "" {
		t.Fatal("string helper failed")
	}
	if headerTraceID(nil) != "" || headerTraceID(&responseHeader{TraceID: "trace"}) != "trace" {
		t.Fatal("trace helper failed")
	}
	plain := errors.New("plain")
	if withOperation(nil, "op") != nil || !errors.Is(withOperation(plain, "op"), plain) {
		t.Fatal("operation helper failed")
	}
	hubErr := &socialhub.Error{Code: socialhub.CodePlatformError}
	_ = withOperation(hubErr, "assigned")
	if hubErr.Op != "assigned" {
		t.Fatalf("operation=%q", hubErr.Op)
	}
}

func TestValidationHelpers(t *testing.T) {
	if !validOpaque("opaque", 10) || validOpaque(" opaque", 10) || validOpaque("bad\n", 10) || validOpaque("long", 3) {
		t.Fatal("opaque validation failed")
	}
	if baiduTextLength("A中") != 3 || !validText("中文", 4, 4) || validText(" 中文", 1, 20) {
		t.Fatal("Baidu text validation failed")
	}
	for _, value := range []string{"field", "field2", "camelCase"} {
		if !validFieldName(value) {
			t.Fatalf("field rejected: %q", value)
		}
	}
	for _, value := range []string{"Field", "field_name", ""} {
		if validFieldName(value) {
			t.Fatalf("field accepted: %q", value)
		}
	}
	if validateIDs("ids", []int64{1}, 1, false) != nil || validateIDs("ids", nil, 1, false) == nil ||
		validateIDs("ids", []int64{1, 2}, 1, true) == nil || validateIDs("ids", []int64{0}, 1, true) == nil {
		t.Fatal("ID validation failed")
	}
	if validateFields("fields", []string{"field"}, 1) != nil || validateFields("fields", []string{"bad_name"}, 1) == nil ||
		validateFields("fields", []string{"one", "two"}, 1) == nil {
		t.Fatal("field validation failed")
	}
	fields := appendRequiredFields([]string{"id"}, "id", "name")
	if len(fields) != 2 || fields[1] != "name" {
		t.Fatalf("fields=%v", fields)
	}
	merged, err := mergeFields("merge", map[string]any{"id": 1}, map[string]any{"name": "value"}, "token")
	if err != nil || merged["name"] != "value" {
		t.Fatalf("merged=%v err=%v", merged, err)
	}
	invalidFields := []map[string]any{
		{"bad_name": true}, {"id": 2}, {"token": "secret"}, {"value": func() {}},
	}
	for _, values := range invalidFields {
		if _, err := mergeFields("merge", map[string]any{"id": 1}, values, "token"); err == nil {
			t.Fatalf("invalid extension accepted: %v", values)
		}
	}
	if !validDestinationURL("https://example.com/path?query=1", 100) || validDestinationURL("ftp://example.com", 100) ||
		validDestinationURL("https://user@example.com", 100) || validDestinationURL("", 100) {
		t.Fatal("destination validation failed")
	}
	if !validTabs([]int{0, 30}) || validTabs([]int{-1}) || validTabs([]int{31}) || validTabs(make([]int, 31)) {
		t.Fatal("tab validation failed")
	}
	if boolAsInt(true) != 1 || boolAsInt(false) != 0 || !validDate("2026-08-09") || validDate("2026-8-9") {
		t.Fatal("scalar validation failed")
	}
	if validateCallback("https://example.com/callback") != nil || validateCallback("https://example.com/callback#fragment") == nil {
		t.Fatal("callback validation failed")
	}
}

func TestRawModelDecodeFailures(t *testing.T) {
	for _, target := range []any{&Account{}, &Campaign{}, &AdGroup{}, &Creative{}} {
		if err := target.(interface{ UnmarshalJSON([]byte) error }).UnmarshalJSON([]byte(`{`)); err == nil {
			t.Fatalf("target %T accepted malformed JSON", target)
		}
	}
}

func envelopeError(envelope apiEnvelope[[]int]) error {
	_, err := requireEnvelope("test", envelope, nil)
	return err
}

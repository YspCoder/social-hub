package qq

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestBusinessAndHTTPErrorMapping(t *testing.T) {
	if err := (APIError{}).Err("test"); err != nil {
		t.Fatalf("success response=%v", err)
	}
	business := []struct {
		code int
		want socialhub.ErrorCode
	}{
		{100007, socialhub.CodeUnauthenticated},
		{11253, socialhub.CodeApprovalRequired},
		{10001, socialhub.CodeNotFound},
		{11282, socialhub.CodePermissionDenied},
		{100001, socialhub.CodeRateLimited},
		{11281, socialhub.CodeTemporarilyUnavailable},
		{12002, socialhub.CodeInvalidArgument},
		{999999, socialhub.CodePlatformError},
	}
	for _, test := range business {
		err := (APIError{Code: test.code, Message: strings.Repeat("x", 600), TraceID: "trace-1"}).Err("business")
		var platformErr *socialhub.Error
		if !errors.As(err, &platformErr) || platformErr.Code != test.want || platformErr.PlatformCode != strconv.Itoa(test.code) || platformErr.RequestID != "trace-1" || len([]rune(platformErr.PlatformMessage)) != 512 {
			t.Fatalf("business code %d error=%#v", test.code, err)
		}
		if test.want == socialhub.CodeApprovalRequired && platformErr.ApprovalURL == "" {
			t.Fatalf("approval error=%#v", platformErr)
		}
	}

	statuses := []struct {
		status int
		want   socialhub.ErrorCode
	}{
		{http.StatusBadRequest, socialhub.CodeInvalidArgument},
		{http.StatusUnauthorized, socialhub.CodeUnauthenticated},
		{http.StatusForbidden, socialhub.CodePermissionDenied},
		{http.StatusNotFound, socialhub.CodeNotFound},
		{http.StatusConflict, socialhub.CodeConflict},
		{http.StatusTooManyRequests, socialhub.CodeRateLimited},
		{http.StatusServiceUnavailable, socialhub.CodeTemporarilyUnavailable},
		{http.StatusTeapot, socialhub.CodePlatformError},
	}
	for _, test := range statuses {
		header := http.Header{"X-Request-Id": {"request-1"}, "Retry-After": {"12"}}
		err := decodeHTTPError(test.status, header, []byte("not-json"))
		var platformErr *socialhub.Error
		if !errors.As(err, &platformErr) || platformErr.Code != test.want || platformErr.HTTPStatus != test.status || platformErr.RequestID != "request-1" || platformErr.RetryAfter != 12*time.Second {
			t.Fatalf("HTTP status %d error=%#v", test.status, err)
		}
	}
	err := decodeHTTPError(http.StatusBadGateway, http.Header{"X-Tps-Trace-Id": {"header-trace"}}, []byte(`{"err_code":100001,"message":"rate","trace_id":"body-trace"}`))
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodeRateLimited || platformErr.HTTPStatus != http.StatusBadGateway || platformErr.RequestID != "body-trace" {
		t.Fatalf("business HTTP error=%#v", err)
	}
}

func TestErrorMetadataHelpers(t *testing.T) {
	if retryAfter("0") != 0 || retryAfter("bad") != 0 || retryAfter("90000") != 0 || retryAfter("60") != time.Minute {
		t.Fatal("retry-after parsing mismatch")
	}
	if got := requestID(http.Header{"X-Correlation-Id": {"correlation"}}); got != "correlation" {
		t.Fatalf("request ID=%q", got)
	}
	if got := boundedMessage("short", 10); got != "short" {
		t.Fatalf("bounded short=%q", got)
	}
	if got := boundedMessage("abcdef", 3); got != "abc" {
		t.Fatalf("bounded long=%q", got)
	}
	if err := unsupported("test", "unsupported"); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("unsupported=%v", err)
	}
}

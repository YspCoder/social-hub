package kakao

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
	if err := (APIError{}).Err("test", 0); err != nil {
		t.Fatalf("success response=%v", err)
	}
	business := []struct {
		code int
		want socialhub.ErrorCode
	}{
		{-1, socialhub.CodeTemporarilyUnavailable},
		{-2, socialhub.CodeInvalidArgument},
		{-3, socialhub.CodeApprovalRequired},
		{-4, socialhub.CodePermissionDenied},
		{-9, socialhub.CodeUnsupported},
		{-10, socialhub.CodeRateLimited},
		{-401, socialhub.CodeUnauthenticated},
		{-999, socialhub.CodePlatformError},
	}
	for _, test := range business {
		err := (APIError{
			Code: test.code, Message: strings.Repeat("x", 600), RequiredScopes: []string{"talk_message"},
		}).Err("business", 0)
		var platformErr *socialhub.Error
		if !errors.As(err, &platformErr) || platformErr.Code != test.want || platformErr.PlatformCode != strconv.Itoa(test.code) ||
			len([]rune(platformErr.PlatformMessage)) != 512 || len(platformErr.RequiredScopes) != 1 {
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
		if !errors.As(err, &platformErr) || platformErr.Code != test.want || platformErr.HTTPStatus != test.status ||
			platformErr.RequestID != "request-1" || platformErr.RetryAfter != 12*time.Second {
			t.Fatalf("HTTP status %d error=%#v", test.status, err)
		}
	}
	err := decodeHTTPError(http.StatusForbidden, http.Header{"X-Kakao-Request-Id": {"kakao-request"}}, []byte(`{"code":-402,"msg":"consent","required_scopes":["talk_message"]}`))
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodeApprovalRequired || platformErr.RequestID != "kakao-request" || len(platformErr.RequiredScopes) != 1 {
		t.Fatalf("business HTTP error=%#v", err)
	}
}

func TestErrorAndValidationHelpers(t *testing.T) {
	if retryAfter("0") != 0 || retryAfter("bad") != 0 || retryAfter("90000") != 0 || retryAfter("60") != time.Minute {
		t.Fatal("retry-after parsing mismatch")
	}
	if got := requestID(http.Header{"X-Correlation-Id": {"correlation"}}); got != "correlation" {
		t.Fatalf("request ID=%q", got)
	}
	if boundedMessage("short", 10) != "short" || boundedMessage("abcdef", 3) != "abc" {
		t.Fatal("bounded message mismatch")
	}
	if !validServiceUserID("123") || validServiceUserID("0") || validServiceUserID(" 1") {
		t.Fatal("service user ID validation mismatch")
	}
	if !validOptionalString("", 1) || validOptionalString("bad\n", 10) ||
		!validBoundedString("opaque/\\?#", 20) || validBoundedString("bad\n", 10) {
		t.Fatal("string validation mismatch")
	}
	if err := unsupported("test", "unsupported"); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("unsupported=%v", err)
	}
}

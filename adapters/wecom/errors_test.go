package wecom

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestBusinessAndHTTPErrorMapping(t *testing.T) {
	if err := (APIResponse{}).Err("test"); err != nil {
		t.Fatalf("success response=%v", err)
	}
	business := []struct {
		code int
		want socialhub.ErrorCode
	}{
		{-1, socialhub.CodeTemporarilyUnavailable},
		{40001, socialhub.CodeUnauthenticated},
		{44001, socialhub.CodeInvalidArgument},
		{48002, socialhub.CodeApprovalRequired},
		{60111, socialhub.CodeNotFound},
		{45011, socialhub.CodeRateLimited},
		{99999, socialhub.CodePlatformError},
	}
	for _, test := range business {
		err := APIResponse{ErrCode: test.code, ErrMsg: strings.Repeat("错", 600)}.Err("business")
		var platformErr *socialhub.Error
		if !errors.As(err, &platformErr) || platformErr.Code != test.want || len([]rune(platformErr.PlatformMessage)) != 512 {
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

	err := decodeHTTPError(http.StatusBadGateway, http.Header{"X-Logid": {"log-1"}}, []byte(`{"errcode":45009,"errmsg":"too many calls"}`))
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodeRateLimited || platformErr.HTTPStatus != http.StatusBadGateway || platformErr.RequestID != "log-1" {
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
}

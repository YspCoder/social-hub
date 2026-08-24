package instagram

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestGraphRateLimitMapping(t *testing.T) {
	header := http.Header{"Retry-After": {"0.5"}}
	err := decodeHTTPError(http.StatusTooManyRequests, header, []byte(`{"error":{"message":"slow down","code":4,"fbtrace_id":"trace"}}`))
	if !errors.Is(err, socialhub.ErrRateLimited) {
		t.Fatalf("error=%v", err)
	}
	var platformError *socialhub.Error
	if !errors.As(err, &platformError) || !platformError.Retryable() || platformError.RetryAfter != 500*time.Millisecond || platformError.RequestID != "trace" {
		t.Fatalf("platform error=%#v", platformError)
	}
}

func TestGraphBusinessCodeMapping(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		body      string
		wantCode  socialhub.ErrorCode
		wantClass socialhub.ErrorClass
	}{
		{name: "transient code", status: http.StatusBadRequest, body: `{"error":{"code":2}}`, wantCode: socialhub.CodeTemporarilyUnavailable, wantClass: socialhub.ClassRetryable},
		{name: "rate code", status: http.StatusBadRequest, body: `{"error":{"code":613}}`, wantCode: socialhub.CodeRateLimited, wantClass: socialhub.ClassRetryable},
		{name: "permission code", status: http.StatusBadRequest, body: `{"error":{"code":10}}`, wantCode: socialhub.CodePermissionDenied, wantClass: socialhub.ClassUserAction},
		{name: "invalid parameter", status: http.StatusInternalServerError, body: `{"error":{"code":100}}`, wantCode: socialhub.CodeInvalidArgument, wantClass: socialhub.ClassPermanent},
		{name: "expired token", status: http.StatusBadRequest, body: `{"error":{"code":190}}`, wantCode: socialhub.CodeUnauthenticated, wantClass: socialhub.ClassUserAction},
		{name: "user conflict", status: http.StatusBadRequest, body: `{"error":{"code":551}}`, wantCode: socialhub.CodeConflict, wantClass: socialhub.ClassUserAction},
		{name: "subcode conflict", status: http.StatusBadRequest, body: `{"error":{"code":100,"error_subcode":1545041}}`, wantCode: socialhub.CodeConflict, wantClass: socialhub.ClassUserAction},
		{name: "transient flag", status: http.StatusBadRequest, body: `{"error":{"is_transient":true}}`, wantCode: socialhub.CodeTemporarilyUnavailable, wantClass: socialhub.ClassRetryable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := decodeHTTPError(test.status, nil, []byte(test.body))
			var platformError *socialhub.Error
			if !errors.As(err, &platformError) || platformError.Code != test.wantCode || platformError.Class != test.wantClass {
				t.Fatalf("error=%#v", platformError)
			}
		})
	}
}

func TestGraphHTTPStatusMappingAndBounds(t *testing.T) {
	tests := []struct {
		status   int
		wantCode socialhub.ErrorCode
	}{
		{status: http.StatusBadRequest, wantCode: socialhub.CodeInvalidArgument},
		{status: http.StatusRequestEntityTooLarge, wantCode: socialhub.CodeInvalidArgument},
		{status: http.StatusUnauthorized, wantCode: socialhub.CodeUnauthenticated},
		{status: http.StatusForbidden, wantCode: socialhub.CodePermissionDenied},
		{status: http.StatusNotFound, wantCode: socialhub.CodeNotFound},
		{status: http.StatusGone, wantCode: socialhub.CodeNotFound},
		{status: http.StatusConflict, wantCode: socialhub.CodeConflict},
		{status: http.StatusTooManyRequests, wantCode: socialhub.CodeRateLimited},
		{status: http.StatusBadGateway, wantCode: socialhub.CodeTemporarilyUnavailable},
		{status: http.StatusTeapot, wantCode: socialhub.CodePlatformError},
	}
	for _, test := range tests {
		err := decodeHTTPError(test.status, nil, nil)
		var platformError *socialhub.Error
		if !errors.As(err, &platformError) || platformError.Code != test.wantCode {
			t.Fatalf("status=%d error=%#v", test.status, platformError)
		}
	}
	long := strings.Repeat("界", 600)
	err := decodeHTTPError(http.StatusBadRequest, http.Header{"X-Fb-Request-Id": {"request"}}, []byte(`{"error":{"message":"`+long+`"}}`))
	var platformError *socialhub.Error
	if !errors.As(err, &platformError) || len([]rune(platformError.PlatformMessage)) != 512 || platformError.RequestID != "request" {
		t.Fatalf("bounded error=%#v", platformError)
	}
	if retryAfter("invalid") != 0 || retryAfter(" -1 ") != 0 || retryAfter(" 1.25 ") != 1250*time.Millisecond {
		t.Fatal("retry-after parsing mismatch")
	}
}

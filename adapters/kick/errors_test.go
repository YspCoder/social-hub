package kick

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestErrorMappingAndHelpers(t *testing.T) {
	cases := []struct {
		status int
		target error
	}{
		{http.StatusBadRequest, socialhub.ErrInvalidArgument}, {http.StatusUnauthorized, socialhub.ErrUnauthenticated},
		{http.StatusForbidden, socialhub.ErrPermissionDenied}, {http.StatusNotFound, socialhub.ErrNotFound},
		{http.StatusConflict, socialhub.ErrConflict}, {http.StatusTooManyRequests, socialhub.ErrRateLimited},
		{http.StatusServiceUnavailable, socialhub.ErrUnavailable},
	}
	for _, test := range cases {
		err := decodeHTTPError(test.status, http.Header{"X-Request-Id": {"request-1"}}, []byte(`{"error":"kick_error","error_description":"details"}`))
		if !errors.Is(err, test.target) {
			t.Fatalf("status %d: %v", test.status, err)
		}
		var hubError *socialhub.Error
		if !errors.As(err, &hubError) || hubError.RequestID != "request-1" || hubError.PlatformMessage != "details" {
			t.Fatalf("status %d details: %#v", test.status, hubError)
		}
	}
	if parseRetryAfter("1.5") != 1500*time.Millisecond || parseRetryAfter("bad") != 0 || parseRetryAfter("999999") != 0 {
		t.Fatal("Retry-After parsing mismatch")
	}
	if boundedMessage(strings.Repeat("x", 5), 3) != "xxx" || firstNonEmpty("", " value ") != " value " {
		t.Fatal("bounded/first helper mismatch")
	}
	inner := errors.New("dial failed")
	if sanitizeTransportError(&url.Error{Op: "Get", URL: "https://secret.test/?token=x", Err: inner}) != inner {
		t.Fatal("URL error was not sanitized")
	}
	if !errors.Is(unsupported("op", "no"), socialhub.ErrUnsupported) || !errors.Is(approvalRequired("op", nil, "approval"), socialhub.ErrApprovalRequired) {
		t.Fatal("error helper category mismatch")
	}
}

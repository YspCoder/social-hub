package line

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestDecodeHTTPError(t *testing.T) {
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
		{http.StatusBadGateway, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{http.StatusTeapot, socialhub.CodePlatformError, socialhub.ClassPermanent},
	}
	for _, test := range tests {
		header := http.Header{
			"X-Line-Request-Id":          {"request-id"},
			"X-Line-Accepted-Request-Id": {"accepted-id"},
			"Retry-After":                {"1.5"},
		}
		err := decodeHTTPError(test.status, header, []byte(`{"message":"invalid request","details":[{"property":"messages[0].text","message":"is too long"}]}`))
		var platformErr *socialhub.Error
		if !errors.As(err, &platformErr) || platformErr.Code != test.code || platformErr.Class != test.class || platformErr.HTTPStatus != test.status || platformErr.RequestID != "accepted-id" || platformErr.RetryAfter != 1500*time.Millisecond || platformErr.PlatformMessage != "invalid request: messages[0].text: is too long" {
			t.Fatalf("status %d error=%#v", test.status, err)
		}
	}
}

func TestErrorHelpers(t *testing.T) {
	if retryAfter("-1") != 0 || retryAfter("later") != 0 || retryAfter("0.25") != 250*time.Millisecond {
		t.Fatalf("retry-after parsing failed")
	}
	long := strings.Repeat("界", 513)
	if got := boundedMessage(long, 512); len([]rune(got)) != 512 {
		t.Fatalf("bounded message runes=%d", len([]rune(got)))
	}
	if got := boundedMessage("short", 512); got != "short" {
		t.Fatalf("short message=%q", got)
	}
	if firstNonEmpty(" ", "value", "later") != "value" || firstNonEmpty("", " ") != "" {
		t.Fatal("first non-empty failed")
	}
	values := nonEmpty(" one ", "", " two")
	if len(values) != 2 || values[0] != "one" || values[1] != "two" {
		t.Fatalf("non-empty=%v", values)
	}
}

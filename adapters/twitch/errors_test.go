package twitch

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestHTTPErrorMapping(t *testing.T) {
	cases := []struct {
		status int
		target error
	}{
		{http.StatusBadRequest, socialhub.ErrInvalidArgument},
		{http.StatusUnauthorized, socialhub.ErrUnauthenticated},
		{http.StatusForbidden, socialhub.ErrPermissionDenied},
		{http.StatusNotFound, socialhub.ErrNotFound},
		{http.StatusConflict, socialhub.ErrConflict},
		{http.StatusTooManyRequests, socialhub.ErrRateLimited},
		{http.StatusServiceUnavailable, socialhub.ErrUnavailable},
	}
	for _, test := range cases {
		header := http.Header{"Twitch-Trace-Id": {"trace-1"}, "Retry-After": {"2.5"}}
		err := decodeHTTPError(test.status, header, []byte(`{"error":"Unauthorized","status":401,"message":"denied"}`))
		if !errors.Is(err, test.target) {
			t.Fatalf("status %d: %v", test.status, err)
		}
		var platformErr *socialhub.Error
		if !errors.As(err, &platformErr) || platformErr.RequestID != "trace-1" || platformErr.RetryAfter != 2500*time.Millisecond || platformErr.PlatformMessage != "denied" {
			t.Fatalf("mapped error: %#v", err)
		}
	}
	reset := time.Now().Add(3 * time.Second).Unix()
	delay := twitchRetryAfter(http.Header{"Ratelimit-Reset": {strconv.FormatInt(reset, 10)}})
	if delay <= 0 || delay > 3*time.Second {
		t.Fatalf("reset delay: %v", delay)
	}
	if twitchRetryAfter(http.Header{"Retry-After": {"bad"}}) != 0 || boundedMessage(strings.Repeat("界", 520), 512) != strings.Repeat("界", 512) || firstNonEmpty("", "x") != "x" {
		t.Fatal("error helpers mismatch")
	}
}

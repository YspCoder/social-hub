package tiktok

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestTikTokEnvelopeAndHTTPErrorMapping(t *testing.T) {
	err := checkAPIError("video_init", apiError{Code: "rate_limit_exceeded", Message: "slow", LogID: "log-1"})
	var platformError *socialhub.Error
	if !errors.Is(err, socialhub.ErrRateLimited) || !errors.As(err, &platformError) || platformError.RequestID != "log-1" {
		t.Fatalf("envelope error=%v", err)
	}
	header := http.Header{"Retry-After": {"2.5"}}
	err = decodeHTTPError(http.StatusTooManyRequests, header, []byte(`{"error":{"code":"rate_limit_exceeded","message":"slow","log_id":"log-2"}}`))
	if !errors.As(err, &platformError) || platformError.RetryAfter != 2500*time.Millisecond {
		t.Fatalf("HTTP error=%v", err)
	}
}

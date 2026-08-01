package youtube

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestYouTubeErrorAndDurationMapping(t *testing.T) {
	header := http.Header{"Retry-After": {"2.5"}, "X-Guploader-Uploadid": {"request-1"}}
	err := decodeHTTPError(http.StatusForbidden, header, []byte(`{"error":{"code":403,"message":"quota","errors":[{"reason":"quotaExceeded"}]}}`))
	var platformError *socialhub.Error
	if !errors.Is(err, socialhub.ErrRateLimited) || !errors.As(err, &platformError) || platformError.RetryAfter != 2500*time.Millisecond || platformError.RequestID != "request-1" {
		t.Fatalf("error=%v", err)
	}
	duration := parseISODuration("PT1H2M3.5S")
	if duration == nil || *duration != time.Hour+2*time.Minute+3500*time.Millisecond {
		t.Fatalf("duration=%v", duration)
	}
}

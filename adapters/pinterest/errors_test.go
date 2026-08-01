package pinterest

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestPinterestErrorMapping(t *testing.T) {
	header := http.Header{"X-Pinterest-Rid": {"request-1"}, "X-Ratelimit-Reset": {"2"}}
	err := decodeHTTPError(http.StatusTooManyRequests, header, []byte(`{"code":8,"message":"Rate limit exceeded"}`))
	var typed *socialhub.Error
	if !errors.As(err, &typed) || !errors.Is(err, socialhub.ErrRateLimited) || typed.PlatformCode != "8" || typed.RequestID != "request-1" || typed.RetryAfter != 2*time.Second {
		t.Fatalf("error=%#v", err)
	}
}

package linkedin

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestDecodeHTTPError(t *testing.T) {
	header := http.Header{"Retry-After": {"2.5"}, "X-Li-Uuid": {"request-1"}}
	err := decodeHTTPError(http.StatusTooManyRequests, header, []byte(`{"serviceErrorCode":101,"message":"slow down"}`))
	var platformError *socialhub.Error
	if !errors.Is(err, socialhub.ErrRateLimited) || !errors.As(err, &platformError) {
		t.Fatalf("error=%v", err)
	}
	if platformError.PlatformCode != "101" || platformError.RequestID != "request-1" || platformError.RetryAfter != 2500*time.Millisecond {
		t.Fatalf("platform error=%#v", platformError)
	}
}

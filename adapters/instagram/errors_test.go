package instagram

import (
	"errors"
	"net/http"
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

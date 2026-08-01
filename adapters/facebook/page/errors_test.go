package page

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestDecodeRateLimitError(t *testing.T) {
	t.Parallel()
	header := http.Header{"Retry-After": {"9"}}
	err := decodeError(http.StatusTooManyRequests, header, []byte(`{"error":{"message":"limit","type":"OAuthException","code":4,"error_subcode":99,"fbtrace_id":"trace"}}`))
	if !errors.Is(err, socialhub.ErrRateLimited) {
		t.Fatalf("error = %v", err)
	}
	var platformError *socialhub.Error
	if !errors.As(err, &platformError) || platformError.PlatformCode != "4/99" || platformError.RequestID != "trace" || platformError.RetryAfter != 9*time.Second {
		t.Fatalf("platform error = %#v", platformError)
	}
}

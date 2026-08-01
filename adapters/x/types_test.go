package x

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestDecodeRateLimitError(t *testing.T) {
	t.Parallel()
	header := http.Header{"Retry-After": {"12"}, "X-Transaction-Id": {"tx-1"}}
	err := decodeError(http.StatusTooManyRequests, header, []byte(`{"title":"Too Many Requests","type":"https://api.x.com/problems/usage-capped"}`))
	if !errors.Is(err, socialhub.ErrRateLimited) {
		t.Fatalf("error = %v", err)
	}
	var platformError *socialhub.Error
	if !errors.As(err, &platformError) || platformError.RetryAfter != 12*time.Second || platformError.RequestID != "tx-1" {
		t.Fatalf("platform error = %#v", platformError)
	}
}

func TestDecodeHTTPErrorCategories(t *testing.T) {
	t.Parallel()
	tests := []struct {
		status int
		want   error
	}{
		{http.StatusBadRequest, socialhub.ErrInvalidArgument},
		{http.StatusUnauthorized, socialhub.ErrUnauthenticated},
		{http.StatusForbidden, socialhub.ErrPermissionDenied},
		{http.StatusNotFound, socialhub.ErrNotFound},
		{http.StatusConflict, socialhub.ErrConflict},
		{http.StatusServiceUnavailable, socialhub.ErrUnavailable},
	}
	for _, test := range tests {
		err := decodeError(test.status, nil, []byte(`{"detail":"provider detail"}`))
		if !errors.Is(err, test.want) {
			t.Errorf("status %d: error = %v, want %v", test.status, err, test.want)
		}
	}
}

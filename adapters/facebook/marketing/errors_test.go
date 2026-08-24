package marketing

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestGraphErrorMapping(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		status    int
		body      string
		want      error
		retryable bool
	}{
		{"token", http.StatusBadRequest, `{"error":{"code":190,"message":"expired","fbtrace_id":"trace"}}`, socialhub.ErrUnauthenticated, false},
		{"rate code", http.StatusBadRequest, `{"error":{"code":4,"message":"application limit"}}`, socialhub.ErrRateLimited, true},
		{"permission", http.StatusBadRequest, `{"error":{"code":200,"error_user_msg":"review required"}}`, socialhub.ErrPermissionDenied, false},
		{"invalid", http.StatusBadRequest, `{"error":{"code":100,"message":"bad field"}}`, socialhub.ErrInvalidArgument, false},
		{"transient", http.StatusBadRequest, `{"error":{"code":2,"is_transient":true}}`, socialhub.ErrUnavailable, true},
		{"not found", http.StatusNotFound, `{}`, socialhub.ErrNotFound, false},
		{"conflict", http.StatusConflict, `{}`, socialhub.ErrConflict, false},
		{"http rate", http.StatusTooManyRequests, `{}`, socialhub.ErrRateLimited, true},
		{"server", http.StatusServiceUnavailable, `{}`, socialhub.ErrUnavailable, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := decodeHTTPError(test.status, http.Header{"Retry-After": {"1.5"}}, []byte(test.body))
			if !errors.Is(err, test.want) {
				t.Fatalf("error=%v want=%v", err, test.want)
			}
			var hubError *socialhub.Error
			if !errors.As(err, &hubError) || hubError.Retryable() != test.retryable || hubError.RetryAfter != 1500*time.Millisecond {
				t.Fatalf("hub error=%#v", hubError)
			}
		})
	}
}

func TestBoundedErrorFields(t *testing.T) {
	t.Parallel()
	message := strings.Repeat("界", 600)
	err := decodeHTTPError(http.StatusBadRequest, nil, []byte(`{"error":{"code":100,"message":"`+message+`"}}`))
	var hubError *socialhub.Error
	if !errors.As(err, &hubError) || len([]rune(hubError.PlatformMessage)) != 512 || boundedMessage("short", 10) != "short" {
		t.Fatalf("error=%#v", hubError)
	}
	if parseRetryAfter("invalid") != 0 || parseRetryAfter("999999") != 0 || firstNonEmpty("", " value ") != " value " {
		t.Fatal("error helper contract failed")
	}
}

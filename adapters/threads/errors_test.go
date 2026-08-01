package threads

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestGraphErrorMapping(t *testing.T) {
	tests := []struct {
		name   string
		status int
		code   int
		want   error
		class  socialhub.ErrorClass
	}{
		{"bad request", http.StatusBadRequest, 100, socialhub.ErrInvalidArgument, socialhub.ClassPermanent},
		{"token", http.StatusBadRequest, 190, socialhub.ErrUnauthenticated, socialhub.ClassUserAction},
		{"permission", http.StatusBadRequest, 200, socialhub.ErrPermissionDenied, socialhub.ClassUserAction},
		{"rate", http.StatusBadRequest, 613, socialhub.ErrRateLimited, socialhub.ClassRetryable},
		{"conflict", http.StatusBadRequest, 506, socialhub.ErrConflict, socialhub.ClassPermanent},
		{"temporary", http.StatusBadRequest, 2, socialhub.ErrUnavailable, socialhub.ClassRetryable},
		{"HTTP rate", http.StatusTooManyRequests, 0, socialhub.ErrRateLimited, socialhub.ClassRetryable},
		{"server", http.StatusServiceUnavailable, 0, socialhub.ErrUnavailable, socialhub.ClassRetryable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			header := make(http.Header)
			header.Set("Retry-After", "2.5")
			body := []byte(`{"error":{"message":"failure","code":` + strconv.Itoa(test.code) + `,"error_subcode":7,"fbtrace_id":"trace-1"}}`)
			err := decodeHTTPError(test.status, header, body)
			var typed *socialhub.Error
			if !errors.As(err, &typed) || !errors.Is(err, test.want) || typed.Class != test.class || typed.Platform != "threads" || typed.Product != productName {
				t.Fatalf("error=%#v", err)
			}
			if typed.RetryAfter != 2500*time.Millisecond {
				t.Fatalf("retry after=%v", typed.RetryAfter)
			}
		})
	}
}

func TestErrorBoundsAndTransientOverride(t *testing.T) {
	message := strings.Repeat("界", 600)
	err := decodeHTTPError(http.StatusBadRequest, nil, []byte(`{"error":{"message":"`+message+`","code":100,"is_transient":true}}`))
	var typed *socialhub.Error
	if !errors.As(err, &typed) || !errors.Is(err, socialhub.ErrUnavailable) || len([]rune(typed.PlatformMessage)) != 512 {
		t.Fatalf("error=%#v", err)
	}
	if retryAfter("invalid") != 0 || retryAfter("-1") != 0 {
		t.Fatal("invalid retry headers should be ignored")
	}
}

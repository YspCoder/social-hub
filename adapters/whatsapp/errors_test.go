package whatsapp

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func errorCode(err error) socialhub.ErrorCode {
	var typed *socialhub.Error
	if !errors.As(err, &typed) {
		return ""
	}
	return typed.Code
}

func TestGraphAndWhatsAppErrorMapping(t *testing.T) {
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
		{"rate", http.StatusBadRequest, 130429, socialhub.ErrRateLimited, socialhub.ClassRetryable},
		{"pair rate", http.StatusBadRequest, 131056, socialhub.ErrRateLimited, socialhub.ClassRetryable},
		{"temporary", http.StatusBadRequest, 131016, socialhub.ErrUnavailable, socialhub.ClassRetryable},
		{"window", http.StatusBadRequest, 131047, socialhub.ErrPermissionDenied, socialhub.ClassUserAction},
		{"template missing", http.StatusBadRequest, 132001, socialhub.ErrNotFound, socialhub.ClassPermanent},
		{"recipient", http.StatusBadRequest, 131026, socialhub.ErrInvalidArgument, socialhub.ClassPermanent},
		{"HTTP rate", http.StatusTooManyRequests, 0, socialhub.ErrRateLimited, socialhub.ClassRetryable},
		{"server", http.StatusServiceUnavailable, 0, socialhub.ErrUnavailable, socialhub.ClassRetryable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			header := make(http.Header)
			header.Set("Retry-After", "2.5")
			body := []byte(`{"error":{"message":"failure","type":"OAuthException","code":` + strconv.Itoa(test.code) + `,"error_subcode":7,"fbtrace_id":"trace-1","error_data":{"details":"specific failure"}}}`)
			err := decodeHTTPError(test.status, header, body)
			var typed *socialhub.Error
			if !errors.As(err, &typed) || !errors.Is(err, test.want) || typed.Class != test.class || typed.Platform != "whatsapp" || typed.Product != productName {
				t.Fatalf("error=%#v", err)
			}
			if typed.PlatformMessage != "specific failure" || typed.RequestID != "trace-1" || typed.RetryAfter != 2500*time.Millisecond {
				t.Fatalf("mapped error=%#v", typed)
			}
		})
	}
}

func TestErrorBoundsTransientAndHelpers(t *testing.T) {
	message := strings.Repeat("x", 600)
	err := decodeHTTPError(http.StatusBadRequest, http.Header{"X-Fb-Request-Id": {"request-1"}}, []byte(`{"error":{"message":"`+message+`","code":100,"is_transient":true}}`))
	var typed *socialhub.Error
	if !errors.As(err, &typed) || !errors.Is(err, socialhub.ErrUnavailable) || len([]rune(typed.PlatformMessage)) != 512 || typed.RequestID != "request-1" {
		t.Fatalf("error=%#v", err)
	}
	if retryAfter("invalid") != 0 || retryAfter("-1") != 0 || firstNonEmpty("", "x") != "x" || boundedMessage("short", 10) != "short" {
		t.Fatal("error helpers mismatch")
	}
	var value stringInt64
	if err := json.Unmarshal([]byte(`"42"`), &value); err != nil || value != 42 {
		t.Fatalf("string size=%d error=%v", value, err)
	}
	if err := json.Unmarshal([]byte(`43`), &value); err != nil || value != 43 {
		t.Fatalf("number size=%d error=%v", value, err)
	}
	if err := json.Unmarshal([]byte(`"bad"`), &value); err == nil {
		t.Fatal("invalid size should fail")
	}
}

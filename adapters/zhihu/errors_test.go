package zhihu

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestBusinessErrorMappingOnHTTP200(t *testing.T) {
	tests := []struct {
		name      string
		code      int
		sentinel  error
		retryable bool
	}{
		{name: "frequency", code: 30001, sentinel: socialhub.ErrRateLimited, retryable: true},
		{name: "quota", code: 30002, sentinel: socialhub.ErrRateLimited, retryable: false},
		{name: "authentication", code: 20001, sentinel: socialhub.ErrUnauthenticated, retryable: false},
		{name: "internal", code: 90001, sentinel: socialhub.ErrUnavailable, retryable: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := responseEnvelope[struct{}]{Code: test.code, Message: "fixture"}.Err("fixture", http.StatusOK, nil)
			if !errors.Is(err, test.sentinel) {
				t.Fatalf("error=%v", err)
			}
			var platformError *socialhub.Error
			if !errors.As(err, &platformError) || platformError.Retryable() != test.retryable {
				t.Fatalf("platform error=%#v", platformError)
			}
		})
	}
}

func TestHTTPRateLimitMapping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Retry-After", "3")
		writer.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, true, false, false)
	_, err := client.SearchWorkflow().HotList(context.Background(), 1)
	if !errors.Is(err, socialhub.ErrRateLimited) {
		t.Fatalf("error=%v", err)
	}
	var platformError *socialhub.Error
	if !errors.As(err, &platformError) || !platformError.Retryable() || platformError.RetryAfter.Seconds() != 3 {
		t.Fatalf("platform error=%#v", platformError)
	}
}

package xiaohongshu

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestRateLimitErrorMapping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Retry-After", "4")
		writer.Header().Set("X-Request-ID", "request-1")
		writer.WriteHeader(http.StatusTooManyRequests)
		_, _ = writer.Write([]byte(`{"error_code":42901,"error_msg":"rate limited"}`))
	}))
	defer server.Close()
	clock := &steppingClock{now: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)}
	_, client := newTestAdapter(t, server, true, "", nil, clock)
	_, err := client.ShareWorkflow().Prepare(context.Background(), ShareRequest{Type: ShareTypeNormal, Images: []string{"https://cdn.example/note.jpg"}})
	if !errors.Is(err, socialhub.ErrRateLimited) {
		t.Fatalf("error=%v", err)
	}
	var platformError *socialhub.Error
	if !errors.As(err, &platformError) || !platformError.Retryable() || platformError.PlatformCode != "42901" || platformError.RequestID != "request-1" || platformError.RetryAfter != 4*time.Second {
		t.Fatalf("platform error=%#v", platformError)
	}
}

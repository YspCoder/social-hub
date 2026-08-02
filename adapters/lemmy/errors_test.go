package lemmy

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestHTTPErrorMapping(t *testing.T) {
	header := http.Header{"Retry-After": {"2.5"}, "X-Request-Id": {"request-1"}}
	err := decodeHTTPError(http.StatusTooManyRequests, header, []byte(`{"error":"rate_limit_error","message":"Slow down"}`))
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodeRateLimited || !platformErr.Retryable() ||
		platformErr.RetryAfter != 2500*time.Millisecond || platformErr.PlatformCode != "rate_limit_error" ||
		platformErr.PlatformMessage != "Slow down" || platformErr.RequestID != "request-1" {
		t.Fatalf("rate error=%#v", platformErr)
	}
	platformCodes := map[string]socialhub.ErrorCode{
		"not_logged_in": socialhub.CodeUnauthenticated, "not_a_moderator": socialhub.CodePermissionDenied,
		"community_already_exists": socialhub.CodeConflict, "couldnt_find_post": socialhub.CodeNotFound,
		"invalid_url": socialhub.CodeInvalidArgument, "content_too_long": socialhub.CodeInvalidArgument,
		"no_id_given": socialhub.CodeInvalidArgument,
	}
	for platformCode, want := range platformCodes {
		code, _ := classifyError(http.StatusTeapot, platformCode)
		if code != want {
			t.Fatalf("platform code %s mapped to %s, want %s", platformCode, code, want)
		}
	}
	statuses := map[int]socialhub.ErrorCode{
		http.StatusBadRequest: socialhub.CodeInvalidArgument, http.StatusUnauthorized: socialhub.CodeUnauthenticated,
		http.StatusForbidden: socialhub.CodePermissionDenied, http.StatusNotFound: socialhub.CodeNotFound,
		http.StatusConflict: socialhub.CodeConflict, http.StatusTooManyRequests: socialhub.CodeRateLimited,
		http.StatusInternalServerError: socialhub.CodeTemporarilyUnavailable, http.StatusTeapot: socialhub.CodePlatformError,
	}
	for status, want := range statuses {
		code, _ := classifyError(status, "")
		if code != want {
			t.Fatalf("status %d mapped to %s, want %s", status, code, want)
		}
	}
	if parseRetryAfter("bad") != 0 || parseRetryAfter("-1") != 0 || parseRetryAfter("90000") != 0 ||
		parseRetryAfter("1.5") != 1500*time.Millisecond {
		t.Fatal("Retry-After parsing failed")
	}
	long := strings.Repeat("界", 300)
	if len([]rune(boundedMessage(long, 10))) != 10 || firstNonEmpty(" ", "value") != "value" || firstNonEmpty("", "") != "" {
		t.Fatal("error message bounding failed")
	}
}

func TestTransportRateLimitAndRedirectIsolation(t *testing.T) {
	var redirected atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		redirected.Add(1)
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Query().Get("id") {
		case "1":
			if request.Header.Get("X-Request-ID") != "caller-request" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writer.Header().Set("Retry-After", "3")
			writer.Header().Set("X-Correlation-Id", "correlation-1")
			writeJSON(writer, http.StatusTooManyRequests, `{"error":"rate_limit_error","message":"slow"}`)
		case "2":
			http.Redirect(writer, request, target.URL+"/capture", http.StatusFound)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server)
	_, err := client.GetPost(context.Background(), "1", socialhub.WithRequestID("caller-request"))
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || !errors.Is(err, socialhub.ErrRateLimited) || platformErr.RetryAfter != 3*time.Second ||
		platformErr.RequestID != "correlation-1" {
		t.Fatalf("rate limit=%#v", platformErr)
	}
	if _, err := client.GetPost(context.Background(), "2"); err == nil {
		t.Fatal("cross-origin redirect must surface an error")
	}
	if redirected.Load() != 0 {
		t.Fatalf("cross-origin target received %d requests", redirected.Load())
	}
}

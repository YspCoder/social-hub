package bluesky

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestXRPCErrorMapping(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		platformCode string
		want         error
		class        socialhub.ErrorClass
	}{
		{"bad request", http.StatusBadRequest, "InvalidRequest", socialhub.ErrInvalidArgument, socialhub.ClassPermanent},
		{"unprocessable", http.StatusUnprocessableEntity, "InvalidRequest", socialhub.ErrInvalidArgument, socialhub.ClassPermanent},
		{"unauthorized", http.StatusUnauthorized, "AuthenticationRequired", socialhub.ErrUnauthenticated, socialhub.ClassUserAction},
		{"forbidden", http.StatusForbidden, "Forbidden", socialhub.ErrPermissionDenied, socialhub.ClassUserAction},
		{"not found", http.StatusNotFound, "NotFound", socialhub.ErrNotFound, socialhub.ClassPermanent},
		{"gone", http.StatusGone, "RecordNotFound", socialhub.ErrNotFound, socialhub.ClassPermanent},
		{"conflict", http.StatusConflict, "InvalidSwap", socialhub.ErrConflict, socialhub.ClassPermanent},
		{"rate limited", http.StatusTooManyRequests, "RateLimitExceeded", socialhub.ErrRateLimited, socialhub.ClassRetryable},
		{"server", http.StatusServiceUnavailable, "UpstreamFailure", socialhub.ErrUnavailable, socialhub.ClassRetryable},
		{"MFA", http.StatusBadRequest, "AuthFactorTokenRequired", socialhub.ErrApprovalRequired, socialhub.ClassUserAction},
		{"expired token override", http.StatusBadRequest, "ExpiredToken", socialhub.ErrUnauthenticated, socialhub.ClassUserAction},
		{"account suspended", http.StatusBadRequest, "AccountSuspended", socialhub.ErrPermissionDenied, socialhub.ClassUserAction},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			header := make(http.Header)
			header.Set("X-Request-ID", "request-1")
			if test.status == http.StatusTooManyRequests {
				header.Set("Retry-After", "7")
			}
			body := []byte(`{"error":"` + test.platformCode + `","message":"try again later"}`)
			err := decodeHTTPError(test.status, header, body)
			var typed *socialhub.Error
			if !errors.As(err, &typed) || !errors.Is(err, test.want) || typed.Class != test.class || typed.Platform != "bluesky" || typed.Product != productName || typed.HTTPStatus != test.status || typed.PlatformCode != test.platformCode || typed.PlatformMessage != "try again later" || typed.RequestID != "request-1" {
				t.Fatalf("error=%#v", err)
			}
			if test.status == http.StatusTooManyRequests && typed.RetryAfter != 7*time.Second {
				t.Fatalf("retry after=%v", typed.RetryAfter)
			}
		})
	}
}

func TestRateLimitHeadersAndErrorSanitization(t *testing.T) {
	future := time.Now().Add(10 * time.Second).UTC().Truncate(time.Second)
	httpDate := make(http.Header)
	httpDate.Set("Retry-After", future.Format(http.TimeFormat))
	unixReset := make(http.Header)
	unixReset.Set("RateLimit-Reset", strconv.FormatInt(future.Unix(), 10))
	for name, header := range map[string]http.Header{
		"HTTP date": httpDate, "unix reset": unixReset,
	} {
		t.Run(name, func(t *testing.T) {
			delay := retryAfter(header)
			if delay < 8*time.Second || delay > 11*time.Second {
				t.Fatalf("retry delay=%v", delay)
			}
		})
	}
	past := make(http.Header)
	past.Set("Retry-After", "invalid")
	past.Set("RateLimit-Reset", "0")
	if delay := retryAfter(past); delay != 0 {
		t.Fatalf("past reset delay=%v", delay)
	}
	if got := xrpcErrorCode("bad code: value"); got != "" {
		t.Fatalf("unsafe code=%q", got)
	}
	message := strings.Repeat("界", 600)
	err := decodeHTTPError(http.StatusTeapot, nil, []byte(`{"error":"BadTea","message":"`+message+`"}`))
	var typed *socialhub.Error
	if !errors.As(err, &typed) || len([]rune(typed.PlatformMessage)) != 512 || typed.Code != socialhub.CodePlatformError {
		t.Fatalf("bounded error=%#v", err)
	}
}

func TestATRecordURIParsing(t *testing.T) {
	parsed, err := parseRecordURI("at://did:plc:alice/app.bsky.feed.post/pre:fix_~1.2-3")
	if err != nil || parsed.Repo != "did:plc:alice" || parsed.Collection != collectionPost || parsed.RecordKey != "pre:fix_~1.2-3" {
		t.Fatalf("parsed=%#v error=%v", parsed, err)
	}
	invalid := []string{
		"", "https://bsky.app/profile/alice", "at://alice.test/app.bsky.feed.post/one",
		"at://did:plc:alice/app.bsky.feed.post", "at://did:plc:alice/app.bsky.feed.post/one/extra",
		"at://did:plc:alice/app..bsky/one", "at://did:plc:alice/app.bsky.feed.post/alpha%2Fbeta",
		"at://did:plc:alice/app.bsky.feed.post/bad@key", "at://did:plc:alice/app.bsky.feed.post/one?query=1",
	}
	for _, value := range invalid {
		if _, err := parseRecordURI(value); err == nil {
			t.Fatalf("URI %q should fail", value)
		}
	}
}

package kuaishou

import (
	"errors"
	"net/http"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestRateLimitErrorMapping(t *testing.T) {
	err := resultError(400002, "api rate limited", "fixture", http.StatusOK, http.Header{"Retry-After": {"3"}})
	if !errors.Is(err, socialhub.ErrRateLimited) {
		t.Fatalf("error=%v", err)
	}
	var platformError *socialhub.Error
	if !errors.As(err, &platformError) || !platformError.Retryable() || platformError.RetryAfter.Seconds() != 3 || platformError.PlatformCode != "400002" {
		t.Fatalf("platform error=%#v", platformError)
	}
}

func TestAmbiguousDailyLimitIsNotAutomaticallyRetryable(t *testing.T) {
	err := resultError(100100402, "user banned or daily quota reached", "fixture", http.StatusOK, nil)
	var platformError *socialhub.Error
	if !errors.Is(err, socialhub.ErrRateLimited) || !errors.As(err, &platformError) || platformError.Retryable() {
		t.Fatalf("error=%v", err)
	}
}

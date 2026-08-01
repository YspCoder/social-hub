package socialhub

import (
	"errors"
	"strings"
	"testing"
)

func TestErrorCategoryAndSanitization(t *testing.T) {
	t.Parallel()
	err := &Error{
		Code:            CodeRateLimited,
		Class:           ClassRetryable,
		Platform:        "x",
		Op:              "publish",
		PlatformCode:    "too_many_requests",
		PlatformMessage: "secret-token-must-not-appear",
		RequestID:       "req-1",
	}

	if !errors.Is(err, ErrRateLimited) {
		t.Fatal("rate-limit error should match ErrRateLimited")
	}
	if !err.Retryable() {
		t.Fatal("rate-limit error should be retryable")
	}
	if strings.Contains(err.Error(), err.PlatformMessage) {
		t.Fatalf("error leaked platform message: %q", err.Error())
	}
}

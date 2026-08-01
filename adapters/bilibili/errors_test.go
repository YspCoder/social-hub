package bilibili

import (
	"errors"
	"net/http"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestApprovalAndRateErrors(t *testing.T) {
	approval := responseEnvelope[jsonEmpty]{Code: 127007, Message: "missing interface approval"}.Err("fixture", http.StatusOK, nil)
	if !errors.Is(approval, socialhub.ErrApprovalRequired) {
		t.Fatalf("approval error=%v", approval)
	}
	rate := responseEnvelope[jsonEmpty]{Code: 123026, Message: "submit too quickly"}.Err("fixture", http.StatusOK, nil)
	var platformError *socialhub.Error
	if !errors.Is(rate, socialhub.ErrRateLimited) || !errors.As(rate, &platformError) || !platformError.Retryable() {
		t.Fatalf("rate error=%v", rate)
	}
}

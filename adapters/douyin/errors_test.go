package douyin

import (
	"errors"
	"net/http"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestErrorClassification(t *testing.T) {
	t.Parallel()
	tests := []struct {
		code int64
		want error
	}{
		{2100005, socialhub.ErrInvalidArgument},
		{2190008, socialhub.ErrUnauthenticated},
		{2190004, socialhub.ErrApprovalRequired},
		{2190001, socialhub.ErrRateLimited},
		{2100004, socialhub.ErrUnavailable},
	}
	for _, test := range tests {
		err := (APIResponse{ErrorCode: flexibleInt64(test.code), Description: "provider detail"}).Err("test", responseExtra{}, http.StatusOK, nil)
		if !errors.Is(err, test.want) {
			t.Errorf("code %d: error=%v want=%v", test.code, err, test.want)
		}
	}
}

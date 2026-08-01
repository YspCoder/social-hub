package officialaccount

import (
	"errors"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestBusinessErrorClassification(t *testing.T) {
	t.Parallel()
	tests := []struct {
		code int
		want error
	}{
		{40014, socialhub.ErrUnauthenticated},
		{40003, socialhub.ErrInvalidArgument},
		{48001, socialhub.ErrPermissionDenied},
		{45009, socialhub.ErrRateLimited},
		{-1, socialhub.ErrUnavailable},
	}
	for _, test := range tests {
		err := (APIResponse{ErrCode: test.code, ErrMsg: "provider detail"}).Err("test")
		if !errors.Is(err, test.want) {
			t.Errorf("code %d: error = %v, want %v", test.code, err, test.want)
		}
	}
}

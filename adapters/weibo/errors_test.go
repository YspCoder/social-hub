package weibo

import (
	"errors"
	"net/http"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestErrorClassification(t *testing.T) {
	t.Parallel()
	tests := []struct {
		code int
		want error
	}{
		{21314, socialhub.ErrUnauthenticated},
		{10006, socialhub.ErrInvalidArgument},
		{10014, socialhub.ErrPermissionDenied},
		{10023, socialhub.ErrRateLimited},
		{20003, socialhub.ErrNotFound},
	}
	for _, test := range tests {
		err := (APIError{Code: test.code, Message: "provider detail"}).Err("test", http.StatusBadRequest, nil)
		if !errors.Is(err, test.want) {
			t.Errorf("code %d: error=%v want=%v", test.code, err, test.want)
		}
	}
}

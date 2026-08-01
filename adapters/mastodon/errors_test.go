package mastodon

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestMastodonHTTPErrorMapping(t *testing.T) {
	tests := []struct {
		status int
		want   error
		class  socialhub.ErrorClass
	}{
		{http.StatusUnauthorized, socialhub.ErrUnauthenticated, socialhub.ClassUserAction},
		{http.StatusForbidden, socialhub.ErrPermissionDenied, socialhub.ClassUserAction},
		{http.StatusUnprocessableEntity, socialhub.ErrInvalidArgument, socialhub.ClassPermanent},
		{http.StatusTooManyRequests, socialhub.ErrRateLimited, socialhub.ClassRetryable},
		{http.StatusServiceUnavailable, socialhub.ErrUnavailable, socialhub.ClassRetryable},
	}
	for _, test := range tests {
		header := http.Header{"X-Request-Id": {"request-1"}}
		if test.status == http.StatusTooManyRequests {
			header.Set("Retry-After", "7")
		}
		err := decodeHTTPError(test.status, header, []byte(`{"error":"rate_limit","error_description":"try again later"}`))
		var typed *socialhub.Error
		if !errors.As(err, &typed) || !errors.Is(err, test.want) || typed.Class != test.class || typed.RequestID != "request-1" || typed.PlatformCode != "rate_limit" || typed.PlatformMessage != "try again later" {
			t.Fatalf("status=%d error=%#v", test.status, err)
		}
		if test.status == http.StatusTooManyRequests && typed.RetryAfter != 7*time.Second {
			t.Fatalf("retry after=%v", typed.RetryAfter)
		}
	}
}

func TestMastodonPaginationAndMessageBounds(t *testing.T) {
	header := http.Header{"Link": {`<https://social.example/api/v1/timelines/home?max_id=next-1>; rel="next", <https://social.example/api/v1/timelines/home?since_id=previous>; rel="prev"`}}
	cursor := nextCursor(header)
	if cursor == nil || *cursor != "next-1" {
		t.Fatalf("cursor=%v", cursor)
	}
	if cursor := nextCursor(http.Header{"Link": {`<://bad>; rel="next"`}}); cursor != nil {
		t.Fatalf("invalid cursor=%v", cursor)
	}
	if got := boundedMessage("abcdef", 3); got != "abc" {
		t.Fatalf("bounded message=%q", got)
	}
	if got := oauthErrorCode("not valid"); got != "" {
		t.Fatalf("OAuth error code=%q", got)
	}
}

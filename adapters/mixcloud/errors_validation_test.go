package mixcloud

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestMixcloudErrorClassification(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		header     http.Header
		body       string
		want       error
		retryable  bool
		retryAfter time.Duration
	}{
		{"rate limit header", http.StatusForbidden, http.Header{"Retry-After": {"7"}}, `{"error":{"type":"RateLimitException","message":"slow down","retry_after":9}}`, socialhub.ErrRateLimited, true, 7 * time.Second},
		{"rate limit body", http.StatusForbidden, nil, `{"error":{"type":"RateLimitException","message":"slow down","retry_after":9}}`, socialhub.ErrRateLimited, true, 9 * time.Second},
		{"not found type", http.StatusBadRequest, nil, `{"error":{"type":"ResourceNotFoundException","message":"missing"}}`, socialhub.ErrNotFound, false, 0},
		{"OAuth type", http.StatusForbidden, nil, `{"error":{"type":"OAuthException","message":"bad token"}}`, socialhub.ErrUnauthenticated, false, 0},
		{"bad request", http.StatusUnprocessableEntity, nil, `{}`, socialhub.ErrInvalidArgument, false, 0},
		{"unauthorized", http.StatusUnauthorized, nil, `{}`, socialhub.ErrUnauthenticated, false, 0},
		{"forbidden", http.StatusForbidden, nil, `{}`, socialhub.ErrPermissionDenied, false, 0},
		{"not found", http.StatusNotFound, nil, `{}`, socialhub.ErrNotFound, false, 0},
		{"conflict", http.StatusConflict, nil, `{}`, socialhub.ErrConflict, false, 0},
		{"429", http.StatusTooManyRequests, nil, `{}`, socialhub.ErrRateLimited, true, 0},
		{"server", http.StatusServiceUnavailable, nil, `{}`, socialhub.ErrUnavailable, true, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := decodeHTTPError(test.status, test.header, []byte(test.body))
			var platformErr *socialhub.Error
			if !errors.Is(err, test.want) || !errors.As(err, &platformErr) || platformErr.Retryable() != test.retryable || platformErr.RetryAfter != test.retryAfter {
				t.Fatalf("error=%#v", err)
			}
		})
	}
	if got := parseRetryAfter("-1"); got != 0 {
		t.Fatalf("negative Retry-After=%s", got)
	}
	if got := parseRetryAfter("not-a-number"); got != 0 {
		t.Fatalf("bad Retry-After=%s", got)
	}
}

func TestValidationAndPagingHelpers(t *testing.T) {
	if !validSegment("DJ_name-1", 100) || validSegment("bad.name", 100) || validSegment("..", 100) || validSegment(strings.Repeat("x", 101), 100) {
		t.Fatal("segment validation failed")
	}
	if username, key, ok := parseUserKey("/DJ_name-1/"); !ok || username != "DJ_name-1" || key != "/DJ_name-1/" {
		t.Fatalf("user key username=%q key=%q ok=%t", username, key, ok)
	}
	if _, _, _, ok := parseCloudcastKey("/one/two/three/"); ok {
		t.Fatal("invalid Cloudcast key accepted")
	}
	if _, slug, key, ok := parseCloudcastKey("/dj/супервсё2/"); !ok || slug != "супервсё2" || key != "/dj/супервсё2/" {
		t.Fatalf("Unicode Cloudcast slug=%q key=%q ok=%t", slug, key, ok)
	}
	if !validCommentKey("/comments/cr/64/c123/") || validCommentKey("/comments/") || validCommentKey("/other/id/") {
		t.Fatal("comment key validation failed")
	}
	if !validFilename("show.mp3") || validFilename("../show.mp3") || validFilename(".") {
		t.Fatal("filename validation failed")
	}
	if !validOAuthRedirect("app.example") || !validOAuthRedirect("custom://callback") || validOAuthRedirect("https://user:pass@example.test/cb") {
		t.Fatal("OAuth redirect validation failed")
	}
	base, _ := url.Parse("https://api.mixcloud.com")
	next, previous, err := pageCursors(Paging{
		Next:     "https://api.mixcloud.com/dj/cloudcasts/?limit=2&offset=4",
		Previous: "https://api.mixcloud.com/dj/cloudcasts/?limit=2&offset=0",
	}, base)
	if err != nil || pointerValue(next) != "4" || pointerValue(previous) != "0" {
		t.Fatalf("cursors next=%v previous=%v err=%v", next, previous, err)
	}
	if _, _, err := pageCursors(Paging{Next: "https://api.mixcloud.com/dj/?limit=2"}, base); err == nil {
		t.Fatal("missing paging offset accepted")
	}
	if _, _, err := pageCursors(Paging{Previous: "https://evil.example/?offset=0"}, base); err == nil {
		t.Fatal("foreign previous page accepted")
	}
	paging, err := sanitizedPaging(Paging{Next: "https://api.mixcloud.com/search/?offset=2&access_token=secret"}, base)
	if err != nil || strings.Contains(paging.Next, "secret") {
		t.Fatalf("sanitized paging=%#v err=%v", paging, err)
	}
	if _, err := sanitizedPaging(Paging{Next: "https://evil.example/?offset=2"}, base); err == nil {
		t.Fatal("foreign paging URL sanitized")
	}
	if got := boundedMessage(strings.Repeat("界", 10), 3); got != "界界界" {
		t.Fatalf("bounded=%q", got)
	}
}

func TestMappingGuardsAndEmptyPointers(t *testing.T) {
	client := &Client{accountID: "creator", clock: fixedClock{now: testNow}}
	if _, err := client.mapUser(User{Key: "/one/", Username: "two"}); err == nil {
		t.Fatal("mismatched user mapped")
	}
	if _, err := client.mapCloudcast(Cloudcast{Key: "/one/show/", Slug: "show", User: User{Key: "/two/", Username: "two"}}); err == nil {
		t.Fatal("mismatched owner mapped")
	}
	if _, err := client.mapComment("/one/show/", Comment{Key: "bad", User: User{Key: "/one/"}}); err == nil {
		t.Fatal("invalid comment mapped")
	}
	if _, err := client.mapComment("/one/show/", Comment{Key: "/comments/cr/c1/", User: User{Key: "bad/name"}}); err == nil {
		t.Fatal("invalid comment user mapped")
	}
	if stringPointer("") != nil || timePointer(time.Time{}) != nil || durationPointer(0) != nil {
		t.Fatal("empty pointer helpers returned values")
	}
	if err := rejectRedirect(nil, nil); !errors.Is(err, http.ErrUseLastResponse) {
		t.Fatalf("redirect=%v", err)
	}
}

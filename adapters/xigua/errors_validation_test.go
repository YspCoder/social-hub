package xigua

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestErrorClassificationAndDetails(t *testing.T) {
	t.Parallel()
	tests := []struct {
		platformCode int64
		status       int
		want         error
		class        socialhub.ErrorClass
	}{
		{10002, http.StatusOK, socialhub.ErrInvalidArgument, socialhub.ClassPermanent},
		{10008, http.StatusOK, socialhub.ErrUnauthenticated, socialhub.ClassUserAction},
		{10004, http.StatusOK, socialhub.ErrApprovalRequired, socialhub.ClassUserAction},
		{2100007, http.StatusOK, socialhub.ErrPermissionDenied, socialhub.ClassUserAction},
		{2190001, http.StatusOK, socialhub.ErrRateLimited, socialhub.ClassRetryable},
		{2114007, http.StatusOK, socialhub.ErrRateLimited, socialhub.ClassUserAction},
		{2100004, http.StatusOK, socialhub.ErrUnavailable, socialhub.ClassRetryable},
		{0, http.StatusNotFound, socialhub.ErrNotFound, socialhub.ClassPermanent},
		{0, http.StatusConflict, socialhub.ErrConflict, socialhub.ClassPermanent},
		{0, http.StatusTooManyRequests, socialhub.ErrRateLimited, socialhub.ClassRetryable},
		{0, http.StatusInternalServerError, socialhub.ErrUnavailable, socialhub.ClassRetryable},
	}
	for _, test := range tests {
		name := fmt.Sprintf("%d/%d", test.platformCode, test.status)
		t.Run(name, func(t *testing.T) {
			code, class := classifyError(test.platformCode, test.status)
			err := platformError("test", code, class, nil)
			if !errors.Is(err, test.want) || class != test.class {
				t.Fatalf("code=%s class=%s err=%v", code, class, err)
			}
		})
	}

	header := http.Header{"Retry-After": {"9"}, "X-Tt-Logid": {strings.Repeat("r", 300)}}
	err := responseError(
		apiResponse{ErrorCode: 10004, Description: strings.Repeat("m", 600)},
		responseExtra{SubErrorCode: 7}, "publish", http.StatusOK, header,
	)
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.PlatformCode != "10004/7" || len(platformErr.PlatformMessage) != 512 || len(platformErr.RequestID) != 256 || platformErr.RetryAfter != 9*time.Second || platformErr.ApprovalURL == "" {
		t.Fatalf("error=%#v", platformErr)
	}
}

func TestHTTPErrorDecodingAndBounds(t *testing.T) {
	t.Parallel()
	header := make(http.Header)
	header.Set("Retry-After", "12")
	header.Set("X-Request-ID", "request-1")
	err := decodeHTTPError(http.StatusTooManyRequests, header, []byte(`{"extra":{"error_code":2190001,"description":"slow down","log_id":"log-1"}}`))
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || !errors.Is(err, socialhub.ErrRateLimited) || platformErr.PlatformCode != "2190001" || platformErr.RequestID != "log-1" || platformErr.RetryAfter != 12*time.Second {
		t.Fatalf("decoded error=%#v", err)
	}
	err = decodeHTTPError(http.StatusUnauthorized, header, []byte("not-json"))
	if !errors.Is(err, socialhub.ErrUnauthenticated) || !errors.As(err, &platformErr) || platformErr.RequestID != "request-1" {
		t.Fatalf("fallback error=%#v", err)
	}
	if parseRetryAfter("0") != 0 || parseRetryAfter("bad") != 0 || parseRetryAfter("999999") != 0 || parseRetryAfter("5") != 5*time.Second {
		t.Fatal("retry-after bounds failed")
	}
	if bounded("abc", 3) != "abc" || bounded("abcd", 3) != "abc" || bounded("abcd", 0) != "" {
		t.Fatal("bounded failed")
	}
}

func TestFlexibleIntegerAndValidationHelpers(t *testing.T) {
	t.Parallel()
	for _, fixture := range []struct {
		input string
		want  flexibleInt64
	}{
		{`12`, 12}, {`"13"`, 13}, {`null`, 0}, {``, 0},
	} {
		var value flexibleInt64
		if fixture.input != "" {
			if err := json.Unmarshal([]byte(fixture.input), &value); err != nil {
				t.Fatal(err)
			}
		} else if err := value.UnmarshalJSON(nil); err != nil {
			t.Fatal(err)
		}
		if value != fixture.want {
			t.Fatalf("input=%q value=%d", fixture.input, value)
		}
	}
	var value flexibleInt64
	if json.Unmarshal([]byte(`"bad"`), &value) == nil || json.Unmarshal([]byte(`{}`), &value) == nil {
		t.Fatal("invalid flexible integers accepted")
	}

	if !validEndpoint("https://example.com/api") || validEndpoint("https://user@example.com") || validEndpoint("https://example.com/api?x=1") || validEndpoint("https://example.com/a/../b") {
		t.Fatal("endpoint validation failed")
	}
	if !validRedirectURI("https://app.example/callback?x=1") || validRedirectURI("https://app.example/callback#fragment") {
		t.Fatal("redirect validation failed")
	}
	if !validOpaque("opaque", 6) || validOpaque(" opaque", 20) || validOpaque("a\nb", 20) || validOpaque("toolong", 3) {
		t.Fatal("opaque validation failed")
	}
	if !validTitle("valid title") || validTitle("four") || validTitle(strings.Repeat("x", 31)) || !validSummary(strings.Repeat("x", 400)) || validSummary(strings.Repeat("x", 401)) || validSummary(string([]byte{0xff})) {
		t.Fatal("publication text validation failed")
	}
	if !validScopes([]string{"user_info", "xigua.video:create"}) || validScopes(nil) || validScopes([]string{"bad scope"}) {
		t.Fatal("scope validation failed")
	}
	if value, err := pageSize(0); err != nil || value != defaultPageSize {
		t.Fatalf("default page size=%d err=%v", value, err)
	}
	if _, err := pageSize(-1); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("page size error=%v", err)
	}
	if value, err := parseCursor("42"); err != nil || value != 42 {
		t.Fatalf("cursor=%d err=%v", value, err)
	}
	for _, cursor := range []string{"-1", "01", "abc"} {
		if _, err := parseCursor(cursor); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("cursor %q error=%v", cursor, err)
		}
	}
	if value, err := parseCursor(""); err != nil || value != 0 {
		t.Fatalf("empty cursor=%d err=%v", value, err)
	}
	if !contains([]string{"a", "b"}, "b") || contains([]string{"a"}, "b") {
		t.Fatal("contains failed")
	}
}

func TestOAuthErrorSanitizationAndScopeSplitting(t *testing.T) {
	t.Parallel()
	cause := errors.New("connection refused")
	wrapped := fmt.Errorf("outer: %w", &url.Error{Op: "Post", URL: "https://example.com", Err: cause})
	if sanitized := sanitizeOAuthError(wrapped); sanitized != cause {
		t.Fatalf("sanitized=%v", sanitized)
	}
	plain := errors.New("plain")
	if sanitizeOAuthError(plain) != plain {
		t.Fatal("plain error changed")
	}
	scopes := splitScopes("user_info,xigua.video.data\nother")
	if len(scopes) != 3 || scopes[2] != "other" {
		t.Fatalf("scopes=%v", scopes)
	}
}

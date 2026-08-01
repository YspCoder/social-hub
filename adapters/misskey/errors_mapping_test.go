package misskey

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func errorCode(err error) socialhub.ErrorCode {
	var platformErr *socialhub.Error
	if errors.As(err, &platformErr) {
		return platformErr.Code
	}
	return ""
}

func TestHTTPErrorMapping(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		platformCode string
		kind         string
		want         error
		class        socialhub.ErrorClass
	}{
		{name: "credential", status: 400, platformCode: "CREDENTIAL_REQUIRED", want: socialhub.ErrUnauthenticated, class: socialhub.ClassUserAction},
		{name: "approval", status: 400, platformCode: "PERMISSION_DENIED", want: socialhub.ErrApprovalRequired, class: socialhub.ClassUserAction},
		{name: "access", status: 400, platformCode: "ACCESS_DENIED", want: socialhub.ErrPermissionDenied, class: socialhub.ClassUserAction},
		{name: "rate", status: 400, platformCode: "RATE_LIMIT_EXCEEDED", want: socialhub.ErrRateLimited, class: socialhub.ClassRetryable},
		{name: "missing", status: 400, platformCode: "NO_SUCH_NOTE", want: socialhub.ErrNotFound, class: socialhub.ClassPermanent},
		{name: "conflict", status: 400, platformCode: "ALREADY_REACTED", want: socialhub.ErrConflict, class: socialhub.ClassPermanent},
		{name: "invalid", status: 400, platformCode: "INVALID_PARAM", want: socialhub.ErrInvalidArgument, class: socialhub.ClassPermanent},
		{name: "internal", status: 400, platformCode: "INTERNAL_ERROR", want: socialhub.ErrUnavailable, class: socialhub.ClassRetryable},
		{name: "server kind", status: 400, kind: "server", want: socialhub.ErrUnavailable, class: socialhub.ClassRetryable},
		{name: "permission kind", status: 400, kind: "permission", want: socialhub.ErrPermissionDenied, class: socialhub.ClassUserAction},
		{name: "http unauthorized", status: http.StatusUnauthorized, want: socialhub.ErrUnauthenticated, class: socialhub.ClassUserAction},
		{name: "http forbidden", status: http.StatusForbidden, want: socialhub.ErrPermissionDenied, class: socialhub.ClassUserAction},
		{name: "http not found", status: http.StatusNotFound, want: socialhub.ErrNotFound, class: socialhub.ClassPermanent},
		{name: "http conflict", status: http.StatusConflict, want: socialhub.ErrConflict, class: socialhub.ClassPermanent},
		{name: "http rate", status: http.StatusTooManyRequests, want: socialhub.ErrRateLimited, class: socialhub.ClassRetryable},
		{name: "http server", status: http.StatusServiceUnavailable, want: socialhub.ErrUnavailable, class: socialhub.ClassRetryable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]any{"error": map[string]any{
				"message": "bounded message", "code": test.platformCode, "kind": test.kind,
			}})
			header := http.Header{"X-Request-Id": {"request-1"}, "Retry-After": {"7"}}
			err := decodeHTTPError(test.status, header, body)
			var typed *socialhub.Error
			if !errors.As(err, &typed) || !errors.Is(err, test.want) || typed.Class != test.class || typed.RequestID != "request-1" {
				t.Fatalf("error=%#v", err)
			}
			if test.want == socialhub.ErrRateLimited && typed.RetryAfter != 7*time.Second {
				t.Fatalf("retry after=%v", typed.RetryAfter)
			}
		})
	}
}

func TestErrorAndValidationHelpers(t *testing.T) {
	if retryAfter("-1") != 0 || retryAfter("999999999") != 0 || retryAfter("bad") != 0 {
		t.Fatal("invalid retry-after accepted")
	}
	if got := requestID(http.Header{"X-Correlation-Id": {"correlation"}}); got != "correlation" {
		t.Fatalf("request ID=%q", got)
	}
	if got := boundedMessage("abcdef", 3); got != "abc" {
		t.Fatalf("bounded=%q", got)
	}
	if !validHTTPURL("https://example.test/path") || validHTTPURL("file:///tmp") || validHTTPURL("https://user@example.test") {
		t.Fatal("URL validation mismatch")
	}
	if !validSessionID("C1F6D42B-468B-4FD2-8274-E58ABDEDEF6F") || validSessionID("c1f6d42b-468b-4fd2-8274-e58abdedef6g") {
		t.Fatal("session validation mismatch")
	}
	if err := unsupported("op", "message"); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("unsupported=%v", err)
	}
}

func TestUserAndRenoteMapping(t *testing.T) {
	client := &Client{accountID: "main", instanceURL: "https://social.example.test", clock: fixedClock{now: testNow}}
	userJSON, _ := json.Marshal(testUser("user-1"))
	var userInput misskeyUser
	_ = json.Unmarshal(userJSON, &userInput)
	userInput.Host, userInput.IsBot, userInput.IsCat = stringPointer("remote.example"), true, true
	user, err := client.mapUser(userInput)
	if err != nil || user.Username == nil || *user.Username != "alice@remote.example" || user.AccountType == nil || *user.AccountType != "bot" || len(user.Extensions) != 1 {
		t.Fatalf("user=%#v err=%v", user, err)
	}
	if _, err := client.mapUser(misskeyUser{}); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("invalid user=%v", err)
	}

	originalJSON, _ := json.Marshal(testNote("original", "original text"))
	var original misskeyNote
	_ = json.Unmarshal(originalJSON, &original)
	renoteJSON, _ := json.Marshal(testNote("renote", "temporary"))
	var renote misskeyNote
	_ = json.Unmarshal(renoteJSON, &renote)
	renote.Text, renote.RenoteID, renote.Renote = nil, stringPointer("original"), &original
	post, err := client.mapNote(renote)
	if err != nil || post.Text == nil || *post.Text != "original text" || len(post.Relations) != 1 || post.Relations[0].Type != socialhub.RelationRepost {
		t.Fatalf("renote=%#v err=%v", post, err)
	}
	invalid := original
	invalid.Visibility = "private"
	if _, err := client.mapNote(invalid); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("invalid note=%v", err)
	}
}

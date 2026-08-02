package peertube

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestHTTPErrorClassification(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		code   socialhub.ErrorCode
		class  socialhub.ErrorClass
	}{
		{"invalid grant", http.StatusBadRequest, `{"code":"invalid_grant","detail":"bad credentials"}`, socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{"quota", http.StatusRequestEntityTooLarge, `{"code":"quota_reached"}`, socialhub.CodeRateLimited, socialhub.ClassUserAction},
		{"typed not found", http.StatusNotFound, `{"code":"video_not_found"}`, socialhub.CodeNotFound, socialhub.ClassPermanent},
		{"validation", http.StatusUnprocessableEntity, `{"detail":"bad input"}`, socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{"forbidden", http.StatusForbidden, `{}`, socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{"conflict", http.StatusConflict, `{}`, socialhub.CodeConflict, socialhub.ClassPermanent},
		{"timeout", http.StatusRequestTimeout, `{}`, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{"rate limit", http.StatusTooManyRequests, `{}`, socialhub.CodeRateLimited, socialhub.ClassRetryable},
		{"server", http.StatusServiceUnavailable, `{}`, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{"other", http.StatusTeapot, `{}`, socialhub.CodePlatformError, socialhub.ClassPermanent},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			header := http.Header{}
			header.Set("X-Request-ID", "request-1")
			header.Set("Retry-After", "7")
			err := decodeHTTPError(test.status, header, []byte(test.body))
			var platformErr *socialhub.Error
			if !errors.As(err, &platformErr) || platformErr.Code != test.code || platformErr.Class != test.class || platformErr.RequestID != "request-1" || platformErr.RetryAfter != 7*time.Second {
				t.Fatalf("error=%#v", platformErr)
			}
		})
	}
	typeURL := decodeHTTPError(http.StatusForbidden, nil, []byte(`{"type":"https://docs.example/errors/private_video","detail":"private"}`))
	var platformErr *socialhub.Error
	if !errors.As(typeURL, &platformErr) || platformErr.PlatformCode != "private_video" {
		t.Fatalf("type URL error=%#v", platformErr)
	}
	if parseRetryAfter("invalid") != 0 || parseRetryAfter("-1") != 0 || parseRetryAfter("999999") != 0 {
		t.Fatal("invalid Retry-After must be ignored")
	}
	if boundedMessage(strings.Repeat("x", 5), 3) != "xxx" || firstNonEmpty("", " value ") != " value " {
		t.Fatal("bounded message helpers are wrong")
	}
}

func TestPaginationAndValidationHelpers(t *testing.T) {
	query, start, limit, err := pageQuery("list", "12", 500)
	if err != nil || start != 12 || limit != 100 || query.Get("start") != "12" || query.Get("count") != "100" {
		t.Fatalf("query=%v start=%d limit=%d err=%v", query, start, limit, err)
	}
	for _, input := range []struct {
		cursor string
		max    int
	}{{"bad", 1}, {"-1", 1}, {"", -1}} {
		if _, _, _, err := pageQuery("list", input.cursor, input.max); errorCode(err) != socialhub.CodeInvalidArgument {
			t.Fatalf("cursor=%q max=%d err=%v", input.cursor, input.max, err)
		}
	}
	next, previous, more, err := pageCursors(20, 10, 5, 5)
	if err != nil || dereference(next) != "15" || dereference(previous) != "5" || !more {
		t.Fatalf("next=%v previous=%v more=%t err=%v", next, previous, more, err)
	}
	if _, _, _, err := pageCursors(-1, 0, 0, 0); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("negative total=%v", err)
	}
	if !validResourceID(testVideoUUID) || validResourceID("bad/id") || !validActorHandle("creator@example.org") || validActorHandle("bad handle") {
		t.Fatal("resource validation is wrong")
	}
	if !validFilename("clip.mp4") || validFilename("../clip.mp4") || validFilename("bad\nname.mp4") {
		t.Fatal("filename validation is wrong")
	}
	if validateMIME("video/mp4") != nil || validateMIME("application/octet-stream") != nil || validateMIME("text/plain") == nil {
		t.Fatal("MIME validation is wrong")
	}
	if validateTags([]string{"one", "two"}) != nil || validateTags([]string{"same", "same"}) == nil || validateTags([]string{"x"}) == nil || validateTags([]string{"1", "2", "3", "4", "5", "6"}) == nil {
		t.Fatal("tag validation is wrong")
	}
	if validateSort("-views", "name", "-views") != nil || validateSort("bad", "name") == nil {
		t.Fatal("sort validation is wrong")
	}
}

func TestMappingStateAndFailureBranches(t *testing.T) {
	client := &Client{accountID: "main", instanceURL: "https://video.example", clock: fixedClock{testNow}}
	if _, err := client.mapAccount(Account{}); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("account error=%v", err)
	}
	if _, err := client.mapVideo(Video{}); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("video error=%v", err)
	}
	if _, err := client.mapComment("video", VideoComment{}); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("comment error=%v", err)
	}
	states := []struct {
		input int
		media socialhub.MediaState
		post  socialhub.PublishState
	}{
		{1, socialhub.MediaStateReady, socialhub.PublishStatePublished},
		{3, socialhub.MediaStateUploading, socialhub.PublishStatePending},
		{4, socialhub.MediaStateCreated, socialhub.PublishStatePending},
		{7, socialhub.MediaStateFailed, socialhub.PublishStateFailed},
		{2, socialhub.MediaStateProcessing, socialhub.PublishStatePending},
	}
	for _, state := range states {
		media, post := videoStates(state.input)
		if media != state.media || post != state.post {
			t.Fatalf("state %d => %s/%s", state.input, media, post)
		}
	}
	for id, expected := range map[int]string{1: "public", 2: "unlisted", 3: "private", 4: "internal", 5: "password_protected", 9: ""} {
		if actual := privacyName(NumberConstant{ID: id}); actual != expected {
			t.Fatalf("privacy %d=%q", id, actual)
		}
	}
	if client.resolveURL("/avatar.png") != "https://video.example/avatar.png" || client.resolveURL(":bad") != "" || stringPointer(" ") != nil {
		t.Fatal("mapping URL/string helpers are wrong")
	}
}

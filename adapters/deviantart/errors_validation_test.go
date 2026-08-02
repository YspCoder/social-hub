package deviantart

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
		name         string
		status       int
		body         string
		code         socialhub.ErrorCode
		class        socialhub.ErrorClass
		platformCode string
	}{
		{"invalid", http.StatusBadRequest, `{"error":"invalid_request","error_code":2,"error_description":"bad field"}`, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, "invalid_request:2"},
		{"auth", http.StatusBadRequest, `{"error":"invalid_grant"}`, socialhub.CodeUnauthenticated, socialhub.ClassUserAction, "invalid_grant"},
		{"unverified", http.StatusForbidden, `{"error":"unverified_account"}`, socialhub.CodePermissionDenied, socialhub.ClassUserAction, "unverified_account"},
		{"not found", http.StatusNotFound, `{}`, socialhub.CodeNotFound, socialhub.ClassPermanent, ""},
		{"conflict", http.StatusConflict, `{}`, socialhub.CodeConflict, socialhub.ClassPermanent, ""},
		{"rate", http.StatusTooManyRequests, `{}`, socialhub.CodeRateLimited, socialhub.ClassRetryable, ""},
		{"server code", http.StatusBadRequest, `{"error":"server_error"}`, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, "server_error"},
		{"service", http.StatusServiceUnavailable, `{}`, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, ""},
		{"other", http.StatusTeapot, `{}`, socialhub.CodePlatformError, socialhub.ClassPermanent, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			header := http.Header{"Retry-After": {"2.5"}, "X-Request-Id": {"request-1"}}
			err := decodeHTTPError(test.status, header, []byte(test.body))
			var platformErr *socialhub.Error
			if !errors.As(err, &platformErr) || platformErr.Code != test.code || platformErr.Class != test.class ||
				platformErr.PlatformCode != test.platformCode || platformErr.RequestID != "request-1" || platformErr.RetryAfter != 2500*time.Millisecond {
				t.Fatalf("error=%#v", platformErr)
			}
		})
	}
	if parseRetryAfter("bad") != 0 || parseRetryAfter("-1") != 0 || parseRetryAfter("999999") != 0 {
		t.Fatal("invalid Retry-After accepted")
	}
	if boundedMessage(strings.Repeat("界", 5), 3) != "界界界" || firstNonEmpty("", " value ") != " value " {
		t.Fatal("bounded helpers failed")
	}
}

func TestMappingVariants(t *testing.T) {
	client := &Client{accountID: "artist", clock: fixedClock{now: testNow}}
	videoPost, err := client.mapDeviation(Deviation{
		DeviationID: testDeviationID, Title: "Video", PublishedTime: "1722513600",
		Videos: []VideoResource{{Src: "https://video.example/work.mp4", FileSize: 20, Duration: 7}},
	})
	if err != nil || len(videoPost.Media) != 1 || videoPost.Media[0].Type != socialhub.MediaTypeVideo ||
		videoPost.Media[0].Duration == nil || videoPost.CreatedAt == nil {
		t.Fatalf("video post=%#v err=%v", videoPost, err)
	}
	previewPost, err := client.mapDeviation(Deviation{
		DeviationID: testDeviationID, TextContent: &EditorText{Excerpt: "Literature"},
		Preview: &ImageResource{Src: "https://images.example/preview.png", Width: 10, Height: 20},
	})
	if err != nil || dereference(previewPost.Text) != "Literature" || len(previewPost.Media) != 1 {
		t.Fatalf("preview post=%#v err=%v", previewPost, err)
	}
	profile := &Profile{User: User{UserID: testUserID, Username: "sample-artist", Type: "regular"}, RealName: "Name", ProfileURL: "https://example.test"}
	mappedUser := mapUser("artist", profile.User, profile)
	if dereference(mappedUser.DisplayName) != "Name" || dereference(mappedUser.AccountType) != "regular" {
		t.Fatalf("user=%#v", mappedUser)
	}
	if parseTimestamp("not-time") != nil || parseTimestamp("") != nil || intPointer(0) != nil || int64Pointer(-1) != nil || durationPointer(0) != nil || cleanStringPointer(nil) != nil {
		t.Fatal("mapping helper accepted invalid values")
	}
}

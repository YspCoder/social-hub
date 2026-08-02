package tmdb

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestHTTPErrorClassification(t *testing.T) {
	tests := []struct {
		name   string
		status int
		code   int
		want   socialhub.ErrorCode
		class  socialhub.ErrorClass
	}{
		{"bad request", http.StatusBadRequest, 18, socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{"invalid service", http.StatusNotImplemented, 2, socialhub.CodeUnsupported, socialhub.ClassPermanent},
		{"invalid key", http.StatusUnauthorized, 7, socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{"permission", http.StatusUnauthorized, 36, socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{"not found", http.StatusNotFound, 34, socialhub.CodeNotFound, socialhub.ClassPermanent},
		{"duplicate", http.StatusForbidden, 8, socialhub.CodeConflict, socialhub.ClassPermanent},
		{"maintenance", http.StatusServiceUnavailable, 46, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{"platform rate", http.StatusBadRequest, 25, socialhub.CodeRateLimited, socialhub.ClassRetryable},
		{"approval", http.StatusUnprocessableEntity, 41, socialhub.CodeApprovalRequired, socialhub.ClassUserAction},
		{"HTTP rate", http.StatusTooManyRequests, 0, socialhub.CodeRateLimited, socialhub.ClassRetryable},
		{"other", http.StatusTeapot, 0, socialhub.CodePlatformError, socialhub.ClassPermanent},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			header := http.Header{}
			header.Set("Retry-After", "7")
			header.Set("X-Request-ID", "request-1")
			body := `{"success":false,"status_code":` + intString(int64(test.code)) + `,"status_message":"failure"}`
			err := decodeHTTPError(test.status, header, []byte(body))
			var platformErr *socialhub.Error
			if !errors.As(err, &platformErr) || platformErr.Code != test.want || platformErr.Class != test.class ||
				platformErr.HTTPStatus != test.status || platformErr.RequestID != "request-1" || platformErr.RetryAfter != 7*time.Second {
				t.Fatalf("error=%#v", platformErr)
			}
		})
	}
	if parseRetryAfter("bad") != 0 || parseRetryAfter("-1") != 0 || parseRetryAfter("86401") != 0 ||
		bounded(strings.Repeat("界", 5), 3) != strings.Repeat("界", 3) || firstNonEmpty("", " value ") != " value " {
		t.Fatal("error helpers accepted invalid input")
	}
}

func TestPaginationStatusAndValidationHelpers(t *testing.T) {
	page, err := pageFromEnvelope(pageEnvelope[int]{Page: 2, Results: []int{1}, TotalPages: 3, TotalResults: 21})
	if err != nil || !page.HasMore || page.NextCursor == nil || *page.NextCursor != "3" || page.PrevCursor == nil || *page.PrevCursor != "1" {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	if _, err := pageFromEnvelope(pageEnvelope[int]{Page: 0, TotalPages: 3}); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("bad page=%v", err)
	}
	query := url.Values{}
	setPageAndLanguage(query, 12, "zh-CN")
	if query.Get("page") != "12" || query.Get("language") != "zh-CN" {
		t.Fatalf("query=%v", query)
	}
	for _, cursor := range []string{"0", "501", "bad"} {
		if _, err := validatePage(cursor); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("cursor=%q err=%v", cursor, err)
		}
	}
	if !validLocale("zh-CN") || validLocale("bad locale") || !validRating(8.5) || validRating(8.25) || validMediaType(MediaPerson, false, false) {
		t.Fatal("validation helper mismatch")
	}
	for _, code := range []int{1, 12, 13, 40} {
		if err := validateStatus("write", &StatusResponse{StatusCode: code}); err != nil {
			t.Fatalf("status %d=%v", code, err)
		}
	}
	if err := validateStatus("write", &StatusResponse{StatusCode: 99, StatusMessage: "failed"}); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("status error=%v", err)
	}
	if err := validateStatus("write", &StatusResponse{StatusCode: 21, StatusMessage: "missing"}); errorCode(err) != socialhub.CodeNotFound {
		t.Fatalf("not found status=%v", err)
	}
	if err := validateStatus("write", nil); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("nil status=%v", err)
	}
	if _, err := parseTMDBTime("not-time"); err == nil {
		t.Fatal("invalid timestamp accepted")
	}
}

func TestMalformedAndInvalidSuccessResponses(t *testing.T) {
	call := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		call++
		switch call {
		case 1:
			writeJSON(writer, http.StatusOK, `{`)
		case 2:
			writeJSON(writer, http.StatusOK, `{"success":false,"expires_at":"bad","request_token":""}`)
		case 3:
			writeJSON(writer, http.StatusOK, `{"images":{},"change_keys":[]}`)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, true, false, false)
	if _, err := client.GetMovie(context.Background(), 1, ""); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("malformed JSON=%v", err)
	}
	if _, err := client.RequestToken(context.Background()); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("invalid token=%v", err)
	}
	if _, err := client.GetConfiguration(context.Background()); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("invalid config=%v", err)
	}
}

func errorCode(err error) socialhub.ErrorCode {
	var platformErr *socialhub.Error
	if errors.As(err, &platformErr) {
		return platformErr.Code
	}
	return ""
}

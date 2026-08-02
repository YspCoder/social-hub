package trakt

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"social-hub/internal/transport"
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
		{"bad request", http.StatusBadRequest, `{}`, socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{"invalid grant", http.StatusBadRequest, `{"error":"invalid_grant"}`, socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{"invalid token", http.StatusUnauthorized, `{"error":"invalid_token"}`, socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{"access denied", http.StatusForbidden, `{"error":"access_denied"}`, socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{"not found", http.StatusNotFound, `{}`, socialhub.CodeNotFound, socialhub.ClassPermanent},
		{"conflict", http.StatusConflict, `{}`, socialhub.CodeConflict, socialhub.ClassPermanent},
		{"gone", http.StatusGone, `{}`, socialhub.CodeNotFound, socialhub.ClassPermanent},
		{"account limit", 420, `{}`, socialhub.CodeApprovalRequired, socialhub.ClassUserAction},
		{"unprocessable", http.StatusUnprocessableEntity, `{}`, socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{"locked", http.StatusLocked, `{}`, socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{"upgrade", http.StatusUpgradeRequired, `{}`, socialhub.CodeApprovalRequired, socialhub.ClassUserAction},
		{"rate limit", http.StatusTooManyRequests, `{}`, socialhub.CodeRateLimited, socialhub.ClassRetryable},
		{"temporary OAuth", http.StatusServiceUnavailable, `{"error":"temporarily_unavailable"}`, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{"server", http.StatusBadGateway, `{}`, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{"other", http.StatusTeapot, `not-json`, socialhub.CodePlatformError, socialhub.ClassPermanent},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			header := http.Header{}
			header.Set("Retry-After", "7")
			header.Set("X-Correlation-ID", "request-1")
			err := decodeHTTPError(test.status, header, []byte(test.body))
			var platformErr *socialhub.Error
			if !errors.As(err, &platformErr) || platformErr.Code != test.code || platformErr.Class != test.class ||
				platformErr.HTTPStatus != test.status || platformErr.RequestID != "request-1" || platformErr.RetryAfter != 7*time.Second {
				t.Fatalf("error=%#v", platformErr)
			}
		})
	}

	header := http.Header{}
	header.Set("X-Request-ID", strings.Repeat("r", 300))
	err := decodeHTTPError(http.StatusBadRequest, header, []byte(`{"error":"code","error_description":"details"}`))
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.PlatformCode != "code" || platformErr.PlatformMessage != "details" || len(platformErr.RequestID) != 256 {
		t.Fatalf("bounded error=%#v", platformErr)
	}
	if parseRetryAfter("bad") != 0 || parseRetryAfter("-1") != 0 || parseRetryAfter("86401") != 0 ||
		bounded(strings.Repeat("界", 5), 3) != strings.Repeat("界", 3) || firstNonEmpty("", " value ") != " value " {
		t.Fatal("error helpers accepted invalid input")
	}
}

func TestDeviceErrorStatesAndMalformedOAuthResponse(t *testing.T) {
	responses := []struct {
		status int
		body   string
	}{
		{http.StatusGone, `{}`},
		{http.StatusTeapot, `{}`},
		{http.StatusOK, `{`},
	}
	call := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		response := responses[call]
		call++
		writeJSON(writer, response.status, response.body)
	}))
	defer server.Close()
	_, client := newTestClient(t, server, true, false)
	authorization := DeviceAuthorization{DeviceCode: "device", ExpiresAt: testNow.Add(time.Minute), Interval: time.Second}

	_, err := client.PollDevice(context.Background(), authorization)
	assertPlatformError(t, err, socialhub.CodeUnauthenticated, socialhub.ClassUserAction, "expired_token")
	_, err = client.PollDevice(context.Background(), authorization)
	assertPlatformError(t, err, socialhub.CodePermissionDenied, socialhub.ClassUserAction, "access_denied")
	_, err = client.PollDevice(context.Background(), authorization)
	assertPlatformError(t, err, socialhub.CodePlatformError, socialhub.ClassPermanent, "")
}

func TestPaginationValidationAndReferenceEncoding(t *testing.T) {
	header := http.Header{"X-Pagination-Page": {"12"}, "X-Pagination-Page-Count": {"13"}}
	page := pageFromMetadata([]int{1}, 0, transport.ResponseMetadata{Header: header})
	if !page.HasMore || page.NextCursor == nil || *page.NextCursor != "13" || page.PrevCursor == nil || *page.PrevCursor != "11" {
		t.Fatalf("page=%#v", page)
	}
	first := pageFromMetadata([]int{1}, 0, transport.ResponseMetadata{Header: http.Header{"X-Pagination-Page": {"bad"}}})
	if first.HasMore || first.NextCursor != nil || first.PrevCursor != nil {
		t.Fatalf("first page=%#v", first)
	}
	query := url.Values{}
	setPage(query, 12, 100)
	if query.Get("page") != "12" || query.Get("limit") != "100" || headerInt(http.Header{"Page": {"-1"}}, "Page") != 0 {
		t.Fatalf("query=%v", query)
	}
	for _, input := range []struct {
		cursor string
		max    int
	}{
		{"bad", 1}, {"0", 1}, {"1000001", 1}, {"", -1}, {"", 101},
	} {
		if _, err := validatePage(input.cursor, input.max); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("cursor=%q max=%d err=%v", input.cursor, input.max, err)
		}
	}

	encoded, err := json.Marshal(MediaMutation{
		Movies: []MovieRef{{Title: "TRON", Year: 1982}},
		Shows:  []ShowRef{{Title: "TRON: Uprising", Year: 2012}},
	})
	if err != nil || strings.Contains(string(encoded), `"ids"`) {
		t.Fatalf("reference JSON=%s err=%v", encoded, err)
	}
	if err := validateMediaMutation(MediaMutation{Movies: []MovieRef{{Title: "TRON", Year: 1982}}}); err != nil {
		t.Fatalf("title/year reference=%v", err)
	}
}

func assertPlatformError(t *testing.T, err error, code socialhub.ErrorCode, class socialhub.ErrorClass, platformCode string) {
	t.Helper()
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.Code != code || platformErr.Class != class || platformErr.PlatformCode != platformCode {
		t.Fatalf("error=%#v", platformErr)
	}
}

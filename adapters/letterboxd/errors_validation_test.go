package letterboxd

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
		name         string
		status       int
		platformCode string
		want         socialhub.ErrorCode
		class        socialhub.ErrorClass
	}{
		{"bad request", http.StatusBadRequest, "", socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{"invalid grant", http.StatusBadRequest, "invalid_grant", socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{"invalid token", http.StatusUnauthorized, "invalid_token", socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{"access denied", http.StatusBadRequest, "access_denied", socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{"insufficient scope", http.StatusForbidden, "insufficient_scope", socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{"not found", http.StatusNotFound, "", socialhub.CodeNotFound, socialhub.ClassPermanent},
		{"conflict", http.StatusConflict, "", socialhub.CodeConflict, socialhub.ClassPermanent},
		{"rate", http.StatusTooManyRequests, "", socialhub.CodeRateLimited, socialhub.ClassRetryable},
		{"temporary", http.StatusBadRequest, "temporarily_unavailable", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{"server", http.StatusServiceUnavailable, "server_error", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{"other", http.StatusTeapot, "unknown", socialhub.CodePlatformError, socialhub.ClassPermanent},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			header := http.Header{}
			header.Set("Retry-After", "7")
			header.Set("X-Correlation-ID", "request-1")
			body := `{"error":"` + test.platformCode + `","error_description":"fixture failure"}`
			err := decodeHTTPError(test.status, header, []byte(body))
			var platformErr *socialhub.Error
			if !errors.As(err, &platformErr) || platformErr.Code != test.want || platformErr.Class != test.class ||
				platformErr.HTTPStatus != test.status || platformErr.RequestID != "request-1" ||
				platformErr.RetryAfter != 7*time.Second || platformErr.PlatformMessage != "fixture failure" {
				t.Fatalf("error=%#v", platformErr)
			}
		})
	}
}

func TestOAuthErrorAndResponseLimits(t *testing.T) {
	call := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		call++
		switch call {
		case 1:
			writer.Header().Set("Retry-After", "3")
			writeJSON(writer, http.StatusTooManyRequests, `{"error":"temporarily_unavailable","error_description":"slow down"}`)
		case 2:
			writeJSON(writer, http.StatusOK, `{`)
		case 3:
			writeJSON(writer, http.StatusOK, `{"access_token":"","token_type":"Bearer","expires_in":3600}`)
		case 4:
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write([]byte(strings.Repeat("x", int(maxOAuthResponseBytes)+1)))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, TokenClient, false, nil)

	_, err := client.ClientCredentials(context.Background(), nil)
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodeTemporarilyUnavailable ||
		platformErr.Class != socialhub.ClassRetryable || platformErr.RetryAfter != 3*time.Second || platformErr.Op != "oauth_client_credentials" {
		t.Fatalf("rate error=%#v", platformErr)
	}
	for index := 0; index < 3; index++ {
		if _, err := client.ClientCredentials(context.Background(), nil); errorCode(err) != socialhub.CodePlatformError {
			t.Fatalf("invalid response %d=%v", index, err)
		}
	}
}

func TestTransportErrorsAreSanitized(t *testing.T) {
	client := &Client{
		clientID: testClientID, clientSecret: testClientSecret, tokenURL: "https://api.example.test/token",
		userAgent: "test-agent", clock: fixedClock{now: testNow},
		httpClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, &url.Error{Op: "Post", URL: "https://api.example.test/token?secret=leak", Err: errors.New("dial failed")}
		})},
	}
	_, err := client.ClientCredentials(context.Background(), nil)
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.Class != socialhub.ClassRetryable || platformErr.Cause == nil ||
		strings.Contains(platformErr.Cause.Error(), "secret=leak") {
		t.Fatalf("transport error=%#v", platformErr)
	}
}

func TestMalformedAPIResponseAndErrorOperation(t *testing.T) {
	call := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		call++
		if call == 1 {
			writeJSON(writer, http.StatusOK, `{`)
			return
		}
		writer.Header().Set("X-Request-ID", "request-2")
		writeJSON(writer, http.StatusNotFound, `{"type":"FilmNotFound","message":"missing"}`)
	}))
	defer server.Close()
	_, client := newTestClient(t, server, TokenClient, true, nil)
	if _, err := client.GetFilm(context.Background(), "film-1"); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("malformed JSON=%v", err)
	}
	_, err := client.GetFilm(context.Background(), "film-2")
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodeNotFound || platformErr.Op != "GET /film/film-2" ||
		platformErr.PlatformCode != "FilmNotFound" || platformErr.RequestID != "request-2" {
		t.Fatalf("HTTP error=%#v", platformErr)
	}
}

func TestValidationAndPaginationHelpers(t *testing.T) {
	if !validEndpoint("https://api.example.test/v0") || validEndpoint("ftp://api.example.test") ||
		!validRedirectURI("http://localhost/callback") || validRedirectURI("https://u:p@example.test/cb") ||
		!validCredential("token") || validCredential(" token") || !validReference("env://token") || validReference("bad\nref") ||
		!validIdentifier("imdb:tt0083658") || validIdentifier("bad/id") || !validSearch("Blade Runner") || validSearch(" query ") ||
		!validText("line one\nline two") || validText(" \t") || !validUserAgent("test-agent") || validUserAgent("bad\ragent") {
		t.Fatal("basic validation mismatch")
	}
	for _, rating := range []float64{0.5, 1, 4.5, 5} {
		if !validRating(rating) {
			t.Fatalf("valid rating rejected: %v", rating)
		}
	}
	for _, rating := range []float64{0, 0.75, 5.5} {
		if validRating(rating) {
			t.Fatalf("invalid rating accepted: %v", rating)
		}
	}
	if !validPage("cursor", 100) || validPage("cursor", 101) || validPage("bad\ncursor", 20) ||
		!validScopes([]string{"profile", "email"}) || validScopes([]string{"user"}) ||
		!containsScope([]string{"profile", "content:modify"}, "content:modify") ||
		!validUniqueValues([]string{"FullText"}, allowedSearchMethods) || validUniqueValues([]string{"FullText", "FullText"}, allowedSearchMethods) ||
		!validDate("2026-08-02") || validDate("2026-02-30") || !validCommentPolicy("Friends") || validCommentPolicy("Draft") ||
		!validPrivacyPolicy("Draft") || validPrivacyPolicy("Team") ||
		!validTags([]string{"sci-fi"}) || validTags([]string{""}) || !validYear(2026) || validYear(1800) ||
		!validDecade(1980) || validDecade(1982) {
		t.Fatal("structured validation mismatch")
	}
	page := toPage([]int{1, 2}, "next")
	if !page.HasMore || page.NextCursor == nil || *page.NextCursor != "next" || len(page.Items) != 2 {
		t.Fatalf("page=%#v", page)
	}
	if parseRetryAfter("bad") != 0 || parseRetryAfter("-1") != 0 || parseRetryAfter("86401") != 0 ||
		bounded(strings.Repeat("界", 5), 3) != strings.Repeat("界", 3) || firstNonEmpty("", " value ") != " value " {
		t.Fatal("error helper mismatch")
	}
}

func errorCode(err error) socialhub.ErrorCode {
	var platformErr *socialhub.Error
	if errors.As(err, &platformErr) {
		return platformErr.Code
	}
	return ""
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

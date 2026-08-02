package myanimelist

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
		{"invalid client", http.StatusUnauthorized, "invalid_client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{"access denied", http.StatusBadRequest, "access_denied", socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{"insufficient scope", http.StatusForbidden, "insufficient_scope", socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{"not found", http.StatusNotFound, "", socialhub.CodeNotFound, socialhub.ClassPermanent},
		{"gone", http.StatusGone, "", socialhub.CodeNotFound, socialhub.ClassPermanent},
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

	header := http.Header{}
	header.Set("X-Request-ID", strings.Repeat("r", 300))
	err := decodeHTTPError(http.StatusBadRequest, header, []byte(`{"error":"code","message":"details"}`))
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.PlatformCode != "code" || platformErr.PlatformMessage != "details" || len(platformErr.RequestID) != 256 {
		t.Fatalf("bounded error=%#v", platformErr)
	}
}

func TestOAuthErrorsAndResponseLimits(t *testing.T) {
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
			writeJSON(writer, http.StatusOK, `{"access_token":"","refresh_token":"refresh","expires_in":3600}`)
		case 4:
			writeJSON(writer, http.StatusOK, `{"access_token":"token","refresh_token":"refresh","token_type":"MAC","expires_in":3600}`)
		case 5:
			writeJSON(writer, http.StatusOK, `{"access_token":"token","refresh_token":"refresh","expires_in":31622401}`)
		case 6:
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write([]byte(strings.Repeat("x", int(maxOAuthResponseBytes)+1)))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, false, false, nil)

	_, err := client.Exchange(context.Background(), "code", "https://app.example/cb", strings.Repeat("a", 43))
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodeTemporarilyUnavailable ||
		platformErr.Class != socialhub.ClassRetryable || platformErr.RetryAfter != 3*time.Second || platformErr.Op != "oauth_exchange" {
		t.Fatalf("OAuth error=%#v", platformErr)
	}
	for index := 0; index < 5; index++ {
		if _, err := client.Exchange(context.Background(), "code", "https://app.example/cb", strings.Repeat("a", 43)); errorCode(err) != socialhub.CodePlatformError {
			t.Fatalf("invalid OAuth response %d=%v", index, err)
		}
	}
}

func TestAPIResponseLimitsMalformedJSONAndOperation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Query().Get("q") {
		case "malformed":
			writeJSON(writer, http.StatusOK, `{`)
		case "oversized":
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write([]byte(strings.Repeat("x", (8<<20)+1)))
		default:
			writer.Header().Set("X-Request-ID", "request-2")
			writeJSON(writer, http.StatusNotFound, `{"error":"missing","message":"anime not found"}`)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, false, false, nil)
	for _, query := range []string{"malformed", "oversized"} {
		if _, err := client.SearchAnime(context.Background(), SearchRequest{Query: query}); errorCode(err) != socialhub.CodePlatformError {
			t.Fatalf("%s response=%v", query, err)
		}
	}
	_, err := client.SearchAnime(context.Background(), SearchRequest{Query: "missing"})
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodeNotFound || platformErr.Op != "GET /anime" ||
		platformErr.PlatformCode != "missing" || platformErr.PlatformMessage != "anime not found" || platformErr.RequestID != "request-2" {
		t.Fatalf("API error=%#v", platformErr)
	}
}

func TestTransportErrorsAreSanitized(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, &url.Error{Op: "Get", URL: "https://api.example/anime?access_token=leak", Err: errors.New("dial failed")}
	})}
	config := socialhub.AdapterConfig{
		Adapter: adapterName,
		Settings: map[string]any{
			"base_url": "https://api.example", "auth_url": "https://auth.example/authorize",
			"token_url": "https://auth.example/token", "user_agent": "test-agent",
		},
		Accounts: []socialhub.AccountConfig{{ID: "fan", ClientID: testClientID}},
	}
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), config, socialhub.WithHTTPClient(httpClient), socialhub.WithClock(fixedClock{now: testNow})); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "fan")
	if err != nil {
		t.Fatal(err)
	}
	client := common.(*Client)

	_, err = client.SearchAnime(context.Background(), SearchRequest{Query: "Gundam"})
	assertSanitizedTransportError(t, err)
	_, err = client.Exchange(context.Background(), "code", "https://app.example/cb", strings.Repeat("a", 43))
	assertSanitizedTransportError(t, err)
}

func TestRedirectsAreNotFollowed(t *testing.T) {
	targetCalls := 0
	target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		targetCalls++
		writeJSON(writer, http.StatusOK, `{"data":[],"paging":{}}`)
	}))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, target.URL+request.URL.Path, http.StatusFound)
	}))
	defer server.Close()
	_, client := newTestClient(t, server, false, false, nil)
	if _, err := client.SearchAnime(context.Background(), SearchRequest{Query: "redirect"}); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("API redirect=%v", err)
	}
	if _, err := client.Exchange(context.Background(), "code", "https://app.example/cb", strings.Repeat("a", 43)); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("OAuth redirect=%v", err)
	}
	if targetCalls != 0 {
		t.Fatalf("redirect target calls=%d", targetCalls)
	}
}

func TestValidationAndPagingHelpers(t *testing.T) {
	if !validEndpoint("https://api.example/v2") || validEndpoint("ftp://api.example") ||
		!validRedirectURI("http://localhost/callback") || validRedirectURI("https://u:p@example/cb") ||
		!validCredential("token") || validCredential(" token") || !validReference("env://token") || validReference("bad\nref") ||
		!validUserAgent("test-agent") || validUserAgent("bad\ragent") || !validUsername("user_name") || validUsername("bad/user") ||
		!validQuery("Gundam") || validQuery(" query ") || !validPKCEValue(strings.Repeat("A0-._~", 8)[:43]) ||
		validPKCEValue(strings.Repeat("é", 43)) || validPKCEValue(strings.Repeat("a", 42)) {
		t.Fatal("basic validation mismatch")
	}
	if !validPage("0", 100) || validPage("01", 20) || validPage("1000000001", 20) || validPage("", 101) ||
		!validFields([]string{"authors{first_name,last_name}"}) || validFields([]string{"bad field"}) ||
		validFields([]string{"authors{first_name"}) || !validTags([]string{}) || validTags([]string{"bad,tag"}) ||
		!validComment(pointer("line one\nline two")) || validComment(pointer("bad\x00comment")) {
		t.Fatal("structured validation mismatch")
	}
	page, err := toPage([]int{1}, paging{Previous: "https://api.example/items?offset=0", Next: "https://api.example/items?offset=20"})
	if err != nil || !page.HasMore || page.NextCursor == nil || *page.NextCursor != "20" || page.PrevCursor == nil || *page.PrevCursor != "0" {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	for _, value := range []string{"https://api.example/items?page=2", "https://api.example/items?offset=01", "://bad"} {
		if _, err := pagingCursor(value); err == nil || strings.Contains(err.Error(), value) {
			t.Fatalf("paging URL %q err=%v", value, err)
		}
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

func assertSanitizedTransportError(t *testing.T, err error) {
	t.Helper()
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodeTemporarilyUnavailable ||
		platformErr.Class != socialhub.ClassRetryable || platformErr.Cause == nil ||
		strings.Contains(platformErr.Cause.Error(), "access_token=leak") {
		t.Fatalf("transport error=%#v", platformErr)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func pointer[T any](value T) *T { return &value }

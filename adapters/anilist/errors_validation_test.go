package anilist

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

func TestHTTPAndGraphQLErrorClassification(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		platformCode string
		message      string
		want         socialhub.ErrorCode
		class        socialhub.ErrorClass
	}{
		{"bad request", http.StatusBadRequest, "", "bad query", socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{"invalid grant", http.StatusBadRequest, "invalid_grant", "expired code", socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{"invalid token", http.StatusUnauthorized, "invalid_token", "expired token", socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{"access denied", http.StatusForbidden, "access_denied", "denied", socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{"OAuth server error", http.StatusBadRequest, "server_error", "try again", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{"OAuth unavailable", http.StatusBadRequest, "temporarily_unavailable", "try again", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{"forbidden", http.StatusForbidden, "", "private", socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{"disabled", http.StatusForbidden, "403", "API temporarily disabled", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{"not found", http.StatusNotFound, "", "missing", socialhub.CodeNotFound, socialhub.ClassPermanent},
		{"gone", http.StatusGone, "", "gone", socialhub.CodeNotFound, socialhub.ClassPermanent},
		{"conflict", http.StatusConflict, "", "conflict", socialhub.CodeConflict, socialhub.ClassPermanent},
		{"rate", http.StatusTooManyRequests, "", "slow down", socialhub.CodeRateLimited, socialhub.ClassRetryable},
		{"server", http.StatusServiceUnavailable, "", "offline", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{"other", http.StatusTeapot, "", "unknown", socialhub.CodePlatformError, socialhub.ClassPermanent},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			header := http.Header{}
			header.Set("Retry-After", "7")
			header.Set("CF-Ray", "request-1")
			var err error
			if test.platformCode != "" && test.platformCode != "403" {
				body := `{"error":"` + test.platformCode + `","error_description":"` + test.message + `"}`
				err = decodeHTTPError(test.status, header, []byte(body))
			} else {
				source := graphQLError{Status: test.status, Message: test.message}
				err = graphQLPlatformError("operation", test.status, header, source)
			}
			var platformErr *socialhub.Error
			if !errors.As(err, &platformErr) || platformErr.Code != test.want || platformErr.Class != test.class ||
				platformErr.HTTPStatus != test.status || platformErr.RequestID != "request-1" ||
				platformErr.RetryAfter != 7*time.Second || platformErr.PlatformMessage != test.message {
				t.Fatalf("error=%#v", platformErr)
			}
		})
	}

	header := http.Header{}
	header.Set("X-Request-ID", strings.Repeat("r", 300))
	source := graphQLError{Status: http.StatusBadRequest, Message: strings.Repeat("界", 600)}
	source.Extensions.Code = strings.Repeat("c", 200)
	err := graphQLPlatformError("operation", http.StatusOK, header, source)
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || len(platformErr.PlatformCode) != 128 ||
		len([]rune(platformErr.PlatformMessage)) != 512 || len(platformErr.RequestID) != 256 {
		t.Fatalf("bounded error=%#v", platformErr)
	}
}

func TestGraphQLErrorsMalformedAndResponseLimits(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body := readGraphQLRequest(t, writer, request)
		query, _ := body.Variables["search"].(string)
		switch query {
		case "rate":
			writeJSON(writer, http.StatusOK, `{"data":null,"errors":[{"message":"Too Many Requests.","status":429}]}`)
		case "disabled":
			writeJSON(writer, http.StatusOK, `{"data":null,"errors":[{"message":"The AniList API has been temporarily disabled due to severe stability issues.","status":403}]}`)
		case "partial":
			writeJSON(writer, http.StatusOK, `{"data":{"Page":{"media":[]}},"errors":[{"message":"validation","status":400,"validation":{"score":["invalid"]}}]}`)
		case "malformed":
			writeJSON(writer, http.StatusOK, `{`)
		case "null":
			writeJSON(writer, http.StatusOK, `{"data":null}`)
		case "oversized":
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write([]byte(strings.Repeat("x", (8<<20)+1)))
		default:
			writer.Header().Set("Retry-After", "9")
			writer.Header().Set("X-Request-ID", "request-2")
			writeJSON(writer, http.StatusTooManyRequests, `{"errors":[{"message":"Too Many Requests.","status":429}]}`)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, false, false, false)

	tests := []struct {
		query string
		code  socialhub.ErrorCode
		class socialhub.ErrorClass
	}{
		{"rate", socialhub.CodeRateLimited, socialhub.ClassRetryable},
		{"disabled", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{"partial", socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{"malformed", socialhub.CodePlatformError, socialhub.ClassPermanent},
		{"null", socialhub.CodePlatformError, socialhub.ClassPermanent},
		{"oversized", socialhub.CodePlatformError, socialhub.ClassPermanent},
	}
	for _, test := range tests {
		_, err := client.SearchMedia(context.Background(), SearchMediaRequest{Query: test.query, Type: MediaAnime})
		var platformErr *socialhub.Error
		if !errors.As(err, &platformErr) || platformErr.Code != test.code || platformErr.Class != test.class || platformErr.Op != "search_media" {
			t.Fatalf("query=%s error=%#v", test.query, platformErr)
		}
	}
	_, err := client.SearchMedia(context.Background(), SearchMediaRequest{Query: "http-rate", Type: MediaAnime})
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodeRateLimited ||
		platformErr.RetryAfter != 9*time.Second || platformErr.RequestID != "request-2" {
		t.Fatalf("HTTP rate error=%#v", platformErr)
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
			writeJSON(writer, http.StatusOK, `{"access_token":"","expires_in":31536000}`)
		case 4:
			writeJSON(writer, http.StatusOK, `{"access_token":"token","token_type":"MAC","expires_in":31536000}`)
		case 5:
			writeJSON(writer, http.StatusOK, `{"access_token":"token","expires_in":31622401}`)
		case 6:
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write([]byte(strings.Repeat("x", int(maxOAuthResponseBytes)+1)))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, true, true, false)

	_, err := client.Exchange(context.Background(), "code", "https://app.example/cb")
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodeRateLimited ||
		platformErr.Class != socialhub.ClassRetryable || platformErr.RetryAfter != 3*time.Second || platformErr.Op != "oauth_exchange" {
		t.Fatalf("OAuth error=%#v", platformErr)
	}
	for index := 0; index < 5; index++ {
		if _, err := client.Exchange(context.Background(), "code", "https://app.example/cb"); errorCode(err) != socialhub.CodePlatformError {
			t.Fatalf("invalid OAuth response %d=%v", index, err)
		}
	}
}

func TestTransportErrorsAreSanitized(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, &url.Error{Op: "Post", URL: "https://graphql.example?access_token=leak", Err: errors.New("dial failed")}
	})}
	config := socialhub.AdapterConfig{
		Adapter: adapterName,
		Settings: map[string]any{
			"graphql_url": "https://graphql.example", "auth_url": "https://auth.example/authorize",
			"token_url": "https://auth.example/token", "user_agent": "test-agent",
		},
		Accounts: []socialhub.AccountConfig{{
			ID: "fan", ClientID: testClientID, SecretRef: "test://secret",
		}},
	}
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), config, socialhub.WithHTTPClient(httpClient),
		socialhub.WithSecretResolver(mapResolver{"test://secret": testClientSecret}), socialhub.WithClock(fixedClock{now: testNow})); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "fan")
	if err != nil {
		t.Fatal(err)
	}
	client := common.(*Client)
	_, err = client.GetMedia(context.Background(), 1)
	assertSanitizedTransportError(t, err)
	_, err = client.Exchange(context.Background(), "code", "https://app.example/cb")
	assertSanitizedTransportError(t, err)
}

func TestRedirectsAreNotFollowed(t *testing.T) {
	targetCalls := 0
	target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		targetCalls++
		writeJSON(writer, http.StatusOK, `{"data":{"Media":null}}`)
	}))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, target.URL+request.URL.Path, http.StatusFound)
	}))
	defer server.Close()
	_, client := newTestClient(t, server, true, true, false)
	if _, err := client.GetMedia(context.Background(), 1); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("GraphQL redirect=%v", err)
	}
	if _, err := client.Exchange(context.Background(), "code", "https://app.example/cb"); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("OAuth redirect=%v", err)
	}
	if targetCalls != 0 {
		t.Fatalf("redirect target calls=%d", targetCalls)
	}
}

func TestInvalidPlatformResponses(t *testing.T) {
	call := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		call++
		switch call {
		case 1:
			writeJSON(writer, http.StatusOK, `{"data":{"Page":{"pageInfo":{"currentPage":2,"perPage":50,"hasNextPage":false},"media":[]}}}`)
		case 2:
			writeJSON(writer, http.StatusOK, `{"data":{"DeleteMediaListEntry":{"deleted":false}}}`)
		case 3:
			writeJSON(writer, http.StatusOK, `{"data":{"Page":{"pageInfo":{"currentPage":1,"perPage":50,"hasNextPage":false},"activities":[{"__typename":"MessageActivity","id":1}]}}}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, false, false, true)
	if _, err := client.ListTrendingMedia(context.Background(), ListMediaRequest{Type: MediaAnime}); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("page mismatch=%v", err)
	}
	if err := client.DeleteMediaListEntry(context.Background(), 1); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("delete=false error=%v", err)
	}
	if _, err := client.ListActivities(context.Background(), ListActivitiesRequest{}); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("union error=%v", err)
	}
}

func TestValidationHelpers(t *testing.T) {
	if !validEndpoint("https://graphql.example") || validEndpoint("ftp://graphql.example") ||
		!validRedirectURI("https://app.example/cb?tenant=1") || !validRedirectURI("myapp://oauth/cb") ||
		validRedirectURI("javascript:alert(1)") || !validCredential("token") || validCredential(" token") ||
		!validReference("env://token") || validReference("bad\nref") || !validUserAgent("test-agent") ||
		validUserAgent("bad\ragent") || !validState("state") || validState(" state") || !validSearch("Gundam") ||
		validSearch(" query ") || !validUsername("user_name") || validUsername("bad/user") {
		t.Fatal("basic validation mismatch")
	}
	if page, valid := validPage("50", 50); !valid || page != 50 {
		t.Fatal("valid page rejected")
	}
	if _, valid := validPage("01", 10); valid {
		t.Fatal("invalid cursor accepted")
	}
	if !validFuzzyDate(FuzzyDate{}) || !validFuzzyDate(FuzzyDate{Year: 2026, Month: 8}) ||
		validFuzzyDate(FuzzyDate{Year: 2026, Month: 2, Day: 30}) || validFuzzyDate(FuzzyDate{Day: 1}) ||
		!validActivityTypes([]ActivityType{ActivityText, ActivityAnimeList, ActivityMediaList}) ||
		validActivityTypes([]ActivityType{ActivityText, ActivityText}) || !validCustomLists([]string{}) ||
		validCustomLists([]string{"dup", "dup"}) || !validText("line one\nline two") || validText("bad\x00text") {
		t.Fatal("structured validation mismatch")
	}
	page, err := toPage([]int{1}, pageInfo{CurrentPage: 2, PerPage: 10, HasNextPage: true}, 2)
	if err != nil || page.NextCursor == nil || *page.NextCursor != "3" || page.PrevCursor == nil || *page.PrevCursor != "1" {
		t.Fatalf("page=%#v err=%v", page, err)
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

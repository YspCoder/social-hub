package giphy

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestHTTPAndEnvelopeErrors(t *testing.T) {
	targetHits := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/v1/gifs/limited":
			writer.Header().Set("Retry-After", "7")
			writer.Header().Set("X-Request-ID", "header-id")
			writeJSON(writer, http.StatusTooManyRequests, `{"data":[],"meta":{"status":429,"msg":"slow down","response_id":"meta-id"}}`)
		case "/v1/gifs/meta-error":
			writeJSON(writer, http.StatusOK, `{"data":[],"meta":{"status":404,"msg":"not found","response_id":"meta-id"}}`)
		case "/v1/gifs/bad-json":
			writeJSON(writer, http.StatusOK, `{`)
		case "/v1/gifs/bad-object":
			writeSingle(writer, `{"id":"","type":"gif","url":"bad"}`)
		case "/v1/gifs/mismatch":
			writeSingle(writer, `{"id":"other","type":"gif","url":"https://giphy.com/gifs/other"}`)
		case "/v1/gifs/redirect":
			http.Redirect(writer, request, "/target", http.StatusFound)
		case "/target":
			targetHits++
			writeSingle(writer, `{"id":"redirect","type":"gif","url":"https://giphy.com/gifs/redirect"}`)
		case "/v1/gifs/search":
			writeList(writer, `[{"id":"gif1","type":"gif","url":"https://giphy.com/gifs/gif1"}]`, 0, 1, 2)
		case "/v1/gifs":
			writeList(writer, `[{"id":"other","type":"gif","url":"https://giphy.com/gifs/other"}]`, 0, 1, 1)
		case "/v1/randomid":
			writeSingle(writer, `{"random_id":""}`)
		case "/v1/gifs/categories":
			writeList(writer, `[{"name":"one"}]`, 0, 1, 2)
		case "/v1/gifs/search/tags":
			writeSingle(writer, `[{"name":""}]`)
		case "/v1/trending/searches":
			writeSingle(writer, `[""]`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server)
	ctx := context.Background()

	_, err := client.Get(ctx, GetRequest{ID: "limited"})
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodeRateLimited || platformErr.RetryAfter != 7*time.Second || platformErr.RequestID != "meta-id" || platformErr.PlatformMessage != "slow down" {
		t.Fatalf("limited error=%#v", err)
	}
	tests := []struct {
		id   string
		code socialhub.ErrorCode
	}{
		{"meta-error", socialhub.CodeNotFound}, {"bad-json", socialhub.CodePlatformError},
		{"bad-object", socialhub.CodePlatformError}, {"mismatch", socialhub.CodePlatformError}, {"redirect", socialhub.CodePlatformError},
	}
	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			_, err := client.Get(ctx, GetRequest{ID: test.id})
			if errorCode(err) != test.code {
				t.Fatalf("error=%v", err)
			}
		})
	}
	if targetHits != 0 {
		t.Fatalf("redirect target was followed %d times", targetHits)
	}
	if _, err := client.Search(ctx, SearchRequest{Content: ContentGIF, Query: "cat"}); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("bad pagination=%v", err)
	}
	if _, err := client.GetMany(ctx, GetManyRequest{IDs: []string{"gif1"}}); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("unexpected ID=%v", err)
	}
	if _, err := client.RandomID(ctx); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("empty random ID=%v", err)
	}
	if _, err := client.Categories(ctx, ""); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("category pagination=%v", err)
	}
	if _, err := client.Autocomplete(ctx, TermRequest{Query: "ca"}); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("bad term=%v", err)
	}
	if _, err := client.TrendingSearches(ctx, ""); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("bad trending term=%v", err)
	}
}

func TestWorkflowValidation(t *testing.T) {
	client := &Client{analyticsOrigin: "https://giphy-analytics.giphy.com", clock: fixedClock{testNow}}
	badControl := string([]byte{0})
	tests := []struct {
		name string
		run  func() error
	}{
		{"search content", func() error {
			_, err := client.Search(context.Background(), SearchRequest{Content: "clips", Query: "cat"})
			return err
		}},
		{"search query", func() error {
			_, err := client.Search(context.Background(), SearchRequest{Content: ContentGIF, Query: strings.Repeat("x", 51)})
			return err
		}},
		{"search channel", func() error {
			_, err := client.Search(context.Background(), SearchRequest{Content: ContentGIF, Query: "cat", ChannelIDs: []int64{-1}})
			return err
		}},
		{"search common", func() error {
			_, err := client.Search(context.Background(), SearchRequest{Content: ContentGIF, Query: "cat", CommonOptions: CommonOptions{Rating: "adult"}})
			return err
		}},
		{"trending", func() error {
			_, err := client.Trending(context.Background(), TrendingRequest{Content: ContentGIF, Offset: 500})
			return err
		}},
		{"translate", func() error {
			_, err := client.Translate(context.Background(), TranslateRequest{Content: ContentGIF})
			return err
		}},
		{"random", func() error {
			_, err := client.Random(context.Background(), RandomRequest{Content: ContentGIF, Tag: badControl})
			return err
		}},
		{"get", func() error { _, err := client.Get(context.Background(), GetRequest{ID: "bad/id"}); return err }},
		{"many empty", func() error { _, err := client.GetMany(context.Background(), GetManyRequest{}); return err }},
		{"many duplicate", func() error {
			_, err := client.GetMany(context.Background(), GetManyRequest{IDs: []string{"one", "one"}})
			return err
		}},
		{"many id", func() error {
			_, err := client.GetMany(context.Background(), GetManyRequest{IDs: []string{"bad/id"}})
			return err
		}},
		{"category customer", func() error { _, err := client.Categories(context.Background(), " bad"); return err }},
		{"autocomplete", func() error {
			_, err := client.Autocomplete(context.Background(), TermRequest{Query: "", Limit: 51})
			return err
		}},
		{"related", func() error { _, err := client.Related(context.Background(), "bad/term", ""); return err }},
		{"trending customer", func() error { _, err := client.TrendingSearches(context.Background(), " bad"); return err }},
		{"analytics customer", func() error {
			return client.Register(context.Background(), AnalyticsRequest{TrackingURL: "https://giphy-analytics.giphy.com/v2/pingback_simple"})
		}},
		{"analytics origin", func() error {
			return client.Register(context.Background(), AnalyticsRequest{TrackingURL: "https://evil.example/v2/pingback_simple?analytics_response_payload=x&action_type=SEEN", CustomerID: "customer"})
		}},
		{"analytics parameters", func() error {
			return client.Register(context.Background(), AnalyticsRequest{TrackingURL: "https://giphy-analytics.giphy.com/v2/pingback_simple", CustomerID: "customer"})
		}},
		{"analytics timestamp", func() error {
			return client.Register(context.Background(), AnalyticsRequest{TrackingURL: "https://giphy-analytics.giphy.com/v2/pingback_simple?analytics_response_payload=x&action_type=SEEN", CustomerID: "customer", Timestamp: time.Unix(0, 0)})
		}},
		{"upload reader", func() error {
			_, err := client.Upload(context.Background(), UploadRequest{Filename: "x.gif", MIME: "image/gif", Size: 1}, nil)
			return err
		}},
		{"upload filename", func() error {
			_, err := client.Upload(context.Background(), UploadRequest{Filename: "../x.gif", MIME: "image/gif", Size: 1}, strings.NewReader("x"))
			return err
		}},
		{"upload MIME", func() error {
			_, err := client.Upload(context.Background(), UploadRequest{Filename: "x.png", MIME: "image/png", Size: 1}, strings.NewReader("x"))
			return err
		}},
		{"upload size", func() error {
			_, err := client.Upload(context.Background(), UploadRequest{Filename: "x.gif", MIME: "image/gif", Size: maxUploadBytes + 1}, strings.NewReader("x"))
			return err
		}},
		{"upload metadata", func() error {
			_, err := client.Upload(context.Background(), UploadRequest{Filename: "x.gif", MIME: "image/gif", Size: 1, CountryCode: "us"}, strings.NewReader("x"))
			return err
		}},
		{"upload tag", func() error {
			_, err := client.Upload(context.Background(), UploadRequest{Filename: "x.gif", MIME: "image/gif", Size: 1, Tags: []string{"bad,tag"}}, strings.NewReader("x"))
			return err
		}},
		{"upload tags length", func() error {
			_, err := client.Upload(context.Background(), UploadRequest{Filename: "x.gif", MIME: "image/gif", Size: 1, Tags: repeatedTags(50, strings.Repeat("x", 100))}, strings.NewReader("x"))
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if code := errorCode(test.run()); code != socialhub.CodeInvalidArgument {
				t.Fatalf("code=%q", code)
			}
		})
	}
	if normalizedURLOrigin(nil) != "" {
		t.Fatal("nil URL must have empty origin")
	}
}

func TestNumberAndErrorHelpers(t *testing.T) {
	for input, want := range map[string]Number{`"42"`: 42, `42`: 42, `null`: 0, `""`: 0} {
		var number Number
		if err := number.UnmarshalJSON([]byte(input)); err != nil || number != want {
			t.Fatalf("input=%s number=%d err=%v", input, number, err)
		}
	}
	for _, input := range []string{`-1`, `"bad"`, `{}`} {
		var number Number
		if err := number.UnmarshalJSON([]byte(input)); err == nil {
			t.Fatalf("input=%s must fail", input)
		}
	}
	classifications := map[int]socialhub.ErrorCode{
		http.StatusBadRequest: socialhub.CodeInvalidArgument, http.StatusRequestURITooLong: socialhub.CodeInvalidArgument,
		http.StatusUnauthorized: socialhub.CodeUnauthenticated, http.StatusForbidden: socialhub.CodePermissionDenied,
		http.StatusGone: socialhub.CodeNotFound, http.StatusConflict: socialhub.CodeConflict,
		http.StatusTooManyRequests: socialhub.CodeRateLimited, http.StatusServiceUnavailable: socialhub.CodeTemporarilyUnavailable,
		http.StatusTeapot: socialhub.CodePlatformError,
	}
	for status, want := range classifications {
		if got, _ := classifyError(status); got != want {
			t.Fatalf("status=%d got=%q want=%q", status, got, want)
		}
	}
	if parseRetryAfter("7.5") != 7500*time.Millisecond || parseRetryAfter("bad") != 0 || parseRetryAfter("86401") != 0 {
		t.Fatal("Retry-After parsing failed")
	}
	if boundedMessage(strings.Repeat("界", 4), 2) != "界界" || firstNonEmpty("", "value") != "value" {
		t.Fatal("message helpers failed")
	}
}

func repeatedTags(count int, value string) []string {
	result := make([]string, count)
	for index := range result {
		result[index] = value
	}
	return result
}

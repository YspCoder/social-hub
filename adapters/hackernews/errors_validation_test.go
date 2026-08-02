package hackernews

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestHTTPErrorMapping(t *testing.T) {
	tests := []struct {
		status int
		code   socialhub.ErrorCode
		class  socialhub.ErrorClass
	}{
		{http.StatusBadRequest, socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{http.StatusUnauthorized, socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{http.StatusForbidden, socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{http.StatusNotFound, socialhub.CodeNotFound, socialhub.ClassPermanent},
		{http.StatusConflict, socialhub.CodeConflict, socialhub.ClassPermanent},
		{http.StatusTooManyRequests, socialhub.CodeRateLimited, socialhub.ClassRetryable},
		{http.StatusBadGateway, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{http.StatusTeapot, socialhub.CodePlatformError, socialhub.ClassPermanent},
	}
	for _, test := range tests {
		code, class := classifyHTTPError(test.status)
		if code != test.code || class != test.class {
			t.Fatalf("status %d=%s/%s", test.status, code, class)
		}
	}
	header := make(http.Header)
	header.Set("Retry-After", "7")
	header.Set("CF-Ray", "ray-1")
	err := decodeHTTPError(http.StatusTooManyRequests, header, []byte(`{"error":"slow down"}`))
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || !errors.Is(err, socialhub.ErrRateLimited) || platformErr.RetryAfter != 7*time.Second ||
		platformErr.RequestID != "ray-1" || platformErr.PlatformMessage != "slow down" {
		t.Fatalf("error=%#v", platformErr)
	}
	if parseRetryAfter("bad") != 0 || parseRetryAfter("90000") != 0 || parseRetryAfter("0") != 0 {
		t.Fatal("invalid Retry-After accepted")
	}
}

func TestValidationAndPlatformResponseErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/topstories.json":
			writeJSON(writer, http.StatusOK, `[0]`)
		case "/api/newstories.json":
			writeJSON(writer, http.StatusOK, `[101]`)
		case "/api/beststories.json":
			writeJSON(writer, http.StatusOK, `[201]`)
		case "/api/item/101.json":
			writeJSON(writer, http.StatusOK, `{"id":999,"type":"story"}`)
		case "/api/item/201.json":
			writeJSON(writer, http.StatusOK, `{"id":201,"type":"comment","parent":1}`)
		case "/api/item/202.json":
			writeJSON(writer, http.StatusOK, `{"id":202,"type":"job","title":"job"}`)
		case "/api/item/301.json":
			writeJSON(writer, http.StatusOK, `{"id":301,"type":"story","kids":[0]}`)
		case "/api/item/302.json":
			writeJSON(writer, http.StatusOK, `{"id":302,"type":"unknown"}`)
		case "/api/item/303.json":
			writeJSON(writer, http.StatusOK, `{"id":303,"type":"story","kids":[202]}`)
		case "/api/maxitem.json":
			writeJSON(writer, http.StatusOK, `0`)
		case "/api/updates.json":
			writeJSON(writer, http.StatusOK, `{"items":[0],"profiles":["bad/name"]}`)
		case "/api/user/alice.json":
			writeJSON(writer, http.StatusOK, `{"id":"bob","created":1,"karma":1}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server)

	if _, err := client.GetItem(context.Background(), 0); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("zero item=%v", err)
	}
	for _, id := range []int64{101, 301, 302} {
		if _, err := client.GetItem(context.Background(), id); errorCode(err) != socialhub.CodePlatformError {
			t.Fatalf("invalid item %d=%v", id, err)
		}
	}
	if _, err := client.ListFeed(context.Background(), FeedRequest{Feed: "bad"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("invalid feed=%v", err)
	}
	if _, err := client.ListFeed(context.Background(), FeedRequest{Feed: FeedTop}); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("invalid feed IDs=%v", err)
	}
	if _, err := client.ListFeed(context.Background(), FeedRequest{Feed: FeedNew}); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("mismatched feed item=%v", err)
	}
	if _, err := client.ListFeed(context.Background(), FeedRequest{Feed: FeedBest}); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("non-post feed item=%v", err)
	}
	if _, err := client.ListFeed(context.Background(), FeedRequest{Feed: FeedNew, MaxResults: 101}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("oversized page=%v", err)
	}
	if _, err := client.ListFeed(context.Background(), FeedRequest{Feed: FeedNew, Cursor: "01"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("noncanonical cursor=%v", err)
	}
	if _, err := client.ListChildren(context.Background(), ChildrenRequest{}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("zero parent=%v", err)
	}
	if _, err := client.ListChildren(context.Background(), ChildrenRequest{ParentID: 303, Cursor: "2"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("out-of-range cursor=%v", err)
	}
	if _, err := client.MaxItemID(context.Background()); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("bad max ID=%v", err)
	}
	if _, err := client.GetUserProfile(context.Background(), "bad/name"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("bad username=%v", err)
	}
	if _, err := client.GetUserProfile(context.Background(), "alice"); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("mismatched user=%v", err)
	}
	if _, err := client.GetUpdates(context.Background()); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("bad updates=%v", err)
	}
	if _, err := client.GetPost(context.Background(), "01"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("bad post ID=%v", err)
	}
	if _, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("missing comment post=%v", err)
	}
	if _, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "303"}); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("non-comment child=%v", err)
	}
}

func TestCallOptionsTimeFilterMalformedJSONAndRedirectRefusal(t *testing.T) {
	var targetCalls atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		targetCalls.Add(1)
		writeJSON(writer, http.StatusOK, `1`)
	}))
	defer target.Close()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch calls.Add(1) {
		case 1:
			writeJSON(writer, http.StatusOK, `{`)
		case 2:
			writer.Header().Set("Retry-After", "3")
			writeJSON(writer, http.StatusTooManyRequests, `{"message":"busy"}`)
		default:
			http.Redirect(writer, request, target.URL, http.StatusFound)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server)

	if _, err := client.MaxItemID(context.Background()); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("malformed JSON=%v", err)
	}
	_, err := client.MaxItemID(context.Background())
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || !errors.Is(err, socialhub.ErrRateLimited) || platformErr.Op != "max_item_id" || platformErr.RetryAfter != 3*time.Second {
		t.Fatalf("rate error=%#v", platformErr)
	}
	if _, err := client.MaxItemID(context.Background()); err == nil {
		t.Fatal("redirect should fail")
	}
	if targetCalls.Load() != 0 {
		t.Fatalf("redirect target calls=%d", targetCalls.Load())
	}

	server2 := httptest.NewServer(http.NotFoundHandler())
	defer server2.Close()
	_, optionsClient := newTestClient(t, server2)
	if _, err := optionsClient.GetItem(context.Background(), 1, socialhub.WithFields("title")); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("fields=%v", err)
	}
	if _, err := optionsClient.GetItem(context.Background(), 1, socialhub.WithIdempotencyKey("key")); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("idempotency=%v", err)
	}
	now := time.Now()
	if _, err := optionsClient.ListPosts(context.Background(), socialhub.ListPostsRequest{StartTime: &now}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("time filter=%v", err)
	}
}

func errorCode(err error) socialhub.ErrorCode {
	var platformErr *socialhub.Error
	if errors.As(err, &platformErr) {
		return platformErr.Code
	}
	return ""
}

func TestBoundedMessages(t *testing.T) {
	long := strings.Repeat("界", 600)
	if len([]rune(bounded(long, 512))) != 512 || firstNonEmpty("", " value ") != " value " {
		t.Fatal("message helpers failed")
	}
}

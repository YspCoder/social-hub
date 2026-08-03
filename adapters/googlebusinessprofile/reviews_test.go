package googlebusinessprofile

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestReviewWorkflow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/v4/accounts/1001/locations/2002/reviews/"+testReviewID:
			writeJSON(writer, http.StatusOK, reviewJSON(testReviewID))
		case request.Method == http.MethodGet && request.URL.Path == "/v4/accounts/1001/locations/2002/reviews":
			if request.URL.Query().Get("pageSize") != "25" || request.URL.Query().Get("pageToken") != "review-page" {
				t.Errorf("list query=%v", request.URL.Query())
			}
			writeJSON(writer, http.StatusOK, `{"reviews":[`+reviewJSON(testReviewID)+`],"averageRating":4.7,"totalReviewCount":42,"nextPageToken":"next-review-page"}`)
		case request.Method == http.MethodPut && request.URL.Path == "/v4/accounts/1001/locations/2002/reviews/"+testReviewID+"/reply":
			var body ReviewReply
			if json.NewDecoder(request.Body).Decode(&body) != nil || body.Comment != "We appreciate your feedback" {
				t.Errorf("reply body=%#v", body)
			}
			writeJSON(writer, http.StatusOK, `{"comment":"We appreciate your feedback","updateTime":"2026-08-03T02:00:00Z","reviewReplyState":"PENDING"}`)
		case request.Method == http.MethodDelete && request.URL.Path == "/v4/accounts/1001/locations/2002/reviews/"+testReviewID+"/reply":
			writer.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, []string{businessScope})
	review, err := client.GetReview(context.Background(), testReviewID)
	if err != nil || review.ID != testReviewID || review.Reviewer.DisplayName != "Ada" || review.StarRating != StarFive ||
		review.ReviewReply == nil || review.ReviewReply.ReviewReplyState != "APPROVED" || len(review.Raw) == 0 {
		t.Fatalf("review=%#v err=%v", review, err)
	}
	page, err := client.ListReviews(context.Background(), ReviewListRequest{Cursor: "review-page", MaxResults: 25})
	if err != nil || len(page.Items) != 1 || page.AverageRating != 4.7 || page.TotalReviewCount != 42 ||
		!page.HasMore || page.NextCursor == nil || *page.NextCursor != "next-review-page" {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	reply, err := client.UpdateReviewReply(context.Background(), testReviewID, "We appreciate your feedback")
	if err != nil || reply.Comment != "We appreciate your feedback" || reply.ReviewReplyState != "PENDING" || reply.UpdateTime == nil {
		t.Fatalf("reply=%#v err=%v", reply, err)
	}
	if err := client.DeleteReviewReply(context.Background(), testReviewID); err != nil {
		t.Fatal(err)
	}
}

func TestReviewValidationAndOwnership(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/v4/accounts/1001/locations/2002/reviews/" + testReviewID:
			writeJSON(writer, http.StatusOK, reviewJSON("other-review"))
		case "/v4/accounts/1001/locations/2002/reviews":
			writeJSON(writer, http.StatusOK, `{"reviews":[`+reviewJSON(testReviewID)+`],"nextPageToken":"\n"}`)
		case "/v4/accounts/1001/locations/2002/reviews/empty-reply/reply":
			writeJSON(writer, http.StatusOK, `{"comment":""}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, nil)
	if _, err := client.GetReview(context.Background(), "bad/id"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("review ID error=%v", err)
	}
	if _, err := client.GetReview(context.Background(), testReviewID); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("ownership error=%v", err)
	}
	if _, err := client.ListReviews(context.Background(), ReviewListRequest{MaxResults: 51}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("page size error=%v", err)
	}
	if _, err := client.ListReviews(context.Background(), ReviewListRequest{Cursor: "\n"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("cursor error=%v", err)
	}
	if _, err := client.ListReviews(context.Background(), ReviewListRequest{}); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("response cursor error=%v", err)
	}
	if _, err := client.UpdateReviewReply(context.Background(), "bad/id", "reply"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("reply ID error=%v", err)
	}
	if _, err := client.UpdateReviewReply(context.Background(), testReviewID, ""); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("empty reply error=%v", err)
	}
	if _, err := client.UpdateReviewReply(context.Background(), testReviewID, strings.Repeat("x", 4097)); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("long reply error=%v", err)
	}
	if _, err := client.UpdateReviewReply(context.Background(), "empty-reply", "reply"); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("malformed reply error=%v", err)
	}
	if err := client.DeleteReviewReply(context.Background(), "bad/id"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("delete error=%v", err)
	}
}

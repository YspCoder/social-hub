package strava

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestFetcherMappingPaginationAndCallOptions(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer access-token" {
			t.Errorf("authorization=%q", request.Header.Get("Authorization"))
		}
		switch request.URL.Path {
		case "/api/athlete":
			if request.Header.Get("X-Request-ID") != "request-1" {
				t.Errorf("request ID=%q", request.Header.Get("X-Request-ID"))
			}
			writeJSON(writer, http.StatusOK, `{"id":`+testAthleteID+`,"username":"rider","firstname":"Ada","lastname":"Lovelace","profile":"https://cdn.example/profile.jpg","city":"Shanghai","country":"CN"}`)
		case "/api/activities/" + testActivityID:
			writeJSON(writer, http.StatusOK, activityJSON(testActivityID, testAthleteID))
		case "/api/athlete/activities":
			query := request.URL.Query()
			if query.Get("page") != "2" || query.Get("per_page") != "2" || query.Get("after") != "1785542400" || query.Get("before") != "1785715200" {
				t.Errorf("activity query=%v", query)
			}
			writeJSON(writer, http.StatusOK, `[`+activityJSON(testActivityID, testAthleteID)+`,`+activityJSON("789012345678902", testAthleteID)+`]`)
		case "/api/activities/" + testActivityID + "/comments":
			query := request.URL.Query()
			if query.Get("page_size") != "2" || query.Get("after_cursor") != "cursor%20one" {
				t.Errorf("comment query=%v", query)
			}
			writeJSON(writer, http.StatusOK, `[{"id":1,"activity_id":`+testActivityID+`,"text":"Nice ride","created_at":"2026-08-02T13:00:00Z","athlete":{"id":22,"firstname":"Grace"},"cursor":"next-1"},{"id":2,"activity_id":`+testActivityID+`,"text":"Strong","created_at":"2026-08-02T14:00:00Z","athlete":{"firstname":"Private"},"cursor":"next-2"}]`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, false, []string{"read", "activity:read_all", "activity:write"})

	user, err := client.GetUser(context.Background(), "me", socialhub.WithRequestID("request-1"))
	if err != nil || user.ID != testAthleteID || user.Username == nil || *user.Username != "rider" || user.DisplayName == nil || *user.DisplayName != "Ada Lovelace" || user.ProfileURL == nil || len(user.Extensions["strava.athlete"]) == 0 {
		t.Fatalf("user=%#v err=%v", user, err)
	}
	post, err := client.GetPost(context.Background(), testActivityID)
	if err != nil || post.ID != testActivityID || post.AuthorID == nil || *post.AuthorID != testAthleteID || post.Text == nil || *post.Text != "Steady effort" || post.Visibility == nil || *post.Visibility != "followers_or_everyone" || len(post.Metrics) != 5 || len(post.Extensions["strava.activity"]) == 0 {
		t.Fatalf("post=%#v err=%v", post, err)
	}
	page, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{
		UserID: testAthleteID, Cursor: "2", MaxResults: 2, StartTime: &start, EndTime: &end,
	})
	if err != nil || len(page.Items) != 2 || !page.HasMore || page.NextCursor == nil || *page.NextCursor != "3" {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	comments, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: testActivityID, Cursor: "cursor%20one", MaxResults: 2})
	if err != nil || len(comments.Items) != 2 || !comments.HasMore || comments.NextCursor == nil || *comments.NextCursor != "next-2" || comments.Items[0].AuthorID == nil || *comments.Items[0].AuthorID != "22" || comments.Items[1].AuthorID != nil {
		t.Fatalf("comments=%#v err=%v", comments, err)
	}
}

func TestFetcherValidationAndResponseOwnership(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/athlete":
			writeJSON(writer, http.StatusOK, `{"id":999}`)
		case "/api/activities/" + testActivityID:
			writeJSON(writer, http.StatusOK, activityJSON(testActivityID, "999"))
		case "/api/athlete/activities":
			writeJSON(writer, http.StatusOK, `[`+activityJSON(testActivityID, "999")+`]`)
		case "/api/activities/" + testActivityID + "/comments":
			writeJSON(writer, http.StatusOK, `[{"id":1,"activity_id":2,"text":"wrong","cursor":"x"}]`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, false, nil)

	if _, err := client.GetUser(context.Background(), "999"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("user filter error=%v", err)
	}
	if _, err := client.GetUser(context.Background(), "me"); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("user ownership error=%v", err)
	}
	if _, err := client.GetPost(context.Background(), "bad"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("post ID error=%v", err)
	}
	if _, err := client.GetPost(context.Background(), testActivityID); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("post ownership error=%v", err)
	}
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{UserID: "999"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("user list error=%v", err)
	}
	now := time.Now()
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{StartTime: &now, EndTime: &now}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("time error=%v", err)
	}
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{Cursor: "zero"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("cursor error=%v", err)
	}
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{MaxResults: 201}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("page size error=%v", err)
	}
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{}); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("list ownership error=%v", err)
	}
	if _, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "bad"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("comment post error=%v", err)
	}
	if _, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: testActivityID, Cursor: "\n"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("comment cursor error=%v", err)
	}
	if _, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: testActivityID}); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("comment ownership error=%v", err)
	}
}

func TestRecordedScopesAreEnforced(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("request must not be sent without required scope")
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, false, []string{"read"})
	if _, err := client.GetPost(context.Background(), testActivityID); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("fetch scope error=%v", err)
	}
	if _, err := client.CreateManualActivity(context.Background(), ManualActivityRequest{
		Name: "Ride", SportType: SportRide, StartDateLocal: testNow, ElapsedTime: time.Hour,
	}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("write scope error=%v", err)
	}
}

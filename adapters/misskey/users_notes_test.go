package misskey

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestUserNoteTimelineAndReplyContracts(t *testing.T) {
	withRenotes := false
	calls := make(map[string]int)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.Header.Get("Authorization") != "Bearer access-token" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		calls[request.URL.Path]++
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Errorf("decode %s: %v", request.URL.Path, err)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		switch request.URL.Path {
		case "/api/i":
			if len(body) != 0 {
				t.Errorf("i body=%v", body)
			}
			writeTestJSON(t, writer, testUser("user-1"))
		case "/api/users/show":
			if body["userId"] != "user-2" {
				t.Errorf("users/show body=%v", body)
			}
			user := testUser("user-2")
			user["host"], user["isBot"] = "remote.example", true
			writeTestJSON(t, writer, user)
		case "/api/notes/show":
			if body["noteId"] != "note-1" || request.Header.Get("X-Request-ID") != "request-1" {
				t.Errorf("notes/show body=%v request-id=%q", body, request.Header.Get("X-Request-ID"))
			}
			note := testNote("note-1", "hello")
			note["replyId"] = "root-1"
			note["files"] = []any{testDriveFile("file-1", "image/png")}
			writeTestJSON(t, writer, note)
		case "/api/users/notes":
			if body["userId"] != "user-2" || body["untilId"] != "note-cursor" || body["limit"] != float64(1) ||
				body["sinceDate"] != float64(testNow.Add(-time.Hour).UnixMilli()) {
				t.Errorf("users/notes body=%v", body)
			}
			writeTestJSON(t, writer, []any{testNote("note-page-1", "page")})
		case "/api/notes/replies":
			if body["noteId"] != "root-1" || body["limit"] != float64(1) {
				t.Errorf("notes/replies body=%v", body)
			}
			reply := testNote("reply-1", "reply")
			reply["replyId"] = "root-1"
			writeTestJSON(t, writer, []any{reply})
		case "/api/notes/timeline":
			if body["limit"] != float64(2) || body["untilDate"] != float64(testNow.UnixMilli()) ||
				body["withFiles"] != true || body["withRenotes"] != withRenotes {
				t.Errorf("notes/timeline body=%v", body)
			}
			writeTestJSON(t, writer, []any{testNote("home-1", "home")})
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, allTestPermissions())

	current, err := client.GetUser(context.Background(), "")
	if err != nil || current.ID != "user-1" || current.Username == nil || *current.Username != "alice" {
		t.Fatalf("current=%#v err=%v", current, err)
	}
	remote, err := client.GetUser(context.Background(), "user-2")
	if err != nil || remote.Username == nil || *remote.Username != "alice@remote.example" || remote.AccountType == nil || *remote.AccountType != "bot" {
		t.Fatalf("remote=%#v err=%v", remote, err)
	}
	post, err := client.GetPost(context.Background(), "note-1", socialhub.WithRequestID("request-1"))
	if err != nil || post.ID != "note-1" || len(post.Media) != 1 || len(post.Relations) != 1 || post.Relations[0].Type != socialhub.RelationReply {
		t.Fatalf("post=%#v err=%v", post, err)
	}
	page, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{
		UserID: "user-2", Cursor: "note-cursor", MaxResults: 1, StartTime: timePointer(testNow.Add(-time.Hour)),
	})
	if err != nil || len(page.Items) != 1 || !page.HasMore || page.NextCursor == nil || *page.NextCursor != "note-page-1" {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	comments, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "root-1", MaxResults: 1})
	if err != nil || len(comments.Items) != 1 || !comments.HasMore || comments.Items[0].ParentID == nil || *comments.Items[0].ParentID != "root-1" {
		t.Fatalf("comments=%#v err=%v", comments, err)
	}
	home, err := client.HomeTimeline(context.Background(), TimelineRequest{
		MaxResults: 2, EndTime: timePointer(testNow), WithFiles: true, WithRenotes: &withRenotes,
	})
	if err != nil || len(home.Items) != 1 || home.HasMore {
		t.Fatalf("home=%#v err=%v", home, err)
	}
	for _, path := range []string{"/api/i", "/api/users/show", "/api/notes/show", "/api/users/notes", "/api/notes/replies", "/api/notes/timeline"} {
		if calls[path] != 1 {
			t.Fatalf("calls[%s]=%d", path, calls[path])
		}
	}
}

func TestFetchAndPaginationValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, allTestPermissions())
	if _, err := client.GetUser(context.Background(), " bad"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("bad user=%v", err)
	}
	if _, err := client.GetPost(context.Background(), ""); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("bad post=%v", err)
	}
	invalidPosts := []socialhub.ListPostsRequest{
		{MaxResults: -1},
		{Cursor: "cursor", EndTime: timePointer(testNow)},
		{StartTime: timePointer(testNow), EndTime: timePointer(testNow.Add(-time.Hour))},
		{UserID: " bad"},
	}
	for index, input := range invalidPosts {
		if _, err := client.ListPosts(context.Background(), input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("invalid posts %d=%v", index, err)
		}
	}
	if _, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("bad comments=%v", err)
	}
	if _, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "note", MaxResults: -1}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("bad comment page=%v", err)
	}
	if _, err := client.HomeTimeline(context.Background(), TimelineRequest{MaxResults: -1}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("bad home=%v", err)
	}
}

func TestMalformedFetchResponses(t *testing.T) {
	tests := []struct {
		name string
		path string
		body any
		call func(*Client) error
	}{
		{name: "current user mismatch", path: "/api/i", body: testUser("other"), call: func(client *Client) error { _, err := client.GetUser(context.Background(), ""); return err }},
		{name: "post mismatch", path: "/api/notes/show", body: testNote("other", "x"), call: func(client *Client) error { _, err := client.GetPost(context.Background(), "note"); return err }},
		{name: "invalid note", path: "/api/notes/show", body: map[string]any{"id": "note"}, call: func(client *Client) error { _, err := client.GetPost(context.Background(), "note"); return err }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path != test.path {
					writer.WriteHeader(http.StatusNotFound)
					return
				}
				writeTestJSON(t, writer, test.body)
			}))
			defer server.Close()
			_, client := newTestClient(t, server, allTestPermissions())
			if err := test.call(client); errorCode(err) != socialhub.CodePlatformError {
				t.Fatalf("error=%v code=%s", err, errorCode(err))
			}
		})
	}
}

func timePointer(value time.Time) *time.Time { return &value }

package vimeo

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

func TestFetchFeedAndReactionContracts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer access-token" || request.Header.Get("Accept") != vimeoAccept {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/api/me":
			if request.Header.Get("X-Request-ID") != "fetch-request" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, `{"uri":"/users/user-1","name":"Creator","link":"https://vimeo.com/creator","location":"Shanghai","bio":"bio","account":"pro","resource_key":"user-key","pictures":{"sizes":[{"width":100,"height":100,"link":"https://cdn.example/small.jpg"},{"width":400,"height":400,"link":"https://cdn.example/avatar.jpg"}]},"websites":[{"name":"Site","link":"https://example.com"}]}`)
		case request.Method == http.MethodGet && request.URL.Path == "/api/users/user-2":
			writeJSON(writer, `{"uri":"/users/user-2","name":"Other","link":"https://vimeo.com/other"}`)
		case request.Method == http.MethodGet && request.URL.Path == "/api/videos/video-1":
			writeJSON(writer, videoJSON("video-1", "available"))
		case request.Method == http.MethodGet && request.URL.Path == "/api/me/videos":
			if request.URL.Query().Get("page") != "2" || request.URL.Query().Get("per_page") != "100" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, `{"total":3,"page":2,"per_page":100,"paging":{"next":"/me/videos?page=3","previous":"/me/videos?page=1"},"data":[`+videoJSON("video-2", "transcoding")+`]}`)
		case request.Method == http.MethodGet && request.URL.Path == "/api/users/user-2/feed":
			writeJSON(writer, `{"paging":{},"data":[{"type":"like","time":"2026-08-02T01:02:03Z","user":{"uri":"/users/user-2"},"clip":`+videoJSON("video-4", "available")+`}]}`)
		case request.Method == http.MethodGet && request.URL.Path == "/api/videos/video-1/comments":
			writeJSON(writer, `{"paging":{"next":"/videos/video-1/comments?page=2"},"data":[{"uri":"/videos/video-1/comments/comment-1","type":"video","text":"great","created_on":"2026-08-02T01:02:03Z","resource_key":"comment-key","metadata":{"connections":{"user":{"uri":"/users/user-2"}}}}]}`)
		case request.Method == http.MethodPut && request.URL.Path == "/api/me/likes/video-1":
			writer.WriteHeader(http.StatusNoContent)
		case request.Method == http.MethodDelete && request.URL.Path == "/api/me/likes/video-1":
			writer.WriteHeader(http.StatusNoContent)
		case request.Method == http.MethodPost && request.URL.Path == "/api/videos/video-1/comments":
			var body map[string]string
			if json.NewDecoder(request.Body).Decode(&body) != nil || body["text"] != "thanks" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, `{"uri":"/videos/video-1/comments/comment-2","type":"video","text":"thanks","user":{"uri":"/users/user-1"}}`)
		case request.Method == http.MethodPost && request.URL.Path == "/api/videos/video-1/comments/comment-2/replies":
			writeJSON(writer, `{"uri":"/videos/video-1/comments/reply-1","type":"video","text":"reply","user":{"uri":"/users/user-1"}}`)
		case request.Method == http.MethodDelete && request.URL.Path == "/api/videos/video-1/comments/comment-2":
			writer.WriteHeader(http.StatusNoContent)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, []string{"public", "interact", "delete"})

	user, err := client.GetUser(context.Background(), "", socialhub.WithRequestID("fetch-request"))
	if err != nil || user.ID != "user-1" || user.DisplayName == nil || *user.DisplayName != "Creator" || user.AvatarURL == nil || *user.AvatarURL != "https://cdn.example/avatar.jpg" {
		t.Fatalf("user=%#v err=%v", user, err)
	}
	other, err := client.GetUser(context.Background(), "user-2")
	if err != nil || other.ID != "user-2" {
		t.Fatalf("other=%#v err=%v", other, err)
	}
	post, err := client.GetPost(context.Background(), "video-1")
	if err != nil || post.Status.State != socialhub.PublishStatePublished || len(post.Media) != 1 || post.Media[0].Duration == nil || *post.Media[0].Duration != 62*time.Second || len(post.Metrics) != 3 {
		t.Fatalf("post=%#v err=%v", post, err)
	}
	page, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{Cursor: "2", MaxResults: 150})
	if err != nil || len(page.Items) != 1 || page.Items[0].Status.State != socialhub.PublishStatePending || page.NextCursor == nil || *page.NextCursor != "3" || page.PrevCursor == nil || *page.PrevCursor != "1" || !page.HasMore {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	feed, err := client.FeedWorkflow().HomeFeed(context.Background(), socialhub.ListPostsRequest{UserID: "user-2"})
	if err != nil || len(feed.Items) != 1 || feed.HasMore || feed.Items[0].Extensions["vimeo.activity"] == nil {
		t.Fatalf("feed=%#v err=%v", feed, err)
	}
	comments, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "video-1", MaxResults: 25})
	if err != nil || len(comments.Items) != 1 || comments.Items[0].ID != "video-1/comment-1" || comments.Items[0].AuthorID == nil || *comments.Items[0].AuthorID != "user-2" || comments.NextCursor == nil || *comments.NextCursor != "2" {
		t.Fatalf("comments=%#v err=%v", comments, err)
	}
	reaction := socialhub.ReactionRequest{ActorID: "user-1", TargetID: "video-1", Kind: socialhub.ReactionLike}
	if err := client.React(context.Background(), reaction); err != nil {
		t.Fatal(err)
	}
	if err := client.RemoveReaction(context.Background(), reaction); err != nil {
		t.Fatal(err)
	}
	comment, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: "video-1", Text: "thanks"})
	if err != nil || comment.ID != "video-1/comment-2" {
		t.Fatalf("comment=%#v err=%v", comment, err)
	}
	parent := comment.ID
	reply, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: "video-1", ParentID: &parent, Text: "reply"})
	if err != nil || reply.ParentID == nil || *reply.ParentID != comment.ID || reply.ID != "video-1/reply-1" {
		t.Fatalf("reply=%#v err=%v", reply, err)
	}
	if err := client.DeleteComment(context.Background(), comment.ID); err != nil {
		t.Fatal(err)
	}
}

func TestFetchAndReactionValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, []string{"public", "interact", "delete"})
	if _, err := client.GetUser(context.Background(), "bad/id"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("get user error=%v", err)
	}
	if _, err := client.GetPost(context.Background(), ""); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("get post error=%v", err)
	}
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{Cursor: "zero"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("cursor error=%v", err)
	}
	start := testNow
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{StartTime: &start}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("time filter error=%v", err)
	}
	if _, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("comments error=%v", err)
	}
	if err := client.React(context.Background(), socialhub.ReactionRequest{ActorID: "other", TargetID: "video-1", Kind: socialhub.ReactionLike}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("actor error=%v", err)
	}
	if err := client.React(context.Background(), socialhub.ReactionRequest{TargetID: "video-1", Kind: socialhub.ReactionRepost}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("reaction error=%v", err)
	}
	if _, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: "video-1", Text: " "}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("comment error=%v", err)
	}
	parent := "other/comment-1"
	if _, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: "video-1", ParentID: &parent, Text: "reply"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("parent error=%v", err)
	}
	if err := client.DeleteComment(context.Background(), "comment-1"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("delete error=%v", err)
	}
}

func TestMalformedVimeoResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/videos/bad":
			writeJSON(writer, `{"uri":"invalid"}`)
		case "/api/me/videos":
			writeJSON(writer, `{"paging":{"next":"/me/videos?page=bad"},"data":[]}`)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, []string{"public"})
	if _, err := client.GetPost(context.Background(), "bad"); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("mapping error=%v", err)
	}
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{}); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("pagination error=%v", err)
	}
}

func videoJSON(id, status string) string {
	return `{"uri":"/videos/` + id + `","name":"Video","description":"Description","link":"https://vimeo.com/` + id + `","duration":62,"width":1920,"height":1080,"created_time":"2026-08-01T00:00:00Z","modified_time":"2026-08-02T00:00:00Z","release_time":"2026-08-01T01:00:00Z","privacy":{"view":"anybody"},"pictures":{"sizes":[{"width":640,"height":360,"link":"https://cdn.example/thumb.jpg"}]},"stats":{"plays":10},"user":{"uri":"/users/user-1"},"status":"` + status + `","upload":{"status":"complete"},"transcode":{"status":"complete"},"resource_key":"video-key","metadata":{"connections":{"comments":{"total":2},"likes":{"total":3}}}}`
}

func writeJSON(writer http.ResponseWriter, value string) {
	writer.Header().Set("Content-Type", "application/json")
	_, _ = writer.Write([]byte(value))
}

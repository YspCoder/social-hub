package youtube

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

func TestYouTubeFetchAndReactionContracts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer access-token" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/youtube/v3/channels":
			writeJSON(writer, `{"items":[{"id":"channel-1","snippet":{"title":"Channel","customUrl":"@channel","thumbnails":{"high":{"url":"https://cdn.example/avatar.jpg"}}},"statistics":{"subscriberCount":"12","videoCount":"3"}}]}`)
		case request.Method == http.MethodGet && request.URL.Path == "/youtube/v3/videos":
			writeJSON(writer, `{"items":[{"id":"video-1","snippet":{"channelId":"channel-1","title":"Video","description":"hello","publishedAt":"2026-08-01T00:00:00Z","thumbnails":{"high":{"url":"https://cdn.example/thumb.jpg","width":480,"height":360}}},"contentDetails":{"duration":"PT1M2.5S"},"status":{"uploadStatus":"processed","privacyStatus":"public"},"statistics":{"viewCount":"10","likeCount":"2","commentCount":"1"}}]}`)
		case request.Method == http.MethodGet && request.URL.Path == "/youtube/v3/search":
			if request.URL.Query().Get("channelId") != "channel-1" || request.URL.Query().Get("pageToken") != "cursor" || request.URL.Query().Get("publishedAfter") == "" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, `{"nextPageToken":"next","items":[{"id":{"videoId":"video-2"},"snippet":{"channelId":"channel-1","title":"Next","publishedAt":"2026-08-01T00:00:00Z"}}]}`)
		case request.Method == http.MethodGet && request.URL.Path == "/youtube/v3/commentThreads":
			writeJSON(writer, `{"items":[{"snippet":{"videoId":"video-1","topLevelComment":{"id":"comment-1","snippet":{"videoId":"video-1","textOriginal":"nice","authorChannelId":{"value":"author-1"},"publishedAt":"2026-08-01T00:00:01Z","likeCount":1}}},"replies":{"comments":[{"id":"reply-1","snippet":{"parentId":"comment-1","textOriginal":"reply"}}]}}]}`)
		case request.Method == http.MethodPost && request.URL.Path == "/youtube/v3/videos/rate":
			if request.URL.Query().Get("id") != "video-1" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writer.WriteHeader(http.StatusNoContent)
		case request.Method == http.MethodPost && request.URL.Path == "/youtube/v3/commentThreads":
			var body commentThread
			if json.NewDecoder(request.Body).Decode(&body) != nil || body.Snippet.VideoID != "video-1" || body.Snippet.TopLevelComment.Snippet.TextOriginal != "thanks" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, `{"snippet":{"topLevelComment":{"id":"comment-2","snippet":{"videoId":"video-1","textOriginal":"thanks","authorChannelId":{"value":"channel-1"}}}}}`)
		case request.Method == http.MethodDelete && request.URL.Path == "/youtube/v3/comments":
			writer.WriteHeader(http.StatusNoContent)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, []string{"https://www.googleapis.com/auth/youtube.readonly", "https://www.googleapis.com/auth/youtube.force-ssl"})
	user, err := client.GetUser(context.Background(), "channel-1")
	if err != nil || user.ID != "channel-1" || user.DisplayName == nil || *user.DisplayName != "Channel" {
		t.Fatalf("user=%#v err=%v", user, err)
	}
	post, err := client.GetPost(context.Background(), "video-1")
	if err != nil || post.Status.State != socialhub.PublishStatePublished || post.Media[0].Duration == nil || *post.Media[0].Duration != 62500*time.Millisecond {
		t.Fatalf("post=%#v err=%v", post, err)
	}
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	page, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{Cursor: "cursor", MaxResults: 100, StartTime: &start})
	if err != nil || len(page.Items) != 1 || page.NextCursor == nil || *page.NextCursor != "next" {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	comments, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "video-1"})
	if err != nil || len(comments.Items) != 2 || comments.Items[1].ParentID == nil {
		t.Fatalf("comments=%#v err=%v", comments, err)
	}
	reaction := socialhub.ReactionRequest{ActorID: "channel-1", TargetID: "video-1", Kind: socialhub.ReactionLike}
	if err := client.React(context.Background(), reaction); err != nil {
		t.Fatal(err)
	}
	if err := client.RemoveReaction(context.Background(), reaction); err != nil {
		t.Fatal(err)
	}
	comment, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: "video-1", Text: "thanks"})
	if err != nil || comment.ID != "comment-2" {
		t.Fatalf("comment=%#v err=%v", comment, err)
	}
	if err := client.DeleteComment(context.Background(), comment.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{UserID: "other"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("validation error=%v", err)
	}
}

func writeJSON(writer http.ResponseWriter, value string) {
	writer.Header().Set("Content-Type", "application/json")
	_, _ = writer.Write([]byte(value))
}

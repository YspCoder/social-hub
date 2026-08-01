package threads

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestCommonContentAndReplyContracts(t *testing.T) {
	var deleteCalls atomic.Int32
	thread := map[string]any{
		"id": "post-1", "media_product_type": "THREADS", "media_type": "CAROUSEL_ALBUM",
		"permalink": "https://www.threads.com/@alice/post/abc", "owner": map[string]string{"id": "user-1"},
		"username": "alice", "text": "root post", "timestamp": "2026-08-01T10:30:00+0000", "shortcode": "abc",
		"children": map[string]any{"data": []any{
			map[string]any{"id": "image-1", "media_type": "IMAGE", "media_url": "https://cdn.test/image.jpg", "alt_text": "image alt"},
			map[string]any{"id": "video-1", "media_type": "VIDEO", "media_url": "https://cdn.test/video.mp4", "thumbnail_url": "https://cdn.test/video.jpg", "is_spoiler_media": true},
		}},
		"is_reply": true, "root_post": map[string]string{"id": "root-1"}, "replied_to": map[string]string{"id": "parent-1"},
		"is_quote_post": true, "quoted_post": map[string]string{"id": "quote-1"}, "reposted_post": map[string]string{"id": "original-1"},
		"topic_tag": "Go", "is_verified": true,
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("access_token") != "access-token" || request.Header.Get("Authorization") != "" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/me":
			if !strings.Contains(request.URL.Query().Get("fields"), "threads_biography") {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeTestJSON(t, writer, map[string]any{
				"id": "user-1", "username": "alice", "name": "Alice", "is_verified": true,
				"threads_profile_picture_url": "https://cdn.test/avatar.jpg", "threads_biography": "bio",
			})
		case request.Method == http.MethodGet && request.URL.Path == "/post-1":
			writeTestJSON(t, writer, thread)
		case request.Method == http.MethodGet && request.URL.Path == "/new-post":
			writeTestJSON(t, writer, map[string]any{
				"id": "new-post", "media_product_type": "THREADS", "media_type": "TEXT_POST", "owner": map[string]string{"id": "user-1"},
				"username": "alice", "text": "hello", "timestamp": "2026-08-01T12:00:00Z",
			})
		case request.Method == http.MethodGet && request.URL.Path == "/me/threads":
			query := request.URL.Query()
			if query.Get("after") != "cursor-1" || query.Get("limit") != "100" || !strings.Contains(query.Get("fields"), "poll_attachment") {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeTestJSON(t, writer, map[string]any{
				"data": []any{thread}, "paging": map[string]any{"cursors": map[string]string{"before": "prev-1", "after": "next-1"}},
			})
		case request.Method == http.MethodGet && request.URL.Path == "/post-1/replies":
			query := request.URL.Query()
			if query.Get("after") != "reply-cursor" || query.Get("limit") != "20" || query.Get("reverse") != "false" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeTestJSON(t, writer, map[string]any{
				"data": []any{map[string]any{
					"id": "reply-1", "text": "a reply", "timestamp": "2026-08-01T11:00:00+0000",
					"owner": map[string]string{"id": "user-2"}, "is_reply": true,
					"root_post": map[string]string{"id": "post-1"}, "replied_to": map[string]string{"id": "post-1"},
				}}, "paging": map[string]any{"cursors": map[string]string{"after": "reply-next"}},
			})
		case request.Method == http.MethodPost && request.URL.Path == "/me/threads":
			if request.ParseForm() != nil || request.Header.Get("Content-Type") != "application/x-www-form-urlencoded" || request.PostForm.Get("media_type") != "TEXT" || request.PostForm.Get("auto_publish_text") != "true" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			switch request.PostForm.Get("text") {
			case "hello":
				if request.PostForm.Get("reply_to_id") != "parent-1" || request.PostForm.Get("quote_post_id") != "quote-1" {
					writer.WriteHeader(http.StatusBadRequest)
					return
				}
				writeTestJSON(t, writer, map[string]string{"id": "new-post"})
			case "reply text":
				if request.PostForm.Get("reply_to_id") != "reply-parent" {
					writer.WriteHeader(http.StatusBadRequest)
					return
				}
				writeTestJSON(t, writer, map[string]string{"id": "new-reply"})
			default:
				writer.WriteHeader(http.StatusBadRequest)
			}
		case request.Method == http.MethodDelete && (request.URL.Path == "/new-post" || request.URL.Path == "/new-reply"):
			deleteCalls.Add(1)
			writeTestJSON(t, writer, map[string]any{"success": true, "deleted_id": strings.TrimPrefix(request.URL.Path, "/")})
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, allScopes())

	user, err := client.GetUser(context.Background(), "")
	if err != nil || user.ID != "user-1" || user.Username == nil || *user.Username != "alice" || user.ProfileURL == nil || *user.ProfileURL != "https://www.threads.com/@alice" || len(user.Extensions) != 1 {
		t.Fatalf("user=%#v error=%v", user, err)
	}
	post, err := client.GetPost(context.Background(), "post-1")
	if err != nil || post.Text == nil || *post.Text != "root post" || len(post.Media) != 2 || post.Media[0].Type != socialhub.MediaTypeImage || post.Media[1].Type != socialhub.MediaTypeVideo || !hasRelation(*post, socialhub.RelationReply, "parent-1") || !hasRelation(*post, socialhub.RelationQuote, "quote-1") || !hasRelation(*post, socialhub.RelationRepost, "original-1") || post.CreatedAt == nil || post.CreatedAt.Hour() != 10 {
		t.Fatalf("post=%#v error=%v", post, err)
	}
	page, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{Cursor: "cursor-1", MaxResults: 500})
	if err != nil || len(page.Items) != 1 || page.NextCursor == nil || *page.NextCursor != "next-1" || page.PrevCursor == nil || *page.PrevCursor != "prev-1" || !page.HasMore {
		t.Fatalf("posts=%#v error=%v", page, err)
	}
	comments, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "post-1", Cursor: "reply-cursor", MaxResults: 20})
	if err != nil || len(comments.Items) != 1 || comments.Items[0].ID != "reply-1" || comments.Items[0].ParentID == nil || *comments.Items[0].ParentID != "post-1" || comments.NextCursor == nil || *comments.NextCursor != "reply-next" {
		t.Fatalf("comments=%#v error=%v", comments, err)
	}

	text, parent, quote, visibility := "hello", "parent-1", "quote-1", "public"
	created, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text, ReplyToID: &parent, QuotePostID: &quote, Visibility: &visibility})
	if err != nil || created.ID != "new-post" || !hasRelation(*created, socialhub.RelationReply, parent) || !hasRelation(*created, socialhub.RelationQuote, quote) || created.Status == nil || created.Status.State != socialhub.PublishStatePublished {
		t.Fatalf("created=%#v error=%v", created, err)
	}
	status, err := client.PublishStatus(context.Background(), created.ID)
	if err != nil || status.ID != "new-post" || status.UpdatedAt == nil {
		t.Fatalf("status=%#v error=%v", status, err)
	}
	comment, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: "post-1", ParentID: stringPointer("reply-parent"), Text: "reply text"})
	if err != nil || comment.ID != "new-reply" || comment.PostID != "post-1" || comment.ParentID == nil || *comment.ParentID != "reply-parent" {
		t.Fatalf("comment=%#v error=%v", comment, err)
	}
	if err := client.DeleteComment(context.Background(), comment.ID); err != nil {
		t.Fatal(err)
	}
	if err := client.DeletePost(context.Background(), created.ID); err != nil {
		t.Fatal(err)
	}
	if deleteCalls.Load() != 2 {
		t.Fatalf("delete calls=%d", deleteCalls.Load())
	}
	if err := client.React(context.Background(), socialhub.ReactionRequest{}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("react error=%v", err)
	}
	if err := client.RemoveReaction(context.Background(), socialhub.ReactionRequest{}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("remove reaction error=%v", err)
	}
}

func TestCommonRequestValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, allScopes())
	if _, err := client.GetUser(context.Background(), "other"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("foreign user error=%v", err)
	}
	if _, err := client.GetPost(context.Background(), ""); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("post error=%v", err)
	}
	now := time.Now()
	for _, input := range []socialhub.ListPostsRequest{{UserID: "other"}, {StartTime: &now}, {MaxResults: -1}} {
		if _, err := client.ListPosts(context.Background(), input); err == nil {
			t.Fatalf("list input=%#v should fail", input)
		}
	}
	for _, input := range []socialhub.ListCommentsRequest{{}, {PostID: "post", MaxResults: -1}} {
		if _, err := client.ListComments(context.Background(), input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("comment list input=%#v error=%v", input, err)
		}
	}
	text, empty, private := "text", " ", "private"
	for _, input := range []socialhub.CreatePostRequest{
		{}, {Text: &text, MediaIDs: []string{"media"}}, {Text: &text, Visibility: &private},
		{Text: &text, ReplyToID: &empty}, {Text: &text, QuotePostID: &empty},
	} {
		if _, err := client.Publish(context.Background(), input); err == nil {
			t.Fatalf("publish input=%#v should fail", input)
		}
	}
	if err := client.DeletePost(context.Background(), ""); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("delete error=%v", err)
	}
	if _, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("comment error=%v", err)
	}
	if _, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: "post", ParentID: &empty, Text: "reply"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("comment parent error=%v", err)
	}
}

func TestMappingVariantsAndTimestampValidation(t *testing.T) {
	post, err := mapPost("main", "user-1", graphPost{ID: "gif", GIFURL: "https://cdn.test/a.gif", Text: "gif"})
	if err != nil || len(post.Media) != 1 || post.Media[0].Type != socialhub.MediaTypeAnimation {
		t.Fatalf("GIF post=%#v error=%v", post, err)
	}
	if _, err := mapPost("main", "user-1", graphPost{}); err == nil {
		t.Fatal("missing post ID should fail")
	}
	if _, err := mapCommentPage("main", "post", graphPostPage{Data: []graphPost{{}}}); err == nil {
		t.Fatal("missing reply ID should fail")
	}
	var timestamp graphTime
	if err := json.Unmarshal([]byte(`"not-a-time"`), &timestamp); err == nil {
		t.Fatal("invalid timestamp should fail")
	}
	if err := json.Unmarshal([]byte(`null`), &timestamp); err != nil || timestamp.pointer() != nil {
		t.Fatalf("null timestamp=%v error=%v", timestamp, err)
	}
}

func hasRelation(post socialhub.Post, relationType socialhub.RelationType, target string) bool {
	for _, relation := range post.Relations {
		if relation.Type == relationType && relation.PostID == target {
			return true
		}
	}
	return false
}

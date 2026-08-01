package weibo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-hub/pkg/socialhub"
)

const statusFixture = `{"idstr":"100","mid":"100","text":"hello","created_at":"Sat Aug 01 08:00:00 +0800 2026","user":{"idstr":"42","screen_name":"operator"},"pic_ids":["pic-1"],"pic_urls":[{"thumbnail_pic":"https://img.example/thumb.jpg"}],"original_pic":"https://img.example/original.jpg","reposts_count":3,"comments_count":2,"attitudes_count":5,"retweeted_status":{"idstr":"90","text":"original"}}`

func TestPublishTextPicturesAndRepost(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("access_token") != "access-token" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		if err := request.ParseForm(); err != nil {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		if request.Form.Get("rip") != "203.0.113.10" {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		switch request.URL.Path {
		case "/2/statuses/update.json":
			if request.Form.Get("status") != "text post" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
		case "/2/statuses/upload_url_text.json":
			if request.Form.Get("pic_id") != "pic-1,pic-2" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
		case "/2/statuses/repost.json":
			if request.Form.Get("id") != "90" || request.Form.Get("status") != "commentary" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
		default:
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = writer.Write([]byte(statusFixture))
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)

	text := "text post"
	if _, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text, MediaIDs: []string{"pic-1", "pic-2"}}); err != nil {
		t.Fatal(err)
	}
	commentary, target := "commentary", "90"
	post, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &commentary, QuotePostID: &target})
	if err != nil || !hasRelation(post.Relations, socialhub.RelationRepost, "90") {
		t.Fatalf("repost=%#v err=%v", post, err)
	}
}

func TestFetchPostsAndCommentsPreservesWeiboSemantics(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/2/statuses/show.json":
			_, _ = writer.Write([]byte(statusFixture))
		case "/2/statuses/user_timeline.json":
			if request.URL.Query().Get("uid") != "42" || request.URL.Query().Get("page") != "2" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"statuses":[` + statusFixture + `],"total_number":250}`))
		case "/2/comments/show.json":
			_, _ = writer.Write([]byte(`{"comments":[{"idstr":"501","text":"reply","created_at":"Sat Aug 01 08:01:00 +0800 2026","user":{"idstr":"43"},"reply_comment":{"idstr":"500"}}],"total_number":1}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	post, err := client.GetPost(context.Background(), "100")
	if err != nil || len(post.Media) != 1 || post.Media[0].URL != "https://img.example/original.jpg" || !hasRelation(post.Relations, socialhub.RelationRepost, "90") || len(post.Metrics) != 3 {
		t.Fatalf("post=%#v err=%v", post, err)
	}
	page, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{UserID: "42", Cursor: "2", MaxResults: 100})
	if err != nil || len(page.Items) != 1 || page.NextCursor == nil || *page.NextCursor != "3" || page.PrevCursor == nil || *page.PrevCursor != "1" {
		t.Fatalf("post page=%#v err=%v", page, err)
	}
	comments, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "100"})
	if err != nil || len(comments.Items) != 1 || comments.Items[0].ParentID == nil || *comments.Items[0].ParentID != "500" {
		t.Fatalf("comments=%#v err=%v", comments, err)
	}
}

func TestCommentLikeAndDeleteRequestShapes(t *testing.T) {
	t.Parallel()
	seen := make(map[string]int)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		seen[request.URL.Path]++
		if err := request.ParseForm(); err != nil {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		switch request.URL.Path {
		case "/2/comments/reply.json":
			if request.Form.Get("id") != "100" || request.Form.Get("cid") != "500" || request.Form.Get("rip") == "" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"idstr":"501","text":"reply"}`))
		case "/2/attitudes/create.json", "/2/attitudes/destroy.json":
			if request.Form.Get("id") != "100" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"id":"attitude-1"}`))
		case "/2/comments/destroy.json":
			if request.Form.Get("cid") != "501" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"idstr":"501"}`))
		case "/2/statuses/destroy.json":
			_, _ = writer.Write([]byte(statusFixture))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	parent := "500"
	comment, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: "100", ParentID: &parent, Text: "reply"})
	if err != nil || comment.ParentID == nil || *comment.ParentID != "500" {
		t.Fatalf("comment=%#v err=%v", comment, err)
	}
	reaction := socialhub.ReactionRequest{TargetID: "100", Kind: socialhub.ReactionLike}
	if err := client.React(context.Background(), reaction); err != nil {
		t.Fatal(err)
	}
	if err := client.RemoveReaction(context.Background(), reaction); err != nil {
		t.Fatal(err)
	}
	if err := client.DeleteComment(context.Background(), "501"); err != nil {
		t.Fatal(err)
	}
	if err := client.DeletePost(context.Background(), "100"); err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(seen)
	if len(seen) != 5 {
		t.Fatalf("seen=%s", encoded)
	}
}

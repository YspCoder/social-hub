package douyin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-hub/pkg/socialhub"
)

const videoFixture = `{"item_id":"item-1","video_id":"video-1","title":"demo","create_time":1785542400,"video_status":5,"share_url":"https://www.douyin.com/video/item-1","cover":"https://img.example/cover.jpg","is_reviewed":true,"statistics":{"digg_count":5,"comment_count":2,"play_count":100}}`

func TestUserVideoAndCommentContracts(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("access-token") != "user-token" || request.Header.Get("Authorization") != "" || request.URL.Query().Get("open_id") != "open-id-1" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/oauth/userinfo/":
			_, _ = writer.Write([]byte(`{"data":{"open_id":"open-id-1","union_id":"union-1","nickname":"Creator","avatar":"https://img.example/avatar.jpg","error_code":0}}`))
		case "/video/list/":
			if request.URL.Query().Get("cursor") != "0" || request.URL.Query().Get("count") != "10" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"data":{"list":[` + videoFixture + `],"cursor":100,"has_more":true,"error_code":0}}`))
		case "/video/data/":
			var body struct {
				ItemIDs []string `json:"item_ids"`
			}
			_ = json.NewDecoder(request.Body).Decode(&body)
			if len(body.ItemIDs) != 1 || body.ItemIDs[0] != "item-1" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"data":{"list":[` + videoFixture + `],"error_code":0}}`))
		case "/item/comment/list/":
			_, _ = writer.Write([]byte(`{"data":{"list":[{"comment_id":"comment-1","comment_user_id":"fan-1","content":"nice","create_time":1785542401,"digg_count":3}],"cursor":0,"has_more":false,"error_code":0}}`))
		case "/item/comment/reply/":
			var body map[string]string
			_ = json.NewDecoder(request.Body).Decode(&body)
			if body["item_id"] != "item-1" || body["comment_id"] != "comment-1" || body["content"] != "thanks" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"data":{"comment_id":"reply-1","error_code":0}}`))
		case "/video/create/":
			var body map[string]string
			_ = json.NewDecoder(request.Body).Decode(&body)
			if body["video_id"] != "video-1" || body["text"] != "demo" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"data":{"item_id":"item-1","error_code":0}}`))
		case "/video/delete/":
			_, _ = writer.Write([]byte(`{"data":{"error_code":0}}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server)
	user, err := client.GetUser(context.Background(), "open-id-1")
	if err != nil || user.DisplayName == nil || *user.DisplayName != "Creator" {
		t.Fatalf("user=%#v err=%v", user, err)
	}
	page, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{UserID: "open-id-1", MaxResults: 10})
	if err != nil || len(page.Items) != 1 || page.NextCursor == nil || *page.NextCursor != "100" {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	post, err := client.GetPost(context.Background(), "item-1")
	if err != nil || post.Status == nil || post.Status.State != socialhub.PublishStatePublished || len(post.Metrics) != 6 {
		t.Fatalf("post=%#v err=%v", post, err)
	}
	comments, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "item-1"})
	if err != nil || len(comments.Items) != 1 || comments.Items[0].ID != "comment-1" {
		t.Fatalf("comments=%#v err=%v", comments, err)
	}
	parent := "comment-1"
	reply, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: "item-1", ParentID: &parent, Text: "thanks"})
	if err != nil || reply.ID != "reply-1" {
		t.Fatalf("reply=%#v err=%v", reply, err)
	}
	text := "demo"
	published, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text, MediaIDs: []string{"video-1"}})
	if err != nil || published.Status.State != socialhub.PublishStatePending {
		t.Fatalf("published=%#v err=%v", published, err)
	}
	status, err := client.PublishStatus(context.Background(), "item-1")
	if err != nil || status.State != socialhub.PublishStatePublished {
		t.Fatalf("status=%#v err=%v", status, err)
	}
	if err := client.DeletePost(context.Background(), "item-1"); err != nil {
		t.Fatal(err)
	}
}

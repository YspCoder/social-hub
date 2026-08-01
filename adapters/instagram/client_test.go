package instagram

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestFetchCommentAndContainerContracts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer access-token" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/178":
			writeJSON(writer, `{"id":"graph-178","user_id":"178","username":"brand","name":"Brand","account_type":"BUSINESS","profile_picture_url":"https://cdn.example/avatar.jpg","followers_count":12,"media_count":3}`)
		case request.Method == http.MethodGet && request.URL.Path == "/media-1":
			writeJSON(writer, `{"id":"media-1","caption":"hello","media_type":"IMAGE","media_product_type":"FEED","media_url":"https://cdn.example/image.jpg","permalink":"https://www.instagram.com/p/code/","timestamp":"2026-08-01T00:00:00Z","username":"brand"}`)
		case request.Method == http.MethodGet && request.URL.Path == "/178/media":
			if request.URL.Query().Get("after") != "cursor" || request.URL.Query().Get("limit") != "2" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, `{"data":[{"id":"media-1","caption":"hello","media_type":"IMAGE","media_url":"https://cdn.example/image.jpg","timestamp":"2026-08-01T00:00:00Z"}],"paging":{"cursors":{"after":"next"},"next":"https://graph.instagram.com/next"}}`)
		case request.Method == http.MethodGet && request.URL.Path == "/media-1/comments":
			writeJSON(writer, `{"data":[{"id":"comment-1","text":"nice","timestamp":"2026-08-01T00:00:01Z","username":"alice","from":{"id":"user-1","username":"alice"},"like_count":2}]}`)
		case request.Method == http.MethodPost && request.URL.Path == "/comment-1/replies":
			if err := request.ParseForm(); err != nil || request.Form.Get("message") != "thanks" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, `{"id":"reply-1"}`)
		case request.Method == http.MethodDelete && request.URL.Path == "/reply-1":
			writeJSON(writer, `{"success":true}`)
		case request.Method == http.MethodPost && request.URL.Path == "/178/media":
			if err := request.ParseForm(); err != nil || request.Form.Get("image_url") != "https://cdn.example/new.jpg" || request.Form.Get("caption") != "caption" || request.Form.Get("is_ai_generated") != "true" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, `{"id":"container-1"}`)
		case request.Method == http.MethodGet && request.URL.Path == "/container-1":
			writeJSON(writer, `{"id":"container-1","status_code":"FINISHED","status":"Finished"}`)
		case request.Method == http.MethodPost && request.URL.Path == "/178/media_publish":
			if err := request.ParseForm(); err != nil || request.Form.Get("creation_id") != "container-1" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, `{"id":"media-2"}`)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, []string{
		"instagram_business_basic", "instagram_business_content_publish", "instagram_business_manage_comments",
	}, false)

	user, err := client.GetUser(context.Background(), "178")
	if err != nil || user.ID != "178" || user.Username == nil || *user.Username != "brand" {
		t.Fatalf("user=%#v err=%v", user, err)
	}
	post, err := client.GetPost(context.Background(), "media-1")
	if err != nil || post.ID != "media-1" || len(post.Media) != 1 || post.Media[0].Type != socialhub.MediaTypeImage {
		t.Fatalf("post=%#v err=%v", post, err)
	}
	page, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{Cursor: "cursor", MaxResults: 2})
	if err != nil || len(page.Items) != 1 || page.NextCursor == nil || *page.NextCursor != "next" {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	comments, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "media-1"})
	if err != nil || len(comments.Items) != 1 || len(comments.Items[0].Metrics) != 1 || comments.Items[0].Metrics[0].AsOf != (time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("comments=%#v err=%v", comments, err)
	}
	parent := "comment-1"
	reply, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: "media-1", ParentID: &parent, Text: "thanks"})
	if err != nil || reply.ID != "reply-1" || reply.ParentID == nil || *reply.ParentID != parent {
		t.Fatalf("reply=%#v err=%v", reply, err)
	}
	if err := client.DeleteComment(context.Background(), reply.ID); err != nil {
		t.Fatal(err)
	}

	container, err := client.ContainerWorkflow().Create(context.Background(), ContainerRequest{Type: ContainerImage, MediaURL: "https://cdn.example/new.jpg", Caption: "caption", IsAIGenerated: true})
	if err != nil || container.ID != "container-1" {
		t.Fatalf("container=%#v err=%v", container, err)
	}
	status, err := client.ContainerWorkflow().Status(context.Background(), container.ID)
	if err != nil || status.StatusCode != "FINISHED" {
		t.Fatalf("status=%#v err=%v", status, err)
	}
	published, err := client.ContainerWorkflow().Publish(context.Background(), container.ID)
	if err != nil || published.ID != "media-2" || published.Status == nil || published.Status.State != socialhub.PublishStatePublished {
		t.Fatalf("published=%#v err=%v", published, err)
	}
}

func TestContainerAndReactionValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, nil, false)
	invalid := []ContainerRequest{
		{Type: ContainerImage, MediaURL: "http://cdn.example/image.jpg"},
		{Type: ContainerStory, MediaURL: "https://cdn.example/story", MediaType: socialhub.MediaTypeDocument},
		{Type: ContainerCarousel, Children: []string{"one"}},
		{Type: ContainerImage, MediaURL: "https://cdn.example/image.jpg", CarouselItem: true, Caption: "invalid"},
	}
	for _, input := range invalid {
		if _, err := client.ContainerWorkflow().Create(context.Background(), input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("input=%#v error=%v", input, err)
		}
	}
	if err := client.React(context.Background(), socialhub.ReactionRequest{TargetID: "media-1", Kind: socialhub.ReactionLike}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("reaction error=%v", err)
	}
	if _, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: "media-1", Text: "top-level"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("comment error=%v", err)
	}
}

func writeJSON(writer http.ResponseWriter, value string) {
	writer.Header().Set("Content-Type", "application/json")
	_, _ = writer.Write([]byte(value))
}

package threads

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestContainerPublicationLifecycle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("access_token") != "access-token" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/me/threads":
			if request.ParseForm() != nil {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			switch request.PostForm.Get("media_type") {
			case "IMAGE":
				if request.PostForm.Get("image_url") != "https://cdn.test/image.jpg" || request.PostForm.Get("alt_text") != "image alt" || request.PostForm.Get("text") != "image post" || request.PostForm.Get("reply_control") != "followers_only" || request.PostForm.Get("topic_tag") != "Go" || request.PostForm.Get("location_id") != "location-1" || request.PostForm.Get("is_spoiler_media") != "true" || request.PostForm.Get("enable_reply_approvals") != "true" {
					writer.WriteHeader(http.StatusBadRequest)
					return
				}
				writeTestJSON(t, writer, map[string]string{"id": "container-image"})
			case "VIDEO":
				if request.PostForm.Get("video_url") != "https://cdn.test/video.mp4" || request.PostForm.Get("is_carousel_item") != "true" {
					writer.WriteHeader(http.StatusBadRequest)
					return
				}
				writeTestJSON(t, writer, map[string]string{"id": "container-video-child"})
			case "CAROUSEL":
				if request.PostForm.Get("children") != "child-1,child-2" || request.PostForm.Get("text") != "carousel" {
					writer.WriteHeader(http.StatusBadRequest)
					return
				}
				writeTestJSON(t, writer, map[string]string{"id": "container-carousel"})
			default:
				writer.WriteHeader(http.StatusBadRequest)
			}
		case request.Method == http.MethodGet && request.URL.Path == "/container-image":
			if request.URL.Query().Get("fields") != "id,status,error_message" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeTestJSON(t, writer, map[string]string{"id": "container-image", "status": "FINISHED"})
		case request.Method == http.MethodPost && request.URL.Path == "/me/threads_publish":
			if request.ParseForm() != nil || request.PostForm.Get("creation_id") != "container-image" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeTestJSON(t, writer, map[string]string{"id": "published-image"})
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, allScopes())

	created, err := client.CreateContainer(context.Background(), ContainerRequest{
		Type: ContainerImage, Text: "image post", ImageURL: "https://cdn.test/image.jpg", AltText: "image alt",
		ReplyControl: ReplyFollowersOnly, TopicTag: "Go", LocationID: "location-1", SpoilerMedia: true, EnableReplyApprovals: true,
	})
	if err != nil || created.ID != "container-image" || created.Status != "IN_PROGRESS" {
		t.Fatalf("created=%#v error=%v", created, err)
	}
	video, err := client.CreateContainer(context.Background(), ContainerRequest{
		Type: ContainerVideo, VideoURL: "https://cdn.test/video.mp4", AltText: "clip", CarouselItem: true, SpoilerMedia: true,
	})
	if err != nil || video.ID != "container-video-child" {
		t.Fatalf("video child=%#v error=%v", video, err)
	}
	carousel, err := client.CreateContainer(context.Background(), ContainerRequest{Type: ContainerCarousel, Text: "carousel", Children: []string{"child-1", "child-2"}})
	if err != nil || carousel.ID != "container-carousel" {
		t.Fatalf("carousel=%#v error=%v", carousel, err)
	}
	status, err := client.ContainerStatus(context.Background(), created.ID)
	if err != nil || status.Status != "FINISHED" {
		t.Fatalf("status=%#v error=%v", status, err)
	}
	published, err := client.PublishContainer(context.Background(), created.ID)
	if err != nil || published.ID != "published-image" || published.Status == nil || published.Status.State != socialhub.PublishStatePublished || len(published.Extensions) != 1 {
		t.Fatalf("published=%#v error=%v", published, err)
	}
}

func TestContainerFormTextFeatures(t *testing.T) {
	form, err := containerForm(ContainerRequest{
		Type: ContainerText, Text: "question", LinkAttachmentURL: "https://example.test/article",
		ReplyToID: "parent", QuotePostID: "quote", ReplyControl: ReplyEveryone, TopicTag: "API", GhostPost: true, EnableReplyApprovals: true,
	})
	if err != nil || form.Get("link_attachment") == "" || form.Get("is_ghost_post") != "true" || form.Get("reply_to_id") != "parent" || form.Get("quote_post_id") != "quote" {
		t.Fatalf("text form=%v error=%v", form, err)
	}
	poll, err := containerForm(ContainerRequest{Type: ContainerText, Text: "vote", Poll: &PollAttachment{OptionA: "A", OptionB: "B", OptionC: "C"}})
	if err != nil || !strings.Contains(poll.Get("poll_attachment"), `"option_c":"C"`) {
		t.Fatalf("poll form=%v error=%v", poll, err)
	}
}

func TestContainerValidation(t *testing.T) {
	invalid := []ContainerRequest{
		{},
		{Type: ContainerText},
		{Type: ContainerText, Text: "text", AltText: "invalid"},
		{Type: ContainerImage, ImageURL: "http://cdn.test/image.jpg"},
		{Type: ContainerVideo, VideoURL: "https://user:pass@cdn.test/video.mp4"},
		{Type: ContainerCarousel, Children: []string{"one"}},
		{Type: ContainerCarousel, Children: []string{"one", ""}},
		{Type: ContainerImage, ImageURL: "https://cdn.test/image.jpg", CarouselItem: true, Text: "not allowed"},
		{Type: ContainerText, Text: "x", ReplyControl: "invalid"},
		{Type: ContainerText, Text: "x", LinkAttachmentURL: "http://example.test"},
		{Type: ContainerText, Text: "x", LinkAttachmentURL: "https://example.test", Poll: &PollAttachment{OptionA: "A", OptionB: "B"}},
		{Type: ContainerText, Text: "x", Poll: &PollAttachment{OptionA: "A"}},
		{Type: ContainerText, Text: "x", Poll: &PollAttachment{OptionA: "A", OptionB: "B", OptionD: "D"}},
	}
	for _, input := range invalid {
		if _, err := containerForm(input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("input=%#v error=%v", input, err)
		}
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, allScopes())
	if _, err := client.ContainerStatus(context.Background(), ""); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("status error=%v", err)
	}
	if _, err := client.PublishContainer(context.Background(), ""); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("publish error=%v", err)
	}
	if !validHTTPSURL("https://example.test/a") || validHTTPSURL("ftp://example.test/a") || !validReplyControl(ReplyMentionedOnly) {
		t.Fatal("URL or reply-control validation mismatch")
	}
}

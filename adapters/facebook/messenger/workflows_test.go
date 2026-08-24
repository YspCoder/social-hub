package messenger

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

type sendRequestWire struct {
	Recipient struct {
		ID string `json:"id"`
	} `json:"recipient"`
	MessagingType MessagingType `json:"messaging_type"`
	Message       struct {
		Text    string `json:"text"`
		ReplyTo *struct {
			ID string `json:"mid"`
		} `json:"reply_to"`
		Attachment *struct {
			Type    AttachmentType `json:"type"`
			Payload struct {
				ID       string `json:"attachment_id"`
				URL      string `json:"url"`
				Reusable bool   `json:"is_reusable"`
			} `json:"payload"`
		} `json:"attachment"`
	} `json:"message"`
}

func TestMessageAndProfileContracts(t *testing.T) {
	var sends []sendRequestWire
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer page-token" {
			t.Errorf("authorization=%q", request.Header.Get("Authorization"))
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/v26.0/123456789/messages":
			if request.Header.Get("Content-Type") != "application/json" {
				t.Errorf("content type=%q", request.Header.Get("Content-Type"))
			}
			var body sendRequestWire
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Errorf("decode send body: %v", err)
			}
			sends = append(sends, body)
			writeTestJSON(t, writer, SendResult{RecipientID: body.Recipient.ID, MessageID: "mid." + body.Recipient.ID + "." + string(rune('0'+len(sends)))})
		case request.Method == http.MethodGet && request.URL.Path == "/v26.0/111222333":
			if request.URL.Query().Get("fields") != "id,name,first_name,last_name,profile_pic" {
				t.Errorf("fields=%q", request.URL.Query().Get("fields"))
			}
			writeTestJSON(t, writer, UserProfile{
				ID: "111222333", Name: "Ada Lovelace", FirstName: "Ada", LastName: "Lovelace",
				ProfilePic: "https://cdn.example/ada.jpg",
			})
		case request.Method == http.MethodGet && request.URL.Path == "/v26.0/999":
			writeTestJSON(t, writer, map[string]any{})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, false)

	text := "hello"
	replyTo := "mid.parent"
	message, err := client.SendMessage(context.Background(), socialhub.SendMessageRequest{
		ConversationID: "111222333", RecipientIDs: []string{"111222333"}, Text: &text, ReplyToID: &replyTo,
	})
	if err != nil || message.ID == "" || message.Direction != socialhub.DirectionOutbound || message.SentAt == nil ||
		!message.SentAt.Equal(testNow) || message.ReplyToID == nil || *message.ReplyToID != replyTo || len(message.Extensions) != 1 {
		t.Fatalf("common message=%#v err=%v", message, err)
	}
	typed, err := client.SendText(context.Background(), TextMessageRequest{
		RecipientID: "111222333", Text: "update", Type: MessagingUpdate,
	})
	if err != nil || typed.Text == nil || *typed.Text != "update" {
		t.Fatalf("typed message=%#v err=%v", typed, err)
	}
	media, err := client.SendAttachment(context.Background(), AttachmentMessageRequest{
		RecipientID: "111222333", Attachment: AttachmentImage,
		Reference: AttachmentReference{URL: "https://cdn.example/photo.jpg", Reusable: true},
	})
	if err != nil || len(media.Media) != 1 || media.Media[0].Type != socialhub.MediaTypeImage || media.Media[0].URL == "" {
		t.Fatalf("URL attachment=%#v err=%v", media, err)
	}
	reusable, err := client.SendAttachment(context.Background(), AttachmentMessageRequest{
		RecipientID: "111222333", Type: MessagingUpdate, Attachment: AttachmentVideo,
		Reference: AttachmentReference{ID: "attachment-id"}, ReplyToID: "mid.parent",
	})
	if err != nil || len(reusable.Media) != 1 || reusable.Media[0].ID != "attachment-id" || reusable.Media[0].Type != socialhub.MediaTypeVideo {
		t.Fatalf("reusable attachment=%#v err=%v", reusable, err)
	}
	if len(sends) != 4 || sends[0].MessagingType != MessagingResponse || sends[0].Message.Text != "hello" ||
		sends[0].Message.ReplyTo == nil || sends[0].Message.ReplyTo.ID != replyTo || sends[1].MessagingType != MessagingUpdate ||
		sends[2].Message.Attachment == nil || sends[2].Message.Attachment.Payload.URL != "https://cdn.example/photo.jpg" ||
		!sends[2].Message.Attachment.Payload.Reusable || sends[3].Message.Attachment.Payload.ID != "attachment-id" ||
		sends[3].Message.ReplyTo == nil {
		t.Fatalf("send bodies=%#v", sends)
	}

	profile, err := client.GetUserProfile(context.Background(), "111222333")
	if err != nil || profile.Name != "Ada Lovelace" || profile.FirstName != "Ada" {
		t.Fatalf("profile=%#v err=%v", profile, err)
	}
	user, err := client.GetUser(context.Background(), "111222333")
	if err != nil || user.DisplayName == nil || *user.DisplayName != "Ada Lovelace" || user.AvatarURL == nil || len(user.Extensions) != 1 {
		t.Fatalf("user=%#v err=%v", user, err)
	}
	if _, err := client.GetUserProfile(context.Background(), "999"); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("empty profile error=%v", err)
	}
}

func TestWorkflowValidationAndUnsupportedOperations(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, false)

	invalidMessages := []socialhub.SendMessageRequest{
		{},
		{ConversationID: "bad id", Text: stringPointer("hello")},
		{ConversationID: "111222333", Text: stringPointer("hello"), RecipientIDs: []string{"222"}},
	}
	for index, input := range invalidMessages {
		if _, err := client.SendMessage(context.Background(), input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("message %d error=%v", index, err)
		}
	}
	if _, err := client.SendMessage(context.Background(), socialhub.SendMessageRequest{
		ConversationID: "111222333", Text: stringPointer("hello"), MediaIDs: []string{"media"},
	}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("common media error=%v", err)
	}
	if _, err := client.SendText(context.Background(), TextMessageRequest{RecipientID: "111222333", Text: strings.Repeat("x", 2001)}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("long text error=%v", err)
	}
	if _, err := client.SendText(context.Background(), TextMessageRequest{RecipientID: "111222333", Text: "x", Type: "MARKETING"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("messaging type error=%v", err)
	}
	invalidAttachments := []AttachmentMessageRequest{
		{RecipientID: "111222333", Attachment: AttachmentImage},
		{RecipientID: "111222333", Attachment: AttachmentImage, Reference: AttachmentReference{ID: "id", URL: "https://cdn.example/a.jpg"}},
		{RecipientID: "111222333", Attachment: AttachmentImage, Reference: AttachmentReference{URL: "http://cdn.example/a.jpg"}},
		{RecipientID: "111222333", Attachment: AttachmentImage, Reference: AttachmentReference{ID: "id", Reusable: true}},
		{RecipientID: "111222333", Attachment: "sticker", Reference: AttachmentReference{ID: "id"}},
	}
	for index, input := range invalidAttachments {
		if _, err := client.SendAttachment(context.Background(), input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("attachment %d error=%v", index, err)
		}
	}
	if _, err := client.GetUserProfile(context.Background(), "bad"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("bad profile=%v", err)
	}
	if _, err := client.GetMessage(context.Background(), "id"); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("get message=%v", err)
	}
	if _, err := client.GetPost(context.Background(), "id"); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("get post=%v", err)
	}
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("list posts=%v", err)
	}
	if _, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("list comments=%v", err)
	}
}

func TestSendRejectsInvalidPlatformResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writeTestJSON(t, writer, SendResult{RecipientID: "different", MessageID: "mid.valid"})
	}))
	defer server.Close()
	_, client := newTestClient(t, server, false)
	if _, err := client.SendText(context.Background(), TextMessageRequest{RecipientID: "111222333", Text: "hello"}); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("invalid response error=%v", err)
	}
}

package instagram

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

type messagingRequestWire struct {
	Recipient struct {
		ID string `json:"id"`
	} `json:"recipient"`
	SenderAction string `json:"sender_action"`
	Payload      struct {
		MessageID string `json:"message_id"`
		Reaction  string `json:"reaction"`
	} `json:"payload"`
	Message struct {
		Text       string `json:"text"`
		Attachment *struct {
			Type    string `json:"type"`
			Payload struct {
				URL string `json:"url"`
				ID  string `json:"id"`
			} `json:"payload"`
		} `json:"attachment"`
	} `json:"message"`
}

func TestMessagingAndProfileContracts(t *testing.T) {
	var sends []messagingRequestWire
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer access-token" {
			t.Errorf("authorization=%q", request.Header.Get("Authorization"))
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/178/messages":
			if request.Header.Get("Content-Type") != "application/json" {
				t.Errorf("content type=%q", request.Header.Get("Content-Type"))
			}
			var body messagingRequestWire
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Errorf("decode send body: %v", err)
			}
			sends = append(sends, body)
			if body.SenderAction != "" {
				writeJSON(writer, `{"recipient_id":"`+body.Recipient.ID+`"}`)
				return
			}
			writeJSON(writer, `{"recipient_id":"`+body.Recipient.ID+`","message_id":"mid.`+string(rune('0'+len(sends)))+`"}`)
		case request.Method == http.MethodGet && request.URL.Path == "/mid.inbound":
			if request.URL.Query().Get("fields") != "id,created_time,from,to,message,reply_to" {
				t.Errorf("message fields=%q", request.URL.Query().Get("fields"))
			}
			writeJSON(writer, `{"id":"mid.inbound","created_time":"2026-08-03T09:10:11+0000","from":{"id":"111","username":"alice"},"to":{"data":[{"id":"178","username":"brand"}]},"message":"hello","reply_to":{"id":"mid.parent"}}`)
		case request.Method == http.MethodGet && request.URL.Path == "/mid.outbound":
			writeJSON(writer, `{"id":"mid.outbound","created_time":"2026-08-03T09:10:12Z","from":{"id":"178","username":"brand"},"to":{"data":[{"id":"111","username":"alice"}]},"message":"sent"}`)
		case request.Method == http.MethodGet && request.URL.Path == "/111":
			if request.URL.Query().Get("fields") != "id,name,username,profile_pic,follower_count,is_verified_user,is_user_follow_business,is_business_follow_user" {
				t.Errorf("profile fields=%q", request.URL.Query().Get("fields"))
			}
			writeJSON(writer, `{"id":"111","name":"Alice","username":"alice","profile_pic":"https://cdn.example/profile.jpg","follower_count":12,"is_verified_user":true,"is_user_follow_business":true,"is_business_follow_user":false}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, []string{"instagram_business_basic", messagingScope}, false)

	text := "hello"
	common, err := client.SendMessage(context.Background(), socialhub.SendMessageRequest{
		ConversationID: "111", RecipientIDs: []string{"111"}, Text: &text,
	})
	if err != nil || common.ID != "mid.1" || common.Direction != socialhub.DirectionOutbound || common.SenderID == nil ||
		*common.SenderID != "178" || common.SentAt == nil || len(common.Extensions) != 1 {
		t.Fatalf("common message=%#v err=%v", common, err)
	}
	typed, err := client.SendText(context.Background(), TextMessageRequest{RecipientID: "111", Text: "typed"})
	if err != nil || typed.Text == nil || *typed.Text != "typed" || typed.ID != "mid.2" {
		t.Fatalf("typed message=%#v err=%v", typed, err)
	}
	media, err := client.SendMedia(context.Background(), MediaMessageRequest{
		RecipientID: "111", Type: MessageMediaImage, URL: "https://cdn.example/photo.jpg",
	})
	if err != nil || len(media.Media) != 1 || media.Media[0].Type != socialhub.MediaTypeImage || media.Media[0].URL == "" {
		t.Fatalf("media message=%#v err=%v", media, err)
	}
	shared, err := client.SharePublishedMedia(context.Background(), PublishedMediaMessageRequest{RecipientID: "111", MediaID: "9001"})
	if err != nil || len(shared.Media) != 1 || shared.Media[0].ID != "9001" {
		t.Fatalf("shared message=%#v err=%v", shared, err)
	}
	reaction, err := client.SendReaction(context.Background(), MessageReactionRequest{
		RecipientID: "111", MessageID: "mid.inbound", Action: MessageReactionAdd, Reaction: "love",
	})
	if err != nil || reaction.RecipientID != "111" {
		t.Fatalf("reaction=%#v err=%v", reaction, err)
	}
	if _, err := client.SendReaction(context.Background(), MessageReactionRequest{
		RecipientID: "111", MessageID: "mid.inbound", Action: MessageReactionRemove, Reaction: "love",
	}); err != nil {
		t.Fatal(err)
	}
	if len(sends) != 6 || sends[0].Message.Text != "hello" || sends[1].Message.Text != "typed" ||
		sends[2].Message.Attachment == nil || sends[2].Message.Attachment.Type != "image" ||
		sends[2].Message.Attachment.Payload.URL != "https://cdn.example/photo.jpg" ||
		sends[3].Message.Attachment == nil || sends[3].Message.Attachment.Type != "MEDIA_SHARE" ||
		sends[3].Message.Attachment.Payload.ID != "9001" || sends[4].SenderAction != "react" ||
		sends[4].Payload.MessageID != "mid.inbound" || sends[4].Payload.Reaction != "love" || sends[5].SenderAction != "unreact" {
		t.Fatalf("send bodies=%#v", sends)
	}

	inbound, err := client.GetMessage(context.Background(), "mid.inbound")
	wantTime := time.Date(2026, time.August, 3, 9, 10, 11, 0, time.UTC)
	if err != nil || inbound.Direction != socialhub.DirectionInbound || inbound.ConversationID != "111" ||
		inbound.SenderID == nil || *inbound.SenderID != "111" || inbound.ReplyToID == nil || *inbound.ReplyToID != "mid.parent" ||
		inbound.SentAt == nil || !inbound.SentAt.Equal(wantTime) || len(inbound.Extensions) != 1 {
		t.Fatalf("inbound=%#v err=%v", inbound, err)
	}
	outbound, err := client.GetMessage(context.Background(), "mid.outbound")
	if err != nil || outbound.Direction != socialhub.DirectionOutbound || outbound.ConversationID != "111" || outbound.SenderID == nil || *outbound.SenderID != "178" {
		t.Fatalf("outbound=%#v err=%v", outbound, err)
	}
	profile, err := client.GetMessagingUserProfile(context.Background(), "111")
	if err != nil || profile.Name != "Alice" || !profile.Verified || !profile.UserFollowsBusiness || profile.FollowerCount != 12 {
		t.Fatalf("profile=%#v err=%v", profile, err)
	}
}

func TestMessagingValidationScopesAndPlatformResponses(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, []string{"instagram_business_basic", messagingScope}, false)
	text := "hello"
	invalidCommon := []socialhub.SendMessageRequest{
		{},
		{ConversationID: "bad", Text: &text},
		{ConversationID: "111", Text: &text, RecipientIDs: []string{"222"}},
	}
	for index, input := range invalidCommon {
		if _, err := client.SendMessage(context.Background(), input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("common %d error=%v", index, err)
		}
	}
	if _, err := client.SendMessage(context.Background(), socialhub.SendMessageRequest{ConversationID: "111", Text: &text, MediaIDs: []string{"1"}}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("common media error=%v", err)
	}
	reply := "mid.parent"
	if _, err := client.SendMessage(context.Background(), socialhub.SendMessageRequest{ConversationID: "111", Text: &text, ReplyToID: &reply}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("common reply error=%v", err)
	}
	for index, input := range []MediaMessageRequest{
		{RecipientID: "111", Type: "file", URL: "https://cdn.example/file"},
		{RecipientID: "111", Type: MessageMediaImage, URL: "http://cdn.example/file"},
		{RecipientID: "111", Type: MessageMediaImage, URL: "https://user@cdn.example/file"},
	} {
		if _, err := client.SendMedia(context.Background(), input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("media %d error=%v", index, err)
		}
	}
	if _, err := client.SendText(context.Background(), TextMessageRequest{RecipientID: "111", Text: "  "}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("blank text error=%v", err)
	}
	if _, err := client.SharePublishedMedia(context.Background(), PublishedMediaMessageRequest{RecipientID: "111", MediaID: "bad"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("share error=%v", err)
	}
	if _, err := client.SendReaction(context.Background(), MessageReactionRequest{RecipientID: "111", MessageID: "mid", Action: "toggle", Reaction: "love"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("reaction error=%v", err)
	}
	if _, err := client.GetMessage(context.Background(), strings.Repeat("m", 513)); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("message ID error=%v", err)
	}
	if _, err := client.GetMessagingUserProfile(context.Background(), "178"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("own profile error=%v", err)
	}

	_, limited := newTestAdapter(t, server, []string{"instagram_business_basic"}, false)
	if _, err := limited.SendText(context.Background(), TextMessageRequest{RecipientID: "111", Text: "hello"}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("scope error=%v", err)
	}
}

func TestMessagingRejectsInvalidResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.Method {
		case http.MethodPost:
			writeJSON(writer, `{"recipient_id":"999","message_id":"mid"}`)
		case http.MethodGet:
			writeJSON(writer, `{"id":"mid","created_time":"invalid","from":{"id":"111"},"to":{"data":[{"id":"178"}]},"message":"hello"}`)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, []string{"instagram_business_basic", messagingScope}, false)
	if _, err := client.SendText(context.Background(), TextMessageRequest{RecipientID: "111", Text: "hello"}); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("send response error=%v", err)
	}
	if _, err := client.GetMessage(context.Background(), "mid"); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("get response error=%v", err)
	}
}

func errorCode(err error) socialhub.ErrorCode {
	var platformError *socialhub.Error
	if errors.As(err, &platformError) {
		return platformError.Code
	}
	return ""
}

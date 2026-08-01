package whatsapp

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

func TestSendTextReplyAndCommonBoundaries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requireBearer(t, request)
		if request.Method != http.MethodPost || request.URL.Path != "/123456789/messages" {
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		text, _ := body["text"].(map[string]any)
		contextValue, _ := body["context"].(map[string]any)
		if body["messaging_product"] != "whatsapp" || body["recipient_type"] != "individual" || body["to"] != "15550001111" || body["type"] != "text" || text["body"] != "hello" || text["preview_url"] != false || contextValue["message_id"] != "wamid.parent" {
			t.Errorf("body=%#v", body)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		writeTestJSON(t, writer, map[string]any{
			"messaging_product": "whatsapp", "contacts": []map[string]string{{"input": "15550001111", "wa_id": "15550001111"}},
			"messages": []map[string]string{{"id": "wamid.outbound", "message_status": "accepted"}},
		})
	}))
	defer server.Close()
	client := newTestClient(t, server, allScopes(), true)
	text := "hello"
	reply := " wamid.parent "
	message, err := client.SendMessage(context.Background(), socialhub.SendMessageRequest{ConversationID: " 15550001111 ", Text: &text, ReplyToID: &reply})
	if err != nil {
		t.Fatal(err)
	}
	if message.ID != "wamid.outbound" || message.ConversationID != "15550001111" || message.Direction != socialhub.DirectionOutbound || message.SentAt == nil || !message.SentAt.Equal(testNow) || message.ReplyToID == nil || *message.ReplyToID != "wamid.parent" || len(message.Extensions["whatsapp.message"]) == 0 {
		t.Fatalf("message=%#v", message)
	}
	if _, err := client.GetMessage(context.Background(), "wamid.outbound"); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("get message error=%v", err)
	}
	if _, err := client.SendMessage(context.Background(), socialhub.SendMessageRequest{ConversationID: "15550001111", Text: &text, RecipientIDs: []string{"other"}}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("multiple recipients error=%v", err)
	}
	if _, err := client.SendMessage(context.Background(), socialhub.SendMessageRequest{ConversationID: "15550001111", Text: &text, MediaIDs: []string{"media"}}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("media IDs error=%v", err)
	}
	blank := " "
	if _, err := client.SendMessage(context.Background(), socialhub.SendMessageRequest{ConversationID: "15550001111", Text: &blank}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("blank text error=%v", err)
	}
	long := strings.Repeat("x", 4097)
	if _, err := client.SendMessage(context.Background(), socialhub.SendMessageRequest{ConversationID: "15550001111", Text: &long}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("long text error=%v", err)
	}
}

func TestTypedMessageWorkflows(t *testing.T) {
	var templateCalls int
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requireBearer(t, request)
		if request.URL.Path != "/123456789/messages" {
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		if request.Method == http.MethodPut {
			if body["status"] != "read" || body["message_id"] != "wamid.inbound" {
				t.Errorf("read body=%#v", body)
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeTestJSON(t, writer, map[string]bool{"success": true})
			return
		}
		if request.Method != http.MethodPost {
			writer.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		switch body["type"] {
		case "image":
			image, _ := body["image"].(map[string]any)
			contextValue, _ := body["context"].(map[string]any)
			if image["id"] != "media-1" || image["caption"] != "caption" || contextValue["message_id"] != "wamid.parent" || body["to"] != "15550001111" {
				t.Errorf("image body=%#v", body)
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
		case "template":
			templateCalls++
			template, _ := body["template"].(map[string]any)
			language, _ := template["language"].(map[string]any)
			if language["code"] != "en_US" {
				t.Errorf("template body=%#v", body)
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, hasComponents := template["components"]
			if templateCalls == 1 && !hasComponents || templateCalls == 2 && hasComponents {
				t.Errorf("template components body=%#v", body)
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
		case "reaction":
			reaction, _ := body["reaction"].(map[string]any)
			if reaction["message_id"] != "wamid.target" || reaction["emoji"] != "" {
				t.Errorf("reaction body=%#v", body)
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
		default:
			t.Errorf("unknown body=%#v", body)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		writeTestJSON(t, writer, map[string]any{
			"messaging_product": "whatsapp", "messages": []map[string]string{{"id": "wamid.sent"}},
		})
	}))
	defer server.Close()
	client := newTestClient(t, server, allScopes(), true)

	media, err := client.SendMedia(context.Background(), MediaMessageRequest{
		To: " 15550001111 ", Type: MediaImage, Media: MediaReference{ID: " media-1 "}, Caption: "caption", ReplyToID: " wamid.parent ",
	})
	if err != nil || media.ID != "wamid.sent" || media.ReplyToID == nil || *media.ReplyToID != "wamid.parent" {
		t.Fatalf("media=%#v error=%v", media, err)
	}
	parameter := json.RawMessage(`  {"type":"text","text":"Ada"} `)
	if _, err := client.SendTemplate(context.Background(), TemplateMessageRequest{
		To: "15550001111", Name: "order_update", LanguageCode: " en_US ",
		Components: []TemplateComponent{{Type: "body", Parameters: []json.RawMessage{parameter}}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.SendTemplate(context.Background(), TemplateMessageRequest{To: "15550001111", Name: "hello_world", LanguageCode: "en_US"}); err != nil {
		t.Fatal(err)
	}
	reaction, err := client.SendReaction(context.Background(), "15550001111", " wamid.target ", "")
	if err != nil || reaction.ReplyToID == nil || *reaction.ReplyToID != "wamid.target" {
		t.Fatalf("reaction=%#v error=%v", reaction, err)
	}
	if err := client.MarkRead(context.Background(), " wamid.inbound "); err != nil {
		t.Fatal(err)
	}
}

func TestTypedMessageValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	client := newTestClient(t, server, allScopes(), true)
	mediaCases := []MediaMessageRequest{
		{},
		{To: "1555", Type: MediaImage},
		{To: "1555", Type: MediaImage, Media: MediaReference{ID: "one", Link: "https://example.test/image.jpg"}},
		{To: "1555", Type: MediaImage, Media: MediaReference{Link: "http://example.test/image.jpg"}},
		{To: "1555", Type: MediaAudio, Media: MediaReference{ID: "one"}, Caption: "caption"},
		{To: "1555", Type: MediaImage, Media: MediaReference{ID: "one"}, Filename: "image.jpg"},
	}
	for _, input := range mediaCases {
		if _, err := client.SendMedia(context.Background(), input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("media input=%#v error=%v", input, err)
		}
	}
	templateCases := []TemplateMessageRequest{
		{},
		{To: "1555", Name: "bad name", LanguageCode: "en_US"},
		{To: "1555", Name: "valid", LanguageCode: "en_US", Components: []TemplateComponent{{Type: "body", Parameters: []json.RawMessage{json.RawMessage(`[1]`)}}}},
		{To: "1555", Name: "valid", LanguageCode: "en_US", Components: []TemplateComponent{{Type: "bad type"}}},
	}
	for _, input := range templateCases {
		if _, err := client.SendTemplate(context.Background(), input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("template input=%#v error=%v", input, err)
		}
	}
	if _, err := client.SendReaction(context.Background(), "1555", "wamid", "         "); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("reaction whitespace error=%v", err)
	}
	if err := client.MarkRead(context.Background(), " "); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("mark read error=%v", err)
	}
	if !validRecipient("1555") || validRecipient("bad recipient") || !validMediaKind(MediaDocument) || validMediaKind("gif") || !validHTTPSURL("https://example.test/a") || validHTTPSURL("https://user:secret@example.test") {
		t.Fatal("message validation helpers mismatch")
	}
}

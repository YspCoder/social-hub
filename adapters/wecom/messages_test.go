package wecom

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestApplicationMessageWireContractsAndCommonMessage(t *testing.T) {
	var payloads []applicationMessagePayload
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/cgi-bin/message/send" || request.URL.Query().Get("access_token") != "access-token" || request.Header.Get("Content-Type") != "application/json" {
			http.Error(writer, "bad request", http.StatusBadRequest)
			return
		}
		var payload applicationMessagePayload
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			http.Error(writer, "bad body", http.StatusBadRequest)
			return
		}
		payloads = append(payloads, payload)
		if payload.Text != nil && payload.Text.Content == "rejected" {
			writeTestJSON(t, writer, map[string]any{"errcode": 81013, "errmsg": "invalid user"})
			return
		}
		response := map[string]any{"errcode": 0, "errmsg": "ok", "msgid": "message-" + string(rune('0'+len(payloads)))}
		if len(payloads) == 1 {
			response["invaliduser"] = "bob|carol"
			response["invalidparty"] = "9,10"
			response["invalidtag"] = "11"
			response["unlicenseduser"] = "dave"
		}
		writeTestJSON(t, writer, response)
	}))
	defer server.Close()
	_, client := newTestClient(t, server, false)

	first, err := client.SendApplicationMessage(context.Background(), ApplicationMessageRequest{
		Recipients: RecipientSet{UserIDs: []string{"alice", "alice", "bob"}, PartyIDs: []int64{2}, TagIDs: []int64{3}},
		Content:    TextContent{Content: "hello"}, Safe: true, EnableIDTranslation: true,
	})
	if err != nil || first.MessageID != "message-1" || len(first.InvalidUserIDs) != 2 || len(first.InvalidPartyIDs) != 2 || len(first.InvalidTagIDs) != 1 || len(first.UnlicensedUserIDs) != 1 {
		t.Fatalf("first result=%#v err=%v", first, err)
	}
	calls := []ApplicationMessageRequest{
		{Content: MarkdownContent{Content: "**notice**"}},
		{Recipients: RecipientSet{ToAll: true}, Content: ImageContent{MediaID: "image-id"}},
		{Recipients: RecipientSet{UserIDs: []string{"alice"}}, Content: VoiceContent{MediaID: "voice-id"}},
		{Recipients: RecipientSet{PartyIDs: []int64{2}}, Content: VideoContent{MediaID: "video-id", Title: "Title", Description: "Description"}, EnableDuplicateCheck: true, DuplicateCheckInterval: 30 * time.Minute},
		{Recipients: RecipientSet{TagIDs: []int64{3}}, Content: FileContent{MediaID: "file-id"}},
	}
	for index, input := range calls {
		result, err := client.SendApplicationMessage(context.Background(), input)
		if err != nil || result.MessageID == "" {
			t.Fatalf("typed message %d result=%#v err=%v", index, result, err)
		}
	}
	text := "common hello"
	message, err := client.SendMessage(context.Background(), socialhub.SendMessageRequest{ConversationID: "carol", Text: &text})
	if err != nil || message.ID == "" || message.ConversationID != "carol" || message.Direction != socialhub.DirectionOutbound || message.Extensions["wecom.delivery"] == nil {
		t.Fatalf("common message=%#v err=%v", message, err)
	}

	if len(payloads) != 7 {
		t.Fatalf("payload count=%d", len(payloads))
	}
	if got := payloads[0]; got.ToUser != "alice|bob" || got.ToParty != "2" || got.ToTag != "3" || got.MessageType != "text" || got.Text == nil || got.Text.Content != "hello" || got.AgentID != 1000002 || got.Safe != 1 || got.EnableIDTranslation != 1 {
		t.Fatalf("text payload=%#v", got)
	}
	if got := payloads[1]; got.ToUser != "alice" || got.MessageType != "markdown" || got.Markdown == nil {
		t.Fatalf("markdown payload=%#v", got)
	}
	if got := payloads[2]; got.ToUser != "@all" || got.MessageType != "image" || got.Image == nil {
		t.Fatalf("image payload=%#v", got)
	}
	if got := payloads[3]; got.MessageType != "voice" || got.Voice == nil {
		t.Fatalf("voice payload=%#v", got)
	}
	if got := payloads[4]; got.ToParty != "2" || got.MessageType != "video" || got.Video == nil || got.Video.Title != "Title" || got.EnableDuplicateCheck != 1 || got.DuplicateCheckInterval != 1800 {
		t.Fatalf("video payload=%#v", got)
	}
	if got := payloads[5]; got.ToTag != "3" || got.MessageType != "file" || got.File == nil {
		t.Fatalf("file payload=%#v", got)
	}
	if got := payloads[6]; got.ToUser != "carol" || got.Text == nil || got.Text.Content != text {
		t.Fatalf("common payload=%#v", got)
	}

	_, err = client.SendApplicationMessage(context.Background(), ApplicationMessageRequest{Recipients: RecipientSet{UserIDs: []string{"alice"}}, Content: TextContent{Content: "rejected"}})
	if !errors.Is(err, socialhub.ErrInvalidArgument) || errorCode(err) != socialhub.CodeInvalidArgument {
		t.Fatalf("81013 error=%v", err)
	}
}

func TestMessageValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, false)
	tooManyUsers := make([]string, 1001)
	for index := range tooManyUsers {
		tooManyUsers[index] = "u" + strconv.Itoa(index)
	}
	longText := strings.Repeat("x", 2049)
	invalidTyped := []ApplicationMessageRequest{
		{Recipients: RecipientSet{UserIDs: []string{"alice"}}},
		{Recipients: RecipientSet{ToAll: true, UserIDs: []string{"alice"}}, Content: TextContent{Content: "x"}},
		{Recipients: RecipientSet{UserIDs: []string{"bad|id"}}, Content: TextContent{Content: "x"}},
		{Recipients: RecipientSet{UserIDs: tooManyUsers}, Content: TextContent{Content: "x"}},
		{Content: TextContent{}},
		{Content: TextContent{Content: longText}},
		{Content: MarkdownContent{}},
		{Content: ImageContent{}},
		{Content: VoiceContent{}},
		{Content: VideoContent{MediaID: "id", Title: strings.Repeat("x", 129)}},
		{Content: FileContent{}},
		{Content: TextContent{Content: "x"}, DuplicateCheckInterval: time.Second},
		{Content: TextContent{Content: "x"}, EnableDuplicateCheck: true, DuplicateCheckInterval: 4*time.Hour + time.Second},
		{Content: TextContent{Content: "x"}, EnableDuplicateCheck: true, DuplicateCheckInterval: time.Millisecond},
	}
	for index, input := range invalidTyped {
		if _, err := client.SendApplicationMessage(context.Background(), input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("typed validation %d=%v", index, err)
		}
	}

	defaults := client.defaults
	client.defaults = RecipientSet{}
	_, missingRecipientsErr := client.SendApplicationMessage(context.Background(), ApplicationMessageRequest{Content: TextContent{Content: "x"}})
	client.defaults = defaults
	if !errors.Is(missingRecipientsErr, socialhub.ErrInvalidArgument) {
		t.Fatalf("missing recipients=%v", missingRecipientsErr)
	}
	text := "x"
	reply := "reply"
	invalidCommon := []socialhub.SendMessageRequest{
		{RecipientIDs: []string{"alice"}, ReplyToID: &reply, Text: &text},
		{RecipientIDs: []string{"alice"}, MediaIDs: []string{"media"}, Text: &text},
		{RecipientIDs: []string{"alice"}},
	}
	for index, input := range invalidCommon {
		if _, err := client.SendMessage(context.Background(), input); err == nil {
			t.Fatalf("common validation %d returned nil", index)
		}
	}
	if _, err := client.GetMessage(context.Background(), "message"); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("get message=%v", err)
	}
}

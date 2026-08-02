package lemmy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestPrivateMessageWorkflow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.Method + " " + request.URL.Path {
		case "POST /api/v3/private_message":
			var payload struct {
				Content     string `json:"content"`
				RecipientID int64  `json:"recipient_id"`
			}
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil || payload.Content != "Hello Bob" || payload.RecipientID != 8 {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusCreated, `{"private_message_view":`+privateMessageViewFixture(51, 7, 8, "alice", "bob", payload.Content, false, false)+`}`)
		case "GET /api/v3/private_message/list":
			query := request.URL.Query()
			if query.Get("creator_id") != "8" || query.Get("page") != "2" || query.Get("limit") != "2" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			incoming := privateMessageViewFixture(52, 8, 7, "bob", "alice", "Reply", false, false)
			outgoing := privateMessageViewFixture(51, 7, 8, "alice", "bob", "Hello Bob", false, false)
			writeJSON(writer, http.StatusOK, `{"private_messages":[`+incoming+`,`+outgoing+`]}`)
		case "PUT /api/v3/private_message":
			var payload struct {
				PrivateMessageID int64  `json:"private_message_id"`
				Content          string `json:"content"`
			}
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil || payload.PrivateMessageID != 51 || payload.Content != "Edited" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"private_message_view":`+privateMessageViewFixture(51, 7, 8, "alice", "bob", payload.Content, false, false)+`}`)
		case "POST /api/v3/private_message/delete":
			writeJSON(writer, http.StatusOK, `{"private_message_view":`+privateMessageViewFixture(51, 7, 8, "alice", "bob", "Edited", true, false)+`}`)
		case "POST /api/v3/private_message/mark_as_read":
			writeJSON(writer, http.StatusOK, `{"private_message_view":`+privateMessageViewFixture(52, 8, 7, "bob", "alice", "Reply", false, true)+`}`)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server)

	sent, err := client.SendPrivateMessage(context.Background(), "8", "Hello Bob")
	if err != nil || sent.Common.ID != "51" || sent.Common.Direction != socialhub.DirectionOutbound ||
		sent.Common.ConversationID != "7:8" || sent.Common.SenderID == nil || *sent.Common.SenderID != "7" ||
		len(sent.Common.RecipientIDs) != 1 || sent.Common.RecipientIDs[0] != "8" || sent.Common.SentAt == nil || len(sent.Raw) == 0 {
		t.Fatalf("sent=%#v err=%v", sent, err)
	}
	page, err := client.ListPrivateMessages(context.Background(), "8", "2", 2)
	if err != nil || len(page.Items) != 2 || page.Items[0].Common.Direction != socialhub.DirectionInbound ||
		page.Items[0].Common.ConversationID != "7:8" || page.NextCursor == nil || *page.NextCursor != "3" ||
		page.PrevCursor == nil || *page.PrevCursor != "1" || !page.HasMore {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	edited, err := client.EditPrivateMessage(context.Background(), "51", "Edited")
	if err != nil || edited.Common.Text == nil || *edited.Common.Text != "Edited" {
		t.Fatalf("edited=%#v err=%v", edited, err)
	}
	if err := client.DeletePrivateMessage(context.Background(), "51"); err != nil {
		t.Fatal(err)
	}
	read, err := client.MarkPrivateMessageRead(context.Background(), "52", true)
	if err != nil || !read.Read {
		t.Fatalf("read=%#v err=%v", read, err)
	}
}

func TestPrivateMessageValidationAndBadResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/v3/private_message":
			writeJSON(writer, http.StatusOK, `{"private_message_view":`+privateMessageViewFixture(99, 7, 9, "alice", "other", "Wrong", false, false)+`}`)
		case "/api/v3/private_message/list":
			writeJSON(writer, http.StatusOK, `{"private_messages":[{"private_message":{"id":1,"creator_id":7,"recipient_id":8},"creator":{"id":0},"recipient":{"id":8}}]}`)
		case "/api/v3/private_message/delete":
			writeJSON(writer, http.StatusOK, `{"private_message_view":`+privateMessageViewFixture(51, 7, 8, "alice", "bob", "Text", false, false)+`}`)
		case "/api/v3/private_message/mark_as_read":
			writeJSON(writer, http.StatusOK, `{"private_message_view":`+privateMessageViewFixture(51, 7, 8, "alice", "bob", "Text", false, false)+`}`)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server)
	invalid := []func() error{
		func() error { _, err := client.SendPrivateMessage(context.Background(), "bad", "text"); return err },
		func() error { _, err := client.SendPrivateMessage(context.Background(), "8", " "); return err },
		func() error { _, err := client.ListPrivateMessages(context.Background(), "bad", "", 1); return err },
		func() error { _, err := client.ListPrivateMessages(context.Background(), "", "bad", 1); return err },
		func() error { _, err := client.EditPrivateMessage(context.Background(), "bad", "text"); return err },
		func() error { _, err := client.EditPrivateMessage(context.Background(), "1", " "); return err },
		func() error { return client.DeletePrivateMessage(context.Background(), "bad") },
		func() error { _, err := client.MarkPrivateMessageRead(context.Background(), "bad", true); return err },
	}
	for index, call := range invalid {
		if err := call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("invalid message %d error=%v", index, err)
		}
	}
	badResponses := []func() error{
		func() error { _, err := client.SendPrivateMessage(context.Background(), "8", "text"); return err },
		func() error { _, err := client.ListPrivateMessages(context.Background(), "", "", 1); return err },
		func() error { return client.DeletePrivateMessage(context.Background(), "51") },
		func() error { _, err := client.MarkPrivateMessageRead(context.Background(), "51", true); return err },
	}
	for index, call := range badResponses {
		var platformErr *socialhub.Error
		if err := call(); !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodePlatformError {
			t.Fatalf("bad message response %d error=%v", index, err)
		}
	}
	if validPrivateMessageView(wirePrivateMessageView{}) || validMessageContent("\x00") {
		t.Fatal("message validation accepted invalid input")
	}
}

func privateMessageViewFixture(id, creatorID, recipientID int64, creator, recipient, content string, deleted, read bool) string {
	return fmt.Sprintf(`{"private_message":{"id":%d,"creator_id":%d,"recipient_id":%d,"content":%q,"deleted":%t,"read":%t,"published":"2026-08-01T03:04:05Z","ap_id":"https://lemmy.example/private_message/%d","local":true},"creator":{"id":%d,"name":%q},"recipient":{"id":%d,"name":%q}}`,
		id, creatorID, recipientID, content, deleted, read, id, creatorID, creator, recipientID, recipient)
}

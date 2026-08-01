package kakao

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestTypedAndCommonMessageContracts(t *testing.T) {
	call := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.Header.Get("Authorization") != "Bearer access-token" ||
			!strings.HasPrefix(request.Header.Get("Content-Type"), "application/x-www-form-urlencoded") || request.ParseForm() != nil {
			t.Errorf("request=%s %s auth=%q content-type=%q", request.Method, request.URL.Path, request.Header.Get("Authorization"), request.Header.Get("Content-Type"))
		}
		call++
		switch call {
		case 1:
			if request.URL.Path != "/v2/api/talk/memo/default/send" || request.Form.Get("receiver_uuids") != "" {
				t.Errorf("self default path/form=%s %v", request.URL.Path, request.Form)
			}
			assertTemplateForm(t, request, map[string]any{
				"object_type": "text", "text": "hello self",
				"link":         map[string]any{"web_url": "https://app.example.test/item"},
				"button_title": "Open",
			})
			writeTestJSON(t, writer, map[string]any{"result_code": 0})
		case 2:
			if request.URL.Path != "/v1/api/talk/friends/message/default/send" || request.Form.Get("receiver_uuids") != `["friend-1","friend-2"]` {
				t.Errorf("friend default path/form=%s %v", request.URL.Path, request.Form)
			}
			assertTemplateForm(t, request, map[string]any{
				"object_type": "text", "text": "hello friends",
				"link": map[string]any{"mobile_web_url": "https://m.example.test/item"},
				"buttons": []any{map[string]any{
					"title": "Details", "link": map[string]any{"android_execution_params": "item=1"},
				}},
			})
			writeTestJSON(t, writer, map[string]any{
				"successful_receiver_uuids": []string{"friend-1"},
				"failure_info":              []map[string]any{{"code": -530, "msg": "refused", "receiver_uuids": []string{"friend-2"}}},
			})
		case 3:
			if request.URL.Path != "/v2/api/talk/memo/send" || request.Form.Get("template_id") != "42" || request.Form.Get("template_args") != `{"name":""}` {
				t.Errorf("self custom path/form=%s %v", request.URL.Path, request.Form)
			}
			writeTestJSON(t, writer, map[string]any{"result_code": 0})
		case 4:
			if request.URL.Path != "/v1/api/talk/friends/message/send" || request.Form.Get("template_id") != "43" || request.Form.Get("receiver_uuids") != `["friend-1"]` || request.Form.Get("template_args") != "" {
				t.Errorf("friend custom path/form=%s %v", request.URL.Path, request.Form)
			}
			writeTestJSON(t, writer, map[string]any{"successful_receiver_uuids": []string{"friend-1"}})
		case 5:
			if request.URL.Path != "/v2/api/talk/memo/default/send" {
				t.Errorf("common self path=%s", request.URL.Path)
			}
			assertTemplateForm(t, request, map[string]any{
				"object_type": "text", "text": "common self",
				"link": map[string]any{
					"web_url": "https://app.example.test/message", "mobile_web_url": "https://app.example.test/message",
				},
			})
			writeTestJSON(t, writer, map[string]any{"result_code": 0})
		case 6:
			if request.URL.Path != "/v1/api/talk/friends/message/default/send" || request.Form.Get("receiver_uuids") != `["friend-1","friend-2"]` {
				t.Errorf("common friends path/form=%s %v", request.URL.Path, request.Form)
			}
			writeTestJSON(t, writer, map[string]any{
				"successful_receiver_uuids": []string{"friend-1"},
				"failure_info":              []map[string]any{{"code": -530, "msg": "refused", "receiver_uuids": []string{"friend-2"}}},
			})
		default:
			t.Errorf("unexpected message call %d", call)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, true, true)

	self, err := client.SendDefault(context.Background(), DefaultMessageRequest{
		Target: MessageTargetSelf,
		Template: TextTemplate{
			Text: "hello self", Link: Link{WebURL: "https://app.example.test/item"}, ButtonTitle: "Open",
		},
	})
	if err != nil || self.Target != MessageTargetSelf || self.ResultCode != 0 {
		t.Fatalf("self result=%#v err=%v", self, err)
	}
	friends, err := client.SendDefault(context.Background(), DefaultMessageRequest{
		Target: MessageTargetFriends, ReceiverUUIDs: []string{"friend-1", "friend-2"},
		Template: TextTemplate{
			Text: "hello friends", Link: Link{MobileWebURL: "https://m.example.test/item"},
			Buttons: []Button{{Title: "Details", Link: Link{AndroidExecutionParams: "item=1"}}},
		},
	})
	if err != nil || len(friends.SuccessfulReceiverUUIDs) != 1 || len(friends.Failures) != 1 || friends.Failures[0].ReceiverUUIDs[0] != "friend-2" {
		t.Fatalf("friend result=%#v err=%v", friends, err)
	}
	if _, err := client.SendCustom(context.Background(), CustomMessageRequest{
		Target: MessageTargetSelf, TemplateID: 42, Arguments: map[string]string{"name": ""},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.SendCustom(context.Background(), CustomMessageRequest{
		Target: MessageTargetFriends, ReceiverUUIDs: []string{"friend-1"}, TemplateID: 43,
	}); err != nil {
		t.Fatal(err)
	}
	selfText := "common self"
	commonSelf, err := client.SendMessage(context.Background(), socialhub.SendMessageRequest{ConversationID: "self", Text: &selfText})
	if err != nil || commonSelf.ConversationID != "self" || len(commonSelf.RecipientIDs) != 1 || commonSelf.RecipientIDs[0] != "123456789" || len(commonSelf.Extensions["kakao.delivery"]) == 0 {
		t.Fatalf("common self=%#v err=%v", commonSelf, err)
	}
	friendText := "common friends"
	commonFriends, err := client.SendMessage(context.Background(), socialhub.SendMessageRequest{
		ConversationID: "friends", RecipientIDs: []string{"friend-1", "friend-2"}, Text: &friendText,
	})
	if err != nil || !reflect.DeepEqual(commonFriends.RecipientIDs, []string{"friend-1"}) || len(commonFriends.Extensions["kakao.delivery"]) == 0 {
		t.Fatalf("common friends=%#v err=%v", commonFriends, err)
	}
	if call != 6 {
		t.Fatalf("message calls=%d", call)
	}
}

func assertTemplateForm(t *testing.T, request *http.Request, expected map[string]any) {
	t.Helper()
	var actual map[string]any
	if err := json.Unmarshal([]byte(request.Form.Get("template_object")), &actual); err != nil || !reflect.DeepEqual(actual, expected) {
		t.Errorf("template=%#v want=%#v err=%v", actual, expected, err)
	}
}

func TestMessageValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, true, true)
	link := Link{WebURL: "https://app.example.test"}
	invalidDefaults := []DefaultMessageRequest{
		{Target: "unknown", Template: TextTemplate{Text: "hello", Link: link}},
		{Target: MessageTargetSelf, ReceiverUUIDs: []string{"friend"}, Template: TextTemplate{Text: "hello", Link: link}},
		{Target: MessageTargetFriends, Template: TextTemplate{Text: "hello", Link: link}},
		{Target: MessageTargetFriends, ReceiverUUIDs: []string{"same", "same"}, Template: TextTemplate{Text: "hello", Link: link}},
		{Target: MessageTargetFriends, ReceiverUUIDs: []string{"bad\n"}, Template: TextTemplate{Text: "hello", Link: link}},
		{Target: MessageTargetSelf, Template: TextTemplate{Text: "", Link: link}},
		{Target: MessageTargetSelf, Template: TextTemplate{Text: strings.Repeat("x", 201), Link: link}},
		{Target: MessageTargetSelf, Template: TextTemplate{Text: "hello"}},
		{Target: MessageTargetSelf, Template: TextTemplate{Text: "hello", Link: Link{WebURL: "file:///tmp"}}},
		{Target: MessageTargetSelf, Template: TextTemplate{Text: "hello", Link: link, Buttons: []Button{{Title: "1", Link: link}, {Title: "2", Link: link}, {Title: "3", Link: link}}}},
		{Target: MessageTargetSelf, Template: TextTemplate{Text: "hello", Link: link, Buttons: []Button{{Title: "", Link: link}}}},
	}
	for index, input := range invalidDefaults {
		if _, err := client.SendDefault(context.Background(), input); err == nil {
			t.Fatalf("invalid default %d unexpectedly succeeded", index)
		}
	}
	invalidCustom := []CustomMessageRequest{
		{Target: MessageTargetSelf, TemplateID: 0},
		{Target: MessageTargetSelf, TemplateID: 1, Arguments: map[string]string{"": "value"}},
		{Target: MessageTargetSelf, TemplateID: 1, Arguments: map[string]string{"key": "bad\nvalue"}},
	}
	for index, input := range invalidCustom {
		if _, err := client.SendCustom(context.Background(), input); err == nil {
			t.Fatalf("invalid custom %d unexpectedly succeeded", index)
		}
	}
	text, reply := "hello", "message"
	commonInvalid := []socialhub.SendMessageRequest{
		{ConversationID: "self"},
		{ConversationID: "self", Text: &text, RecipientIDs: []string{"friend"}},
		{ConversationID: "self", Text: &text, MediaIDs: []string{"media"}},
		{ConversationID: "self", Text: &text, ReplyToID: &reply},
		{ConversationID: "unknown", Text: &text},
	}
	for index, input := range commonInvalid {
		if _, err := client.SendMessage(context.Background(), input); err == nil {
			t.Fatalf("invalid common %d unexpectedly succeeded", index)
		}
	}
	_, noCommon := newTestClient(t, server, true, false)
	if _, err := noCommon.SendMessage(context.Background(), socialhub.SendMessageRequest{ConversationID: "self", Text: &text}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("common without link=%v", err)
	}
	if _, err := client.GetMessage(context.Background(), "message"); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("get message=%v", err)
	}
}

func TestMalformedMessageResponses(t *testing.T) {
	responses := []map[string]any{
		{},
		{"successful_receiver_uuids": []string{"unknown"}},
		{
			"successful_receiver_uuids": []string{"friend-1"},
			"failure_info":              []map[string]any{{"code": -530, "receiver_uuids": []string{"friend-1"}}},
		},
		{"failure_info": []map[string]any{{"code": -530, "receiver_uuids": []string{}}}},
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		response := responses[0]
		responses = responses[1:]
		writeTestJSON(t, writer, response)
	}))
	defer server.Close()
	_, client := newTestClient(t, server, true, true)
	template := TextTemplate{Text: "hello", Link: Link{WebURL: "https://app.example.test"}}
	if _, err := client.SendDefault(context.Background(), DefaultMessageRequest{Target: MessageTargetSelf, Template: template}); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("missing result=%v", err)
	}
	for index := 0; index < 3; index++ {
		if _, err := client.SendDefault(context.Background(), DefaultMessageRequest{
			Target: MessageTargetFriends, ReceiverUUIDs: []string{"friend-1"}, Template: template,
		}); errorCode(err) != socialhub.CodePlatformError {
			t.Fatalf("malformed friend %d=%v", index, err)
		}
	}
}

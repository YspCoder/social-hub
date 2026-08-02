package zalo

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestMessageAndProfileContracts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("access_token") != "oa-access-token" || request.Header.Get("Authorization") != "" {
			t.Errorf("auth headers=%v", request.Header)
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/v3.0/oa/message/cs":
			if request.Method != http.MethodPost || request.Header.Get("Content-Type") != "application/json" {
				t.Errorf("message request=%s %s", request.Method, request.Header.Get("Content-Type"))
			}
			var body struct {
				Recipient map[string]string `json:"recipient"`
				Message   map[string]string `json:"message"`
			}
			if json.NewDecoder(request.Body).Decode(&body) != nil || body.Recipient["user_id"] != "2512523625412515" || body.Message["text"] != "hello" {
				t.Errorf("message body=%#v", body)
			}
			writeTestJSON(t, writer, map[string]any{
				"error": 0, "message": "Success",
				"data": map[string]any{
					"message_id": "63ecf43f0df7dba892e6", "user_id": "2512523625412515", "sent_time": "1785672000000",
					"quota": map[string]any{"quota_type": "reply", "remain": "8", "total": "8"},
				},
			})
		case "/v2.0/oa/getoa":
			if request.Method != http.MethodGet {
				t.Errorf("get OA method=%s", request.Method)
			}
			writeTestJSON(t, writer, map[string]any{
				"error": 0, "message": "Success",
				"data": map[string]any{
					"oaid": "388613280878808645", "name": "Social Hub VN", "description": "Support",
					"oa_alias": "socialhub", "is_verified": true, "oa_type": 2, "num_follower": 42,
				},
			})
		case "/v3.0/oa/user/detail":
			var query map[string]string
			if json.Unmarshal([]byte(request.URL.Query().Get("data")), &query) != nil || query["user_id"] != "2512523625412515" {
				t.Errorf("user query=%q", request.URL.RawQuery)
			}
			writeTestJSON(t, writer, map[string]any{
				"error": 0, "message": "Success",
				"data": map[string]any{
					"user_id": "2512523625412515", "user_id_by_app": "4604138790644222978",
					"display_name": "Ada", "user_alias": "ada", "avatar": "https://cdn.example/ada.jpg",
					"user_is_follower": true, "tags_and_notes_info": map[string]any{"tag_names": []string{"VIP"}},
				},
			})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, false, false)

	text := "hello"
	message, err := client.SendMessage(context.Background(), socialhub.SendMessageRequest{
		ConversationID: "2512523625412515", RecipientIDs: []string{"2512523625412515"}, Text: &text,
	})
	if err != nil || message.ID != "63ecf43f0df7dba892e6" || message.Direction != socialhub.DirectionOutbound ||
		message.SentAt == nil || !message.SentAt.Equal(time.Date(2026, time.August, 2, 12, 0, 0, 0, time.UTC)) || len(message.Extensions) != 1 {
		t.Fatalf("message=%#v err=%v", message, err)
	}
	result, err := client.SendConsultationText(context.Background(), "2512523625412515", "hello")
	if err != nil || result.Quota == nil || result.Quota.Type != "reply" || result.Quota.Remain != "8" {
		t.Fatalf("send result=%#v err=%v", result, err)
	}
	oa, err := client.GetOA(context.Background())
	if err != nil || oa.ID != "388613280878808645" || oa.Name != "Social Hub VN" || !oa.Verified || oa.FollowerCount != 42 {
		t.Fatalf("OA=%#v err=%v", oa, err)
	}
	profile, err := client.GetUserProfile(context.Background(), "2512523625412515")
	if err != nil || profile.DisplayName != "Ada" || len(profile.TagsAndNotes.TagNames) != 1 {
		t.Fatalf("profile=%#v err=%v", profile, err)
	}
	user, err := client.GetUser(context.Background(), "2512523625412515")
	if err != nil || user.DisplayName == nil || *user.DisplayName != "Ada" || user.Username == nil || *user.Username != "ada" || user.AvatarURL == nil {
		t.Fatalf("user=%#v err=%v", user, err)
	}
}

func TestWorkflowValidationAndPlatformErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Query().Get("case") {
		case "http":
			writer.Header().Set("Retry-After", "2")
			writer.WriteHeader(http.StatusTooManyRequests)
			writeTestJSON(t, writer, map[string]any{"message": "slow down"})
		default:
			writeTestJSON(t, writer, map[string]any{"error": -32, "message": "Your application reached limit call api"})
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, false, false)

	invalidMessages := []socialhub.SendMessageRequest{
		{},
		{ConversationID: "bad id", Text: pointer("hello")},
		{ConversationID: "2512523625412515", Text: pointer("hello"), RecipientIDs: []string{"2"}},
		{ConversationID: "2512523625412515", Text: pointer("hello"), MediaIDs: []string{"media"}},
		{ConversationID: "2512523625412515", Text: pointer("hello"), ReplyToID: pointer("reply")},
	}
	for index, input := range invalidMessages {
		if _, err := client.SendMessage(context.Background(), input); index < 3 && !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("message %d error=%v", index, err)
		} else if index >= 3 && !errors.Is(err, socialhub.ErrUnsupported) {
			t.Fatalf("message %d error=%v", index, err)
		}
	}
	if _, err := client.SendConsultationText(context.Background(), "bad", "hello"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("bad send=%v", err)
	}
	if _, err := client.SendConsultationText(context.Background(), "2512523625412515", string(make([]rune, 2001))); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("long send=%v", err)
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
	_, err := client.SendConsultationText(context.Background(), "2512523625412515", "hello")
	if !errors.Is(err, socialhub.ErrRateLimited) {
		t.Fatalf("API rate error=%v", err)
	}
	if got := errorCode(mapAPIError("test", -216, "invalid token")); got != socialhub.CodeUnauthenticated {
		t.Fatalf("auth code=%s", got)
	}
	if got := errorCode(mapAPIError("test", -205, "missing")); got != socialhub.CodeNotFound {
		t.Fatalf("not found code=%s", got)
	}
	if got := errorCode(mapAPIError("test", -214, "processing")); got != socialhub.CodeTemporarilyUnavailable {
		t.Fatalf("retry code=%s", got)
	}
}

func pointer(value string) *string { return &value }

package kakao

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestUserAndFriendContracts(t *testing.T) {
	userCalls, friendCalls := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer access-token" {
			t.Errorf("authorization=%q", request.Header.Get("Authorization"))
		}
		switch request.URL.Path {
		case "/v2/user/me":
			userCalls++
			if request.Method != http.MethodGet || request.URL.Query().Get("secure_resource") != "true" {
				t.Errorf("user request=%s %s", request.Method, request.URL.String())
			}
			signedUp := true
			writeTestJSON(t, writer, map[string]any{
				"id": 123456789, "connected_at": "2026-01-02T03:04:05Z", "synched_at": "2026-01-03T03:04:05Z",
				"has_signed_up": signedUp,
				"kakao_account": map[string]any{
					"email": "user@example.test", "is_email_valid": true, "is_email_verified": true,
					"profile": map[string]any{
						"nickname": "Ryan", "profile_image_url": "https://cdn.example.test/profile.png",
						"thumbnail_image_url": "https://cdn.example.test/thumb.png", "is_default_nickname": false,
					},
				},
				"for_partner": map[string]any{"uuid": "friend-self-uuid"},
			})
		case "/v1/api/talk/friends":
			friendCalls++
			query := request.URL.Query()
			if request.Method != http.MethodGet || query.Get("offset") != "3" || query.Get("limit") != "2" || query.Get("order") != "desc" || query.Get("friend_order") != "nickname" {
				t.Errorf("friend request=%s %s", request.Method, request.URL.String())
			}
			writeTestJSON(t, writer, map[string]any{
				"elements": []map[string]any{
					{"id": 101, "uuid": "friend-1", "profile_nickname": "Apeach", "profile_thumbnail_image": "https://cdn.example.test/a.png", "favorite": true},
					{"id": 102, "uuid": "friend-2", "profile_nickname": "Jordy", "favorite": false},
				},
				"total_count": 8, "favorite_count": 1,
				"after_url": "https://untrusted.example.test/should-not-be-followed",
			})
		default:
			t.Errorf("unexpected path %s", request.URL.Path)
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, true, true)

	user, err := client.Me(context.Background())
	if err != nil || user.ID != "123456789" || user.DisplayName == nil || *user.DisplayName != "Ryan" ||
		user.AvatarURL == nil || *user.AvatarURL != "https://cdn.example.test/profile.png" || user.AccountType == nil ||
		len(user.Extensions["kakao.login"]) == 0 {
		t.Fatalf("user=%#v err=%v", user, err)
	}
	var extension map[string]any
	if err := json.Unmarshal(user.Extensions["kakao.login"], &extension); err != nil || extension["friend_uuid"] != "friend-self-uuid" || extension["email"] != "user@example.test" {
		t.Fatalf("extension=%#v err=%v", extension, err)
	}
	if _, err := client.GetUser(context.Background(), "123456789"); err != nil {
		t.Fatal(err)
	}
	page, err := client.ListFriends(context.Background(), ListFriendsRequest{
		Offset: 3, Limit: 2, Order: FriendOrderDescending, Sort: FriendSortNickname,
	})
	if err != nil || len(page.Items) != 2 || page.Items[0].UUID != "friend-1" || page.TotalCount != 8 ||
		page.NextOffset == nil || *page.NextOffset != 5 || !page.HasMore {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	if userCalls != 2 || friendCalls != 1 {
		t.Fatalf("calls user=%d friends=%d", userCalls, friendCalls)
	}
	if _, err := client.GetUser(context.Background(), "other"); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("other user=%v", err)
	}
	if _, err := client.GetPost(context.Background(), "post"); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("get post=%v", err)
	}
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("list posts=%v", err)
	}
	if _, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("list comments=%v", err)
	}
}

func TestUserAndFriendValidationAndMalformedResponses(t *testing.T) {
	call := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		call++
		switch call {
		case 1:
			writeTestJSON(t, writer, map[string]any{"id": 999})
		case 2:
			writeTestJSON(t, writer, map[string]any{
				"id":            123456789,
				"kakao_account": map[string]any{"profile": map[string]any{"nickname": "Ryan", "profile_image_url": "file:///tmp/profile.png"}},
			})
		case 3:
			writeTestJSON(t, writer, map[string]any{"total_count": 1, "favorite_count": 2})
		case 4:
			writeTestJSON(t, writer, map[string]any{
				"elements": []map[string]any{{"id": 1, "uuid": "bad\n"}}, "total_count": 1,
			})
		case 5:
			writeTestJSON(t, writer, map[string]any{"elements": []map[string]any{}, "total_count": 1, "after_url": "https://kapi.kakao.com/next"})
		default:
			t.Errorf("unexpected request %d path=%s", call, request.URL.Path)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, true, true)
	if _, err := client.Me(context.Background()); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("mismatched user=%v", err)
	}
	if _, err := client.Me(context.Background()); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("bad profile URL=%v", err)
	}
	if _, err := client.ListFriends(context.Background(), ListFriendsRequest{}); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("invalid counts=%v", err)
	}
	if _, err := client.ListFriends(context.Background(), ListFriendsRequest{}); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("bad friend=%v", err)
	}
	if _, err := client.ListFriends(context.Background(), ListFriendsRequest{}); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("empty next page=%v", err)
	}
	invalid := []ListFriendsRequest{
		{Offset: -1}, {Limit: -1}, {Limit: 101}, {Order: "sideways"}, {Sort: "unknown"},
	}
	for index, input := range invalid {
		if _, err := client.ListFriends(context.Background(), input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("invalid request %d=%v", index, err)
		}
	}
}

package vk

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func requireVKRequest(t *testing.T, writer http.ResponseWriter, request *http.Request, method string) (url.Values, bool) {
	t.Helper()
	if request.Method != http.MethodPost || request.URL.Path != "/method/"+method || request.URL.RawQuery != "" {
		http.Error(writer, "bad request target", http.StatusBadRequest)
		t.Errorf("request=%s %s", request.Method, request.URL.String())
		return nil, false
	}
	if request.Header.Get("Authorization") != "Bearer access-token" || request.Header.Get("Content-Type") != "application/x-www-form-urlencoded" || request.Header.Get("Idempotency-Key") != "" {
		http.Error(writer, "bad request headers", http.StatusUnauthorized)
		t.Errorf("headers=%v", request.Header)
		return nil, false
	}
	if err := request.ParseForm(); err != nil {
		http.Error(writer, "bad form", http.StatusBadRequest)
		t.Errorf("parse form: %v", err)
		return nil, false
	}
	if request.Form.Get("v") != apiVersion {
		http.Error(writer, "bad API version", http.StatusBadRequest)
		t.Errorf("form=%v", request.Form)
		return nil, false
	}
	return request.Form, true
}

func TestWallPublishingAndRepostContracts(t *testing.T) {
	call := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		method := strings.TrimPrefix(request.URL.Path, "/method/")
		form, ok := requireVKRequest(t, writer, request, method)
		if !ok {
			return
		}
		call++
		switch method {
		case "wall.post":
			if call == 1 {
				if form.Get("owner_id") != "123" || form.Get("message") != "hello" || form.Get("attachments") != "photo123_7_key,video-456_8" || form.Get("friends_only") != "1" || form.Get("guid") != "guid-1" || request.Header.Get("X-Request-ID") != "request-1" {
					t.Errorf("common wall.post form=%v headers=%v", form, request.Header)
				}
				writeTestJSON(t, writer, map[string]any{"response": map[string]any{"post_id": 101}})
				return
			}
			publishAt := testNow.Add(2 * time.Hour).Unix()
			if form.Get("owner_id") != "-456" || form.Get("from_group") != "1" || form.Get("signed") != "1" || form.Get("close_comments") != "1" || form.Get("mute_notifications") != "1" || form.Get("publish_date") != strconv.FormatInt(publishAt, 10) {
				t.Errorf("typed wall.post form=%v", form)
			}
			writeTestJSON(t, writer, map[string]any{"response": map[string]any{"post_id": 102}})
		case "wall.repost":
			if form.Get("object") != "wall-456_9" || form.Get("message") != "boost" || form.Get("group_id") != "456" {
				t.Errorf("wall.repost form=%v", form)
			}
			writeTestJSON(t, writer, map[string]any{"response": map[string]any{"success": 1, "post_id": 103}})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, TokenUser, 123, false)

	text, visibility := "hello", "friends"
	post, err := client.Publish(context.Background(), socialhub.CreatePostRequest{
		Text: &text, MediaIDs: []string{"photo123_7_key", "video-456_8"}, Visibility: &visibility,
	}, socialhub.WithIdempotencyKey("guid-1"), socialhub.WithRequestID("request-1"))
	if err != nil || post.ID != "123_101" || post.Text == nil || *post.Text != text || post.Visibility == nil || *post.Visibility != visibility || post.Status == nil || post.Status.State != socialhub.PublishStatePublished || post.URL == nil || len(post.Extensions) != 1 {
		t.Fatalf("published post=%#v error=%v", post, err)
	}

	publishAt := testNow.Add(2 * time.Hour)
	post, err = client.CreateWallPost(context.Background(), WallPostRequest{
		OwnerID: -456, Message: "scheduled", Attachments: []string{"photo-456_7"}, FromGroup: true,
		Signed: true, CloseComments: true, MuteNotifications: true, PublishAt: &publishAt,
	})
	if err != nil || post.ID != "-456_102" || post.Status == nil || post.Status.State != socialhub.PublishStatePending || post.CreatedAt == nil || !post.CreatedAt.Equal(publishAt) {
		t.Fatalf("scheduled post=%#v error=%v", post, err)
	}

	repost, err := client.Repost(context.Background(), RepostRequest{Object: "wall-456_9", Message: "boost", DestinationOwnerID: -456})
	if err != nil || repost.ID != "-456_103" || len(repost.Relations) != 1 || repost.Relations[0].Type != socialhub.RelationRepost || repost.Relations[0].PostID != "-456_9" {
		t.Fatalf("repost=%#v error=%v", repost, err)
	}
	if call != 3 {
		t.Fatalf("calls=%d", call)
	}
}

func TestFetchProfilesPostsCommentsAndDelete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		method := strings.TrimPrefix(request.URL.Path, "/method/")
		form, ok := requireVKRequest(t, writer, request, method)
		if !ok {
			return
		}
		switch method {
		case "users.get":
			if form.Get("user_ids") != "123" || !strings.Contains(form.Get("fields"), "screen_name") {
				t.Errorf("users.get form=%v", form)
			}
			writeTestJSON(t, writer, map[string]any{"response": []any{map[string]any{"id": 123, "first_name": "Ada", "last_name": "Lovelace", "screen_name": "ada", "photo_200": "https://cdn.test/ada.jpg"}}})
		case "groups.getById":
			if form.Get("group_id") != "456" {
				t.Errorf("groups.getById form=%v", form)
			}
			writeTestJSON(t, writer, map[string]any{"response": map[string]any{"groups": []any{map[string]any{"id": 456, "name": "Community", "type": "group", "photo_200": "https://cdn.test/group.jpg"}}}})
		case "wall.getById":
			if form.Get("posts") != "-456_7" {
				t.Errorf("wall.getById form=%v", form)
			}
			writeTestJSON(t, writer, map[string]any{"response": map[string]any{"items": []any{testWirePost()}}})
		case "wall.get":
			if form.Get("owner_id") != "-456" || form.Get("offset") != "2" || form.Get("count") != "2" || form.Get("filter") != "all" {
				t.Errorf("wall.get form=%v", form)
			}
			writeTestJSON(t, writer, map[string]any{"response": map[string]any{"count": 6, "items": []any{testWirePost(), map[string]any{"id": 8, "owner_id": -456, "from_id": -456, "date": testNow.Unix(), "text": "next"}}}})
		case "wall.getComments":
			if form.Get("owner_id") != "-456" || form.Get("post_id") != "7" || form.Get("offset") != "2" || form.Get("count") != "2" || form.Get("need_likes") != "1" {
				t.Errorf("wall.getComments form=%v", form)
			}
			writeTestJSON(t, writer, map[string]any{"response": map[string]any{"count": 5, "items": []any{map[string]any{"id": 11, "from_id": 123, "date": testNow.Unix(), "text": "nice", "reply_to_comment": 10, "likes": map[string]any{"count": 2}, "thread": map[string]any{"count": 1}}}}})
		case "wall.delete":
			if form.Get("owner_id") != "-456" || form.Get("post_id") != "7" {
				t.Errorf("wall.delete form=%v", form)
			}
			writeTestJSON(t, writer, map[string]any{"response": 1})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, TokenUser, -456, false)

	user, err := client.GetUser(context.Background(), "123")
	if err != nil || user.ID != "123" || user.Username == nil || *user.Username != "ada" || user.DisplayName == nil || *user.DisplayName != "Ada Lovelace" || user.ProfileURL == nil || *user.ProfileURL != "https://vk.ru/id123" || len(user.Extensions) != 1 {
		t.Fatalf("user=%#v error=%v", user, err)
	}
	group, err := client.GetUser(context.Background(), "-456")
	if err != nil || group.ID != "-456" || group.Username == nil || *group.Username != "club456" || group.ProfileURL == nil || *group.ProfileURL != "https://vk.ru/club456" {
		t.Fatalf("group=%#v error=%v", group, err)
	}
	post, err := client.GetPost(context.Background(), "wall-456_7")
	if err != nil || post.ID != "-456_7" || len(post.Media) != 4 || post.Media[0].ID != "photo-456_1_key" || post.Media[0].Width == nil || *post.Media[0].Width != 800 || post.Media[1].URL != "https://cdn.test/video-large.jpg" || len(post.Relations) != 1 || len(post.Metrics) != 4 {
		t.Fatalf("post=%#v error=%v", post, err)
	}
	page, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{UserID: "-456", Cursor: "2", MaxResults: 2})
	if err != nil || len(page.Items) != 2 || page.NextCursor == nil || *page.NextCursor != "4" || page.PrevCursor == nil || *page.PrevCursor != "0" || !page.HasMore {
		t.Fatalf("posts page=%#v error=%v", page, err)
	}
	comments, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "-456_7", Cursor: "2", MaxResults: 2})
	if err != nil || len(comments.Items) != 1 || comments.Items[0].ID != "-456_11" || comments.Items[0].ParentID == nil || *comments.Items[0].ParentID != "-456_10" || comments.NextCursor == nil || *comments.NextCursor != "4" {
		t.Fatalf("comments page=%#v error=%v", comments, err)
	}
	status, err := client.PublishStatus(context.Background(), "-456_7")
	if err != nil || status.ID != "-456_7" || status.State != socialhub.PublishStatePublished {
		t.Fatalf("status=%#v error=%v", status, err)
	}
	if err := client.DeletePost(context.Background(), "-456_7"); err != nil {
		t.Fatal(err)
	}
}

func testWirePost() map[string]any {
	return map[string]any{
		"id": 7, "owner_id": -456, "from_id": 123, "date": testNow.Unix(), "text": "post", "post_type": "post",
		"comments": map[string]any{"count": 3}, "likes": map[string]any{"count": 4}, "reposts": map[string]any{"count": 5}, "views": map[string]any{"count": 6},
		"attachments": []any{
			map[string]any{"type": "photo", "photo": map[string]any{"id": 1, "owner_id": -456, "access_key": "key", "width": 100, "height": 100, "sizes": []any{map[string]any{"url": "https://cdn.test/small.jpg", "width": 200, "height": 200}, map[string]any{"url": "https://cdn.test/large.jpg", "width": 800, "height": 600}}}},
			map[string]any{"type": "video", "video": map[string]any{"id": 2, "owner_id": -456, "duration": 9, "image": []any{map[string]any{"url": "https://cdn.test/video-large.jpg", "width": 640, "height": 480}, map[string]any{"url": "https://cdn.test/video-small.jpg", "width": 100, "height": 100}}}},
			map[string]any{"type": "audio", "audio": map[string]any{"id": 3, "owner_id": -456, "duration": 12, "url": "https://cdn.test/audio.mp3"}},
			map[string]any{"type": "doc", "doc": map[string]any{"id": 4, "owner_id": -456, "size": 99, "ext": "pdf", "url": "https://cdn.test/doc.pdf"}},
			map[string]any{"type": "link"},
		},
		"copy_history": []any{map[string]any{"id": 2, "owner_id": 123}},
	}
}

func TestWallAndFetchValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, user := newTestAdapter(t, server, TokenUser, 123, false)
	_, community := newTestAdapter(t, server, TokenCommunity, -456, false)
	_, service := newTestAdapter(t, server, TokenService, 789, false)
	text, blank, badVisibility, quote, reply := "text", " ", "private", "bad", "123_1"
	tooMany := make([]string, 11)
	for index := range tooMany {
		tooMany[index] = "photo123_" + strconv.Itoa(index+1)
	}
	past := testNow.Add(-time.Second)
	invalidCalls := []func() error{
		func() error {
			_, err := community.CreateWallPost(context.Background(), WallPostRequest{Message: "x"})
			return err
		},
		func() error { _, err := user.CreateWallPost(context.Background(), WallPostRequest{}); return err },
		func() error {
			_, err := user.CreateWallPost(context.Background(), WallPostRequest{Message: "x", Attachments: tooMany})
			return err
		},
		func() error {
			_, err := user.CreateWallPost(context.Background(), WallPostRequest{Message: "x", Attachments: []string{"bad"}})
			return err
		},
		func() error {
			_, err := user.CreateWallPost(context.Background(), WallPostRequest{OwnerID: -1, Message: "x", FriendsOnly: true})
			return err
		},
		func() error {
			_, err := user.CreateWallPost(context.Background(), WallPostRequest{OwnerID: 1, Message: "x", FromGroup: true})
			return err
		},
		func() error {
			_, err := user.CreateWallPost(context.Background(), WallPostRequest{Message: "x", PublishAt: &past})
			return err
		},
		func() error {
			_, err := user.CreateWallPost(context.Background(), WallPostRequest{Message: "x"}, socialhub.WithIdempotencyKey("bad key"))
			return err
		},
		func() error {
			_, err := user.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text, ReplyToID: &reply})
			return err
		},
		func() error {
			_, err := user.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text, QuotePostID: &blank})
			return err
		},
		func() error {
			_, err := user.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text, QuotePostID: &quote, MediaIDs: []string{"photo1_1"}})
			return err
		},
		func() error {
			_, err := user.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text, Visibility: &badVisibility})
			return err
		},
		func() error { _, err := user.Repost(context.Background(), RepostRequest{Object: "bad"}); return err },
		func() error {
			_, err := user.Repost(context.Background(), RepostRequest{Object: "123_1", DestinationOwnerID: 999})
			return err
		},
		func() error { _, err := service.GetUser(context.Background(), "bad"); return err },
		func() error { _, err := user.GetPost(context.Background(), "bad"); return err },
		func() error {
			_, err := user.ListPosts(context.Background(), socialhub.ListPostsRequest{UserID: "bad"})
			return err
		},
		func() error {
			_, err := user.ListPosts(context.Background(), socialhub.ListPostsRequest{Cursor: "-1"})
			return err
		},
		func() error {
			_, err := user.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "bad"})
			return err
		},
	}
	for index, call := range invalidCalls {
		if err := call(); err == nil {
			t.Fatalf("validation %d accepted", index)
		}
	}
	start := testNow.Add(-time.Hour)
	if _, err := user.ListPosts(context.Background(), socialhub.ListPostsRequest{StartTime: &start}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("time filter=%v", err)
	}
	if err := service.DeletePost(context.Background(), "123_1"); !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("service delete=%v", err)
	}
	if !validAttachment("photo-1_2_key") || !validAttachment("doc1_2") || validAttachment("photo0_2") || validAttachment("link1_2") || validAttachment("photo1_2_bad key") {
		t.Fatal("attachment validation mismatch")
	}
}

package lemmy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestFetchUsersPostsAndComments(t *testing.T) {
	server := httptest.NewServer(nil)
	defer server.Close()
	server.Config.Handler = http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer jwt-token" || request.Header.Get("User-Agent") != userAgent {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/api/v3/person":
			if request.URL.Query().Get("person_id") == "9" {
				writeJSON(writer, http.StatusOK, personResponseFixture(9, "bob", nil))
				return
			}
			if request.URL.Query().Get("username") != "alice" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			if request.URL.Query().Get("page") == "2" {
				posts := []string{postViewFixture(21, 7, 5, "First post"), postViewFixture(22, 7, 5, "Second post")}
				writeJSON(writer, http.StatusOK, personResponseFixture(7, "alice", posts))
				return
			}
			writeJSON(writer, http.StatusOK, personResponseFixture(7, "alice", nil))
		case "/api/v3/post":
			if request.URL.Query().Get("id") != "21" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"post_view":`+postViewFixture(21, 7, 5, "First post")+`}`)
		case "/api/v3/comment/list":
			query := request.URL.Query()
			if query.Get("post_id") != "21" || query.Get("page") != "2" || query.Get("limit") != "1" || query.Get("sort") != "Old" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"comments":[`+commentViewFixture(31, 21, 8, "0.30.31", "Nested")+`]}`)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	})
	_, client := newTestClient(t, server)

	user, err := client.GetUser(context.Background(), "")
	if err != nil || user.ID != "7" || user.Username == nil || *user.Username != "alice" || user.DisplayName == nil ||
		user.AvatarURL == nil || *user.AvatarURL != server.URL+"/avatar.png" || user.ProfileURL == nil ||
		*user.ProfileURL != server.URL+"/u/alice" || user.AccountType == nil || *user.AccountType != "admin" ||
		len(user.Extensions["lemmy.person_view"]) == 0 {
		t.Fatalf("user=%#v err=%v", user, err)
	}
	bob, err := client.GetUser(context.Background(), "9")
	if err != nil || bob.ID != "9" || bob.Username == nil || *bob.Username != "bob" {
		t.Fatalf("bob=%#v err=%v", bob, err)
	}

	post, err := client.GetPost(context.Background(), "21")
	if err != nil || post.ID != "21" || post.AuthorID == nil || *post.AuthorID != "7" || post.Text == nil ||
		*post.Text != "A body" || post.URL == nil || *post.URL != server.URL+"/post/21" || len(post.Media) != 1 ||
		post.Media[0].URL != server.URL+"/pictrs/image/image.png" || post.Media[0].Width == nil || *post.Media[0].Width != 640 ||
		len(post.Metrics) != 4 || len(post.Extensions["lemmy.post_view"]) == 0 {
		t.Fatalf("post=%#v err=%v", post, err)
	}

	posts, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{Cursor: "2", MaxResults: 2})
	if err != nil || len(posts.Items) != 2 || posts.NextCursor == nil || *posts.NextCursor != "3" ||
		posts.PrevCursor == nil || *posts.PrevCursor != "1" || !posts.HasMore {
		t.Fatalf("posts=%#v err=%v", posts, err)
	}
	comments, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "21", Cursor: "2", MaxResults: 1})
	if err != nil || len(comments.Items) != 1 || comments.Items[0].ID != "31" || comments.Items[0].ParentID == nil ||
		*comments.Items[0].ParentID != "30" || comments.Items[0].AuthorID == nil || *comments.Items[0].AuthorID != "8" ||
		comments.NextCursor == nil || *comments.NextCursor != "3" || comments.PrevCursor == nil || *comments.PrevCursor != "1" {
		t.Fatalf("comments=%#v err=%v", comments, err)
	}

	start := time.Now()
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{StartTime: &start}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("time range=%v", err)
	}
	if _, err := client.GetPost(context.Background(), "bad"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("bad post=%v", err)
	}
	if _, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "0"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("bad comments=%v", err)
	}
}

func TestMappingFallbacksAndWireValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server)

	for index, target := range []json.Unmarshaler{&wirePersonView{}, &wirePostView{}, &wireCommentView{}, &wirePrivateMessageView{}} {
		if err := target.UnmarshalJSON([]byte(`{`)); err == nil {
			t.Fatalf("decoder %d accepted invalid JSON", index)
		}
	}
	post := client.mapPost(wirePostView{
		Post:      wirePost{ID: 1, Name: "No IDs", APID: ":bad", ThumbnailURL: "data:image/png;base64,eA=="},
		Community: wireCommunity{Visibility: ""}, Raw: json.RawMessage(`{"post":{}}`),
	})
	if post.Common.AuthorID != nil || post.LanguageID != "" || post.Common.URL != nil || len(post.Common.Media) != 0 ||
		post.Common.Visibility == nil || *post.Common.Visibility != "public" {
		t.Fatalf("mapped fallback=%#v", post)
	}
	bot := client.mapUser(wirePersonView{Person: wirePerson{ID: 2, Name: "bot", BotAccount: true}})
	if bot.AccountType == nil || *bot.AccountType != "bot" {
		t.Fatalf("bot=%#v", bot)
	}
	member := client.mapUser(wirePersonView{Person: wirePerson{ID: 3, Name: "member"}})
	if member.AccountType == nil || *member.AccountType != "member" {
		t.Fatalf("member=%#v", member)
	}
	if client.absoluteURL("") != nil || client.absoluteURL(":bad") != nil || client.absoluteURL("data:text/plain,x") != nil {
		t.Fatal("invalid absolute URL accepted")
	}
	if parent, err := parentFromPath("0.31", 31); err != nil || parent != "" {
		t.Fatalf("root parent=%q err=%v", parent, err)
	}
	for _, path := range []string{"bad", "1.31", "0.bad", "0.30"} {
		if _, err := parentFromPath(path, 31); err == nil {
			t.Fatalf("invalid path %q accepted", path)
		}
	}
	if timestamp := parseTimestamp("2026-08-01T01:02:03"); timestamp == nil || timestamp.Location() != time.UTC {
		t.Fatalf("timestamp=%v", timestamp)
	}
	if parseTimestamp("bad") != nil || parseTimestamp("") != nil {
		t.Fatal("invalid timestamp accepted")
	}
	if mediaTypeFromMIME("image/gif") != socialhub.MediaTypeAnimation || mediaTypeFromMIME("video/mp4") != socialhub.MediaTypeVideo ||
		mediaTypeFromMIME("audio/mpeg") != socialhub.MediaTypeAudio || mediaTypeFromMIME("image/png") != socialhub.MediaTypeImage ||
		!isMediaMIME("video/mp4") || isMediaMIME("text/plain") {
		t.Fatal("media mapping failed")
	}
}

func TestFetchBadResponsesAndRequestEncoding(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/v3/person":
			if request.URL.Query().Get("person_id") == "9" {
				writeJSON(writer, http.StatusOK, personResponseFixture(8, "wrong", nil))
				return
			}
			writeJSON(writer, http.StatusOK, `{"person_view":{"person":{"id":0}}}`)
		case "/api/v3/post":
			if request.URL.Query().Get("id") == "2" {
				writeJSON(writer, http.StatusOK, `{`)
				return
			}
			writeJSON(writer, http.StatusOK, `{"post_view":`+postViewFixture(99, 7, 5, "Wrong post")+`}`)
		case "/api/v3/comment/list":
			writeJSON(writer, http.StatusOK, `{"comments":[`+commentViewFixture(31, 99, 8, "invalid", "Bad")+`]}`)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server)
	badCalls := []func() error{
		func() error { _, err := client.GetUser(context.Background(), ""); return err },
		func() error { _, err := client.GetUser(context.Background(), "9"); return err },
		func() error { _, err := client.GetPost(context.Background(), "1"); return err },
		func() error { _, err := client.GetPost(context.Background(), "2"); return err },
		func() error {
			_, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "21"})
			return err
		},
	}
	for index, call := range badCalls {
		var platformErr *socialhub.Error
		if err := call(); !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodePlatformError {
			t.Fatalf("bad response %d error=%v", index, err)
		}
	}
	if err := client.requestJSON(context.Background(), http.MethodPost, "/post", nil, func() {}, nil); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("marshal error=%v", err)
	}
	if _, err := client.GetUser(context.Background(), "bad/name"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("bad user=%v", err)
	}
}

func personResponseFixture(id int64, name string, posts []string) string {
	postJSON := strings.Join(posts, ",")
	return fmt.Sprintf(`{"person_view":{"person":{"id":%d,"name":%q,"display_name":"Display %s","avatar":"/avatar.png","actor_id":"/u/%s"},"counts":{"person_id":%d,"post_count":2,"comment_count":3},"is_admin":true},"posts":[%s]}`,
		id, name, name, name, id, postJSON)
}

func postViewFixture(id, creatorID, communityID int64, title string) string {
	return fmt.Sprintf(`{"post":{"id":%d,"name":%q,"url":"https://links.example/item","body":"A body","creator_id":%d,"community_id":%d,"published":"2026-08-01T01:02:03Z","updated":"2026-08-02T01:02:03Z","ap_id":"/post/%d","local":true,"language_id":2,"url_content_type":"image/png","alt_text":"Alt"},"creator":{"id":%d,"name":"alice"},"community":{"id":%d,"name":"go","title":"Go","actor_id":"https://lemmy.example/c/go","visibility":"Public"},"image_details":{"link":"/pictrs/image/image.png","width":640,"height":480,"content_type":"image/png"},"counts":{"post_id":%d,"comments":3,"score":7,"upvotes":8,"downvotes":1,"published":"2026-08-01T01:02:03Z"}}`,
		id, title, creatorID, communityID, id, creatorID, communityID, id)
}

func commentViewFixture(id, postID, creatorID int64, path, content string) string {
	return fmt.Sprintf(`{"comment":{"id":%d,"creator_id":%d,"post_id":%d,"content":%q,"published":"2026-08-01T02:03:04Z","path":%q},"creator":{"id":%d,"name":"bob"},"post":{"id":%d,"name":"Post","creator_id":7,"community_id":5},"community":{"id":5,"name":"go"},"counts":{"comment_id":%d,"score":2,"upvotes":3,"downvotes":1,"child_count":4}}`,
		id, creatorID, postID, content, path, creatorID, postID, id)
}

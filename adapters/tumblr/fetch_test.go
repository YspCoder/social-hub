package tumblr

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func testPost(id string, timestamp int64) map[string]any {
	return map[string]any{
		"id_string": id, "blog_name": "example", "tumblelog_uuid": "t:example", "post_url": "https://example.tumblr.com/post/" + id,
		"timestamp": timestamp, "state": "published", "note_count": 7, "parent_post_id": "99",
		"content": []any{
			map[string]any{"type": "text", "text": "first block"},
			map[string]any{"type": "text", "text": "second block"},
			map[string]any{"type": "image", "media": []any{map[string]any{"url": "https://cdn.test/image.jpg", "type": "image/jpeg", "width": 640, "height": 480}}},
			map[string]any{"type": "audio", "media": map[string]any{"url": "https://cdn.test/audio.mp3", "type": "audio/mpeg", "duration": 3.5}},
		},
	}
}

func TestFetchContractsAndMapping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("api_key") != "tumblr-key" || request.Header.Get("Authorization") != "" {
			http.Error(writer, "bad public auth", http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/blog/example.tumblr.com/info":
			writeEnvelope(t, writer, map[string]any{"blog": map[string]any{
				"uuid": "t:example", "name": "example", "title": "Example Blog", "url": "https://example.tumblr.com/",
				"avatar": []any{map[string]any{"url": ""}, map[string]any{"url": "https://cdn.test/avatar.png", "width": 128, "height": 128}},
			}})
		case "/blog/example.tumblr.com/posts":
			query := request.URL.Query()
			if query.Get("npf") != "true" || query.Get("notes_info") != "true" {
				http.Error(writer, "missing post fields", http.StatusBadRequest)
				return
			}
			if query.Get("id") != "" {
				if query.Get("id") != "101" {
					http.Error(writer, "bad id", http.StatusBadRequest)
					return
				}
				writeEnvelope(t, writer, map[string]any{"posts": []any{testPost("101", 1_754_046_000)}, "total_posts": 1})
				return
			}
			if query.Get("limit") != "2" || query.Get("offset") != "2" {
				http.Error(writer, "bad page", http.StatusBadRequest)
				return
			}
			writeEnvelope(t, writer, map[string]any{"posts": []any{testPost("102", 1_754_046_100), testPost("103", 1_754_046_200)}, "total_posts": 5})
		case "/blog/example.tumblr.com/notes":
			query := request.URL.Query()
			if query.Get("id") != "101" || query.Get("mode") != "conversation" {
				http.Error(writer, "bad notes", http.StatusBadRequest)
				return
			}
			writeEnvelope(t, writer, map[string]any{
				"notes": []any{
					map[string]any{"type": "reply", "timestamp": 1754046000.25, "blog_name": "alice", "blog_uuid": "t:alice", "reply_id": "reply-1", "reply_text": "hello"},
					map[string]any{"type": "like", "timestamp": 1754045999, "blog_name": "bob"},
					map[string]any{"type": "reblog", "timestamp": 1754045998, "blog_name": "carol", "post_id": "reblog-1", "added_text": "commentary"},
				},
				"rollup_notes": []any{}, "total_notes": 3,
				"_links": map[string]any{"next": map[string]any{"query_params": map[string]any{"before_timestamp": "1754045998.5"}}},
			})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, false, []string{"basic"})

	user, err := client.GetUser(context.Background(), "")
	if err != nil || user.ID != "t:example" || user.Username == nil || *user.Username != "example" || user.DisplayName == nil || *user.DisplayName != "Example Blog" || user.AvatarURL == nil || *user.AvatarURL != "https://cdn.test/avatar.png" || len(user.Extensions) != 1 {
		t.Fatalf("user=%#v error=%v", user, err)
	}
	post, err := client.GetPost(context.Background(), "101")
	if err != nil || post.ID != "101" || post.AuthorID == nil || *post.AuthorID != "t:example" || post.Text == nil || *post.Text != "first block\n\nsecond block" || len(post.Media) != 2 || post.Media[0].Width == nil || *post.Media[0].Width != 640 || post.Media[1].Duration == nil || *post.Media[1].Duration != 3500*time.Millisecond || len(post.Relations) != 1 || post.Relations[0].PostID != "99" || post.Metrics[0].Value != 7 || !post.Metrics[0].AsOf.Equal(testNow) {
		t.Fatalf("post=%#v error=%v", post, err)
	}
	page, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{Cursor: "2", MaxResults: 2})
	if err != nil || len(page.Items) != 2 || page.NextCursor == nil || *page.NextCursor != "4" || page.PrevCursor == nil || *page.PrevCursor != "0" || !page.HasMore {
		t.Fatalf("posts=%#v error=%v", page, err)
	}
	comments, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "101", MaxResults: 2})
	if err != nil || len(comments.Items) != 2 || comments.Items[0].ID != "reply-1" || comments.Items[0].Text != "hello" || comments.Items[1].ID != "reblog-1" || comments.NextCursor == nil || *comments.NextCursor != "1754045998.5" || !comments.HasMore {
		t.Fatalf("comments=%#v error=%v", comments, err)
	}
}

func TestFetchValidationAndMissingResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/blog/example.tumblr.com/posts":
			writeEnvelope(t, writer, map[string]any{"posts": []any{}, "total_posts": 0})
		case "/blog/example.tumblr.com/info":
			writeEnvelope(t, writer, map[string]any{"blog": map[string]any{}})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, false, nil)
	if _, err := client.GetPost(context.Background(), "bad"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("bad post id=%v", err)
	}
	if _, err := client.GetPost(context.Background(), "123"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing post=%v", err)
	}
	if _, err := client.GetUser(context.Background(), ""); err == nil {
		t.Fatal("incomplete blog accepted")
	}
	start, end := testNow.Add(-time.Hour), testNow
	invalidLists := []socialhub.ListPostsRequest{
		{Cursor: "bad"}, {Cursor: "1", StartTime: &start}, {Cursor: "1", EndTime: &end}, {MaxResults: 21}, {UserID: "bad/blog"},
	}
	for index, input := range invalidLists {
		if _, err := client.ListPosts(context.Background(), input); err == nil {
			t.Fatalf("invalid list %d accepted", index)
		}
	}
	for _, input := range []socialhub.ListCommentsRequest{{}, {PostID: "abc"}, {PostID: "123", MaxResults: -1}} {
		if _, err := client.ListComments(context.Background(), input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("invalid comments=%#v error=%v", input, err)
		}
	}
	if _, err := pageLimit(-1); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("negative page limit=%v", err)
	}
	if value, err := pageLimit(0); err != nil || value != 20 {
		t.Fatalf("default page limit=%d error=%v", value, err)
	}
}

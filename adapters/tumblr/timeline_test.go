package tumblr

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestTimelineAndEngagementContracts(t *testing.T) {
	engagementCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/user/dashboard":
			if request.Header.Get("Authorization") != "Bearer access-token" || request.URL.Query().Get("limit") != "1" || request.URL.Query().Get("offset") != "2" || request.URL.Query().Get("npf") != "true" || request.URL.Query().Get("notes_info") != "true" {
				http.Error(writer, "bad dashboard", http.StatusBadRequest)
				return
			}
			writeEnvelope(t, writer, map[string]any{"posts": []any{testPost("201", 1_754_046_300)}})
		case "/tagged":
			query := request.URL.Query()
			if query.Get("api_key") != "tumblr-key" || request.Header.Get("Authorization") != "" || query.Get("tag") != "Go SDK" || query.Get("before") != "1754046300" || query.Get("limit") != "1" || query.Get("npf") != "" {
				http.Error(writer, "bad tagged", http.StatusBadRequest)
				return
			}
			post := testPost("202", 1_754_046_200)
			post["featured_timestamp"] = int64(1_754_046_250)
			writeEnvelope(t, writer, []any{post})
		case "/blog/example.tumblr.com/notes":
			query := request.URL.Query()
			if query.Get("api_key") != "tumblr-key" || query.Get("id") != "201" || query.Get("mode") != "reblogs_with_tags" || query.Get("before_timestamp") != "1754046200.125" {
				http.Error(writer, "bad notes", http.StatusBadRequest)
				return
			}
			writeEnvelope(t, writer, map[string]any{
				"notes":        []any{map[string]any{"type": "reblog", "timestamp": 1754046199.5, "blog_name": "alice", "blog_uuid": "t:alice", "post_id": 301, "added_text": "added", "tags": []string{"go"}}},
				"rollup_notes": []any{map[string]any{"type": "like", "timestamp": 1754046198, "blog_name": "bob"}},
				"total_notes":  10, "total_likes": 6, "total_reblogs": 4,
				"_links": map[string]any{"next": map[string]any{"query_params": map[string]any{"before_timestamp": 1754046199.5}}},
			})
		case "/user/like", "/user/unlike", "/user/follow", "/user/unfollow":
			if request.Method != http.MethodPost || request.Header.Get("Authorization") != "Bearer access-token" {
				http.Error(writer, "bad engagement auth", http.StatusUnauthorized)
				return
			}
			var body map[string]string
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				http.Error(writer, "bad JSON", http.StatusBadRequest)
				return
			}
			if request.URL.Path == "/user/like" || request.URL.Path == "/user/unlike" {
				if body["id"] != "201" || body["reblog_key"] != "rk-1" {
					http.Error(writer, "bad reaction", http.StatusBadRequest)
					return
				}
			} else if body["url"] != "https://staff.tumblr.com/" {
				http.Error(writer, "bad follow", http.StatusBadRequest)
				return
			}
			engagementCalls++
			writeEnvelope(t, writer, map[string]any{})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, true, []string{"basic", "write"})

	dashboard, err := client.Dashboard(context.Background(), PageRequest{Cursor: "2", MaxResults: 1})
	if err != nil || len(dashboard.Items) != 1 || dashboard.NextCursor == nil || *dashboard.NextCursor != "3" || dashboard.PrevCursor == nil || *dashboard.PrevCursor != "1" || !dashboard.HasMore {
		t.Fatalf("dashboard=%#v error=%v", dashboard, err)
	}
	tagged, err := client.Tagged(context.Background(), TaggedRequest{Tag: " Go SDK ", Cursor: "1754046300", MaxResults: 1})
	if err != nil || len(tagged.Items) != 1 || tagged.NextCursor == nil || *tagged.NextCursor != "1754046250" || !tagged.HasMore {
		t.Fatalf("tagged=%#v error=%v", tagged, err)
	}
	notes, err := client.Notes(context.Background(), NotesRequest{PostID: "201", Mode: NotesReblogsWithTags, Cursor: "1754046200.125"})
	if err != nil || len(notes.Items) != 1 || len(notes.Rollup) != 1 || notes.Items[0].PostID != "301" || notes.Items[0].Timestamp == nil || notes.NextCursor == nil || *notes.NextCursor != "1754046199.5" || notes.TotalNotes != 10 || notes.TotalLikes != 6 || notes.TotalReblogs != 4 {
		t.Fatalf("notes=%#v error=%v", notes, err)
	}
	for _, call := range []func() error{
		func() error { return client.Like(context.Background(), "201", "rk-1") },
		func() error { return client.Unlike(context.Background(), "201", "rk-1") },
		func() error { return client.Follow(context.Background(), " https://staff.tumblr.com/ ") },
		func() error { return client.Unfollow(context.Background(), "https://staff.tumblr.com/") },
	} {
		if err := call(); err != nil {
			t.Fatal(err)
		}
	}
	if engagementCalls != 4 {
		t.Fatalf("engagement calls=%d", engagementCalls)
	}
}

func TestTimelineAndEngagementValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, true, []string{"basic", "write"})
	invalid := []func() error{
		func() error { _, err := client.Dashboard(context.Background(), PageRequest{Cursor: "bad"}); return err },
		func() error {
			_, err := client.Dashboard(context.Background(), PageRequest{MaxResults: 21})
			return err
		},
		func() error { _, err := client.Tagged(context.Background(), TaggedRequest{}); return err },
		func() error {
			_, err := client.Tagged(context.Background(), TaggedRequest{Tag: "go", Cursor: "0"})
			return err
		},
		func() error {
			_, err := client.Tagged(context.Background(), TaggedRequest{Tag: "go", MaxResults: -1})
			return err
		},
		func() error { _, err := client.Notes(context.Background(), NotesRequest{PostID: "bad"}); return err },
		func() error {
			_, err := client.Notes(context.Background(), NotesRequest{PostID: "1", BlogIdentifier: "bad/blog"})
			return err
		},
		func() error {
			_, err := client.Notes(context.Background(), NotesRequest{PostID: "1", Mode: "bad"})
			return err
		},
		func() error {
			_, err := client.Notes(context.Background(), NotesRequest{PostID: "1", Cursor: "NaN"})
			return err
		},
		func() error { return client.Like(context.Background(), "bad", "key") },
		func() error { return client.Unlike(context.Background(), "1", " ") },
		func() error { return client.Follow(context.Background(), "staff.tumblr.com") },
		func() error { return client.Unfollow(context.Background(), "https://user:pass@staff.tumblr.com") },
	}
	for index, call := range invalid {
		if err := call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("validation %d=%v", index, err)
		}
	}

	_, public := newTestAdapter(t, server, false, []string{"basic", "write"})
	if _, err := public.Dashboard(context.Background(), PageRequest{}); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("public dashboard=%v", err)
	}
	if err := public.Like(context.Background(), "1", "key"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("public like=%v", err)
	}
	_, limited := newTestAdapter(t, server, true, []string{"basic"})
	if err := limited.Follow(context.Background(), "https://staff.tumblr.com/"); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("follow scope=%v", err)
	}
	limited.scopes = []string{"write"}
	if _, err := limited.Dashboard(context.Background(), PageRequest{}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("dashboard scope=%v", err)
	}

	page := offsetPage(nil, 0, 20, 0, false)
	if page.HasMore || page.NextCursor != nil || page.PrevCursor != nil {
		t.Fatalf("empty offset page=%#v", page)
	}
	if linkCursor(tumblrLinks{}, "before") != nil || !validNotesMode(NotesConversation) || validNotesMode("bad") {
		t.Fatal("timeline helper mismatch")
	}
}

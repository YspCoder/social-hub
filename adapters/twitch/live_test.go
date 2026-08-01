package twitch

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestLiveWorkflow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer user-token" || request.Header.Get("Client-Id") != "twitch-client" {
			http.Error(writer, "bad auth", http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/streams":
			query := request.URL.Query()
			if query.Get("user_id") != "user-1" || query.Get("language") != "en" || query.Get("after") != "stream-cursor" || query.Get("first") != "100" {
				http.Error(writer, "bad stream query", http.StatusBadRequest)
				return
			}
			writeTestJSON(t, writer, map[string]any{
				"data": []map[string]any{{
					"id": "stream-1", "user_id": "user-1", "user_login": "alice", "user_name": "Alice",
					"game_id": "game-1", "game_name": "Coding", "type": "live", "title": "Go live",
					"tags": []string{"English"}, "viewer_count": 123, "started_at": "2026-08-01T10:00:00Z",
					"language": "en", "thumbnail_url": "https://cdn.test/{width}x{height}.jpg", "is_mature": false,
				}}, "pagination": map[string]string{"cursor": "stream-next"},
			})
		case "/channels":
			writeTestJSON(t, writer, map[string]any{"data": []map[string]any{{
				"broadcaster_id": "user-1", "broadcaster_login": "alice", "broadcaster_name": "Alice",
				"broadcaster_language": "en", "game_id": "game-1", "game_name": "Coding", "title": "Go live",
				"delay": 0, "tags": []string{"Go"}, "content_classification_labels": []string{"DrugsIntoxication"}, "is_branded_content": true,
			}}})
		case "/schedule":
			writeTestJSON(t, writer, map[string]any{
				"data": map[string]any{
					"broadcaster_id": "user-1", "broadcaster_login": "alice", "broadcaster_name": "Alice",
					"segments": []map[string]any{{
						"id": "segment-1", "start_time": "2026-08-02T10:00:00Z", "end_time": "2026-08-02T11:00:00Z",
						"title": "Scheduled Go", "canceled_until": nil, "category": map[string]string{"id": "game-1", "name": "Coding"}, "is_recurring": true,
					}},
					"vacation": map[string]string{"start_time": "2026-08-10T00:00:00Z", "end_time": "2026-08-12T00:00:00Z"},
				}, "pagination": map[string]string{"cursor": "schedule-next"},
			})
		case "/clips":
			if request.Method == http.MethodPost {
				if request.URL.Query().Get("broadcaster_id") != "user-1" || request.URL.Query().Get("has_delay") != "true" {
					http.Error(writer, "bad clip create", http.StatusBadRequest)
					return
				}
				writer.WriteHeader(http.StatusAccepted)
				writeTestJSON(t, writer, map[string]any{"data": []map[string]string{{"id": "clip-new", "edit_url": "https://clips.twitch.tv/clip-new/edit"}}})
				return
			}
			query := request.URL.Query()
			if query.Get("broadcaster_id") != "user-1" || query.Get("started_at") == "" || query.Get("is_featured") != "true" {
				http.Error(writer, "bad clip list", http.StatusBadRequest)
				return
			}
			writeTestJSON(t, writer, map[string]any{
				"data": []map[string]any{{
					"id": "clip-1", "url": "https://clips.twitch.tv/clip-1", "embed_url": "https://clips.twitch.tv/embed?clip=clip-1",
					"broadcaster_id": "user-1", "broadcaster_name": "Alice", "creator_id": "viewer-1", "creator_name": "Viewer",
					"video_id": "video-1", "game_id": "game-1", "language": "en", "title": "Great moment", "view_count": 50,
					"created_at": "2026-08-01T11:00:00Z", "thumbnail_url": "https://cdn.test/clip.jpg", "duration": 12.5,
					"vod_offset": 42, "is_featured": true,
				}}, "pagination": map[string]string{"cursor": "clip-next"},
			})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	client := newTestClient(t, server, []string{"clips:edit", "user:write:chat"})
	streams, err := client.ListStreams(context.Background(), StreamRequest{UserIDs: []string{"user-1"}, Languages: []string{"en"}, Cursor: "stream-cursor", MaxResults: 200})
	if err != nil || len(streams.Items) != 1 || streams.Items[0].ViewerCount != 123 || streams.Items[0].UserLogin != "alice" || streams.NextCursor == nil || *streams.NextCursor != "stream-next" {
		t.Fatalf("streams: %#v %v", streams, err)
	}
	channel, err := client.GetChannel(context.Background(), "user-1")
	if err != nil || channel.BroadcasterLogin != "alice" || !channel.BrandedContent || len(channel.ContentLabels) != 1 {
		t.Fatalf("channel: %#v %v", channel, err)
	}
	schedule, err := client.GetSchedule(context.Background(), "user-1", "", 20)
	if err != nil || len(schedule.Segments) != 1 || schedule.Segments[0].CategoryName != "Coding" || !schedule.Segments[0].Recurring || schedule.Vacation == nil || schedule.NextCursor == nil {
		t.Fatalf("schedule: %#v %v", schedule, err)
	}
	started, ended, featured := testNow.Add(-24*time.Hour), testNow, true
	clips, err := client.ListClips(context.Background(), ClipRequest{BroadcasterID: "user-1", StartedAt: &started, EndedAt: &ended, Featured: &featured, MaxResults: 20})
	if err != nil || len(clips.Items) != 1 || clips.Items[0].Duration != 12500*time.Millisecond || clips.Items[0].VODOffset == nil || *clips.Items[0].VODOffset != 42 || clips.NextCursor == nil {
		t.Fatalf("clips: %#v %v", clips, err)
	}
	created, err := client.CreateClip(context.Background(), "user-1", true)
	if err != nil || created.ID != "clip-new" || created.EditURL == "" {
		t.Fatalf("create clip: %#v %v", created, err)
	}
}

func TestLiveWorkflowValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	client := newTestClient(t, server, []string{"user:write:chat"})
	tooMany := make([]string, 101)
	for index := range tooMany {
		tooMany[index] = "id"
	}
	if _, err := client.ListStreams(context.Background(), StreamRequest{UserIDs: tooMany}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("too many stream filters: %v", err)
	}
	if _, err := client.ListStreams(context.Background(), StreamRequest{Languages: []string{""}}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("empty stream filter: %v", err)
	}
	if _, err := client.GetChannel(context.Background(), " "); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("empty channel: %v", err)
	}
	if _, err := client.GetSchedule(context.Background(), "", "", 0); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("empty schedule: %v", err)
	}
	started, ended := testNow, testNow.Add(-time.Hour)
	clipCases := []ClipRequest{
		{}, {BroadcasterID: "a", GameID: "b"}, {IDs: []string{"id"}, Cursor: "cursor"},
		{BroadcasterID: "a", EndedAt: &testNow}, {BroadcasterID: "a", StartedAt: &started, EndedAt: &ended},
		{IDs: strings.Fields(strings.Repeat("id ", 101))},
	}
	for index, input := range clipCases {
		if _, err := client.ListClips(context.Background(), input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("clip case %d: %v", index, err)
		}
	}
	if _, err := client.CreateClip(context.Background(), " ", false); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("empty create clip: %v", err)
	}
	if _, err := client.CreateClip(context.Background(), "user-1", false); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("missing clip scope: %v", err)
	}
	if err := appendValues(make(url.Values), "id", []string{""}, 1); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("append values: %v", err)
	}
	if err := setPaging(make(url.Values), "", -1); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("negative paging: %v", err)
	}
}

func TestLiveMalformedResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/streams":
			writeTestJSON(t, writer, map[string]any{"data": []map[string]string{{"id": ""}}})
		case "/channels":
			writeTestJSON(t, writer, map[string]any{"data": []any{}})
		case "/schedule":
			writeTestJSON(t, writer, map[string]any{"data": map[string]any{}})
		case "/clips":
			if request.Method == http.MethodPost {
				writeTestJSON(t, writer, map[string]any{"data": []any{}})
			} else {
				writeTestJSON(t, writer, map[string]any{"data": []map[string]string{{"id": ""}}})
			}
		}
	}))
	defer server.Close()
	client := newTestClient(t, server, nil)
	if _, err := client.ListStreams(context.Background(), StreamRequest{}); err == nil {
		t.Fatal("malformed stream accepted")
	}
	if _, err := client.GetChannel(context.Background(), "user"); err == nil {
		t.Fatal("missing channel accepted")
	}
	if _, err := client.GetSchedule(context.Background(), "user", "", 0); err == nil {
		t.Fatal("malformed schedule accepted")
	}
	if _, err := client.ListClips(context.Background(), ClipRequest{BroadcasterID: "user"}); err == nil {
		t.Fatal("malformed clip accepted")
	}
	if _, err := client.CreateClip(context.Background(), "user", false); err == nil {
		t.Fatal("malformed clip creation accepted")
	}
}

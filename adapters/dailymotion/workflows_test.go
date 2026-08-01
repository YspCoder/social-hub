package dailymotion

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"social-hub/pkg/socialhub"
)

const profileResponse = `{"profile_id":"profile-1","name":"channel","display_name":"Channel","description":"Profile description","created_at":"2026-08-01T00:00:00Z","can_change_name":true,"social_links":{"website_url":"https://example.com"},"webhook":{"callback_url":"https://hooks.example.com/dm","events":["video.created"]}}`

const videoResponse = `{"video_id":"video-1","title":"Launch","description":"Launch description","category":"tech","visibility":"public","is_for_kids":false,"is_explicit":false,"created_at":"2026-08-01T00:00:00Z","profile":{"profile_id":"profile-1","name":"channel","created_at":"2026-01-01T00:00:00Z"},"video_url":"https://www.dailymotion.com/video/video-1","updated_at":"2026-08-01T01:00:00Z","uploaded_at":"2026-08-01T00:30:00Z","published_at":"2026-08-01T01:00:00Z","language":"en","country":"US","hashtags":["launch"],"tags":["sdk"],"is_published":true,"processing":{"encoding_status":"encoded","encoding_progress":100,"publishing_progress":100},"is_ai_altered":false,"enable_ai_chapter_generation":false,"source":{"duration":12.5,"width":1920,"height":1080,"checksum":"abc"},"thumbnail":{"h720_url":"https://s1.dmcdn.net/thumb.jpg"}}`

const playlistResponse = `{"playlist_id":"playlist-1","title":"Highlights","description":"Best videos","visibility":"public","created_at":"2026-08-01T00:00:00Z","updated_at":"2026-08-01T01:00:00Z","profile":{"profile_id":"profile-1","name":"channel","created_at":"2026-01-01T00:00:00Z"},"playlist_url":"https://www.dailymotion.com/playlist/playlist-1","embed_url":"https://geo.dailymotion.com/player.html?playlist=playlist-1"}`

func TestProfileAndVideoWorkflows(t *testing.T) {
	var profileUpdates, videoUpdates, videoDeletes atomic.Int32
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer access-token" {
			http.Error(writer, "missing bearer", http.StatusUnauthorized)
			return
		}
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/v2/me":
			writeJSON(writer, http.StatusOK, `{"user_id":"user-1","username":"owner","profiles":[{"profile_id":"profile-1","name":"channel"}]}`)
		case request.Method == http.MethodGet && request.URL.Path == "/v2/profiles/profile-1":
			if !strings.Contains(request.URL.Query().Get("fields"), "webhook") {
				http.Error(writer, "fields", 400)
				return
			}
			writeJSON(writer, http.StatusOK, profileResponse)
		case request.Method == http.MethodPatch && request.URL.Path == "/v2/profiles/profile-1":
			var body struct {
				Webhook *WebhookSettings `json:"webhook"`
			}
			if json.NewDecoder(request.Body).Decode(&body) != nil || body.Webhook == nil || len(body.Webhook.Events) != 1 {
				http.Error(writer, "body", 400)
				return
			}
			profileUpdates.Add(1)
			writer.WriteHeader(http.StatusNoContent)
		case request.Method == http.MethodGet && request.URL.Path == "/v2/videos/video-1":
			writeJSON(writer, http.StatusOK, videoResponse)
		case request.Method == http.MethodGet && request.URL.Path == "/v2/profiles/profile-1/videos":
			query := request.URL.Query()
			if query.Get("page_size") != "100" || query.Has("visibility") && (query.Get("visibility") != "public" || query.Get("is_for_kids") != "false") {
				http.Error(writer, "query", 400)
				return
			}
			next := server.URL + "/v2/profiles/profile-1/videos?page=2&page_size=100"
			writeJSON(writer, http.StatusOK, fmt.Sprintf(`{"data":[%s],"pagination":{"page":1,"page_size":100,"total":2,"next":%q,"previous":null}}`, videoResponse, next))
		case request.Method == http.MethodPost && request.URL.Path == "/v2/profiles/profile-1/videos":
			var body struct {
				Title, Category, Visibility string
				IsForKids                   bool `json:"is_for_kids"`
				Source                      struct {
					FileURL string `json:"file_url"`
				} `json:"source"`
			}
			if json.NewDecoder(request.Body).Decode(&body) != nil || body.Title != "Launch" || body.Category != "tech" || body.Visibility != "public" || body.Source.FileURL != "https://cdn.example.com/video.mp4" {
				http.Error(writer, "body", 400)
				return
			}
			writeJSON(writer, http.StatusCreated, videoResponse)
		case request.Method == http.MethodPatch && request.URL.Path == "/v2/videos/video-1":
			videoUpdates.Add(1)
			writer.WriteHeader(http.StatusNoContent)
		case request.Method == http.MethodDelete && request.URL.Path == "/v2/videos/video-1":
			videoDeletes.Add(1)
			writer.WriteHeader(http.StatusNoContent)
		default:
			http.Error(writer, "unexpected "+request.Method+" "+request.URL.Path, http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server)
	ctx := context.Background()
	account, err := client.CurrentAccount(ctx)
	if err != nil || account.UserID != "user-1" || len(account.Profiles) != 1 {
		t.Fatalf("account=%#v err=%v", account, err)
	}
	user, err := client.GetUser(ctx, "profile-1")
	if err != nil || user.ID != "profile-1" || user.DisplayName == nil || *user.DisplayName != "Channel" {
		t.Fatalf("user=%#v err=%v", user, err)
	}
	callback := "https://hooks.example.com/dm"
	if err := client.UpdateProfile(ctx, "profile-1", UpdateProfileRequest{Webhook: &WebhookSettings{CallbackURL: &callback, Events: []string{"video.published"}}}); err != nil {
		t.Fatal(err)
	}

	video, err := client.GetVideo(ctx, "video-1")
	if err != nil || video.VideoID != "video-1" || video.Processing.EncodingStatus != "encoded" {
		t.Fatalf("video=%#v err=%v", video, err)
	}
	post, err := client.GetPost(ctx, "video-1")
	if err != nil || post.ID != "video-1" || post.Status.State != socialhub.PublishStatePublished || post.Media[0].State != socialhub.MediaStateReady || post.Media[0].Duration == nil {
		t.Fatalf("post=%#v err=%v", post, err)
	}
	page, err := client.ListVideos(ctx, VideoListRequest{MaxResults: 200, Visibility: "public", IsForKids: boolPointer(false), Sort: "-created_at"})
	if err != nil || len(page.Items) != 1 || page.NextCursor == nil || *page.NextCursor != "2" || !page.HasMore {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	commonPage, err := client.ListPosts(ctx, socialhub.ListPostsRequest{UserID: "profile-1", MaxResults: 200})
	if err != nil || len(commonPage.Items) != 1 {
		t.Fatalf("posts=%#v err=%v", commonPage, err)
	}
	created, err := client.CreateVideo(ctx, CreateVideoRequest{Title: "Launch", Category: "tech", Visibility: "public", SourceURL: "https://cdn.example.com/video.mp4", Language: "en", Country: "US"})
	if err != nil || created.VideoID != "video-1" {
		t.Fatalf("created=%#v err=%v", created, err)
	}
	title := "Updated"
	if err := client.UpdateVideo(ctx, "video-1", UpdateVideoRequest{Title: &title}); err != nil {
		t.Fatal(err)
	}
	if err := client.DeleteVideo(ctx, "video-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ListComments(ctx, socialhub.ListCommentsRequest{PostID: "video-1"}); errorCode(err) != socialhub.CodeUnsupported {
		t.Fatalf("comments=%v", err)
	}
	if profileUpdates.Load() != 1 || videoUpdates.Load() != 1 || videoDeletes.Load() != 1 {
		t.Fatalf("updates=%d/%d/%d", profileUpdates.Load(), videoUpdates.Load(), videoDeletes.Load())
	}
}

func TestPlaylistWorkflow(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer access-token" {
			http.Error(writer, "auth", 401)
			return
		}
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/v2/playlists/playlist-1":
			writeJSON(writer, 200, playlistResponse)
		case request.Method == http.MethodGet && request.URL.Path == "/v2/profiles/profile-1/playlists":
			previous := server.URL + "/v2/profiles/profile-1/playlists?page=1"
			writeJSON(writer, 200, fmt.Sprintf(`{"data":[%s],"pagination":{"page":2,"page_size":20,"total":1,"next":null,"previous":%q}}`, playlistResponse, previous))
		case request.Method == http.MethodPost && request.URL.Path == "/v2/profiles/profile-1/playlists":
			writeJSON(writer, 201, playlistResponse)
		case request.Method == http.MethodPatch && request.URL.Path == "/v2/playlists/playlist-1":
			writer.WriteHeader(204)
		case request.Method == http.MethodDelete && request.URL.Path == "/v2/playlists/playlist-1":
			writer.WriteHeader(204)
		case request.Method == http.MethodGet && request.URL.Path == "/v2/playlists/playlist-1/videos":
			writeJSON(writer, 200, `{"data":[{"title":"Launch","description":"Video","created_at":"2026-08-01T00:00:00Z"}],"pagination":{"page":1,"page_size":20,"total":1,"next":null,"previous":null}}`)
		case request.Method == http.MethodPost && request.URL.Path == "/v2/playlists/playlist-1/videos":
			var body map[string]string
			_ = json.NewDecoder(request.Body).Decode(&body)
			if body["video_id"] != "video-1" {
				http.Error(writer, "body", 400)
				return
			}
			writeJSON(writer, 201, `{"title":"Launch","description":"Video","created_at":"2026-08-01T00:00:00Z"}`)
		case request.Method == http.MethodDelete && request.URL.Path == "/v2/playlists/playlist-1/videos/video-1":
			writer.WriteHeader(204)
		default:
			http.Error(writer, "unexpected", 404)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server)
	ctx := context.Background()
	playlist, err := client.GetPlaylist(ctx, "playlist-1")
	if err != nil || playlist.Title != "Highlights" {
		t.Fatalf("playlist=%#v err=%v", playlist, err)
	}
	page, err := client.ListPlaylists(ctx, PlaylistListRequest{Cursor: "2", Sort: "-created_at"})
	if err != nil || page.PrevCursor == nil || *page.PrevCursor != "1" {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	created, err := client.CreatePlaylist(ctx, CreatePlaylistRequest{Title: "Highlights", Visibility: "public"})
	if err != nil || created.PlaylistID != "playlist-1" {
		t.Fatalf("created=%#v err=%v", created, err)
	}
	title := "New"
	if err := client.UpdatePlaylist(ctx, "playlist-1", UpdatePlaylistRequest{Title: &title}); err != nil {
		t.Fatal(err)
	}
	videos, err := client.ListPlaylistVideos(ctx, PlaylistVideosRequest{PlaylistID: "playlist-1"})
	if err != nil || len(videos.Items) != 1 || videos.Items[0].Title != "Launch" {
		t.Fatalf("videos=%#v err=%v", videos, err)
	}
	item, err := client.AddPlaylistVideo(ctx, "playlist-1", "video-1", "")
	if err != nil || item.Title != "Launch" {
		t.Fatalf("item=%#v err=%v", item, err)
	}
	if err := client.RemovePlaylistVideo(ctx, "playlist-1", "video-1"); err != nil {
		t.Fatal(err)
	}
	if err := client.DeletePlaylist(ctx, "playlist-1"); err != nil {
		t.Fatal(err)
	}
}

func TestWorkflowValidationAndPaginationSafety(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server)
	ctx := context.Background()
	invalidCalls := []func() error{
		func() error { _, e := client.GetProfile(ctx, "bad/id"); return e },
		func() error { return client.UpdateProfile(ctx, "profile-1", UpdateProfileRequest{}) },
		func() error { _, e := client.GetVideo(ctx, ""); return e },
		func() error { _, e := client.ListVideos(ctx, VideoListRequest{Cursor: "zero"}); return e },
		func() error { _, e := client.CreateVideo(ctx, CreateVideoRequest{}); return e },
		func() error { return client.UpdateVideo(ctx, "video-1", UpdateVideoRequest{}) },
		func() error { return client.DeleteVideo(ctx, "bad/id") },
		func() error { _, e := client.CreatePlaylist(ctx, CreatePlaylistRequest{}); return e },
		func() error { return client.UpdatePlaylist(ctx, "playlist-1", UpdatePlaylistRequest{}) },
		func() error { _, e := client.ListPlaylistVideos(ctx, PlaylistVideosRequest{PlaylistID: ""}); return e },
		func() error { _, e := client.AddPlaylistVideo(ctx, "playlist-1", "", ""); return e },
	}
	for i, call := range invalidCalls {
		if code := errorCode(call()); code != socialhub.CodeInvalidArgument {
			t.Fatalf("call %d code=%s", i, code)
		}
	}
	evil := "https://evil.example/v2/videos?page=2"
	if _, err := pageCursor(&evil, client.apiBaseURL); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("evil cursor=%v", err)
	}
	bad := "not a url %"
	if _, err := pageCursor(&bad, client.apiBaseURL); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("bad cursor=%v", err)
	}
	missingPage := "/v2/videos"
	if _, err := pageCursor(&missingPage, client.apiBaseURL); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("missing page=%v", err)
	}
	if !validRemoteURL("http://example.com/video.mp4") || validRemoteURL("file:///tmp/video") || !validHTTPSURL("https://example.com/hook") || validHTTPSURL("http://example.com/hook") {
		t.Fatal("URL validation")
	}
}

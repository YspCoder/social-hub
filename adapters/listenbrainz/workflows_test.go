package listenbrainz

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"social-hub/pkg/socialhub"
)

const (
	recordingMBID   = "30d08f4c-d825-4ae1-b79c-44242cddd7c0"
	recordingMSID   = "fd0cad33-ea33-453b-8155-1292379277db"
	playlistMBID    = "8357d30f-beac-4639-bdf3-b969d3a5c424"
	listenFixture   = `{"inserted_at":1771414107,"listened_at":1771414109,"recording_msid":"fd0cad33-ea33-453b-8155-1292379277db","track_metadata":{"additional_info":{"artist_mbids":["1c70a3fc-fa3c-4be1-8b55-c3192db8a884"],"duration_ms":402390,"recording_mbid":"30d08f4c-d825-4ae1-b79c-44242cddd7c0","tracknumber":10},"artist_name":"Royksopp","mbid_mapping":{"artist_mbids":["1c70a3fc-fa3c-4be1-8b55-c3192db8a884"],"artists":[{"artist_credit_name":"Royksopp","artist_mbid":"1c70a3fc-fa3c-4be1-8b55-c3192db8a884","join_phrase":""}],"caa_id":32916450708,"caa_release_mbid":"f1418001-7f1e-46af-bfdb-95faeded8841","recording_mbid":"30d08f4c-d825-4ae1-b79c-44242cddd7c0","recording_name":"Some Resolve","release_mbid":"e96c2e7b-94fd-4fbe-9c44-1a27d8664825","url_rels":[{"type":"streaming","url":"https://example.test/track"}]},"release_name":"Profound Mysteries II","track_name":"Some Resolve"},"user_name":"rob"}`
	playlistFixture = `{"playlist":{"annotation":"Daily jams","creator":"rob","date":"2025-09-19T12:26:32.820489+00:00","extension":{"https://musicbrainz.org/doc/jspf#playlist":{"creator":"rob","public":true}},"identifier":"https://listenbrainz.org/playlist/8357d30f-beac-4639-bdf3-b969d3a5c424","title":"Daily Jams","track":[{"extension":{"https://musicbrainz.org/doc/jspf#track":{"added_by":"troi-bot"}},"identifier":["https://musicbrainz.org/recording/30d08f4c-d825-4ae1-b79c-44242cddd7c0"]}]}}`
)

func TestTypedWorkflowsAndAuthentication(t *testing.T) {
	var playingCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		path := request.URL.Path
		writeRequiresToken := request.Method == http.MethodPost || path == "/api/1/validate-token"
		if writeRequiresToken && request.Header.Get("Authorization") != "Token test-token" {
			http.Error(writer, "missing token", http.StatusUnauthorized)
			return
		}
		switch path {
		case "/api/1/search/users/":
			if request.Method != http.MethodGet || request.URL.Query().Get("search_term") != "rob" ||
				request.Header.Get("Authorization") != "" || request.Header.Get("X-Request-ID") != "request-1" {
				http.Error(writer, "bad user search", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"users":[{"user_name":"rob"}]}`)
		case "/api/1/user/rob/listens":
			if request.URL.Query().Get("count") != "1" || request.URL.Query().Get("max_ts") != "1771414200" {
				http.Error(writer, "bad listens query", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"payload":{"count":1,"latest_listen_ts":1771414109,"listens":[`+listenFixture+`],"oldest_listen_ts":1160599019,"user_id":"rob"}}`)
		case "/api/1/user/rob/playing-now":
			if playingCalls.Add(1) == 1 {
				writeJSON(writer, http.StatusOK, `{"payload":{"count":0,"listens":[],"playing_now":true,"user_id":"rob"}}`)
				return
			}
			playing := strings.Replace(listenFixture, `"listened_at":1771414109,`, "", 1)
			writeJSON(writer, http.StatusOK, `{"payload":{"count":1,"listens":[`+playing+`],"playing_now":true,"user_id":"rob"}}`)
		case "/api/1/user/rob/listen-count":
			writeJSON(writer, http.StatusOK, `{"payload":{"count":220287}}`)
		case "/api/1/validate-token":
			writeJSON(writer, http.StatusOK, `{"code":200,"message":"Token valid.","valid":true,"user_name":"private-user"}`)
		case "/api/1/submit-listens":
			body, _ := io.ReadAll(request.Body)
			var kind struct {
				ListenType string            `json:"listen_type"`
				Payload    []json.RawMessage `json:"payload"`
			}
			if json.Unmarshal(body, &kind) != nil || len(kind.Payload) == 0 || request.Header.Get("Content-Type") != "application/json" {
				http.Error(writer, "bad submission", http.StatusBadRequest)
				return
			}
			switch kind.ListenType {
			case "single":
				if len(kind.Payload) != 1 {
					http.Error(writer, "bad single", http.StatusBadRequest)
					return
				}
			case "import":
				if len(kind.Payload) != 2 {
					http.Error(writer, "bad import", http.StatusBadRequest)
					return
				}
			case "playing_now":
				if len(kind.Payload) != 1 || request.URL.Query().Get("return_msid") != "true" || strings.Contains(string(kind.Payload[0]), "listened_at") {
					http.Error(writer, "bad playing now", http.StatusBadRequest)
					return
				}
			default:
				http.Error(writer, "bad listen type", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"status":"ok","payload":{"recording_msid":"`+recordingMSID+`"}}`)
		case "/api/1/delete-listen":
			var input DeleteListenRequest
			if json.NewDecoder(request.Body).Decode(&input) != nil || input.ListenedAt != 1771414109 || input.RecordingMSID != recordingMSID {
				http.Error(writer, "bad delete", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"status":"ok"}`)
		case "/api/1/feedback/recording-feedback":
			var input FeedbackSubmission
			if json.NewDecoder(request.Body).Decode(&input) != nil || input.RecordingMBID != recordingMBID || input.Score != FeedbackLove {
				http.Error(writer, "bad feedback", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"status":"ok"}`)
		case "/api/1/feedback/user/rob/get-feedback":
			if request.URL.Query().Get("score") != "-1" || request.URL.Query().Get("count") != "1" || request.URL.Query().Get("metadata") != "true" {
				http.Error(writer, "bad feedback query", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"count":1,"feedback":[{"created":1749814604,"recording_mbid":"`+recordingMBID+`","recording_msid":null,"score":-1,"track_metadata":{"artist_name":"Ratatat","track_name":"Wildcat"},"user_id":"rob"}],"offset":0,"total_count":2}`)
		case "/api/1/playlist/search":
			if request.URL.Query().Get("query") != "Daily" || request.URL.Query().Get("count") != "1" {
				http.Error(writer, "bad playlist search", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"count":1,"offset":0,"playlist_count":2,"playlists":[`+playlistFixture+`]}`)
		case "/api/1/user/rob/playlists":
			writeJSON(writer, http.StatusOK, `{"count":1,"offset":0,"playlist_count":1,"playlists":[`+playlistFixture+`]}`)
		case "/api/1/playlist/" + playlistMBID:
			if request.URL.Query().Get("fetch_metadata") != "false" {
				http.Error(writer, "bad metadata option", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, playlistFixture)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, public, private := newTestClients(t, server)
	ctx := context.Background()

	users, err := public.SearchUsers(ctx, "rob", socialhub.WithRequestID("request-1"))
	if err != nil || len(users) != 1 || users[0].UserName != "rob" {
		t.Fatalf("users=%#v err=%v", users, err)
	}
	listens, err := public.ListListens(ctx, ListensRequest{MaxTimestamp: 1771414200, Count: 1})
	if err != nil || listens.Count != 1 || listens.Listens[0].TrackMetadata.MBIDMapping == nil ||
		listens.Listens[0].TrackMetadata.AdditionalInfo == nil || len(listens.Listens[0].TrackMetadata.MBIDMapping.URLRelations) != 1 {
		t.Fatalf("listens=%#v err=%v", listens, err)
	}
	if playing, err := public.GetPlayingNow(ctx, ""); err != nil || playing != nil {
		t.Fatalf("empty playing=%#v err=%v", playing, err)
	}
	if playing, err := public.GetPlayingNow(ctx, "rob"); err != nil || playing == nil || playing.ListenedAt != nil {
		t.Fatalf("playing=%#v err=%v", playing, err)
	}
	if count, err := public.GetListenCount(ctx, ""); err != nil || count != 220287 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	validation, err := private.ValidateToken(ctx)
	if err != nil || !validation.Valid || validation.UserName != "private-user" {
		t.Fatalf("validation=%#v err=%v", validation, err)
	}

	submission := ListenSubmission{ListenedAt: 1771414109, TrackMetadata: testTrackMetadata()}
	if result, err := private.SubmitSingle(ctx, submission); err != nil || result.Status != "ok" {
		t.Fatalf("single=%#v err=%v", result, err)
	}
	if result, err := private.SubmitImport(ctx, []ListenSubmission{submission, submission}); err != nil || result.Status != "ok" {
		t.Fatalf("import=%#v err=%v", result, err)
	}
	playingSubmission := PlayingNowSubmission{TrackMetadata: testTrackMetadata()}
	if result, err := private.SubmitPlayingNow(ctx, playingSubmission, true); err != nil || result.Payload == nil || result.Payload.RecordingMSID != recordingMSID {
		t.Fatalf("playing submission=%#v err=%v", result, err)
	}
	if err := private.DeleteListen(ctx, DeleteListenRequest{ListenedAt: 1771414109, RecordingMSID: recordingMSID}); err != nil {
		t.Fatal(err)
	}
	if err := private.SubmitFeedback(ctx, FeedbackSubmission{RecordingMBID: recordingMBID, Score: FeedbackLove}); err != nil {
		t.Fatal(err)
	}
	score := FeedbackHate
	feedback, err := public.ListFeedback(ctx, FeedbackListRequest{Score: &score, Metadata: true, MaxResults: 1})
	if err != nil || len(feedback.Items) != 1 || feedback.Items[0].RecordingMSID != nil || !feedback.HasMore || feedback.NextCursor == nil {
		t.Fatalf("feedback=%#v err=%v", feedback, err)
	}
	playlists, err := public.SearchPlaylists(ctx, PlaylistSearchRequest{Query: "Daily", MaxResults: 1})
	if err != nil || len(playlists.Items) != 1 || playlists.Items[0].Date == nil || !playlists.HasMore {
		t.Fatalf("search playlists=%#v err=%v", playlists, err)
	}
	playlists, err = public.ListUserPlaylists(ctx, "", PlaylistPageRequest{})
	if err != nil || len(playlists.Items) != 1 || playlists.HasMore {
		t.Fatalf("user playlists=%#v err=%v", playlists, err)
	}
	playlist, err := public.GetPlaylist(ctx, playlistMBID, false)
	if err != nil || playlist.Title != "Daily Jams" || len(playlist.Track) != 1 || len(playlist.Extension) != 1 {
		t.Fatalf("playlist=%#v err=%v", playlist, err)
	}
}

func testTrackMetadata() SubmissionTrackMetadata {
	return SubmissionTrackMetadata{
		ArtistName: "Royksopp", TrackName: "Some Resolve", ReleaseName: "Profound Mysteries II",
		AdditionalInfo: &SubmissionAdditionalInfo{
			ArtistMBIDs:   []string{"1c70a3fc-fa3c-4be1-8b55-c3192db8a884"},
			RecordingMBID: recordingMBID, DurationMS: 402390, Tags: []string{"electronic"},
			OriginURL: "https://example.test/track", SubmissionClient: "social-hub-tests",
		},
	}
}

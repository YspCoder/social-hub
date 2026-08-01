package spotify

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestPlaybackWorkflow(t *testing.T) {
	var mu sync.Mutex
	stateCalls := 0
	playCalls := 0
	seen := make(map[string]int)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		mu.Lock()
		seen[request.Method+" "+request.URL.Path]++
		mu.Unlock()
		switch request.Method + " " + request.URL.Path {
		case "GET /v1/me/player":
			stateCalls++
			if stateCalls == 2 {
				writer.WriteHeader(http.StatusNoContent)
				return
			}
			if request.URL.Query().Get("additional_types") != "track,episode" || request.URL.Query().Get("market") != "US" {
				t.Errorf("playback query=%v", request.URL.Query())
			}
			writeJSON(writer, `{"device":{"id":"device-1","is_active":true,"name":"Office","type":"computer","volume_percent":55,"supports_volume":true},"repeat_state":"context","shuffle_state":true,"timestamp":1785632523000,"progress_ms":1200,"is_playing":true,"currently_playing_type":"track","item":`+trackJSON(testTrackID)+`,"context":{"type":"playlist","uri":"spotify:playlist:`+testPlaylistID+`"}}`)
		case "GET /v1/me/player/devices":
			writeJSON(writer, `{"devices":[{"id":"device-1","is_active":true,"name":"Office","type":"computer","volume_percent":55,"supports_volume":true}]}`)
		case "GET /v1/me/player/queue":
			writeJSON(writer, `{"currently_playing":`+trackJSON(testTrackID)+`,"queue":[`+episodeJSON()+`,{"type":"audiobook","id":"future"}]}`)
		case "PUT /v1/me/player":
			var body struct {
				DeviceIDs []string `json:"device_ids"`
				Play      bool     `json:"play"`
			}
			_ = json.NewDecoder(request.Body).Decode(&body)
			if len(body.DeviceIDs) != 1 || body.DeviceIDs[0] != "device-1" || !body.Play {
				t.Errorf("transfer body=%#v", body)
			}
			writer.WriteHeader(http.StatusNoContent)
		case "PUT /v1/me/player/play":
			playCalls++
			var body map[string]json.RawMessage
			_ = json.NewDecoder(request.Body).Decode(&body)
			if request.URL.Query().Get("device_id") != "device-1" {
				t.Errorf("play query=%v body=%v", request.URL.Query(), body)
			}
			if playCalls == 1 && (len(body["context_uri"]) == 0 || len(body["offset"]) == 0 || len(body["position_ms"]) == 0) {
				t.Errorf("context play body=%v", body)
			}
			if playCalls == 2 && len(body["uris"]) == 0 {
				t.Errorf("URI play body=%v", body)
			}
			writer.WriteHeader(http.StatusNoContent)
		case "PUT /v1/me/player/pause", "POST /v1/me/player/next", "POST /v1/me/player/previous":
			if request.URL.Query().Get("device_id") != "device-1" {
				t.Errorf("device query=%v", request.URL.Query())
			}
			writer.WriteHeader(http.StatusNoContent)
		case "PUT /v1/me/player/seek":
			if request.URL.Query().Get("position_ms") != "2500" {
				t.Errorf("seek query=%v", request.URL.Query())
			}
			writer.WriteHeader(http.StatusNoContent)
		case "PUT /v1/me/player/repeat":
			if request.URL.Query().Get("state") != "track" {
				t.Errorf("repeat query=%v", request.URL.Query())
			}
			writer.WriteHeader(http.StatusNoContent)
		case "PUT /v1/me/player/volume":
			if request.URL.Query().Get("volume_percent") != "75" {
				t.Errorf("volume query=%v", request.URL.Query())
			}
			writer.WriteHeader(http.StatusNoContent)
		case "PUT /v1/me/player/shuffle":
			if request.URL.Query().Get("state") != "true" {
				t.Errorf("shuffle query=%v", request.URL.Query())
			}
			writer.WriteHeader(http.StatusNoContent)
		case "POST /v1/me/player/queue":
			if request.URL.Query().Get("uri") != "spotify:episode:"+testEpisodeID {
				t.Errorf("queue query=%v", request.URL.Query())
			}
			writer.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request %s %s", request.Method, request.URL.String())
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, "premium", allTestScopes)

	state, err := client.GetPlaybackState(context.Background(), "US")
	if err != nil || state == nil || state.Device == nil || state.Item.Track == nil || state.Progress == nil || *state.Progress != 1200*time.Millisecond || state.ContextURI != "spotify:playlist:"+testPlaylistID {
		t.Fatalf("state=%#v err=%v", state, err)
	}
	empty, err := client.GetPlaybackState(context.Background(), "")
	if err != nil || empty != nil {
		t.Fatalf("empty state=%#v err=%v", empty, err)
	}
	devices, err := client.ListDevices(context.Background())
	if err != nil || len(devices) != 1 || devices[0].ID != "device-1" {
		t.Fatalf("devices=%#v err=%v", devices, err)
	}
	queue, err := client.GetQueue(context.Background())
	if err != nil || queue.CurrentlyPlaying.Track == nil || len(queue.Items) != 2 || queue.Items[0].Episode == nil || queue.Items[1].Type != "audiobook" || len(queue.Items[1].Raw) == 0 {
		t.Fatalf("queue=%#v err=%v", queue, err)
	}
	if err := client.TransferPlayback(context.Background(), TransferPlaybackRequest{DeviceID: "device-1", Play: true}); err != nil {
		t.Fatal(err)
	}
	offset := 1
	position := 3 * time.Second
	if err := client.StartPlayback(context.Background(), StartPlaybackRequest{
		DeviceID: "device-1", ContextURI: "spotify:playlist:" + testPlaylistID,
		Offset: &PlaybackOffset{Position: &offset}, Position: &position,
	}); err != nil {
		t.Fatal(err)
	}
	if err := client.StartPlayback(context.Background(), StartPlaybackRequest{
		DeviceID: "device-1", URIs: []string{"spotify:track:" + testTrackID, "spotify:track:" + testTrackID},
	}); err != nil {
		t.Fatal(err)
	}
	controls := []func() error{
		func() error { return client.PausePlayback(context.Background(), "device-1") },
		func() error { return client.SkipNext(context.Background(), "device-1") },
		func() error { return client.SkipPrevious(context.Background(), "device-1") },
		func() error { return client.Seek(context.Background(), 2500*time.Millisecond, "device-1") },
		func() error { return client.SetRepeat(context.Background(), "track", "device-1") },
		func() error { return client.SetVolume(context.Background(), 75, "device-1") },
		func() error { return client.SetShuffle(context.Background(), true, "device-1") },
		func() error {
			return client.AddToQueue(context.Background(), "spotify:episode:"+testEpisodeID, "device-1")
		},
	}
	for index, call := range controls {
		if err := call(); err != nil {
			t.Fatalf("control %d error=%v", index, err)
		}
	}
	if seen["POST /v1/me/player/next"] != 1 || seen["PUT /v1/me/player/volume"] != 1 {
		t.Fatalf("seen=%v", seen)
	}
}

func TestPlaybackValidationAndGates(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, "premium", allTestScopes)
	negative := -time.Second
	position := -1
	invalidCalls := []func() error{
		func() error { _, err := client.GetPlaybackState(context.Background(), "us"); return err },
		func() error { return client.TransferPlayback(context.Background(), TransferPlaybackRequest{}) },
		func() error {
			return client.StartPlayback(context.Background(), StartPlaybackRequest{ContextURI: "spotify:playlist:" + testPlaylistID, URIs: []string{"spotify:track:" + testTrackID}})
		},
		func() error {
			return client.StartPlayback(context.Background(), StartPlaybackRequest{Offset: &PlaybackOffset{Position: &position}})
		},
		func() error {
			return client.StartPlayback(context.Background(), StartPlaybackRequest{ContextURI: "spotify:artist:" + testArtistID, Offset: &PlaybackOffset{Position: &position}})
		},
		func() error {
			return client.StartPlayback(context.Background(), StartPlaybackRequest{URIs: []string{"spotify:episode:" + testEpisodeID}})
		},
		func() error {
			return client.StartPlayback(context.Background(), StartPlaybackRequest{Position: &negative})
		},
		func() error { return client.PausePlayback(context.Background(), "bad\n") },
		func() error { return client.Seek(context.Background(), -time.Second, "") },
		func() error { return client.SetRepeat(context.Background(), "all", "") },
		func() error { return client.SetVolume(context.Background(), 101, "") },
		func() error { return client.AddToQueue(context.Background(), "spotify:album:"+testAlbumID, "") },
	}
	for index, call := range invalidCalls {
		if err := call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("invalid call %d error=%v", index, err)
		}
	}
	limited := *client
	limited.scopes = []string{ScopeUserReadPrivate}
	if _, err := limited.GetPlaybackState(context.Background(), ""); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("playback read scope error=%v", err)
	}
	if _, err := limited.GetQueue(context.Background()); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("queue scope error=%v", err)
	}
	limited.scopes = []string{ScopeUserModifyPlaybackState}
	limited.accountType = "free"
	if err := limited.SetShuffle(context.Background(), true, ""); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("Premium gate error=%v", err)
	}
}

package spotify

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestProfileCatalogAndLibraryWorkflows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer access-token" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.Method + " " + request.URL.Path {
		case "GET /v1/me":
			if request.Header.Get("X-Request-ID") != "profile-request" {
				t.Errorf("request ID=%q", request.Header.Get("X-Request-ID"))
			}
			writeJSON(writer, `{"account_id":"AccountStable123","id":"legacy-user","display_name":"Listener","uri":"spotify:user:legacy-user","href":"https://api.spotify.test/v1/users/legacy-user","product":"premium","country":"US","email":"listener@example.com","images":[{"url":"https://cdn.example/avatar.jpg"}],"external_urls":{"spotify":"https://open.spotify.com/user/legacy-user"},"followers":{"total":42}}`)
		case "GET /v1/tracks/" + testTrackID:
			if request.URL.Query().Get("market") != "US" {
				t.Errorf("track query=%v", request.URL.Query())
			}
			writeJSON(writer, trackJSON(testTrackID))
		case "GET /v1/search":
			query := request.URL.Query()
			if query.Get("q") != "ambient" || query.Get("type") != "track" || query.Get("limit") != "10" || query.Get("offset") != "5" || query.Get("market") != "GB" {
				t.Errorf("search query=%v", query)
			}
			writeJSON(writer, `{"tracks":{"items":[`+trackJSON(testTrackID)+`],"limit":10,"offset":5,"total":30,"next":"`+serverURL(request)+`/v1/search?offset=15"}}`)
		case "GET /v1/me/tracks":
			if request.URL.Query().Get("limit") != "25" || request.URL.Query().Get("market") != "CA" {
				t.Errorf("saved query=%v", request.URL.Query())
			}
			writeJSON(writer, `{"items":[{"added_at":"2026-08-01T00:00:00Z","track":`+trackJSON(testTrackID)+`}],"limit":25,"offset":0,"total":2,"next":"`+serverURL(request)+`/v1/me/tracks?offset=25","previous":"`+serverURL(request)+`/v1/me/tracks?offset=0"}`)
		case "PUT /v1/me/library":
			if got := request.URL.Query().Get("uris"); !strings.Contains(got, "spotify:track:") || !strings.Contains(got, "spotify:user:") || !strings.Contains(got, "spotify:playlist:") {
				t.Errorf("save uris=%q", got)
			}
			writer.WriteHeader(http.StatusOK)
		case "DELETE /v1/me/library":
			if request.URL.Query().Get("uris") != "spotify:track:"+testTrackID {
				t.Errorf("remove query=%v", request.URL.Query())
			}
			writer.WriteHeader(http.StatusOK)
		case "GET /v1/me/library/contains":
			writeJSON(writer, `[true,false]`)
		default:
			t.Errorf("unexpected request %s %s", request.Method, request.URL.String())
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, "premium", allTestScopes)

	user, err := client.CurrentUser(context.Background(), socialhub.WithRequestID("profile-request"))
	if err != nil || user.ID != testAccountID || user.Username == nil || *user.Username != "legacy-user" || user.AccountType == nil || *user.AccountType != "premium" {
		t.Fatalf("user=%#v err=%v", user, err)
	}
	track, err := client.GetTrack(context.Background(), testTrackID, "US")
	if err != nil || track.ID != testTrackID || track.Duration != 61*time.Second || track.Album.ID != testAlbumID || len(track.Artists) != 1 {
		t.Fatalf("track=%#v err=%v", track, err)
	}
	search, err := client.SearchTracks(context.Background(), SearchTracksRequest{Query: " ambient ", Market: "GB", Cursor: "5", MaxResults: 99})
	if err != nil || len(search.Items) != 1 || search.NextCursor == nil || *search.NextCursor != "15" {
		t.Fatalf("search=%#v err=%v", search, err)
	}
	saved, err := client.ListSavedTracks(context.Background(), SavedTracksRequest{Market: "CA", MaxResults: 25})
	if err != nil || len(saved.Items) != 1 || saved.Items[0].AddedAt == nil || saved.PrevCursor == nil || *saved.PrevCursor != "0" {
		t.Fatalf("saved=%#v err=%v", saved, err)
	}
	if err := client.SaveItems(context.Background(), []string{
		"spotify:track:" + testTrackID, "spotify:user:" + testAccountID, "spotify:playlist:" + testPlaylistID,
	}); err != nil {
		t.Fatal(err)
	}
	if err := client.RemoveItems(context.Background(), []string{"spotify:track:" + testTrackID}); err != nil {
		t.Fatal(err)
	}
	contains, err := client.ContainsItems(context.Background(), []string{"spotify:track:" + testTrackID, "spotify:artist:" + testArtistID})
	if err != nil || len(contains) != 2 || !contains[0] || contains[1] {
		t.Fatalf("contains=%v err=%v", contains, err)
	}
}

func TestCatalogLibraryValidationAndMappingFailures(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, "premium", allTestScopes)
	invalidCalls := []func() error{
		func() error { _, err := client.GetTrack(context.Background(), "bad-id", ""); return err },
		func() error { _, err := client.GetTrack(context.Background(), testTrackID, "us"); return err },
		func() error { _, err := client.SearchTracks(context.Background(), SearchTracksRequest{}); return err },
		func() error {
			_, err := client.SearchTracks(context.Background(), SearchTracksRequest{Query: "x", Cursor: "1001"})
			return err
		},
		func() error {
			_, err := client.SearchTracks(context.Background(), SearchTracksRequest{Query: "x", MaxResults: -1})
			return err
		},
		func() error {
			_, err := client.ListSavedTracks(context.Background(), SavedTracksRequest{Market: "usa"})
			return err
		},
		func() error { return client.SaveItems(context.Background(), nil) },
		func() error {
			return client.SaveItems(context.Background(), []string{"spotify:artist:" + testArtistID})
		},
		func() error {
			_, err := client.ContainsItems(context.Background(), []string{"spotify:invalid:" + testTrackID})
			return err
		},
	}
	for index, call := range invalidCalls {
		if err := call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("invalid call %d error=%v", index, err)
		}
	}
	limited := *client
	limited.scopes = []string{ScopeUserReadPrivate}
	if _, err := limited.ListSavedTracks(context.Background(), SavedTracksRequest{}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("saved tracks scope error=%v", err)
	}
	if err := limited.SaveItems(context.Background(), []string{"spotify:track:" + testTrackID}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("save scope error=%v", err)
	}
	if _, err := pageCursor("https://evil.example/v1/search?offset=1", client.apiBaseURL); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("cross-origin cursor error=%v", err)
	}
	if _, err := pageCursor("not a url %", client.apiBaseURL); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("invalid cursor error=%v", err)
	}
	if _, err := client.mapUser(spotifyPrivateUser{AccountID: "DifferentAccount"}); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("account mismatch error=%v", err)
	}
	unknown, err := mapPlayable([]byte(`{"type":"audiobook","id":"future"}`))
	if err != nil || unknown.Type != "audiobook" || len(unknown.Raw) == 0 {
		t.Fatalf("unknown playable=%#v err=%v", unknown, err)
	}
	local, err := mapPlayable([]byte(`{"type":"track","is_local":true,"uri":"spotify:local:test"}`))
	if err != nil || local.Type != "track" || len(local.Raw) == 0 {
		t.Fatalf("local playable=%#v err=%v", local, err)
	}
	var overflow spotifyTrack
	if err := json.Unmarshal([]byte(trackJSON(testTrackID)), &overflow); err != nil {
		t.Fatal(err)
	}
	overflow.DurationMS = int64(^uint64(0) >> 1)
	if _, err := mapTrack(overflow); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("overflow duration error=%v", err)
	}
}

func serverURL(request *http.Request) string {
	return "http://" + request.Host
}

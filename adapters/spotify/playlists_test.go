package spotify

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestPlaylistWorkflow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.Method + " " + request.URL.Path {
		case "GET /v1/me/playlists":
			if request.URL.Query().Get("limit") != "50" {
				t.Errorf("playlist query=%v", request.URL.Query())
			}
			writeJSON(writer, `{"items":[`+playlistJSON()+`],"limit":50,"offset":0,"total":1}`)
		case "GET /v1/playlists/" + testPlaylistID:
			if request.URL.Query().Get("market") != "US" || request.URL.Query().Get("additional_types") != "track,episode" {
				t.Errorf("get playlist query=%v", request.URL.Query())
			}
			writeJSON(writer, playlistJSON())
		case "POST /v1/me/playlists":
			var body struct {
				Name          string `json:"name"`
				Public        *bool  `json:"public"`
				Collaborative bool   `json:"collaborative"`
			}
			_ = json.NewDecoder(request.Body).Decode(&body)
			if body.Name != "Shared mix" || body.Public == nil || *body.Public || !body.Collaborative {
				t.Errorf("create body=%#v", body)
			}
			writer.WriteHeader(http.StatusCreated)
			writeJSON(writer, playlistJSON())
		case "PUT /v1/playlists/" + testPlaylistID:
			var body map[string]any
			_ = json.NewDecoder(request.Body).Decode(&body)
			if body["description"] != "Updated" || body["public"] != false {
				t.Errorf("change body=%v", body)
			}
			writer.WriteHeader(http.StatusOK)
		case "GET /v1/playlists/" + testPlaylistID + "/items":
			if request.URL.Query().Get("limit") != "2" || request.URL.Query().Get("additional_types") != "track,episode" {
				t.Errorf("items query=%v", request.URL.Query())
			}
			writeJSON(writer, `{"items":[{"added_at":"2026-08-01T00:00:00Z","added_by":{"account_id":"`+testAccountID+`"},"item":`+trackJSON(testTrackID)+`},{"added_at":"2026-08-01T01:00:00Z","added_by":{"id":"legacy-user"},"item":`+episodeJSON()+`}],"limit":2,"offset":0,"total":3,"next":"`+serverURL(request)+`/v1/playlists/`+testPlaylistID+`/items?offset=2"}`)
		case "POST /v1/playlists/" + testPlaylistID + "/items":
			var body struct {
				URIs     []string `json:"uris"`
				Position *int     `json:"position"`
			}
			_ = json.NewDecoder(request.Body).Decode(&body)
			if len(body.URIs) != 2 || body.URIs[0] != body.URIs[1] || body.Position == nil || *body.Position != 1 {
				t.Errorf("add body=%#v", body)
			}
			writeJSON(writer, `{"snapshot_id":"snapshot-add"}`)
		case "PUT /v1/playlists/" + testPlaylistID + "/items":
			body, _ := io.ReadAll(request.Body)
			var values map[string]json.RawMessage
			_ = json.Unmarshal(body, &values)
			if _, reorder := values["range_start"]; reorder {
				writeJSON(writer, `{"snapshot_id":"snapshot-reorder"}`)
				return
			}
			if _, replace := values["uris"]; !replace {
				t.Errorf("replace body=%s", body)
			}
			writeJSON(writer, `{"snapshot_id":"snapshot-replace"}`)
		case "DELETE /v1/playlists/" + testPlaylistID + "/items":
			var body struct {
				Items      []map[string]string `json:"items"`
				SnapshotID string              `json:"snapshot_id"`
			}
			_ = json.NewDecoder(request.Body).Decode(&body)
			if len(body.Items) != 1 || body.SnapshotID != "snapshot-reorder" {
				t.Errorf("remove body=%#v", body)
			}
			writeJSON(writer, `{"snapshot_id":"snapshot-remove"}`)
		default:
			t.Errorf("unexpected request %s %s", request.Method, request.URL.String())
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, "premium", allTestScopes)

	listed, err := client.ListCurrentUserPlaylists(context.Background(), PaginationRequest{MaxResults: 100})
	if err != nil || len(listed.Items) != 1 || listed.Items[0].OwnerAccountID != testAccountID || listed.Items[0].ItemsTotal != 2 {
		t.Fatalf("playlists=%#v err=%v", listed, err)
	}
	playlist, err := client.GetPlaylist(context.Background(), testPlaylistID, "US")
	if err != nil || playlist.ID != testPlaylistID {
		t.Fatalf("playlist=%#v err=%v", playlist, err)
	}
	private := false
	created, err := client.CreatePlaylist(context.Background(), CreatePlaylistRequest{Name: "Shared mix", Description: "Description", Public: &private, Collaborative: true})
	if err != nil || created.ID != testPlaylistID {
		t.Fatalf("created=%#v err=%v", created, err)
	}
	description := "Updated"
	if err := client.ChangePlaylistDetails(context.Background(), ChangePlaylistDetailsRequest{PlaylistID: testPlaylistID, Description: &description, Public: &private}); err != nil {
		t.Fatal(err)
	}
	items, err := client.ListPlaylistItems(context.Background(), PlaylistItemsRequest{PlaylistID: testPlaylistID, Market: "US", MaxResults: 2})
	if err != nil || len(items.Items) != 2 || items.Items[0].Item.Track == nil || items.Items[1].Item.Episode == nil || items.NextCursor == nil || *items.NextCursor != "2" {
		t.Fatalf("items=%#v err=%v", items, err)
	}
	position := 1
	trackURI := "spotify:track:" + testTrackID
	snapshot, err := client.AddPlaylistItems(context.Background(), AddPlaylistItemsRequest{PlaylistID: testPlaylistID, URIs: []string{trackURI, trackURI}, Position: &position})
	if err != nil || snapshot != "snapshot-add" {
		t.Fatalf("add snapshot=%q err=%v", snapshot, err)
	}
	snapshot, err = client.ReplacePlaylistItems(context.Background(), ReplacePlaylistItemsRequest{PlaylistID: testPlaylistID})
	if err != nil || snapshot != "snapshot-replace" {
		t.Fatalf("replace snapshot=%q err=%v", snapshot, err)
	}
	snapshot, err = client.ReorderPlaylistItems(context.Background(), ReorderPlaylistItemsRequest{PlaylistID: testPlaylistID, RangeStart: 1, InsertBefore: 3, RangeLength: 2, SnapshotID: "snapshot-add"})
	if err != nil || snapshot != "snapshot-reorder" {
		t.Fatalf("reorder snapshot=%q err=%v", snapshot, err)
	}
	snapshot, err = client.RemovePlaylistItems(context.Background(), RemovePlaylistItemsRequest{PlaylistID: testPlaylistID, URIs: []string{trackURI}, SnapshotID: "snapshot-reorder"})
	if err != nil || snapshot != "snapshot-remove" {
		t.Fatalf("remove snapshot=%q err=%v", snapshot, err)
	}
}

func TestPlaylistValidationAndScopeGates(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, "premium", allTestScopes)
	public := true
	negative := -1
	invalidCalls := []func() error{
		func() error {
			_, err := client.ListCurrentUserPlaylists(context.Background(), PaginationRequest{MaxResults: -1})
			return err
		},
		func() error { _, err := client.GetPlaylist(context.Background(), "bad-id", ""); return err },
		func() error {
			_, err := client.CreatePlaylist(context.Background(), CreatePlaylistRequest{})
			return err
		},
		func() error {
			_, err := client.CreatePlaylist(context.Background(), CreatePlaylistRequest{Name: "x", Public: &public, Collaborative: true})
			return err
		},
		func() error {
			return client.ChangePlaylistDetails(context.Background(), ChangePlaylistDetailsRequest{PlaylistID: testPlaylistID})
		},
		func() error {
			return client.ChangePlaylistDetails(context.Background(), ChangePlaylistDetailsRequest{PlaylistID: testPlaylistID, Public: &public, Collaborative: &public})
		},
		func() error {
			_, err := client.ListPlaylistItems(context.Background(), PlaylistItemsRequest{PlaylistID: "bad-id"})
			return err
		},
		func() error {
			_, err := client.AddPlaylistItems(context.Background(), AddPlaylistItemsRequest{PlaylistID: testPlaylistID})
			return err
		},
		func() error {
			_, err := client.AddPlaylistItems(context.Background(), AddPlaylistItemsRequest{PlaylistID: testPlaylistID, URIs: []string{"spotify:track:" + testTrackID}, Position: &negative})
			return err
		},
		func() error {
			_, err := client.ReorderPlaylistItems(context.Background(), ReorderPlaylistItemsRequest{PlaylistID: testPlaylistID, RangeStart: -1})
			return err
		},
		func() error {
			_, err := client.RemovePlaylistItems(context.Background(), RemovePlaylistItemsRequest{PlaylistID: testPlaylistID})
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
	if _, err := limited.ListCurrentUserPlaylists(context.Background(), PaginationRequest{}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("playlist read scope error=%v", err)
	}
	if _, err := limited.AddPlaylistItems(context.Background(), AddPlaylistItemsRequest{PlaylistID: testPlaylistID, URIs: []string{"spotify:track:" + testTrackID}}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("playlist modify scope error=%v", err)
	}
}

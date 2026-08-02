package applemusic

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestTypedWorkflowsAndAuthenticationHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer developer-token" || request.Header.Get("Music-User-Token") != "music-user-token" {
			t.Errorf("authentication headers=%v", request.Header)
		}
		switch request.Method + " " + request.URL.Path {
		case "GET /v1/storefronts":
			writeJSON(writer, `{"data":[`+storefrontJSON()+`]}`)
		case "GET /v1/storefronts/us", "GET /v1/me/storefront":
			writeJSON(writer, `{"data":[`+storefrontJSON()+`]}`)
		case "GET /v1/catalog/us/songs/song1":
			writeJSON(writer, `{"data":[`+songJSON("songs", "song1")+`]}`)
		case "GET /v1/catalog/us/albums/album1":
			writeJSON(writer, `{"data":[`+albumJSON("albums", "album1")+`]}`)
		case "GET /v1/catalog/us/artists/artist1":
			writeJSON(writer, `{"data":[`+artistJSON("artists", "artist1")+`]}`)
		case "GET /v1/catalog/us/playlists/playlist1":
			writeJSON(writer, `{"data":[`+playlistJSON("playlists", "playlist1")+`]}`)
		case "GET /v1/catalog/us/music-videos/video1":
			writeJSON(writer, `{"data":[`+videoJSON("music-videos", "video1")+`]}`)
		case "GET /v1/catalog/us/search":
			if request.URL.Query().Get("term") != "test music" || request.URL.Query().Get("types") != "songs,albums,artists,playlists,music-videos" || request.URL.Query().Get("limit") != "5" {
				t.Errorf("catalog search query=%v", request.URL.Query())
			}
			writeJSON(writer, `{"results":{"songs":{"data":[`+songJSON("songs", "song1")+`],"next":"/v1/catalog/us/search?offset=5&term=test"},"albums":{"data":[`+albumJSON("albums", "album1")+`]},"artists":{"data":[`+artistJSON("artists", "artist1")+`]},"playlists":{"data":[`+playlistJSON("playlists", "playlist1")+`]},"music-videos":{"data":[`+videoJSON("music-videos", "video1")+`]}}}`)
		case "GET /v1/catalog/us/charts":
			if request.URL.Query().Get("types") != "songs,albums" || request.URL.Query().Get("with") != "dailyGlobalTopCharts" || request.URL.Query().Get("limit") != "20" {
				t.Errorf("charts query=%v", request.URL.Query())
			}
			writeJSON(writer, `{"results":{"songs":[{"chart":"most-played","name":"Top Songs","orderId":"most-played:songs","data":[`+songJSON("songs", "song1")+`],"next":"/v1/catalog/us/charts?offset=20&types=songs"}],"albums":[{"chart":"most-played","name":"Top Albums","data":[`+albumJSON("albums", "album1")+`]}]}}`)
		case "GET /v1/me/library/songs":
			writeJSON(writer, libraryPageJSON(songJSON("library-songs", "library-song1"), request.URL.Path))
		case "GET /v1/me/library/albums":
			writeJSON(writer, libraryPageJSON(albumJSON("library-albums", "library-album1"), request.URL.Path))
		case "GET /v1/me/library/artists":
			writeJSON(writer, libraryPageJSON(artistJSON("library-artists", "library-artist1"), request.URL.Path))
		case "GET /v1/me/library/playlists":
			writeJSON(writer, libraryPageJSON(playlistJSON("library-playlists", "library-playlist1"), request.URL.Path))
		case "GET /v1/me/library/music-videos":
			writeJSON(writer, libraryPageJSON(videoJSON("library-music-videos", "library-video1"), request.URL.Path))
		case "GET /v1/me/library/search":
			if request.URL.Query().Get("types") != "library-songs,library-albums" || request.URL.Query().Get("limit") != "25" {
				t.Errorf("library search query=%v", request.URL.Query())
			}
			writeJSON(writer, `{"results":{"library-songs":{"data":[`+songJSON("library-songs", "library-song1")+`]},"library-albums":{"data":[`+albumJSON("library-albums", "library-album1")+`]}}}`)
		case "POST /v1/me/library":
			if request.URL.Query().Get("ids[songs]") != "song1,song2" || request.URL.Query().Get("ids[albums]") != "album1" {
				t.Errorf("add library query=%v", request.URL.Query())
			}
			writer.WriteHeader(http.StatusAccepted)
		case "POST /v1/me/library/playlists":
			body, _ := io.ReadAll(request.Body)
			var payload struct {
				Attributes struct {
					Name     string `json:"name"`
					IsPublic *bool  `json:"isPublic"`
				} `json:"attributes"`
				Relationships struct {
					Tracks struct {
						Data []ResourceReference `json:"data"`
					} `json:"tracks"`
					Parent struct {
						Data []ResourceReference `json:"data"`
					} `json:"parent"`
				} `json:"relationships"`
			}
			if json.Unmarshal(body, &payload) != nil || payload.Attributes.Name != "Road Trip" || payload.Attributes.IsPublic == nil || !*payload.Attributes.IsPublic ||
				len(payload.Relationships.Tracks.Data) != 1 || payload.Relationships.Parent.Data[0].Type != ResourceLibraryPlaylistFolders {
				t.Errorf("create playlist body=%s", body)
			}
			writer.WriteHeader(http.StatusCreated)
			writeJSON(writer, `{"data":[`+playlistJSON("library-playlists", "library-playlist1")+`]}`)
		case "POST /v1/me/library/playlists/library-playlist1/tracks":
			var payload struct {
				Data []ResourceReference `json:"data"`
			}
			_ = json.NewDecoder(request.Body).Decode(&payload)
			if len(payload.Data) != 2 || payload.Data[0].Type != ResourceSongs || payload.Data[1].Type != ResourceLibraryMusicVideos {
				t.Errorf("add tracks body=%#v", payload)
			}
			writer.WriteHeader(http.StatusNoContent)
		case "GET /v1/me/recent/played":
			if request.URL.Query().Get("types") != "albums,stations" || request.URL.Query().Get("limit") != "10" {
				t.Errorf("history query=%v", request.URL.Query())
			}
			writeJSON(writer, `{"data":[`+albumJSON("albums", "album1")+`],"next":"/v1/me/recent/played?offset=10"}`)
		case "GET /v1/me/recent/played/tracks":
			if request.URL.Query().Get("types") != "songs,library-songs" || request.URL.Query().Get("limit") != "30" {
				t.Errorf("track history query=%v", request.URL.Query())
			}
			writeJSON(writer, `{"data":[`+songJSON("songs", "song1")+`],"next":"/v1/me/recent/played/tracks?offset=30"}`)
		default:
			t.Errorf("unexpected request %s %s", request.Method, request.URL.String())
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, true)
	ctx := context.Background()

	storefronts, err := client.ListStorefronts(ctx)
	if err != nil || len(storefronts) != 1 || storefronts[0].ID != "us" {
		t.Fatalf("storefronts=%#v err=%v", storefronts, err)
	}
	storefront, err := client.GetStorefront(ctx, "us", "en-US")
	if err != nil || storefront.Attributes.Name != "United States" {
		t.Fatalf("storefront=%#v err=%v", storefront, err)
	}
	if current, err := client.CurrentUserStorefront(ctx); err != nil || current.ID != "us" {
		t.Fatalf("current storefront=%#v err=%v", current, err)
	}
	if song, err := client.GetCatalogSong(ctx, "", "song1", ""); err != nil || song.Attributes.Name != "Song" {
		t.Fatalf("song=%#v err=%v", song, err)
	}
	if album, err := client.GetCatalogAlbum(ctx, "us", "album1", ""); err != nil || album.Attributes.Name != "Album" {
		t.Fatalf("album=%#v err=%v", album, err)
	}
	if artist, err := client.GetCatalogArtist(ctx, "us", "artist1", ""); err != nil || artist.Attributes.Name != "Artist" {
		t.Fatalf("artist=%#v err=%v", artist, err)
	}
	if playlist, err := client.GetCatalogPlaylist(ctx, "us", "playlist1", ""); err != nil || playlist.Attributes.Name != "Playlist" {
		t.Fatalf("playlist=%#v err=%v", playlist, err)
	}
	if video, err := client.GetCatalogMusicVideo(ctx, "us", "video1", ""); err != nil || video.Attributes.Name != "Video" {
		t.Fatalf("video=%#v err=%v", video, err)
	}

	search, err := client.SearchCatalog(ctx, CatalogSearchRequest{
		Term: "test music", Types: []ResourceType{ResourceSongs, ResourceAlbums, ResourceArtists, ResourcePlaylists, ResourceMusicVideos}, MaxResults: 5,
	})
	if err != nil || len(search.Songs.Data) != 1 || search.Songs.NextCursor == nil || *search.Songs.NextCursor != "5" || len(search.MusicVideos.Data) != 1 {
		t.Fatalf("catalog search=%#v err=%v", search, err)
	}
	charts, err := client.GetCatalogCharts(ctx, CatalogChartsRequest{Types: []ResourceType{ResourceSongs, ResourceAlbums}, With: []string{"dailyGlobalTopCharts"}, MaxResults: 20})
	if err != nil || len(charts.Songs) != 1 || charts.Songs[0].NextCursor == nil || *charts.Songs[0].NextCursor != "20" {
		t.Fatalf("charts=%#v err=%v", charts, err)
	}

	librarySongs, err := client.ListLibrarySongs(ctx, PaginationRequest{MaxResults: 25})
	if err != nil || len(librarySongs.Data) != 1 || librarySongs.Data[0].ID != "library-song1" || librarySongs.NextCursor == nil || *librarySongs.NextCursor != "25" {
		t.Fatalf("library songs=%#v err=%v", librarySongs, err)
	}
	libraryAlbums, err := client.ListLibraryAlbums(ctx, PaginationRequest{MaxResults: 25})
	if err != nil || len(libraryAlbums.Data) != 1 || libraryAlbums.Data[0].ID != "library-album1" {
		t.Fatalf("library albums=%#v err=%v", libraryAlbums, err)
	}
	libraryArtists, err := client.ListLibraryArtists(ctx, PaginationRequest{MaxResults: 25})
	if err != nil || len(libraryArtists.Data) != 1 || libraryArtists.Data[0].ID != "library-artist1" {
		t.Fatalf("library artists=%#v err=%v", libraryArtists, err)
	}
	libraryPlaylists, err := client.ListLibraryPlaylists(ctx, PaginationRequest{MaxResults: 25})
	if err != nil || len(libraryPlaylists.Data) != 1 || libraryPlaylists.Data[0].ID != "library-playlist1" {
		t.Fatalf("library playlists=%#v err=%v", libraryPlaylists, err)
	}
	libraryVideos, err := client.ListLibraryMusicVideos(ctx, PaginationRequest{MaxResults: 25})
	if err != nil || len(libraryVideos.Data) != 1 || libraryVideos.Data[0].ID != "library-video1" {
		t.Fatalf("library videos=%#v err=%v", libraryVideos, err)
	}
	librarySearch, err := client.SearchLibrary(ctx, LibrarySearchRequest{Term: "test", Types: []ResourceType{ResourceLibrarySongs, ResourceLibraryAlbums}, MaxResults: 25})
	if err != nil || len(librarySearch.Songs.Data) != 1 || len(librarySearch.Albums.Data) != 1 {
		t.Fatalf("library search=%#v err=%v", librarySearch, err)
	}
	if err := client.AddToLibrary(ctx, AddToLibraryRequest{SongIDs: []string{"song1", "song2"}, AlbumIDs: []string{"album1"}}); err != nil {
		t.Fatal(err)
	}
	public := true
	created, err := client.CreateLibraryPlaylist(ctx, CreateLibraryPlaylistRequest{
		Name: "Road Trip", Description: "Summer", IsPublic: &public,
		Tracks: []ResourceReference{{ID: "song1", Type: ResourceSongs}}, ParentFolderID: "folder1",
	})
	if err != nil || created.ID != "library-playlist1" {
		t.Fatalf("created=%#v err=%v", created, err)
	}
	if err := client.AddTracksToLibraryPlaylist(ctx, AddPlaylistTracksRequest{
		PlaylistID: "library-playlist1", Tracks: []ResourceReference{{ID: "song1", Type: ResourceSongs}, {ID: "library-video1", Type: ResourceLibraryMusicVideos}},
	}); err != nil {
		t.Fatal(err)
	}
	history, err := client.ListRecentlyPlayed(ctx, RecentlyPlayedRequest{Types: []HistoryResourceType{HistoryAlbums, HistoryStations}, MaxResults: 10})
	if err != nil || len(history.Data) != 1 || history.NextCursor == nil || *history.NextCursor != "10" || len(history.Data[0].Attributes) == 0 {
		t.Fatalf("history=%#v err=%v", history, err)
	}
	tracks, err := client.ListRecentlyPlayedTracks(ctx, RecentlyPlayedTracksRequest{Types: []ResourceType{ResourceSongs, ResourceLibrarySongs}, MaxResults: 30})
	if err != nil || len(tracks.Data) != 1 || tracks.NextCursor == nil || *tracks.NextCursor != "30" {
		t.Fatalf("tracks=%#v err=%v", tracks, err)
	}
}

func TestValidationPaginationAndHTTPErrors(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, true)
	ctx := context.Background()
	invalidCalls := []func() error{
		func() error { _, err := client.GetStorefront(ctx, "USA", ""); return err },
		func() error { _, err := client.GetCatalogSong(ctx, "", "bad/id", ""); return err },
		func() error {
			_, err := client.SearchCatalog(ctx, CatalogSearchRequest{Term: "x", Types: []ResourceType{ResourceSongs}, MaxResults: 26})
			return err
		},
		func() error {
			_, err := client.SearchCatalog(ctx, CatalogSearchRequest{Term: "x", Types: []ResourceType{ResourceSongs, ResourceSongs}})
			return err
		},
		func() error {
			_, err := client.GetCatalogCharts(ctx, CatalogChartsRequest{Types: []ResourceType{ResourceSongs}, With: []string{"unknown"}})
			return err
		},
		func() error { _, err := client.ListLibrarySongs(ctx, PaginationRequest{Cursor: "-1"}); return err },
		func() error {
			_, err := client.SearchLibrary(ctx, LibrarySearchRequest{Term: "", Types: []ResourceType{ResourceLibrarySongs}})
			return err
		},
		func() error { return client.AddToLibrary(ctx, AddToLibraryRequest{}) },
		func() error { return client.AddToLibrary(ctx, AddToLibraryRequest{SongIDs: []string{"same", "same"}}) },
		func() error { _, err := client.CreateLibraryPlaylist(ctx, CreateLibraryPlaylistRequest{}); return err },
		func() error {
			return client.AddTracksToLibraryPlaylist(ctx, AddPlaylistTracksRequest{PlaylistID: "playlist", Tracks: []ResourceReference{{ID: "song", Type: ResourceAlbums}}})
		},
		func() error {
			_, err := client.ListRecentlyPlayed(ctx, RecentlyPlayedRequest{Types: []HistoryResourceType{HistoryAlbums}, MaxResults: 11})
			return err
		},
		func() error {
			_, err := client.ListRecentlyPlayedTracks(ctx, RecentlyPlayedTracksRequest{Types: []ResourceType{ResourceSongs}, MaxResults: 31})
			return err
		},
	}
	for index, call := range invalidCalls {
		if err := call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("invalid call %d error=%v", index, err)
		}
	}
	for _, next := range []string{
		"https://evil.example/v1/me/library/songs?offset=1",
		"/v1/me/library/albums?offset=1",
		"/v1/me/library/songs?offset=-1",
		"/v1/me/library/songs",
		"not a url %",
	} {
		if _, err := cursorFromNext(next, "/me/library/songs", client.apiBaseURL); errorCode(err) != socialhub.CodePlatformError {
			t.Fatalf("next=%q error=%v", next, err)
		}
	}

	tests := []struct {
		status int
		code   socialhub.ErrorCode
		class  socialhub.ErrorClass
	}{
		{http.StatusBadRequest, socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{http.StatusUnauthorized, socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{http.StatusForbidden, socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{http.StatusNotFound, socialhub.CodeNotFound, socialhub.ClassPermanent},
		{http.StatusConflict, socialhub.CodeConflict, socialhub.ClassPermanent},
		{http.StatusTooManyRequests, socialhub.CodeRateLimited, socialhub.ClassRetryable},
		{http.StatusServiceUnavailable, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{http.StatusTeapot, socialhub.CodePlatformError, socialhub.ClassPermanent},
	}
	for _, test := range tests {
		header := http.Header{"Retry-After": {"7"}, "X-Apple-Jingle-Correlation-Key": {"request-1"}}
		err := decodeHTTPError(test.status, header, []byte(`{"errors":[{"status":"failure","code":"APPLE_ERROR","title":"Failed","detail":"failure detail"}]}`))
		var hubError *socialhub.Error
		if !errors.As(err, &hubError) || hubError.Code != test.code || hubError.Class != test.class || hubError.PlatformCode != "APPLE_ERROR" ||
			hubError.PlatformMessage != "failure detail" || hubError.RequestID != "request-1" || hubError.RetryAfter != 7*time.Second {
			t.Fatalf("status=%d error=%#v", test.status, err)
		}
	}
	if parseRetryAfter("invalid") != 0 || parseRetryAfter("-1") != 0 || bounded(strings.Repeat("x", 20), 5) != "xxxxx" {
		t.Fatal("error helper mismatch")
	}
}

func writeJSON(writer http.ResponseWriter, value string) {
	writer.Header().Set("Content-Type", "application/json")
	_, _ = writer.Write([]byte(value))
}

func storefrontJSON() string {
	return `{"id":"us","type":"storefronts","attributes":{"defaultLanguageTag":"en-US","explicitContentPolicy":"allowed","name":"United States","supportedLanguageTags":["en-US"]}}`
}

func songJSON(resourceType, id string) string {
	return `{"id":"` + id + `","type":"` + resourceType + `","attributes":{"albumName":"Album","artistName":"Artist","durationInMillis":123000,"name":"Song","genreNames":["Pop"],"playParams":{"id":"` + id + `","kind":"song"}}}`
}

func albumJSON(resourceType, id string) string {
	return `{"id":"` + id + `","type":"` + resourceType + `","attributes":{"artistName":"Artist","name":"Album","genreNames":["Pop"],"trackCount":1,"playParams":{"id":"` + id + `","kind":"album"}}}`
}

func artistJSON(resourceType, id string) string {
	return `{"id":"` + id + `","type":"` + resourceType + `","attributes":{"name":"Artist","genreNames":["Pop"]}}`
}

func playlistJSON(resourceType, id string) string {
	return `{"id":"` + id + `","type":"` + resourceType + `","attributes":{"name":"Playlist","description":{"standard":"Description"},"playParams":{"id":"` + id + `","kind":"playlist"}}}`
}

func videoJSON(resourceType, id string) string {
	return `{"id":"` + id + `","type":"` + resourceType + `","attributes":{"artistName":"Artist","durationInMillis":1000,"name":"Video","genreNames":["Pop"],"playParams":{"id":"` + id + `","kind":"musicVideo"}}}`
}

func libraryPageJSON(resource, requestPath string) string {
	return `{"data":[` + resource + `],"next":"` + requestPath + `?offset=25","meta":{"total":100}}`
}

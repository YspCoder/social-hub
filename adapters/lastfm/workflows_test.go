package lastfm

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestDiscoveryAndUserWorkflows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		query := request.URL.Query()
		if request.Method != http.MethodGet || query.Get("api_key") != testAPIKey || query.Get("format") != "json" {
			http.Error(writer, "bad request", http.StatusBadRequest)
			return
		}
		switch query.Get("method") {
		case "track.getInfo":
			writeJSON(writer, http.StatusOK, `{"track":{"name":"Believe","mbid":"track-mbid","url":"track-url","duration":"240000","streamable":{"#text":"1","fulltrack":"0"},"listeners":"10","playcount":"20","userplaycount":"3","userloved":"1","artist":{"name":"Cher","mbid":"artist-mbid"},"album":{"title":"Believe","artist":"Cher","image":[{"size":"small","#text":"cover"}]},"toptags":{"tag":[{"name":"pop","url":"tag-url"}]},"wiki":{"summary":"summary"}}}`)
		case "track.search":
			writeJSON(writer, http.StatusOK, searchFixture("trackmatches", "track", `{"name":"Believe","artist":"Cher","streamable":"1","listeners":"9"}`))
		case "artist.getInfo":
			writeJSON(writer, http.StatusOK, `{"artist":{"name":"Cher","mbid":"artist-mbid","url":"artist-url","streamable":"1","ontour":"0","stats":{"listeners":"11","playcount":"22","userplaycount":"2"},"tags":{"tag":[{"name":"pop"}]},"similar":{"artist":[{"name":"Madonna"}]},"bio":{"summary":"bio"}}}`)
		case "artist.search":
			writeJSON(writer, http.StatusOK, searchFixture("artistmatches", "artist", `{"name":"Cher","streamable":"1"}`))
		case "album.getInfo":
			writeJSON(writer, http.StatusOK, `{"album":{"name":"Believe","artist":"Cher","mbid":"album-mbid","listeners":"7","playcount":"8","tracks":{"track":[{"name":"Believe","duration":"240","artist":{"name":"Cher"},"streamable":"1","@attr":{"rank":"1"}}]},"toptags":{"tag":[{"name":"pop"}]},"wiki":{"summary":"album wiki"}}}`)
		case "album.search":
			writeJSON(writer, http.StatusOK, searchFixture("albummatches", "album", `{"name":"Believe","artist":"Cher","image":[{"size":"large","#text":"cover"}]}`))
		case "user.getInfo":
			writeJSON(writer, http.StatusOK, `{"user":{"name":"test-user","realname":"Test User","url":"user-url","country":"CN","age":"30","gender":"n","subscriber":"1","playcount":"100","artist_count":"20","album_count":"30","track_count":"40","playlists":"2","registered":{"unixtime":"1700000000"},"image":[{"size":"medium","#text":"avatar"}]}}`)
		case "user.getRecentTracks":
			writeJSON(writer, http.StatusOK, `{"recenttracks":{"track":[{"name":"Playing","artist":{"#text":"Artist"},"album":{"#text":"Album"},"@attr":{"nowplaying":"true"}},{"name":"Played","artist":{"#text":"Artist"},"date":{"uts":"1700000100"}}],"@attr":{"page":"2","perPage":"2","totalPages":"3","total":"6"}}}`)
		case "user.getTopTracks":
			writeJSON(writer, http.StatusOK, `{"toptracks":{"track":[{"name":"Top","artist":{"name":"Artist"},"duration":"180000","playcount":"9","@attr":{"rank":"1"}}],"@attr":{"page":"1","perPage":"1","totalPages":"1","total":"1"}}}`)
		case "user.getLovedTracks":
			writeJSON(writer, http.StatusOK, `{"lovedtracks":{"track":[{"name":"Loved","artist":{"name":"Artist"},"date":{"uts":"1700000200"}}],"@attr":{"page":"1","perPage":"1","totalPages":"1","total":"1"}}}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, false)

	track, err := client.GetTrack(context.Background(), TrackInfoRequest{Artist: "Cher", Track: "Believe", Autocorrect: true})
	if err != nil || track.Name != "Believe" || track.Duration != 4*time.Minute || !track.Streamable || track.FullTrack || track.Album == nil || !track.UserLoved || len(track.Tags) != 1 {
		t.Fatalf("track=%#v err=%v", track, err)
	}
	trackPage, err := client.SearchTracks(context.Background(), SearchRequest{Query: "Believe", Artist: "Cher", Cursor: "2", MaxResults: 1})
	if err != nil || len(trackPage.Items) != 1 || trackPage.NextCursor == nil || trackPage.PrevCursor == nil {
		t.Fatalf("track page=%#v err=%v", trackPage, err)
	}
	artist, err := client.GetArtist(context.Background(), ArtistInfoRequest{Artist: "Cher", Language: "en", Autocorrect: true})
	if err != nil || artist.Name != "Cher" || artist.Listeners != 11 || len(artist.Similar) != 1 || artist.Biography.Summary != "bio" {
		t.Fatalf("artist=%#v err=%v", artist, err)
	}
	artistPage, err := client.SearchArtists(context.Background(), SearchRequest{Query: "Cher", MaxResults: 1})
	if err != nil || len(artistPage.Items) != 1 || artistPage.Items[0].Name != "Cher" {
		t.Fatalf("artist page=%#v err=%v", artistPage, err)
	}
	album, err := client.GetAlbum(context.Background(), AlbumInfoRequest{Artist: "Cher", Album: "Believe", Language: "en"})
	if err != nil || album.Name != "Believe" || len(album.Tracks) != 1 || album.Tracks[0].Duration != 4*time.Minute {
		t.Fatalf("album=%#v err=%v", album, err)
	}
	albumPage, err := client.SearchAlbums(context.Background(), SearchRequest{Query: "Believe", MaxResults: 1})
	if err != nil || len(albumPage.Items) != 1 || albumPage.Items[0].Artist != "Cher" {
		t.Fatalf("album page=%#v err=%v", albumPage, err)
	}
	user, err := client.GetUser(context.Background(), "")
	if err != nil || user.Name != "test-user" || user.RegisteredAt == nil || user.PlayCount != 100 || len(user.Images) != 1 {
		t.Fatalf("user=%#v err=%v", user, err)
	}
	recent, err := client.RecentTracks(context.Background(), RecentTracksRequest{
		From: time.Unix(1700000000, 0), To: time.Unix(1700000300, 0), Extended: true, Cursor: "2", MaxResults: 2,
	})
	if err != nil || len(recent.Items) != 2 || !recent.Items[0].NowPlaying || recent.Items[1].PlayedAt == nil || !recent.HasMore {
		t.Fatalf("recent=%#v err=%v", recent, err)
	}
	top, err := client.TopTracks(context.Background(), TopTracksRequest{Period: Period7Day, MaxResults: 1})
	if err != nil || len(top.Items) != 1 || top.Items[0].Rank != 1 || top.Items[0].Duration != 3*time.Minute {
		t.Fatalf("top=%#v err=%v", top, err)
	}
	loved, err := client.LovedTracks(context.Background(), UserTracksRequest{MaxResults: 1})
	if err != nil || len(loved.Items) != 1 || loved.Items[0].LovedAt == nil || loved.Items[0].PlayedAt != nil {
		t.Fatalf("loved=%#v err=%v", loved, err)
	}
}

func TestListeningAndLibraryWorkflows(t *testing.T) {
	methods := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.RawQuery != "" {
			http.Error(writer, "writes must use body", http.StatusBadRequest)
			return
		}
		if err := request.ParseForm(); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		method := request.PostForm.Get("method")
		methods[method]++
		if request.PostForm.Get("api_key") != testAPIKey || request.PostForm.Get("sk") != testSession ||
			request.PostForm.Get("api_sig") != signature(request.PostForm, testAPISecret) || request.PostForm.Get("format") != "json" {
			http.Error(writer, "bad auth", http.StatusBadRequest)
			return
		}
		switch method {
		case "track.updateNowPlaying":
			writeJSON(writer, http.StatusOK, `{"nowplaying":{"track":{"#text":"Track","corrected":"0"},"artist":{"#text":"Artist","corrected":"1"},"album":{"#text":"Album","corrected":"0"},"albumArtist":{"#text":"Artist","corrected":"0"}}}`)
		case "track.scrobble":
			if request.PostForm.Get("chosenByUser[0]") != "0" || request.PostForm.Get("duration[0]") != "180" {
				http.Error(writer, "missing metadata", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"scrobbles":{"@attr":{"accepted":"1","ignored":"0"},"scrobble":[{"track":{"#text":"Track","corrected":"0"},"artist":{"#text":"Artist","corrected":"0"},"album":{"#text":"Album","corrected":"0"},"albumArtist":{"#text":"Artist","corrected":"0"},"timestamp":"1700000000","ignoredMessage":{"code":"0","#text":""}}]}}`)
		case "track.love", "track.unlove":
			writeJSON(writer, http.StatusOK, `{}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, true)
	nowPlaying, err := client.UpdateNowPlaying(context.Background(), NowPlayingRequest{
		Artist: "Artist", Track: "Track", Album: "Album", AlbumArtist: "Artist", TrackNumber: 1, MBID: "mbid", Duration: 3 * time.Minute,
	})
	if err != nil || nowPlaying.Track.Value != "Track" || !nowPlaying.Artist.Corrected {
		t.Fatalf("now playing=%#v err=%v", nowPlaying, err)
	}
	chosen := false
	result, err := client.Scrobble(context.Background(), []Scrobble{{
		Artist: "Artist", Track: "Track", StartedAt: time.Unix(1700000000, 0), Album: "Album",
		AlbumArtist: "Artist", TrackNumber: 1, MBID: "mbid", Duration: 3 * time.Minute, ChosenByUser: &chosen,
	}})
	if err != nil || result.Accepted != 1 || result.Ignored != 0 || len(result.Items) != 1 || result.Items[0].Track.Value != "Track" {
		t.Fatalf("scrobble=%#v err=%v", result, err)
	}
	if err := client.LoveTrack(context.Background(), TrackRef{Artist: "Artist", Track: "Track"}); err != nil {
		t.Fatal(err)
	}
	if err := client.UnloveTrack(context.Background(), TrackRef{Artist: "Artist", Track: "Track"}); err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"track.updateNowPlaying", "track.scrobble", "track.love", "track.unlove"} {
		if methods[method] != 1 {
			t.Fatalf("method %s calls=%d", method, methods[method])
		}
	}
}

func TestWorkflowValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, true)
	tests := []struct {
		name string
		call func() error
	}{
		{"track identity", func() error { _, err := client.GetTrack(context.Background(), TrackInfoRequest{}); return err }},
		{"artist language", func() error {
			_, err := client.GetArtist(context.Background(), ArtistInfoRequest{Artist: "A", Language: "english"})
			return err
		}},
		{"album identity", func() error {
			_, err := client.GetAlbum(context.Background(), AlbumInfoRequest{Artist: "A"})
			return err
		}},
		{"search query", func() error { _, err := client.SearchTracks(context.Background(), SearchRequest{}); return err }},
		{"search cursor", func() error {
			_, err := client.SearchArtists(context.Background(), SearchRequest{Query: "A", Cursor: "zero"})
			return err
		}},
		{"page size", func() error {
			_, err := client.SearchAlbums(context.Background(), SearchRequest{Query: "A", MaxResults: 201})
			return err
		}},
		{"recent range", func() error {
			_, err := client.RecentTracks(context.Background(), RecentTracksRequest{From: time.Unix(2, 0), To: time.Unix(1, 0)})
			return err
		}},
		{"top period", func() error {
			_, err := client.TopTracks(context.Background(), TopTracksRequest{Period: "daily"})
			return err
		}},
		{"now playing", func() error { _, err := client.UpdateNowPlaying(context.Background(), NowPlayingRequest{}); return err }},
		{"empty scrobble", func() error { _, err := client.Scrobble(context.Background(), nil); return err }},
		{"invalid scrobble", func() error {
			_, err := client.Scrobble(context.Background(), []Scrobble{{Artist: "A", Track: "T"}})
			return err
		}},
		{"love", func() error { return client.LoveTrack(context.Background(), TrackRef{}) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
				t.Fatalf("error=%v", err)
			}
		})
	}

	config := testConfig(server, true)
	config.Accounts[0].Settings = nil
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), config, socialhub.WithHTTPClient(server.Client()), socialhub.WithSecretResolver(mapResolver{"test://secret": testAPISecret, "test://session": testSession})); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "listener")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := common.(*Client).GetUser(context.Background(), ""); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("missing username=%v", err)
	}
}

func searchFixture(matchKey, itemKey, item string) string {
	return `{"results":{"opensearch:totalResults":"3","opensearch:itemsPerPage":"1","` + matchKey + `":{"` + itemKey + `":[` + item + `]}}}`
}

func TestBatchLimitAndIndexedSignature(t *testing.T) {
	items := make([]Scrobble, 51)
	for index := range items {
		items[index] = Scrobble{Artist: "A", Track: strconv.Itoa(index), StartedAt: time.Unix(1700000000, 0)}
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, true)
	if _, err := client.Scrobble(context.Background(), items); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("batch limit=%v", err)
	}

	values := url.Values{"artist[1]": {"one"}, "artist[10]": {"ten"}, "method": {"track.scrobble"}}
	if signature(values, testAPISecret) == "" {
		t.Fatal("indexed signature is empty")
	}
}

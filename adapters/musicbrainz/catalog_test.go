package musicbrainz

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-hub/pkg/socialhub"
)

const (
	artistMBID       = "b10bbbfc-cf9e-42e0-be17-e2c3e1d2600d"
	releaseGroupMBID = "9162580e-5df4-32de-80cc-f45a8d8a9b1d"
	releaseMBID      = "6bb3793b-f991-378e-9bff-0bd3117f2298"
	recordingMBID    = "564a05ec-dfa4-4f4c-8a4a-c66b6ef317b5"
	workMBID         = "bcd490e5-dac7-3b8a-b423-ae17e1209f3d"

	artistFixture       = `{"id":"` + artistMBID + `","type":"Group","type-id":"e431f5f6-b5d2-343d-8b36-72607fffb74b","name":"The Beatles","sort-name":"Beatles, The","country":"GB","area":{"id":"8a754a16-0027-3a29-b6d7-2b40ea0481ed","name":"United Kingdom","iso-3166-1-codes":["GB"]},"life-span":{"begin":"1960","end":"1970-04-10","ended":true},"aliases":[{"name":"Beatles","sort-name":"Beatles","locale":"en","type":null,"primary":true,"begin-date":null,"end-date":null}],"genres":[{"id":"genre-1","name":"rock","count":12}],"score":100}`
	releaseGroupFixture = `{"id":"` + releaseGroupMBID + `","title":"Abbey Road","primary-type":"Album","secondary-types":[],"first-release-date":"1969-09-26","artist-credit":[{"name":"The Beatles","joinphrase":"","artist":` + artistFixture + `}],"releases":[{"id":"` + releaseMBID + `","title":"Abbey Road"}],"genres":[{"id":"genre-1","name":"rock","count":8}],"score":100}`
	recordingFixture    = `{"id":"` + recordingMBID + `","title":"Come Together","length":259000,"video":false,"first-release-date":"1969-09-26","artist-credit":[{"name":"The Beatles","artist":` + artistFixture + `}],"isrcs":["GBAYE0601690"],"genres":[{"id":"genre-1","name":"rock","count":4}],"score":100}`
	releaseFixture      = `{"id":"` + releaseMBID + `","title":"Abbey Road","status":"Official","quality":"normal","packaging":"Jewel Case","date":"1987-10-10","country":"US","barcode":null,"asin":null,"text-representation":{"language":"eng","script":"Latn"},"artist-credit":[{"name":"The Beatles","artist":` + artistFixture + `}],"release-group":{"id":"` + releaseGroupMBID + `","title":"Abbey Road"},"release-events":[{"date":"1987-10-10","area":{"id":"area-1","name":"United States"}}],"label-info":[{"catalog-number":"CDP 7 46446 2","label":{"id":"label-1","name":"Capitol Records"}}],"media":[{"position":1,"format":"CD","track-count":1,"tracks":[{"id":"track-1","position":1,"number":"1","title":"Come Together","length":259000,"recording":` + recordingFixture + `}]}],"cover-art-archive":{"artwork":true,"count":1,"front":true,"back":false,"darkened":false},"score":100}`
	workFixture         = `{"id":"` + workMBID + `","title":"Come Together","type":"Song","language":"eng","languages":["eng"],"iswcs":["T-010.140.236-1"],"aliases":[{"name":"Come Together","sort-name":"Come Together","locale":null,"type":null,"primary":null,"begin-date":null,"end-date":null}],"attributes":[{"type":"ASCAP ID","value":"330153099"}],"genres":[{"id":"genre-1","name":"rock","count":2}],"relations":[{"type":"writer","target-type":"artist","direction":"backward","begin":null,"end":null,"ended":false,"artist":` + artistFixture + `},{"type":"performance","target-type":"recording","direction":"backward","begin":null,"end":null,"ended":false,"recording":` + recordingFixture + `}],"score":100}`
)

func TestCatalogSearchLookupAndBrowse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.Header.Get("User-Agent") != "social-hub-musicbrainz-tests/1.0 (tests@example.com)" ||
			request.Header.Get("Accept") != "application/json" || request.Header.Get("Authorization") != "" || request.URL.Query().Get("fmt") != "json" {
			http.Error(writer, "bad request identity", http.StatusBadRequest)
			return
		}
		if request.URL.Query().Has("query") && (request.URL.Query().Get("query") != `title:"fixture"` || request.URL.Query().Get("limit") != "1" || request.URL.Query().Get("offset") != "1") {
			http.Error(writer, "bad search query", http.StatusBadRequest)
			return
		}
		switch request.URL.Path {
		case "/api/artist":
			writeJSON(writer, http.StatusOK, `{"count":3,"offset":1,"artists":[`+artistFixture+`]}`)
		case "/api/artist/" + artistMBID:
			if request.URL.Query().Get("inc") != "aliases+genres" {
				http.Error(writer, "bad artist includes", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, artistFixture)
		case "/api/release-group":
			if request.URL.Query().Has("artist") {
				if request.URL.Query().Get("artist") != artistMBID || request.URL.Query().Get("inc") != "artist-credits" {
					http.Error(writer, "bad release-group browse", http.StatusBadRequest)
					return
				}
				writeJSON(writer, http.StatusOK, `{"release-group-count":3,"release-group-offset":1,"release-groups":[`+releaseGroupFixture+`]}`)
				return
			}
			writeJSON(writer, http.StatusOK, `{"count":3,"offset":1,"release-groups":[`+releaseGroupFixture+`]}`)
		case "/api/release-group/" + releaseGroupMBID:
			if request.URL.Query().Get("inc") != "artist-credits+genres+releases" {
				http.Error(writer, "bad release-group includes", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, releaseGroupFixture)
		case "/api/release":
			writeJSON(writer, http.StatusOK, `{"count":3,"offset":1,"releases":[`+releaseFixture+`]}`)
		case "/api/release/" + releaseMBID:
			if request.URL.Query().Get("inc") != "artist-credits+labels+recordings+release-groups+media+genres" {
				http.Error(writer, "bad release includes", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, releaseFixture)
		case "/api/recording":
			if request.URL.Query().Has("artist") {
				if request.URL.Query().Get("artist") != artistMBID || request.URL.Query().Get("inc") != "artist-credits+isrcs" {
					http.Error(writer, "bad recording browse", http.StatusBadRequest)
					return
				}
				writeJSON(writer, http.StatusOK, `{"recording-count":3,"recording-offset":1,"recordings":[`+recordingFixture+`]}`)
				return
			}
			writeJSON(writer, http.StatusOK, `{"count":3,"offset":1,"recordings":[`+recordingFixture+`]}`)
		case "/api/recording/" + recordingMBID:
			if request.URL.Query().Get("inc") != "artist-credits+isrcs+genres+releases" {
				http.Error(writer, "bad recording includes", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, recordingFixture)
		case "/api/work":
			writeJSON(writer, http.StatusOK, `{"count":3,"offset":1,"works":[`+workFixture+`]}`)
		case "/api/work/" + workMBID:
			if request.URL.Query().Get("inc") != "aliases+genres+artist-rels+recording-rels" {
				http.Error(writer, "bad work includes", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, workFixture)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server)
	ctx := context.Background()
	search := SearchRequest{Query: `title:"fixture"`, Limit: 1, Cursor: "1"}

	artists, err := client.SearchArtists(ctx, search, socialhub.WithRequestID("request-1"))
	if err != nil || len(artists.Items) != 1 || artists.Items[0].Name != "The Beatles" || artists.Items[0].Area == nil ||
		len(artists.Items[0].Aliases) != 1 || artists.NextCursor == nil || *artists.NextCursor != "2" || artists.PrevCursor == nil || *artists.PrevCursor != "0" {
		t.Fatalf("artists=%#v err=%v", artists, err)
	}
	artist, err := client.GetArtist(ctx, artistMBID)
	if err != nil || artist.ID != artistMBID || !artist.LifeSpan.Ended || len(artist.Genres) != 1 {
		t.Fatalf("artist=%#v err=%v", artist, err)
	}
	releaseGroups, err := client.SearchReleaseGroups(ctx, search)
	if err != nil || releaseGroups.Items[0].PrimaryType != "Album" || len(releaseGroups.Items[0].ArtistCredit) != 1 {
		t.Fatalf("release groups=%#v err=%v", releaseGroups, err)
	}
	releaseGroup, err := client.GetReleaseGroup(ctx, releaseGroupMBID)
	if err != nil || releaseGroup.Title != "Abbey Road" || len(releaseGroup.Releases) != 1 {
		t.Fatalf("release group=%#v err=%v", releaseGroup, err)
	}
	releases, err := client.SearchReleases(ctx, search)
	if err != nil || releases.Items[0].Status != "Official" || releases.Items[0].Barcode != nil {
		t.Fatalf("releases=%#v err=%v", releases, err)
	}
	release, err := client.GetRelease(ctx, releaseMBID)
	if err != nil || len(release.Media) != 1 || len(release.Media[0].Tracks) != 1 || release.Media[0].Tracks[0].Recording.ID != recordingMBID ||
		release.LabelInfo[0].Label == nil || !release.CoverArtArchive.Front {
		t.Fatalf("release=%#v err=%v", release, err)
	}
	recordings, err := client.SearchRecordings(ctx, search)
	if err != nil || recordings.Items[0].Length == nil || len(recordings.Items[0].ISRCs) != 1 {
		t.Fatalf("recordings=%#v err=%v", recordings, err)
	}
	recording, err := client.GetRecording(ctx, recordingMBID)
	if err != nil || recording.Title != "Come Together" || recording.Video {
		t.Fatalf("recording=%#v err=%v", recording, err)
	}
	works, err := client.SearchWorks(ctx, search)
	if err != nil || works.Items[0].Type != "Song" || len(works.Items[0].Relations) != 2 {
		t.Fatalf("works=%#v err=%v", works, err)
	}
	work, err := client.GetWork(ctx, workMBID)
	if err != nil || work.ISWCs[0] != "T-010.140.236-1" || work.Relations[0].Artist == nil || work.Relations[1].Recording == nil {
		t.Fatalf("work=%#v err=%v", work, err)
	}
	browse := BrowseRequest{Limit: 1, Cursor: "1"}
	artistReleaseGroups, err := client.ListArtistReleaseGroups(ctx, artistMBID, browse)
	if err != nil || len(artistReleaseGroups.Items) != 1 || artistReleaseGroups.NextCursor == nil {
		t.Fatalf("artist release groups=%#v err=%v", artistReleaseGroups, err)
	}
	artistRecordings, err := client.ListArtistRecordings(ctx, artistMBID, browse)
	if err != nil || len(artistRecordings.Items) != 1 || artistRecordings.Items[0].ID != recordingMBID {
		t.Fatalf("artist recordings=%#v err=%v", artistRecordings, err)
	}
}

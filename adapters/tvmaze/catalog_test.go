package tvmaze

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	showFixture    = `{"id":1,"url":"https://www.tvmaze.com/shows/1/show","name":"Severance","type":"Scripted","language":"English","genres":["Drama"],"status":"Running","runtime":50,"averageRuntime":52,"premiered":"2022-02-18","ended":null,"officialSite":"https://example.test","schedule":{"time":"21:00","days":["Friday"]},"rating":{"average":8.2},"weight":99,"network":null,"webChannel":{"id":2,"name":"Stream","country":null,"officialSite":"https://stream.test"},"dvdCountry":null,"externals":{"tvrage":null,"thetvdb":371980,"imdb":"tt11280740"},"image":{"medium":"medium.jpg","original":"original.jpg"},"summary":"<p>Work is mysterious.</p>","updated":1700000000,"_links":{"self":{"href":"https://api.tvmaze.com/shows/1"}}}`
	episodeFixture = `{"id":10,"url":"https://www.tvmaze.com/episodes/10","name":"Good News About Hell","season":1,"number":1,"type":"regular","airdate":"2022-02-18","airtime":"00:00","airstamp":"2022-02-18T05:00:00+00:00","runtime":57,"rating":{"average":8.0},"image":null,"summary":"<p>Episode</p>","_links":{"self":{"href":"https://api.tvmaze.com/episodes/10"}}}`
)

func TestCatalogWorkflowsAndCanonicalLookup(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.Header.Get("User-Agent") != "social-hub-tvmaze-tests/1.0" || request.Header.Get("Authorization") != "" {
			http.Error(writer, "bad request identity", http.StatusBadRequest)
			return
		}
		switch request.URL.Path {
		case "/api/search/shows":
			if request.URL.Query().Get("q") != "Severance" || request.Header.Get("X-Request-ID") != "request-1" {
				http.Error(writer, "bad search", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `[{"score":0.9,"show":`+showFixture+`}]`)
		case "/api/lookup/shows":
			if request.URL.Query().Get("imdb") != "tt11280740" {
				http.Error(writer, "bad lookup", http.StatusBadRequest)
				return
			}
			writer.Header().Set("Location", server.URL+"/api/shows/1")
			writer.WriteHeader(http.StatusMovedPermanently)
		case "/api/shows/1":
			writeJSON(writer, http.StatusOK, showFixture)
		case "/api/shows/1/episodes":
			if values, present := request.URL.Query()["specials"]; present && (len(values) != 1 || values[0] != "1") {
				http.Error(writer, "bad specials", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `[`+episodeFixture+`]`)
		case "/api/episodes/10":
			writeJSON(writer, http.StatusOK, episodeFixture)
		case "/api/shows/1/episodebynumber":
			if request.URL.Query().Get("season") != "1" || request.URL.Query().Get("number") != "1" {
				http.Error(writer, "bad episode number", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, episodeFixture)
		case "/api/shows/1/episodesbydate":
			if request.URL.Query().Get("date") != "2026-08-02" {
				http.Error(writer, "bad date", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `[`+episodeFixture+`]`)
		case "/api/shows/1/seasons":
			writeJSON(writer, http.StatusOK, `[{"id":20,"url":"season","number":1,"name":"Season 1","episodeOrder":9,"premiereDate":"2022-02-18","endDate":null,"network":null,"webChannel":null,"image":null,"summary":null,"_links":{"self":{"href":"season"}}}]`)
		case "/api/seasons/20/episodes":
			writeJSON(writer, http.StatusOK, `[`+episodeFixture+`]`)
		case "/api/shows/1/cast":
			writeJSON(writer, http.StatusOK, `[{"person":{"id":30,"name":"Adam Scott"},"character":{"id":31,"name":"Mark Scout"},"self":false,"voice":false}]`)
		case "/api/shows/1/crew":
			writeJSON(writer, http.StatusOK, `[{"type":"Creator","person":{"id":32,"name":"Dan Erickson"}}]`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server)
	ctx := context.Background()

	search, err := client.SearchShows(ctx, "Severance", socialhub.WithRequestID("request-1"))
	if err != nil || len(search) != 1 || search[0].Show.ID != 1 || search[0].Show.Language == nil || *search[0].Show.Language != "English" ||
		search[0].Show.Runtime == nil || search[0].Show.Externals.IMDB == nil || search[0].Show.WebChannel == nil {
		t.Fatalf("search=%#v err=%v", search, err)
	}
	show, err := client.GetShow(ctx, 1)
	if err != nil || show.Name != "Severance" || show.Rating.Average == nil || show.Ended != nil || show.Network != nil {
		t.Fatalf("show=%#v err=%v", show, err)
	}
	lookedUp, err := client.LookupShow(ctx, LookupShowRequest{IMDB: "tt11280740"})
	if err != nil || lookedUp.ID != 1 {
		t.Fatalf("lookup=%#v err=%v", lookedUp, err)
	}
	episodes, err := client.ListEpisodes(ctx, 1, true)
	if err != nil || len(episodes) != 1 || episodes[0].Season == nil || *episodes[0].Season != 1 {
		t.Fatalf("episodes=%#v err=%v", episodes, err)
	}
	if _, err := client.ListEpisodes(ctx, 1, false); err != nil {
		t.Fatal(err)
	}
	episode, err := client.GetEpisode(ctx, 10)
	if err != nil || episode.Number == nil || *episode.Number != 1 {
		t.Fatalf("episode=%#v err=%v", episode, err)
	}
	if episode, err = client.GetEpisodeByNumber(ctx, 1, 1, 1); err != nil || episode.ID != 10 {
		t.Fatalf("episode by number=%#v err=%v", episode, err)
	}
	date := time.Date(2026, time.August, 2, 23, 30, 0, 0, time.FixedZone("test", 8*60*60))
	if episodes, err = client.ListEpisodesByDate(ctx, 1, date); err != nil || len(episodes) != 1 {
		t.Fatalf("episodes by date=%#v err=%v", episodes, err)
	}
	seasons, err := client.ListSeasons(ctx, 1)
	if err != nil || len(seasons) != 1 || seasons[0].EpisodeOrder == nil || *seasons[0].EpisodeOrder != 9 {
		t.Fatalf("seasons=%#v err=%v", seasons, err)
	}
	if episodes, err = client.ListSeasonEpisodes(ctx, 20); err != nil || len(episodes) != 1 {
		t.Fatalf("season episodes=%#v err=%v", episodes, err)
	}
	cast, err := client.ListCast(ctx, 1)
	if err != nil || len(cast) != 1 || cast[0].Character.Name != "Mark Scout" {
		t.Fatalf("cast=%#v err=%v", cast, err)
	}
	crew, err := client.ListCrew(ctx, 1)
	if err != nil || len(crew) != 1 || crew[0].Type != "Creator" {
		t.Fatalf("crew=%#v err=%v", crew, err)
	}
}

func TestLookupRejectsUnexpectedRedirectsAndResponses(t *testing.T) {
	var targetCalls atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		targetCalls.Add(1)
		writeJSON(writer, http.StatusOK, showFixture)
	}))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.URL.Query().Has("imdb"):
			writer.Header().Set("Location", target.URL+"/shows/1")
			writer.WriteHeader(http.StatusMovedPermanently)
		case request.URL.Query().Has("thetvdb"):
			writeJSON(writer, http.StatusOK, `{}`)
		default:
			writeJSON(writer, http.StatusNotFound, `null`)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server)

	if _, err := client.LookupShow(context.Background(), LookupShowRequest{IMDB: "tt1"}); err == nil {
		t.Fatal("expected cross-origin redirect rejection")
	}
	if targetCalls.Load() != 0 {
		t.Fatal("redirect target was followed")
	}
	if _, err := client.LookupShow(context.Background(), LookupShowRequest{TVDB: 1}); err == nil {
		t.Fatal("expected missing documented redirect error")
	}
	if _, err := client.LookupShow(context.Background(), LookupShowRequest{TVRage: 1}); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("expected lookup not found, got %v", err)
	}

	base := server.URL + "/api"
	invalidLocations := []string{
		"/api/shows/1",
		target.URL + "/api/shows/1",
		base + "/shows/1?x=1",
		base + "/shows/1#fragment",
		base + "/shows/%31",
		base + "/shows/01",
		base + "/shows/1/more",
		server.URL + "/other/shows/1",
	}
	for _, location := range invalidLocations {
		if _, err := client.showIDFromLocation(location); err == nil {
			t.Fatalf("accepted invalid location %q", location)
		}
	}
}

func TestLookupRejectsRedirectErrorPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Location", "https://api.tvmaze.com/shows/1")
		writeJSON(writer, http.StatusMovedPermanently, `{"name":"Unexpected redirect payload"}`)
	}))
	defer server.Close()
	_, client := newTestClient(t, server)
	if _, err := client.LookupShow(context.Background(), LookupShowRequest{IMDB: "tt1"}); err == nil {
		t.Fatal("expected redirect payload rejection")
	}
}

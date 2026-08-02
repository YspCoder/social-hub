package trakt

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestCatalogAndUserWorkflows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("trakt-api-key") != testClientID || request.Header.Get("trakt-api-version") != apiVersion ||
			request.Header.Get("Content-Type") != "application/json" || request.Header.Get("Authorization") != "Bearer "+testAccessToken {
			http.Error(writer, "bad headers", http.StatusBadRequest)
			return
		}
		setPagination(writer, 2, 3)
		switch request.URL.Path {
		case "/search/movie,show":
			if request.URL.Query().Get("query") != "Tron" || request.URL.Query().Get("fields") != "title" {
				http.Error(writer, "bad search", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `[{"score":42.5,"type":"movie","movie":{"title":"TRON: Legacy","year":2010,"ids":{"trakt":12601,"slug":"tron-legacy-2010","imdb":"tt1104001","tmdb":20526}}}]`)
		case "/movies/tron-legacy-2010":
			writeJSON(writer, http.StatusOK, movieJSON("TRON: Legacy", 12601))
		case "/shows/tron-uprising":
			writeJSON(writer, http.StatusOK, showJSON("Tron: Uprising", 34209))
		case "/shows/tron-uprising/seasons/1/episodes/2":
			writeJSON(writer, http.StatusOK, episodeJSON("The Renegade", 793693))
		case "/movies/trending":
			writeJSON(writer, http.StatusOK, `[{"watchers":100,"movie":`+movieJSON("TRON: Legacy", 12601)+`}]`)
		case "/movies/popular":
			writeJSON(writer, http.StatusOK, `[`+movieJSON("TRON: Legacy", 12601)+`]`)
		case "/shows/trending":
			writeJSON(writer, http.StatusOK, `[{"watchers":50,"show":`+showJSON("Tron: Uprising", 34209)+`}]`)
		case "/shows/popular":
			writeJSON(writer, http.StatusOK, `[`+showJSON("Tron: Uprising", 34209)+`]`)
		case "/users/test-user":
			writeJSON(writer, http.StatusOK, profileJSON())
		case "/users/settings":
			writeJSON(writer, http.StatusOK, `{"user":`+profileJSON()+`,"permissions":{"commenting":true,"liking":true,"following":true},"account":{"timezone":"Asia/Shanghai","date_format":"mdy","time_24hr":true},"limits":{"list":{"count":2,"item_count":100},"watchlist":{"item_count":500},"favorites":{"item_count":50}}}`)
		case "/users/test-user/history/episodes":
			writeJSON(writer, http.StatusOK, `[{"id":9007199254740991,"watched_at":"2026-08-01T01:02:03Z","action":"scrobble","type":"episode","episode":`+episodeJSON("The Renegade", 793693)+`,"show":`+showJSON("Tron: Uprising", 34209)+`}]`)
		case "/users/test-user/watchlist/movies/rank":
			writeJSON(writer, http.StatusOK, `[{"rank":1,"id":10,"listed_at":"2026-07-01T01:02:03Z","type":"movie","movie":`+movieJSON("TRON: Legacy", 12601)+`}]`)
		case "/users/test-user/ratings/movies/10":
			writeJSON(writer, http.StatusOK, `[{"rated_at":"2026-07-02T01:02:03Z","rating":10,"type":"movie","movie":`+movieJSON("TRON: Legacy", 12601)+`}]`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, true, true)

	search, err := client.Search(context.Background(), SearchRequest{
		Query: "Tron", Types: []MediaType{MediaMovie, MediaShow, MediaMovie}, Fields: []string{"title"}, Cursor: "2", MaxResults: 1, Extended: "full",
	})
	if err != nil || len(search.Items) != 1 || search.Items[0].Movie == nil || search.Items[0].Movie.IDs.Trakt != 12601 || search.NextCursor == nil || search.PrevCursor == nil {
		t.Fatalf("search=%#v err=%v", search, err)
	}
	movie, err := client.GetMovie(context.Background(), "tron-legacy-2010", "full")
	if err != nil || movie.Title != "TRON: Legacy" || movie.Runtime != 125 || len(movie.Images.Poster) != 1 {
		t.Fatalf("movie=%#v err=%v", movie, err)
	}
	show, err := client.GetShow(context.Background(), "tron-uprising", "full")
	if err != nil || show.Title != "Tron: Uprising" || show.AiredEpisodes != 19 || show.FirstAired == nil {
		t.Fatalf("show=%#v err=%v", show, err)
	}
	episode, err := client.GetEpisode(context.Background(), "tron-uprising", 1, 2, "full")
	if err != nil || episode.Title != "The Renegade" || episode.Season != 1 || episode.Number != 2 {
		t.Fatalf("episode=%#v err=%v", episode, err)
	}
	trendingMovies, err := client.TrendingMovies(context.Background(), PageRequest{Cursor: "2", MaxResults: 1, Extended: "full"})
	if err != nil || trendingMovies.Items[0].Watchers != 100 {
		t.Fatalf("trending movies=%#v err=%v", trendingMovies, err)
	}
	popularMovies, err := client.PopularMovies(context.Background(), PageRequest{Cursor: "2", MaxResults: 1})
	if err != nil || popularMovies.Items[0].Title != "TRON: Legacy" {
		t.Fatalf("popular movies=%#v err=%v", popularMovies, err)
	}
	trendingShows, err := client.TrendingShows(context.Background(), PageRequest{Cursor: "2", MaxResults: 1})
	if err != nil || trendingShows.Items[0].Watchers != 50 {
		t.Fatalf("trending shows=%#v err=%v", trendingShows, err)
	}
	popularShows, err := client.PopularShows(context.Background(), PageRequest{Cursor: "2", MaxResults: 1})
	if err != nil || popularShows.Items[0].Title != "Tron: Uprising" {
		t.Fatalf("popular shows=%#v err=%v", popularShows, err)
	}
	profile, err := client.GetProfile(context.Background(), "", "full")
	if err != nil || profile.Username != "test-user" || profile.JoinedAt == nil {
		t.Fatalf("profile=%#v err=%v", profile, err)
	}
	settings, err := client.GetSettings(context.Background())
	if err != nil || !settings.Permissions.Commenting || settings.Limits == nil || settings.Limits.Watchlist.ItemCount != 500 {
		t.Fatalf("settings=%#v err=%v", settings, err)
	}
	history, err := client.ListHistory(context.Background(), HistoryRequest{
		Type: MediaEpisode, StartAt: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), EndAt: testNow, Cursor: "2", MaxResults: 1, Extended: "full",
	})
	if err != nil || len(history.Items) != 1 || history.Items[0].ID != 9007199254740991 || history.Items[0].Episode == nil {
		t.Fatalf("history=%#v err=%v", history, err)
	}
	watchlist, err := client.ListWatchlist(context.Background(), WatchlistRequest{Type: MediaMovie, Cursor: "2", MaxResults: 1})
	if err != nil || len(watchlist.Items) != 1 || watchlist.Items[0].Movie == nil {
		t.Fatalf("watchlist=%#v err=%v", watchlist, err)
	}
	ratings, err := client.ListRatings(context.Background(), RatingsRequest{Type: MediaMovie, Rating: 10, Cursor: "2", MaxResults: 1})
	if err != nil || len(ratings.Items) != 1 || ratings.Items[0].Rating != 10 {
		t.Fatalf("ratings=%#v err=%v", ratings, err)
	}
}

func TestPublicCatalogDoesNotSendBearer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "" || request.Header.Get("trakt-api-key") != testClientID {
			http.Error(writer, "unexpected auth", http.StatusBadRequest)
			return
		}
		writeJSON(writer, http.StatusOK, `[]`)
	}))
	defer server.Close()
	_, client := newTestClient(t, server, false, false)
	if _, err := client.Search(context.Background(), SearchRequest{Query: "Tron", Types: []MediaType{MediaMovie}}); err != nil {
		t.Fatal(err)
	}
}

func TestCatalogAndUserValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, true, true)
	tests := []struct {
		name string
		call func() error
	}{
		{"search query", func() error { _, err := client.Search(context.Background(), SearchRequest{}); return err }},
		{"search type", func() error {
			_, err := client.Search(context.Background(), SearchRequest{Query: "x", Types: []MediaType{"audio"}})
			return err
		}},
		{"search field", func() error {
			_, err := client.Search(context.Background(), SearchRequest{Query: "x", Types: []MediaType{MediaMovie}, Fields: []string{"bad/field"}})
			return err
		}},
		{"movie", func() error { _, err := client.GetMovie(context.Background(), "", ""); return err }},
		{"show", func() error { _, err := client.GetShow(context.Background(), "id", "everything"); return err }},
		{"episode", func() error { _, err := client.GetEpisode(context.Background(), "show", -1, 0, ""); return err }},
		{"catalog page", func() error {
			_, err := client.TrendingMovies(context.Background(), PageRequest{MaxResults: 101})
			return err
		}},
		{"profile", func() error { _, err := client.GetProfile(context.Background(), "bad/name", ""); return err }},
		{"history range", func() error {
			_, err := client.ListHistory(context.Background(), HistoryRequest{StartAt: testNow, EndAt: testNow.Add(-time.Hour)})
			return err
		}},
		{"history type", func() error {
			_, err := client.ListHistory(context.Background(), HistoryRequest{Type: MediaPerson})
			return err
		}},
		{"watchlist type", func() error {
			_, err := client.ListWatchlist(context.Background(), WatchlistRequest{Type: MediaEpisode})
			return err
		}},
		{"watchlist sort", func() error {
			_, err := client.ListWatchlist(context.Background(), WatchlistRequest{Type: MediaMovie, Sort: "random"})
			return err
		}},
		{"rating", func() error {
			_, err := client.ListRatings(context.Background(), RatingsRequest{Type: MediaMovie, Rating: 11})
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func setPagination(writer http.ResponseWriter, page, pageCount int) {
	writer.Header().Set("X-Pagination-Page", strconv.Itoa(page))
	writer.Header().Set("X-Pagination-Page-Count", strconv.Itoa(pageCount))
}

func movieJSON(title string, id int64) string {
	return `{"title":"` + title + `","year":2010,"ids":{"trakt":` + intString(id) + `,"slug":"slug","imdb":"tt1104001","tmdb":20526},"images":{"poster":["poster.webp"]},"overview":"overview","runtime":125,"rating":8.5,"updated_at":"2026-07-01T00:00:00Z"}`
}

func showJSON(title string, id int64) string {
	return `{"title":"` + title + `","year":2012,"ids":{"trakt":` + intString(id) + `,"slug":"slug","tvdb":258480},"aired_episodes":19,"first_aired":"2012-06-07T00:00:00Z","runtime":30,"rating":8.1}`
}

func episodeJSON(title string, id int64) string {
	return `{"season":1,"number":2,"title":"` + title + `","first_aired":"2012-06-08T00:00:00Z","runtime":22,"ids":{"trakt":` + intString(id) + `,"tvdb":4318713}}`
}

func profileJSON() string {
	return `{"username":"test-user","private":false,"deleted":false,"name":"Test User","vip":true,"director":false,"ids":{"trakt":1,"slug":"test-user"},"joined_at":"2020-01-01T00:00:00Z","location":"Shanghai"}`
}

func intString(value int64) string {
	return strconv.FormatInt(value, 10)
}

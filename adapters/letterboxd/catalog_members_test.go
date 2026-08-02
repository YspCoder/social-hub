package letterboxd

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestCatalogAndMemberWorkflows(t *testing.T) {
	seen := make([]string, 0, 7)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer "+testAccessToken || request.Header.Get("User-Agent") != "social-hub-letterboxd-tests/1.0" {
			http.Error(writer, "bad auth", http.StatusUnauthorized)
			return
		}
		seen = append(seen, request.Method+" "+request.URL.Path)
		switch request.URL.Path {
		case "/search":
			query := request.URL.Query()
			if query.Get("input") != "Blade Runner" || query.Get("searchMethod") != "FullText" ||
				!slices.Equal(query["include"], []string{"FilmSearchItem", "MemberSearchItem"}) ||
				query.Get("cursor") != "search-cursor" || query.Get("perPage") != "25" {
				http.Error(writer, "bad search query", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"next":"next-search","items":[{"type":"FilmSearchItem","score":0.98,"film":{"id":"film-1","name":"Blade Runner","releaseYear":1982}}]}`)
		case "/film/tmdb:78":
			writeJSON(writer, http.StatusOK, `{"id":"film-1","name":"Blade Runner","fullDisplayName":"Blade Runner (1982)","releaseYear":1982,"runTime":118,"rating":4.2}`)
		case "/films":
			query := request.URL.Query()
			if !slices.Equal(query["filmId"], []string{"film-1", "imdb:tt0083658"}) || query.Get("genre") != "sci-fi" ||
				query.Get("country") != "US" || query.Get("language") != "en" || query.Get("decade") != "1980" ||
				query.Get("year") != "1982" || query.Get("sort") != "FilmName" || query.Get("perPage") != "10" {
				http.Error(writer, "bad films query", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"items":[{"id":"film-1","name":"Blade Runner","releaseYear":1982}]}`)
		case "/member/member-1":
			writeJSON(writer, http.StatusOK, memberJSON("member-1", "deckard"))
		case "/me":
			writeJSON(writer, http.StatusOK, `{"id":"member-me","username":"rachel","displayName":"Rachel","emailAddress":"rachel@example.test","privateAccount":true}`)
		case "/member/member-1/activity":
			if request.URL.Query().Get("cursor") != "activity-cursor" || request.URL.Query().Get("perPage") != "5" {
				http.Error(writer, "bad activity query", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"next":"activity-next","items":[{"type":"FilmWatchActivity","member":{"id":"member-1","username":"deckard"},"whenCreated":"2026-08-02T08:00:00Z","film":{"id":"film-1","name":"Blade Runner"}}]}`)
		case "/member/member-1/watchlist":
			if request.URL.Query().Get("genre") != "sci-fi" || request.URL.Query().Get("cursor") != "watch-cursor" {
				http.Error(writer, "bad watchlist query", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"next":"watch-next","items":[{"id":"film-2","name":"Arrival","releaseYear":2016}]}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, TokenUser, true, []string{"content:modify"})

	search, err := client.Search(context.Background(), SearchRequest{
		Input: "Blade Runner", Method: "FullText", IncludeTypes: []string{"FilmSearchItem", "MemberSearchItem"},
		Cursor: "search-cursor", PerPage: 25,
	})
	if err != nil || len(search.Items) != 1 || search.Items[0].Film == nil || search.Items[0].Film.ID != "film-1" ||
		!search.HasMore || search.NextCursor == nil || *search.NextCursor != "next-search" {
		t.Fatalf("search=%#v err=%v", search, err)
	}
	film, err := client.GetFilm(context.Background(), "tmdb:78")
	if err != nil || film.ID != "film-1" || film.RunTime != 118 || film.Rating != 4.2 {
		t.Fatalf("film=%#v err=%v", film, err)
	}
	films, err := client.ListFilms(context.Background(), FilmListRequest{
		FilmIDs: []string{"film-1", "imdb:tt0083658"}, Genre: "sci-fi", Country: "US", Language: "en",
		Decade: 1980, Year: 1982, Sort: "FilmName", PerPage: 10,
	})
	if err != nil || len(films.Items) != 1 || films.Items[0].Name != "Blade Runner" || films.HasMore {
		t.Fatalf("films=%#v err=%v", films, err)
	}
	member, err := client.GetMember(context.Background(), "member-1")
	if err != nil || member.Username != "deckard" || member.Location != "Los Angeles" {
		t.Fatalf("member=%#v err=%v", member, err)
	}
	me, err := client.GetMe(context.Background())
	if err != nil || me.EmailAddress != "rachel@example.test" || !me.PrivateAccount {
		t.Fatalf("me=%#v err=%v", me, err)
	}
	activity, err := client.ListActivity(context.Background(), "member-1", PageRequest{Cursor: "activity-cursor", PerPage: 5})
	if err != nil || len(activity.Items) != 1 || activity.Items[0].Film == nil || !activity.HasMore {
		t.Fatalf("activity=%#v err=%v", activity, err)
	}
	watchlist, err := client.ListWatchlist(context.Background(), "member-1", FilmListRequest{Genre: "sci-fi", Cursor: "watch-cursor"})
	if err != nil || len(watchlist.Items) != 1 || watchlist.Items[0].ID != "film-2" || !watchlist.HasMore {
		t.Fatalf("watchlist=%#v err=%v", watchlist, err)
	}
	if len(seen) != 7 {
		t.Fatalf("requests=%v", seen)
	}
}

func TestCatalogAndMemberValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, TokenUser, true, []string{"content:modify"})
	tests := []func() error{
		func() error { _, err := client.Search(context.Background(), SearchRequest{}); return err },
		func() error {
			_, err := client.Search(context.Background(), SearchRequest{Input: "x", Method: "Fuzzy"})
			return err
		},
		func() error {
			_, err := client.Search(context.Background(), SearchRequest{Input: "x", IncludeTypes: []string{"FilmSearchItem", "FilmSearchItem"}})
			return err
		},
		func() error {
			_, err := client.Search(context.Background(), SearchRequest{Input: "x", PerPage: 101})
			return err
		},
		func() error { _, err := client.GetFilm(context.Background(), "bad/id"); return err },
		func() error {
			_, err := client.ListFilms(context.Background(), FilmListRequest{Decade: 1982})
			return err
		},
		func() error {
			_, err := client.ListFilms(context.Background(), FilmListRequest{Year: 1800})
			return err
		},
		func() error {
			_, err := client.ListFilms(context.Background(), FilmListRequest{FilmIDs: []string{"bad/id"}})
			return err
		},
		func() error { _, err := client.GetMember(context.Background(), ""); return err },
		func() error {
			_, err := client.ListActivity(context.Background(), "member-1", PageRequest{PerPage: -1})
			return err
		},
		func() error {
			_, err := client.ListWatchlist(context.Background(), "bad/id", FilmListRequest{})
			return err
		},
	}
	for _, call := range tests {
		if err := call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("error=%v", err)
		}
	}
}

func memberJSON(id, username string) string {
	return `{"id":"` + id + `","username":"` + username + `","displayName":"Deckard","location":"Los Angeles","bio":"Replicant hunter","favoriteFilms":[{"id":"film-1","name":"Blade Runner"}]}`
}

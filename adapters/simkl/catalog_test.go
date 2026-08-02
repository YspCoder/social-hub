package simkl

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestCatalogAndTrendingWorkflows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("client_id") != "client-id" || request.URL.Query().Get("app-name") != "social-hub-tests" ||
			request.URL.Query().Get("app-version") != "1.2.3" || request.Header.Get("User-Agent") != "social-hub-simkl-tests/1.0" {
			t.Errorf("missing Simkl application identification: %s", request.URL.String())
		}
		if request.Header.Get("Authorization") != "" {
			t.Error("public catalog/CDN request must not carry a bearer token")
		}
		switch request.URL.Path {
		case "/api/search/movie":
			if request.URL.Query().Get("q") != "Dune" || request.URL.Query().Get("page") != "2" ||
				request.URL.Query().Get("limit") != "1" || request.URL.Query().Get("extended") != "full" {
				t.Errorf("unexpected search query: %s", request.URL.RawQuery)
			}
			writer.Header().Set("X-Pagination-Page", "2")
			writer.Header().Set("X-Pagination-Page-Count", "3")
			_, _ = writer.Write([]byte(`[{"title":"Dune","year":2021,"endpoint_type":"movies","poster":null,"ids":{"simkl_id":101,"slug":"dune","tmdb":"438631"},"overview":"Arrakis"}]`))
		case "/api/movies/101":
			_, _ = writer.Write([]byte(`{"title":"Dune","year":2021,"type":"movie","ids":{"simkl":101,"imdb":"tt1160419"},"runtime":155,"released":"2021-10-22"}`))
		case "/api/tv/202":
			_, _ = writer.Write([]byte(`{"title":"Silo","year":2023,"type":"show","ids":{"simkl":202},"network":"Apple TV","total_episodes":30}`))
		case "/api/anime/303":
			_, _ = writer.Write([]byte(`{"title":"Cowboy Bebop","en_title":"Cowboy Bebop","year":1998,"type":"anime","anime_type":"tv","ids":{"simkl":303,"mal":"1"},"mapped_tvdb_seasons":[1]}`))
		case "/cdn/discover/trending/movies/week_500.json":
			_, _ = writer.Write([]byte(`[{"title":"Dune","url":"/movies/101/dune","rank":1,"ids":{"simkl_id":101,"slug":"dune"},"watched":100}]`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, true, true)
	page, err := client.Search(context.Background(), SearchRequest{
		Type: MediaMovie, Query: "Dune", Cursor: "2", Limit: 1, Extended: SearchFull,
	})
	if err != nil || len(page.Items) != 1 || page.Items[0].IDs.Simkl != 101 || !page.HasMore ||
		page.NextCursor == nil || *page.NextCursor != "3" || page.PrevCursor == nil || *page.PrevCursor != "1" {
		t.Fatalf("unexpected search page: %#v, %v", page, err)
	}
	movie, err := client.GetMovie(context.Background(), 101)
	if err != nil || movie.IDs.IMDB != "tt1160419" || movie.Runtime == nil || *movie.Runtime != 155 {
		t.Fatalf("unexpected movie: %#v, %v", movie, err)
	}
	tv, err := client.GetTV(context.Background(), 202)
	if err != nil || tv.Network == nil || *tv.Network != "Apple TV" || tv.TotalEpisodes == nil || *tv.TotalEpisodes != 30 {
		t.Fatalf("unexpected TV detail: %#v, %v", tv, err)
	}
	anime, err := client.GetAnime(context.Background(), 303)
	if err != nil || anime.AnimeType != "tv" || anime.IDs.MAL != "1" || len(anime.MappedTVDBSeasons) != 1 {
		t.Fatalf("unexpected anime detail: %#v, %v", anime, err)
	}
	trending, err := client.ListTrending(context.Background(), TrendingRequest{Type: MediaMovie, Period: TrendingWeek, Limit: 500})
	if err != nil || len(trending) != 1 || trending[0].IDs.Simkl != 101 || trending[0].Watched != 100 {
		t.Fatalf("unexpected trending: %#v, %v", trending, err)
	}
}

func TestCatalogValidationAndStructuredIDs(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, true, true)
	tests := []func() error{
		func() error {
			_, err := client.Search(context.Background(), SearchRequest{Type: MediaMovie})
			return err
		},
		func() error {
			_, err := client.Search(context.Background(), SearchRequest{Type: "bad", Query: "Dune"})
			return err
		},
		func() error {
			_, err := client.Search(context.Background(), SearchRequest{Type: MediaMovie, Query: "Dune", Cursor: "0"})
			return err
		},
		func() error {
			_, err := client.Search(context.Background(), SearchRequest{Type: MediaMovie, Query: "Dune", Cursor: "21"})
			return err
		},
		func() error {
			_, err := client.Search(context.Background(), SearchRequest{Type: MediaMovie, Query: "Dune", Limit: 51})
			return err
		},
		func() error {
			_, err := client.Search(context.Background(), SearchRequest{Type: MediaMovie, Query: "Dune", Extended: "raw"})
			return err
		},
		func() error { _, err := client.GetMovie(context.Background(), 0); return err },
		func() error {
			_, err := client.ListTrending(context.Background(), TrendingRequest{Type: MediaMovie, Period: TrendingToday, Limit: 50})
			return err
		},
		func() error {
			_, err := client.Search(context.Background(), SearchRequest{Type: MediaMovie, Query: "Dune"}, socialhub.WithFields("title"))
			return err
		},
		func() error {
			_, err := client.GetMovie(context.Background(), 1, socialhub.WithIdempotencyKey("unsupported"))
			return err
		},
	}
	for index, test := range tests {
		if err := test(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("case %d: expected invalid argument, got %v", index, err)
		}
	}
	var ids IDs
	if err := json.Unmarshal([]byte(`{"simkl_id":42,"imdb":"tt1"}`), &ids); err != nil || ids.Simkl != 42 || ids.IMDB != "tt1" {
		t.Fatalf("unexpected normalized IDs: %#v, %v", ids, err)
	}
	if err := json.Unmarshal([]byte(`{"simkl":1,"simkl_id":2}`), &ids); err == nil {
		t.Fatal("expected conflicting Simkl IDs to fail")
	}
}

func TestMalformedCatalogResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"not":"an array"}`))
	}))
	defer server.Close()
	_, client := newTestClient(t, server, false, false)
	_, err := client.Search(context.Background(), SearchRequest{Type: MediaMovie, Query: "Dune"})
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodePlatformError {
		t.Fatalf("unexpected malformed-response error: %#v", platformErr)
	}
}

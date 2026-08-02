package tmdb

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestCatalogWorkflows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer "+testBearerToken || request.URL.Query().Get("api_key") != "" {
			http.Error(writer, "bad auth", http.StatusUnauthorized)
			return
		}
		if request.URL.Query().Get("session_id") != "" || request.URL.Query().Get("guest_session_id") != "" {
			http.Error(writer, "session leaked", http.StatusBadRequest)
			return
		}
		switch request.URL.Path {
		case "/search/multi":
			if request.URL.Query().Get("query") != "TRON" || request.URL.Query().Get("include_adult") != "true" || request.URL.Query().Get("language") != "zh-CN" || request.URL.Query().Get("page") != "2" {
				http.Error(writer, "bad search", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, pageJSON(movieItemJSON(603, "The Matrix"), 2, 3))
		case "/movie/603":
			writeJSON(writer, http.StatusOK, movieJSON())
		case "/tv/1399":
			writeJSON(writer, http.StatusOK, tvJSON())
		case "/person/6384":
			writeJSON(writer, http.StatusOK, personJSON())
		case "/trending/all/week":
			writeJSON(writer, http.StatusOK, pageJSON(`{"id":6384,"media_type":"person","name":"Keanu Reeves","profile_path":"/person.jpg","popularity":50}`, 2, 3))
		case "/movie/popular":
			writeJSON(writer, http.StatusOK, pageJSON(movieItemJSON(603, "The Matrix"), 2, 3))
		case "/tv/popular":
			writeJSON(writer, http.StatusOK, pageJSON(`{"id":1399,"name":"Game of Thrones","first_air_date":"2011-04-17","vote_average":8.5}`, 2, 3))
		case "/configuration":
			writeJSON(writer, http.StatusOK, `{"images":{"base_url":"http://image.tmdb.org/t/p/","secure_base_url":"https://image.tmdb.org/t/p/","backdrop_sizes":["w300"],"logo_sizes":["w45"],"poster_sizes":["w500"],"profile_sizes":["h632"],"still_sizes":["w300"]},"change_keys":["adult","title"]}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, true, true, true)

	search, err := client.Search(context.Background(), SearchRequest{Query: "TRON", IncludeAdult: true, Language: "zh-CN", Cursor: "2"})
	if err != nil || len(search.Items) != 1 || search.Items[0].ID != 603 || search.NextCursor == nil || search.PrevCursor == nil {
		t.Fatalf("search=%#v err=%v", search, err)
	}
	movie, err := client.GetMovie(context.Background(), 603, "en-US")
	if err != nil || movie.Title != "The Matrix" || movie.IMDBID != "tt0133093" || movie.BelongsToCollection == nil || movie.Runtime != 136 {
		t.Fatalf("movie=%#v err=%v", movie, err)
	}
	series, err := client.GetTVSeries(context.Background(), 1399, "en-US")
	if err != nil || series.Name != "Game of Thrones" || series.NumberOfSeasons != 8 || len(series.Seasons) != 1 {
		t.Fatalf("series=%#v err=%v", series, err)
	}
	person, err := client.GetPerson(context.Background(), 6384, "en-US")
	if err != nil || person.Name != "Keanu Reeves" || person.IMDBID != "nm0000206" {
		t.Fatalf("person=%#v err=%v", person, err)
	}
	trending, err := client.Trending(context.Background(), TrendingRequest{MediaType: MediaAll, Window: "week", Language: "en-US", Cursor: "2"})
	if err != nil || trending.Items[0].MediaType != MediaPerson {
		t.Fatalf("trending=%#v err=%v", trending, err)
	}
	popularMovies, err := client.PopularMovies(context.Background(), PageRequest{Language: "en-US", Cursor: "2"})
	if err != nil || popularMovies.Items[0].MediaType != MediaMovie {
		t.Fatalf("popular movies=%#v err=%v", popularMovies, err)
	}
	popularTV, err := client.PopularTV(context.Background(), PageRequest{Language: "en-US", Cursor: "2"})
	if err != nil || popularTV.Items[0].MediaType != MediaTV {
		t.Fatalf("popular TV=%#v err=%v", popularTV, err)
	}
	configuration, err := client.GetConfiguration(context.Background())
	if err != nil || configuration.Images.SecureBaseURL != "https://image.tmdb.org/t/p/" || len(configuration.ChangeKeys) != 2 {
		t.Fatalf("configuration=%#v err=%v", configuration, err)
	}
}

func TestLegacyAPIKeyAuthentication(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "" || request.URL.Query().Get("api_key") != testAPIKey {
			http.Error(writer, "bad legacy auth", http.StatusUnauthorized)
			return
		}
		writeJSON(writer, http.StatusOK, pageJSON(movieItemJSON(603, "The Matrix"), 1, 1))
	}))
	defer server.Close()
	_, client := newTestClient(t, server, false, false, false)
	page, err := client.Search(context.Background(), SearchRequest{Query: "Matrix"})
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("page=%#v err=%v", page, err)
	}
}

func TestCatalogValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, true, false, false)
	tests := []func() error{
		func() error { _, err := client.Search(context.Background(), SearchRequest{}); return err },
		func() error {
			_, err := client.Search(context.Background(), SearchRequest{Query: "x", Language: "bad locale"})
			return err
		},
		func() error {
			_, err := client.Search(context.Background(), SearchRequest{Query: "x", Cursor: "501"})
			return err
		},
		func() error { _, err := client.GetMovie(context.Background(), 0, ""); return err },
		func() error { _, err := client.GetTVSeries(context.Background(), -1, ""); return err },
		func() error { _, err := client.GetPerson(context.Background(), 1, "bad locale"); return err },
		func() error {
			_, err := client.Trending(context.Background(), TrendingRequest{MediaType: MediaMovie, Window: "month"})
			return err
		},
		func() error {
			_, err := client.Trending(context.Background(), TrendingRequest{MediaType: "audio", Window: "day"})
			return err
		},
		func() error {
			_, err := client.PopularMovies(context.Background(), PageRequest{Language: "bad locale"})
			return err
		},
	}
	for _, call := range tests {
		if err := call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("error=%v", err)
		}
	}
}

func pageJSON(item string, page, totalPages int) string {
	return `{"page":` + intString(int64(page)) + `,"results":[` + item + `],"total_pages":` + intString(int64(totalPages)) + `,"total_results":21}`
}

func movieItemJSON(id int64, title string) string {
	return `{"id":` + intString(id) + `,"media_type":"movie","title":"` + title + `","original_title":"` + title + `","original_language":"en","overview":"overview","release_date":"1999-03-31","poster_path":"/poster.jpg","backdrop_path":"/backdrop.jpg","popularity":42.5,"genre_ids":[28,878],"adult":false,"video":false,"vote_average":8.2,"vote_count":100}`
}

func movieJSON() string {
	return `{"adult":false,"backdrop_path":"/backdrop.jpg","belongs_to_collection":{"id":2344,"name":"Matrix Collection","poster_path":"/collection.jpg"},"budget":63000000,"genres":[{"id":28,"name":"Action"}],"homepage":"https://example.test","id":603,"imdb_id":"tt0133093","original_language":"en","original_title":"The Matrix","overview":"overview","popularity":42.5,"poster_path":"/poster.jpg","origin_country":["US"],"production_companies":[{"id":79,"name":"Village Roadshow Pictures","origin_country":"US"}],"production_countries":[{"iso_3166_1":"US","name":"United States"}],"release_date":"1999-03-31","revenue":463517383,"runtime":136,"spoken_languages":[{"iso_639_1":"en","english_name":"English","name":"English"}],"status":"Released","tagline":"Welcome","title":"The Matrix","vote_average":8.2,"vote_count":100}`
}

func tvJSON() string {
	return `{"adult":false,"backdrop_path":"/tv.jpg","first_air_date":"2011-04-17","genres":[{"id":18,"name":"Drama"}],"homepage":"https://example.test","id":1399,"in_production":false,"languages":["en"],"last_air_date":"2019-05-19","name":"Game of Thrones","networks":[{"id":49,"name":"HBO","origin_country":"US"}],"number_of_episodes":73,"number_of_seasons":8,"origin_country":["US"],"original_language":"en","original_name":"Game of Thrones","overview":"overview","popularity":90,"poster_path":"/poster.jpg","seasons":[{"air_date":"2011-04-17","episode_count":10,"id":3624,"name":"Season 1","season_number":1}],"status":"Ended","tagline":"Winter is coming","type":"Scripted","vote_average":8.5,"vote_count":200}`
}

func personJSON() string {
	return `{"adult":false,"also_known_as":["K Reeves"],"biography":"biography","birthday":"1964-09-02","gender":2,"homepage":"https://example.test","id":6384,"imdb_id":"nm0000206","known_for_department":"Acting","name":"Keanu Reeves","place_of_birth":"Beirut","popularity":50,"profile_path":"/person.jpg"}`
}

func intString(value int64) string { return strconv.FormatInt(value, 10) }

package tmdb

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestAccountLibraryAndActions(t *testing.T) {
	calls := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer "+testBearerToken || request.URL.Query().Get("session_id") != testSessionID || request.URL.Query().Get("guest_session_id") != "" {
			http.Error(writer, "bad session", http.StatusUnauthorized)
			return
		}
		key := request.Method + " " + request.URL.Path
		calls[key]++
		switch key {
		case "GET /account/42":
			writeJSON(writer, http.StatusOK, `{"avatar":{"gravatar":{"hash":"hash"},"tmdb":{"avatar_path":"/avatar.jpg"}},"id":42,"iso_639_1":"en","iso_3166_1":"US","name":"Viewer","include_adult":false,"username":"viewer"}`)
		case "GET /account/42/favorite/movies":
			writeJSON(writer, http.StatusOK, pageJSON(movieItemJSON(603, "The Matrix"), 2, 3))
		case "GET /account/42/watchlist/tv":
			writeJSON(writer, http.StatusOK, pageJSON(`{"id":1399,"name":"Game of Thrones","first_air_date":"2011-04-17"}`, 2, 3))
		case "GET /account/42/rated/movies":
			writeJSON(writer, http.StatusOK, pageJSON(`{"id":603,"title":"The Matrix","rating":8.5}`, 2, 3))
		case "POST /account/42/favorite":
			if !assertJSONFields(writer, request, map[string]any{"media_type": "movie", "media_id": float64(603), "favorite": true}) {
				return
			}
			writeJSON(writer, http.StatusOK, `{"status_code":1,"status_message":"Success."}`)
		case "POST /account/42/watchlist":
			if !assertJSONFields(writer, request, map[string]any{"media_type": "tv", "media_id": float64(1399), "watchlist": false}) {
				return
			}
			writeJSON(writer, http.StatusCreated, `{"status_code":12,"status_message":"Updated."}`)
		case "POST /movie/603/rating":
			if !assertJSONFields(writer, request, map[string]any{"value": 8.5}) {
				return
			}
			writeJSON(writer, http.StatusCreated, `{"status_code":12,"status_message":"Updated."}`)
		case "DELETE /tv/1399/rating":
			if request.ContentLength > 0 {
				http.Error(writer, "unexpected body", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"status_code":13,"status_message":"Deleted."}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, true, true, true)

	account, err := client.GetAccount(context.Background())
	if err != nil || account.ID != 42 || account.Username != "viewer" || account.Avatar.TMDB.AvatarPath != "/avatar.jpg" {
		t.Fatalf("account=%#v err=%v", account, err)
	}
	favorites, err := client.ListLibrary(context.Background(), LibraryRequest{Kind: LibraryFavorites, MediaType: MediaMovie, Language: "en-US", Sort: "created_at.desc", Cursor: "2"})
	if err != nil || favorites.Items[0].MediaType != MediaMovie || favorites.NextCursor == nil {
		t.Fatalf("favorites=%#v err=%v", favorites, err)
	}
	watchlist, err := client.ListLibrary(context.Background(), LibraryRequest{Kind: LibraryWatchlist, MediaType: MediaTV, Cursor: "2"})
	if err != nil || watchlist.Items[0].MediaType != MediaTV {
		t.Fatalf("watchlist=%#v err=%v", watchlist, err)
	}
	rated, err := client.ListLibrary(context.Background(), LibraryRequest{Kind: LibraryRated, MediaType: MediaMovie, Cursor: "2"})
	if err != nil || rated.Items[0].Rating != 8.5 {
		t.Fatalf("rated=%#v err=%v", rated, err)
	}
	movie := MediaTarget{MediaType: MediaMovie, MediaID: 603}
	series := MediaTarget{MediaType: MediaTV, MediaID: 1399}
	if response, err := client.SetFavorite(context.Background(), movie, true); err != nil || response.StatusCode != 1 {
		t.Fatalf("favorite=%#v err=%v", response, err)
	}
	if response, err := client.SetWatchlist(context.Background(), series, false); err != nil || response.StatusCode != 12 {
		t.Fatalf("watchlist response=%#v err=%v", response, err)
	}
	if response, err := client.SetRating(context.Background(), RatingRequest{Target: movie, Value: 8.5}); err != nil || response.StatusCode != 12 {
		t.Fatalf("rating=%#v err=%v", response, err)
	}
	if response, err := client.DeleteRating(context.Background(), series); err != nil || response.StatusCode != 13 {
		t.Fatalf("delete rating=%#v err=%v", response, err)
	}
	for _, key := range []string{
		"GET /account/42", "GET /account/42/favorite/movies", "GET /account/42/watchlist/tv", "GET /account/42/rated/movies",
		"POST /account/42/favorite", "POST /account/42/watchlist", "POST /movie/603/rating", "DELETE /tv/1399/rating",
	} {
		if calls[key] != 1 {
			t.Fatalf("call %s=%d", key, calls[key])
		}
	}
}

func TestGuestSessionRating(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("guest_session_id") != testGuestSessionID || request.URL.Query().Get("session_id") != "" {
			http.Error(writer, "bad guest session", http.StatusUnauthorized)
			return
		}
		writeJSON(writer, http.StatusOK, `{"status_code":1,"status_message":"Success."}`)
	}))
	defer server.Close()
	_, client := newTestClient(t, server, true, false, true)
	capabilities, _ := client.Capabilities(context.Background())
	if !capabilities.Has(CapabilityRating) || capabilities.Has(CapabilityLibrary) {
		t.Fatalf("capabilities=%#v", capabilities)
	}
	_, err := client.SetRating(context.Background(), RatingRequest{Target: MediaTarget{MediaType: MediaTV, MediaID: 1399}, Value: 7})
	if err != nil {
		t.Fatal(err)
	}
}

func TestLibraryAndActionValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, true, true, true)
	tests := []func() error{
		func() error { _, err := client.ListLibrary(context.Background(), LibraryRequest{}); return err },
		func() error {
			_, err := client.ListLibrary(context.Background(), LibraryRequest{Kind: LibraryFavorites, MediaType: MediaPerson})
			return err
		},
		func() error {
			_, err := client.ListLibrary(context.Background(), LibraryRequest{Kind: LibraryFavorites, MediaType: MediaMovie, Sort: "random"})
			return err
		},
		func() error {
			_, err := client.ListLibrary(context.Background(), LibraryRequest{Kind: LibraryFavorites, MediaType: MediaMovie, Language: "bad locale"})
			return err
		},
		func() error { _, err := client.SetFavorite(context.Background(), MediaTarget{}, true); return err },
		func() error {
			_, err := client.SetWatchlist(context.Background(), MediaTarget{MediaType: MediaPerson, MediaID: 1}, true)
			return err
		},
		func() error {
			_, err := client.SetRating(context.Background(), RatingRequest{Target: MediaTarget{MediaType: MediaMovie, MediaID: 1}, Value: 8.25})
			return err
		},
		func() error {
			_, err := client.DeleteRating(context.Background(), MediaTarget{MediaType: MediaMovie})
			return err
		},
	}
	for _, call := range tests {
		if err := call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("error=%v", err)
		}
	}
}

func assertJSONFields(writer http.ResponseWriter, request *http.Request, expected map[string]any) bool {
	var payload map[string]any
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		http.Error(writer, "bad JSON", http.StatusBadRequest)
		return false
	}
	for key, value := range expected {
		if payload[key] != value {
			http.Error(writer, "bad field", http.StatusBadRequest)
			return false
		}
	}
	return true
}

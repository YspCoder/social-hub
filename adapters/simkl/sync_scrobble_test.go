package simkl

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestUserSyncAndScrobbleWorkflows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer "+testAccessToken || request.URL.Query().Get("client_id") != "client-id" ||
			request.URL.Query().Get("app-name") != "social-hub-tests" {
			t.Errorf("missing user authentication or application metadata: %s", request.URL.String())
		}
		switch request.Method + " " + request.URL.Path {
		case "POST /api/users/settings":
			_, _ = writer.Write([]byte(`{"user":{"name":"Ada","joined_at":"2020-01-01"},"account":{"id":7,"timezone":"Asia/Shanghai","type":"pro"}}`))
		case "GET /api/sync/activities":
			_, _ = writer.Write([]byte(`{"all":"2026-08-02T08:00:00Z","settings":{"all":"2026-08-01T08:00:00Z"},"tv_shows":{"all":"2026-08-02T08:00:00Z","rated_at":null,"playback":null,"plantowatch":null,"watching":null,"completed":null,"hold":null,"dropped":null,"removed_from_list":null},"anime":{"all":null,"rated_at":null,"playback":null,"plantowatch":null,"watching":null,"completed":null,"hold":null,"dropped":null,"removed_from_list":null},"movies":{"all":null,"rated_at":null,"playback":null,"plantowatch":null,"watching":null,"completed":null,"hold":null,"dropped":null,"removed_from_list":null}}`))
		case "GET /api/sync/all-items/all/all":
			query := request.URL.Query()
			if query.Get("date_from") != "2026-07-31T16:00:00Z" || query.Get("extended") != "full" ||
				query.Get("episode_watched_at") != "yes" || query.Get("include_all_episodes") != "original" ||
				query.Get("next_watch_info") != "yes" || query.Get("memos") != "yes" || query.Get("allow_rewatch") != "" {
				t.Errorf("unexpected all-items query: %s", request.URL.RawQuery)
			}
			_, _ = writer.Write([]byte(`{"movies":[{"status":"completed","last_watched_at":"2026-08-01T20:00:00Z","movie":{"title":"Dune","year":2021,"ids":{"simkl":101}}}],"shows":[{"status":"watching","show":{"title":"Silo","ids":{"simkl":202}},"seasons":[{"number":1,"episodes":[{"number":1,"watched_at":"2026-08-01T20:00:00Z","ids":{"tvdb_id":9}}]}]}],"anime":[]}`))
		case "POST /api/sync/add-to-list":
			var body AddToListRequest
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil || body.To != StatusPlanToWatch || len(body.Movies) != 1 {
				t.Errorf("unexpected add-to-list body: %#v, %v", body, err)
			}
			_, _ = writer.Write([]byte(`{"added":{"movies":[{"title":"Dune","year":2021,"to":"plantowatch","ids":{"simkl":101}}]},"not_found":{}}`))
		case "POST /api/sync/history":
			var body map[string]any
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil || body["movies"] == nil || body["shows"] == nil {
				t.Errorf("unexpected history body: %#v, %v", body, err)
			}
			_, _ = writer.Write([]byte(`{"added":{"movies":1,"shows":1,"episodes":2,"statuses":[]},"not_found":{}}`))
		case "POST /api/sync/history/remove":
			_, _ = writer.Write([]byte(`{"deleted":{"movies":1,"shows":0,"episodes":0},"not_found":{"movies":[],"shows":[]}}`))
		case "POST /api/sync/ratings":
			_, _ = writer.Write([]byte(`{"added":{"movies":1,"shows":0,"statuses":[]},"not_found":{}}`))
		case "POST /api/sync/ratings/remove":
			_, _ = writer.Write([]byte(`{"deleted":{"movies":1,"shows":0},"not_found":{"movies":[],"shows":[]}}`))
		case "POST /api/scrobble/start":
			_, _ = writer.Write([]byte(`{"action":"start","progress":25,"sid":"session-1","movie":{"title":"Dune","ids":{"simkl":101}}}`))
		case "POST /api/scrobble/pause":
			_, _ = writer.Write([]byte(`{"action":"pause","progress":40,"sid":"session-1","show":{"ids":{"simkl":202}},"episode":{"season":1,"number":2,"title":"The Engineer"}}`))
		case "POST /api/scrobble/stop":
			_, _ = writer.Write([]byte(`{"action":"scrobble","progress":90,"sid":"session-1","anime":{"ids":{"simkl":303}},"episode":{"season":0,"number":1}}`))
		case "POST /api/scrobble/checkin":
			_, _ = writer.Write([]byte(`{"id":99,"action":"checkin","progress":0,"show":{"ids":{"simkl":202}},"episode":{"season":1,"number":1}}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, true, true)
	ctx := context.Background()
	settings, err := client.GetSettings(ctx)
	if err != nil || settings.User.Name != "Ada" || settings.Account.ID != 7 {
		t.Fatalf("unexpected settings: %#v, %v", settings, err)
	}
	activities, err := client.GetActivities(ctx)
	if err != nil || activities.All == nil || activities.TVShows.All == nil {
		t.Fatalf("unexpected activities: %#v, %v", activities, err)
	}
	dateFrom := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.FixedZone("offset", 8*60*60))
	items, err := client.ListAllItems(ctx, AllItemsRequest{
		DateFrom: &dateFrom, Extended: SyncFull, NextWatchInfo: true,
		EpisodeWatchedAt: true, IncludeAllEpisodes: IncludeEpisodesOriginal, Memos: true,
	})
	if err != nil || len(items.Movies) != 1 || items.Movies[0].Movie.IDs.Simkl != 101 ||
		len(items.Shows) != 1 || len(items.Shows[0].Seasons[0].Episodes) != 1 {
		t.Fatalf("unexpected all items: %#v, %v", items, err)
	}
	listResult, err := client.AddToList(ctx, AddToListRequest{To: StatusPlanToWatch, Movies: []MediaRef{{IDs: IDs{Simkl: 101}}}})
	if err != nil || len(listResult.Added.Movies) != 1 {
		t.Fatalf("unexpected list result: %#v, %v", listResult, err)
	}
	watchedAt := testNow.Add(-time.Hour)
	history := HistoryMutation{
		Movies: []HistoryMedia{{MediaRef: MediaRef{IDs: IDs{Simkl: 101}}, WatchedAt: &watchedAt, Status: StatusCompleted, Rating: 9}},
		Shows:  []HistorySeries{{HistoryMedia: HistoryMedia{MediaRef: MediaRef{IDs: IDs{Simkl: 202}}}, Seasons: []SeasonRef{{Number: 1, Episodes: []EpisodeRef{{Number: 1}, {Number: 2}}}}}},
	}
	historyResult, err := client.AddHistory(ctx, history)
	if err != nil || historyResult.Added.Episodes != 2 {
		t.Fatalf("unexpected history result: %#v, %v", historyResult, err)
	}
	removed, err := client.RemoveHistory(ctx, HistoryMutation{Movies: []HistoryMedia{{MediaRef: MediaRef{IDs: IDs{Simkl: 101}}}}})
	if err != nil || removed.Deleted.Movies != 1 {
		t.Fatalf("unexpected history removal: %#v, %v", removed, err)
	}
	rated, err := client.AddRatings(ctx, RatingsMutation{Movies: []MediaRating{{MediaRef: MediaRef{IDs: IDs{Simkl: 101}}, Rating: 10}}})
	if err != nil || rated.Added.Movies != 1 {
		t.Fatalf("unexpected rating result: %#v, %v", rated, err)
	}
	unrated, err := client.RemoveRatings(ctx, RatingRemoval{Movies: []MediaRef{{IDs: IDs{Simkl: 101}}}})
	if err != nil || unrated.Deleted.Movies != 1 {
		t.Fatalf("unexpected rating removal: %#v, %v", unrated, err)
	}
	start, err := client.Start(ctx, ScrobbleRequest{Progress: 25, Movie: &MediaRef{IDs: IDs{Simkl: 101}}})
	if err != nil || start.Action != "start" || start.SessionID != "session-1" {
		t.Fatalf("unexpected start: %#v, %v", start, err)
	}
	pause, err := client.Pause(ctx, ScrobbleRequest{Progress: 40, Show: &MediaRef{IDs: IDs{Simkl: 202}}, Episode: &PlaybackEpisodeRef{Season: 1, Number: 2}})
	if err != nil || pause.Episode == nil || pause.Episode.Title != "The Engineer" {
		t.Fatalf("unexpected pause: %#v, %v", pause, err)
	}
	stop, err := client.Stop(ctx, ScrobbleRequest{Progress: 90, Anime: &MediaRef{IDs: IDs{MAL: "1"}}, Episode: &PlaybackEpisodeRef{IDs: IDs{AniDB: "777"}}})
	if err != nil || stop.Action != "scrobble" {
		t.Fatalf("unexpected stop: %#v, %v", stop, err)
	}
	checkin, err := client.Checkin(ctx, ScrobbleRequest{Show: &MediaRef{IDs: IDs{TVDB: "81189"}}, Episode: &PlaybackEpisodeRef{Season: 1, Number: 1}})
	if err != nil || checkin.ID != 99 || checkin.Action != "checkin" {
		t.Fatalf("unexpected checkin: %#v, %v", checkin, err)
	}
}

func TestSyncAndScrobbleValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, true, true)
	zeroTime := time.Time{}
	validMovie := MediaRef{IDs: IDs{Simkl: 1}}
	tests := []func() error{
		func() error {
			_, err := client.ListAllItems(context.Background(), AllItemsRequest{Type: "bad"})
			return err
		},
		func() error {
			_, err := client.ListAllItems(context.Background(), AllItemsRequest{EpisodeWatchedAt: true})
			return err
		},
		func() error {
			_, err := client.ListAllItems(context.Background(), AllItemsRequest{DateFrom: &zeroTime})
			return err
		},
		func() error { _, err := client.AddToList(context.Background(), AddToListRequest{}); return err },
		func() error {
			_, err := client.AddToList(context.Background(), AddToListRequest{To: StatusWatching, Movies: []MediaRef{validMovie}})
			return err
		},
		func() error { _, err := client.AddHistory(context.Background(), HistoryMutation{}); return err },
		func() error {
			_, err := client.AddHistory(context.Background(), HistoryMutation{Movies: []HistoryMedia{{MediaRef: validMovie, Status: StatusWatching}}})
			return err
		},
		func() error {
			_, err := client.AddHistory(context.Background(), HistoryMutation{Shows: []HistorySeries{{HistoryMedia: HistoryMedia{MediaRef: validMovie}, Episodes: []EpisodeRef{{Number: 0}}}}})
			return err
		},
		func() error {
			_, err := client.RemoveHistory(context.Background(), HistoryMutation{Movies: []HistoryMedia{{MediaRef: validMovie, Rating: 1}}})
			return err
		},
		func() error {
			_, err := client.AddRatings(context.Background(), RatingsMutation{Movies: []MediaRating{{MediaRef: validMovie}}})
			return err
		},
		func() error { _, err := client.RemoveRatings(context.Background(), RatingRemoval{}); return err },
		func() error {
			_, err := client.Start(context.Background(), ScrobbleRequest{Progress: math.NaN(), Movie: &validMovie})
			return err
		},
		func() error {
			_, err := client.Start(context.Background(), ScrobbleRequest{Movie: &validMovie, Show: &validMovie})
			return err
		},
		func() error {
			_, err := client.Start(context.Background(), ScrobbleRequest{Show: &validMovie})
			return err
		},
		func() error {
			_, err := client.Start(context.Background(), ScrobbleRequest{Show: &validMovie, Episode: &PlaybackEpisodeRef{IDs: IDs{IMDB: "tt1"}}})
			return err
		},
		func() error {
			_, err := client.AddRatings(context.Background(), RatingsMutation{Movies: []MediaRating{{MediaRef: validMovie, Rating: 5}}}, socialhub.WithIdempotencyKey("key"))
			return err
		},
	}
	for index, test := range tests {
		if err := test(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("case %d: expected invalid argument, got %v", index, err)
		}
	}
	_, noToken := newTestClient(t, server, true, false)
	if _, err := noToken.GetActivities(context.Background()); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("expected OAuth approval error, got %v", err)
	}
}

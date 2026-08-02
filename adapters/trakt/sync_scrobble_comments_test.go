package trakt

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

func TestSyncScrobbleAndCommentWorkflows(t *testing.T) {
	calls := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer "+testAccessToken || request.Header.Get("trakt-api-key") != testClientID {
			http.Error(writer, "bad auth", http.StatusUnauthorized)
			return
		}
		key := request.Method + " " + request.URL.Path
		calls[key]++
		setPagination(writer, 1, 2)
		switch key {
		case "POST /sync/history":
			writeJSON(writer, http.StatusOK, `{"added":{"movies":1,"episodes":1},"updated":{"movies":0,"episodes":0},"not_found":{}}`)
		case "POST /sync/history/remove":
			writeJSON(writer, http.StatusOK, `{"deleted":{"movies":1,"episodes":1},"not_found":{}}`)
		case "POST /sync/watchlist":
			writeJSON(writer, http.StatusCreated, `{"added":{"movies":1,"shows":1,"seasons":1,"episodes":1},"existing":{"movies":0,"shows":0,"seasons":0,"episodes":0},"not_found":{},"list":{"updated_at":"2026-08-02T00:00:00Z","item_count":4}}`)
		case "POST /sync/watchlist/remove":
			writeJSON(writer, http.StatusOK, `{"deleted":{"movies":1,"shows":1,"seasons":1,"episodes":1},"not_found":{},"list":{"updated_at":"2026-08-02T00:00:00Z","item_count":0}}`)
		case "POST /sync/ratings":
			if !strings.Contains(requestBody(request), `"rating":10`) {
				http.Error(writer, "missing rating", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusCreated, `{"added":{"movies":1,"shows":1,"seasons":1,"episodes":1},"not_found":{}}`)
		case "POST /sync/ratings/remove":
			if strings.Contains(requestBody(request), `"rating"`) {
				http.Error(writer, "rating must be omitted", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"deleted":{"movies":1,"shows":1,"seasons":1,"episodes":1},"not_found":{}}`)
		case "POST /scrobble/start", "POST /scrobble/pause", "POST /scrobble/stop":
			var input ScrobbleRequest
			if err := json.NewDecoder(request.Body).Decode(&input); err != nil || input.Movie == nil || input.Progress <= 0 {
				http.Error(writer, "bad scrobble", http.StatusBadRequest)
				return
			}
			action := strings.TrimPrefix(request.URL.Path, "/scrobble/")
			writeJSON(writer, http.StatusCreated, `{"id":123,"progress":`+floatString(input.Progress)+`,"action":"`+action+`","movie":`+movieJSON("TRON: Legacy", 12601)+`}`)
		case "GET /comments/recent/all/all":
			writeJSON(writer, http.StatusOK, `[`+commentJSON(5, "hello")+`]`)
		case "GET /comments/5":
			writeJSON(writer, http.StatusOK, commentJSON(5, "hello"))
		case "GET /comments/5/replies":
			writeJSON(writer, http.StatusOK, `[`+commentJSON(6, "reply")+`]`)
		case "POST /comments":
			body := requestBody(request)
			if !strings.Contains(body, `"movie":{"ids":{"trakt":12601}}`) {
				http.Error(writer, "bad target", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusCreated, commentJSON(7, "new comment"))
		case "POST /comments/5/replies":
			writeJSON(writer, http.StatusCreated, commentJSON(8, "new reply"))
		case "PUT /comments/5":
			writeJSON(writer, http.StatusOK, commentJSON(5, "edited"))
		case "DELETE /comments/5", "POST /comments/5/like", "DELETE /comments/5/like":
			if request.ContentLength > 0 {
				http.Error(writer, "unexpected body", http.StatusBadRequest)
				return
			}
			writer.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, true, true)

	movie := MovieRef{Title: "TRON: Legacy", Year: 2010, IDs: IDs{Trakt: 12601}}
	show := ShowRef{IDs: IDs{Slug: "tron-uprising"}}
	season := SeasonRef{Number: 1, IDs: IDs{Trakt: 11}}
	episode := EpisodeRef{IDs: IDs{TVDB: 4318713}}
	watchedAt := testNow.Add(-time.Hour)
	history := HistoryMutation{
		Movies:   []HistoryMovie{{MovieRef: movie, WatchedAt: &watchedAt}},
		Episodes: []HistoryEpisode{{EpisodeRef: episode, WatchedAt: &watchedAt}},
	}
	addedHistory, err := client.AddHistory(context.Background(), history)
	if err != nil || addedHistory.Added.Movies != 1 {
		t.Fatalf("add history=%#v err=%v", addedHistory, err)
	}
	history.IDs = []int64{123}
	removedHistory, err := client.RemoveHistory(context.Background(), history)
	if err != nil || removedHistory.Deleted.Episodes != 1 {
		t.Fatalf("remove history=%#v err=%v", removedHistory, err)
	}
	media := MediaMutation{Movies: []MovieRef{movie}, Shows: []ShowRef{show}, Seasons: []SeasonRef{season}, Episodes: []EpisodeRef{episode}}
	watchlist, err := client.AddWatchlist(context.Background(), media)
	if err != nil || watchlist.Added.Shows != 1 || watchlist.List == nil || watchlist.List.ItemCount != 4 {
		t.Fatalf("add watchlist=%#v err=%v", watchlist, err)
	}
	removedWatchlist, err := client.RemoveWatchlist(context.Background(), media)
	if err != nil || removedWatchlist.Deleted.Seasons != 1 {
		t.Fatalf("remove watchlist=%#v err=%v", removedWatchlist, err)
	}
	ratings := RatingsMutation{
		Movies: []RatedMovie{{IDs: movie.IDs, Rating: 10}}, Shows: []RatedShow{{IDs: show.IDs, Rating: 10}},
		Seasons: []RatedSeason{{IDs: season.IDs, Rating: 10}}, Episodes: []RatedEpisode{{IDs: episode.IDs, Rating: 10}},
	}
	addedRatings, err := client.AddRatings(context.Background(), ratings)
	if err != nil || addedRatings.Added.Episodes != 1 {
		t.Fatalf("add ratings=%#v err=%v", addedRatings, err)
	}
	removedRatings, err := client.RemoveRatings(context.Background(), ratings)
	if err != nil || removedRatings.Deleted.Movies != 1 {
		t.Fatalf("remove ratings=%#v err=%v", removedRatings, err)
	}

	scrobbleInput := ScrobbleRequest{Progress: 50, Movie: &movie}
	started, err := client.StartScrobble(context.Background(), scrobbleInput)
	if err != nil || started.Action != "start" {
		t.Fatalf("start=%#v err=%v", started, err)
	}
	paused, err := client.PauseScrobble(context.Background(), scrobbleInput)
	if err != nil || paused.Action != "pause" {
		t.Fatalf("pause=%#v err=%v", paused, err)
	}
	stopped, err := client.StopScrobble(context.Background(), ScrobbleRequest{Progress: 90, Movie: &movie})
	if err != nil || stopped.ID != 123 {
		t.Fatalf("stop=%#v err=%v", stopped, err)
	}

	comments, err := client.ListComments(context.Background(), CommentActivityRequest{
		Activity: "recent", CommentType: "all", MediaType: "all", IncludeReplies: true, MaxResults: 1,
	})
	if err != nil || len(comments.Items) != 1 || comments.Items[0].Text != "hello" || comments.NextCursor == nil {
		t.Fatalf("comments=%#v err=%v", comments, err)
	}
	comment, err := client.GetComment(context.Background(), 5)
	if err != nil || comment.ID != 5 {
		t.Fatalf("comment=%#v err=%v", comment, err)
	}
	replies, err := client.ListReplies(context.Background(), 5, PageRequest{MaxResults: 1})
	if err != nil || len(replies.Items) != 1 || replies.Items[0].ParentID != 5 {
		t.Fatalf("replies=%#v err=%v", replies, err)
	}
	posted, err := client.PostComment(context.Background(), CreateCommentRequest{
		Target: CommentTarget{Type: MediaMovie, TraktID: 12601}, Text: "new comment", Spoiler: true,
	})
	if err != nil || posted.ID != 7 {
		t.Fatalf("posted=%#v err=%v", posted, err)
	}
	reply, err := client.ReplyComment(context.Background(), 5, "new reply", false)
	if err != nil || reply.ID != 8 {
		t.Fatalf("reply=%#v err=%v", reply, err)
	}
	edited, err := client.UpdateComment(context.Background(), EditCommentRequest{ID: 5, Text: "edited"})
	if err != nil || edited.Text != "edited" {
		t.Fatalf("edited=%#v err=%v", edited, err)
	}
	if err := client.DeleteComment(context.Background(), 5); err != nil {
		t.Fatal(err)
	}
	if err := client.LikeComment(context.Background(), 5); err != nil {
		t.Fatal(err)
	}
	if err := client.UnlikeComment(context.Background(), 5); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{
		"POST /sync/history", "POST /sync/history/remove", "POST /sync/watchlist", "POST /sync/watchlist/remove",
		"POST /sync/ratings", "POST /sync/ratings/remove", "POST /scrobble/start", "POST /scrobble/pause", "POST /scrobble/stop",
		"GET /comments/recent/all/all", "GET /comments/5", "GET /comments/5/replies", "POST /comments", "POST /comments/5/replies",
		"PUT /comments/5", "DELETE /comments/5", "POST /comments/5/like", "DELETE /comments/5/like",
	} {
		if calls[key] != 1 {
			t.Fatalf("call %s=%d", key, calls[key])
		}
	}
}

func TestMutationScrobbleAndCommentValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, true, true)
	tests := []struct {
		name string
		call func() error
	}{
		{"add history empty", func() error { _, err := client.AddHistory(context.Background(), HistoryMutation{}); return err }},
		{"add history IDs", func() error {
			_, err := client.AddHistory(context.Background(), HistoryMutation{IDs: []int64{1}})
			return err
		}},
		{"remove history ID", func() error {
			_, err := client.RemoveHistory(context.Background(), HistoryMutation{IDs: []int64{-1}})
			return err
		}},
		{"watchlist", func() error { _, err := client.AddWatchlist(context.Background(), MediaMutation{}); return err }},
		{"movie ID", func() error {
			_, err := client.RemoveWatchlist(context.Background(), MediaMutation{Movies: []MovieRef{{}}})
			return err
		}},
		{"ratings", func() error { _, err := client.AddRatings(context.Background(), RatingsMutation{}); return err }},
		{"rating value", func() error {
			_, err := client.AddRatings(context.Background(), RatingsMutation{Movies: []RatedMovie{{IDs: IDs{Trakt: 1}, Rating: 0}}})
			return err
		}},
		{"scrobble union", func() error { _, err := client.StartScrobble(context.Background(), ScrobbleRequest{}); return err }},
		{"stop progress", func() error {
			_, err := client.StopScrobble(context.Background(), ScrobbleRequest{Progress: 0, Episode: &EpisodeRef{IDs: IDs{Trakt: 1}}})
			return err
		}},
		{"comment filters", func() error {
			_, err := client.ListComments(context.Background(), CommentActivityRequest{})
			return err
		}},
		{"comment ID", func() error { _, err := client.GetComment(context.Background(), 0); return err }},
		{"reply ID", func() error { _, err := client.ListReplies(context.Background(), 0, PageRequest{}); return err }},
		{"post comment", func() error { _, err := client.PostComment(context.Background(), CreateCommentRequest{}); return err }},
		{"reply text", func() error { _, err := client.ReplyComment(context.Background(), 1, "", false); return err }},
		{"update ID", func() error { _, err := client.UpdateComment(context.Background(), EditCommentRequest{}); return err }},
		{"delete ID", func() error { return client.DeleteComment(context.Background(), 0) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestWriteOAuthGates(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, true, false)
	movie := MovieRef{Title: "Movie", Year: 2020, IDs: IDs{Trakt: 1}}
	calls := []func() error{
		func() error {
			_, err := client.AddHistory(context.Background(), HistoryMutation{Movies: []HistoryMovie{{MovieRef: movie}}})
			return err
		},
		func() error {
			_, err := client.StartScrobble(context.Background(), ScrobbleRequest{Progress: 1, Movie: &movie})
			return err
		},
		func() error {
			_, err := client.PostComment(context.Background(), CreateCommentRequest{Target: CommentTarget{Type: MediaMovie, TraktID: 1}, Text: "text"})
			return err
		},
		func() error { return client.LikeComment(context.Background(), 1) },
	}
	for _, call := range calls {
		if err := call(); !errors.Is(err, socialhub.ErrApprovalRequired) {
			t.Fatalf("OAuth gate=%v", err)
		}
	}
}

func commentJSON(id int64, text string) string {
	parent := int64(0)
	if id == 6 || id == 8 {
		parent = 5
	}
	return `{"id":` + intString(id) + `,"parent_id":` + intString(parent) + `,"created_at":"2026-08-01T00:00:00Z","updated_at":"2026-08-01T00:00:00Z","comment":"` + text + `","spoiler":false,"review":false,"replies":1,"likes":2,"language":"en","user_stats":{"rating":8,"play_count":1,"completed_count":1},"user":` + profileJSON() + `}`
}

func floatString(value float64) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func readAll(reader io.Reader) string {
	body, _ := io.ReadAll(reader)
	return string(body)
}

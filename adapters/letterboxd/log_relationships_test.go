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

func TestLogEntryAndRelationshipWorkflows(t *testing.T) {
	seen := make([]string, 0, 11)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer "+testAccessToken {
			http.Error(writer, "bad auth", http.StatusUnauthorized)
			return
		}
		key := request.Method + " " + request.URL.Path
		seen = append(seen, key)
		switch key {
		case "GET /log-entries":
			query := request.URL.Query()
			if query.Get("film") != "film-1" || query.Get("member") != "member-1" || query.Get("year") != "2026" ||
				query.Get("month") != "8" || query.Get("minRating") != "3.5" || query.Get("maxRating") != "5.0" ||
				query.Get("sort") != "ReviewPopularity" || !slices.Equal(query["where"], []string{"HasReview", "NoSpoilers"}) ||
				query.Get("cursor") != "log-cursor" || query.Get("perPage") != "20" {
				http.Error(writer, "bad log query", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"next":"log-next","items":[`+logEntryJSON("log-1")+`]}`)
		case "GET /log-entry/log-1":
			writeJSON(writer, http.StatusOK, logEntryJSON("log-1"))
		case "GET /log-entry/log-1/comments":
			if request.URL.Query().Get("cursor") != "comment-cursor" || request.URL.Query().Get("perPage") != "10" {
				http.Error(writer, "bad comments query", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"next":"comment-next","items":[`+commentJSON("comment-1", "Great review")+`]}`)
		case "POST /log-entries":
			if requestBody(request) != `{"filmId":"film-1","diaryDetails":{"diaryDate":"2026-08-01","rewatch":true},"review":{"text":"A measured review.","containsSpoilers":true},"tags":["neo-noir"],"rating":4.5,"like":true,"commentPolicy":"Friends","privacyPolicy":"Anyone"}` {
				http.Error(writer, "bad create body", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, logEntryJSON("log-created"))
		case "PATCH /log-entry/log-created":
			if requestBody(request) != `{"review":{"text":"Updated review."},"tags":["sci-fi"],"rating":5,"like":false}` {
				http.Error(writer, "bad update body", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"data":`+logEntryJSON("log-created")+`,"messages":[{"type":"Success","code":"Updated","title":"Updated"}]}`)
		case "POST /log-entry/log-1/comments":
			if requestBody(request) != `{"comment":"Thoughtful take."}` {
				http.Error(writer, "bad comment body", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, commentJSON("comment-created", "Thoughtful take."))
		case "DELETE /log-entry/log-created":
			writer.WriteHeader(http.StatusNoContent)
		case "PATCH /me/like/film-1":
			assertBody(t, writer, request, `{"liked":true}`)
		case "PATCH /me/rate/film-1":
			assertBody(t, writer, request, `{"rating":null}`)
		case "PATCH /me/watch/film-1":
			assertBody(t, writer, request, `{"watched":false}`)
		case "PATCH /me/watchlist/film-1":
			assertBody(t, writer, request, `{"inWatchlist":true}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, TokenUser, true, []string{"content:modify"})

	logs, err := client.ListLogEntries(context.Background(), LogEntriesRequest{
		FilmID: "film-1", MemberID: "member-1", Year: 2026, Month: 8, MinRating: 3.5, MaxRating: 5,
		Where: []string{"HasReview", "NoSpoilers"}, Sort: "ReviewPopularity", Cursor: "log-cursor", PerPage: 20,
	})
	if err != nil || len(logs.Items) != 1 || logs.Items[0].ID != "log-1" || !logs.HasMore {
		t.Fatalf("logs=%#v err=%v", logs, err)
	}
	entry, err := client.GetLogEntry(context.Background(), "log-1")
	if err != nil || entry.Review == nil || entry.Review.Text != "A measured review." || entry.Review.LBML != "A measured review." ||
		len(entry.Tags2) != 1 || entry.Tags2[0].DisplayTag != "neo-noir" || entry.Rating == nil || *entry.Rating != 4.5 {
		t.Fatalf("entry=%#v err=%v", entry, err)
	}
	comments, err := client.ListReviewComments(context.Background(), "log-1", PageRequest{Cursor: "comment-cursor", PerPage: 10})
	if err != nil || len(comments.Items) != 1 || comments.Items[0].Comment != "Great review" || !comments.HasMore {
		t.Fatalf("comments=%#v err=%v", comments, err)
	}
	rating := 4.5
	liked := true
	created, err := client.CreateLogEntry(context.Background(), LogEntryCreationRequest{
		FilmID: "film-1", DiaryDetails: &LogEntryCreationDiaryDetails{DiaryDate: "2026-08-01", Rewatch: true},
		Review: &LogEntryCreationReview{Text: "A measured review.", ContainsSpoilers: true}, Tags: []string{"neo-noir"},
		Rating: &rating, Like: &liked, CommentPolicy: "Friends", PrivacyPolicy: "Anyone",
	})
	if err != nil || created.ID != "log-created" {
		t.Fatalf("created=%#v err=%v", created, err)
	}
	updatedRating := 5.0
	updatedLiked := false
	updated, err := client.UpdateLogEntry(context.Background(), "log-created", LogEntryUpdateRequest{
		Review: &LogEntryCreationReview{Text: "Updated review."}, Tags: []string{"sci-fi"}, Rating: &updatedRating, Like: &updatedLiked,
	})
	if err != nil || updated.Data.ID != "log-created" || len(updated.Messages) != 1 {
		t.Fatalf("updated=%#v err=%v", updated, err)
	}
	comment, err := client.CreateReviewComment(context.Background(), "log-1", "Thoughtful take.")
	if err != nil || comment.ID != "comment-created" {
		t.Fatalf("comment=%#v err=%v", comment, err)
	}
	if err := client.DeleteLogEntry(context.Background(), "log-created"); err != nil {
		t.Fatal(err)
	}
	if err := client.SetLike(context.Background(), "film-1", true); err != nil {
		t.Fatal(err)
	}
	if err := client.SetRating(context.Background(), "film-1", nil); err != nil {
		t.Fatal(err)
	}
	if err := client.SetWatched(context.Background(), "film-1", false); err != nil {
		t.Fatal(err)
	}
	if err := client.SetWatchlist(context.Background(), "film-1", true); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 11 {
		t.Fatalf("requests=%v", seen)
	}
}

func TestLogEntryAndRelationshipValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, TokenUser, true, []string{"content:modify"})
	badRating := 4.25
	tests := []func() error{
		func() error {
			_, err := client.ListLogEntries(context.Background(), LogEntriesRequest{Month: 8})
			return err
		},
		func() error {
			_, err := client.ListLogEntries(context.Background(), LogEntriesRequest{Year: 2026, Month: 13})
			return err
		},
		func() error {
			_, err := client.ListLogEntries(context.Background(), LogEntriesRequest{MinRating: 4, MaxRating: 3})
			return err
		},
		func() error {
			_, err := client.ListLogEntries(context.Background(), LogEntriesRequest{Where: []string{"FirstParty"}})
			return err
		},
		func() error { _, err := client.GetLogEntry(context.Background(), "bad/id"); return err },
		func() error {
			_, err := client.ListReviewComments(context.Background(), "log-1", PageRequest{PerPage: 101})
			return err
		},
		func() error {
			_, err := client.CreateLogEntry(context.Background(), LogEntryCreationRequest{FilmID: "film-1"})
			return err
		},
		func() error {
			_, err := client.CreateLogEntry(context.Background(), LogEntryCreationRequest{FilmID: "film-1", DiaryDetails: &LogEntryCreationDiaryDetails{DiaryDate: "2026-02-30"}})
			return err
		},
		func() error {
			_, err := client.CreateLogEntry(context.Background(), LogEntryCreationRequest{FilmID: "film-1", Review: &LogEntryCreationReview{Text: " "}})
			return err
		},
		func() error {
			_, err := client.UpdateLogEntry(context.Background(), "log-1", LogEntryUpdateRequest{})
			return err
		},
		func() error {
			_, err := client.UpdateLogEntry(context.Background(), "log-1", LogEntryUpdateRequest{Rating: &badRating})
			return err
		},
		func() error { return client.DeleteLogEntry(context.Background(), "") },
		func() error { _, err := client.CreateReviewComment(context.Background(), "log-1", " "); return err },
		func() error { return client.SetRating(context.Background(), "film-1", &badRating) },
		func() error { return client.SetLike(context.Background(), "bad/id", true) },
	}
	for _, call := range tests {
		if err := call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("error=%v", err)
		}
	}
}

func assertBody(t *testing.T, writer http.ResponseWriter, request *http.Request, expected string) {
	t.Helper()
	if request.Header.Get("Content-Type") != "application/json" || requestBody(request) != expected {
		http.Error(writer, "bad relationship body", http.StatusBadRequest)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func logEntryJSON(id string) string {
	return `{"id":"` + id + `","film":{"id":"film-1","name":"Blade Runner"},"owner":{"id":"member-1","username":"deckard"},"diaryDetails":{"diaryDate":"2026-08-01","rewatch":true},"review":{"containsSpoilers":false,"text":"A measured review.","lbml":"A measured review."},"tags2":[{"code":"neo-noir","displayTag":"neo-noir"}],"whenCreated":"2026-08-02T08:00:00Z","rating":4.5,"like":true}`
}

func commentJSON(id, comment string) string {
	return `{"id":"` + id + `","member":{"id":"member-2","username":"rachel"},"whenCreated":"2026-08-02T08:01:00Z","whenUpdated":"2026-08-02T08:01:00Z","comment":"` + comment + `"}`
}

package myanimelist

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestUserAndListWorkflows(t *testing.T) {
	calls := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer "+testAccessToken {
			http.Error(writer, "bad auth", http.StatusUnauthorized)
			return
		}
		key := request.Method + " " + request.URL.Path
		calls[key]++
		switch key {
		case "GET /users/@me":
			if request.URL.Query().Get("fields") != "anime_statistics,is_supporter" {
				http.Error(writer, "bad user fields", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"id":7,"name":"fan","joined_at":"2020-01-02T03:04:05Z","anime_statistics":{"num_items":12,"mean_score":8.5},"is_supporter":true}`)
		case "GET /users/alice/animelist":
			if request.URL.Query().Get("status") != "watching" || request.URL.Query().Get("sort") != "list_updated_at" ||
				request.URL.Query().Get("offset") != "20" || request.URL.Query().Get("limit") != "10" ||
				request.URL.Query().Get("fields") != "node{id,title},list_status" {
				http.Error(writer, "bad anime list query", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"data":[{"node":{"id":1,"title":"Gundam"},"list_status":{"status":"watching","score":8,"num_episodes_watched":10}}],"paging":{"next":"https://api.example/users/alice/animelist?offset=30"}}`)
		case "GET /users/alice/mangalist":
			if request.URL.Query().Get("status") != "reading" || request.URL.Query().Get("sort") != "manga_title" {
				http.Error(writer, "bad manga list query", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"data":[{"node":{"id":10,"title":"Berserk"},"list_status":{"status":"reading","score":9,"num_volumes_read":20,"num_chapters_read":200}}],"paging":{}}`)
		case "PATCH /anime/1/my_list_status":
			if !assertForm(t, writer, request, url.Values{
				"status": {"completed"}, "is_rewatching": {"false"}, "score": {"0"},
				"num_watched_episodes": {"0"}, "priority": {"0"}, "num_times_rewatched": {"0"},
				"rewatch_value": {"0"}, "tags": {""}, "comments": {""},
			}) {
				return
			}
			writeJSON(writer, http.StatusOK, `{"status":"completed","score":0,"num_episodes_watched":0,"is_rewatching":false,"tags":[],"comments":""}`)
		case "PATCH /manga/10/my_list_status":
			if !assertForm(t, writer, request, url.Values{
				"status": {"completed"}, "is_rereading": {"false"}, "score": {"0"},
				"num_volumes_read": {"0"}, "num_chapters_read": {"0"}, "priority": {"0"},
				"num_times_reread": {"0"}, "reread_value": {"0"}, "tags": {""}, "comments": {""},
			}) {
				return
			}
			writeJSON(writer, http.StatusOK, `{"status":"completed","score":0,"num_volumes_read":0,"num_chapters_read":0,"is_rereading":false,"tags":[],"comments":""}`)
		case "DELETE /anime/1/my_list_status", "DELETE /manga/10/my_list_status":
			writer.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, false, true, []string{scopeWriteUsers})

	user, err := client.GetMe(context.Background(), socialhub.WithFields("anime_statistics", "is_supporter"))
	if err != nil || user.ID != 7 || user.AnimeStatistics == nil || user.AnimeStatistics.NumItems != 12 || user.IsSupporter == nil || !*user.IsSupporter {
		t.Fatalf("user=%#v err=%v", user, err)
	}
	anime, err := client.ListAnimeList(context.Background(), AnimeListRequest{
		Username: "alice", Status: AnimeWatching, Sort: AnimeListSortUpdatedAt, Cursor: "20", Limit: 10,
	}, socialhub.WithFields("node{id,title}", "list_status"))
	if err != nil || len(anime.Items) != 1 || anime.Items[0].Anime.Title != "Gundam" ||
		anime.Items[0].ListStatus.NumEpisodesWatched != 10 || anime.NextCursor == nil || *anime.NextCursor != "30" {
		t.Fatalf("anime list=%#v err=%v", anime, err)
	}
	manga, err := client.ListMangaList(context.Background(), MangaListRequest{
		Username: "alice", Status: MangaReading, Sort: MangaListSortTitle,
	})
	if err != nil || len(manga.Items) != 1 || manga.Items[0].Manga.Title != "Berserk" || manga.Items[0].ListStatus.NumChaptersRead != 200 {
		t.Fatalf("manga list=%#v err=%v", manga, err)
	}

	zero, no, empty := 0, false, ""
	animeState := AnimeCompleted
	animeStatus, err := client.UpdateAnimeListStatus(context.Background(), UpdateAnimeListStatusRequest{
		AnimeID: 1, Status: &animeState, IsRewatching: &no, Score: &zero, NumWatchedEpisodes: &zero,
		Priority: &zero, NumTimesRewatched: &zero, RewatchValue: &zero, Tags: []string{}, Comments: &empty,
	})
	if err != nil || animeStatus.Status != AnimeCompleted || animeStatus.Score != 0 {
		t.Fatalf("anime status=%#v err=%v", animeStatus, err)
	}
	mangaState := MangaCompleted
	mangaStatus, err := client.UpdateMangaListStatus(context.Background(), UpdateMangaListStatusRequest{
		MangaID: 10, Status: &mangaState, IsRereading: &no, Score: &zero, NumVolumesRead: &zero,
		NumChaptersRead: &zero, Priority: &zero, NumTimesReread: &zero, RereadValue: &zero,
		Tags: []string{}, Comments: &empty,
	})
	if err != nil || mangaStatus.Status != MangaCompleted || mangaStatus.NumChaptersRead != 0 {
		t.Fatalf("manga status=%#v err=%v", mangaStatus, err)
	}
	if err := client.DeleteAnimeListStatus(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if err := client.DeleteMangaListStatus(context.Background(), 10); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{
		"GET /users/@me", "GET /users/alice/animelist", "GET /users/alice/mangalist",
		"PATCH /anime/1/my_list_status", "PATCH /manga/10/my_list_status",
		"DELETE /anime/1/my_list_status", "DELETE /manga/10/my_list_status",
	} {
		if calls[key] != 1 {
			t.Fatalf("call %s=%d", key, calls[key])
		}
	}
}

func TestPublicListReadsAndMutationFieldRejection(t *testing.T) {
	publicCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-MAL-CLIENT-ID") != testClientID || request.Header.Get("Authorization") != "" {
			http.Error(writer, "bad public auth", http.StatusUnauthorized)
			return
		}
		publicCalls++
		writeJSON(writer, http.StatusOK, `{"data":[],"paging":{}}`)
	}))
	defer server.Close()
	_, public := newTestClient(t, server, false, false, nil)
	if _, err := public.ListAnimeList(context.Background(), AnimeListRequest{Username: "alice"}); err != nil {
		t.Fatal(err)
	}
	if _, err := public.ListMangaList(context.Background(), MangaListRequest{Username: "alice"}); err != nil {
		t.Fatal(err)
	}
	if publicCalls != 2 {
		t.Fatalf("public calls=%d", publicCalls)
	}
	if _, err := public.ListMangaList(context.Background(), MangaListRequest{Username: "@me"}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("@me gate=%v", err)
	}

	_, user := newTestClient(t, server, false, true, []string{scopeWriteUsers})
	state := AnimeWatching
	if _, err := user.UpdateAnimeListStatus(context.Background(), UpdateAnimeListStatusRequest{AnimeID: 1, Status: &state}, socialhub.WithFields("id")); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("anime fields=%v", err)
	}
	if err := user.DeleteMangaListStatus(context.Background(), 1, socialhub.WithFields("id")); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("delete fields=%v", err)
	}
}

func TestListMutationValidationAndScopeGate(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, false, true, []string{scopeWriteUsers})
	badAnimeState, badMangaState := AnimeListState("paused"), MangaListState("paused")
	badScore, badPriority, badRepeat, negative := 11, 3, 6, -1
	badComment := "bad\x00comment"
	tests := []func() error{
		func() error { _, err := client.ListAnimeList(context.Background(), AnimeListRequest{}); return err },
		func() error {
			_, err := client.ListAnimeList(context.Background(), AnimeListRequest{Username: "bad/name"})
			return err
		},
		func() error {
			_, err := client.ListMangaList(context.Background(), MangaListRequest{Username: "alice", Status: "paused"})
			return err
		},
		func() error {
			_, err := client.ListMangaList(context.Background(), MangaListRequest{Username: "alice", Sort: "updated"})
			return err
		},
		func() error {
			_, err := client.UpdateAnimeListStatus(context.Background(), UpdateAnimeListStatusRequest{AnimeID: 1})
			return err
		},
		func() error {
			_, err := client.UpdateAnimeListStatus(context.Background(), UpdateAnimeListStatusRequest{AnimeID: 0, Status: &badAnimeState})
			return err
		},
		func() error {
			_, err := client.UpdateAnimeListStatus(context.Background(), UpdateAnimeListStatusRequest{AnimeID: 1, Status: &badAnimeState})
			return err
		},
		func() error {
			_, err := client.UpdateAnimeListStatus(context.Background(), UpdateAnimeListStatusRequest{AnimeID: 1, Score: &badScore})
			return err
		},
		func() error {
			_, err := client.UpdateAnimeListStatus(context.Background(), UpdateAnimeListStatusRequest{AnimeID: 1, Priority: &badPriority})
			return err
		},
		func() error {
			_, err := client.UpdateMangaListStatus(context.Background(), UpdateMangaListStatusRequest{MangaID: 1, Status: &badMangaState})
			return err
		},
		func() error {
			_, err := client.UpdateMangaListStatus(context.Background(), UpdateMangaListStatusRequest{MangaID: 1, NumVolumesRead: &negative})
			return err
		},
		func() error {
			_, err := client.UpdateMangaListStatus(context.Background(), UpdateMangaListStatusRequest{MangaID: 1, RereadValue: &badRepeat})
			return err
		},
		func() error {
			_, err := client.UpdateMangaListStatus(context.Background(), UpdateMangaListStatusRequest{MangaID: 1, Comments: &badComment})
			return err
		},
		func() error { return client.DeleteAnimeListStatus(context.Background(), 0) },
		func() error { return client.DeleteMangaListStatus(context.Background(), -1) },
	}
	for _, call := range tests {
		if err := call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("error=%v", err)
		}
	}

	client.scopes = []string{"read:users"}
	state := AnimeWatching
	err := func() error {
		_, err := client.UpdateAnimeListStatus(context.Background(), UpdateAnimeListStatusRequest{AnimeID: 1, Status: &state})
		return err
	}()
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodeApprovalRequired ||
		len(platformErr.RequiredScopes) != 1 || platformErr.RequiredScopes[0] != scopeWriteUsers {
		t.Fatalf("scope gate=%#v", platformErr)
	}
}

func assertForm(t *testing.T, writer http.ResponseWriter, request *http.Request, expected url.Values) bool {
	t.Helper()
	if request.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
		http.Error(writer, "bad content type", http.StatusBadRequest)
		t.Error("bad content type")
		return false
	}
	if err := request.ParseForm(); err != nil {
		http.Error(writer, "bad form", http.StatusBadRequest)
		t.Error(err)
		return false
	}
	if request.Form.Encode() != expected.Encode() {
		http.Error(writer, "bad form values", http.StatusBadRequest)
		t.Errorf("form=%v want=%v", request.Form, expected)
		return false
	}
	return true
}

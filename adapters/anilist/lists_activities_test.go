package anilist

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestMediaListWorkflows(t *testing.T) {
	calls := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer "+testAccessToken {
			http.Error(writer, "bad auth", http.StatusUnauthorized)
			return
		}
		body := readGraphQLRequest(t, writer, request)
		switch {
		case operationIs(body, "MediaList"):
			calls["list"]++
			if body.Variables["userName"] != "alice" || body.Variables["type"] != "ANIME" ||
				body.Variables["status"] != "CURRENT" || firstValue(body.Variables["sort"]) != "SCORE_DESC" ||
				body.Variables["page"] != float64(3) || body.Variables["perPage"] != float64(20) {
				http.Error(writer, "bad list variables", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"data":{"Page":{"pageInfo":{"currentPage":3,"perPage":20,"hasNextPage":false},"mediaList":[`+mediaListJSON()+`]}}}`)
		case operationIs(body, "SaveMediaListEntry"):
			calls["save"]++
			if body.Variables["id"] != float64(50) || body.Variables["status"] != "COMPLETED" ||
				body.Variables["score"] != float64(0) || body.Variables["progress"] != float64(0) ||
				body.Variables["progressVolumes"] != float64(0) || body.Variables["repeat"] != float64(0) ||
				body.Variables["priority"] != float64(0) || body.Variables["private"] != false ||
				body.Variables["notes"] != "" || body.Variables["hiddenFromStatusLists"] != false ||
				len(arrayValue(body.Variables["customLists"])) != 0 {
				http.Error(writer, "bad save variables", http.StatusBadRequest)
				return
			}
			started, _ := body.Variables["startedAt"].(map[string]any)
			if started["year"] != float64(2026) || started["month"] != float64(8) || started["day"] != float64(2) {
				http.Error(writer, "bad start date", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"data":{"SaveMediaListEntry":`+mediaListJSON()+`}}`)
		case operationIs(body, "DeleteMediaListEntry"):
			calls["delete"]++
			writeJSON(writer, http.StatusOK, `{"data":{"DeleteMediaListEntry":{"deleted":true}}}`)
		default:
			http.Error(writer, "unknown operation", http.StatusBadRequest)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, false, false, true)

	page, err := client.ListMediaList(context.Background(), ListMediaListRequest{
		Username: "alice", Type: MediaAnime, Status: ListCurrent, Sort: MediaListSortScoreDesc,
		Cursor: "3", Limit: 20,
	})
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != 50 || page.Items[0].Media == nil ||
		page.Items[0].Media.Title.Romaji != "Gundam" || page.PrevCursor == nil || *page.PrevCursor != "2" || page.HasMore {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	status, zeroScore, zero, no, notes := ListCompleted, float64(0), 0, false, ""
	entry, err := client.SaveMediaListEntry(context.Background(), SaveMediaListEntryRequest{
		ID: 50, Status: &status, Score: &zeroScore, Progress: &zero, ProgressVolumes: &zero,
		Repeat: &zero, Priority: &zero, Private: &no, Notes: &notes, HiddenFromStatusLists: &no,
		CustomLists: []string{}, StartedAt: &FuzzyDate{Year: 2026, Month: 8, Day: 2},
	})
	if err != nil || entry.ID != 50 || entry.Status != ListCurrent || entry.Media == nil {
		t.Fatalf("entry=%#v err=%v", entry, err)
	}
	if err := client.DeleteMediaListEntry(context.Background(), 50); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"list", "save", "delete"} {
		if calls[name] != 1 {
			t.Fatalf("calls[%s]=%d", name, calls[name])
		}
	}
}

func TestActivityWorkflows(t *testing.T) {
	calls := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer "+testAccessToken {
			http.Error(writer, "bad auth", http.StatusUnauthorized)
			return
		}
		body := readGraphQLRequest(t, writer, request)
		switch {
		case operationIs(body, "Activities"):
			calls["list"]++
			if body.Variables["userId"] != float64(7) || body.Variables["mediaId"] != float64(1) ||
				firstValue(body.Variables["typeIn"]) != "TEXT" || body.Variables["isFollowing"] != true ||
				firstValue(body.Variables["sort"]) != "ID_DESC" {
				http.Error(writer, "bad activity variables", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"data":{"Page":{"pageInfo":{"currentPage":1,"perPage":10,"hasNextPage":true},"activities":[`+textActivityJSON(100, "hello")+`,`+listActivityJSON(101)+`]}}}`)
		case operationIs(body, "SaveTextActivity"):
			calls["save"]++
			if body.Variables["id"] != float64(100) || body.Variables["text"] != "updated" || body.Variables["locked"] != false {
				http.Error(writer, "bad activity save", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"data":{"SaveTextActivity":`+textActivityJSON(100, "updated")+`}}`)
		case operationIs(body, "DeleteActivity"):
			calls["delete"]++
			writeJSON(writer, http.StatusOK, `{"data":{"DeleteActivity":{"deleted":true}}}`)
		case operationIs(body, "SaveActivityReply"):
			calls["reply"]++
			if body.Variables["activityId"] != float64(100) || body.Variables["text"] != "reply" {
				http.Error(writer, "bad reply", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"data":{"SaveActivityReply":`+activityReplyJSON()+`}}`)
		case operationIs(body, "DeleteActivityReply"):
			calls["delete_reply"]++
			writeJSON(writer, http.StatusOK, `{"data":{"DeleteActivityReply":{"deleted":true}}}`)
		case operationIs(body, "ToggleLike"):
			calls["like"]++
			if body.Variables["id"] != float64(100) || body.Variables["type"] != "ACTIVITY" {
				http.Error(writer, "bad like", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"data":{"ToggleLikeV2":{"__typename":"TextActivity","id":100,"likeCount":3,"isLiked":true}}}`)
		default:
			http.Error(writer, "unknown operation", http.StatusBadRequest)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, false, false, true)

	page, err := client.ListActivities(context.Background(), ListActivitiesRequest{
		UserID: 7, MediaID: 1, Types: []ActivityType{ActivityText}, Following: true, Limit: 10,
	})
	if err != nil || len(page.Items) != 2 || page.Items[0].Text == nil || *page.Items[0].Text != "hello" ||
		page.Items[1].Media == nil || page.Items[1].Media.ID != 1 || page.NextCursor == nil || *page.NextCursor != "2" {
		t.Fatalf("activities=%#v err=%v", page, err)
	}
	locked := false
	activity, err := client.SaveTextActivity(context.Background(), SaveTextActivityRequest{ID: 100, Text: "updated", Locked: &locked})
	if err != nil || activity.ID != 100 || activity.Text == nil || *activity.Text != "updated" {
		t.Fatalf("activity=%#v err=%v", activity, err)
	}
	if err := client.DeleteActivity(context.Background(), 100); err != nil {
		t.Fatal(err)
	}
	reply, err := client.ReplyActivity(context.Background(), 100, "reply")
	if err != nil || reply.ID != 200 || reply.ActivityID != 100 || reply.User == nil {
		t.Fatalf("reply=%#v err=%v", reply, err)
	}
	if err := client.DeleteActivityReply(context.Background(), 200); err != nil {
		t.Fatal(err)
	}
	like, err := client.ToggleLike(context.Background(), 100, LikeActivity)
	if err != nil || like.ID != 100 || !like.IsLiked || like.LikeCount != 3 {
		t.Fatalf("like=%#v err=%v", like, err)
	}
	for _, name := range []string{"list", "save", "delete", "reply", "delete_reply", "like"} {
		if calls[name] != 1 {
			t.Fatalf("calls[%s]=%d", name, calls[name])
		}
	}
}

func TestPublicListAndActivityReads(t *testing.T) {
	publicCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "" {
			http.Error(writer, "unexpected auth", http.StatusBadRequest)
			return
		}
		body := readGraphQLRequest(t, writer, request)
		publicCalls++
		if operationIs(body, "MediaList") {
			writeJSON(writer, http.StatusOK, `{"data":{"Page":{"pageInfo":{"currentPage":1,"perPage":50,"hasNextPage":false},"mediaList":[]}}}`)
			return
		}
		writeJSON(writer, http.StatusOK, `{"data":{"Page":{"pageInfo":{"currentPage":1,"perPage":50,"hasNextPage":false},"activities":[]}}}`)
	}))
	defer server.Close()
	_, client := newTestClient(t, server, false, false, false)
	if _, err := client.ListMediaList(context.Background(), ListMediaListRequest{Username: "alice", Type: MediaManga}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ListActivities(context.Background(), ListActivitiesRequest{}); err != nil {
		t.Fatal(err)
	}
	if publicCalls != 2 {
		t.Fatalf("public calls=%d", publicCalls)
	}
}

func TestListAndActivityValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, false, false, true)
	badStatus := MediaListStatus("")
	badScore, negative, badPriority := 101.0, -1, 6
	badNotes := "bad\x00notes"
	tests := []func() error{
		func() error { _, err := client.ListMediaList(context.Background(), ListMediaListRequest{}); return err },
		func() error {
			_, err := client.ListMediaList(context.Background(), ListMediaListRequest{UserID: 1, Username: "alice", Type: MediaAnime})
			return err
		},
		func() error {
			_, err := client.ListMediaList(context.Background(), ListMediaListRequest{Username: "alice", Type: "NOVEL"})
			return err
		},
		func() error {
			_, err := client.ListMediaList(context.Background(), ListMediaListRequest{Username: "alice", Type: MediaAnime, Status: "WATCHING"})
			return err
		},
		func() error {
			_, err := client.SaveMediaListEntry(context.Background(), SaveMediaListEntryRequest{})
			return err
		},
		func() error {
			_, err := client.SaveMediaListEntry(context.Background(), SaveMediaListEntryRequest{ID: 1, MediaID: 2})
			return err
		},
		func() error {
			_, err := client.SaveMediaListEntry(context.Background(), SaveMediaListEntryRequest{ID: 1})
			return err
		},
		func() error {
			_, err := client.SaveMediaListEntry(context.Background(), SaveMediaListEntryRequest{MediaID: 1, Status: &badStatus})
			return err
		},
		func() error {
			_, err := client.SaveMediaListEntry(context.Background(), SaveMediaListEntryRequest{MediaID: 1, Score: &badScore})
			return err
		},
		func() error {
			_, err := client.SaveMediaListEntry(context.Background(), SaveMediaListEntryRequest{MediaID: 1, Progress: &negative})
			return err
		},
		func() error {
			_, err := client.SaveMediaListEntry(context.Background(), SaveMediaListEntryRequest{MediaID: 1, Priority: &badPriority})
			return err
		},
		func() error {
			_, err := client.SaveMediaListEntry(context.Background(), SaveMediaListEntryRequest{MediaID: 1, Notes: &badNotes})
			return err
		},
		func() error {
			_, err := client.SaveMediaListEntry(context.Background(), SaveMediaListEntryRequest{MediaID: 1, CustomLists: []string{"dup", "dup"}})
			return err
		},
		func() error {
			_, err := client.SaveMediaListEntry(context.Background(), SaveMediaListEntryRequest{MediaID: 1, StartedAt: &FuzzyDate{Year: 2026, Month: 2, Day: 30}})
			return err
		},
		func() error { return client.DeleteMediaListEntry(context.Background(), 0) },
		func() error {
			_, err := client.ListActivities(context.Background(), ListActivitiesRequest{Types: []ActivityType{"MESSAGE"}})
			return err
		},
		func() error {
			_, err := client.ListActivities(context.Background(), ListActivitiesRequest{Types: []ActivityType{ActivityText, ActivityText}})
			return err
		},
		func() error {
			_, err := client.SaveTextActivity(context.Background(), SaveTextActivityRequest{Text: " \t"})
			return err
		},
		func() error { return client.DeleteActivity(context.Background(), 0) },
		func() error { _, err := client.ReplyActivity(context.Background(), 0, "reply"); return err },
		func() error { return client.DeleteActivityReply(context.Background(), -1) },
		func() error { _, err := client.ToggleLike(context.Background(), 1, "THREAD"); return err },
		func() error {
			_, err := client.ListActivities(context.Background(), ListActivitiesRequest{}, socialhub.WithFields("id"))
			return err
		},
		func() error {
			_, err := client.SaveTextActivity(context.Background(), SaveTextActivityRequest{Text: "hello"}, socialhub.WithFields("id"))
			return err
		},
	}
	for _, call := range tests {
		if err := call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("error=%v", err)
		}
	}
}

func mediaListJSON() string {
	return `{"id":50,"userId":7,"mediaId":1,"status":"CURRENT","score":8.5,"progress":10,"progressVolumes":0,"repeat":0,"priority":1,"private":false,"notes":"note","hiddenFromStatusLists":false,"customLists":{"Favorites":true},"advancedScores":{"Story":9},"startedAt":{"year":2026,"month":1,"day":2},"completedAt":{"year":0,"month":0,"day":0},"updatedAt":1785660000,"createdAt":1780000000,"media":` + mediaJSON(1, "Gundam", "ANIME") + `}`
}

func textActivityJSON(id int64, text string) string {
	return `{"__typename":"TextActivity","id":` + intString(id) + `,"userId":7,"type":"TEXT","replyCount":1,"text":"` + text + `","siteUrl":"https://anilist.co/activity/` + intString(id) + `","isLocked":false,"isSubscribed":true,"likeCount":2,"isLiked":false,"isPinned":false,"createdAt":1785660000,"user":` + userJSON(7, "fan") + `}`
}

func listActivityJSON(id int64) string {
	return `{"__typename":"ListActivity","id":` + intString(id) + `,"userId":7,"type":"ANIME_LIST","replyCount":0,"status":"watched episode","progress":"1","siteUrl":"https://anilist.co/activity/` + intString(id) + `","isLocked":false,"isSubscribed":false,"likeCount":1,"isLiked":true,"isPinned":false,"createdAt":1785660000,"user":` + userJSON(7, "fan") + `,"media":` + mediaJSON(1, "Gundam", "ANIME") + `}`
}

func activityReplyJSON() string {
	return `{"id":200,"userId":7,"activityId":100,"text":"reply","likeCount":0,"isLiked":false,"createdAt":1785660000,"user":` + userJSON(7, "fan") + `}`
}

func arrayValue(value any) []any {
	values, _ := value.([]any)
	return values
}

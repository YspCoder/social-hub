package anilist

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestMediaAndUserWorkflows(t *testing.T) {
	calls := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer "+testAccessToken {
			http.Error(writer, "bad auth", http.StatusUnauthorized)
			return
		}
		body := readGraphQLRequest(t, writer, request)
		switch {
		case operationIs(body, "SearchMedia"):
			calls["search"]++
			if body.Variables["search"] != "Gundam" || body.Variables["type"] != "ANIME" ||
				body.Variables["page"] != float64(2) || body.Variables["perPage"] != float64(10) ||
				body.Variables["isAdult"] != false || firstValue(body.Variables["sort"]) != "SEARCH_MATCH" {
				http.Error(writer, "bad search variables", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"data":{"Page":{"pageInfo":{"currentPage":2,"perPage":10,"hasNextPage":true},"media":[`+mediaJSON(1, "Mobile Suit Gundam", "ANIME")+`]}}}`)
		case operationIs(body, "GetMedia"):
			calls["get"]++
			if body.Variables["id"] != float64(1) {
				http.Error(writer, "bad media ID", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"data":{"Media":`+mediaJSON(1, "Mobile Suit Gundam", "ANIME")+`}}`)
		case operationIs(body, "ListMedia"):
			calls["trending"]++
			if body.Variables["type"] != "MANGA" || firstValue(body.Variables["sort"]) != "TRENDING_DESC" ||
				body.Variables["page"] != float64(1) || body.Variables["perPage"] != float64(5) {
				http.Error(writer, "bad trending variables", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"data":{"Page":{"pageInfo":{"currentPage":1,"perPage":5,"hasNextPage":false},"media":[`+mediaJSON(2, "Berserk", "MANGA")+`]}}}`)
		case operationIs(body, "SeasonalMedia"):
			calls["seasonal"]++
			if body.Variables["year"] != float64(2026) || body.Variables["season"] != "SUMMER" ||
				firstValue(body.Variables["sort"]) != "SCORE_DESC" {
				http.Error(writer, "bad seasonal variables", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"data":{"Page":{"pageInfo":{"currentPage":1,"perPage":50,"hasNextPage":false},"media":[`+mediaJSON(3, "Summer Show", "ANIME")+`]}}}`)
		case strings.Contains(body.Query, "query Viewer"):
			calls["viewer"]++
			writeJSON(writer, http.StatusOK, `{"data":{"Viewer":`+userJSON(7, "fan")+`}}`)
		case operationIs(body, "User"):
			calls["user"]++
			if body.Variables["name"] != "alice" {
				http.Error(writer, "bad username", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"data":{"User":`+userJSON(8, "alice")+`}}`)
		default:
			http.Error(writer, "unknown operation", http.StatusBadRequest)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, false, false, true)

	adult := false
	search, err := client.SearchMedia(context.Background(), SearchMediaRequest{
		Query: "Gundam", Type: MediaAnime, IsAdult: &adult, Cursor: "2", Limit: 10,
	})
	if err != nil || len(search.Items) != 1 || search.Items[0].Title.English != "Mobile Suit Gundam" ||
		search.Items[0].IDMal == nil || *search.Items[0].IDMal != 101 || search.Items[0].NextAiringEpisode == nil ||
		search.NextCursor == nil || *search.NextCursor != "3" || search.PrevCursor == nil || *search.PrevCursor != "1" {
		t.Fatalf("search=%#v err=%v", search, err)
	}
	media, err := client.GetMedia(context.Background(), 1)
	if err != nil || media.ID != 1 || media.CoverImage.ExtraLarge == "" || media.StartDate.Year != 1979 || media.AverageScore == nil {
		t.Fatalf("media=%#v err=%v", media, err)
	}
	trending, err := client.ListTrendingMedia(context.Background(), ListMediaRequest{Type: MediaManga, Limit: 5})
	if err != nil || len(trending.Items) != 1 || trending.Items[0].Type != MediaManga || trending.HasMore {
		t.Fatalf("trending=%#v err=%v", trending, err)
	}
	seasonal, err := client.ListSeasonalMedia(context.Background(), SeasonalMediaRequest{
		Year: 2026, Season: SeasonSummer, Sort: MediaSortScoreDesc,
	})
	if err != nil || len(seasonal.Items) != 1 || seasonal.Items[0].SeasonYear != 2026 {
		t.Fatalf("seasonal=%#v err=%v", seasonal, err)
	}
	viewer, err := client.GetViewer(context.Background())
	if err != nil || viewer.ID != 7 || viewer.Avatar.Large == "" || viewer.IsFollowing == nil {
		t.Fatalf("viewer=%#v err=%v", viewer, err)
	}
	user, err := client.GetUser(context.Background(), UserLookup{Name: "alice"})
	if err != nil || user.ID != 8 || user.Name != "alice" {
		t.Fatalf("user=%#v err=%v", user, err)
	}
	for _, name := range []string{"search", "get", "trending", "seasonal", "viewer", "user"} {
		if calls[name] != 1 {
			t.Fatalf("calls[%s]=%d", name, calls[name])
		}
	}
}

func TestMediaAndUserNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body := readGraphQLRequest(t, writer, request)
		if operationIs(body, "GetMedia") {
			writeJSON(writer, http.StatusOK, `{"data":{"Media":null}}`)
			return
		}
		writeJSON(writer, http.StatusOK, `{"data":{"User":null}}`)
	}))
	defer server.Close()
	_, client := newTestClient(t, server, false, false, false)
	if _, err := client.GetMedia(context.Background(), 1); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("media error=%v", err)
	}
	if _, err := client.GetUser(context.Background(), UserLookup{ID: 1}); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("user error=%v", err)
	}
}

func TestMediaAndUserValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, false, false, true)
	tests := []func() error{
		func() error { _, err := client.SearchMedia(context.Background(), SearchMediaRequest{}); return err },
		func() error {
			_, err := client.SearchMedia(context.Background(), SearchMediaRequest{Query: "x", Type: "NOVEL"})
			return err
		},
		func() error {
			_, err := client.SearchMedia(context.Background(), SearchMediaRequest{Query: "x", Type: MediaAnime, Sort: "TITLE"})
			return err
		},
		func() error {
			_, err := client.SearchMedia(context.Background(), SearchMediaRequest{Query: "x", Type: MediaAnime, Limit: 51})
			return err
		},
		func() error { _, err := client.GetMedia(context.Background(), 0); return err },
		func() error {
			_, err := client.ListTrendingMedia(context.Background(), ListMediaRequest{Type: MediaAnime, Sort: MediaSortScoreDesc})
			return err
		},
		func() error {
			_, err := client.ListSeasonalMedia(context.Background(), SeasonalMediaRequest{Year: 1900, Season: SeasonWinter})
			return err
		},
		func() error {
			_, err := client.ListSeasonalMedia(context.Background(), SeasonalMediaRequest{Year: 2026, Season: "MONSOON"})
			return err
		},
		func() error { _, err := client.GetUser(context.Background(), UserLookup{}); return err },
		func() error {
			_, err := client.GetUser(context.Background(), UserLookup{ID: 1, Name: "alice"})
			return err
		},
		func() error { _, err := client.GetUser(context.Background(), UserLookup{Name: "bad/name"}); return err },
		func() error {
			_, err := client.GetMedia(context.Background(), 1, socialhub.WithFields("title"))
			return err
		},
	}
	for _, call := range tests {
		if err := call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("error=%v", err)
		}
	}
}

func mediaJSON(id int64, title, mediaType string) string {
	return `{"id":` + intString(id) + `,"idMal":` + intString(id+100) + `,"title":{"romaji":"` + title + `","english":"` + title + `","native":"Native","userPreferred":"` + title + `"},"type":"` + mediaType + `","format":"TV","status":"FINISHED","description":"Description","startDate":{"year":1979,"month":4,"day":7},"endDate":{"year":1980,"month":1,"day":26},"season":"SUMMER","seasonYear":2026,"seasonInt":202602,"episodes":43,"duration":24,"chapters":380,"volumes":42,"countryOfOrigin":"JP","isLicensed":true,"source":"ORIGINAL","coverImage":{"extraLarge":"https://cdn.example/xl.jpg","large":"https://cdn.example/l.jpg","medium":"https://cdn.example/m.jpg","color":"#ffffff"},"bannerImage":"https://cdn.example/banner.jpg","genres":["Action"],"synonyms":["Alt"],"averageScore":80,"meanScore":79,"popularity":1000,"favourites":100,"trending":10,"siteUrl":"https://anilist.co/media/` + intString(id) + `","updatedAt":1785660000,"nextAiringEpisode":{"id":99,"airingAt":1785663600,"timeUntilAiring":3600,"episode":2}}`
}

func userJSON(id int64, name string) string {
	return `{"id":` + intString(id) + `,"name":"` + name + `","about":"About","avatar":{"large":"https://cdn.example/avatar-l.jpg","medium":"https://cdn.example/avatar-m.jpg"},"bannerImage":"https://cdn.example/user-banner.jpg","siteUrl":"https://anilist.co/user/` + name + `","isFollowing":false,"isFollower":true,"isBlocked":false,"unreadNotificationCount":2,"donatorTier":1,"donatorBadge":"Supporter","createdAt":1600000000,"updatedAt":1785660000}`
}

func firstValue(value any) any {
	values, _ := value.([]any)
	if len(values) == 0 {
		return nil
	}
	return values[0]
}

func intString(value int64) string {
	return strconv.FormatInt(value, 10)
}

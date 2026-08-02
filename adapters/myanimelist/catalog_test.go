package myanimelist

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestAnimeAndMangaCatalog(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer "+testAccessToken || request.Header.Get("User-Agent") != "social-hub-myanimelist-tests/1.0" {
			http.Error(writer, "bad auth", http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/anime":
			if request.URL.Query().Get("q") != "Gundam" || request.URL.Query().Get("offset") != "50" ||
				request.URL.Query().Get("limit") != "25" || request.URL.Query().Get("fields") != "mean,rank" {
				http.Error(writer, "bad anime search", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"data":[{"node":{"id":1,"title":"Mobile Suit Gundam","main_picture":{"medium":"https://cdn.example/gundam.jpg"},"mean":7.7,"rank":900}}],"paging":{"previous":"https://api.example/anime?offset=0","next":"https://api.example/anime?offset=100"}}`)
		case "/anime/1":
			if request.URL.Query().Get("fields") != "pictures,related_anime,statistics" {
				http.Error(writer, "bad anime fields", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, animeDetailsJSON())
		case "/anime/ranking":
			if request.URL.Query().Get("ranking_type") != "airing" {
				http.Error(writer, "bad ranking", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"data":[{"node":{"id":2,"title":"Current Show"},"ranking":{"rank":1,"previous_rank":2}}],"paging":{}}`)
		case "/anime/season/2026/summer":
			if request.URL.Query().Get("sort") != "anime_score" {
				http.Error(writer, "bad seasonal", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"data":[{"node":{"id":3,"title":"Summer Show","start_season":{"year":2026,"season":"summer"}}}],"paging":{}}`)
		case "/anime/suggestions":
			writeJSON(writer, http.StatusOK, `{"data":[{"node":{"id":4,"title":"Suggested Show"}}],"paging":{}}`)
		case "/manga":
			if request.URL.Query().Get("q") != "Berserk" {
				http.Error(writer, "bad manga search", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"data":[{"node":{"id":10,"title":"Berserk","num_volumes":42}}],"paging":{}}`)
		case "/manga/10":
			if request.URL.Query().Get("fields") != "authors{first_name,last_name},serialization" {
				http.Error(writer, "bad manga fields", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, mangaDetailsJSON())
		case "/manga/ranking":
			if request.URL.Query().Get("ranking_type") != "manga" {
				http.Error(writer, "bad manga ranking", http.StatusBadRequest)
				return
			}
			writeJSON(writer, http.StatusOK, `{"data":[{"node":{"id":11,"title":"Ranked Manga"},"ranking":{"rank":2}}],"paging":{}}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, false, true, nil)

	animePage, err := client.SearchAnime(context.Background(), SearchRequest{Query: "Gundam", Cursor: "50", Limit: 25}, socialhub.WithFields("mean", "rank"))
	if err != nil || len(animePage.Items) != 1 || animePage.Items[0].Title != "Mobile Suit Gundam" ||
		animePage.NextCursor == nil || *animePage.NextCursor != "100" || animePage.PrevCursor == nil || *animePage.PrevCursor != "0" {
		t.Fatalf("anime page=%#v err=%v", animePage, err)
	}
	anime, err := client.GetAnime(context.Background(), 1, socialhub.WithFields("pictures", "related_anime", "statistics"))
	if err != nil || anime.MediaType != "tv" || anime.NumEpisodes != 43 || anime.StartSeason == nil ||
		anime.StartSeason.Season != SeasonSpring || len(anime.RelatedAnime) != 1 || anime.Statistics == nil ||
		anime.Statistics.Status.Completed != 80 || anime.CreatedAt.IsZero() {
		t.Fatalf("anime=%#v err=%v", anime, err)
	}
	rankedAnime, err := client.ListAnimeRanking(context.Background(), AnimeRankingRequest{Type: AnimeRankingAiring, Limit: 10})
	if err != nil || rankedAnime.Items[0].Ranking.Rank != 1 || rankedAnime.Items[0].Ranking.PreviousRank == nil {
		t.Fatalf("ranked anime=%#v err=%v", rankedAnime, err)
	}
	seasonal, err := client.ListSeasonalAnime(context.Background(), SeasonalAnimeRequest{Year: 2026, Season: SeasonSummer, Sort: SeasonalSortScore})
	if err != nil || seasonal.Items[0].StartSeason == nil || seasonal.Items[0].StartSeason.Year != 2026 {
		t.Fatalf("seasonal=%#v err=%v", seasonal, err)
	}
	suggestions, err := client.ListAnimeSuggestions(context.Background(), PageRequest{Limit: 5})
	if err != nil || suggestions.Items[0].ID != 4 {
		t.Fatalf("suggestions=%#v err=%v", suggestions, err)
	}

	mangaPage, err := client.SearchManga(context.Background(), SearchRequest{Query: "Berserk"})
	if err != nil || mangaPage.Items[0].NumVolumes != 42 {
		t.Fatalf("manga page=%#v err=%v", mangaPage, err)
	}
	manga, err := client.GetManga(context.Background(), 10, socialhub.WithFields("authors{first_name,last_name}", "serialization"))
	if err != nil || manga.NumChapters != 380 || len(manga.Authors) != 1 || manga.Authors[0].Person.LastName != "Miura" ||
		len(manga.Serialization) != 1 || len(manga.RelatedAnime) != 1 || len(manga.Recommendations) != 1 {
		t.Fatalf("manga=%#v err=%v", manga, err)
	}
	rankedManga, err := client.ListMangaRanking(context.Background(), MangaRankingRequest{Type: MangaRankingManga})
	if err != nil || rankedManga.Items[0].Manga.ID != 11 || rankedManga.Items[0].Ranking.Rank != 2 {
		t.Fatalf("ranked manga=%#v err=%v", rankedManga, err)
	}
}

func TestCatalogValidationAndPagingErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writeJSON(writer, http.StatusOK, `{"data":[],"paging":{"next":"https://api.example/anime?page=2"}}`)
	}))
	defer server.Close()
	_, client := newTestClient(t, server, false, true, nil)

	tests := []func() error{
		func() error { _, err := client.SearchAnime(context.Background(), SearchRequest{}); return err },
		func() error {
			_, err := client.SearchAnime(context.Background(), SearchRequest{Query: "x", Cursor: "01"})
			return err
		},
		func() error {
			_, err := client.SearchManga(context.Background(), SearchRequest{Query: "x", Limit: 101})
			return err
		},
		func() error { _, err := client.GetAnime(context.Background(), 0); return err },
		func() error { _, err := client.GetManga(context.Background(), -1); return err },
		func() error {
			_, err := client.ListAnimeRanking(context.Background(), AnimeRankingRequest{Type: "weekly"})
			return err
		},
		func() error {
			_, err := client.ListMangaRanking(context.Background(), MangaRankingRequest{Type: "weekly"})
			return err
		},
		func() error {
			_, err := client.ListSeasonalAnime(context.Background(), SeasonalAnimeRequest{Year: 1800, Season: SeasonWinter})
			return err
		},
		func() error {
			_, err := client.ListSeasonalAnime(context.Background(), SeasonalAnimeRequest{Year: 2026, Season: "monsoon"})
			return err
		},
		func() error {
			_, err := client.ListSeasonalAnime(context.Background(), SeasonalAnimeRequest{Year: 2026, Season: SeasonFall, Sort: "title"})
			return err
		},
		func() error {
			_, err := client.ListAnimeSuggestions(context.Background(), PageRequest{Limit: -1})
			return err
		},
		func() error {
			_, err := client.GetAnime(context.Background(), 1, socialhub.WithFields("bad field"))
			return err
		},
		func() error {
			_, err := client.GetManga(context.Background(), 1, socialhub.WithFields("authors{first_name"))
			return err
		},
	}
	for _, call := range tests {
		if err := call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("error=%v", err)
		}
	}
	_, err := client.SearchAnime(context.Background(), SearchRequest{Query: "paging"})
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodePlatformError {
		t.Fatalf("paging error=%v", err)
	}
}

func animeDetailsJSON() string {
	return `{"id":1,"title":"Mobile Suit Gundam","alternative_titles":{"en":"Mobile Suit Gundam","ja":"Kidou Senshi Gundam"},"start_date":"1979-04-07","end_date":"1980-01-26","synopsis":"A war story","mean":7.7,"rank":900,"popularity":500,"num_list_users":100000,"num_scoring_users":80000,"nsfw":"white","genres":[{"id":1,"name":"Action"}],"created_at":"2020-01-02T03:04:05Z","updated_at":"2026-01-02T03:04:05Z","media_type":"tv","status":"finished_airing","num_episodes":43,"start_season":{"year":1979,"season":"spring"},"broadcast":{"day_of_the_week":"saturday","start_time":"17:30"},"source":"original","average_episode_duration":1440,"rating":"pg_13","studios":[{"id":14,"name":"Sunrise"}],"pictures":[{"medium":"https://cdn.example/1.jpg","large":"https://cdn.example/1l.jpg"}],"related_anime":[{"node":{"id":2,"title":"Zeta Gundam"},"relation_type":"sequel","relation_type_formatted":"Sequel"}],"related_manga":[{"node":{"id":20,"title":"Gundam Manga"},"relation_type":"alternative_version","relation_type_formatted":"Alternative version"}],"recommendations":[{"node":{"id":3,"title":"Space Runaway Ideon"},"num_recommendations":12}],"statistics":{"num_list_users":100,"status":{"watching":5,"completed":80,"on_hold":5,"dropped":5,"plan_to_watch":5}}}`
}

func mangaDetailsJSON() string {
	return `{"id":10,"title":"Berserk","media_type":"manga","status":"currently_publishing","num_volumes":42,"num_chapters":380,"authors":[{"node":{"id":99,"first_name":"Kentaro","last_name":"Miura"},"role":"Story & Art"}],"pictures":[{"medium":"https://cdn.example/berserk.jpg"}],"related_anime":[{"node":{"id":30,"title":"Berserk TV"},"relation_type":"adaptation","relation_type_formatted":"Adaptation"}],"related_manga":[{"node":{"id":31,"title":"Berserk Guidebook"},"relation_type":"side_story","relation_type_formatted":"Side story"}],"recommendations":[{"node":{"id":32,"title":"Vagabond"},"num_recommendations":50}],"serialization":[{"node":{"id":2,"name":"Young Animal"},"role":"Serialization"}]}`
}

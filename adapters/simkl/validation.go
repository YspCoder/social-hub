package simkl

import (
	"math"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

const (
	maxCredentialLength = 8192
	maxReferenceLength  = 2048
	maxApplicationValue = 128
	maxUserAgentLength  = 256
	maxQueryLength      = 512
	maxIdentifierLength = 512
	maxTitleLength      = 2048
	maxMemoLength       = 140
	maxSearchPage       = 20
	maxSearchPageSize   = 50
)

var (
	mediaTypes        = []MediaType{MediaMovie, MediaTV, MediaAnime}
	searchExtensions  = []SearchExtended{SearchSimple, SearchFull}
	trendingPeriods   = []TrendingPeriod{TrendingToday, TrendingWeek, TrendingMonth}
	syncMediaTypes    = []SyncMediaType{SyncMovies, SyncShows, SyncAnime, SyncAll}
	watchlistStatuses = []WatchlistStatus{StatusWatching, StatusPlanToWatch, StatusHold, StatusCompleted, StatusDropped, StatusAll}
	syncExtensions    = []SyncExtended{SyncFull, SyncFullAnimeSeasons, SyncSimklIDsOnly, SyncIDsOnly}
	includeEpisodes   = []IncludeEpisodes{IncludeEpisodesYes, IncludeEpisodesOriginal, IncludeEpisodesNo}
)

func validEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" &&
		parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == ""
}

func validRedirectURI(value string) bool {
	if value == "" || len(value) > maxReferenceLength || strings.TrimSpace(value) != value || containsControl(value) {
		return false
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.User != nil || parsed.Fragment != "" {
		return false
	}
	switch strings.ToLower(parsed.Scheme) {
	case "javascript", "data", "file":
		return false
	}
	return parsed.Host != "" || parsed.Opaque != "" || parsed.Path != ""
}

func validCredential(value string) bool {
	return value != "" && len(value) <= maxCredentialLength && strings.TrimSpace(value) == value && !containsControl(value)
}

func validReference(value string) bool {
	return value != "" && len(value) <= maxReferenceLength && strings.TrimSpace(value) == value && !containsControl(value)
}

func validApplicationValue(value string) bool {
	return value != "" && len(value) <= maxApplicationValue && strings.TrimSpace(value) == value && !containsControl(value)
}

func validUserAgent(value string) bool {
	return value != "" && len(value) <= maxUserAgentLength && strings.TrimSpace(value) == value && !containsControl(value)
}

func validOpaque(value string, maximum int) bool {
	return value != "" && len(value) <= maximum && strings.TrimSpace(value) == value && !containsControl(value)
}

func validIdentifier(value string) bool {
	return value != "" && len(value) <= maxIdentifierLength && strings.TrimSpace(value) == value &&
		!strings.ContainsAny(value, "/\\?#") && !containsControl(value)
}

func validOptionalIdentifier(value string) bool { return value == "" || validIdentifier(value) }

func validTitle(value string) bool {
	return value != "" && len(value) <= maxTitleLength && strings.TrimSpace(value) != "" && !containsControl(value)
}

func containsControl(value string) bool {
	for _, character := range value {
		if unicode.IsControl(character) {
			return true
		}
	}
	return false
}

func validateSearchPage(cursor string, limit int) (int, error) {
	if limit < 0 || limit > maxSearchPageSize {
		return 0, invalidArgument("pagination", "limit must be between 1 and 50 when set")
	}
	if cursor == "" {
		return 0, nil
	}
	page, err := strconv.Atoi(cursor)
	if err != nil || page < 1 || page > maxSearchPage || strconv.Itoa(page) != cursor {
		return 0, invalidArgument("pagination", "cursor must be a page number between 1 and 20")
	}
	return page, nil
}

func validIDs(ids IDs) bool {
	if ids.Simkl < 0 || (ids.Simkl == 0 && ids.Slug == "" && ids.IMDB == "" && ids.TMDB == "" && ids.TVDB == "" &&
		ids.MAL == "" && ids.AniDB == "" && ids.AniList == "" && ids.Kitsu == "" && ids.AniSearch == "" &&
		ids.AnimePlanet == "" && ids.LiveChart == "" && ids.Letterboxd == "" && ids.Netflix == "" &&
		ids.Hulu == "" && ids.Crunchyroll == "" && ids.TraktSlug == "") {
		return false
	}
	values := []string{ids.Slug, ids.IMDB, ids.TMDB, ids.TVDB, ids.MAL, ids.AniDB, ids.AniList, ids.Kitsu,
		ids.AniSearch, ids.AnimePlanet, ids.LiveChart, ids.Letterboxd, ids.Netflix, ids.Hulu, ids.Crunchyroll, ids.TraktSlug}
	for _, value := range values {
		if !validOptionalIdentifier(value) {
			return false
		}
	}
	return true
}

func validMediaRef(ref MediaRef) bool {
	return validIDs(ref.IDs) && (ref.Title == "" || validTitle(ref.Title)) && (ref.Year == 0 || ref.Year >= 1800 && ref.Year <= 3000)
}

func validStatus(status WatchlistStatus, allowAll bool) bool {
	return slices.Contains(watchlistStatuses, status) && (allowAll || status != StatusAll)
}

func validateHistoryMedia(item HistoryMedia, movie bool) bool {
	if !validMediaRef(item.MediaRef) || item.Rating < 0 || item.Rating > 10 ||
		(item.Status != "" && !validStatus(item.Status, false)) {
		return false
	}
	if movie && (item.Status == StatusWatching || item.Status == StatusHold) {
		return false
	}
	return item.Memo == nil || item.Memo.Text != "" && len(item.Memo.Text) <= maxMemoLength &&
		strings.TrimSpace(item.Memo.Text) != "" && !containsControl(item.Memo.Text)
}

func validateHistorySeries(item HistorySeries) bool {
	if !validateHistoryMedia(item.HistoryMedia, false) {
		return false
	}
	for _, season := range item.Seasons {
		if season.Number < 0 || !validEpisodes(season.Episodes) {
			return false
		}
	}
	return validEpisodes(item.Episodes)
}

func validEpisodes(episodes []EpisodeRef) bool {
	for _, episode := range episodes {
		if episode.Number < 1 || (!zeroIDs(episode.IDs) && !validIDs(episode.IDs)) {
			return false
		}
	}
	return true
}

func zeroIDs(ids IDs) bool { return ids == (IDs{}) }

func validEpisodeIDs(ids IDs) bool {
	return ids.Simkl == 0 && ids.Slug == "" && ids.IMDB == "" && ids.TMDB == "" && ids.MAL == "" &&
		ids.AniList == "" && ids.Kitsu == "" && ids.AniSearch == "" && ids.AnimePlanet == "" &&
		ids.LiveChart == "" && ids.Letterboxd == "" && ids.Netflix == "" && ids.Hulu == "" &&
		ids.Crunchyroll == "" && ids.TraktSlug == "" &&
		((ids.TVDB != "" && ids.AniDB == "" && validIdentifier(ids.TVDB)) ||
			(ids.AniDB != "" && ids.TVDB == "" && validIdentifier(ids.AniDB)))
}

func validProgress(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value <= 100
}

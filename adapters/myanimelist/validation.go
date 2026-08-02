package myanimelist

import (
	"net/url"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

const (
	scopeWriteUsers     = "write:users"
	maxCredentialLength = 8192
	maxIdentifierLength = 256
	maxQueryLength      = 512
	maxFieldsLength     = 2048
	maxPageSize         = 100
	maxOffset           = 1_000_000_000
	maxCommentLength    = 100_000
)

var animeRankingTypes = []AnimeRankingType{
	AnimeRankingAll, AnimeRankingAiring, AnimeRankingUpcoming, AnimeRankingTV,
	AnimeRankingOVA, AnimeRankingMovie, AnimeRankingSpecial, AnimeRankingPopularity, AnimeRankingFavorite,
}

var mangaRankingTypes = []MangaRankingType{
	MangaRankingAll, MangaRankingManga, MangaRankingNovels, MangaRankingOneShots,
	MangaRankingDoujin, MangaRankingManhwa, MangaRankingManhua, MangaRankingPopularity, MangaRankingFavorite,
}

var animeListStates = []AnimeListState{AnimeWatching, AnimeCompleted, AnimeOnHold, AnimeDropped, AnimePlanToWatch}
var mangaListStates = []MangaListState{MangaReading, MangaCompleted, MangaOnHold, MangaDropped, MangaPlanToRead}
var animeListSorts = []AnimeListSort{AnimeListSortScore, AnimeListSortUpdatedAt, AnimeListSortTitle, AnimeListSortStartDate}
var mangaListSorts = []MangaListSort{MangaListSortScore, MangaListSortUpdatedAt, MangaListSortTitle, MangaListSortStartDate}

func validEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" &&
		parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == ""
}

func validRedirectURI(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" &&
		parsed.User == nil && parsed.Fragment == ""
}

func validCredential(value string) bool {
	return value != "" && len(value) <= maxCredentialLength && strings.TrimSpace(value) == value && !containsControl(value)
}

func validReference(value string) bool {
	return value != "" && len(value) <= 2048 && strings.TrimSpace(value) == value && !containsControl(value)
}

func validUserAgent(value string) bool {
	return value != "" && len(value) <= 256 && strings.TrimSpace(value) == value && !containsControl(value)
}

func validOpaque(value string, maximum int) bool {
	return value != "" && len(value) <= maximum && strings.TrimSpace(value) == value && !containsControl(value)
}

func validPKCEValue(value string) bool {
	if len(value) < 43 || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if character >= 'A' && character <= 'Z' || character >= 'a' && character <= 'z' ||
			character >= '0' && character <= '9' || strings.ContainsRune("-._~", character) {
			continue
		}
		return false
	}
	return true
}

func validQuery(value string) bool {
	return value != "" && len(value) <= maxQueryLength && strings.TrimSpace(value) == value && !containsControl(value)
}

func validUsername(value string) bool {
	return value != "" && len(value) <= maxIdentifierLength && strings.TrimSpace(value) == value &&
		!strings.ContainsAny(value, "/\\?#") && !containsControl(value)
}

func validPage(cursor string, limit int) bool {
	if limit < 0 || limit > maxPageSize {
		return false
	}
	if cursor == "" {
		return true
	}
	offset, err := strconv.ParseInt(cursor, 10, 64)
	return err == nil && offset >= 0 && offset <= maxOffset && strconv.FormatInt(offset, 10) == cursor
}

func validFields(fields []string) bool {
	if len(fields) == 0 {
		return true
	}
	joined := strings.Join(fields, ",")
	if len(joined) > maxFieldsLength || strings.TrimSpace(joined) != joined {
		return false
	}
	depth := 0
	for _, character := range joined {
		switch {
		case unicode.IsLetter(character), unicode.IsDigit(character), character == '_', character == ',':
		case character == '{':
			depth++
		case character == '}':
			depth--
			if depth < 0 {
				return false
			}
		default:
			return false
		}
	}
	return depth == 0
}

func validScopes(scopes []string) bool {
	seen := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		if scope != scopeWriteUsers {
			return false
		}
		if _, exists := seen[scope]; exists {
			return false
		}
		seen[scope] = struct{}{}
	}
	return true
}

func containsScope(scopes []string, scope string) bool { return slices.Contains(scopes, scope) }

func validAnimeRanking(value AnimeRankingType) bool { return slices.Contains(animeRankingTypes, value) }
func validMangaRanking(value MangaRankingType) bool { return slices.Contains(mangaRankingTypes, value) }
func validAnimeListState(value AnimeListState) bool {
	return value == "" || slices.Contains(animeListStates, value)
}
func validMangaListState(value MangaListState) bool {
	return value == "" || slices.Contains(mangaListStates, value)
}
func validAnimeListSort(value AnimeListSort) bool {
	return value == "" || slices.Contains(animeListSorts, value)
}
func validMangaListSort(value MangaListSort) bool {
	return value == "" || slices.Contains(mangaListSorts, value)
}

func validSeason(value AnimeSeason) bool {
	return value == SeasonWinter || value == SeasonSpring || value == SeasonSummer || value == SeasonFall
}

func validSeasonalSort(value SeasonalAnimeSort) bool {
	return value == "" || value == SeasonalSortScore || value == SeasonalSortListUsers
}

func validScore(value *int) bool       { return value == nil || *value >= 0 && *value <= 10 }
func validPriority(value *int) bool    { return value == nil || *value >= 0 && *value <= 2 }
func validRepeatValue(value *int) bool { return value == nil || *value >= 0 && *value <= 5 }
func validCount(value *int) bool       { return value == nil || *value >= 0 }

func validTags(tags []string) bool {
	if tags == nil {
		return true
	}
	if len(tags) > 100 || len(strings.Join(tags, ",")) > 4096 {
		return false
	}
	for _, tag := range tags {
		if !validOpaque(tag, 100) || strings.Contains(tag, ",") {
			return false
		}
	}
	return true
}

func validComment(value *string) bool {
	return value == nil || len(*value) <= maxCommentLength && !containsDisallowedControl(*value)
}

func containsControl(value string) bool {
	for _, character := range value {
		if unicode.IsControl(character) {
			return true
		}
	}
	return false
}

func containsDisallowedControl(value string) bool {
	for _, character := range value {
		if unicode.IsControl(character) && character != '\n' && character != '\r' && character != '\t' {
			return true
		}
	}
	return false
}

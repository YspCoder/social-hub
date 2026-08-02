package anilist

import (
	"math"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	maxCredentialLength = 8192
	maxReferenceLength  = 2048
	maxStateLength      = 2048
	maxNameLength       = 256
	maxQueryLength      = 512
	maxTextLength       = 100_000
	maxPageSize         = 50
	maxPageNumber       = 1_000_000
)

var mediaTypes = []MediaType{MediaAnime, MediaManga}
var mediaSorts = []MediaSort{
	MediaSortSearchMatch, MediaSortTrendingDesc, MediaSortPopularityDesc,
	MediaSortScoreDesc, MediaSortStartDateDesc,
}
var mediaSeasons = []MediaSeason{SeasonWinter, SeasonSpring, SeasonSummer, SeasonFall}
var mediaListStatuses = []MediaListStatus{ListCurrent, ListPlanning, ListCompleted, ListDropped, ListPaused, ListRepeating}
var mediaListSorts = []MediaListSort{MediaListSortUpdatedDesc, MediaListSortScoreDesc, MediaListSortProgressDesc, MediaListSortTitle}
var activityTypes = []ActivityType{ActivityText, ActivityAnimeList, ActivityMangaList, ActivityMediaList}
var likeableTypes = []LikeableType{LikeActivity, LikeActivityReply}

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

func validUserAgent(value string) bool {
	return value != "" && len(value) <= maxNameLength && strings.TrimSpace(value) == value && !containsControl(value)
}

func validState(value string) bool {
	return value != "" && len(value) <= maxStateLength && strings.TrimSpace(value) == value && !containsControl(value)
}

func validOpaque(value string, maximum int) bool {
	return value != "" && len(value) <= maximum && strings.TrimSpace(value) == value && !containsControl(value)
}

func validID(value int64) bool { return value > 0 && value <= math.MaxInt32 }

func validPage(cursor string, limit int) (int, bool) {
	if limit < 0 || limit > maxPageSize {
		return 0, false
	}
	if cursor == "" {
		return 1, true
	}
	page, err := strconv.Atoi(cursor)
	return page, err == nil && page > 0 && page <= maxPageNumber && strconv.Itoa(page) == cursor
}

func validMediaType(value MediaType) bool { return slices.Contains(mediaTypes, value) }
func validMediaSort(value MediaSort) bool { return value == "" || slices.Contains(mediaSorts, value) }
func validMediaSeason(value MediaSeason) bool {
	return slices.Contains(mediaSeasons, value)
}
func validMediaListStatus(value MediaListStatus) bool {
	return value == "" || slices.Contains(mediaListStatuses, value)
}
func validConcreteMediaListStatus(value MediaListStatus) bool {
	return slices.Contains(mediaListStatuses, value)
}
func validMediaListSort(value MediaListSort) bool {
	return value == "" || slices.Contains(mediaListSorts, value)
}
func validActivityTypes(values []ActivityType) bool {
	seen := make(map[ActivityType]struct{}, len(values))
	for _, value := range values {
		if !slices.Contains(activityTypes, value) {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}
func validLikeableType(value LikeableType) bool { return slices.Contains(likeableTypes, value) }

func validSearch(value string) bool {
	return value != "" && len(value) <= maxQueryLength && strings.TrimSpace(value) == value && !containsControl(value)
}

func validUsername(value string) bool {
	return value != "" && len(value) <= maxNameLength && strings.TrimSpace(value) == value &&
		!strings.ContainsAny(value, "/\\?#") && !containsControl(value)
}

func validFuzzyDate(value FuzzyDate) bool {
	if value.Year < 0 || value.Year > 3000 || value.Month < 0 || value.Month > 12 || value.Day < 0 || value.Day > 31 {
		return false
	}
	if value.Day > 0 && value.Month == 0 || value.Month > 0 && value.Year == 0 {
		return false
	}
	if value.Year > 0 && value.Month > 0 && value.Day > 0 {
		date := time.Date(value.Year, time.Month(value.Month), value.Day, 0, 0, 0, 0, time.UTC)
		return date.Year() == value.Year && int(date.Month()) == value.Month && date.Day() == value.Day
	}
	return true
}

func validScore(value *float64) bool { return value == nil || *value >= 0 && *value <= 100 }
func validCount(value *int) bool     { return value == nil || *value >= 0 && *value <= math.MaxInt32 }
func validPriority(value *int) bool  { return value == nil || *value >= 0 && *value <= 5 }

func validText(value string) bool {
	return value != "" && len(value) <= maxTextLength && strings.TrimSpace(value) != "" && !containsDisallowedControl(value)
}

func validOptionalText(value *string) bool {
	return value == nil || len(*value) <= maxTextLength && !containsDisallowedControl(*value)
}

func validCustomLists(values []string) bool {
	if values == nil {
		return true
	}
	if len(values) > 100 {
		return false
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value == "" || len(value) > maxNameLength || strings.TrimSpace(value) != value || containsControl(value) {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
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

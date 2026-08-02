package letterboxd

import (
	"math"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode"
)

const (
	maxCredentialLength = 4096
	maxIdentifierLength = 512
	maxTextLength       = 100000
	maxSearchLength     = 512
	maxPageSize         = 100
)

var allowedScopes = []string{
	"profile:private:view", "profile:modify", "security:modify", "content:modify",
	"oauth:refresh", "openid", "profile", "email",
}

var allowedSearchMethods = []string{"FullText", "Autocomplete", "NamesAndKeywords"}

var allowedSearchTypes = []string{
	"ContributorSearchItem", "FilmSearchItem", "ListSearchItem", "MemberSearchItem", "ReviewSearchItem",
	"TagSearchItem", "StorySearchItem", "ArticleSearchItem", "PodcastSearchItem", "ShowSearchItem",
}

var allowedLogWhere = []string{
	"HasDiaryDate", "HasReview", "Clean", "NoSpoilers", "NoDrafts", "Released", "NotReleased",
	"FeatureLength", "NotFeatureLength", "InWatchlist", "NotInWatchlist", "Watched", "NotWatched",
	"Rated", "NotRated", "Logged", "NotLogged", "Rewatched", "NotRewatched", "Reviewed", "NotReviewed",
	"Owned", "NotOwned", "Customised", "NotCustomised", "CustomisedBackdrop", "NotCustomisedBackdrop",
	"AddedPrivateNote", "NotAddedPrivateNote", "Liked", "NotLiked", "Fiction", "Film", "TV",
}

func validEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" &&
		parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == ""
}

func validRedirectURI(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" && parsed.User == nil && parsed.Fragment == ""
}

func validCredential(value string) bool {
	return value != "" && len(value) <= maxCredentialLength && strings.TrimSpace(value) == value && !containsControl(value)
}

func validOpaque(value string, maximum int) bool {
	return value != "" && len(value) <= maximum && strings.TrimSpace(value) == value && !containsControl(value)
}

func validReference(value string) bool {
	return value != "" && len(value) <= 2048 && strings.TrimSpace(value) == value && !containsControl(value)
}

func validIdentifier(value string) bool {
	return value != "" && len(value) <= maxIdentifierLength && strings.TrimSpace(value) == value &&
		!strings.ContainsAny(value, "/\\?#") && !containsControl(value)
}

func validSearch(value string) bool {
	return value != "" && len(value) <= maxSearchLength && strings.TrimSpace(value) == value && !containsControl(value)
}

func validText(value string) bool {
	if value == "" || len(value) > maxTextLength || strings.TrimSpace(value) == "" {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) && character != '\n' && character != '\r' && character != '\t' {
			return false
		}
	}
	return true
}

func validUserAgent(value string) bool {
	return value != "" && len(value) <= 256 && strings.TrimSpace(value) == value && !containsControl(value)
}

func validRating(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0.5 && value <= 5 && math.Mod(value*2, 1) == 0
}

func validPage(cursor string, perPage int) bool {
	return (cursor == "" || (len(cursor) <= 2048 && strings.TrimSpace(cursor) == cursor && !containsControl(cursor))) &&
		perPage >= 0 && perPage <= maxPageSize
}

func validScopes(scopes []string) bool {
	seen := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		if !slices.Contains(allowedScopes, scope) {
			return false
		}
		if _, exists := seen[scope]; exists {
			return false
		}
		seen[scope] = struct{}{}
	}
	return true
}

func validUniqueValues(values, allowed []string) bool {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !slices.Contains(allowed, value) {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validQueryValue(value string, maximum int) bool {
	return value == "" || (len(value) <= maximum && strings.TrimSpace(value) == value && !containsControl(value))
}

func validYear(value int) bool { return value == 0 || value >= 1888 && value <= 3000 }

func validDecade(value int) bool { return value == 0 || validYear(value) && value%10 == 0 }

func validDate(value string) bool {
	if len(value) != len("2006-01-02") {
		return false
	}
	parsed, err := time.Parse("2006-01-02", value)
	return err == nil && parsed.Format("2006-01-02") == value
}

func validCommentPolicy(value string) bool {
	return value == "" || value == "Anyone" || value == "Friends" || value == "You"
}

func validPrivacyPolicy(value string) bool {
	return validCommentPolicy(value) || value == "Draft"
}

func validTags(tags []string) bool {
	if len(tags) > 100 {
		return false
	}
	for _, tag := range tags {
		if !validOpaque(tag, 100) {
			return false
		}
	}
	return true
}

func containsScope(scopes []string, target string) bool {
	return slices.Contains(scopes, target)
}

func containsControl(value string) bool {
	for _, character := range value {
		if unicode.IsControl(character) {
			return true
		}
	}
	return false
}

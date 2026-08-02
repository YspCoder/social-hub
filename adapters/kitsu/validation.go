package kitsu

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
	maxNameLength       = 256
	maxQueryLength      = 512
	maxTextLength       = 9000
	maxPageSize         = 20
	maxOffset           = math.MaxInt32
)

var libraryStatuses = []LibraryStatus{LibraryCurrent, LibraryPlanned, LibraryCompleted, LibraryOnHold, LibraryDropped}
var mediaKinds = []MediaKind{MediaAnime, MediaManga}

func validEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" &&
		parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == ""
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

func validID(value string) bool {
	if value == "" || len(value) > 20 || strings.TrimSpace(value) != value {
		return false
	}
	number, err := strconv.ParseUint(value, 10, 64)
	return err == nil && number > 0 && strconv.FormatUint(number, 10) == value
}

func validPage(cursor string, limit int) (int, bool) {
	if limit < 0 || limit > maxPageSize {
		return 0, false
	}
	if cursor == "" {
		return 0, true
	}
	offset, err := strconv.Atoi(cursor)
	return offset, err == nil && offset >= 0 && offset <= maxOffset && strconv.Itoa(offset) == cursor
}

func validSearch(value string) bool {
	return value != "" && len(value) <= maxQueryLength && strings.TrimSpace(value) == value && !containsControl(value)
}

func validSlug(value string) bool {
	return value != "" && len(value) <= maxNameLength && strings.TrimSpace(value) == value &&
		!strings.ContainsAny(value, "/\\?#") && !containsControl(value)
}

func validText(value string) bool {
	return value != "" && len(value) <= maxTextLength && strings.TrimSpace(value) != "" && !containsDisallowedControl(value)
}

func validOptionalText(value *string) bool {
	return value == nil || len(*value) <= maxTextLength && !containsDisallowedControl(*value)
}

func validLibraryStatus(value LibraryStatus) bool {
	return value == "" || slices.Contains(libraryStatuses, value)
}

func validConcreteLibraryStatus(value *LibraryStatus) bool {
	return value == nil || slices.Contains(libraryStatuses, *value)
}

func validMediaKind(value MediaKind) bool { return value == "" || slices.Contains(mediaKinds, value) }

func validRating(value *int) bool { return value == nil || *value >= 2 && *value <= 20 }
func validCount(value *int) bool  { return value == nil || *value >= 0 && *value <= math.MaxInt32 }

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

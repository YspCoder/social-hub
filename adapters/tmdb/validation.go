package tmdb

import (
	"math"
	"net/url"
	"strconv"
	"strings"
	"unicode"
)

const (
	maxCredentialLength = 4096
	maxReferenceLength  = 2048
	maxSearchLength     = 512
	maxPageNumber       = 500
)

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

func validReference(value string) bool {
	return value != "" && len(value) <= maxReferenceLength && strings.TrimSpace(value) == value && !containsControl(value)
}

func validSearch(value string) bool {
	return value != "" && len(value) <= maxSearchLength && strings.TrimSpace(value) == value && !containsControl(value)
}

func validUserAgent(value string) bool {
	return value != "" && len(value) <= 256 && strings.TrimSpace(value) == value && !containsControl(value)
}

func validLocale(value string) bool {
	if value == "" {
		return true
	}
	if len(value) < 2 || len(value) > 16 {
		return false
	}
	for _, character := range value {
		if !unicode.IsLetter(character) && !unicode.IsDigit(character) && character != '-' && character != '_' {
			return false
		}
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

func validatePage(cursor string) (int, error) {
	if cursor == "" {
		return 1, nil
	}
	page, err := strconv.Atoi(cursor)
	if err != nil || page < 1 || page > maxPageNumber {
		return 0, invalidArgument("pagination", "cursor must be a page number between 1 and 500")
	}
	return page, nil
}

func validMediaType(value MediaType, allowAll, allowPerson bool) bool {
	if value == MediaMovie || value == MediaTV {
		return true
	}
	return (allowAll && value == MediaAll) || (allowPerson && value == MediaPerson)
}

func validSort(value string) bool {
	return value == "" || value == "created_at.asc" || value == "created_at.desc"
}

func validRating(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0.5 && value <= 10 && math.Mod(value*2, 1) == 0
}

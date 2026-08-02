package applemusic

import (
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxCursorOffset = 1_000_000_000

func validEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == ""
}

func validAppleIdentifier(value string) bool {
	if len(value) != 10 {
		return false
	}
	for _, character := range value {
		if character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' {
			continue
		}
		return false
	}
	return true
}

func validStorefront(value string) bool {
	if len(value) != 2 {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' {
			continue
		}
		return false
	}
	return true
}

func validCredential(value string) bool {
	return value != "" && strings.TrimSpace(value) == value && len(value) <= 16*1024 && !strings.ContainsFunc(value, unicode.IsControl)
}

func validIdentifier(value string) bool {
	if value == "" || len(value) > 1024 || strings.TrimSpace(value) != value || strings.ContainsAny(value, "/,?#") {
		return false
	}
	return !strings.ContainsFunc(value, unicode.IsControl)
}

func validText(value string, allowEmpty bool, maximum int) bool {
	return utf8.ValidString(value) && utf8.RuneCountInString(value) <= maximum && !strings.ContainsRune(value, '\x00') &&
		(allowEmpty || strings.TrimSpace(value) != "")
}

func parseOffset(value string) (string, bool) {
	if value == "" {
		return "", true
	}
	offset, err := strconv.Atoi(value)
	return value, err == nil && offset >= 0 && offset <= maxCursorOffset
}

func validLanguage(value string) bool {
	if value == "" {
		return true
	}
	if len(value) > 35 || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func validUniqueTypes[T ~string](values []T, allowed map[T]struct{}) bool {
	if len(values) == 0 {
		return false
	}
	seen := make(map[T]struct{}, len(values))
	for _, value := range values {
		if _, ok := allowed[value]; !ok {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

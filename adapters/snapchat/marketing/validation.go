package marketing

import (
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

func validUUID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	for index := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			continue
		}
		character := value[index]
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f') || (character >= 'A' && character <= 'F')) {
			return false
		}
	}
	return true
}

func validOpaque(value string, maximum int) bool {
	if value == "" || len(value) > maximum || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validText(value string, maximum int) bool {
	return strings.TrimSpace(value) != "" && strings.TrimSpace(value) == value && len(value) <= maximum &&
		utf8.ValidString(value) && !strings.ContainsRune(value, '\x00')
}

func validUpperIdentifier(value string) bool {
	if value == "" || len(value) > 128 || value[0] < 'A' || value[0] > 'Z' {
		return false
	}
	for index := 1; index < len(value); index++ {
		character := value[index]
		if character != '_' && (character < 'A' || character > 'Z') && (character < '0' || character > '9') {
			return false
		}
	}
	return true
}

func validPage(cursor string, limit, maximum int) bool {
	if cursor != "" && !validOpaque(cursor, 16384) || limit < 0 || limit > maximum {
		return false
	}
	return limit == 0 || limit >= 50
}

func validStatus(status EntityStatus) bool { return status == StatusActive || status == StatusPaused }

func validCallbackURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && parsed.Fragment == ""
}

func validFields(fields []string) bool {
	if len(fields) == 0 || len(fields) > 100 {
		return false
	}
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		if !validMetricIdentifier(field) {
			return false
		}
		if _, found := seen[field]; found {
			return false
		}
		seen[field] = struct{}{}
	}
	return true
}

func validMetricIdentifier(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for index := range value {
		character := value[index]
		if character != '_' && (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') && (character < '0' || character > '9') {
			return false
		}
	}
	return true
}

func validGranularity(value Granularity) bool {
	return value == GranularityTotal || value == GranularityDay || value == GranularityHour || value == GranularityLifetime
}

func hourAligned(value time.Time) bool {
	return !value.IsZero() && value.Equal(value.Truncate(time.Hour))
}

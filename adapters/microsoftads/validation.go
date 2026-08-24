package microsoftads

import (
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"
)

func validNumericID(value string) bool {
	if value == "" || len(value) > 20 {
		return false
	}
	nonZero := false
	for index := range value {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
		nonZero = nonZero || value[index] != '0'
	}
	return nonZero
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

func validRequiredText(value string, maximum int) bool {
	return strings.TrimSpace(value) != "" && strings.TrimSpace(value) == value && len(value) <= maximum &&
		utf8.ValidString(value) && !strings.ContainsRune(value, '\x00')
}

func validOptionalText(value string, maximum int) bool {
	return value == "" || validRequiredText(value, maximum)
}

func validStatus(value Status) bool { return value == StatusActive || value == StatusPaused }

func validMatchType(value MatchType) bool {
	return value == MatchTypeBroad || value == MatchTypeExact || value == MatchTypePhrase
}

func validateFinalURLs(values []string) bool {
	if len(values) == 0 || len(values) > 10 {
		return false
	}
	for _, value := range values {
		parsed, err := url.Parse(value)
		if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" ||
			parsed.User != nil || parsed.Fragment != "" || len(value) > 2048 {
			return false
		}
	}
	return true
}

func validateTextAssets(values []AdTextAsset, minimum, maximum int) bool {
	if len(values) < minimum || len(values) > maximum {
		return false
	}
	for _, value := range values {
		if !validRequiredText(value.Text, 1000) || strings.ContainsRune(value.Text, '\n') ||
			!validOptionalText(value.PinnedField, 64) {
			return false
		}
	}
	return true
}

func validCallbackURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme != "" && parsed.Host != "" && parsed.User == nil && parsed.Fragment == ""
}

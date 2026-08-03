package googlebusinessprofile

import (
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"
)

func validResourceSegment(value string) bool {
	return validOpaque(value, 1024) && !strings.Contains(value, "/")
}

func validOpaque(value string, maximum int) bool {
	if value == "" || len(value) > maximum || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return strings.TrimSpace(value) == value
}

func validOptionalText(value string, maximumBytes int) bool {
	return len(value) <= maximumBytes && utf8.ValidString(value) && !strings.ContainsRune(value, '\x00')
}

func validRequiredText(value string, maximumBytes int) bool {
	return strings.TrimSpace(value) != "" && validOptionalText(value, maximumBytes)
}

func validLanguageCode(value string) bool {
	if !validOpaque(value, 64) {
		return false
	}
	for _, character := range value {
		if character != '-' && character != '_' && !unicode.IsLetter(character) && !unicode.IsDigit(character) {
			return false
		}
	}
	return true
}

func validCallbackURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && parsed.Fragment == ""
}

func validPublicURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil
}

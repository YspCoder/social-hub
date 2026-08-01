package imgur

import (
	"mime"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxOpaqueLength = 4096

func validOpaque(value string, maximum int) bool {
	return value != "" && len(value) <= maximum && strings.TrimSpace(value) == value &&
		utf8.ValidString(value) && !strings.ContainsFunc(value, unicode.IsControl)
}

func validIdentifier(value string) bool {
	return validOpaque(value, 255) && !strings.ContainsAny(value, "/?#")
}

func validText(value string, required bool) bool {
	if !utf8.ValidString(value) || len(value) > 1<<20 || strings.ContainsFunc(value, unsafeControl) {
		return false
	}
	return !required || strings.TrimSpace(value) != ""
}

func unsafeControl(character rune) bool {
	return unicode.IsControl(character) && character != '\n' && character != '\r' && character != '\t'
}

func validFilename(value string) bool {
	return validOpaque(value, 1024) && !strings.ContainsAny(value, `/\`) && value != "." && value != ".."
}

func validUploadMIME(value string) bool {
	mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(value))
	return err == nil && (strings.HasPrefix(mediaType, "image/") || strings.HasPrefix(mediaType, "video/")) && !strings.Contains(mediaType, "*")
}

func validHTTPURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil
}

func parsePage(value string) (int, error) {
	if value == "" {
		return 0, nil
	}
	page, err := strconv.Atoi(value)
	if err != nil || page < 0 || page > 1_000_000 {
		return 0, invalidArgument("pagination", "cursor must be a non-negative Imgur page number")
	}
	return page, nil
}

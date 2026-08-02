package mixcloud

import (
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxUsernameLength = 100
	maxSlugLength     = 256
	maxOpaqueLength   = 8192
	maxCursorOffset   = 1_000_000_000
)

func validEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == ""
}

func validUserAgent(value string) bool {
	return strings.TrimSpace(value) == value && value != "" && len(value) <= 512 && !strings.ContainsFunc(value, unicode.IsControl)
}

func validOpaque(value string, maximum int) bool {
	return strings.TrimSpace(value) == value && value != "" && len(value) <= maximum && !strings.ContainsFunc(value, unicode.IsControl)
}

func validSegment(value string, maximum int) bool {
	if value == "" || len(value) > maximum || strings.TrimSpace(value) != value || value == "." || value == ".." {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || character == '-' || character == '_' {
			continue
		}
		return false
	}
	return true
}

func parseUserKey(value string) (username, key string, ok bool) {
	value = strings.Trim(value, "/")
	if !validSegment(value, maxUsernameLength) {
		return "", "", false
	}
	return value, "/" + value + "/", true
}

func parseCloudcastKey(value string) (username, slug, key string, ok bool) {
	trimmed := strings.Trim(value, "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 2 || !validSegment(parts[0], maxUsernameLength) || !validSlug(parts[1]) {
		return "", "", "", false
	}
	return parts[0], parts[1], "/" + parts[0] + "/" + parts[1] + "/", true
}

func validSlug(value string) bool {
	if !utf8.ValidString(value) || value == "" || utf8.RuneCountInString(value) > maxSlugLength || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if unicode.IsLetter(character) || unicode.IsDigit(character) || character == '-' || character == '_' {
			continue
		}
		return false
	}
	return true
}

func validCommentKey(value string) bool {
	if len(value) > 1024 || !strings.HasPrefix(value, "/comments/") || !strings.HasSuffix(value, "/") {
		return false
	}
	parts := strings.Split(strings.Trim(value, "/"), "/")
	if len(parts) < 3 || parts[0] != "comments" {
		return false
	}
	for _, part := range parts[1:] {
		if !validSegment(part, maxSlugLength) {
			return false
		}
	}
	return true
}

func validText(value string, allowEmpty bool, maximum int) bool {
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > maximum || strings.ContainsRune(value, '\x00') {
		return false
	}
	return allowEmpty || strings.TrimSpace(value) != ""
}

func validFilename(value string) bool {
	if !validText(value, false, 255) || filepath.Base(value) != value {
		return false
	}
	return value != "." && value != ".." && !strings.ContainsAny(value, `/\\`)
}

func validPictureMIME(value string) bool {
	return strings.HasPrefix(value, "image/") && len(value) <= 128 && !strings.ContainsFunc(value, unicode.IsControl)
}

func parseOffset(value string) (int, bool) {
	if value == "" {
		return 0, true
	}
	offset, err := strconv.Atoi(value)
	return offset, err == nil && offset >= 0 && offset <= maxCursorOffset
}

func validOAuthRedirect(value string) bool {
	if value == "" {
		return true
	}
	if !validText(value, false, 2048) {
		return false
	}
	if !strings.Contains(value, "://") {
		return !strings.ContainsAny(value, "/?#@") && strings.Contains(value, ".")
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme != "" && parsed.Host != "" && parsed.User == nil && parsed.Fragment == ""
}

package discourse

import (
	"net/url"
	"strconv"
	"strings"
)

func validID(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	return err == nil && parsed > 0
}

func mustID(value string) int64 {
	parsed, _ := strconv.ParseInt(value, 10, 64)
	return parsed
}

func validCursor(value string) bool {
	return value == "" || validID(value)
}

func path(parts ...string) string {
	value := ""
	for _, part := range parts {
		value += "/" + url.PathEscape(part)
	}
	return value + ".json"
}

func validText(value string, maximum int) bool {
	return strings.TrimSpace(value) != "" && len(value) <= maximum && !strings.ContainsRune(value, '\x00')
}

package patreon

import (
	"net/url"
	"strconv"
	"strings"
)

func validResourceID(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 256 || strings.ContainsAny(value, "/\\?#") {
		return false
	}
	for _, character := range value {
		if character < 0x21 || character == 0x7f {
			return false
		}
	}
	return true
}

func pageQuery(maximum int, cursor string) (url.Values, error) {
	if maximum < 0 || maximum > 1000 {
		return nil, invalidArgument("pagination", "max_results must be between 0 and 1000")
	}
	if cursor != "" && !validCursor(cursor) {
		return nil, invalidArgument("pagination", "cursor is invalid")
	}
	if maximum == 0 {
		maximum = 20
	}
	query := url.Values{"page[count]": {strconv.Itoa(maximum)}}
	if cursor != "" {
		query.Set("page[cursor]", cursor)
	}
	return query, nil
}

func validCursor(value string) bool {
	if value == "" || len(value) > 4096 {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}

func resourcePath(kind, id string) string {
	return "/" + kind + "/" + url.PathEscape(id)
}

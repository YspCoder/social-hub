package lemmy

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

const userAgent = "social-hub/lemmy"

func (client *Client) requestJSON(ctx context.Context, method, path string, query url.Values, input, output any, options ...socialhub.CallOption) error {
	var body *bytes.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return platformError(method+" "+path, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		body = bytes.NewReader(encoded)
	} else {
		body = bytes.NewReader(nil)
	}
	request, err := client.api.NewRequest(ctx, method, "/api/v3/"+strings.TrimLeft(path, "/"), query, body, options...)
	if err != nil {
		return err
	}
	request.Header.Set("User-Agent", userAgent)
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	return client.api.Do(request, output)
}

func pageQuery(cursor string, maximum int) (url.Values, int, int, error) {
	if maximum < 0 || maximum > 50 {
		return nil, 0, 0, invalidArgument("pagination", "max_results must be between 0 and 50")
	}
	page := 1
	if cursor != "" {
		if !validID(cursor) {
			return nil, 0, 0, invalidArgument("pagination", "cursor must be a positive decimal page number")
		}
		parsed, _ := strconv.Atoi(cursor)
		page = parsed
	}
	if maximum == 0 {
		maximum = 20
	}
	return url.Values{"page": {strconv.Itoa(page)}, "limit": {strconv.Itoa(maximum)}}, page, maximum, nil
}

func pageCursors(itemCount, page, pageSize int) (*string, *string, bool) {
	var next, previous *string
	if itemCount == pageSize {
		value := strconv.Itoa(page + 1)
		next = &value
	}
	if page > 1 {
		value := strconv.Itoa(page - 1)
		previous = &value
	}
	return next, previous, next != nil
}

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

func validUsername(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 512 || strings.ContainsAny(value, "/\\?#\x00\r\n") {
		return false
	}
	for _, character := range value {
		if character < 0x21 || character == 0x7f {
			return false
		}
	}
	return true
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

func validTitle(value string) bool {
	trimmed := strings.TrimSpace(value)
	length := utf8.RuneCountInString(trimmed)
	return length >= 3 && length <= 200 && !strings.ContainsAny(value, "\x00\r\n")
}

func validBody(value string, maximum int) bool {
	return !strings.ContainsRune(value, '\x00') && len(utf16.Encode([]rune(value))) <= maximum
}

func validPostURL(value string) bool {
	if value == "" {
		return true
	}
	if len(utf16.Encode([]rune(value))) > 2000 || strings.ContainsAny(value, "\x00\r\n") {
		return false
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.User != nil {
		return false
	}
	if parsed.Scheme == "magnet" {
		return parsed.RawQuery != ""
	}
	return (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != ""
}

func validHTTPURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil
}

func int64Pointer(value int64) *int64 {
	copy := value
	return &copy
}

func stringPointer(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	copy := value
	return &copy
}

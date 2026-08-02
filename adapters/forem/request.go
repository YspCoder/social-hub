package forem

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

const (
	foremAccept = "application/vnd.forem.api-v1+json"
	userAgent   = "social-hub/forem"
)

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
	request, err := client.api.NewRequest(ctx, method, path, query, body, options...)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", foremAccept)
	request.Header.Set("User-Agent", userAgent)
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	return client.api.Do(request, output)
}

func pageQuery(cursor string, maximum int) (url.Values, int, int, error) {
	if maximum < 0 || maximum > 1000 {
		return nil, 0, 0, invalidArgument("pagination", "max_results must be between 0 and 1000")
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
		maximum = 30
	}
	return url.Values{"page": {strconv.Itoa(page)}, "per_page": {strconv.Itoa(maximum)}}, page, maximum, nil
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

func validIdentifier(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 512 || strings.ContainsAny(value, "/\\?#\x00\r\n") {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}

func validText(value string, maximum int) bool {
	return strings.TrimSpace(value) != "" && len(value) <= maximum && !strings.ContainsRune(value, '\x00')
}

func resourcePath(kind, id string) string {
	return "/api/" + kind + "/" + url.PathEscape(id)
}

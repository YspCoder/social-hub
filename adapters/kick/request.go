package kick

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type responseEnvelope[T any] struct {
	Data    T      `json:"data"`
	Message string `json:"message"`
}

type paginatedEnvelope[T any] struct {
	Data       []T    `json:"data"`
	Message    string `json:"message"`
	Pagination struct {
		NextCursor string `json:"next_cursor"`
	} `json:"pagination"`
}

func (client *Client) request(ctx context.Context, method, path string, query url.Values, input, output any, options ...socialhub.CallOption) error {
	return client.api.JSON(ctx, method, path, query, input, output, options...)
}

func addPositiveIDs(query url.Values, key string, values []string, maximum int) error {
	if maximum > 0 && len(values) > maximum {
		return invalidArgument("request", key+" exceeds the platform maximum")
	}
	for _, value := range values {
		if !validPositiveID(value) {
			return invalidArgument("request", key+" values must be positive decimal integers")
		}
		query.Add(key, value)
	}
	return nil
}

func validOpaque(value string, maximum int) bool {
	if strings.TrimSpace(value) != value || value == "" || len(value) > maximum {
		return false
	}
	for _, character := range value {
		if character < 0x21 || character == 0x7f {
			return false
		}
	}
	return true
}

func validPathID(value string, maximum int) bool {
	return validOpaque(value, maximum) && !strings.ContainsAny(value, "/\\?#")
}

func validFilterValue(value string, maximum int, allowComma bool) bool {
	if strings.TrimSpace(value) != value || value == "" || utf8.RuneCountInString(value) > maximum {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f || (!allowComma && character == ',') {
			return false
		}
	}
	return true
}

func positiveInt64(value string) int64 {
	parsed, _ := strconv.ParseInt(value, 10, 64)
	return parsed
}

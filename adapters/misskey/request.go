package misskey

import (
	"context"
	"net/http"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

func (c *Client) post(ctx context.Context, endpoint string, input, output any, options ...socialhub.CallOption) error {
	return c.api.JSON(ctx, http.MethodPost, "/"+strings.TrimLeft(endpoint, "/"), nil, input, output, options...)
}

type paginationRequest struct {
	Limit     int    `json:"limit"`
	UntilID   string `json:"untilId,omitempty"`
	SinceDate *int64 `json:"sinceDate,omitempty"`
	UntilDate *int64 `json:"untilDate,omitempty"`
}

func makePagination(operation, cursor string, maximum int, start, end *time.Time) (paginationRequest, error) {
	if maximum < 0 {
		return paginationRequest{}, invalidArgument(operation, "max results must not be negative")
	}
	if cursor != "" && !validID(cursor) {
		return paginationRequest{}, invalidArgument(operation, "cursor is invalid")
	}
	if cursor != "" && end != nil {
		return paginationRequest{}, invalidArgument(operation, "cursor and end time cannot be combined")
	}
	if start != nil && end != nil && start.After(*end) {
		return paginationRequest{}, invalidArgument(operation, "start time must not be after end time")
	}
	if maximum == 0 {
		maximum = 10
	} else if maximum > 100 {
		maximum = 100
	}
	request := paginationRequest{Limit: maximum, UntilID: cursor}
	if start != nil {
		value := start.UnixMilli()
		request.SinceDate = &value
	}
	if end != nil {
		value := end.UnixMilli()
		request.UntilDate = &value
	}
	return request, nil
}

func (c *Client) mapNotePage(input []misskeyNote, maximum int) (socialhub.Page[socialhub.Post], error) {
	items := make([]socialhub.Post, 0, len(input))
	for _, note := range input {
		post, err := c.mapNote(note)
		if err != nil {
			return socialhub.Page[socialhub.Post]{}, err
		}
		items = append(items, *post)
	}
	page := socialhub.Page[socialhub.Post]{Items: items, HasMore: len(input) == maximum}
	if page.HasMore && len(input) > 0 {
		cursor := input[len(input)-1].ID
		page.NextCursor = &cursor
	}
	return page, nil
}

func validateIDs(operation string, values []string, maximum int) error {
	if maximum > 0 && len(values) > maximum {
		return invalidArgument(operation, "too many IDs")
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validID(value) {
			return invalidArgument(operation, "IDs must not be empty or invalid")
		}
		if _, exists := seen[value]; exists {
			return invalidArgument(operation, "IDs must be unique")
		}
		seen[value] = struct{}{}
	}
	return nil
}

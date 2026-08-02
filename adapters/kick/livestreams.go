package kick

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListLivestreams(ctx context.Context, input LivestreamListRequest, options ...socialhub.CallOption) (socialhub.Page[Livestream], error) {
	if input.Limit < 0 || input.Limit > 1000 {
		return socialhub.Page[Livestream]{}, invalidArgument("list_livestreams", "limit must be between 0 and 1000")
	}
	if len(input.CategoryIDs) > 25 || len(input.LanguageCodes) > 25 {
		return socialhub.Page[Livestream]{}, invalidArgument("list_livestreams", "category and language filters are limited to 25 values each")
	}
	if input.Cursor != "" && !validOpaque(input.Cursor, 4096) {
		return socialhub.Page[Livestream]{}, invalidArgument("list_livestreams", "cursor is invalid")
	}
	query := make(url.Values)
	if err := addPositiveIDs(query, "category_id", input.CategoryIDs, 25); err != nil {
		return socialhub.Page[Livestream]{}, err
	}
	for _, languageCode := range input.LanguageCodes {
		if !validFilterValue(languageCode, 64, false) {
			return socialhub.Page[Livestream]{}, invalidArgument("list_livestreams", "language codes must be bounded BCP 47 values")
		}
		query.Add("language_code", languageCode)
	}
	if input.Limit > 0 {
		query.Set("limit", strconv.Itoa(input.Limit))
	}
	if input.Cursor != "" {
		query.Set("cursor", input.Cursor)
	}
	var response paginatedEnvelope[Livestream]
	if err := client.request(ctx, http.MethodGet, "/public/v2/livestreams", query, nil, &response, options...); err != nil {
		return socialhub.Page[Livestream]{}, err
	}
	return cursorPage(response.Data, response.Pagination.NextCursor), nil
}

func (client *Client) ListUserLivestreams(ctx context.Context, userIDs []string, options ...socialhub.CallOption) ([]Livestream, error) {
	if len(userIDs) == 0 {
		return nil, invalidArgument("list_user_livestreams", "at least one user ID is required")
	}
	query := make(url.Values)
	if err := addPositiveIDs(query, "user_id", userIDs, 100); err != nil {
		return nil, err
	}
	var response responseEnvelope[[]Livestream]
	if err := client.request(ctx, http.MethodGet, "/public/v1/users/livestreams", query, nil, &response, options...); err != nil {
		return nil, err
	}
	return response.Data, nil
}

func (client *Client) ListCategories(ctx context.Context, input CategoryListRequest, options ...socialhub.CallOption) (socialhub.Page[Category], error) {
	if input.Limit < 0 || input.Limit > 1000 {
		return socialhub.Page[Category]{}, invalidArgument("list_categories", "limit must be between 0 and 1000")
	}
	if input.Cursor != "" && (len(input.Cursor) < 4 || len(input.Cursor) > 28 || !validOpaque(input.Cursor, 28)) {
		return socialhub.Page[Category]{}, invalidArgument("list_categories", "cursor must be 4-28 safe characters")
	}
	query := make(url.Values)
	if len(input.Names) != 0 {
		for _, name := range input.Names {
			if !validFilterValue(name, 100, false) {
				return socialhub.Page[Category]{}, invalidArgument("list_categories", "category names must be bounded and cannot contain commas")
			}
		}
		query.Set("name", strings.Join(input.Names, ","))
	}
	if len(input.Tags) != 0 {
		for _, tag := range input.Tags {
			if !validFilterValue(tag, 100, false) {
				return socialhub.Page[Category]{}, invalidArgument("list_categories", "category tags must be bounded and cannot contain commas")
			}
		}
		query.Set("tag", strings.Join(input.Tags, ","))
	}
	if len(input.IDs) != 0 {
		for _, id := range input.IDs {
			if !validPositiveID(id) {
				return socialhub.Page[Category]{}, invalidArgument("list_categories", "category IDs must be positive decimal integers")
			}
		}
		query.Set("id", strings.Join(input.IDs, ","))
	}
	if input.Limit > 0 {
		query.Set("limit", strconv.Itoa(input.Limit))
	}
	if input.Cursor != "" {
		query.Set("cursor", input.Cursor)
	}
	var response paginatedEnvelope[Category]
	if err := client.request(ctx, http.MethodGet, "/public/v2/categories", query, nil, &response, options...); err != nil {
		return socialhub.Page[Category]{}, err
	}
	return cursorPage(response.Data, response.Pagination.NextCursor), nil
}

func cursorPage[T any](items []T, cursor string) socialhub.Page[T] {
	var next *string
	if cursor != "" {
		copy := cursor
		next = &copy
	}
	return socialhub.Page[T]{Items: items, NextCursor: next, HasMore: next != nil}
}

var _ LivestreamWorkflow = (*Client)(nil)
var _ CategoryWorkflow = (*Client)(nil)

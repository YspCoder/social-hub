package musicbrainz

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

func (c *Client) requestJSON(ctx context.Context, operation, path string, query url.Values, output any, options ...socialhub.CallOption) error {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return err
	}
	if len(resolved.Fields) > 0 {
		return invalidArgument(operation, "field selection is fixed by the typed MusicBrainz operation")
	}
	if resolved.IdempotencyKey != "" {
		return invalidArgument(operation, "MusicBrainz reads do not use idempotency keys")
	}
	if err := c.gate.Wait(ctx); err != nil {
		return err
	}
	request, err := c.api.NewRequest(ctx, http.MethodGet, path, query, nil, options...)
	if err != nil {
		return err
	}
	_, err = c.api.DoWithMetadata(request, output)
	if platformErr, ok := err.(*socialhub.Error); ok {
		platformErr.Op = operation
	}
	return err
}

func pageQuery(query string, limit int, cursor string) (url.Values, error) {
	if !validQuery(query) {
		return nil, invalidArgument("search", "query must be a nonempty bounded value without surrounding whitespace")
	}
	offset, err := validatePage(cursor, limit)
	if err != nil {
		return nil, err
	}
	values := url.Values{"query": {query}}
	setPage(values, limit, cursor, offset)
	return values, nil
}

func browseQuery(mbid string, limit int, cursor string) (url.Values, error) {
	if !validMBID(mbid) {
		return nil, invalidArgument("browse", "artist_mbid must be a canonical lowercase UUID")
	}
	offset, err := validatePage(cursor, limit)
	if err != nil {
		return nil, err
	}
	values := url.Values{"artist": {mbid}}
	setPage(values, limit, cursor, offset)
	return values, nil
}

func setPage(query url.Values, limit int, cursor string, offset int) {
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}
	if cursor != "" {
		query.Set("offset", strconv.Itoa(offset))
	}
}

func pageFromEnvelope[T any](operation string, items []T, count, offset, requestedLimit int) (socialhub.Page[T], error) {
	if count < 0 || offset < 0 || offset > count || len(items) > maxPageSize || offset+len(items) > count ||
		(len(items) == 0 && offset < count) {
		return socialhub.Page[T]{}, invalidPlatformResponse(operation, "response contained invalid pagination metadata")
	}
	result := socialhub.Page[T]{Items: items}
	nextOffset := offset + len(items)
	result.HasMore = nextOffset < count
	if result.HasMore {
		next := strconv.Itoa(nextOffset)
		result.NextCursor = &next
	}
	if offset > 0 {
		limit := requestedLimit
		if limit == 0 {
			limit = defaultPageSize
		}
		previousOffset := offset - limit
		if previousOffset < 0 {
			previousOffset = 0
		}
		previous := strconv.Itoa(previousOffset)
		result.PrevCursor = &previous
	}
	return result, nil
}

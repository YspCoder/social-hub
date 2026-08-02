package peertube

import (
	"net/url"
	"strconv"
)

func pageQuery(operation, cursor string, maximum int) (url.Values, int64, int, error) {
	if maximum < 0 {
		return nil, 0, 0, invalidArgument(operation, "max results must not be negative")
	}
	var start int64
	if cursor != "" {
		parsed, err := strconv.ParseInt(cursor, 10, 64)
		if err != nil || parsed < 0 {
			return nil, 0, 0, invalidArgument(operation, "cursor must be a non-negative decimal offset")
		}
		start = parsed
	}
	limit := maximum
	if limit > 100 {
		limit = 100
	}
	query := url.Values{}
	if start > 0 {
		query.Set("start", strconv.FormatInt(start, 10))
	}
	if limit > 0 {
		query.Set("count", strconv.Itoa(limit))
	}
	return query, start, limit, nil
}

func pageCursors(total, start int64, requested, itemCount int) (next, previous *string, hasMore bool, err error) {
	if total < 0 || itemCount < 0 || start > total && itemCount > 0 {
		return nil, nil, false, platformError("pagination", "platform_error", "permanent", nil)
	}
	consumed := int64(itemCount)
	if consumed > 0 && start+consumed < total {
		value := strconv.FormatInt(start+consumed, 10)
		next, hasMore = &value, true
	}
	if start > 0 {
		step := int64(requested)
		if step <= 0 {
			step = consumed
		}
		if step <= 0 || step > start {
			step = start
		}
		value := strconv.FormatInt(start-step, 10)
		previous = &value
	}
	return next, previous, hasMore, nil
}

package lastfm

import (
	"strconv"

	"social-hub/pkg/socialhub"
)

func makePage[T any](items []T, page, totalPages int) socialhub.Page[T] {
	if page < 1 {
		page = 1
	}
	result := socialhub.Page[T]{Items: items, HasMore: totalPages > 0 && page < totalPages}
	if result.HasMore {
		next := strconv.Itoa(page + 1)
		result.NextCursor = &next
	}
	if page > 1 {
		previous := strconv.Itoa(page - 1)
		result.PrevCursor = &previous
	}
	return result
}

func searchTotalPages(total, perPage int64) int {
	if total <= 0 || perPage <= 0 {
		return 0
	}
	return int((total + perPage - 1) / perPage)
}

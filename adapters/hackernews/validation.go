package hackernews

import (
	"strconv"
	"strings"
	"unicode"
)

const (
	defaultPageSize    = 20
	maxPageSize        = 100
	maxFeedIDs         = 500
	maxUpdateItems     = 10000
	maxSubmittedIDs    = 250000
	maxUserScanPerPage = 500
)

func validFeed(feed Feed) bool {
	switch feed {
	case FeedTop, FeedNew, FeedBest, FeedAsk, FeedShow, FeedJob:
		return true
	default:
		return false
	}
}

func validItemType(itemType ItemType) bool {
	switch itemType {
	case "", ItemJob, ItemStory, ItemComment, ItemPoll, ItemPollOpt:
		return true
	default:
		return false
	}
}

func postItem(itemType ItemType) bool {
	return itemType == ItemStory || itemType == ItemJob || itemType == ItemPoll
}

func pageSize(value int) (int, error) {
	if value == 0 {
		return defaultPageSize, nil
	}
	if value < 1 || value > maxPageSize {
		return 0, invalidArgument("pagination", "max results must be between 1 and 100")
	}
	return value, nil
}

func pageOffset(cursor string, total int) (int, error) {
	if cursor == "" {
		return 0, nil
	}
	offset, err := strconv.Atoi(cursor)
	if err != nil || offset < 0 || offset > total || strconv.Itoa(offset) != cursor {
		return 0, invalidArgument("pagination", "cursor must be a canonical in-range decimal offset")
	}
	return offset, nil
}

func nextPageCursor(offset, consumed, total int) (*string, bool) {
	next := offset + consumed
	if next >= total {
		return nil, false
	}
	cursor := strconv.Itoa(next)
	return &cursor, true
}

func parseItemID(value, operation string) (int64, error) {
	if value == "" {
		return 0, invalidArgument(operation, "item ID is required")
	}
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 || strconv.FormatInt(id, 10) != value {
		return 0, invalidArgument(operation, "item ID must be a positive canonical decimal")
	}
	return id, nil
}

func validUsername(value string) bool {
	if value == "" || len(value) > 64 || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) || strings.ContainsRune("/\\?#", character) {
			return false
		}
	}
	return true
}

func validIDs(values []int64, maximum int) bool {
	if len(values) > maximum {
		return false
	}
	for _, value := range values {
		if value <= 0 {
			return false
		}
	}
	return true
}

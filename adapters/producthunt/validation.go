package producthunt

import (
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

func validOpaque(value string, maximum int) bool {
	if value == "" || value != strings.TrimSpace(value) || len(value) > maximum || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validAccessToken(value string) bool {
	if !validOpaque(value, 16_384) {
		return false
	}
	for _, character := range value {
		if unicode.IsSpace(character) {
			return false
		}
	}
	return true
}

func validOptionalOpaque(value string, maximum int) bool {
	return value == "" || validOpaque(value, maximum)
}

func validSlug(value string) bool {
	return validOpaque(value, 256) && value != "." && value != ".." && !strings.ContainsAny(value, "/\\?#%")
}

func validWebURL(value string) bool {
	if !validOpaque(value, 4096) {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil
}

func validScopes(values []string) bool {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		switch value {
		case "public", "private", "write":
		default:
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func containsScope(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func validObjectLookup(value ObjectLookup) bool {
	return (value.ID == "") != (value.Slug == "") &&
		(value.ID == "" || validOpaque(value.ID, 256)) && (value.Slug == "" || validSlug(value.Slug))
}

func validUserLookup(value UserLookup) bool {
	return (value.ID == "") != (value.Username == "") &&
		(value.ID == "" || validOpaque(value.ID, 256)) && (value.Username == "" || validSlug(value.Username))
}

func validPagination(value Pagination) bool {
	if value.First < 0 || value.Last < 0 || value.First > 0 && value.Last > 0 {
		return false
	}
	if !validOptionalOpaque(value.After, 2048) || !validOptionalOpaque(value.Before, 2048) {
		return false
	}
	if value.After != "" && value.First == 0 || value.Before != "" && value.Last == 0 {
		return false
	}
	if value.First > 0 && value.Before != "" || value.Last > 0 && value.After != "" {
		return false
	}
	return value.After == "" || value.Before == ""
}

func validPostsOrder(value PostsOrder) bool {
	switch value {
	case "", PostsOrderFeaturedAt, PostsOrderNewest, PostsOrderRanking, PostsOrderVotes:
		return true
	default:
		return false
	}
}

func validTopicsOrder(value TopicsOrder) bool {
	return value == "" || value == TopicsOrderFollowersCount || value == TopicsOrderNewest
}

func validCollectionsOrder(value CollectionsOrder) bool {
	switch value {
	case "", CollectionsOrderFeaturedAt, CollectionsOrderFollowersCount, CollectionsOrderNewest:
		return true
	default:
		return false
	}
}

func validCommentsOrder(value CommentsOrder) bool {
	return value == "" || value == CommentsOrderNewest || value == CommentsOrderVotesCount
}

func validConnection[T any](value *Connection[T], id func(T) string) bool {
	if value == nil || value.Edges == nil || value.TotalCount < len(value.Edges) ||
		!validOptionalCursor(value.PageInfo.StartCursor) || !validOptionalCursor(value.PageInfo.EndCursor) {
		return false
	}
	cursors := make(map[string]struct{}, len(value.Edges))
	ids := make(map[string]struct{}, len(value.Edges))
	for _, edge := range value.Edges {
		objectID := id(edge.Node)
		if !validOpaque(edge.Cursor, 2048) || !validOpaque(objectID, 256) {
			return false
		}
		if _, exists := cursors[edge.Cursor]; exists {
			return false
		}
		if _, exists := ids[objectID]; exists {
			return false
		}
		cursors[edge.Cursor] = struct{}{}
		ids[objectID] = struct{}{}
	}
	return true
}

func validOptionalCursor(value *string) bool {
	return value == nil || validOpaque(*value, 2048)
}

func prepareCallOptions(operation string, options []socialhub.CallOption) error {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" {
		return invalidArgument(operation, "Product Hunt does not document a caller request-ID header")
	}
	if resolved.IdempotencyKey != "" {
		return invalidArgument(operation, "read-only GraphQL operations do not use idempotency keys")
	}
	if len(resolved.Fields) > 0 {
		return invalidArgument(operation, "field selection is fixed by the typed GraphQL operation")
	}
	return nil
}

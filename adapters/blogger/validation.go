package blogger

import (
	"errors"
	"net/url"
	"path"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

const (
	blogKind        = "blogger#blog"
	blogListKind    = "blogger#blogList"
	postKind        = "blogger#post"
	postListKind    = "blogger#postList"
	commentKind     = "blogger#comment"
	commentListKind = "blogger#commentList"
	pageKind        = "blogger#page"
	pageListKind    = "blogger#pageList"
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

func validOptionalOpaque(value string, maximum int) bool {
	return value == "" || validOpaque(value, maximum)
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

func validStringSet(values []string, maximum int) bool {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validOpaque(value, maximum) {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validResourceID(value string) bool {
	return validOpaque(value, 1024) && value != "." && value != ".." && !strings.ContainsAny(value, "/\\?#%")
}

func validBlogURL(value string) bool {
	if !validOpaque(value, 4096) {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && parsed.Fragment == ""
}

func validPostPath(value string) bool {
	if !validOpaque(value, 4096) || !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") ||
		strings.Contains(value, "\\") {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "" && parsed.Host == "" && parsed.User == nil &&
		parsed.RawQuery == "" && parsed.Fragment == "" && parsed.Path != "" && path.Clean(parsed.Path) == parsed.Path
}

func validPageToken(value string) bool {
	if !validOptionalOpaque(value, 8192) {
		return false
	}
	for _, character := range value {
		if unicode.IsSpace(character) {
			return false
		}
	}
	return true
}

func validView(value ViewType) bool {
	switch value {
	case "", ViewUnspecified, ViewReader, ViewAuthor, ViewAdmin:
		return true
	default:
		return false
	}
}

func validBlogStatus(value BlogStatus) bool {
	return value == "" || value == BlogStatusLive || value == BlogStatusDeleted
}

func validPostStatus(value PostStatus) bool {
	switch value {
	case "", PostStatusLive, PostStatusDraft, PostStatusScheduled, PostStatusSoftTrashed:
		return true
	default:
		return false
	}
}

func validCommentStatus(value CommentStatus) bool {
	switch value {
	case "", CommentStatusLive, CommentStatusEmptied, CommentStatusPending, CommentStatusSpam:
		return true
	default:
		return false
	}
}

func validPageStatus(value PageStatus) bool {
	switch value {
	case "", PageStatusLive, PageStatusDraft, PageStatusSoftTrashed:
		return true
	default:
		return false
	}
}

func validPostOrder(value PostOrder) bool {
	switch value {
	case "", PostOrderUnspecified, PostOrderPublished, PostOrderUpdated:
		return true
	default:
		return false
	}
}

func validSortOption(value SortOption) bool {
	switch value {
	case "", SortUnspecified, SortDescending, SortAscending:
		return true
	default:
		return false
	}
}

func parseOptionalRFC3339(value string) (time.Time, bool) {
	if value == "" {
		return time.Time{}, true
	}
	if !validOpaque(value, 128) {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339, value)
	return parsed, err == nil
}

func validDateRange(start, end string) bool {
	startTime, validStart := parseOptionalRFC3339(start)
	endTime, validEnd := parseOptionalRFC3339(end)
	if !validStart || !validEnd {
		return false
	}
	return start == "" || end == "" || !endTime.Before(startTime)
}

func validLabels(values []string) bool {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validOpaque(value, 256) || strings.Contains(value, ",") {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func prepareCallOptions(operation string, options []socialhub.CallOption) error {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" {
		return invalidArgument(operation, "Blogger API v3 does not document a caller request-ID header")
	}
	if resolved.IdempotencyKey != "" {
		return invalidArgument(operation, "read-only Blogger operations do not use idempotency keys")
	}
	if len(resolved.Fields) > 0 {
		return invalidArgument(operation, "partial response fields are fixed by the typed Blogger operation")
	}
	return nil
}

func validBlogResponse(value Blog, expectedID string) bool {
	if value.Kind != blogKind || !validResourceID(value.ID) || len(value.Raw) == 0 || expectedID != "" && value.ID != expectedID {
		return false
	}
	for _, post := range value.Posts.Items {
		if !validPostResponse(post, value.ID, "") {
			return false
		}
	}
	return true
}

func validBlogListResponse(value BlogList) bool {
	if value.Kind != blogListKind || len(value.Raw) == 0 {
		return false
	}
	for _, blog := range value.Items {
		if !validBlogResponse(blog, "") {
			return false
		}
	}
	return true
}

func validPostResponse(value Post, expectedBlogID, expectedPostID string) bool {
	if value.Kind != postKind || !validResourceID(value.ID) || !validResourceID(value.Blog.ID) || len(value.Raw) == 0 ||
		(expectedBlogID != "" && value.Blog.ID != expectedBlogID) || (expectedPostID != "" && value.ID != expectedPostID) {
		return false
	}
	for _, comment := range value.Replies.Items {
		if !validCommentResponse(comment, value.Blog.ID, value.ID, "") {
			return false
		}
	}
	return true
}

func validPostListResponse(value PostList, expectedBlogID string) bool {
	if value.Kind != postListKind || len(value.Raw) == 0 || !validPageToken(value.NextPageToken) || !validPageToken(value.PrevPageToken) {
		return false
	}
	for _, post := range value.Items {
		if !validPostResponse(post, expectedBlogID, "") {
			return false
		}
	}
	return true
}

func validCommentResponse(value Comment, expectedBlogID, expectedPostID, expectedCommentID string) bool {
	if value.Kind != commentKind || !validResourceID(value.ID) || !validResourceID(value.Blog.ID) ||
		!validResourceID(value.Post.ID) || len(value.Raw) == 0 ||
		(expectedBlogID != "" && value.Blog.ID != expectedBlogID) ||
		(expectedPostID != "" && value.Post.ID != expectedPostID) ||
		(expectedCommentID != "" && value.ID != expectedCommentID) {
		return false
	}
	return value.InReplyTo == nil || validResourceID(value.InReplyTo.ID)
}

func validCommentListResponse(value CommentList, expectedBlogID, expectedPostID string) bool {
	if value.Kind != commentListKind || len(value.Raw) == 0 || !validPageToken(value.NextPageToken) || !validPageToken(value.PrevPageToken) {
		return false
	}
	for _, comment := range value.Items {
		if !validCommentResponse(comment, expectedBlogID, expectedPostID, "") {
			return false
		}
	}
	return true
}

func validPageResponse(value Page, expectedBlogID, expectedPageID string) bool {
	return value.Kind == pageKind && validResourceID(value.ID) && validResourceID(value.Blog.ID) && len(value.Raw) > 0 &&
		(expectedBlogID == "" || value.Blog.ID == expectedBlogID) && (expectedPageID == "" || value.ID == expectedPageID)
}

func validPageListResponse(value PageList, expectedBlogID string) bool {
	if value.Kind != pageListKind || len(value.Raw) == 0 || !validPageToken(value.NextPageToken) {
		return false
	}
	for _, page := range value.Items {
		if !validPageResponse(page, expectedBlogID, "") {
			return false
		}
	}
	return true
}

func sanitizeCause(err error) error {
	if err == nil {
		return nil
	}
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}

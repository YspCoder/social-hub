package applemusic

import (
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

func toPage[T any](collection apiCollection[T], requestPath string, baseURL *url.URL) (Page[T], error) {
	cursor, err := cursorFromNext(collection.Next, requestPath, baseURL)
	if err != nil {
		return Page[T]{}, err
	}
	return Page[T]{Data: collection.Data, NextCursor: cursor, Total: collection.Meta.Total}, nil
}

func cursorFromNext(next, requestPath string, baseURL *url.URL) (*string, error) {
	if next == "" {
		return nil, nil
	}
	parsed, err := url.Parse(next)
	if err != nil || parsed.User != nil || parsed.Fragment != "" || (parsed.Host != "" && !parsed.IsAbs()) {
		return nil, platformError("pagination", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if parsed.IsAbs() && (parsed.Scheme != baseURL.Scheme || parsed.Host != baseURL.Host) {
		return nil, platformError("pagination", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	expectedPath := strings.TrimRight(baseURL.Path, "/") + "/" + strings.TrimLeft(requestPath, "/")
	if parsed.Path != expectedPath {
		return nil, platformError("pagination", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	values := parsed.Query()["offset"]
	if len(values) != 1 {
		return nil, platformError("pagination", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	offset, ok := parseOffset(values[0])
	if !ok {
		return nil, platformError("pagination", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &offset, nil
}

func requestStorefront(requested, configured, operation string) (string, error) {
	storefront := requested
	if storefront == "" {
		storefront = configured
	}
	if !validStorefront(storefront) {
		return "", invalidArgument(operation, "storefront must be an ISO 3166 alpha-2 code")
	}
	return strings.ToLower(storefront), nil
}

func addPageQuery(query url.Values, request PaginationRequest) error {
	if _, ok := parseOffset(request.Cursor); !ok || request.MaxResults < 0 || !validLanguage(request.Language) {
		return invalidArgument("pagination", "cursor, max results, or language is invalid")
	}
	if request.Cursor != "" {
		query.Set("offset", request.Cursor)
	}
	if request.MaxResults > 0 {
		query.Set("limit", strconv.Itoa(request.MaxResults))
	}
	if request.Language != "" {
		query.Set("l", request.Language)
	}
	return nil
}

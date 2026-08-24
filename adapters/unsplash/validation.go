package unsplash

import (
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

const maxPageNumber = 1_000_000_000

func validOpaque(value string, maximum int) bool {
	if value == "" || value != strings.TrimSpace(value) || len(value) > maximum || !utf8.ValidString(value) {
		return false
	}
	return !strings.ContainsFunc(value, unicode.IsControl)
}

func validResourceID(value string) bool {
	if !validOpaque(value, 512) || value == "." || value == ".." {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || strings.ContainsRune("-_.~", character) {
			continue
		}
		return false
	}
	return true
}

func validPage(page, perPage int) bool {
	return page >= 0 && page <= maxPageNumber && perPage >= 0 && perPage <= MaximumPageSize
}

func effectivePage(value int) int {
	if value == 0 {
		return 1
	}
	return value
}

func effectivePerPage(value int) int {
	if value == 0 {
		return 10
	}
	return value
}

func validSearchRequest(input SearchPhotosRequest) bool {
	if !validOpaque(input.Query, 1000) || !validPage(input.Page, input.PerPage) ||
		!validSearchOrder(input.OrderBy) || !validOrientation(input.Orientation) ||
		!validContentFilter(input.ContentFilter) || !validColor(input.Color) ||
		!validSearchLanguage(input.Language) || len(input.Collections) > 50 {
		return false
	}
	seen := make(map[string]struct{}, len(input.Collections))
	for _, identifier := range input.Collections {
		if !validResourceID(identifier) {
			return false
		}
		if _, duplicate := seen[identifier]; duplicate {
			return false
		}
		seen[identifier] = struct{}{}
	}
	return true
}

const supportedSearchLanguages = "|af|sq|am|ar|hy|as|az|bn|ba|eu|bs|bg|yue|ca|lzh|zh-Hans|zh-Hant|hr|cs|da|prs|dv|nl|en|et|fo|fj|fil|fi|fr|fr-ca|gl|ka|de|el|gu|ht|he|hi|mww|hu|is|id|ikt|iu|iu-Latn|ga|it|ja|kn|kk|km|ko|ku|kmr|ky|lo|lv|lt|mk|mg|ms|ml|mt|mi|mr|mn-Cyrl|mn-Mong|my|ne|nb|or|ps|fa|pl|pt|pt-pt|pa|otq|ro|ru|sm|sr-Cyrl|sr-Latn|sk|sl|so|es|sw|sv|ty|ta|tt|te|th|bo|ti|to|tr|tk|uk|hsb|ur|ug|uz|vi|cy|yua|zu|"

func validSearchLanguage(value SearchLanguage) bool {
	if value == "" {
		return true
	}
	text := string(value)
	if len(text) > 16 || strings.ContainsFunc(text, func(character rune) bool {
		return character != '-' && (character < 'A' || character > 'Z') && (character < 'a' || character > 'z')
	}) {
		return false
	}
	return strings.Contains(supportedSearchLanguages, "|"+text+"|")
}

func validSearchOrder(value SearchOrder) bool {
	return value == "" || value == SearchOrderLatest || value == SearchOrderEditorial || value == SearchOrderRelevant
}

func validUserPhotoOrder(value UserPhotoOrder) bool {
	switch value {
	case "", UserPhotoOrderLatest, UserPhotoOrderOldest, UserPhotoOrderPopular,
		UserPhotoOrderViews, UserPhotoOrderDownloads, UserPhotoOrderPinned:
		return true
	default:
		return false
	}
}

func validOrientation(value Orientation) bool {
	return value == "" || value == OrientationLandscape || value == OrientationPortrait || value == OrientationSquarish
}

func validContentFilter(value ContentFilter) bool {
	return value == "" || value == ContentFilterLow || value == ContentFilterHigh
}

func validColor(value Color) bool {
	switch value {
	case "", ColorBlackAndWhite, ColorBlack, ColorWhite, ColorYellow, ColorOrange, ColorRed,
		ColorPurple, ColorMagenta, ColorGreen, ColorTeal, ColorBlue:
		return true
	default:
		return false
	}
}

func prepareCallOptions(operation string, options []socialhub.CallOption) error {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" {
		return invalidArgument(operation, "Unsplash does not document a caller request-ID header")
	}
	if resolved.IdempotencyKey != "" {
		return invalidArgument(operation, "read-only Unsplash operations do not use idempotency keys")
	}
	if len(resolved.Fields) > 0 {
		return invalidArgument(operation, "field selection is fixed by the typed Unsplash operation")
	}
	return nil
}

func validHTTPSURL(value string) bool {
	if !validOpaque(value, 8192) {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil && parsed.Fragment == ""
}

func validatePhoto(value Photo) bool {
	if !validResourceID(value.ID) || value.Width <= 0 || value.Height <= 0 || value.Likes < 0 || value.Downloads < 0 ||
		value.CurrentUserCollections == nil || value.User == nil || !validateUser(*value.User) {
		return false
	}
	if !validHTTPSURL(value.Links.Self) || !validHTTPSURL(value.Links.HTML) || !validHTTPSURL(value.Links.Download) {
		return false
	}
	for _, imageURL := range []string{value.URLs.Raw, value.URLs.Full, value.URLs.Regular, value.URLs.Small, value.URLs.Thumb} {
		if !validHTTPSURL(imageURL) {
			return false
		}
	}
	for _, collection := range value.CurrentUserCollections {
		if !validResourceID(string(collection.ID)) || collection.TotalPhotos < 0 ||
			!validHTTPSURL(collection.Links.Self) || !validHTTPSURL(collection.Links.HTML) ||
			!validHTTPSURL(collection.Links.Photos) {
			return false
		}
	}
	_, _, err := parseDownloadLocation(value.Links.DownloadLocation)
	return err == nil
}

func validateUser(value User) bool {
	if !validResourceID(value.ID) || !validResourceID(value.Username) || value.TotalCollections < 0 ||
		value.TotalLikes < 0 || value.TotalPhotos < 0 || value.Downloads < 0 ||
		!validHTTPSURL(value.Links.Self) || !validHTTPSURL(value.Links.HTML) || !validHTTPSURL(value.Links.Photos) {
		return false
	}
	for _, imageURL := range []string{value.ProfileImage.Small, value.ProfileImage.Medium, value.ProfileImage.Large} {
		if !validHTTPSURL(imageURL) {
			return false
		}
	}
	return true
}

func parseDownloadLocation(value string) (string, url.Values, error) {
	const operation = "track_download"
	if !validOpaque(value, 8192) {
		return "", nil, invalidArgument(operation, "download_location is invalid")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Opaque != "" || parsed.User != nil || parsed.Fragment != "" || parsed.RawPath != "" {
		return "", nil, invalidArgument(operation, "download_location is invalid")
	}
	if !strings.EqualFold(parsed.Scheme, "https") || !strings.EqualFold(parsed.Host, "api.unsplash.com") {
		return "", nil, invalidArgument(operation, "download_location must use the official Unsplash API origin")
	}
	segments := strings.Split(strings.TrimPrefix(parsed.Path, "/"), "/")
	if len(segments) != 3 || segments[0] != "photos" || !validResourceID(segments[1]) || segments[2] != "download" {
		return "", nil, invalidArgument(operation, "download_location path is invalid")
	}
	query, err := url.ParseQuery(parsed.RawQuery)
	if err != nil || len(query) > 32 {
		return "", nil, invalidArgument(operation, "download_location query is invalid")
	}
	for key, values := range query {
		lower := strings.ToLower(key)
		if !validOpaque(key, 128) || strings.Contains(lower, "token") || strings.Contains(lower, "secret") ||
			lower == "client_id" || lower == "authorization" || len(values) > 8 {
			return "", nil, invalidArgument(operation, "download_location query is invalid")
		}
		for _, item := range values {
			if !validOpaque(item, 4096) {
				return "", nil, invalidArgument(operation, "download_location query is invalid")
			}
		}
	}
	return parsed.Path, query, nil
}

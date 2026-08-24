package ximalaya

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

const maxPage = 1_000_000

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

func validCredential(value string) bool {
	if !validOpaque(value, 4096) {
		return false
	}
	for _, character := range value {
		if unicode.IsSpace(character) {
			return false
		}
	}
	return true
}

func validDeviceIDType(value DeviceIDType) bool {
	switch value {
	case DeviceOAID, DeviceOAIDMD5, DeviceAndroidID, DeviceAndroidMD5, DeviceIDFA, DeviceIDFAMD5, DeviceUUID:
		return true
	default:
		return false
	}
}

func validAccountSettings(settings AccountSettings) bool {
	return settings.ClientOSType >= 1 && settings.ClientOSType <= 8 &&
		validOpaque(settings.DeviceID, 256) && validDeviceIDType(settings.DeviceIDType)
}

func validID(value ID) bool { return value > 0 }

func validOptionalID(value ID) bool { return value >= 0 }

func validText(value string, maximum int) bool {
	return value == "" || validOpaque(value, maximum)
}

func validPage(page, count, maximumCount int) bool {
	return page >= 0 && page <= maxPage && count >= 0 && count <= maximumCount
}

func effectivePage(value int) int {
	if value == 0 {
		return 1
	}
	return value
}

func effectiveCount(value, fallback int) int {
	if value == 0 {
		return fallback
	}
	return value
}

func validateListAlbums(input ListAlbumsRequest) error {
	if !validID(input.CategoryID) {
		return invalidArgument("list_albums", "category_id must be a positive integer")
	}
	if input.Dimension < AlbumsHot || input.Dimension > AlbumsMostPlayed {
		return invalidArgument("list_albums", "dimension must be 1, 2, or 3")
	}
	if !validText(input.TagName, 512) {
		return invalidArgument("list_albums", "tag_name is invalid")
	}
	if !validPage(input.Page, input.Count, 50) {
		return invalidArgument("list_albums", "page must be positive and count must be between 1 and 50 when set")
	}
	return nil
}

func validateBrowseAlbum(input BrowseAlbumRequest) error {
	if !validID(input.AlbumID) {
		return invalidArgument("browse_album", "album_id must be a positive integer")
	}
	switch input.Sort {
	case "", TrackSortXimalayaAscending, TrackSortXimalayaDescending, TrackSortTimeAscending, TrackSortTimeDescending:
	default:
		return invalidArgument("browse_album", "sort is invalid")
	}
	if !validPage(input.Page, input.Count, 200) {
		return invalidArgument("browse_album", "page must be positive and count must be between 1 and 200 when set")
	}
	return nil
}

func validateSearchWindow(operation string, page, count int) error {
	if !validPage(page, count, 50) {
		return invalidArgument(operation, "page must be positive and count must be between 1 and 50 when set")
	}
	if int64(effectivePage(page))*int64(effectiveCount(count, 20)) > 5000 {
		return invalidArgument(operation, "requested page exceeds Ximalaya's first-5000 search result window")
	}
	return nil
}

func validateSearchAlbums(input SearchAlbumsRequest) error {
	const operation = "search_albums"
	if !validOptionalID(input.ID) || !validOptionalID(input.AnnouncerID) || !validOptionalID(input.CategoryID) {
		return invalidArgument(operation, "album, announcer, and category IDs must be positive when set")
	}
	if !validText(input.Title, 4096) || !validText(input.Nickname, 512) ||
		!validText(input.Tags, 512) || !validText(input.CategoryName, 512) {
		return invalidArgument(operation, "a search text field is invalid")
	}
	if input.PriceType < 0 || input.PriceType > 2 {
		return invalidArgument(operation, "price_type must be 1 or 2 when set")
	}
	switch input.SortBy {
	case "", AlbumSortCreated, AlbumSortUpdated, AlbumSortDiscountedPrice, AlbumSortPlayCount, AlbumSortWeekScore:
	default:
		return invalidArgument(operation, "sort_by is invalid")
	}
	return validateSearchWindow(operation, input.Page, input.Count)
}

func validateSearchTracks(input SearchTracksRequest) error {
	const operation = "search_tracks"
	if !validOptionalID(input.ID) || !validOptionalID(input.AlbumID) ||
		!validOptionalID(input.AnnouncerID) || !validOptionalID(input.CategoryID) {
		return invalidArgument(operation, "track, album, announcer, and category IDs must be positive when set")
	}
	if !validText(input.Title, 4096) || !validText(input.AlbumTitle, 4096) ||
		!validText(input.Nickname, 512) || !validText(input.Tags, 512) || !validText(input.CategoryName, 512) {
		return invalidArgument(operation, "a search text field is invalid")
	}
	switch input.SortBy {
	case "", TrackSearchCreated, TrackSearchUpdated:
	default:
		return invalidArgument(operation, "sort_by is invalid")
	}
	return validateSearchWindow(operation, input.Page, input.Count)
}

func prepareCallOptions(operation string, options []socialhub.CallOption) error {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" {
		return invalidArgument(operation, "Ximalaya Open API does not document a caller request-ID parameter")
	}
	if resolved.IdempotencyKey != "" {
		return invalidArgument(operation, "read-only Ximalaya operations do not use idempotency keys")
	}
	if len(resolved.Fields) > 0 {
		return invalidArgument(operation, "field selection is fixed by the typed Ximalaya operation")
	}
	return nil
}

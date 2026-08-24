package googlephotos

import (
	"strings"
	"time"
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

func containsScope(scopes []string, target string) bool {
	for _, scope := range scopes {
		if scope == target {
			return true
		}
	}
	return false
}

func removedLibraryScope(scope string) bool {
	switch scope {
	case "https://www.googleapis.com/auth/photoslibrary",
		"https://www.googleapis.com/auth/photoslibrary.readonly",
		"https://www.googleapis.com/auth/photoslibrary.sharing":
		return true
	default:
		return false
	}
}

func validResourceID(value string) bool {
	return validOpaque(value, 1024) && value != "." && value != ".." && !strings.ContainsAny(value, "/\\?#%")
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

func validPage(options PageOptions, maximum int) bool {
	return options.PageSize >= 0 && options.PageSize <= maximum && validPageToken(options.PageToken)
}

func validDate(value Date) bool {
	if value.Year < 0 || value.Year > 9999 || value.Month < 0 || value.Month > 12 || value.Day < 0 || value.Day > 31 {
		return false
	}
	if value.Year == 0 && value.Month == 0 && value.Day == 0 {
		return false
	}
	if value.Month == 0 {
		return value.Year > 0 && value.Day == 0
	}
	if value.Day == 0 {
		return value.Year > 0
	}
	validationYear := value.Year
	if validationYear == 0 {
		validationYear = 2000
	}
	date := time.Date(validationYear, time.Month(value.Month), value.Day, 0, 0, 0, 0, time.UTC)
	return date.Year() == validationYear && int(date.Month()) == value.Month && date.Day() == value.Day
}

func dateShape(value Date) int {
	shape := 0
	if value.Year != 0 {
		shape |= 4
	}
	if value.Month != 0 {
		shape |= 2
	}
	if value.Day != 0 {
		shape |= 1
	}
	return shape
}

func dateKey(value Date) int {
	return value.Year*10_000 + value.Month*100 + value.Day
}

func validDateFilter(filter *DateFilter) bool {
	if filter == nil || len(filter.Dates) == 0 && len(filter.Ranges) == 0 || len(filter.Dates) > 5 || len(filter.Ranges) > 5 {
		return false
	}
	for _, date := range filter.Dates {
		if !validDate(date) {
			return false
		}
	}
	for _, dateRange := range filter.Ranges {
		if !validDate(dateRange.StartDate) || !validDate(dateRange.EndDate) ||
			dateShape(dateRange.StartDate) != dateShape(dateRange.EndDate) || dateKey(dateRange.StartDate) > dateKey(dateRange.EndDate) {
			return false
		}
	}
	return true
}

func validMediaType(value MediaType) bool {
	switch value {
	case MediaTypeAll, MediaTypeVideo, MediaTypePhoto:
		return true
	default:
		return false
	}
}

func validMediaTypeFilter(filter *MediaTypeFilter) bool {
	return filter != nil && len(filter.MediaTypes) == 1 && validMediaType(filter.MediaTypes[0])
}

func validContentCategory(value ContentCategory) bool {
	switch value {
	case ContentNone, ContentLandscapes, ContentReceipts, ContentCityscapes, ContentLandmarks,
		ContentSelfies, ContentPeople, ContentPets, ContentWeddings, ContentBirthdays,
		ContentDocuments, ContentTravel, ContentAnimals, ContentFood, ContentSport,
		ContentNight, ContentPerformances, ContentWhiteboards, ContentScreenshots,
		ContentUtility, ContentArts, ContentCrafts, ContentFashion, ContentHouses,
		ContentGardens, ContentFlowers, ContentHolidays:
		return true
	default:
		return false
	}
}

func validCategorySet(values []ContentCategory) bool {
	if len(values) > 10 {
		return false
	}
	seen := make(map[ContentCategory]struct{}, len(values))
	for _, value := range values {
		if !validContentCategory(value) {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	if _, hasNone := seen[ContentNone]; hasNone && len(values) > 1 {
		return false
	}
	return true
}

func validContentFilter(filter *ContentFilter) bool {
	if filter == nil || len(filter.IncludedContentCategories) == 0 && len(filter.ExcludedContentCategories) == 0 ||
		!validCategorySet(filter.IncludedContentCategories) || !validCategorySet(filter.ExcludedContentCategories) {
		return false
	}
	included := make(map[ContentCategory]struct{}, len(filter.IncludedContentCategories))
	for _, category := range filter.IncludedContentCategories {
		included[category] = struct{}{}
	}
	for _, category := range filter.ExcludedContentCategories {
		if _, exists := included[category]; exists {
			return false
		}
	}
	return true
}

func validFeatureFilter(filter *FeatureFilter) bool {
	if filter == nil || len(filter.IncludedFeatures) != 1 {
		return false
	}
	return filter.IncludedFeatures[0] == FeatureNone || filter.IncludedFeatures[0] == FeatureFavorites
}

func validSearchFilters(filters *SearchFilters) bool {
	if filters == nil {
		return true
	}
	return (filters.DateFilter == nil || validDateFilter(filters.DateFilter)) &&
		(filters.ContentFilter == nil || validContentFilter(filters.ContentFilter)) &&
		(filters.FeatureFilter == nil || validFeatureFilter(filters.FeatureFilter)) &&
		(filters.MediaTypeFilter == nil || validMediaTypeFilter(filters.MediaTypeFilter))
}

func validSearchRequest(input SearchMediaItemsRequest) bool {
	if !validPage(input.Page, 100) || !validSearchFilters(input.Filters) || input.AlbumID != "" && !validResourceID(input.AlbumID) {
		return false
	}
	if input.AlbumID != "" && (input.Filters != nil || input.OrderBy != "") {
		return false
	}
	if input.OrderBy == "" {
		return true
	}
	if input.OrderBy != SearchOrderOldestFirst && input.OrderBy != SearchOrderNewestFirst || input.Filters == nil || input.Filters.DateFilter == nil {
		return false
	}
	return input.Filters.ContentFilter == nil && input.Filters.FeatureFilter == nil && input.Filters.MediaTypeFilter == nil
}

func validAlbum(value Album) bool {
	return validResourceID(value.ID)
}

func validMediaItem(value MediaItem) bool {
	return validResourceID(value.ID)
}

func prepareCallOptions(operation string, options []socialhub.CallOption) error {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" {
		return invalidArgument(operation, "Google Photos Library API does not document a caller request-ID header")
	}
	if resolved.IdempotencyKey != "" {
		return invalidArgument(operation, "read-only Google Photos operations do not use idempotency keys")
	}
	if len(resolved.Fields) > 0 {
		return invalidArgument(operation, "partial response fields are fixed by the typed Google Photos operation")
	}
	return nil
}

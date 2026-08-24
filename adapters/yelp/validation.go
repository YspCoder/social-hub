package yelp

import (
	"math"
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

func validAPIKey(value string) bool {
	if len(value) != 128 || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || character == '-' || character == '_' {
			continue
		}
		return false
	}
	return true
}

func validOptionalOpaque(value string, maximum int) bool {
	return value == "" || validOpaque(value, maximum)
}

func validBusinessIDOrAlias(value string) bool {
	return validOpaque(value, 512) && value != "." && value != ".." && !strings.ContainsAny(value, "/\\?#%")
}

func validLocale(value string) bool {
	if value == "" {
		return true
	}
	if len(value) != 5 && len(value) != 6 {
		return false
	}
	underscore := len(value) - 3
	if value[underscore] != '_' {
		return false
	}
	for index := 0; index < underscore; index++ {
		if value[index] < 'a' || value[index] > 'z' {
			return false
		}
	}
	return value[underscore+1] >= 'A' && value[underscore+1] <= 'Z' &&
		value[underscore+2] >= 'A' && value[underscore+2] <= 'Z'
}

func validCoordinates(latitude, longitude *float64) bool {
	if latitude == nil || longitude == nil || math.IsNaN(*latitude) || math.IsInf(*latitude, 0) ||
		math.IsNaN(*longitude) || math.IsInf(*longitude, 0) {
		return false
	}
	return *latitude >= -90 && *latitude <= 90 && *longitude >= -180 && *longitude <= 180
}

func validSearchBusinesses(input SearchBusinessesRequest) bool {
	hasLocation := input.Location != ""
	hasAnyCoordinate := input.Latitude != nil || input.Longitude != nil
	if hasLocation == hasAnyCoordinate || hasLocation && !validOpaque(input.Location, 250) ||
		hasAnyCoordinate && !validCoordinates(input.Latitude, input.Longitude) {
		return false
	}
	if !validOptionalOpaque(input.Term, 300) || !validLocale(input.Locale) {
		return false
	}
	if input.Radius != nil && (*input.Radius < 0 || *input.Radius > MaximumSearchRadiusMeters) {
		return false
	}
	seenCategories := make(map[string]struct{}, len(input.Categories))
	for _, category := range input.Categories {
		if !validOpaque(category, 256) || strings.Contains(category, ",") {
			return false
		}
		if _, exists := seenCategories[category]; exists {
			return false
		}
		seenCategories[category] = struct{}{}
	}
	seenPrices := make(map[int]struct{}, len(input.Price))
	for _, price := range input.Price {
		if price < 1 || price > 4 {
			return false
		}
		if _, exists := seenPrices[price]; exists {
			return false
		}
		seenPrices[price] = struct{}{}
	}
	if input.OpenNow != nil && input.OpenAt != nil || input.OpenAt != nil && *input.OpenAt <= 0 {
		return false
	}
	seenAttributes := make(map[BusinessAttribute]struct{}, len(input.Attributes))
	for _, attribute := range input.Attributes {
		if !validBusinessAttribute(attribute) {
			return false
		}
		if _, exists := seenAttributes[attribute]; exists {
			return false
		}
		seenAttributes[attribute] = struct{}{}
	}
	if !validBusinessSort(input.SortBy) || input.Limit < 0 || input.Limit > MaximumPageSize ||
		input.Offset < 0 || input.Offset > MaximumOffset {
		return false
	}
	return true
}

func validBusinessAttribute(value BusinessAttribute) bool {
	switch value {
	case AttributeHotAndNew, AttributeRequestAQuote, AttributeReservation,
		AttributeWaitlistReservation, AttributeGenderNeutralRestroom,
		AttributeOpenToAll, AttributeWheelchairAccessible:
		return true
	default:
		return false
	}
}

func validBusinessSort(value BusinessSort) bool {
	switch value {
	case "", SortBestMatch, SortRating, SortReviewCount, SortDistance:
		return true
	default:
		return false
	}
}

func validGetBusiness(input GetBusinessRequest) bool {
	return validBusinessIDOrAlias(input.BusinessIDOrAlias) && validLocale(input.Locale)
}

func validListReviews(input ListReviewsRequest) bool {
	return validBusinessIDOrAlias(input.BusinessIDOrAlias) && validLocale(input.Locale) &&
		input.Offset >= 0 && input.Offset <= MaximumOffset && input.Limit >= 0 && input.Limit <= MaximumPageSize
}

func validListCategories(input ListCategoriesRequest) bool {
	return validLocale(input.Locale)
}

func validBusiness(value Business) bool {
	return validOpaque(value.ID, 512)
}

func validSearchBusinessesResponse(value SearchBusinessesResponse) bool {
	if value.Businesses == nil || len(value.Businesses) > MaximumPageSize || value.Total < len(value.Businesses) {
		return false
	}
	seen := make(map[string]struct{}, len(value.Businesses))
	for _, business := range value.Businesses {
		if !validBusiness(business) {
			return false
		}
		if _, exists := seen[business.ID]; exists {
			return false
		}
		seen[business.ID] = struct{}{}
	}
	return true
}

func validReviewsResponse(value ReviewsResponse) bool {
	if value.Reviews == nil || len(value.Reviews) > MaximumPageSize || value.Total < len(value.Reviews) {
		return false
	}
	seen := make(map[string]struct{}, len(value.Reviews))
	for _, review := range value.Reviews {
		if !validOpaque(review.ID, 512) {
			return false
		}
		if _, exists := seen[review.ID]; exists {
			return false
		}
		seen[review.ID] = struct{}{}
	}
	return true
}

func validCategoriesResponse(value CategoriesResponse) bool {
	if value.Categories == nil {
		return false
	}
	seen := make(map[string]struct{}, len(value.Categories))
	for _, category := range value.Categories {
		if !validOpaque(category.Alias, 256) {
			return false
		}
		if _, exists := seen[category.Alias]; exists {
			return false
		}
		seen[category.Alias] = struct{}{}
	}
	return true
}

func prepareCallOptions(operation string, options []socialhub.CallOption) error {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" {
		return invalidArgument(operation, "Yelp Places does not document a caller request-ID header")
	}
	if resolved.IdempotencyKey != "" {
		return invalidArgument(operation, "read-only Yelp Places operations do not use idempotency keys")
	}
	if len(resolved.Fields) > 0 {
		return invalidArgument(operation, "field selection is fixed by the typed Yelp Places operation")
	}
	return nil
}

package tripadvisor

import (
	"math"
	"strings"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

var supportedLanguages = map[string]struct{}{
	"ar": {}, "zh": {}, "zh_TW": {}, "da": {}, "nl": {}, "en_AU": {}, "en_CA": {}, "en_HK": {},
	"en_IN": {}, "en_IE": {}, "en_MY": {}, "en_NZ": {}, "en_PH": {}, "en_SG": {}, "en_ZA": {},
	"en_UK": {}, "en": {}, "fr": {}, "fr_BE": {}, "fr_CA": {}, "fr_CH": {}, "de_AT": {}, "de": {},
	"el": {}, "iw": {}, "in": {}, "it": {}, "it_CH": {}, "ja": {}, "ko": {}, "no": {}, "pt_PT": {},
	"pt": {}, "ru": {}, "es_AR": {}, "es_CO": {}, "es_MX": {}, "es_PE": {}, "es": {}, "es_VE": {},
	"es_CL": {}, "sv": {}, "th": {}, "tr": {}, "vi": {},
}

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

func normalizeDecimalID(value string) (string, bool) {
	if value == "" || len(value) > 64 {
		return "", false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return "", false
		}
	}
	normalized := strings.TrimLeft(value, "0")
	if normalized == "" {
		return "", false
	}
	return normalized, true
}

func validID(value ID) bool {
	normalized, ok := normalizeDecimalID(string(value))
	return ok && normalized == string(value)
}

func validCategory(value Category) bool {
	switch value {
	case "", CategoryHotels, CategoryAttractions, CategoryRestaurants, CategoryGeos:
		return true
	default:
		return false
	}
}

func validRadiusUnit(value RadiusUnit) bool {
	switch value {
	case "", RadiusKilometers, RadiusMiles, RadiusMeters:
		return true
	default:
		return false
	}
}

func validLanguage(value string) bool {
	if value == "" {
		return true
	}
	_, found := supportedLanguages[value]
	return found
}

func validCurrency(value string) bool {
	if value == "" {
		return true
	}
	if len(value) != 3 {
		return false
	}
	for _, character := range value {
		if character < 'A' || character > 'Z' {
			return false
		}
	}
	return true
}

func validCoordinate(value Coordinate) bool {
	return !math.IsNaN(value.Latitude) && !math.IsInf(value.Latitude, 0) &&
		!math.IsNaN(value.Longitude) && !math.IsInf(value.Longitude, 0) &&
		value.Latitude >= -90 && value.Latitude <= 90 && value.Longitude >= -180 && value.Longitude <= 180
}

func validRadius(value *float64) bool {
	return value == nil || !math.IsNaN(*value) && !math.IsInf(*value, 0) && *value > 0
}

func validPhone(value string) bool {
	return validOptionalOpaque(value, 128) && !strings.HasPrefix(value, "+")
}

func validSearchRequest(input SearchLocationsRequest) bool {
	if !validOpaque(input.SearchQuery, 512) || !validCategory(input.Category) || !validPhone(input.Phone) ||
		!validOptionalOpaque(input.Address, 512) || !validRadius(input.Radius) ||
		!validRadiusUnit(input.RadiusUnit) || !validLanguage(input.Language) {
		return false
	}
	if input.Coordinate != nil && !validCoordinate(*input.Coordinate) {
		return false
	}
	if input.Radius != nil && input.Coordinate == nil || input.RadiusUnit != "" && input.Radius == nil {
		return false
	}
	return true
}

func validNearbyRequest(input SearchNearbyRequest) bool {
	return validCoordinate(input.Coordinate) && validCategory(input.Category) && validPhone(input.Phone) &&
		validOptionalOpaque(input.Address, 512) && validRadius(input.Radius) &&
		validRadiusUnit(input.RadiusUnit) && validLanguage(input.Language) &&
		(input.RadiusUnit == "" || input.Radius != nil)
}

func validDetailsRequest(input GetLocationDetailsRequest) bool {
	return validID(input.LocationID) && validLanguage(input.Language) && validCurrency(input.Currency)
}

func validPhotosRequest(input ListPhotosRequest) bool {
	if !validID(input.LocationID) || !validLanguage(input.Language) || input.Limit < 0 ||
		input.Limit > MaximumPageSize || input.Offset < 0 || input.Offset > MaximumOffset {
		return false
	}
	seen := make(map[PhotoSource]struct{}, len(input.Sources))
	for _, source := range input.Sources {
		switch source {
		case PhotoSourceExpert, PhotoSourceManagement, PhotoSourceTraveler:
		default:
			return false
		}
		if _, found := seen[source]; found {
			return false
		}
		seen[source] = struct{}{}
	}
	return true
}

func validReviewsRequest(input ListReviewsRequest) bool {
	return validID(input.LocationID) && validLanguage(input.Language) && input.Limit >= 0 &&
		input.Limit <= MaximumPageSize && input.Offset >= 0 && input.Offset <= MaximumOffset
}

func prepareCallOptions(operation string, options []socialhub.CallOption) error {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" {
		return invalidArgument(operation, "Tripadvisor Content API does not document a caller request-ID header")
	}
	if resolved.IdempotencyKey != "" {
		return invalidArgument(operation, "read-only Tripadvisor operations do not use idempotency keys")
	}
	if len(resolved.Fields) > 0 {
		return invalidArgument(operation, "field selection is fixed by the typed Tripadvisor operation")
	}
	return nil
}

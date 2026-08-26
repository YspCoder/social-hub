package foursquare

import (
	"math"
	"strings"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

var proFields = map[string]struct{}{
	string(FieldFSQPlaceID): {}, string(FieldName): {}, string(FieldCategories): {},
	string(FieldLocation): {}, string(FieldLatitude): {}, string(FieldLongitude): {},
	string(FieldDistance): {}, string(FieldTelephone): {}, string(FieldEmail): {},
	string(FieldWebsite): {}, string(FieldSocialMedia): {}, string(FieldLink): {},
	string(FieldDateClosed): {}, string(FieldPlacemakerURL): {}, string(FieldChains): {},
	string(FieldStoreID): {}, string(FieldRelatedPlaces): {}, string(FieldExtendedLocation): {},
	string(FieldUnresolvedFlags): {},
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

func validCoordinate(value Coordinate) bool {
	return !math.IsNaN(value.Latitude) && !math.IsInf(value.Latitude, 0) && value.Latitude >= -90 && value.Latitude <= 90 &&
		!math.IsNaN(value.Longitude) && !math.IsInf(value.Longitude, 0) && value.Longitude >= -180 && value.Longitude <= 180
}

func validateSearchRequest(input SearchRequest) error {
	const operation = "search_places"
	if !validOptionalOpaque(input.Query, 255) || !validOptionalOpaque(input.Near, 255) ||
		!validOptionalOpaque(input.Cursor, 4096) || input.Radius < 0 || input.Radius > 100_000 ||
		input.Limit < 0 || input.Limit > 50 || !validSort(input.Sort) {
		return invalidArgument(operation, "query, near, radius, sort, limit, or cursor is invalid")
	}
	if input.LL != nil && !validCoordinate(*input.LL) {
		return invalidArgument(operation, "ll coordinate is invalid")
	}
	if (input.NorthEast == nil) != (input.SouthWest == nil) {
		return invalidArgument(operation, "ne and sw must be supplied together")
	}
	if input.NorthEast != nil {
		if !validCoordinate(*input.NorthEast) || !validCoordinate(*input.SouthWest) ||
			input.NorthEast.Latitude <= input.SouthWest.Latitude || input.NorthEast.Longitude <= input.SouthWest.Longitude {
			return invalidArgument(operation, "ne and sw do not define a valid rectangle")
		}
	}
	locationModes := 0
	if input.LL != nil {
		locationModes++
	}
	if input.Near != "" {
		locationModes++
	}
	if input.NorthEast != nil {
		locationModes++
	}
	if locationModes > 1 {
		return invalidArgument(operation, "use at most one location mode")
	}
	if input.Radius > 0 && (input.Near != "" || input.NorthEast != nil) {
		return invalidArgument(operation, "radius can be used only with ll or IP-biased geolocation")
	}
	if len(input.CategoryIDs) > 50 {
		return invalidArgument(operation, "at most 50 fsq_category_ids may be supplied")
	}
	seen := make(map[string]struct{}, len(input.CategoryIDs))
	for _, identifier := range input.CategoryIDs {
		if !validIdentifier(identifier, 128) {
			return invalidArgument(operation, "fsq_category_ids contain an invalid value")
		}
		if _, exists := seen[identifier]; exists {
			return invalidArgument(operation, "fsq_category_ids contain a duplicate")
		}
		seen[identifier] = struct{}{}
	}
	return nil
}

func validSort(value Sort) bool {
	switch value {
	case "", SortRelevance, SortRating, SortDistance, SortPopularity:
		return true
	default:
		return false
	}
}

func validIdentifier(value string, maximum int) bool {
	if value == "" || len(value) > maximum {
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

func validPlaceID(value string) bool { return validIdentifier(value, 128) }

func prepareCallOptions(operation string, options []socialhub.CallOption) (socialhub.CallOptions, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return socialhub.CallOptions{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.IdempotencyKey != "" {
		return socialhub.CallOptions{}, invalidArgument(operation, "read-only Places API operations do not support idempotency keys")
	}
	if !validOptionalOpaque(resolved.RequestID, 256) {
		return socialhub.CallOptions{}, invalidArgument(operation, "request ID is invalid")
	}
	return resolved, nil
}

func resolveFields(operation string, typed []PlaceField, generic []string) ([]string, error) {
	if len(typed) > 0 && len(generic) > 0 {
		return nil, invalidArgument(operation, "use SearchRequest.Fields or socialhub.WithFields, not both")
	}
	requested := make([]string, 0, len(typed)+len(generic)+1)
	for _, field := range typed {
		requested = append(requested, string(field))
	}
	requested = append(requested, generic...)
	if len(requested) == 0 {
		return nil, nil
	}
	output := []string{string(FieldFSQPlaceID)}
	seen := map[string]struct{}{string(FieldFSQPlaceID): {}}
	for _, field := range requested {
		if _, allowed := proFields[field]; !allowed {
			return nil, invalidArgument(operation, "fields contains a Premium or unsupported response field")
		}
		if _, exists := seen[field]; exists {
			continue
		}
		seen[field] = struct{}{}
		output = append(output, field)
	}
	return output, nil
}

package googleplaces

import (
	"math"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

var supportedPlaceFields = map[PlaceField]struct{}{
	FieldID: {}, FieldResourceName: {}, FieldDisplayName: {}, FieldFormattedAddress: {},
	FieldShortFormattedAddress: {}, FieldLocation: {}, FieldViewport: {}, FieldTypes: {},
	FieldPrimaryType: {}, FieldPrimaryTypeDisplayName: {}, FieldBusinessStatus: {},
	FieldGoogleMapsURI: {}, FieldWebsiteURI: {}, FieldInternationalPhoneNumber: {},
	FieldNationalPhoneNumber: {}, FieldRating: {}, FieldUserRatingCount: {},
	FieldPriceLevel: {}, FieldUTCOffsetMinutes: {}, FieldPhotos: {}, FieldAttributions: {},
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

func validResourceSegment(value string, maximum int) bool {
	return validOpaque(value, maximum) && value != "." && value != ".." && !strings.ContainsAny(value, "/\\?#%")
}

func validPlaceID(value string) bool {
	return validResourceSegment(value, 512)
}

func validPlaceName(value string) bool {
	prefix, placeID, found := strings.Cut(value, "/")
	return found && prefix == "places" && validPlaceID(placeID)
}

func validPhotoName(value string) bool {
	parts := strings.Split(value, "/")
	return len(parts) == 4 && parts[0] == "places" && validPlaceID(parts[1]) &&
		parts[2] == "photos" && validResourceSegment(parts[3], 2048)
}

func validPhotoMediaName(value string) bool {
	return strings.HasSuffix(value, "/media") && validPhotoName(strings.TrimSuffix(value, "/media"))
}

func placeIDFromPhotoName(value string) string {
	parts := strings.Split(value, "/")
	if len(parts) != 4 {
		return ""
	}
	return parts[1]
}

func validLanguageCode(value string) bool {
	if value == "" {
		return true
	}
	if len(value) > 35 || value[0] == '-' || value[len(value)-1] == '-' || strings.Contains(value, "--") {
		return false
	}
	for _, character := range value {
		if character != '-' && (character < 'A' || character > 'Z') &&
			(character < 'a' || character > 'z') && (character < '0' || character > '9') {
			return false
		}
	}
	return true
}

func validRegionCode(value string) bool {
	if value == "" {
		return true
	}
	return len(value) == 2 && isASCIILetter(value[0]) && isASCIILetter(value[1])
}

func isASCIILetter(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}

func validPlaceType(value string) bool {
	if value == "" || len(value) > 100 {
		return false
	}
	for _, character := range value {
		if character != '_' && (character < 'a' || character > 'z') && (character < '0' || character > '9') {
			return false
		}
	}
	return true
}

func validLatLng(value LatLng) bool {
	return !math.IsNaN(value.Latitude) && !math.IsInf(value.Latitude, 0) &&
		!math.IsNaN(value.Longitude) && !math.IsInf(value.Longitude, 0) &&
		value.Latitude >= -90 && value.Latitude <= 90 && value.Longitude >= -180 && value.Longitude <= 180
}

func validCircle(value Circle) bool {
	return validLatLng(value.Center) && !math.IsNaN(value.Radius) && !math.IsInf(value.Radius, 0) &&
		value.Radius >= 0 && value.Radius <= 50_000
}

func validViewport(value Viewport) bool {
	if !validLatLng(value.Low) || !validLatLng(value.High) || value.Low.Latitude > value.High.Latitude {
		return false
	}
	if value.Low.Longitude == 180 && value.High.Longitude == -180 {
		return false
	}
	return true
}

func validLocationBias(value *LocationBias) bool {
	if value == nil {
		return true
	}
	if (value.Circle == nil) == (value.Rectangle == nil) {
		return false
	}
	return value.Circle != nil && validCircle(*value.Circle) || value.Rectangle != nil && validViewport(*value.Rectangle)
}

func validPriceLevel(value PriceLevel) bool {
	switch value {
	case PriceLevelUnspecified, PriceLevelFree, PriceLevelInexpensive, PriceLevelModerate,
		PriceLevelExpensive, PriceLevelVeryExpensive:
		return true
	default:
		return false
	}
}

func validTextRank(value TextRankPreference) bool {
	return value == "" || value == TextRankDistance || value == TextRankRelevance
}

func validNearbyRank(value NearbyRankPreference) bool {
	return value == "" || value == NearbyRankDistance || value == NearbyRankPopularity
}

func validMinRating(value *float64) bool {
	if value == nil {
		return true
	}
	return !math.IsNaN(*value) && !math.IsInf(*value, 0) && *value >= 0 && *value <= 5
}

func validTextSearch(input TextSearchRequest) bool {
	if !validOpaque(input.TextQuery, 4096) || !validLanguageCode(input.LanguageCode) ||
		!validRegionCode(input.RegionCode) || !validTextRank(input.RankPreference) ||
		input.IncludedType != "" && !validPlaceType(input.IncludedType) ||
		!validMinRating(input.MinRating) || input.PageSize < 0 || input.PageSize > MaximumSearchPageSize ||
		!validOptionalOpaque(input.PageToken, 8192) || !validLocationBias(input.LocationBias) ||
		input.LocationRestriction != nil && !validViewport(input.LocationRestriction.Rectangle) ||
		input.LocationBias != nil && input.LocationRestriction != nil {
		return false
	}
	seenPrices := make(map[PriceLevel]struct{}, len(input.PriceLevels))
	for _, price := range input.PriceLevels {
		if !validPriceLevel(price) {
			return false
		}
		if _, found := seenPrices[price]; found {
			return false
		}
		seenPrices[price] = struct{}{}
	}
	return true
}

func validNearbySearch(input NearbySearchRequest) bool {
	if !validLanguageCode(input.LanguageCode) || !validRegionCode(input.RegionCode) ||
		!validCircle(input.LocationRestriction.Circle) || !validNearbyRank(input.RankPreference) ||
		input.MaxResultCount < 0 || input.MaxResultCount > MaximumSearchPageSize {
		return false
	}
	if input.MaxResultCount == 0 {
		// Omission selects the documented default of 20.
	} else if input.MaxResultCount < 1 {
		return false
	}
	included, ok := validateTypeList(input.IncludedTypes)
	if !ok {
		return false
	}
	excluded, ok := validateTypeList(input.ExcludedTypes)
	if !ok || intersects(included, excluded) {
		return false
	}
	includedPrimary, ok := validateTypeList(input.IncludedPrimaryTypes)
	if !ok {
		return false
	}
	excludedPrimary, ok := validateTypeList(input.ExcludedPrimaryTypes)
	return ok && !intersects(includedPrimary, excludedPrimary)
}

func validateTypeList(values []string) (map[string]struct{}, bool) {
	if len(values) > MaximumNearbyTypes {
		return nil, false
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validPlaceType(value) {
			return nil, false
		}
		if _, found := seen[value]; found {
			return nil, false
		}
		seen[value] = struct{}{}
	}
	return seen, true
}

func intersects(left, right map[string]struct{}) bool {
	for value := range left {
		if _, found := right[value]; found {
			return true
		}
	}
	return false
}

func validSessionToken(value string) bool {
	if value == "" {
		return true
	}
	if len(value) > 36 {
		return false
	}
	for _, character := range value {
		if character != '-' && character != '_' && (character < 'A' || character > 'Z') &&
			(character < 'a' || character > 'z') && (character < '0' || character > '9') {
			return false
		}
	}
	return true
}

func validGetPlace(input GetPlaceRequest) bool {
	return validPlaceID(input.PlaceID) && validLanguageCode(input.LanguageCode) &&
		validRegionCode(input.RegionCode) && validSessionToken(input.SessionToken)
}

func validGetPhotoMedia(input GetPhotoMediaRequest) bool {
	return validPhotoName(input.PhotoName) && input.MaxWidthPx >= 0 && input.MaxWidthPx <= MaximumPhotoDimension &&
		input.MaxHeightPx >= 0 && input.MaxHeightPx <= MaximumPhotoDimension &&
		(input.MaxWidthPx > 0 || input.MaxHeightPx > 0)
}

func resolveFieldMask(operation string, fields []PlaceField, prefix string, includeNextPageToken bool) (string, error) {
	output := make([]string, 0, len(fields)+2)
	seen := make(map[PlaceField]struct{}, len(fields)+1)
	appendField := func(field PlaceField) {
		if _, found := seen[field]; found {
			return
		}
		seen[field] = struct{}{}
		output = append(output, prefix+string(field))
	}
	appendField(FieldID)
	for _, field := range fields {
		if _, allowed := supportedPlaceFields[field]; !allowed {
			return "", invalidArgument(operation, "fields contains a wildcard or unsupported Place field")
		}
		appendField(field)
	}
	if includeNextPageToken {
		output = append(output, "nextPageToken")
	}
	return strings.Join(output, ","), nil
}

func prepareCallOptions(operation string, options []socialhub.CallOption) ([]socialhub.CallOption, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return nil, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" {
		return nil, invalidArgument(operation, "Places API (New) does not document a caller request-ID header")
	}
	if resolved.IdempotencyKey != "" {
		return nil, invalidArgument(operation, "read-only Places API operations do not use idempotency keys")
	}
	if len(resolved.Fields) > 0 {
		return nil, invalidArgument(operation, "use typed PlaceField values; generic field strings are not accepted")
	}
	if resolved.Timeout == 0 {
		return nil, nil
	}
	return []socialhub.CallOption{socialhub.WithCallTimeout(resolved.Timeout)}, nil
}

func validHTTPSURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil && parsed.Fragment == ""
}

func validatePlace(operation string, place Place) error {
	if !validPlaceID(place.ID) {
		return platformContractError(operation, "Google returned a place without a valid id")
	}
	if place.Name != "" && (!validPlaceName(place.Name) || place.Name != "places/"+place.ID) {
		return platformContractError(operation, "Google returned a mismatched Place resource name")
	}
	if place.Location != nil && !validLatLng(*place.Location) || place.Viewport != nil && !validViewport(*place.Viewport) {
		return platformContractError(operation, "Google returned invalid place geometry")
	}
	if place.Rating != nil && (*place.Rating < 1 || *place.Rating > 5 || math.IsNaN(*place.Rating) || math.IsInf(*place.Rating, 0)) {
		return platformContractError(operation, "Google returned an invalid place rating")
	}
	if place.UserRatingCount != nil && *place.UserRatingCount < 0 {
		return platformContractError(operation, "Google returned a negative user rating count")
	}
	for _, photo := range place.Photos {
		if !validPhotoName(photo.Name) || placeIDFromPhotoName(photo.Name) != place.ID || photo.WidthPx < 0 || photo.HeightPx < 0 {
			return platformContractError(operation, "Google returned invalid or mismatched photo metadata")
		}
	}
	return nil
}

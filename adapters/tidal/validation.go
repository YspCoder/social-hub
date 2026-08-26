package tidal

import (
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

const (
	maxCredentialLength = 16_384
	maxIDLength         = 4_096
	maxCursorLength     = 4_096
	maxIncludePaths     = 10
	maxFilterValues     = 20
)

var (
	searchIncludeRoots = stringSet("albums", "artists", "tracks")
	artistIncludeRoots = stringSet("albums", "profileArt", "roles", "similarArtists", "tracks")
	albumIncludeRoots  = stringSet("artists", "coverArt", "genres", "items", "similarAlbums")
	trackIncludeRoots  = stringSet("albums", "artists", "credits", "genres", "lyrics", "similarTracks", "suggestedTracks")
)

func stringSet(values ...string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}

func validOpaque(value string, maximum int) bool {
	if value == "" || len(value) > maximum || value != strings.TrimSpace(value) || !utf8.ValidString(value) {
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

func validateSearchRequest(input SearchRequest) error {
	const operation = "search"
	if !validLength(input.Query, 1, 256) || input.Query != strings.TrimSpace(input.Query) {
		return invalidArgument(operation, "query must contain 1 to 256 characters without surrounding whitespace")
	}
	if input.ExplicitFilter != "" && input.ExplicitFilter != ExplicitInclude && input.ExplicitFilter != ExplicitExclude {
		return invalidArgument(operation, "explicit filter must be INCLUDE or EXCLUDE")
	}
	return validateCommon(operation, input.CountryCode, input.Include, searchIncludeRoots)
}

func validateResourceRequest(operation string, input ResourceRequest, roots map[string]struct{}) error {
	return validateCommon(operation, input.CountryCode, input.Include, roots)
}

func validateListArtistsRequest(input ListArtistsRequest) error {
	const operation = "list_artists"
	if (len(input.IDs) == 0) == (len(input.Handles) == 0) {
		return invalidArgument(operation, "supply exactly one of ids or handles")
	}
	if err := validateValues(operation, "ids", input.IDs, maxFilterValues, maxIDLength); err != nil {
		return err
	}
	if err := validateValues(operation, "handles", input.Handles, maxFilterValues, 256); err != nil {
		return err
	}
	return validateCommon(operation, input.CountryCode, input.Include, artistIncludeRoots)
}

func validateListAlbumsRequest(input ListAlbumsRequest) error {
	const operation = "list_albums"
	if err := validateValues(operation, "ids", input.IDs, maxFilterValues, maxIDLength); err != nil {
		return err
	}
	if err := validateValues(operation, "barcode ids", input.BarcodeIDs, maxFilterValues, 256); err != nil {
		return err
	}
	if !validOptionalOpaque(input.Cursor, maxCursorLength) {
		return invalidArgument(operation, "cursor is invalid")
	}
	if !validSort(input.Sort) {
		return invalidArgument(operation, "sort is invalid")
	}
	return validateCommon(operation, input.CountryCode, input.Include, albumIncludeRoots)
}

func validateListTracksRequest(input ListTracksRequest) error {
	const operation = "list_tracks"
	if err := validateValues(operation, "ids", input.IDs, maxFilterValues, maxIDLength); err != nil {
		return err
	}
	if err := validateValues(operation, "isrcs", input.ISRCs, maxFilterValues, 256); err != nil {
		return err
	}
	if !validOptionalOpaque(input.Cursor, maxCursorLength) {
		return invalidArgument(operation, "cursor is invalid")
	}
	if len(input.ISRCs) > 1 && input.Cursor != "" {
		return invalidArgument(operation, "cursor cannot be used with multiple ISRC values")
	}
	if !validSort(input.Sort) {
		return invalidArgument(operation, "sort is invalid")
	}
	return validateCommon(operation, input.CountryCode, input.Include, trackIncludeRoots)
}

func validateCommon(operation, countryCode string, include []string, roots map[string]struct{}) error {
	if countryCode != "" && !validCountryCode(countryCode) {
		return invalidArgument(operation, "country code must be an ISO 3166-1 alpha-2 value")
	}
	return validateIncludes(operation, include, roots)
}

func validateValues(operation, name string, values []string, maximumCount, maximumLength int) error {
	if len(values) > maximumCount {
		return invalidArgument(operation, name+" exceeds the documented maximum")
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validOpaque(value, maximumLength) {
			return invalidArgument(operation, name+" contains an invalid value")
		}
		if _, exists := seen[value]; exists {
			return invalidArgument(operation, name+" contains a duplicate value")
		}
		seen[value] = struct{}{}
	}
	return nil
}

func validateIncludes(operation string, include []string, roots map[string]struct{}) error {
	if len(include) > maxIncludePaths {
		return invalidArgument(operation, "include names more than 10 relationships")
	}
	values := make(map[string]struct{}, len(include))
	for _, value := range include {
		if !validOpaque(value, 512) || !validIncludeSegment(value) {
			return invalidArgument(operation, "include contains an invalid relationship")
		}
		if _, allowed := roots[value]; !allowed {
			return invalidArgument(operation, "include contains an unsupported relationship")
		}
		if _, exists := values[value]; exists {
			return invalidArgument(operation, "include contains a duplicate relationship")
		}
		values[value] = struct{}{}
	}
	return nil
}

func validIncludeSegment(value string) bool {
	parts := strings.Split(value, ":")
	if len(parts) > 2 {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
		for _, character := range part {
			if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
				character >= '0' && character <= '9' || character == '_' || character == '-' {
				continue
			}
			return false
		}
	}
	return true
}

func validCountryCode(value string) bool {
	if len(value) != 2 {
		return false
	}
	for _, character := range value {
		if character < 'A' || character > 'Z' && (character < 'a' || character > 'z') {
			return false
		}
	}
	return true
}

func validSort(value Sort) bool {
	switch value {
	case "", SortCreatedAtAscending, SortCreatedAtDescending, SortTitleAscending, SortTitleDescending:
		return true
	default:
		return false
	}
}

func validLength(value string, minimum, maximum int) bool {
	if !utf8.ValidString(value) {
		return false
	}
	length := utf8.RuneCountInString(value)
	return length >= minimum && length <= maximum
}

func prepareCallOptions(operation string, options []socialhub.CallOption) (socialhub.CallOptions, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return socialhub.CallOptions{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" || resolved.IdempotencyKey != "" || len(resolved.Fields) != 0 {
		return socialhub.CallOptions{}, invalidArgument(operation, "only per-call timeouts are supported by TIDAL catalog reads")
	}
	if resolved.Timeout < 0 {
		return socialhub.CallOptions{}, invalidArgument(operation, "timeout must not be negative")
	}
	return resolved, nil
}

func addCommonQuery(query url.Values, countryCode string, include []string) {
	if countryCode != "" {
		query.Set("countryCode", strings.ToUpper(countryCode))
	}
	if len(include) != 0 {
		query.Set("include", strings.Join(include, ","))
	}
}

func addArrayQuery(query url.Values, name string, values []string) {
	for _, value := range values {
		query.Add(name, value)
	}
}

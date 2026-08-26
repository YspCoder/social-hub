package amap

import (
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

func validReference(value string) bool {
	return validOpaque(value, 4096)
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

func validOptionalText(value string, maximumBytes int) bool {
	return value == "" || validOpaque(value, maximumBytes)
}

func validKeyword(value string) bool {
	return value == "" || validOpaque(value, 320) && utf8.RuneCountInString(value) <= 80
}

func validLanguage(value Language) bool {
	return value == "" || value == LanguageChinese || value == LanguageEnglish
}

func validTypeCodes(values []string) bool {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if len(value) != 6 {
			return false
		}
		for _, character := range value {
			if character < '0' || character > '9' {
				return false
			}
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validShowFields(values []ShowField) bool {
	seen := make(map[ShowField]struct{}, len(values))
	for _, value := range values {
		switch value {
		case ShowChildren, ShowBusiness, ShowIndoor, ShowNavi, ShowPhotos:
		default:
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func effectivePageSize(value int) int {
	if value == 0 {
		return 10
	}
	return value
}

func effectivePageNumber(value int) int {
	if value == 0 {
		return 1
	}
	return value
}

func validPage(pageSize, pageNumber int) bool {
	if pageSize < 0 || pageSize > MaximumPageSize || pageNumber < 0 {
		return false
	}
	effectiveSize := effectivePageSize(pageSize)
	effectiveNumber := effectivePageNumber(pageNumber)
	return effectiveNumber > 0 && effectiveNumber <= MaximumSearchWindow/effectiveSize
}

func validateSearchCommon(operation, keywords, region string, typeCodes []string, language Language, cityLimit *bool, showFields []ShowField, pageSize, pageNumber int) error {
	if !validKeyword(keywords) {
		return invalidArgument(operation, "keywords must be one valid value of at most 80 characters")
	}
	if !validTypeCodes(typeCodes) {
		return invalidArgument(operation, "type_codes must contain unique six-digit POI typecodes")
	}
	if !validOptionalText(region, 256) {
		return invalidArgument(operation, "region is invalid")
	}
	if cityLimit != nil && *cityLimit && region == "" {
		return invalidArgument(operation, "city_limit=true requires region")
	}
	if !validLanguage(language) {
		return invalidArgument(operation, "language must be zh or en")
	}
	if !validShowFields(showFields) {
		return invalidArgument(operation, "show_fields contains an unsupported or duplicate field group")
	}
	if !validPage(pageSize, pageNumber) {
		return invalidArgument(operation, "page_size must be 1..25 and the requested page must stay within the first 200 results")
	}
	return nil
}

func validateTextSearch(input TextSearchRequest) error {
	const operation = "search_text"
	if input.Keywords == "" && len(input.TypeCodes) == 0 {
		return invalidArgument(operation, "keywords or type_codes is required")
	}
	return validateSearchCommon(
		operation, input.Keywords, input.Region, input.TypeCodes, input.Language,
		input.CityLimit, input.ShowFields, input.PageSize, input.PageNumber,
	)
}

func validateAroundSearch(input AroundSearchRequest) error {
	const operation = "search_around"
	if !validCoordinate(input.Location) {
		return invalidArgument(operation, "location must be a valid GCJ-02 longitude/latitude pair")
	}
	if input.Radius < 0 || input.Radius > MaximumAroundRadiusMeters {
		return invalidArgument(operation, "radius must be between 0 and 50000 meters")
	}
	switch input.Sort {
	case "", AroundSortDistance, AroundSortWeight:
	default:
		return invalidArgument(operation, "sort must be distance or weight")
	}
	return validateSearchCommon(
		operation, input.Keywords, input.Region, input.TypeCodes, input.Language,
		input.CityLimit, input.ShowFields, input.PageSize, input.PageNumber,
	)
}

func validateDetail(input DetailRequest) error {
	const operation = "get_details"
	if len(input.IDs) == 0 || len(input.IDs) > MaximumDetailIDs {
		return invalidArgument(operation, "ids must contain between 1 and 10 POI IDs")
	}
	seen := make(map[string]struct{}, len(input.IDs))
	for _, id := range input.IDs {
		if !validOpaque(id, 128) || strings.ContainsAny(id, "|,/?#%\\") {
			return invalidArgument(operation, "ids contains an invalid POI ID")
		}
		if _, exists := seen[id]; exists {
			return invalidArgument(operation, "ids contains a duplicate POI ID")
		}
		seen[id] = struct{}{}
	}
	if !validLanguage(input.Language) {
		return invalidArgument(operation, "language must be zh or en")
	}
	if !validShowFields(input.ShowFields) {
		return invalidArgument(operation, "show_fields contains an unsupported or duplicate field group")
	}
	return nil
}

func prepareCallOptions(operation string, options []socialhub.CallOption) ([]socialhub.CallOption, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return nil, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" {
		return nil, invalidArgument(operation, "Amap Place Search v5 does not document a caller request-ID parameter")
	}
	if resolved.IdempotencyKey != "" {
		return nil, invalidArgument(operation, "read-only place operations do not use idempotency keys")
	}
	if len(resolved.Fields) > 0 {
		return nil, invalidArgument(operation, "use the typed show_fields request field")
	}
	if resolved.Timeout == 0 {
		return nil, nil
	}
	return []socialhub.CallOption{socialhub.WithCallTimeout(resolved.Timeout)}, nil
}

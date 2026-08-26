package ebaybrowse

import (
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

func validEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == "" && !strings.HasSuffix(value, "/")
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

func validMarketplaceID(value string) bool {
	if !strings.HasPrefix(value, "EBAY_") || len(value) > 32 {
		return false
	}
	for _, character := range value[len("EBAY_"):] {
		if character != '_' && (character < 'A' || character > 'Z') {
			return false
		}
	}
	return len(value) > len("EBAY_")
}

func validLanguageTag(value string) bool {
	if value == "" || len(value) > 35 || value[0] == '-' || value[len(value)-1] == '-' {
		return false
	}
	for index := range value {
		character := value[index]
		if character != '-' && (character < '0' || character > '9') &&
			(character < 'A' || character > 'Z') && (character < 'a' || character > 'z') {
			return false
		}
	}
	return !strings.Contains(value, "--")
}

func validPathSegment(value string, maximum int) bool {
	if !validOpaque(value, maximum) {
		return false
	}
	return !strings.ContainsAny(value, "/\\?#")
}

func validDigits(value string, maximum int) bool {
	if value == "" || len(value) > maximum {
		return false
	}
	for index := range value {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	return true
}

func validSearch(input SearchItemsRequest) bool {
	if input.Query == "" && input.GTIN == "" && input.EPID == "" && input.CategoryID == "" {
		return false
	}
	if input.Query != "" {
		if !validOpaque(input.Query, 400) || utf8.RuneCountInString(input.Query) > 100 || strings.Contains(input.Query, "*") ||
			input.GTIN != "" || input.EPID != "" {
			return false
		}
	}
	if !validOptionalOpaque(input.GTIN, 64) || !validOptionalOpaque(input.EPID, 128) ||
		(input.CategoryID != "" && !validDigits(input.CategoryID, 32)) {
		return false
	}
	if !validOptionalOpaque(input.Filter, 4096) || !validOptionalOpaque(input.AspectFilter, 4096) ||
		!validOptionalOpaque(input.CompatibilityFilter, 4096) {
		return false
	}
	if input.AspectFilter != "" && input.CategoryID == "" {
		return false
	}
	if input.CompatibilityFilter != "" && (input.Query == "" || input.CategoryID == "") {
		return false
	}
	if !validSearchFieldGroups(input.FieldGroups) || !validSearchSort(input.Sort) {
		return false
	}
	limit := input.Limit
	if limit == 0 {
		limit = 50
	}
	return limit >= 1 && limit <= 200 && input.Offset >= 0 && input.Offset <= 9999 && input.Offset%limit == 0
}

func validSearchSort(value SearchSort) bool {
	switch value {
	case "", SearchSortPrice, SearchSortPriceDescending, SearchSortDistance, SearchSortNewlyListed, SearchSortEndingSoonest:
		return true
	default:
		return false
	}
}

func validSearchFieldGroups(values []SearchFieldGroup) bool {
	seen := make(map[SearchFieldGroup]struct{}, len(values))
	for _, value := range values {
		switch value {
		case SearchFieldMatchingItems, SearchFieldAspectRefinements, SearchFieldBuyingOptionRefinements,
			SearchFieldCategoryRefinements, SearchFieldConditionRefinements, SearchFieldExtended, SearchFieldFull:
		default:
			return false
		}
		if _, found := seen[value]; found {
			return false
		}
		seen[value] = struct{}{}
	}
	_, full := seen[SearchFieldFull]
	return !full || len(values) == 1
}

func validItemFieldGroups(values []ItemFieldGroup, allowCompact, allowProduct bool) bool {
	seen := make(map[ItemFieldGroup]struct{}, len(values))
	for _, value := range values {
		switch value {
		case ItemFieldAdditionalSellerDetails, ItemFieldCharityDetails:
		case ItemFieldCompact:
			if !allowCompact {
				return false
			}
		case ItemFieldProduct:
			if !allowProduct {
				return false
			}
		default:
			return false
		}
		if _, found := seen[value]; found {
			return false
		}
		seen[value] = struct{}{}
	}
	_, compact := seen[ItemFieldCompact]
	return !compact || len(values) == 1
}

func validRequestContext(value RequestContext, affiliateCampaignID string) bool {
	if value.MarketplaceID != "" && !validMarketplaceID(value.MarketplaceID) {
		return false
	}
	if value.AcceptLanguage != "" && !validLanguageTag(value.AcceptLanguage) {
		return false
	}
	if !validOptionalOpaque(value.AffiliateReferenceID, 256) ||
		(value.AffiliateReferenceID != "" && affiliateCampaignID == "") {
		return false
	}
	location := value.DeliveryCountry != "" || value.DeliveryPostalCode != ""
	if location && (!validCountryCode(value.DeliveryCountry) || !validOpaque(value.DeliveryPostalCode, 32)) {
		return false
	}
	return true
}

func validCountryCode(value string) bool {
	return len(value) == 2 && value[0] >= 'A' && value[0] <= 'Z' && value[1] >= 'A' && value[1] <= 'Z'
}

func validQuantity(value int) bool { return value >= 0 }

func validateSearchResponse(operation string, response SearchPage, expectedLimit, expectedOffset int) error {
	if response.Limit != expectedLimit || response.Offset != expectedOffset || response.Total < 0 || len(response.Items) > expectedLimit {
		return platformContractError(operation, "eBay returned inconsistent search pagination metadata")
	}
	if !validResponseURL(response.Next) || !validResponseURL(response.Previous) {
		return platformContractError(operation, "eBay returned an invalid search pagination URL")
	}
	seen := make(map[string]struct{}, len(response.Items))
	for _, item := range response.Items {
		if !validPathSegment(item.ItemID, 512) ||
			(item.ListingMarketplaceID != "" && !validMarketplaceID(item.ListingMarketplaceID)) {
			return platformContractError(operation, "eBay returned an invalid item summary identifier")
		}
		if _, duplicate := seen[item.ItemID]; duplicate {
			return platformContractError(operation, "eBay returned a duplicate item summary ID")
		}
		seen[item.ItemID] = struct{}{}
	}
	return nil
}

func validateItemResponse(operation string, item Item, expectedItemID, expectedLegacyItemID string) error {
	if !validPathSegment(item.ItemID, 512) ||
		(expectedItemID != "" && item.ItemID != expectedItemID) ||
		(expectedLegacyItemID != "" && item.LegacyItemID != expectedLegacyItemID) ||
		(item.ListingMarketplaceID != "" && !validMarketplaceID(item.ListingMarketplaceID)) {
		return platformContractError(operation, "eBay returned an invalid or mismatched item identifier")
	}
	return nil
}

func validateItemGroupResponse(operation string, response ItemGroup) error {
	if len(response.Items) == 0 {
		return platformContractError(operation, "eBay returned an empty item group")
	}
	items := make(map[string]struct{}, len(response.Items))
	for _, item := range response.Items {
		if err := validateItemResponse(operation, item, "", ""); err != nil {
			return err
		}
		if _, duplicate := items[item.ItemID]; duplicate {
			return platformContractError(operation, "eBay returned a duplicate item-group item ID")
		}
		items[item.ItemID] = struct{}{}
	}
	described := make(map[string]struct{})
	for _, description := range response.CommonDescriptions {
		if len(description.ItemIDs) == 0 {
			return platformContractError(operation, "eBay returned a common description without item IDs")
		}
		for _, itemID := range description.ItemIDs {
			if _, exists := items[itemID]; !exists {
				return platformContractError(operation, "eBay returned a common description for an unknown item ID")
			}
			if _, duplicate := described[itemID]; duplicate {
				return platformContractError(operation, "eBay returned duplicate common-description item IDs")
			}
			described[itemID] = struct{}{}
		}
	}
	return nil
}

func validResponseURL(value string) bool {
	if value == "" {
		return true
	}
	if !validOpaque(value, 32_768) {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && parsed.Fragment == ""
}

func prepareCallOptions(operation string, options []socialhub.CallOption) (socialhub.CallOptions, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return socialhub.CallOptions{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.IdempotencyKey != "" {
		return socialhub.CallOptions{}, invalidArgument(operation, "read-only eBay Browse workflows do not define idempotency keys")
	}
	if len(resolved.Fields) > 0 {
		return socialhub.CallOptions{}, invalidArgument(operation, "field selection must use the typed eBay fieldgroups request field")
	}
	if resolved.RequestID != "" && !validOpaque(resolved.RequestID, 256) {
		return socialhub.CallOptions{}, invalidArgument(operation, "request ID is invalid")
	}
	return resolved, nil
}

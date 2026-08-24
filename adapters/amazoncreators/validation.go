package amazoncreators

import (
	"math"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

var marketplaceHosts = map[string]struct{}{
	"www.amazon.ae": {}, "www.amazon.ca": {}, "www.amazon.co.jp": {}, "www.amazon.co.uk": {},
	"www.amazon.com": {}, "www.amazon.com.au": {}, "www.amazon.com.be": {}, "www.amazon.com.br": {},
	"www.amazon.com.mx": {}, "www.amazon.com.tr": {}, "www.amazon.de": {}, "www.amazon.eg": {},
	"www.amazon.es": {}, "www.amazon.fr": {}, "www.amazon.in": {}, "www.amazon.ie": {}, "www.amazon.it": {},
	"www.amazon.nl": {}, "www.amazon.pl": {}, "www.amazon.sa": {}, "www.amazon.se": {},
	"www.amazon.sg": {},
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

func validMarketplace(value string) bool {
	_, found := marketplaceHosts[value]
	return found
}

func validCredentialVersion(value string) bool {
	return value == "3.1" || value == "3.2" || value == "3.3"
}

func tokenEndpoint(version string) string {
	switch version {
	case "3.1":
		return "https://api.amazon.com/auth/o2/token"
	case "3.2":
		return "https://api.amazon.co.uk/auth/o2/token"
	case "3.3":
		return "https://api.amazon.co.jp/auth/o2/token"
	default:
		return ""
	}
}

func validASIN(value string) bool {
	if len(value) != 10 {
		return false
	}
	if value[0] >= '0' && value[0] <= '9' {
		for index := 0; index < 9; index++ {
			if value[index] < '0' || value[index] > '9' {
				return false
			}
		}
		return value[9] == 'X' || value[9] >= '0' && value[9] <= '9'
	}
	if value[0] < 'A' || value[0] > 'Z' {
		return false
	}
	for index := 1; index < len(value); index++ {
		if (value[index] < '0' || value[index] > '9') && (value[index] < 'A' || value[index] > 'Z') {
			return false
		}
	}
	return true
}

func validBrowseNodeID(value string) bool {
	parsed, err := strconv.ParseUint(value, 10, 63)
	return err == nil && parsed > 0
}

func validLanguageTag(value string) bool {
	parts := strings.Split(value, "_")
	if len(parts) != 2 || len(parts[0]) < 2 || len(parts[0]) > 3 || len(parts[1]) != 2 {
		return false
	}
	for _, character := range parts[0] {
		if character < 'a' || character > 'z' {
			return false
		}
	}
	for _, character := range parts[1] {
		if character < 'A' || character > 'Z' {
			return false
		}
	}
	return true
}

func validCurrency(value string) bool {
	return value == "" || (len(value) == 3 && value[0] >= 'A' && value[0] <= 'Z' &&
		value[1] >= 'A' && value[1] <= 'Z' && value[2] >= 'A' && value[2] <= 'Z')
}

func validLanguages(values []string) bool {
	return len(values) <= 1 && (len(values) == 0 || validLanguageTag(values[0]))
}

func validProperties(values map[string]string) bool {
	if len(values) > 64 {
		return false
	}
	for key, value := range values {
		if !validOpaque(key, 128) || !validOpaque(value, 1024) {
			return false
		}
	}
	return true
}

func validSearch(input SearchItemsRequest) bool {
	terms := []string{input.Keywords, input.Actor, input.Artist, input.Author, input.Brand, input.Title}
	foundTerm := false
	for _, value := range terms {
		if value != "" {
			foundTerm = true
		}
		if !validOptionalOpaque(value, 1000) {
			return false
		}
	}
	if !foundTerm || !validOptionalOpaque(input.SearchIndex, 1000) ||
		(input.BrowseNodeID != "" && !validBrowseNodeID(input.BrowseNodeID)) ||
		!validCurrency(input.CurrencyOfPreference) || !validLanguages(input.LanguagesOfPreference) ||
		!validProperties(input.Properties) || !validResources(input.Resources, operationSearchItems) {
		return false
	}
	itemCount := input.ItemCount
	if itemCount == 0 {
		itemCount = 10
	}
	itemPage := input.ItemPage
	if itemPage == 0 {
		itemPage = 1
	}
	if itemCount < 1 || itemCount > 10 || itemPage < 1 || itemPage > 10 ||
		input.MinPrice < 0 || input.MaxPrice < 0 || (input.MinPrice > 0 && input.MaxPrice > 0 && input.MaxPrice < input.MinPrice) ||
		math.IsNaN(input.MinReviewsRating) || math.IsInf(input.MinReviewsRating, 0) || input.MinReviewsRating < 0 || input.MinReviewsRating > 4 ||
		input.MinSavingPercent < 0 || input.MinSavingPercent > 99 {
		return false
	}
	if input.Availability != "" && input.Availability != AvailabilityAvailable && input.Availability != AvailabilityIncludeOutOfStock {
		return false
	}
	if !validCondition(input.Condition) || !validSort(input.SortBy) {
		return false
	}
	seen := make(map[DeliveryFlag]struct{}, len(input.DeliveryFlags))
	for _, value := range input.DeliveryFlags {
		switch value {
		case DeliveryAmazonGlobal, DeliveryFreeShipping, DeliveryFulfilledByAmazon, DeliveryPrime:
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

func validGetItems(input GetItemsRequest) bool {
	return validASINs(input.ItemIDs) && validCondition(input.Condition) && validCurrency(input.CurrencyOfPreference) &&
		validLanguages(input.LanguagesOfPreference) && validProperties(input.Properties) &&
		validResources(input.Resources, operationGetItems)
}

func validGetVariations(input GetVariationsRequest) bool {
	count := input.VariationCount
	if count == 0 {
		count = 10
	}
	page := input.VariationPage
	if page == 0 {
		page = 1
	}
	return validASIN(input.ASIN) && count >= 1 && count <= 10 && page >= 1 &&
		validCondition(input.Condition) && validCurrency(input.CurrencyOfPreference) &&
		validLanguages(input.LanguagesOfPreference) && validProperties(input.Properties) &&
		validResources(input.Resources, operationGetVariations)
}

func validGetBrowseNodes(input GetBrowseNodesRequest) bool {
	if len(input.BrowseNodeIDs) < 1 || len(input.BrowseNodeIDs) > 10 || !validLanguages(input.LanguagesOfPreference) ||
		!validResources(input.Resources, operationGetBrowseNodes) {
		return false
	}
	seen := make(map[string]struct{}, len(input.BrowseNodeIDs))
	for _, value := range input.BrowseNodeIDs {
		if !validBrowseNodeID(value) {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validASINs(values []string) bool {
	if len(values) < 1 || len(values) > 10 {
		return false
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validASIN(value) {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validCondition(value Condition) bool {
	return value == "" || value == ConditionAny || value == ConditionNew
}

func validSort(value SortBy) bool {
	switch value {
	case "", SortAverageCustomerReviews, SortFeatured, SortNewestArrivals, SortPriceHighToLow, SortPriceLowToHigh, SortRelevance:
		return true
	default:
		return false
	}
}

func validResources(values []Resource, operation catalogOperation) bool {
	seen := make(map[Resource]struct{}, len(values))
	for _, value := range values {
		if !resourceAllowed(value, operation) {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func resourceAllowed(value Resource, operation catalogOperation) bool {
	if operation == operationGetBrowseNodes {
		return value == ResourceBrowseNodesAncestor || value == ResourceBrowseNodesChildren
	}
	if value == ResourceSearchRefinements {
		return operation == operationSearchItems
	}
	if value == ResourceVariationSummaryHighestPrice || value == ResourceVariationSummaryLowestPrice ||
		value == ResourceVariationSummaryDimension {
		return operation == operationGetVariations
	}
	for _, shared := range sharedItemResources {
		if value == shared {
			return true
		}
	}
	return false
}

func prepareCallOptions(operation string, options []socialhub.CallOption) (socialhub.CallOptions, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return socialhub.CallOptions{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.IdempotencyKey != "" {
		return socialhub.CallOptions{}, invalidArgument(operation, "read-only Creators API Catalog workflows do not define idempotency keys")
	}
	if len(resolved.Fields) > 0 {
		return socialhub.CallOptions{}, invalidArgument(operation, "field selection must use the typed Amazon resources request field")
	}
	if resolved.RequestID != "" && !validOpaque(resolved.RequestID, 256) {
		return socialhub.CallOptions{}, invalidArgument(operation, "request ID is invalid")
	}
	return resolved, nil
}

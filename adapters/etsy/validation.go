package etsy

import (
	"math"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

var oauthScopes = map[string]struct{}{
	"address_r": {}, "address_w": {}, "billing_r": {}, "cart_r": {}, "cart_w": {},
	"email_r": {}, "favorites_r": {}, "favorites_w": {}, "feedback_r": {}, "listings_d": {},
	"listings_r": {}, "listings_w": {}, "profile_r": {}, "profile_w": {}, "recommend_r": {},
	"recommend_w": {}, "shops_r": {}, "shops_w": {}, "transactions_r": {}, "transactions_w": {},
}

func validEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == "" && !strings.HasSuffix(value, "/")
}

func validCallbackURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil && parsed.Fragment == ""
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

func validOptionalText(value string, maximum int) bool {
	return value == "" || validOpaque(value, maximum)
}

func validOAuthScope(value string) bool {
	_, found := oauthScopes[value]
	return found
}

func validScopeSet(scopes []string) bool {
	if len(scopes) == 0 {
		return false
	}
	seen := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		if !validOAuthScope(scope) {
			return false
		}
		if _, found := seen[scope]; found {
			return false
		}
		seen[scope] = struct{}{}
	}
	return true
}

func validLanguageTag(value string) bool {
	if value == "" {
		return true
	}
	if len(value) > 64 || value[0] == '-' || value[len(value)-1] == '-' {
		return false
	}
	for _, character := range value {
		if character != '-' && (character < 'A' || character > 'Z') && (character < 'a' || character > 'z') && (character < '0' || character > '9') {
			return false
		}
	}
	return true
}

func validExactDecimal(value ExactDecimal, allowZero bool) bool {
	text := string(value)
	if text == "" || len(text) > 128 || len(text) > 1 && text[0] == '0' && text[1] != '.' {
		return false
	}
	dot, nonZero := false, false
	for index, character := range text {
		if character == '.' && !dot && index > 0 && index < len(text)-1 {
			dot = true
			continue
		}
		if character < '0' || character > '9' {
			return false
		}
		if character != '0' {
			nonZero = true
		}
	}
	return allowZero || nonZero
}

func validListingState(value ListingState) bool {
	switch value {
	case "", ListingActive, ListingInactive, ListingSoldOut, ListingDraft, ListingExpired:
		return true
	default:
		return false
	}
}

func validListingType(value ListingType) bool {
	return value == "" || value == ListingPhysical || value == ListingDownload || value == ListingBoth
}

func validWhoMade(value WhoMade) bool {
	return value == MadeByMe || value == MadeByOther || value == MadeByCollective
}

func validWhenMade(value WhenMade) bool {
	switch value {
	case MadeToOrder, Made2020s, Made2010s, Made2007, MadeBefore2007, Made2000s, Made1990s,
		Made1980s, Made1970s, Made1960s, Made1950s, Made1940s, Made1930s, Made1920s,
		Made1910s, Made1900s, Made1800s, Made1700s, MadeBefore1700:
		return true
	default:
		return false
	}
}

func validWeightUnit(value WeightUnit) bool {
	return value == "" || value == WeightOunce || value == WeightPound || value == WeightGram || value == WeightKilogram
}

func validDimensionUnit(value DimensionUnit) bool {
	switch value {
	case "", DimensionInch, DimensionFoot, DimensionMillimeter, DimensionCentimeter, DimensionMeter, DimensionYard, DimensionInches:
		return true
	default:
		return false
	}
}

func validSortField(value ListingSortField) bool {
	return value == "" || value == SortCreated || value == SortPrice || value == SortUpdated || value == SortScore
}

func validSortOrder(value SortOrder) bool {
	return value == "" || value == SortAscending || value == SortDescending
}

func validIncludes(values []ListingInclude, allowed map[ListingInclude]struct{}) bool {
	seen := make(map[ListingInclude]struct{}, len(values))
	for _, value := range values {
		if _, found := allowed[value]; !found {
			return false
		}
		if _, found := seen[value]; found {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validGetListing(input GetListingRequest) bool {
	allowed := map[ListingInclude]struct{}{
		IncludeImages: {}, IncludeShop: {}, IncludeUser: {}, IncludeTranslations: {},
		IncludeVideos: {}, IncludePersonalization: {}, IncludeBuyerPrice: {},
	}
	return validIncludes(input.Includes, allowed) && validLanguageTag(input.Language)
}

func validListShopListings(input ListShopListingsRequest) bool {
	allowed := map[ListingInclude]struct{}{
		IncludeShipping: {}, IncludeImages: {}, IncludeShop: {}, IncludeUser: {}, IncludeTranslations: {},
		IncludeInventory: {}, IncludeVideos: {}, IncludePersonalization: {}, IncludeBuyerPrice: {},
	}
	return validListingState(input.State) && (input.Limit == 0 || input.Limit >= 1 && input.Limit <= 100) &&
		input.Offset >= 0 && validSortField(input.SortOn) && validSortOrder(input.SortOrder) &&
		validIncludes(input.Includes, allowed)
}

func validPositivePointer(value *int64, allowZero bool) bool {
	return value == nil || *value > 0 || allowZero && *value == 0
}

func validFinitePositivePointer(value *float64) bool {
	return value == nil || *value > 0 && !math.IsNaN(*value) && !math.IsInf(*value, 0)
}

func validTextSlice(values []string) bool {
	for _, value := range values {
		if !validOpaque(value, maxRequestBytes) {
			return false
		}
	}
	return true
}

func validUniquePositiveIDs(values []int64) bool {
	seen := make(map[int64]struct{}, len(values))
	for _, value := range values {
		if value <= 0 {
			return false
		}
		if _, found := seen[value]; found {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validMeasurements(input CreateDraftListingRequest) bool {
	hasWeight, hasWeightUnit := input.ItemWeight != nil, input.ItemWeightUnit != ""
	hasDimensions := input.ItemLength != nil || input.ItemWidth != nil || input.ItemHeight != nil
	hasDimensionUnit := input.ItemDimensionsUnit != ""
	return hasWeight == hasWeightUnit && hasDimensions == hasDimensionUnit
}

func validProcessingRange(minimum, maximum *int64) bool {
	return minimum == nil || maximum == nil || *minimum <= *maximum
}

func validCreateDraftListing(input CreateDraftListingRequest) bool {
	return input.Quantity > 0 && validOpaque(input.Title, maxRequestBytes) &&
		validOpaque(input.Description, maxRequestBytes) && validExactDecimal(input.Price, false) &&
		validWhoMade(input.WhoMade) && validWhenMade(input.WhenMade) && input.TaxonomyID > 0 &&
		validPositivePointer(input.ShippingProfileID, false) && validPositivePointer(input.ReturnPolicyID, false) &&
		validPositivePointer(input.ShopSectionID, false) && validPositivePointer(input.ProcessingMin, true) &&
		validPositivePointer(input.ProcessingMax, true) && validPositivePointer(input.ReadinessStateID, false) &&
		validTextSlice(input.Materials) && validTextSlice(input.Tags) && validTextSlice(input.Styles) && len(input.Styles) <= 2 &&
		validFinitePositivePointer(input.ItemWeight) && validFinitePositivePointer(input.ItemLength) &&
		validFinitePositivePointer(input.ItemWidth) && validFinitePositivePointer(input.ItemHeight) &&
		validWeightUnit(input.ItemWeightUnit) && validDimensionUnit(input.ItemDimensionsUnit) &&
		validUniquePositiveIDs(input.ProductionPartnerIDs) && validUniquePositiveIDs(input.ImageIDs) && len(input.ImageIDs) <= 20 &&
		validListingType(input.Type) && validMeasurements(input) && validProcessingRange(input.ProcessingMin, input.ProcessingMax)
}

func validUploadListingImage(input UploadListingImageRequest) bool {
	if input.ListingImageID < 0 {
		return false
	}
	hasImage := input.Image != nil
	hasExisting := input.ListingImageID > 0
	return hasImage != hasExisting && (!hasImage || validOpaque(input.FileName, 255)) &&
		validPositivePointer(input.Rank, true) && (input.AltText == nil || validOptionalText(*input.AltText, 500))
}

func validInventoryProperty(input InventoryPropertyInput) bool {
	if input.PropertyID <= 0 || !validPositivePointer(input.ScaleID, false) ||
		!validOptionalText(input.PropertyName, 4096) || !validUniquePositiveIDs(input.ValueIDs) || !validTextSlice(input.Values) {
		return false
	}
	for _, value := range input.Values {
		if strings.ContainsAny(value, "()") {
			return false
		}
	}
	return true
}

func validInventoryProduct(input InventoryProductInput) bool {
	if !validOptionalText(input.SKU, 4096) || len(input.Offerings) == 0 {
		return false
	}
	propertyIDs := make(map[int64]struct{}, len(input.PropertyValues))
	for _, property := range input.PropertyValues {
		if !validInventoryProperty(property) {
			return false
		}
		if _, found := propertyIDs[property.PropertyID]; found {
			return false
		}
		propertyIDs[property.PropertyID] = struct{}{}
	}
	for _, offering := range input.Offerings {
		if !validExactDecimal(offering.Price, false) || offering.Quantity < 0 || !validPositivePointer(offering.ReadinessStateID, false) {
			return false
		}
	}
	return true
}

func validUpdateInventory(input UpdateListingInventoryRequest) bool {
	if len(input.Products) == 0 || input.MaxVariationsSupported != "" && input.MaxVariationsSupported != "2" && input.MaxVariationsSupported != "3" {
		return false
	}
	for _, product := range input.Products {
		if !validInventoryProduct(product) {
			return false
		}
	}
	return validUniquePositiveIDs(input.PriceOnProperty) && validUniquePositiveIDs(input.QuantityOnProperty) &&
		validUniquePositiveIDs(input.SKUOnProperty) && validUniquePositiveIDs(input.ReadinessStateOnProperty)
}

func validListingsResponse(value ListingsResponse, shopID int64, offset, limit int) bool {
	if value.Count < 0 || value.Results == nil {
		return false
	}
	effectiveLimit := limit
	if effectiveLimit == 0 {
		effectiveLimit = 25
	}
	resultCount := int64(len(value.Results))
	if len(value.Results) > effectiveLimit || resultCount > value.Count ||
		resultCount > 0 && int64(offset) > value.Count-resultCount {
		return false
	}
	seen := make(map[int64]struct{}, len(value.Results))
	for _, listing := range value.Results {
		if listing.ListingID <= 0 || listing.ShopID != shopID {
			return false
		}
		if _, found := seen[listing.ListingID]; found {
			return false
		}
		seen[listing.ListingID] = struct{}{}
	}
	return true
}

func validListingImagesResponse(value ListingImagesResponse, listingID int64) bool {
	if value.Count < 0 || value.Results == nil || value.Count != int64(len(value.Results)) {
		return false
	}
	seen := make(map[int64]struct{}, len(value.Results))
	for _, image := range value.Results {
		if image.ListingID != listingID || image.ListingImageID <= 0 {
			return false
		}
		if _, found := seen[image.ListingImageID]; found {
			return false
		}
		seen[image.ListingImageID] = struct{}{}
	}
	return true
}

func validMoney(value Money) bool {
	if value.Amount < 0 || value.Divisor <= 0 || len(value.CurrencyCode) != 3 {
		return false
	}
	for _, character := range value.CurrencyCode {
		if character < 'A' || character > 'Z' {
			return false
		}
	}
	return true
}

func validListingInventoryResponse(value ListingInventory, listingID int64, requireListing bool) bool {
	if len(value.Products) == 0 || requireListing && value.Listing == nil ||
		value.Listing != nil && value.Listing.ListingID != listingID ||
		!validUniquePositiveIDs(value.PriceOnProperty) || !validUniquePositiveIDs(value.QuantityOnProperty) ||
		!validUniquePositiveIDs(value.SKUOnProperty) || !validUniquePositiveIDs(value.ReadinessStateOnProperty) {
		return false
	}
	productIDs := make(map[int64]struct{}, len(value.Products))
	offeringIDs := make(map[int64]struct{})
	for _, product := range value.Products {
		if product.ProductID <= 0 || len(product.Offerings) == 0 {
			return false
		}
		if _, found := productIDs[product.ProductID]; found {
			return false
		}
		productIDs[product.ProductID] = struct{}{}
		propertyIDs := make(map[int64]struct{}, len(product.PropertyValues))
		for _, property := range product.PropertyValues {
			if property.PropertyID <= 0 || !validPositivePointer(property.ScaleID, false) ||
				!validUniquePositiveIDs(property.ValueIDs) {
				return false
			}
			if _, found := propertyIDs[property.PropertyID]; found {
				return false
			}
			propertyIDs[property.PropertyID] = struct{}{}
		}
		for _, offering := range product.Offerings {
			if offering.OfferingID <= 0 || offering.Quantity < 0 || !validMoney(offering.Price) ||
				!validPositivePointer(offering.ReadinessStateID, false) {
				return false
			}
			if _, found := offeringIDs[offering.OfferingID]; found {
				return false
			}
			offeringIDs[offering.OfferingID] = struct{}{}
		}
	}
	return true
}

func prepareCallOptions(operation string, options []socialhub.CallOption) (socialhub.CallOptions, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return socialhub.CallOptions{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.IdempotencyKey != "" {
		return socialhub.CallOptions{}, invalidArgument(operation, "Etsy Open API v3 does not document idempotency keys for these endpoints")
	}
	if len(resolved.Fields) > 0 {
		return socialhub.CallOptions{}, invalidArgument(operation, "these Etsy endpoints do not define field selection")
	}
	if resolved.RequestID != "" && !validOpaque(resolved.RequestID, 256) {
		return socialhub.CallOptions{}, invalidArgument(operation, "request ID is invalid")
	}
	return resolved, nil
}

func formatID(value int64) string { return strconv.FormatInt(value, 10) }

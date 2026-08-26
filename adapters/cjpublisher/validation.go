package cjpublisher

import (
	"math"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

const (
	maximumFilterValues     = 1000
	maximumCommissionWindow = 31 * 24 * time.Hour
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

func validIdentifier(value string) bool {
	return validOpaque(value, 256)
}

func validOptionalIdentifier(value string) bool {
	return value == "" || validIdentifier(value)
}

func validIdentifiers(values []string) bool {
	if len(values) > maximumFilterValues {
		return false
	}
	for _, value := range values {
		if !validIdentifier(value) {
			return false
		}
	}
	return true
}

func validStrings(values []string, maximum int) bool {
	if len(values) > maximumFilterValues {
		return false
	}
	for _, value := range values {
		if !validOpaque(value, maximum) {
			return false
		}
	}
	return true
}

func validCountry(value string, optional bool) bool {
	if value == "" {
		return optional
	}
	return len(value) == 2 && value[0] >= 'A' && value[0] <= 'Z' && value[1] >= 'A' && value[1] <= 'Z'
}

func validCountries(values []string) bool {
	if len(values) > maximumFilterValues {
		return false
	}
	for _, value := range values {
		if !validCountry(value, false) {
			return false
		}
	}
	return true
}

func validCurrency(value string) bool {
	if value == "" {
		return true
	}
	return len(value) == 3 && value[0] >= 'A' && value[0] <= 'Z' && value[1] >= 'A' && value[1] <= 'Z' && value[2] >= 'A' && value[2] <= 'Z'
}

func validSearchProductFeeds(input SearchProductFeedsRequest) bool {
	validFeedType := input.FeedType == "" || input.FeedType == FeedShopping || input.FeedType == FeedTravel ||
		input.FeedType == FeedFinance || input.FeedType == FeedAll
	return validFeedType && validIdentifiers(input.PartnerIDs) && validCountry(input.AdvertiserCountry, true) &&
		input.Offset >= 0 && input.Limit >= 0 && input.Limit <= 10_000
}

func validSearchProducts(input SearchProductsRequest, propertyID string) bool {
	if !validIdentifiers(input.AdIDs) || !validStrings(input.Keywords, 1024) || !validIdentifiers(input.PartnerIDs) ||
		!validIdentifiers(input.ExcludePartnerIDs) || !validIdentifiers(input.ProductIDs) || !validIdentifiers(input.ExcludeProductIDs) ||
		!validCountries(input.AdvertiserCountries) || !validCurrency(input.Currency) || !validIdentifiers(input.ItemListIDs) ||
		!validCountries(input.ServiceableAreas) || !validCountries(input.ExcludeServiceableAreas) || input.Offset < 0 ||
		input.Limit < 0 || input.Limit > 10_000 || !validOptionalOpaque(input.Page, 4096) || (input.Page != "" && input.Offset != 0) {
		return false
	}
	if input.PartnerStatus != "" && input.PartnerStatus != PartnerJoined && input.PartnerStatus != PartnerNotJoined {
		return false
	}
	if input.Availability != "" && input.Availability != AvailabilityInStock && input.Availability != AvailabilityOutOfStock &&
		input.Availability != AvailabilityPreorder && input.Availability != AvailabilityBackorder {
		return false
	}
	if input.SortBy != "" && input.SortBy != ProductSortLastUpdated && input.SortBy != ProductSortPrice {
		return false
	}
	if input.SortOrder != "" && input.SortOrder != SortAscending && input.SortOrder != SortDescending {
		return false
	}
	if !validPrice(input.LowPrice) || !validPrice(input.HighPrice) ||
		(input.LowPrice != nil && input.HighPrice != nil && *input.LowPrice > *input.HighPrice) {
		return false
	}
	if input.IncludeLinkCode {
		return validIdentifier(propertyID) && validOptionalIdentifier(input.ShopperID)
	}
	return input.PromotionalPropertyID == "" && input.ShopperID == ""
}

func validPrice(value *float64) bool {
	return value == nil || (!math.IsNaN(*value) && !math.IsInf(*value, 0) && *value >= 0)
}

func validListPublisherCommissions(input ListPublisherCommissionsRequest) bool {
	if !validDateWindow(input.SincePostingDate, input.BeforePostingDate, maximumCommissionWindow) ||
		!validDateWindow(input.SinceEventDate, input.BeforeEventDate, maximumCommissionWindow) ||
		!validDateWindow(input.SinceLockingDate, input.BeforeLockingDate, maximumCommissionWindow) ||
		!validOptionalIdentifier(input.SinceCommissionID) || !validIdentifiers(input.CommissionIDs) ||
		!validIdentifiers(input.AdvertiserIDs) || !validIdentifiers(input.AdIDs) || !validIdentifiers(input.WebsiteIDs) ||
		!validStrings(input.ActionStatuses, 256) || !validStrings(input.ActionTypes, 256) {
		return false
	}
	for _, value := range input.LockingMethods {
		if value != LockingImmediate && value != LockingFixedDate && value != LockingOpenEnded && value != LockingFixedDuration {
			return false
		}
	}
	for _, value := range input.ValidationStatuses {
		if value != ValidationPending && value != ValidationAccepted && value != ValidationDeclined && value != ValidationAutomated {
			return false
		}
	}
	return len(input.LockingMethods) <= maximumFilterValues && len(input.ValidationStatuses) <= maximumFilterValues
}

func validDateWindow(start, end time.Time, maximum time.Duration) bool {
	if start.IsZero() || end.IsZero() {
		return true
	}
	return end.After(start) && (maximum <= 0 || end.Sub(start) <= maximum)
}

func validListProgramTerms(input ListProgramTermsRequest) bool {
	return validOptionalIdentifier(input.AdvertiserID) && validDateWindow(input.ActiveAfter, input.ActiveBefore, 0) &&
		input.Offset >= 0 && input.Limit >= 0 && input.Limit <= 100
}

func validSearchLinks(input SearchLinksRequest, websiteID string) bool {
	if !validIdentifier(websiteID) || !validIdentifiers(input.AdvertiserIDs) ||
		(input.Relationship != "" && input.Relationship != LinkRelationshipJoined && input.Relationship != LinkRelationshipNotJoined) ||
		(len(input.AdvertiserIDs) > 0 && input.Relationship != "") || !validOptionalOpaque(input.Keywords, 4096) ||
		!validStrings(input.Categories, 1024) || !validOptionalOpaque(input.LinkType, 256) ||
		!validOptionalOpaque(input.PromotionType, 256) || input.PageNumber < 0 || input.RecordsPerPage < 0 ||
		!validOptionalOpaque(input.Language, 256) || !validOptionalOpaque(input.EventName, 1024) ||
		!validOptionalIdentifier(input.LinkID) || !validCountry(input.TargetedCountry, true) {
		return false
	}
	if (!input.PromotionStartDate.IsZero() || !input.PromotionEndDate.IsZero() || input.OngoingPromotion) && input.PromotionType == "" {
		return false
	}
	if input.OngoingPromotion && !input.PromotionEndDate.IsZero() {
		return false
	}
	if !input.PromotionStartDate.IsZero() && !input.PromotionEndDate.IsZero() && input.PromotionEndDate.Before(input.PromotionStartDate) {
		return false
	}
	return hasLinkFilter(input)
}

func hasLinkFilter(input SearchLinksRequest) bool {
	return len(input.AdvertiserIDs) > 0 || input.Relationship != "" || input.Keywords != "" || len(input.Categories) > 0 ||
		input.LinkType != "" || input.PromotionType != "" || !input.PromotionStartDate.IsZero() || !input.PromotionEndDate.IsZero() ||
		input.OngoingPromotion || input.Language != "" || input.AllowDeepLinking != nil || input.EventName != "" || input.LinkID != "" ||
		!input.LastUpdated.IsZero() || input.CrossDeviceOnly != nil || input.MobileAppDownload != nil || input.MobileOptimized != nil ||
		input.TargetedCountry != ""
}

func validateProductFeedsResponse(operation string, response ProductFeedsResponse, requestedLimit int) error {
	limit := requestedLimit
	if limit == 0 {
		limit = 10_000
	}
	if response.Feeds == nil || len(response.Feeds) > limit {
		return platformContractError(operation, "CJ returned a null or oversized product-feed result list")
	}
	seen := make(map[string]struct{}, len(response.Feeds))
	for _, feed := range response.Feeds {
		if !validIdentifier(feed.AdID) || !validIdentifier(feed.AdvertiserID) {
			return platformContractError(operation, "CJ returned an invalid product-feed identifier")
		}
		if _, duplicate := seen[feed.AdID]; duplicate {
			return platformContractError(operation, "CJ returned a duplicate product-feed ad ID")
		}
		seen[feed.AdID] = struct{}{}
	}
	return nil
}

func validateProductsResponse(operation string, response ProductsResponse, requestedLimit int) error {
	limit := requestedLimit
	if limit == 0 {
		limit = 10_000
	}
	if response.Products == nil || len(response.Products) > limit {
		return platformContractError(operation, "CJ returned a null or oversized product result list")
	}
	if !validOptionalOpaque(response.NextPage, 4096) {
		return platformContractError(operation, "CJ returned an invalid nextPage cursor")
	}
	seen := make(map[string]struct{}, len(response.Products))
	for _, product := range response.Products {
		if !validIdentifier(product.ID) || !validIdentifier(product.AdvertiserID) {
			return platformContractError(operation, "CJ returned an invalid product identifier")
		}
		if _, duplicate := seen[product.ID]; duplicate {
			return platformContractError(operation, "CJ returned a duplicate product ID")
		}
		seen[product.ID] = struct{}{}
	}
	return nil
}

func validateCommissionsResponse(operation string, response CommissionsResponse, publisherID string) error {
	if response.Commissions == nil || len(response.Commissions) > 10_000 {
		return platformContractError(operation, "CJ returned a null or oversized commission record list")
	}
	seen := make(map[string]struct{}, len(response.Commissions))
	for _, commission := range response.Commissions {
		if !validIdentifier(commission.CommissionID) || commission.PublisherID != publisherID {
			return platformContractError(operation, "CJ returned an invalid or mismatched commission identifier")
		}
		if _, duplicate := seen[commission.CommissionID]; duplicate {
			return platformContractError(operation, "CJ returned a duplicate commission ID")
		}
		seen[commission.CommissionID] = struct{}{}
	}
	return nil
}

func validateProgramTermsResponse(
	operation string,
	response ProgramTermsResponse,
	requestedAdvertiserID string,
	limit int,
) error {
	if response.Contracts == nil || len(response.Contracts) > limit {
		return platformContractError(operation, "CJ returned a null or oversized program-terms result list")
	}
	for _, contract := range response.Contracts {
		if !validIdentifier(contract.AdvertiserID) || !validIdentifier(contract.ProgramTerms.ID) {
			return platformContractError(operation, "CJ returned an invalid program-terms identifier")
		}
		if requestedAdvertiserID != "" && contract.AdvertiserID != requestedAdvertiserID {
			return platformContractError(operation, "CJ returned program terms for an unrequested advertiser")
		}
	}
	return nil
}

func validateLinksResponse(operation string, response LinksResponse, request SearchLinksRequest) error {
	if response.RecordsReturned != len(response.Links) || response.TotalMatched < response.RecordsReturned {
		return platformContractError(operation, "CJ returned inconsistent Link Search record metadata")
	}
	expectedPage := request.PageNumber
	if expectedPage == 0 {
		expectedPage = 1
	}
	if response.PageNumber != expectedPage {
		return platformContractError(operation, "CJ returned an unexpected Link Search page number")
	}
	if request.RecordsPerPage > 0 && len(response.Links) > request.RecordsPerPage {
		return platformContractError(operation, "CJ returned more links than requested")
	}
	seen := make(map[string]struct{}, len(response.Links))
	for _, link := range response.Links {
		if !validIdentifier(link.LinkID) || !validIdentifier(link.AdvertiserID) {
			return platformContractError(operation, "CJ returned an invalid Link Search identifier")
		}
		if _, duplicate := seen[link.LinkID]; duplicate {
			return platformContractError(operation, "CJ returned a duplicate Link Search link ID")
		}
		seen[link.LinkID] = struct{}{}
	}
	return nil
}

func resolvePropertyID(request, configured string) string {
	if request != "" {
		return request
	}
	return configured
}

func prepareCallOptions(operation string, options []socialhub.CallOption) (socialhub.CallOptions, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return socialhub.CallOptions{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.IdempotencyKey != "" {
		return socialhub.CallOptions{}, invalidArgument(operation, "these CJ read workflows do not define idempotency keys")
	}
	if len(resolved.Fields) > 0 {
		return socialhub.CallOptions{}, invalidArgument(operation, "the adapter uses a contract-verified GraphQL selection and does not accept dynamic fields")
	}
	if resolved.RequestID != "" && !validOpaque(resolved.RequestID, 256) {
		return socialhub.CallOptions{}, invalidArgument(operation, "request ID is invalid")
	}
	return resolved, nil
}

package sovrncommerce

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
	maximumFilterValues   = 2500
	maximumMerchantWindow = 31 * 24 * time.Hour
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

func validOptionalOpaque(value string, maximum int) bool {
	return value == "" || validOpaque(value, maximum)
}

func validOptionalOpaqueCharacters(value string, maximum int) bool {
	return value == "" || (value == strings.TrimSpace(value) && utf8.ValidString(value) &&
		utf8.RuneCountInString(value) <= maximum && !strings.ContainsFunc(value, unicode.IsControl))
}

func validURL(value string, optional bool) bool {
	if value == "" {
		return optional
	}
	if !validOpaque(value, 8192) || strings.Contains(strings.ToLower(value), "%00") {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil
}

func validBuildAffiliateLink(input BuildAffiliateLinkRequest) bool {
	if !validURL(input.DestinationURL, false) || !validURL(input.FallbackURL, true) ||
		!validOptionalOpaqueCharacters(input.CUID, 2048) {
		return false
	}
	for _, value := range []string{input.UTMSource, input.UTMMedium, input.UTMCampaign, input.UTMTerm, input.UTMContent} {
		if !validOptionalOpaque(value, 2048) {
			return false
		}
	}
	return input.BidFloor == nil || (!math.IsNaN(*input.BidFloor) && !math.IsInf(*input.BidFloor, 0) && *input.BidFloor >= 0)
}

func validProgramType(value ProgramType) bool {
	return value == "" || value == ProgramCPA || value == ProgramCPC
}

func validProduct(value SovrnProduct) bool {
	switch value {
	case "", ProductUnknown, ProductJavaScript, ProductInsert, ProductLink, ProductClickAPI,
		ProductLinkAPI, ProductCouponAPI, ProductProductAPI, ProductPriceComparison,
		ProductInText, ProductShoppingGalleries, ProductFeed:
		return true
	default:
		return false
	}
}

func validDevice(value DeviceType) bool {
	return value == "" || value == DeviceDesktop || value == DeviceMobile || value == DeviceTablet || value == DeviceUnknown
}

func validPositiveIDs(values []int64) bool {
	if len(values) > maximumFilterValues {
		return false
	}
	for _, value := range values {
		if value <= 0 {
			return false
		}
	}
	return true
}

func validFilterStrings(values []string, maximum int) bool {
	if len(values) > maximumFilterValues {
		return false
	}
	for _, value := range values {
		if !validOpaque(value, maximum) || strings.Contains(value, ",") {
			return false
		}
	}
	return true
}

func validListTransactions(input ListTransactionsRequest) bool {
	if input.ClickDate.IsZero() && input.CommissionDate.IsZero() && input.UpdateDate.IsZero() {
		return false
	}
	return validPositiveIDs(input.CampaignIDs) && validPositiveIDs(input.MerchantGroupIDs) && validProgramType(input.ProgramType)
}

func validCountry(value string, optional bool) bool {
	if value == "" {
		return optional
	}
	return len(value) == 2 && value[0] >= 'A' && value[0] <= 'Z' && value[1] >= 'A' && value[1] <= 'Z'
}

func validLowerCountry(value string) bool {
	return len(value) == 2 && value[0] >= 'a' && value[0] <= 'z' && value[1] >= 'a' && value[1] <= 'z'
}

func validUTMFilters(filters UTMFilters) bool {
	for _, values := range [][]string{filters.Source, filters.Medium, filters.Campaign, filters.Term, filters.Content} {
		if !validFilterStrings(values, 2048) {
			return false
		}
	}
	return true
}

func calendarDate(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

func validMerchantPerformance(input GetMerchantPerformanceRequest) bool {
	if input.ClickDateStart.IsZero() || input.ClickDateEnd.IsZero() {
		return false
	}
	start, end := calendarDate(input.ClickDateStart), calendarDate(input.ClickDateEnd)
	if !end.After(start) || end.Sub(start) > maximumMerchantWindow {
		return false
	}
	return validPositiveIDs(input.CampaignIDs) && validFilterStrings(input.SubIDs, 2048) &&
		validPositiveIDs(input.MerchantGroupIDs) && validFilterStrings(input.CUIDs, 2048) &&
		validUTMFilters(input.PageUTM) && validUTMFilters(input.LinkUTM) && validProgramType(input.ProgramType) &&
		validProduct(input.SovrnProduct) && validDevice(input.DeviceType) && validCountry(input.Country, true)
}

func validMerchantCategory(value MerchantCategory) bool {
	switch value {
	case CategoryConsumerElectronics, CategoryAutomotive, CategoryFashion, CategoryHealthBeauty,
		CategoryRealEstate, CategoryArtEntertainment, CategorySportsFitness, CategorySelfHelp,
		CategoryTravel, CategoryFinancialServices, CategoryPets, CategoryMobile, CategoryBooksMagazines,
		CategoryEducation, CategoryOther, CategoryDating, CategoryMusic, CategoryFoodDrink,
		CategoryHomeGarden, CategoryAdultGambling, CategoryCareerEmployment, CategoryCollectibles,
		CategoryOnlineServices, CategoryFamilyBaby, CategoryFirearmsHunting, CategoryGaming,
		CategoryJewelryWatches, CategoryLifestyle, CategoryMotorcycles, CategoryShoppingCoupons,
		CategoryToysHobbies, CategoryCamerasPhoto, CategoryUndefined:
		return true
	default:
		return false
	}
}

func validListApprovedMerchants(input ListApprovedMerchantsRequest) bool {
	if input.CampaignID <= 0 || input.Page < 0 || input.PageSize < 0 || input.PageSize > 2500 ||
		!validFilterStrings(input.Names, 2048) || !validPositiveIDs(input.GroupIDs) ||
		!validFilterStrings(input.Domains, 2048) {
		return false
	}
	filterCount := len(input.Names) + len(input.GroupIDs) + len(input.Categories) + len(input.Geos) + len(input.ProgramTypes) + len(input.Domains)
	if filterCount > maximumFilterValues {
		return false
	}
	for _, category := range input.Categories {
		if !validMerchantCategory(category) {
			return false
		}
	}
	for _, geo := range input.Geos {
		if !validLowerCountry(geo) {
			return false
		}
	}
	for _, programType := range input.ProgramTypes {
		if programType == "" || !validProgramType(programType) || (len(input.Geos) > 0 && programType == ProgramCPC) {
			return false
		}
	}
	return true
}

func prepareCallOptions(operation string, options []socialhub.CallOption) (socialhub.CallOptions, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return socialhub.CallOptions{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" || resolved.IdempotencyKey != "" || len(resolved.Fields) != 0 {
		return socialhub.CallOptions{}, invalidArgument(operation, "only per-call timeouts are supported by Sovrn Commerce")
	}
	return resolved, nil
}

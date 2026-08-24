package ironsourcereporting

import (
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

const maximumQueryBytes = 64 << 10

func validateAdvertiserRequest(input AdvertiserReportRequest) error {
	if !validDateWindow(input.Start, input.End, true) || !uniqueValues(input.Metrics, validAdvertiserMetric) ||
		!uniqueValuesOptional(input.Breakdowns, validAdvertiserBreakdown) || !validAdvertiserFilters(input.Filters) ||
		!validPagination(input.Count, input.Cursor) || !validOrdering(input.Order, input.Direction, validAdvertiserOrder) {
		return invalidArgument("advertiser_report", "date window, metrics, breakdowns, filters, ordering, count, or cursor is invalid")
	}
	return nil
}

func validateCostRequest(input CostReportRequest) error {
	if !validDateWindow(input.Start, input.End, false) || !uniqueValues(input.Metrics, validCostMetric) ||
		!uniqueValuesOptional(input.Breakdowns, validCostBreakdown) || !validReportFilters(input.Filters) ||
		!validPagination(input.Count, input.Cursor) || !validOrdering(input.Order, input.Direction, validCostOrder) {
		return invalidArgument("cost_report", "date window, metrics, breakdowns, filters, ordering, count, or cursor is invalid")
	}
	return nil
}

func validateSKANRequest(input SKANReportRequest) error {
	if !validDateWindow(input.Start, input.End, false) || !uniqueValues(input.Metrics, validSKANMetric) ||
		!uniqueValuesOptional(input.Breakdowns, validSKANBreakdown) || !validSKANFilters(input.Filters) ||
		(input.AdUnit != "" && input.AdUnit != AdUnitRewardedVideo && input.AdUnit != AdUnitInterstitial) ||
		!validPagination(input.Count, input.Cursor) || !validOrdering(input.Order, input.Direction, validSKANOrder) {
		return invalidArgument("skan_report", "date window, metrics, breakdowns, filters, ad unit, ordering, count, or cursor is invalid")
	}
	return nil
}

func validateSKANCVRequest(input SKANConversionValueRequest) error {
	if !validDateWindow(input.Start, input.End, false) || !uniqueValuesOptional(input.Breakdowns, validSKANCVBreakdown) ||
		!validPositiveIDs(input.CampaignIDs) || !validStrings(input.BundleIDs) || !validPagination(input.Count, input.Cursor) ||
		!validOrdering(input.Order, input.Direction, validSKANCVOrder) {
		return invalidArgument("skan_conversion_values", "date window, breakdowns, filters, ordering, count, or cursor is invalid")
	}
	return nil
}

func validDateWindow(start, end Date, maximumThreeMonths bool) bool {
	startTime, startErr := time.Parse("2006-01-02", string(start))
	endTime, endErr := time.Parse("2006-01-02", string(end))
	if startErr != nil || endErr != nil || Date(startTime.Format("2006-01-02")) != start ||
		Date(endTime.Format("2006-01-02")) != end || endTime.Before(startTime) {
		return false
	}
	return !maximumThreeMonths || !endTime.After(startTime.AddDate(0, 3, 0))
}

func validAdvertiserFilters(filters AdvertiserFilters) bool {
	return validReportFilters(filters.ReportFilters) && validPositiveIDs(filters.CreativeIDs) &&
		(filters.DeviceType == "" || filters.DeviceType == DevicePhone || filters.DeviceType == DeviceTablet) &&
		(filters.AdUnit == "" || validAdUnit(filters.AdUnit)) && validPositiveIDs(filters.ExcludeCampaignIDs) &&
		validStrings(filters.ExcludeBundleIDs) && validPositiveIDs(filters.ExcludeCreativeIDs) && validCountries(filters.ExcludeCountries)
}

func validReportFilters(filters ReportFilters) bool {
	return validPositiveIDs(filters.CampaignIDs) && validStrings(filters.BundleIDs) && validCountries(filters.Countries) &&
		(filters.OS == "" || filters.OS == OSIOS || filters.OS == OSAndroid)
}

func validSKANFilters(filters SKANFilters) bool {
	return validPositiveIDs(filters.CampaignIDs) && validStrings(filters.BundleIDs) && validCountries(filters.Countries)
}

func validPositiveIDs(values []int64) bool {
	seen := make(map[int64]struct{}, len(values))
	for _, value := range values {
		if value <= 0 {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validStrings(values []string) bool {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validFilterValue(value) {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validCountries(values []string) bool {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if len(value) != 2 || value[0] < 'A' || value[0] > 'Z' || value[1] < 'A' || value[1] > 'Z' {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validPagination(count int, cursor string) bool {
	return count >= 0 && count <= MaximumCount && (cursor == "" || validOpaque(cursor, 8192))
}

func normalizedCount(count int) int {
	if count == 0 {
		return DefaultCount
	}
	return count
}

func validOrdering(order Order, direction Direction, validOrder func(Order) bool) bool {
	if order == "" {
		return direction == ""
	}
	return validOrder(order) && (direction == "" || direction == DirectionAscending || direction == DirectionDescending)
}

func validAdvertiserMetric(value AdvertiserMetric) bool {
	switch value {
	case AdvertiserMetricImpressions, AdvertiserMetricClicks, AdvertiserMetricCompletions, AdvertiserMetricInstalls, AdvertiserMetricSpend:
		return true
	default:
		return false
	}
}

func validCostMetric(value CostMetric) bool {
	switch value {
	case CostMetricImpressions, CostMetricClicks, CostMetricInstalls, CostMetricBillableSpend, CostMetricECPI:
		return true
	default:
		return false
	}
}

func validSKANMetric(value SKANMetric) bool {
	switch value {
	case SKANMetricImpressions, SKANMetricStoreOpens, SKANMetricInstalls, SKANMetricSpend:
		return true
	default:
		return false
	}
}

func validAdvertiserBreakdown(value AdvertiserBreakdown) bool {
	switch value {
	case AdvertiserBreakdownDay, AdvertiserBreakdownCampaign, AdvertiserBreakdownTitle, AdvertiserBreakdownApplication,
		AdvertiserBreakdownCountry, AdvertiserBreakdownDeviceType, AdvertiserBreakdownCreative,
		AdvertiserBreakdownAdUnit, AdvertiserBreakdownOptimizedEvent:
		return true
	default:
		return false
	}
}

func validCostBreakdown(value CostBreakdown) bool {
	switch value {
	case CostBreakdownDay, CostBreakdownCampaign, CostBreakdownTitle, CostBreakdownOS, CostBreakdownCountry:
		return true
	default:
		return false
	}
}

func validSKANBreakdown(value SKANBreakdown) bool {
	switch value {
	case SKANBreakdownDay, SKANBreakdownCampaign, SKANBreakdownTitle, SKANBreakdownApplication, SKANBreakdownAdUnit, SKANBreakdownCountry:
		return true
	default:
		return false
	}
}

func validSKANCVBreakdown(value SKANCVBreakdown) bool {
	switch value {
	case SKANCVBreakdownDay, SKANCVBreakdownCampaign, SKANCVBreakdownTitle, SKANCVBreakdownApplication:
		return true
	default:
		return false
	}
}

func validAdvertiserOrder(value Order) bool {
	switch value {
	case OrderDay, OrderCampaign, OrderTitle, OrderApplication, OrderCreative, OrderCountry, OrderOS,
		OrderImpressions, OrderClicks, OrderCompletions, OrderInstalls, OrderSpend:
		return true
	default:
		return false
	}
}

func validCostOrder(value Order) bool {
	switch value {
	case OrderDay, OrderCampaign, OrderTitle, OrderOS, OrderCountry, OrderImpressions, OrderInstalls, OrderBillableSpend:
		return true
	default:
		return false
	}
}

func validSKANOrder(value Order) bool {
	switch value {
	case OrderDay, OrderCampaign, OrderTitle, OrderApplication, OrderCountry, OrderImpressions, OrderInstalls, OrderSpend:
		return true
	default:
		return false
	}
}

func validSKANCVOrder(value Order) bool {
	return value == OrderDay || value == OrderCampaign || value == OrderTitle || value == OrderApplication
}

func validAdUnit(value AdUnit) bool {
	return value == AdUnitRewardedVideo || value == AdUnitInterstitial || value == AdUnitOfferWall || value == AdUnitBanner
}

func uniqueValues[T comparable](values []T, valid func(T) bool) bool {
	return len(values) > 0 && uniqueValuesOptional(values, valid)
}

func uniqueValuesOptional[T comparable](values []T, valid func(T) bool) bool {
	seen := make(map[T]struct{}, len(values))
	for _, value := range values {
		if !valid(value) {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validFilterValue(value string) bool {
	return validOpaque(value, 512) && !strings.Contains(value, ",")
}

func validOpaque(value string, maximum int) bool {
	return value != "" && value == strings.TrimSpace(value) && len(value) <= maximum && utf8.ValidString(value) &&
		!strings.ContainsFunc(value, unicode.IsControl)
}

func validQuery(query url.Values) bool { return len(query.Encode()) <= maximumQueryBytes }

func joinIDs(values []int64) string {
	parts := make([]string, len(values))
	for index, value := range values {
		parts[index] = strconv.FormatInt(value, 10)
	}
	return strings.Join(parts, ",")
}

func joinStrings[T ~string](values []T) string {
	parts := make([]string, len(values))
	for index, value := range values {
		parts[index] = string(value)
	}
	return strings.Join(parts, ",")
}

func validateCallOptions(operation string, options []socialhub.CallOption) (socialhub.CallOptions, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return socialhub.CallOptions{}, err
	}
	if resolved.RequestID != "" || resolved.IdempotencyKey != "" || len(resolved.Fields) > 0 {
		return socialhub.CallOptions{}, invalidArgument(operation, "only per-call timeouts are supported for reporting GET requests")
	}
	if resolved.Timeout < 0 {
		return socialhub.CallOptions{}, invalidArgument(operation, "timeout must not be negative")
	}
	return resolved, nil
}

func resolvedCallOptions(resolved socialhub.CallOptions) []socialhub.CallOption {
	if resolved.Timeout == 0 {
		return nil
	}
	return []socialhub.CallOption{socialhub.WithCallTimeout(resolved.Timeout)}
}

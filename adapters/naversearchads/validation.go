package naversearchads

import (
	"bytes"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

func validOpaque(value string, maximum int) bool {
	if value == "" || len(value) > maximum || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validText(value string, maximumRunes int) bool {
	return strings.TrimSpace(value) != "" && strings.TrimSpace(value) == value && utf8.ValidString(value) &&
		utf8.RuneCountInString(value) <= maximumRunes && !strings.ContainsRune(value, '\x00')
}

func validID(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for index := range value {
		character := value[index]
		if character != '-' && character != '_' && (character < '0' || character > '9') &&
			(character < 'A' || character > 'Z') && (character < 'a' || character > 'z') {
			return false
		}
	}
	return true
}

func validUniqueIDs(values []string, minimum, maximum int) bool {
	if len(values) < minimum || len(values) > maximum {
		return false
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validID(value) {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validCampaignType(value CampaignType, optional bool) bool {
	if optional && value == "" {
		return true
	}
	switch value {
	case CampaignWebSite, CampaignShopping, CampaignBrandSearch, CampaignPlace, CampaignPowerContent:
		return true
	default:
		return false
	}
}

func validAdGroupType(value AdGroupType, optional bool) bool {
	if optional && value == "" {
		return true
	}
	switch value {
	case AdGroupWebSite, AdGroupShopping, AdGroupInformation, AdGroupProduct, AdGroupBrandSearch, AdGroupPlace, AdGroupCatalog:
		return true
	default:
		return false
	}
}

func validDeliveryMethod(value DeliveryMethod, optional bool) bool {
	return optional && value == "" || value == DeliveryAccelerated || value == DeliveryStandard
}

func validTrackingMode(value TrackingMode, optional bool) bool {
	return optional && value == "" || value == TrackingDisabled || value == TrackingAuto || value == TrackingPassThrough
}

func validURL(value string, optional bool) bool {
	if optional && value == "" {
		return true
	}
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil
}

func validRawObject(value json.RawMessage, optional bool) bool {
	trimmed := bytes.TrimSpace(value)
	if len(trimmed) == 0 {
		return optional
	}
	return len(trimmed) <= 1<<20 && json.Valid(trimmed) && trimmed[0] == '{' && trimmed[len(trimmed)-1] == '}'
}

func validRawArray(value json.RawMessage, optional bool) bool {
	trimmed := bytes.TrimSpace(value)
	if len(trimmed) == 0 {
		return optional
	}
	return len(trimmed) <= 1<<20 && json.Valid(trimmed) && trimmed[0] == '[' && trimmed[len(trimmed)-1] == ']'
}

func validDate(value Date) bool {
	if len(value) != len("2006-01-02") {
		return false
	}
	_, err := time.Parse("2006-01-02", string(value))
	return err == nil
}

func validReportDate(value StatReportDate) bool {
	if len(value) != len("20060102") {
		return false
	}
	_, err := time.Parse("20060102", string(value))
	return err == nil
}

func validReportResponseDate(value StatReportDate) bool {
	if validReportDate(value) {
		return true
	}
	_, err := time.Parse(time.RFC3339, string(value))
	return err == nil
}

func reportDateMatches(requested, response StatReportDate) bool {
	if !validReportDate(requested) || !validReportResponseDate(response) {
		return false
	}
	if validReportDate(response) {
		return requested == response
	}
	parsed, _ := time.Parse(time.RFC3339, string(response))
	kst := time.FixedZone("KST", 9*60*60)
	return string(requested) == parsed.In(kst).Format("20060102")
}

func validListOptions(input ListOptions) bool {
	return input.Limit >= 0 && input.Limit <= 1000 && (input.Cursor == "" || validID(input.Cursor)) &&
		(input.Direction == "" || input.Direction == DirectionNext || input.Direction == DirectionPrevious) &&
		(input.Cursor != "" || input.Direction != DirectionPrevious)
}

func normalizeList(input ListOptions) ListOptions {
	if input.Limit == 0 {
		input.Limit = 100
	}
	if input.Direction == "" {
		input.Direction = DirectionNext
	}
	return input
}

func listPage[T any](items []T, input ListOptions, id func(T) string) Page[T] {
	page := Page[T]{Items: items}
	if len(items) != input.Limit || len(items) == 0 {
		return page
	}
	if input.Direction == DirectionPrevious {
		page.PreviousCursor = id(items[0])
	} else {
		page.NextCursor = id(items[len(items)-1])
	}
	return page
}

func prepareCallOptions(operation string, options []socialhub.CallOption) ([]socialhub.CallOption, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return nil, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" {
		return nil, invalidArgument(operation, "NAVER Search AD API does not document caller-supplied request IDs")
	}
	if resolved.IdempotencyKey != "" {
		return nil, invalidArgument(operation, "NAVER Search AD API does not document an idempotency-key contract")
	}
	if len(resolved.Fields) > 0 {
		return nil, invalidArgument(operation, "field selection is controlled by typed NAVER update methods")
	}
	if resolved.Timeout == 0 {
		return nil, nil
	}
	return []socialhub.CallOption{socialhub.WithCallTimeout(resolved.Timeout)}, nil
}

func validStatField(value StatField) bool {
	switch value {
	case StatImpressions, StatClicks, StatSpend, StatCTR, StatCPC, StatAverageRank, StatConversions,
		StatRecentAverageRank, StatRecentAverageCPC, StatPCNetworkAverageRank, StatMobileNetworkAverageRank,
		StatConversionRate, StatConversionAmount, StatReturnOnRevenue, StatCostPerConversion,
		StatVideoViews, StatPurchaseConversions, StatPurchaseAmount, StatPurchaseReturn:
		return true
	default:
		return false
	}
}

func validateStatQuery(input StatQuery) error {
	if !validUniqueIDs(input.IDs, 1, 1000) || len(input.Fields) == 0 || len(input.Fields) > 20 {
		return invalidArgument("stats", "1..1000 unique entity IDs and 1..20 fields are required")
	}
	seen := make(map[StatField]struct{}, len(input.Fields))
	for _, field := range input.Fields {
		if !validStatField(field) {
			return invalidArgument("stats", "stat field is invalid")
		}
		if _, exists := seen[field]; exists {
			return invalidArgument("stats", "stat fields must be unique")
		}
		seen[field] = struct{}{}
	}
	if input.TimeRange != nil {
		if !validDate(input.TimeRange.Since) || !validDate(input.TimeRange.Until) || input.TimeRange.Since > input.TimeRange.Until {
			return invalidArgument("stats", "KST time range is invalid")
		}
	}
	if !validDatePreset(input.DatePreset) || input.TimeRange == nil && input.DatePreset == "" {
		return invalidArgument("stats", "a valid time range or date preset is required")
	}
	if input.TimeRange != nil && input.DatePreset != "" {
		return invalidArgument("stats", "time range and date preset are mutually exclusive")
	}
	if input.TimeIncrement != "" && input.TimeIncrement != TimeIncrementDaily && input.TimeIncrement != TimeIncrementAllDays {
		return invalidArgument("stats", "time increment is invalid")
	}
	if len(input.IDs) > 1 && input.TimeIncrement == TimeIncrementDaily {
		return invalidArgument("stats", "bulk Stats support only allDays aggregation")
	}
	if input.Breakdown != "" && input.Breakdown != BreakdownDevice && input.Breakdown != BreakdownDayOfWeek &&
		input.Breakdown != BreakdownHour && input.Breakdown != BreakdownRegion {
		return invalidArgument("stats", "breakdown is invalid")
	}
	return nil
}

func validDatePreset(value DatePreset) bool {
	switch value {
	case "", DateToday, DateYesterday, DateLast7Days, DateLast30Days, DateLastWeek, DateLastMonth, DateLastQuarter:
		return true
	default:
		return false
	}
}

func validStatReportType(value StatReportType) bool {
	switch value {
	case ReportAd, ReportAdDetail, ReportAdConversion, ReportAdConversionDetail,
		ReportAdExtension, ReportAdExtensionConversion, ReportExpandedKeyword,
		ReportShoppingKeywordDetail, ReportShoppingKeywordConversionDetail,
		ReportShoppingBrandProduct, ReportShoppingBrandProductConversion,
		ReportCriterion, ReportCriterionConversion:
		return true
	default:
		return false
	}
}

func formatInt64(value int64) string { return strconv.FormatInt(value, 10) }

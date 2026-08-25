package yandexdirect

import (
	"context"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

func prepareCallOptions(operation string, options []socialhub.CallOption) (socialhub.CallOptions, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return socialhub.CallOptions{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" {
		return socialhub.CallOptions{}, invalidArgument(operation, "Yandex assigns RequestId; caller request IDs are not supported")
	}
	if resolved.IdempotencyKey != "" {
		return socialhub.CallOptions{}, invalidArgument(operation, "Yandex Direct does not document an idempotency-key contract")
	}
	if len(resolved.Fields) > 0 {
		return socialhub.CallOptions{}, invalidArgument(operation, "field selection is fixed by the typed operation")
	}
	if resolved.Timeout < 0 {
		return socialhub.CallOptions{}, invalidArgument(operation, "call timeout must not be negative")
	}
	return resolved, nil
}

func forwardCallOptions(options socialhub.CallOptions) []socialhub.CallOption {
	if options.Timeout == 0 {
		return nil
	}
	return []socialhub.CallOption{socialhub.WithCallTimeout(options.Timeout)}
}

func withCallTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}

func validLanguage(value string) bool {
	switch value {
	case "en", "ru", "tr", "uk":
		return true
	default:
		return false
	}
}

func validLogin(value string) bool {
	return validOpaque(value, 255) && !strings.ContainsAny(value, " \t\r\n")
}

func validOpaque(value string, maximum int) bool {
	if value == "" || len(value) > maximum || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validText(value string, maximum int) bool {
	value = strings.TrimSpace(value)
	if value == "" || utf8.RuneCountInString(value) > maximum || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) && character != '\n' && character != '\t' {
			return false
		}
	}
	return true
}

func validDate(value Date) bool {
	if len(value) != len("2006-01-02") {
		return false
	}
	parsed, err := time.Parse("2006-01-02", string(value))
	return err == nil && parsed.Format("2006-01-02") == string(value)
}

func validOptionalDate(value Date) bool { return value == "" || validDate(value) }

func validPage(value PageRequest) bool {
	return value.Limit >= 0 && value.Limit <= MaximumPageSize && value.Offset >= 0
}

func maximumPageItems(value PageRequest) int {
	if value.Limit > 0 {
		return int(value.Limit)
	}
	return int(MaximumPageSize)
}

func pagePointer(value PageRequest) *PageRequest {
	if value.Limit == 0 && value.Offset == 0 {
		return nil
	}
	copy := value
	return &copy
}

func validIDs(ids []int64, maximum int, allowEmpty bool) bool {
	if (!allowEmpty && len(ids) == 0) || len(ids) > maximum {
		return false
	}
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return false
		}
		if _, exists := seen[id]; exists {
			return false
		}
		seen[id] = struct{}{}
	}
	return true
}

func validStringArray(value *StringArray, maximumItems, maximumLength int, allowEmpty bool) bool {
	if value == nil {
		return true
	}
	if (!allowEmpty && len(value.Items) == 0) || len(value.Items) > maximumItems {
		return false
	}
	for _, item := range value.Items {
		if !validText(item, maximumLength) {
			return false
		}
	}
	return true
}

func validInt64Array(value *Int64Array, maximum int, allowEmpty bool) bool {
	if value == nil {
		return true
	}
	return validIDs(value.Items, maximum, allowEmpty)
}

func validCampaignUpdate(input CampaignUpdate) bool {
	if input.ID <= 0 || input.Name != "" && !validText(input.Name, 255) ||
		!validOptionalDate(input.StartDate) || !validOptionalDate(input.EndDate) ||
		input.StartDate != "" && input.EndDate != "" && input.EndDate < input.StartDate {
		return false
	}
	return input.Name != "" || input.StartDate != "" || input.EndDate != ""
}

func validAttributionModel(value string) bool {
	switch value {
	case "FCCD", "LC", "LSCCD", "AUTO":
		return true
	default:
		return false
	}
}

func validOptionalText(value string, maximum int) bool {
	return value == "" || validText(value, maximum)
}

func validAdGroupAdd(input AdGroupAdd) bool {
	if !validText(input.Name, 255) || !validRegions(input.RegionIDs) || !validOptionalText(input.TrackingParams, 1024) {
		return false
	}
	return validStringArray(input.NegativeKeywords, 1000, 4096, true) &&
		validInt64Array(input.NegativeKeywordSharedSetIDs, 3, true)
}

func validAdGroupUpdate(input AdGroupUpdate) bool {
	if input.ID <= 0 || input.Name != "" && !validText(input.Name, 255) ||
		len(input.RegionIDs) > 0 && !validRegions(input.RegionIDs) || !validOptionalText(input.TrackingParams, 1024) ||
		!validStringArray(input.NegativeKeywords, 1000, 4096, true) || !validInt64Array(input.NegativeKeywordSharedSetIDs, 3, true) {
		return false
	}
	return input.Name != "" || len(input.RegionIDs) > 0 || input.NegativeKeywords != nil ||
		input.NegativeKeywordSharedSetIDs != nil || input.TrackingParams != ""
}

func validRegions(ids []int64) bool {
	if len(ids) == 0 || len(ids) > 10_000 {
		return false
	}
	seen, hasAll := make(map[int64]struct{}, len(ids)), false
	positive := false
	for _, id := range ids {
		if id < 0 {
			return false
		}
		if id == 0 {
			hasAll = true
			positive = true
		} else if id > 0 {
			positive = true
		}
		if _, exists := seen[id]; exists {
			return false
		}
		seen[id] = struct{}{}
	}
	return positive && !(hasAll && len(ids) > 1)
}

func validKeywordAdd(input KeywordAdd) bool {
	if !validText(input.Keyword, 4096) || input.Bid != nil && *input.Bid <= 0 ||
		input.ContextBid != nil && *input.ContextBid <= 0 || !validOptionalText(input.UserParam1, 255) ||
		!validOptionalText(input.UserParam2, 255) || !validOptionalPriority(input.StrategyPriority) ||
		input.AutotargetingSearchBidIsAuto != "" && !validYesNo(input.AutotargetingSearchBidIsAuto) {
		return false
	}
	return true
}

func validKeywordUpdate(input KeywordUpdate) bool {
	if input.ID <= 0 || input.Keyword != nil && !validText(*input.Keyword, 4096) || input.Bid != nil && *input.Bid <= 0 ||
		input.ContextBid != nil && *input.ContextBid <= 0 || input.AutotargetingSearchBidIsAuto != nil && !validYesNo(*input.AutotargetingSearchBidIsAuto) ||
		input.StrategyPriority != nil && !validPriority(*input.StrategyPriority) || input.UserParam1 != nil && !validText(*input.UserParam1, 255) ||
		input.UserParam2 != nil && !validText(*input.UserParam2, 255) {
		return false
	}
	return input.Keyword != nil || input.Bid != nil || input.ContextBid != nil || input.AutotargetingSearchBidIsAuto != nil ||
		input.StrategyPriority != nil || input.UserParam1 != nil || input.UserParam2 != nil
}

func validYesNo(value YesNo) bool { return value == Yes || value == No }

func validOptionalPriority(value StrategyPriority) bool { return value == "" || validPriority(value) }
func validPriority(value StrategyPriority) bool {
	return value == PriorityLow || value == PriorityNormal || value == PriorityHigh
}

func validCampaignSelection(value CampaignSelection) bool {
	return validIDs(value.IDs, MaximumCampaignSelectionIDs, true) &&
		validUniqueValues(value.Types, validCampaignType) && validUniqueValues(value.States, validCampaignState) &&
		validUniqueValues(value.Statuses, validModerationStatus) && validUniqueValues(value.StatusesPayment, validPaymentStatus)
}

func validAdGroupSelection(value AdGroupSelection) bool {
	return validIDs(value.IDs, MaximumPageSizeInt(), true) && validIDs(value.CampaignIDs, MaximumCampaignMutationBatch, true) &&
		validUniqueValues(value.Types, validAdGroupType) && validUniqueValues(value.Statuses, validModerationStatus) &&
		validUniqueValues(value.ServingStatuses, validServingStatus)
}

func validKeywordSelection(value KeywordSelection) bool {
	if !validIDs(value.IDs, MaximumKeywordActionBatch, true) || !validIDs(value.AdGroupIDs, MaximumAdGroupMutationBatch, true) ||
		!validIDs(value.CampaignIDs, MaximumCampaignMutationBatch, true) || !validUniqueValues(value.States, validKeywordState) ||
		!validUniqueValues(value.Statuses, validModerationStatus) || !validUniqueValues(value.ServingStatuses, validServingStatus) {
		return false
	}
	if value.ModifiedSince == "" {
		return true
	}
	parsed, err := time.Parse(time.RFC3339, value.ModifiedSince)
	return err == nil && parsed.Format(time.RFC3339) == value.ModifiedSince
}

func validUniqueValues[T comparable](values []T, valid func(T) bool) bool {
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

func validCampaignType(value CampaignType) bool {
	return value == CampaignText || value == CampaignUnified
}

func validCampaignState(value CampaignState) bool {
	switch value {
	case CampaignStateConverted, CampaignStateArchived, CampaignStateSuspended, CampaignStateEnded,
		CampaignStateOn, CampaignStateOff, CampaignStateUnknown:
		return true
	default:
		return false
	}
}

func validModerationStatus(value ModerationStatus) bool {
	switch value {
	case StatusDraft, StatusModeration, StatusPreaccepted, StatusAccepted, StatusRejected, StatusUnknown:
		return true
	default:
		return false
	}
}

func validPaymentStatus(value PaymentStatus) bool {
	return value == PaymentAllowed || value == PaymentDisallowed
}

func validAdGroupType(value AdGroupType) bool {
	return value == AdGroupText || value == AdGroupUnified
}

func validServingStatus(value ServingStatus) bool {
	return value == ServingEligible || value == ServingRarelyServed
}

func validKeywordState(value KeywordState) bool {
	return value == KeywordStateOff || value == KeywordStateOn || value == KeywordStateSuspended
}

func MaximumPageSizeInt() int { return int(MaximumPageSize) }

func validReport(input ReportDefinition, options ReportOptions) bool {
	if !validText(input.ReportName, 255) || !validReportType(input.ReportType) || !validDateRange(input.DateRangeType) ||
		len(input.FieldNames) == 0 || len(input.FieldNames) > 256 || input.Format != "TSV" || !validYesNo(input.IncludeVAT) ||
		len(input.Goals) > 10 || len(input.AttributionModels) > 4 || len(input.OrderBy) > 256 || options.MaxBytes < 0 ||
		!validProcessingMode(options.ProcessingMode) {
		return false
	}
	criteria := input.SelectionCriteria
	if input.DateRangeType == DateRangeCustom {
		if !validDate(criteria.DateFrom) || !validDate(criteria.DateTo) || criteria.DateTo < criteria.DateFrom {
			return false
		}
	} else if criteria.DateFrom != "" || criteria.DateTo != "" {
		return false
	}
	if input.ReportType == ReportSearchQuery && options.ProcessingMode != ProcessingOffline {
		return false
	}
	seenFields := make(map[ReportField]struct{}, len(input.FieldNames))
	for _, field := range input.FieldNames {
		if !validReportField(field) {
			return false
		}
		seenFields[field] = struct{}{}
	}
	seenFilters := make(map[ReportField]struct{}, len(criteria.Filter))
	for _, filter := range criteria.Filter {
		if !validReportField(filter.Field) || !validFilterOperator(filter.Operator) || len(filter.Values) == 0 || len(filter.Values) > 10_000 {
			return false
		}
		if _, exists := seenFilters[filter.Field]; exists {
			return false
		}
		seenFilters[filter.Field] = struct{}{}
		for _, value := range filter.Values {
			if !validText(value, 4096) {
				return false
			}
		}
	}
	if input.Page != nil && (input.Page.Limit <= 0 || input.Page.Limit > 1_000_000 || input.Page.Offset < 0) {
		return false
	}
	for _, order := range input.OrderBy {
		if !validReportField(order.Field) || order.SortOrder != "" && order.SortOrder != SortAscending && order.SortOrder != SortDescending {
			return false
		}
	}
	for _, goal := range input.Goals {
		if !validOpaque(goal, 255) {
			return false
		}
	}
	for _, model := range input.AttributionModels {
		if !validAttributionModel(model) {
			return false
		}
	}
	return true
}

func validReportType(value ReportType) bool {
	switch value {
	case ReportAccount, ReportCampaign, ReportAdGroup, ReportAd, ReportCriteria, ReportCustom, ReportReachFrequency, ReportSearchQuery:
		return true
	default:
		return false
	}
}

func validDateRange(value DateRangeType) bool {
	switch value {
	case DateRangeAllTime, DateRangeAuto, DateRangeCustom, DateRangeToday, DateRangeYesterday,
		DateRangeLast7Days, DateRangeLast30Days, DateRangeThisMonth, DateRangeLastMonth:
		return true
	default:
		return false
	}
}

func validProcessingMode(value ProcessingMode) bool {
	return value == ProcessingAuto || value == ProcessingOnline || value == ProcessingOffline
}

func validReportField(value ReportField) bool {
	return validOpaque(string(value), 128) && !strings.ContainsAny(string(value), " \t\r\n")
}

func validFilterOperator(value FilterOperator) bool {
	switch value {
	case FilterEquals, FilterNotEquals, FilterIn, FilterNotIn, FilterLessThan, FilterGreaterThan,
		FilterStartsWithIgnoreCase, FilterNotStartsWithIgnoreCase:
		return true
	default:
		return false
	}
}

func validReportMediaType(value string) bool {
	switch strings.ToLower(value) {
	case "text/tab-separated-values", "text/tsv", "text/plain", "application/octet-stream", "binary/octet-stream":
		return true
	default:
		return false
	}
}

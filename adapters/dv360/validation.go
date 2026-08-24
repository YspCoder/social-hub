package dv360

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

func validEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == "" && !strings.HasSuffix(value, "/")
}

func validCallbackURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Host != "" && parsed.User == nil && parsed.Fragment == "" &&
		(parsed.Scheme == "https" || parsed.Scheme == "http")
}

func validOpaque(value string, maximum int) bool {
	return strings.TrimSpace(value) != "" && value == strings.TrimSpace(value) && len(value) <= maximum &&
		!strings.ContainsAny(value, "\x00\r\n")
}

func validID(value string) bool {
	if value == "" || len(value) > 20 {
		return false
	}
	nonzero := false
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
		nonzero = nonzero || character != '0'
	}
	return nonzero
}

func validDisplayName(value string) bool {
	return strings.TrimSpace(value) != "" && value == strings.TrimSpace(value) && utf8.ValidString(value) &&
		len([]byte(value)) <= 240 && !strings.ContainsAny(value, "\x00\r\n")
}

func validOptionalText(value string, maximumBytes int) bool {
	return utf8.ValidString(value) && len([]byte(value)) <= maximumBytes && !strings.ContainsAny(value, "\x00\r\n")
}

func validPageToken(value string) bool { return value == "" || validOpaque(value, 4096) }

func validFilter(value string) bool {
	if len(value) > 500 || !utf8.ValidString(value) || strings.ContainsAny(value, "\x00\r\n") {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validPage(input ListRequest, maximum int, orderFields map[string]struct{}) bool {
	if input.PageSize < 0 || input.PageSize > maximum || !validPageToken(input.PageToken) || !validFilter(input.Filter) {
		return false
	}
	if input.OrderBy == "" {
		return true
	}
	parts := strings.Fields(input.OrderBy)
	if len(parts) == 0 || len(parts) > 2 {
		return false
	}
	if _, ok := orderFields[parts[0]]; !ok {
		return false
	}
	return len(parts) == 1 || parts[1] == "desc"
}

func validDate(value Date) bool {
	if value.Year < 1 || value.Year >= 2037 || value.Month < 1 || value.Month > 12 || value.Day < 1 || value.Day > 31 {
		return false
	}
	parsed := time.Date(value.Year, time.Month(value.Month), value.Day, 0, 0, 0, 0, time.UTC)
	return parsed.Year() == value.Year && int(parsed.Month()) == value.Month && parsed.Day() == value.Day
}

func validDateRange(value DateRange) bool {
	if !validDate(value.StartDate) || value.EndDate != nil && !validDate(*value.EndDate) {
		return false
	}
	if value.EndDate == nil {
		return true
	}
	start := time.Date(value.StartDate.Year, time.Month(value.StartDate.Month), value.StartDate.Day, 0, 0, 0, 0, time.UTC)
	end := time.Date(value.EndDate.Year, time.Month(value.EndDate.Month), value.EndDate.Day, 0, 0, 0, 0, time.UTC)
	return !end.Before(start)
}

func validPositiveInt64String(value string) bool {
	parsed, err := strconv.ParseInt(value, 10, 64)
	return err == nil && parsed > 0 && value == strconv.FormatInt(parsed, 10)
}

func validNonnegativeInt64String(value string) bool {
	parsed, err := strconv.ParseInt(value, 10, 64)
	return err == nil && parsed >= 0 && value == strconv.FormatInt(parsed, 10)
}

func validPositiveInt64AtMost(value string, maximum int64) bool {
	parsed, err := strconv.ParseInt(value, 10, 64)
	return err == nil && parsed > 0 && parsed <= maximum && value == strconv.FormatInt(parsed, 10)
}

func validReadEntityStatus(value EntityStatus) bool {
	return value == EntityStatusActive || value == EntityStatusPaused || value == EntityStatusArchived ||
		value == EntityStatusDraft || value == EntityStatusScheduledForDeletion
}

func validUpdateEntityStatus(value EntityStatus) bool {
	return value == EntityStatusActive || value == EntityStatusPaused || value == EntityStatusArchived
}

func validPoliticalStatus(value EUPoliticalAdvertisingStatus) bool {
	return value == ContainsEUPoliticalAdvertising || value == DoesNotContainEUPoliticalAdvertising
}

func validFrequencyCap(value FrequencyCap) bool {
	if value.Unlimited {
		return value.TimeUnit == "" && value.TimeUnitCount == 0 && value.MaxImpressions == 0
	}
	if value.MaxImpressions <= 0 {
		return false
	}
	switch value.TimeUnit {
	case TimeUnitMonths:
		return value.TimeUnitCount == 1
	case TimeUnitWeeks:
		return value.TimeUnitCount >= 1 && value.TimeUnitCount <= 4
	case TimeUnitDays:
		return value.TimeUnitCount >= 1 && value.TimeUnitCount <= 6
	case TimeUnitHours:
		return value.TimeUnitCount >= 1 && value.TimeUnitCount <= 23
	case TimeUnitMinutes:
		return value.TimeUnitCount >= 1 && value.TimeUnitCount <= 59
	default:
		return false
	}
}

func validPerformanceGoal(value PerformanceGoal) bool {
	if utf8.RuneCountInString(value.Value) > 100 || !validOptionalText(value.Value, 400) {
		return false
	}
	amount := validPositiveInt64String(value.AmountMicros) && value.PercentageMicros == "" && value.Value == ""
	percentage := validPositiveInt64String(value.PercentageMicros) && value.AmountMicros == "" && value.Value == ""
	switch value.Type {
	case PerformanceGoalCPM, PerformanceGoalCPC, PerformanceGoalCPA, PerformanceGoalCPIAVC, PerformanceGoalVCPM:
		return amount
	case PerformanceGoalCTR, PerformanceGoalViewability, PerformanceGoalClickCVR, PerformanceGoalImpressionCVR,
		PerformanceGoalVTR, PerformanceGoalAudioCompletionRate, PerformanceGoalVideoCompletionRate:
		return percentage
	case PerformanceGoalOther:
		return value.AmountMicros == "" && value.PercentageMicros == ""
	default:
		return false
	}
}

func validCampaignGoal(value CampaignGoal) bool {
	switch value.Type {
	case CampaignGoalAppInstall, CampaignGoalBrandAwareness, CampaignGoalOfflineAction, CampaignGoalOnlineAction:
		switch value.PerformanceGoal.Type {
		case PerformanceGoalCPM, PerformanceGoalCPC, PerformanceGoalCPA, PerformanceGoalCPIAVC,
			PerformanceGoalCTR, PerformanceGoalViewability, PerformanceGoalOther:
			return validPerformanceGoal(value.PerformanceGoal)
		default:
			return false
		}
	default:
		return false
	}
}

func validCampaignFlight(value CampaignFlight) bool {
	return validDateRange(value.PlannedDates) &&
		(value.PlannedSpendAmountMicros == "" || validNonnegativeInt64String(value.PlannedSpendAmountMicros))
}

func validCampaignBudget(value CampaignBudget) bool {
	if value.ID != "" && !validID(value.ID) || !validDisplayName(value.DisplayName) ||
		!validPositiveInt64String(value.BudgetAmountMicros) || !validDateRange(value.DateRange) || value.DateRange.EndDate == nil ||
		(value.BudgetUnit != BudgetUnitCurrency && value.BudgetUnit != BudgetUnitImpressions) {
		return false
	}
	if value.ExternalBudgetSource != "EXTERNAL_BUDGET_SOURCE_NONE" && value.ExternalBudgetSource != "EXTERNAL_BUDGET_SOURCE_MEDIA_OCEAN" {
		return false
	}
	return validOptionalText(value.ExternalBudgetID, 500) && validOptionalText(value.InvoiceGroupingID, 500)
}

func validCampaignBudgets(values []CampaignBudget) bool {
	if len(values) > 1000 {
		return false
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validCampaignBudget(value) {
			return false
		}
		if value.ID != "" {
			if _, exists := seen[value.ID]; exists {
				return false
			}
			seen[value.ID] = struct{}{}
		}
	}
	return true
}

func validPacing(value Pacing) bool {
	if value.Type != PacingAhead && value.Type != PacingASAP && value.Type != PacingEven {
		return false
	}
	if value.Period != PacingPeriodDaily && value.Period != PacingPeriodFlight ||
		value.Type == PacingASAP && value.Period == PacingPeriodFlight {
		return false
	}
	if value.Period == PacingPeriodFlight {
		return value.DailyMaxMicros == "" && value.DailyMaxImpressions == ""
	}
	return validPositiveInt64String(value.DailyMaxMicros) != validPositiveInt64String(value.DailyMaxImpressions)
}

func validInsertionOrderBudget(value InsertionOrderBudget) bool {
	if value.BudgetUnit != BudgetUnitCurrency && value.BudgetUnit != BudgetUnitImpressions || len(value.BudgetSegments) == 0 || len(value.BudgetSegments) > 1000 {
		return false
	}
	if value.AutomationType != "" && value.AutomationType != InsertionOrderAutomationBudget &&
		value.AutomationType != InsertionOrderAutomationNone && value.AutomationType != InsertionOrderAutomationBidBudget {
		return false
	}
	for _, segment := range value.BudgetSegments {
		if !validPositiveInt64String(segment.BudgetAmountMicros) || !validDateRange(segment.DateRange) ||
			segment.DateRange.EndDate == nil || !validOptionalText(segment.Description, 1000) ||
			segment.CampaignBudgetID != "" && !validID(segment.CampaignBudgetID) {
			return false
		}
	}
	return true
}

func validKPI(value KPI) bool {
	if utf8.RuneCountInString(value.Value) > 100 || !validOptionalText(value.Value, 400) || value.AlgorithmID != "" && !validID(value.AlgorithmID) {
		return false
	}
	switch value.Type {
	case KPICPM, KPICPC, KPICPA, KPICPIAVC, KPIVCPM:
		return validPositiveInt64String(value.AmountMicros) && value.PercentageMicros == "" && value.Value == ""
	case KPICTR, KPIViewability, KPIClickCVR, KPIImpressionCVR, KPIVTR, KPIAudioCompletionRate, KPIVideoCompletionRate:
		return validPositiveInt64String(value.PercentageMicros) && value.AmountMicros == "" && value.Value == ""
	case KPICustomValueOverCost:
		return value.AlgorithmID != "" && value.AmountMicros == "" && value.PercentageMicros == "" && value.Value == ""
	case KPIOther:
		return value.AmountMicros == "" && value.PercentageMicros == "" && value.AlgorithmID == ""
	case KPICPE, KPICPV, KPICPCL, KPICPCV, KPITOS10, KPIMaximizePacing:
		return value.AmountMicros == "" && value.PercentageMicros == "" && value.AlgorithmID == "" && value.Value == ""
	default:
		return false
	}
}

func validBiddingGoal(value BiddingPerformanceGoalType) bool {
	switch value {
	case BiddingGoalCPA, BiddingGoalCPC, BiddingGoalViewableCPM, BiddingGoalCustomAlgo,
		BiddingGoalCIVA, BiddingGoalIVOTen, BiddingGoalAVViewed, BiddingGoalReach:
		return true
	default:
		return false
	}
}

func validBiddingStrategy(value BiddingStrategy, insertionOrder bool) bool {
	count := 0
	if value.FixedBid != nil {
		count++
		if insertionOrder {
			if value.FixedBid.BidAmountMicros != "0" {
				return false
			}
		} else if !validPositiveInt64AtMost(value.FixedBid.BidAmountMicros, 1_000_000_000) {
			return false
		}
	}
	if value.PerformanceGoalAuto != nil {
		count++
		goal := value.PerformanceGoalAuto
		if insertionOrder || (goal.Type != BiddingGoalCPA && goal.Type != BiddingGoalCPC &&
			goal.Type != BiddingGoalViewableCPM && goal.Type != BiddingGoalCustomAlgo) ||
			!validPositiveInt64String(goal.AmountMicros) ||
			goal.CustomBiddingAlgorithmID != "" && !validID(goal.CustomBiddingAlgorithmID) ||
			goal.MaxAverageCPMBidAmountMicros != "" && !validPositiveInt64String(goal.MaxAverageCPMBidAmountMicros) {
			return false
		}
	}
	if value.MaximizeSpendAuto != nil {
		count++
		goal := value.MaximizeSpendAuto
		if !validBiddingGoal(goal.Type) || goal.Type == BiddingGoalViewableCPM ||
			goal.CustomBiddingAlgorithmID != "" && !validID(goal.CustomBiddingAlgorithmID) ||
			goal.MaxAverageCPMBidAmountMicros != "" && !validPositiveInt64String(goal.MaxAverageCPMBidAmountMicros) {
			return false
		}
	}
	return count == 1
}

func validOptimizationObjective(value OptimizationObjective) bool {
	return value == OptimizationConversion || value == OptimizationClick || value == OptimizationBrandAwareness ||
		value == OptimizationCustom || value == OptimizationNoObjective
}

func validInsertionOrderType(value InsertionOrderType) bool {
	return value == "" || value == InsertionOrderRTB || value == InsertionOrderOverTheTop
}

func validLineItemType(value LineItemType) bool {
	return value == LineItemDisplayDefault || value == LineItemVideoDefault || value == LineItemAudioDefault
}

func validReadLineItemType(value LineItemType) bool {
	return strings.HasPrefix(string(value), "LINE_ITEM_TYPE_") && validOpaque(string(value), 128)
}

func validPacingForBudget(value Pacing, unit BudgetUnit) bool {
	if !validPacing(value) || value.Period != PacingPeriodDaily {
		return validPacing(value)
	}
	if unit == BudgetUnitCurrency {
		return validPositiveInt64String(value.DailyMaxMicros) && value.DailyMaxImpressions == ""
	}
	if unit == BudgetUnitImpressions {
		return validPositiveInt64String(value.DailyMaxImpressions) && value.DailyMaxMicros == ""
	}
	return false
}

func validLineItemFlight(value LineItemFlight) bool {
	switch value.Type {
	case LineItemFlightInherited:
		return value.DateRange == nil
	case LineItemFlightCustom:
		return value.DateRange != nil && value.DateRange.EndDate != nil && validDateRange(*value.DateRange)
	default:
		return false
	}
}

func validLineItemBudget(value LineItemBudget) bool {
	switch value.AllocationType {
	case LineItemBudgetFixed:
		return validPositiveInt64String(value.MaxAmount)
	case LineItemBudgetAutomatic:
		return value.MaxAmount == ""
	case LineItemBudgetUnlimited:
		return value.MaxAmount == ""
	default:
		return false
	}
}

func validPartnerRevenue(value PartnerRevenueModel) bool {
	return (value.MarkupType == PartnerRevenueCPM || value.MarkupType == PartnerRevenueMediaCost ||
		value.MarkupType == PartnerRevenueTotalMediaCost) && validNonnegativeInt64String(value.MarkupAmount)
}

func sanitizeCause(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}

package xiaohongshumarketing

import (
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

var chinaTimeZone = time.FixedZone("Asia/Shanghai", 8*60*60)

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

func validOptionalText(value string, maximum int) bool {
	return value == "" || len(value) <= maximum && utf8.ValidString(value)
}

func validName(value string) bool {
	count := utf8.RuneCountInString(value)
	return count >= 1 && count <= 50 && strings.TrimSpace(value) == value && utf8.ValidString(value)
}

func validDate(value Date) bool {
	if len(value) != len("2006-01-02") {
		return false
	}
	_, err := time.Parse("2006-01-02", string(value))
	return err == nil
}

func validDatePair(start, end Date) bool {
	if start == "" && end == "" {
		return true
	}
	return validDate(start) && validDate(end) && start <= end
}

func validIDs(values []uint64, maximum int, required bool) bool {
	if required && len(values) == 0 || len(values) > maximum {
		return false
	}
	seen := make(map[uint64]struct{}, len(values))
	for _, value := range values {
		if value == 0 {
			return false
		}
		if _, found := seen[value]; found {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func normalizePage(operation string, page, size int) (int, int, error) {
	if page == 0 {
		page = 1
	}
	if size == 0 {
		size = 20
	}
	if page < 1 || page > 1_000_000 || size < 1 || size > 100 {
		return 0, 0, invalidArgument(operation, "page must be 1..1000000 and page size must be 1..100")
	}
	return page, size, nil
}

func validateCallOptions(operation string, options []socialhub.CallOption) error {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" && !validOpaque(resolved.RequestID, 256) {
		return invalidArgument(operation, "request ID is invalid")
	}
	if resolved.IdempotencyKey != "" {
		return invalidArgument(operation, "Spotlight does not document an idempotency-key contract")
	}
	if len(resolved.Fields) > 0 {
		return invalidArgument(operation, "field selection is not supported")
	}
	return nil
}

func validateListCampaigns(input ListCampaignsRequest) error {
	const operation = "campaign_list"
	if !validIDs(input.IDs, 20, false) || !validDatePair(input.StartDate, input.EndDate) ||
		!validDatePair(input.UpdateStartDate, input.UpdateEndDate) ||
		input.Status != 0 && (input.Status < 1 || input.Status > 11) {
		return invalidArgument(operation, "campaign IDs, status, or date filters are invalid")
	}
	return nil
}

func validateUpdateCampaign(input UpdateCampaignRequest, now time.Time) error {
	const operation = "campaign_update"
	if input.MarketingTarget != MarketingTargetProductSeeding && input.MarketingTarget != MarketingTargetLeadGeneration {
		return invalidArgument(operation, "the current cascade modify API supports only product-seeding or lead-generation campaigns")
	}
	if input.Name == nil && input.TimeType == nil && input.StartDate == nil && input.EndDate == nil &&
		input.TimePeriodType == nil && input.TimePeriod == nil && input.LimitDayBudget == nil &&
		input.DayBudgetCents == nil && input.SmartSwitch == nil && input.ExploreState == nil &&
		input.ExploreConfig == nil && input.SearchFlag == nil {
		return invalidArgument(operation, "at least one campaign patch field is required")
	}
	if input.Name != nil && !validName(*input.Name) {
		return invalidArgument(operation, "campaign name must contain 1..50 characters")
	}
	if err := validateCampaignDates(input, now); err != nil {
		return err
	}
	if input.TimePeriodType != nil {
		if *input.TimePeriodType != 0 && *input.TimePeriodType != 1 {
			return invalidArgument(operation, "time period type must be 0 or 1")
		}
	}
	if input.TimePeriod != nil && !validTimePeriod(*input.TimePeriod) {
		return invalidArgument(operation, "time period must contain seven 24-character binary bitmaps")
	}
	if input.TimePeriodType != nil && *input.TimePeriodType == 0 && input.TimePeriod != nil && !uniformTimePeriod(*input.TimePeriod, '1') {
		return invalidArgument(operation, "an unrestricted time period may contain only full-day bitmaps")
	}
	if input.LimitDayBudget != nil {
		if *input.LimitDayBudget != 0 && *input.LimitDayBudget != 1 {
			return invalidArgument(operation, "limit_day_budget must be 0 or 1")
		}
		if *input.LimitDayBudget == 0 && input.DayBudgetCents != nil && *input.DayBudgetCents != 0 {
			return invalidArgument(operation, "an unlimited budget cannot include a positive day budget")
		}
	}
	if input.DayBudgetCents != nil && *input.DayBudgetCents < 0 {
		return invalidArgument(operation, "day budget cannot be negative")
	}
	if !validOptionalSwitch(input.SmartSwitch) || !validOptionalSwitch(input.ExploreState) || !validOptionalSwitch(input.SearchFlag) {
		return invalidArgument(operation, "smart, explore, and search switches must be 0 or 1")
	}
	if input.ExploreState != nil && *input.ExploreState == 0 && input.ExploreConfig != nil {
		return invalidArgument(operation, "disabled explore cannot include explore_config")
	}
	if input.ExploreConfig != nil && !validUpdateExploreConfig(*input.ExploreConfig) {
		return invalidArgument(operation, "explore_config contains an invalid patch field")
	}
	return nil
}

func validateCampaignDates(input UpdateCampaignRequest, now time.Time) error {
	const operation = "campaign_update"
	if input.TimeType != nil && *input.TimeType != 0 && *input.TimeType != 1 {
		return invalidArgument(operation, "time type must be 0 or 1")
	}
	today := Date(now.In(chinaTimeZone).Format("2006-01-02"))
	if input.StartDate != nil && (!validDate(*input.StartDate) || *input.StartDate < today) {
		return invalidArgument(operation, "campaign start date must be today or later in Asia/Shanghai")
	}
	if input.EndDate != nil && (!validDate(*input.EndDate) || *input.EndDate < today) {
		return invalidArgument(operation, "campaign end date must be today or later in Asia/Shanghai")
	}
	if input.StartDate != nil && input.EndDate != nil && *input.StartDate > *input.EndDate {
		return invalidArgument(operation, "campaign date range is invalid")
	}
	return nil
}

func validOptionalSwitch(value *int) bool {
	return value == nil || *value == 0 || *value == 1
}

func validTimePeriod(value TimePeriod) bool {
	for _, bitmap := range []string{value.Mon, value.Tues, value.Wed, value.Thur, value.Fri, value.Sat, value.Sun} {
		if len(bitmap) != 24 {
			return false
		}
		for index := range bitmap {
			if bitmap[index] != '0' && bitmap[index] != '1' {
				return false
			}
		}
	}
	return true
}

func uniformTimePeriod(value TimePeriod, expected byte) bool {
	if !validTimePeriod(value) {
		return false
	}
	for _, bitmap := range []string{value.Mon, value.Tues, value.Wed, value.Thur, value.Fri, value.Sat, value.Sun} {
		for index := range bitmap {
			if bitmap[index] != expected {
				return false
			}
		}
	}
	return true
}

func validUpdateExploreConfig(value UpdateExploreConfig) bool {
	if value.DayBudgetCents == nil && value.TimePeriod == nil && value.TimePeriodType == nil &&
		value.StartTimeMS == nil && value.ExpireHours == nil {
		return false
	}
	if value.DayBudgetCents != nil && *value.DayBudgetCents < 0 ||
		value.TimePeriodType != nil && *value.TimePeriodType != 0 && *value.TimePeriodType != 1 ||
		value.StartTimeMS != nil && *value.StartTimeMS <= 0 ||
		value.ExpireHours != nil && (*value.ExpireHours < 1 || *value.ExpireHours > 6) {
		return false
	}
	if value.TimePeriod != nil && !validTimePeriod(*value.TimePeriod) {
		return false
	}
	if value.TimePeriodType != nil && *value.TimePeriodType == 0 && value.TimePeriod != nil {
		return uniformTimePeriod(*value.TimePeriod, '1')
	}
	return true
}

func validateListUnits(input ListUnitsRequest) error {
	const operation = "unit_list"
	if !validIDs(input.IDs, 10, false) || input.Status != 0 && input.Status != 1 && input.Status != 2 ||
		!validOptionalText(input.Name, 200) || !validDatePair(input.StartDate, input.EndDate) ||
		!validDatePair(input.UpdateStartDate, input.UpdateEndDate) {
		return invalidArgument(operation, "unit IDs, status, name, or date filters are invalid")
	}
	return nil
}

func validateSearchCreatives(input SearchCreativesRequest) error {
	const operation = "creative_search"
	if !validIDs(input.IDs, 20, false) || !validCreativeStatus(input.Status) ||
		!validDatePair(input.StartDate, input.EndDate) || !validOptionalText(input.NoteID, 256) {
		return invalidArgument(operation, "creative IDs, status, dates, or note ID are invalid")
	}
	if len(input.IDs) > 0 && (input.CampaignID != 0 || input.UnitID != 0 || input.Status != 0 ||
		input.StartDate != "" || input.EndDate != "" || input.NoteID != "") {
		return invalidArgument(operation, "creative IDs cannot be combined with filters the provider would ignore")
	}
	return nil
}

func validCreativeStatus(value int) bool {
	if value == 0 {
		return true
	}
	switch value {
	case 1, 2, 3, 4, 5, 8, 9, 10, 11, 12, 13, 14, 16:
		return true
	default:
		return false
	}
}

func validResponsePage(pageIndex, requestedPage int, total int64, itemCount, pageSize int) bool {
	return pageIndex == requestedPage && total >= 0 && int64(itemCount) <= total && itemCount <= pageSize
}

func validStatusAction(action StatusAction) bool {
	return action == StatusActionResume || action == StatusActionPause || action == StatusActionDelete
}

package xiaohongshureporting

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

var (
	errInvalidReportEnvelope = errors.New("Spotlight returned an invalid report envelope")
	errInvalidReportRows     = errors.New("Spotlight returned an invalid report row list")
	errInvalidReportRow      = errors.New("Spotlight returned an invalid report row")
	errInvalidReportPage     = errors.New("Spotlight returned invalid report pagination")
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

func validOptionalOpaque(value string, maximum int) bool {
	return value == "" || validOpaque(value, maximum)
}

func validIdentifier(value string, maximum int) bool {
	if value == "" || len(value) > maximum {
		return false
	}
	for index := range value {
		character := value[index]
		if character != '_' && character != '-' && character != '.' &&
			(character < '0' || character > '9') &&
			(character < 'A' || character > 'Z') &&
			(character < 'a' || character > 'z') {
			return false
		}
	}
	return true
}

func validDate(value Date) bool {
	if len(value) != len("2006-01-02") {
		return false
	}
	_, err := time.Parse("2006-01-02", string(value))
	return err == nil
}

func validDateRange(start, end Date) bool {
	return validDate(start) && validDate(end) && start <= end
}

func validTimeUnit(value TimeUnit, allowHour bool) bool {
	return value == "" || value == TimeUnitDay || value == TimeUnitSummary || allowHour && value == TimeUnitHour
}

func validSort(column string, order SortOrder) bool {
	if (column == "") != (order == "") {
		return false
	}
	return column == "" || validIdentifier(column, 128) && (order == SortAscending || order == SortDescending)
}

func validPage(page, size, maximum int) bool {
	return page >= 0 && page <= 1_000_000 && size >= 0 && size <= maximum
}

func validIntList(values []int, maximum int) bool {
	if len(values) > maximum {
		return false
	}
	seen := make(map[int]struct{}, len(values))
	for _, value := range values {
		if value < 0 || value > 1_000_000 {
			return false
		}
		if _, found := seen[value]; found {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validStringList(values []string, maximum, itemMaximum int) bool {
	if len(values) > maximum {
		return false
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validIdentifier(value, itemMaximum) {
			return false
		}
		if _, found := seen[value]; found {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validFilters(filters []FilterClause) bool {
	if len(filters) > 64 {
		return false
	}
	for _, filter := range filters {
		if !validIdentifier(filter.Column, 128) || !validOpaque(filter.Operator, 64) ||
			len(filter.Values) == 0 || len(filter.Values) > 256 {
			return false
		}
		for _, value := range filter.Values {
			if !validOpaque(value, 1024) {
				return false
			}
		}
	}
	return true
}

func validateCallOptions(operation string, options []socialhub.CallOption) error {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" && !validOpaque(resolved.RequestID, 256) {
		return invalidArgument(operation, "request ID is invalid")
	}
	if resolved.IdempotencyKey != "" || len(resolved.Fields) > 0 {
		return invalidArgument(operation, "idempotency keys and field selection are not supported")
	}
	return nil
}

func validateOfflineReport(operation string, input OfflineReportRequest) error {
	if !validDateRange(input.StartDate, input.EndDate) || !validTimeUnit(input.TimeUnit, true) {
		return invalidArgument(operation, "start and end dates or time unit are invalid")
	}
	if !validPage(input.PageNum, input.PageSize, 500) || !validSort(input.SortColumn, input.Sort) {
		return invalidArgument(operation, "page, page size, or sort fields are invalid")
	}
	if input.DataCaliber != DataCaliberBillingTime && input.DataCaliber != DataCaliberConversionTime {
		return invalidArgument(operation, "data caliber must be billing time or conversion time")
	}
	lists := [][]int{
		input.MarketingTarget, input.BiddingStrategy, input.OptimizeTarget, input.Placement,
		input.PromotionTarget, input.Programmatic, input.BuildType, input.DeliveryMode,
	}
	for _, values := range lists {
		if !validIntList(values, 64) {
			return invalidArgument(operation, "one or more report filter lists are invalid")
		}
	}
	if !validStringList(input.SplitColumns, 64, 128) || !validFilters(input.Filters) {
		return invalidArgument(operation, "split columns or filter clauses are invalid")
	}
	return nil
}

func validateOfflineSimpleReport(operation string, input OfflineSimpleReportRequest) error {
	if !validDateRange(input.StartDate, input.EndDate) || !validTimeUnit(input.TimeUnit, false) ||
		!validPage(input.PageNum, input.PageSize, 500) || !validSort(input.SortColumn, input.Sort) {
		return invalidArgument(operation, "date range, time unit, pagination, or sort fields are invalid")
	}
	return nil
}

func validateOfflineSearchWord(operation string, input OfflineSearchWordRequest) error {
	base := OfflineReportRequest{
		StartDate: input.StartDate, EndDate: input.EndDate, TimeUnit: input.TimeUnit,
		MarketingTarget: input.MarketingTarget, BiddingStrategy: input.BiddingStrategy,
		OptimizeTarget: input.OptimizeTarget, Placement: input.Placement,
		PromotionTarget: input.PromotionTarget, Programmatic: input.Programmatic,
		BuildType: input.BuildType, SortColumn: input.SortColumn, Sort: input.Sort,
		PageNum: input.PageNum, PageSize: input.PageSize, DataCaliber: input.DataCaliber,
	}
	return validateOfflineReport(operation, base)
}

func validateRealtimeBase(operation string, start, end Date, page, size int, column string, order SortOrder, hourly bool, clock socialhub.Clock) error {
	if !validDateRange(start, end) || !validPage(page, size, 100) || !validSort(column, order) {
		return invalidArgument(operation, "date range, pagination, or sort fields are invalid")
	}
	if hourly {
		today := Date(clock.Now().In(chinaTimeZone).Format("2006-01-02"))
		if start != today || end != today {
			return invalidArgument(operation, "hourly data is available only when start and end are today in Asia/Shanghai")
		}
	}
	return nil
}

func validateRealtimeCampaign(operation string, input RealtimeCampaignRequest, clock socialhub.Clock) error {
	if err := validateRealtimeBase(operation, input.StartDate, input.EndDate, input.PageNum, input.PageSize, input.SortColumn, input.Sort, input.NeedHourlyData, clock); err != nil {
		return err
	}
	if !validOptionalOpaque(input.Name, 256) || input.ID < 0 || !validTimestampRange(input.CampaignCreateBeginTime, input.CampaignCreateEndTime) ||
		!validStatuses(input.CampaignFilterState, input.CombineAuditStatus) || !validDataCaliber(input.DataCaliber) {
		return invalidArgument(operation, "campaign report filters are invalid")
	}
	lists := [][]int{input.MarketingTargetList, input.PlacementList, input.LimitDayBudgetList, input.OptimizeTargetList,
		input.BuildTypeList, input.BiddingStrategyList, input.ConstraintTypeList, input.PromotionTargetList, input.MigrationStatusList}
	return validateRealtimeLists(operation, lists)
}

func validateRealtimeUnit(operation string, input RealtimeUnitRequest, clock socialhub.Clock) error {
	if err := validateRealtimeBase(operation, input.StartDate, input.EndDate, input.PageNum, input.PageSize, input.SortColumn, input.Sort, input.NeedHourlyData, clock); err != nil {
		return err
	}
	if !validOptionalOpaque(input.Name, 256) || input.ID < 0 || !validTimestampRange(input.UnitCreateBeginTime, input.UnitCreateEndTime) ||
		!validStatuses(input.UnitFilterState, input.CombineAuditStatus) || !validDataCaliber(input.DataCaliber) {
		return invalidArgument(operation, "unit report filters are invalid")
	}
	return validateRealtimeLists(operation, [][]int{
		input.MarketingTargetList, input.PlacementList, input.BiddingStrategyList, input.PromotionTargetList,
	})
}

func validateRealtimeCreative(operation string, input RealtimeCreativeRequest, clock socialhub.Clock) error {
	if err := validateRealtimeBase(operation, input.StartDate, input.EndDate, input.PageNum, input.PageSize, input.SortColumn, input.Sort, input.NeedHourlyData, clock); err != nil {
		return err
	}
	if !validOptionalOpaque(input.Name, 256) || input.ID < 0 || !validTimestampRange(input.CreativityCreateBeginTime, input.CreativityCreateEndTime) ||
		!validStatuses(input.CreativityFilterState, input.ConversionType, input.CreativityAuditState) || !validDataCaliber(input.DataCaliber) {
		return invalidArgument(operation, "creative report filters are invalid")
	}
	return validateRealtimeLists(operation, [][]int{input.PlacementList, input.ProgrammaticList})
}

func validateRealtimeKeyword(operation string, input RealtimeKeywordRequest, clock socialhub.Clock) error {
	if err := validateRealtimeBase(operation, input.StartDate, input.EndDate, input.PageNum, input.PageSize, input.SortColumn, input.Sort, input.NeedHourlyData, clock); err != nil {
		return err
	}
	if !validStatuses(input.KeywordFilterState, input.UseBidStrategy) || !validDataCaliber(input.DataCaliber) ||
		!validOptionalOpaque(input.KeywordName, 256) || !validOptionalOpaque(input.CampaignName, 256) || !validOptionalOpaque(input.UnitName, 256) {
		return invalidArgument(operation, "keyword report filters are invalid")
	}
	return nil
}

func validateRealtimeTarget(operation string, input RealtimeTargetRequest, clock socialhub.Clock) error {
	if err := validateRealtimeBase(operation, input.StartDate, input.EndDate, input.PageNum, input.PageSize, input.SortColumn, input.Sort, input.NeedHourlyData, clock); err != nil {
		return err
	}
	if !validOptionalOpaque(input.Name, 256) {
		return invalidArgument(operation, "target name is invalid")
	}
	return validateRealtimeLists(operation, [][]int{input.MarketingTargetList})
}

func validateRealtimeLists(operation string, lists [][]int) error {
	for _, values := range lists {
		if !validIntList(values, 64) {
			return invalidArgument(operation, "one or more realtime filter lists are invalid")
		}
	}
	return nil
}

func validDataCaliber(value DataCaliber) bool {
	return value == DataCaliberBillingTime || value == DataCaliberConversionTime
}

func validStatuses(values ...int) bool {
	for _, value := range values {
		if value < 0 || value > 1_000_000 {
			return false
		}
	}
	return true
}

func validTimestampRange(start, end string) bool {
	if start == "" && end == "" {
		return true
	}
	if start == "" || end == "" || len(start) != len("2006-01-02 15:04:05") || len(end) != len("2006-01-02 15:04:05") {
		return false
	}
	if _, err := time.Parse("2006-01-02 15:04:05", start); err != nil {
		return false
	}
	if _, err := time.Parse("2006-01-02 15:04:05", end); err != nil {
		return false
	}
	return start <= end
}

func validReportRow(row ReportRow, allowEmpty bool) bool {
	if row == nil || len(row) > 512 || !allowEmpty && len(row) == 0 {
		return false
	}
	for key, value := range row {
		if !validIdentifier(key, 128) || len(value.raw) == 0 || len(value.raw) > maxReportValueBytes || !json.Valid(value.raw) {
			return false
		}
	}
	return true
}

func validBalance(balance AccountBalance) bool {
	values := []ReportValue{
		balance.TotalBalance, balance.CashBalance, balance.ReturnBalance, balance.CreditBalance,
		balance.FreezeBalance, balance.AvailableBalance, balance.TodaySpend, balance.CompensateReturnBalance,
		balance.AccountBudget, balance.LimitDayBudget,
	}
	for _, value := range values {
		if len(value.raw) == 0 || len(value.raw) > 256 || !json.Valid(value.raw) {
			return false
		}
	}
	return true
}

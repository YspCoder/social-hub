package applovinreporting

import (
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

const (
	maximumColumns      = 256
	maximumFilters      = 64
	maximumFilterValues = 100
	maximumSorts        = 32
	maximumHaving       = 32
	maximumValueBytes   = 65_536
)

var (
	numericIDPattern   = regexp.MustCompile(`^[1-9][0-9]{0,63}$`)
	decimalPattern     = regexp.MustCompile(`^-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?$`)
	campaignDimensions = map[CampaignColumn]struct{}{
		"ad": {}, "ad_creative_type": {}, "ad_type": {}, "app_id_external": {}, "application": {},
		"application_is_hidden": {}, "audience_strategy": {}, "bidding_and_billing_method": {}, "bidding_integration": {},
		"campaign": {}, "campaign_ad_type": {}, "campaign_id_external": {}, "campaign_package_name": {}, "campaign_store_id": {},
		"campaign_type": {}, "country": {}, "creative_set": {}, "creative_set_id": {}, "custom_page_id": {}, "day": {},
		"device_type": {}, "external_placement_id": {}, "hour": {}, "optimization_day_target": {}, "package_name": {},
		"placement_type": {}, "platform": {}, "size": {}, "store_id": {}, "target_event": {}, "traffic_source": {}, "zone": {}, "zone_id": {},
	}
)

func validAccountType(value AccountType) bool {
	return value == AccountTypeApp || value == AccountTypeWeb
}

func validNumericID(value string) bool { return numericIDPattern.MatchString(value) }

func validOpaque(value string, maximum int) bool {
	return value != "" && value == strings.TrimSpace(value) && len(value) <= maximum &&
		utf8.ValidString(value) && !strings.ContainsFunc(value, unicode.IsControl)
}

func supportedCallOptions(operation string, options []socialhub.CallOption) (socialhub.CallOptions, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return socialhub.CallOptions{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" {
		return socialhub.CallOptions{}, invalidArgument(operation, "AppLovin Growth Reporting does not document caller-supplied request IDs")
	}
	if resolved.IdempotencyKey != "" {
		return socialhub.CallOptions{}, invalidArgument(operation, "read-only AppLovin reports do not define idempotency keys")
	}
	if len(resolved.Fields) != 0 {
		return socialhub.CallOptions{}, invalidArgument(operation, "use the typed report Columns field for AppLovin field selection")
	}
	return resolved, nil
}

func forwardCallOptions(options socialhub.CallOptions) []socialhub.CallOption {
	if options.Timeout == 0 {
		return nil
	}
	return []socialhub.CallOption{socialhub.WithCallTimeout(options.Timeout)}
}

func validDecimal(value string) bool {
	if len(value) > 128 || !decimalPattern.MatchString(value) {
		return false
	}
	_, ok := new(big.Rat).SetString(value)
	return ok
}

func parseCalendarDate(value string) (time.Time, bool) {
	if len(value) != len("2006-01-02") {
		return time.Time{}, false
	}
	parsed, err := time.Parse("2006-01-02", value)
	return parsed, err == nil
}

func parseReportTime(value ReportTime, allowNow bool, now time.Time) (time.Time, bool) {
	text := string(value)
	if text == string(ReportNow) {
		if !allowNow {
			return time.Time{}, false
		}
		return now.UTC(), true
	}
	if parsed, ok := parseCalendarDate(text); ok {
		return parsed, true
	}
	if text == "" || strings.Trim(text, "0123456789") != "" {
		return time.Time{}, false
	}
	seconds, err := strconv.ParseInt(text, 10, 64)
	if err != nil || seconds <= 0 || seconds > 253402300799 {
		return time.Time{}, false
	}
	return time.Unix(seconds, 0).UTC(), true
}

func validReportWindow(start, end ReportTime, maximumDays int, now time.Time) bool {
	startTime, startOK := parseReportTime(start, false, now)
	endTime, endOK := parseReportTime(end, true, now)
	return startOK && endOK && !endTime.Before(startTime) && endTime.Sub(startTime) <= time.Duration(maximumDays)*24*time.Hour
}

func validDateWindow(start, end Date, maximumDays int) bool {
	startTime, startOK := parseCalendarDate(string(start))
	endTime, endOK := parseCalendarDate(string(end))
	return startOK && endOK && !endTime.Before(startTime) && endTime.Sub(startTime) <= time.Duration(maximumDays)*24*time.Hour
}

func normalizedPagination(value Pagination) (Pagination, bool) {
	if value.Offset < 0 || value.Limit < 0 || value.Limit > MaximumReportLimit {
		return Pagination{}, false
	}
	if value.Limit == 0 {
		value.Limit = DefaultReportLimit
	}
	if value.Offset%value.Limit != 0 {
		return Pagination{}, false
	}
	return value, true
}

func validSortOrder(value SortOrder) bool {
	return value == SortAscending || value == SortDescending
}

func validAttribution(value AttributionMode) bool {
	return value == "" || value == AttributionCohort || value == AttributionRealtime
}

func validFilterValues(values []string) bool {
	if len(values) == 0 || len(values) > maximumFilterValues {
		return false
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validOpaque(value, 4096) || strings.Contains(value, ",") {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validCampaignRequest(input CampaignReportRequest, accountType AccountType, now time.Time) bool {
	columns := campaignColumnSet(accountType, input.Type)
	maximumDays := 45
	if accountType == AccountTypeWeb {
		maximumDays = 90
	}
	if columns == nil || !validReportWindow(input.Start, input.End, maximumDays, now) || !validAttribution(input.Attribution) ||
		!validCampaignColumns(input.Columns, columns) || !validCampaignFilters(input.Filters, columns) || !validCampaignSorts(input.Sorts, columns) ||
		!validHaving(input.Having, columns) || !validCustomPageFilters(input.CustomPageFilters, columns) {
		return false
	}
	_, paginationOK := normalizedPagination(input.Pagination)
	if !paginationOK {
		return false
	}
	selected := toCampaignColumnSet(input.Columns)
	if _, needsCampaign := selected["campaign_bid_goal"]; needsCampaign {
		if _, present := selected[CampaignColumnCampaign]; !present {
			return false
		}
	}
	if _, needsCampaign := selected["campaign_roas_goal"]; needsCampaign {
		if _, present := selected[CampaignColumnCampaign]; !present {
			return false
		}
	}
	return true
}

func validCampaignColumns(values []CampaignColumn, known map[CampaignColumn]struct{}) bool {
	if len(values) == 0 || len(values) > maximumColumns {
		return false
	}
	seen := make(map[CampaignColumn]struct{}, len(values))
	for _, value := range values {
		if _, ok := known[value]; !ok {
			return false
		}
		if _, duplicate := seen[value]; duplicate {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validCampaignFilters(values []CampaignFilter, known map[CampaignColumn]struct{}) bool {
	if len(values) > maximumFilters {
		return false
	}
	seen := make(map[CampaignColumn]struct{}, len(values))
	for _, value := range values {
		if _, ok := known[value.Column]; !ok || !validFilterValues(value.Values) {
			return false
		}
		if _, duplicate := seen[value.Column]; duplicate {
			return false
		}
		seen[value.Column] = struct{}{}
	}
	return true
}

func validCampaignSorts(values []CampaignSort, known map[CampaignColumn]struct{}) bool {
	if len(values) > maximumSorts {
		return false
	}
	seen := make(map[CampaignColumn]struct{}, len(values))
	for _, value := range values {
		if _, ok := known[value.Column]; !ok || !validSortOrder(value.Order) {
			return false
		}
		if _, duplicate := seen[value.Column]; duplicate {
			return false
		}
		seen[value.Column] = struct{}{}
	}
	return true
}

func validHaving(value *Having, known map[CampaignColumn]struct{}) bool {
	if value == nil {
		return true
	}
	combine := value.Combine
	if combine == "" {
		combine = HavingAND
	}
	if combine != HavingAND && combine != HavingOR || len(value.Conditions) == 0 || len(value.Conditions) > maximumHaving {
		return false
	}
	for _, condition := range value.Conditions {
		if _, ok := known[condition.Column]; !ok || !validComparison(condition.Operator) || !validDecimal(condition.Value) {
			return false
		}
		if _, dimension := campaignDimensions[condition.Column]; dimension {
			return false
		}
	}
	return true
}

func validComparison(value ComparisonOperator) bool {
	switch value {
	case CompareGreaterThan, CompareLessThan, CompareGreaterThanOrEqual, CompareLessThanOrEqual, CompareEqual, CompareNotEqual:
		return true
	default:
		return false
	}
}

func validCustomPageFilters(values []CustomPageFilter, known map[CampaignColumn]struct{}) bool {
	if len(values) > 1 {
		return false
	}
	if len(values) > 0 {
		if _, supported := known["custom_page_id"]; !supported {
			return false
		}
	}
	seen := make(map[CustomPageFilter]struct{}, len(values))
	for _, value := range values {
		if value != CustomPageNull && value != CustomPageBlank && value != CustomPageNotNull && value != CustomPageNotBlank {
			return false
		}
		if _, duplicate := seen[value]; duplicate {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validAssetRequest(input AssetReportRequest, accountType AccountType) bool {
	known := assetColumnSet(accountType)
	rangeSelector := input.Range != ""
	dateSelector := input.Start != "" || input.End != ""
	if known == nil || rangeSelector == dateSelector || rangeSelector && input.Range != AssetYesterday && input.Range != AssetLast7Days && input.Range != AssetLastMonth ||
		dateSelector && !validDateWindow(input.Start, input.End, 45) || !validAssetColumns(input.Columns, known) ||
		!validAssetFilters(input.Filters, known) || !validMetricFilters(input.Metrics, known) || !validAssetSorts(input.Sorts, known) {
		return false
	}
	_, paginationOK := normalizedPagination(input.Pagination)
	return paginationOK
}

func validAssetColumns(values []AssetColumn, known map[AssetColumn]struct{}) bool {
	if len(values) == 0 || len(values) > maximumColumns {
		return false
	}
	seen := make(map[AssetColumn]struct{}, len(values))
	for _, value := range values {
		if _, ok := known[value]; !ok {
			return false
		}
		if _, duplicate := seen[value]; duplicate {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validAssetFilters(values []AssetFilter, known map[AssetColumn]struct{}) bool {
	if len(values) > maximumFilters {
		return false
	}
	seen := make(map[AssetColumn]struct{}, len(values))
	for _, value := range values {
		_, available := known[value.Column]
		_, filterable := assetFilterColumnSet[value.Column]
		if !available || !filterable || !validFilterValues(value.Values) {
			return false
		}
		if _, duplicate := seen[value.Column]; duplicate {
			return false
		}
		seen[value.Column] = struct{}{}
	}
	return true
}

func validMetricFilters(values []MetricFilter, known map[AssetColumn]struct{}) bool {
	if len(values) > maximumFilters {
		return false
	}
	seen := make(map[AssetColumn]struct{}, len(values))
	for _, value := range values {
		_, available := known[value.Column]
		_, metric := assetMetricColumnSet[value.Column]
		if !available || !metric || value.GreaterThan == "" && value.LessThan == "" ||
			value.GreaterThan != "" && !validDecimal(value.GreaterThan) || value.LessThan != "" && !validDecimal(value.LessThan) {
			return false
		}
		if value.GreaterThan != "" && value.LessThan != "" {
			lower, _ := new(big.Rat).SetString(value.GreaterThan)
			upper, _ := new(big.Rat).SetString(value.LessThan)
			if lower.Cmp(upper) >= 0 {
				return false
			}
		}
		if _, duplicate := seen[value.Column]; duplicate {
			return false
		}
		seen[value.Column] = struct{}{}
	}
	return true
}

func validAssetSorts(values []AssetSort, known map[AssetColumn]struct{}) bool {
	if len(values) > maximumSorts {
		return false
	}
	seen := make(map[AssetColumn]struct{}, len(values))
	for _, value := range values {
		if _, ok := known[value.Column]; !ok || !validSortOrder(value.Order) {
			return false
		}
		if _, duplicate := seen[value.Column]; duplicate {
			return false
		}
		seen[value.Column] = struct{}{}
	}
	return true
}

func validPlayableRequest(input PlayableReportRequest, accountType AccountType) bool {
	if accountType != AccountTypeApp || !validDateWindow(input.Start, input.End, 45) || !validAttribution(input.Attribution) ||
		!validPlayableColumns(input.Columns) || !validPlayableFilters(input.Filters) || !validPlayableSorts(input.Sorts) {
		return false
	}
	_, paginationOK := normalizedPagination(input.Pagination)
	return paginationOK
}

func validPlayableColumns(values []PlayableColumn) bool {
	if len(values) == 0 || len(values) > maximumColumns {
		return false
	}
	seen := make(map[PlayableColumn]struct{}, len(values))
	for _, value := range values {
		if _, ok := playableColumnSet[value]; !ok {
			return false
		}
		if _, duplicate := seen[value]; duplicate {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validPlayableFilters(values []PlayableFilter) bool {
	if len(values) > maximumFilters {
		return false
	}
	seen := make(map[PlayableColumn]struct{}, len(values))
	for _, value := range values {
		if _, ok := playableColumnSet[value.Column]; !ok || !validFilterValues(value.Values) {
			return false
		}
		if _, duplicate := seen[value.Column]; duplicate {
			return false
		}
		seen[value.Column] = struct{}{}
	}
	return true
}

func validPlayableSorts(values []PlayableSort) bool {
	if len(values) > maximumSorts {
		return false
	}
	seen := make(map[PlayableColumn]struct{}, len(values))
	for _, value := range values {
		if _, ok := playableColumnSet[value.Column]; !ok || !validSortOrder(value.Order) {
			return false
		}
		if _, duplicate := seen[value.Column]; duplicate {
			return false
		}
		seen[value.Column] = struct{}{}
	}
	return true
}

func validRawRows(rows []map[string]ReportValue, expected map[string]struct{}) bool {
	for _, row := range rows {
		if len(row) != len(expected) {
			return false
		}
		for column, value := range row {
			if _, ok := expected[column]; !ok || len(value.Text) > maximumValueBytes {
				return false
			}
		}
	}
	return true
}

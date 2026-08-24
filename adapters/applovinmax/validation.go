package applovinmax

import (
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type orderedSort struct {
	column string
	order  SortOrder
}

type preparedReportQuery struct {
	values url.Values
	sorts  []orderedSort
}

var revenueColumns = map[RevenueColumn]struct{}{
	RevenueAdFormat: {}, RevenueAdUnitWaterfallName: {}, RevenueApplication: {}, RevenueAttempts: {},
	RevenueCountry: {}, RevenueCustomNetworkName: {}, RevenueDay: {}, RevenueDeviceType: {}, RevenueECPM: {},
	RevenueEstimatedRevenue: {}, RevenueFillRate: {}, RevenueHasIDFA: {}, RevenueHour: {}, RevenueImpressions: {},
	RevenueMAXAdUnit: {}, RevenueMAXAdUnitID: {}, RevenueMAXAdUnitTest: {}, RevenueMAXPlacement: {},
	RevenueNetwork: {}, RevenueNetworkPlacement: {}, RevenuePackageName: {}, RevenuePlatform: {}, RevenueRequests: {},
	RevenueResponses: {}, RevenueStoreID: {},
}

var cohortCommonColumns = map[CohortColumn]struct{}{
	CohortApplication: {}, CohortCountry: {}, CohortDayColumn: {}, CohortInstalls: {}, CohortPackageName: {}, CohortPlatform: {},
}

var cohortMetrics = map[CohortKind]map[CohortMetric]struct{}{
	CohortRevenue: {
		CohortAdsPublisherRevenue: {}, CohortAdsRevenuePerInstall: {}, CohortBannerPublisherRevenue: {},
		CohortBannerRevenuePerInstall: {}, CohortIAPPublisherRevenue: {}, CohortIAPRevenuePerInstall: {},
		CohortInterstitialRevenue: {}, CohortInterstitialRPI: {}, CohortMRECPublisherRevenue: {},
		CohortMRECRevenuePerInstall: {}, CohortPublisherRevenue: {}, CohortRewardedPublisherRevenue: {},
		CohortRewardedRevenuePerInstall: {}, CohortRevenuePerInstall: {},
	},
	CohortImpressions: {
		CohortBannerImpressions: {}, CohortBannerImpressionsPerUser: {}, CohortImpressionCount: {},
		CohortImpressionsPerUser: {}, CohortInterstitialImpressions: {}, CohortInterstitialImpsPerUser: {},
		CohortMRECImpressions: {}, CohortMRECImpressionsPerUser: {}, CohortRewardedImpressions: {},
		CohortRewardedImpsPerUser: {}, CohortUserCount: {},
	},
	CohortSessions: {
		CohortDailyUsage: {}, CohortRetention: {}, CohortSessionCount: {}, CohortSessionLength: {}, CohortUserCount: {},
	},
}

var cohortAges = map[CohortAge]struct{}{
	0: {}, 1: {}, 2: {}, 3: {}, 4: {}, 5: {}, 6: {}, 7: {}, 10: {}, 14: {}, 18: {}, 21: {}, 24: {}, 27: {}, 30: {}, 45: {},
}

func supportedCallOptions(operation string, options []socialhub.CallOption) (socialhub.CallOptions, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return socialhub.CallOptions{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" {
		return socialhub.CallOptions{}, invalidArgument(operation, "AppLovin does not document caller-supplied request IDs")
	}
	if resolved.IdempotencyKey != "" {
		return socialhub.CallOptions{}, invalidArgument(operation, "read-only MAX report methods do not define idempotency keys")
	}
	if len(resolved.Fields) > 0 {
		return socialhub.CallOptions{}, invalidArgument(operation, "use the typed report Columns field for MAX field selection")
	}
	return resolved, nil
}

func forwardCallOptions(options socialhub.CallOptions) []socialhub.CallOption {
	if options.Timeout == 0 {
		return nil
	}
	return []socialhub.CallOption{socialhub.WithCallTimeout(options.Timeout)}
}

func validOpaque(value string, maximum int) bool {
	return strings.TrimSpace(value) != "" && value == strings.TrimSpace(value) && len(value) <= maximum &&
		utf8.ValidString(value) && !strings.ContainsFunc(value, unicode.IsControl)
}

func validText(value string, maximum int) bool {
	return validOpaque(value, maximum)
}

func validDate(value Date) bool {
	if value.Year < 1 || value.Year > 9999 || value.Month < time.January || value.Month > time.December || value.Day < 1 || value.Day > 31 {
		return false
	}
	parsed := dateTime(value)
	year, month, day := parsed.Date()
	return year == value.Year && month == value.Month && day == value.Day
}

func dateTime(value Date) time.Time {
	return time.Date(value.Year, value.Month, value.Day, 0, 0, 0, 0, time.UTC)
}

func dateString(value Date) string { return dateTime(value).Format(time.DateOnly) }

func validReportWindow(start, end Date, now time.Time) bool {
	if !validDate(start) || !validDate(end) {
		return false
	}
	startTime, endTime := dateTime(start), dateTime(end)
	today := dateTime(DateFromTime(now))
	return !endTime.Before(startTime) && !endTime.After(today) && !startTime.Before(today.AddDate(0, 0, -44))
}

func validRevenueColumn(value RevenueColumn) bool {
	_, found := revenueColumns[value]
	return found
}

func prepareRevenueRequest(input RevenueReportRequest, maximumLimit int, now time.Time) (preparedReportQuery, error) {
	const operation = "revenue_report"
	if !validReportWindow(input.Start, input.End, now) || len(input.Columns) == 0 || len(input.Columns) > len(revenueColumns) ||
		input.Limit < 0 || input.Limit > maximumLimit || input.Offset < 0 || input.Offset > 1_000_000_000 ||
		len(input.Filters) > 32 || len(input.Sorts) > 32 {
		return preparedReportQuery{}, invalidArgument(operation, "date window, columns, pagination, filters, or sorts are invalid")
	}
	seenColumns := make(map[RevenueColumn]struct{}, len(input.Columns))
	for _, column := range input.Columns {
		if !validRevenueColumn(column) {
			return preparedReportQuery{}, invalidArgument(operation, "report contains an unknown column")
		}
		if _, exists := seenColumns[column]; exists {
			return preparedReportQuery{}, invalidArgument(operation, "report columns must be unique")
		}
		seenColumns[column] = struct{}{}
	}
	if !validRevenueCombinations(seenColumns, input.Start, now) {
		return preparedReportQuery{}, invalidArgument(operation, "report contains an incompatible column combination")
	}
	query := url.Values{
		"start":   {dateString(input.Start)},
		"end":     {dateString(input.End)},
		"columns": {joinRevenueColumns(input.Columns)},
	}
	setPagination(query, input.Limit, input.Offset, input.NotZero)
	seenFilters := make(map[RevenueColumn]struct{}, len(input.Filters))
	for _, filter := range input.Filters {
		if !validRevenueColumn(filter.Column) || !validText(filter.Value, 4096) {
			return preparedReportQuery{}, invalidArgument(operation, "report filter is invalid")
		}
		if _, exists := seenFilters[filter.Column]; exists {
			return preparedReportQuery{}, invalidArgument(operation, "report filters must use unique columns")
		}
		seenFilters[filter.Column] = struct{}{}
		query.Set("filter_"+string(filter.Column), filter.Value)
	}
	ordered, err := prepareRevenueSorts(input.Sorts)
	if err != nil {
		return preparedReportQuery{}, err
	}
	return preparedReportQuery{values: query, sorts: ordered}, nil
}

func validRevenueCombinations(columns map[RevenueColumn]struct{}, start Date, now time.Time) bool {
	has := func(column RevenueColumn) bool { _, found := columns[column]; return found }
	networkBreakdown := has(RevenueNetwork) || has(RevenueNetworkPlacement)
	if has(RevenueRequests) && (networkBreakdown || has(RevenueMAXPlacement)) {
		return false
	}
	for _, metric := range []RevenueColumn{RevenueAttempts, RevenueResponses, RevenueFillRate} {
		if has(metric) && (!networkBreakdown || has(RevenueMAXPlacement)) {
			return false
		}
	}
	if has(RevenueHour) {
		today := dateTime(DateFromTime(now))
		return !dateTime(start).Before(today.AddDate(0, 0, -29))
	}
	return true
}

func prepareRevenueSorts(values []RevenueSort) ([]orderedSort, error) {
	seen := make(map[RevenueColumn]struct{}, len(values))
	result := make([]orderedSort, 0, len(values))
	for _, value := range values {
		if !validRevenueColumn(value.Column) || value.Order != SortAscending && value.Order != SortDescending {
			return nil, invalidArgument("revenue_report", "report sort is invalid")
		}
		if _, exists := seen[value.Column]; exists {
			return nil, invalidArgument("revenue_report", "report sorts must use unique columns")
		}
		seen[value.Column] = struct{}{}
		result = append(result, orderedSort{column: string(value.Column), order: value.Order})
	}
	return result, nil
}

func joinRevenueColumns(values []RevenueColumn) string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = string(value)
	}
	return strings.Join(result, ",")
}

func validCohortColumn(kind CohortKind, column CohortColumn) bool {
	if _, found := cohortCommonColumns[column]; found {
		return true
	}
	metrics, found := cohortMetrics[kind]
	if !found {
		return false
	}
	for metric := range metrics {
		prefix := string(metric) + "_"
		if !strings.HasPrefix(string(column), prefix) {
			continue
		}
		age, err := strconv.Atoi(strings.TrimPrefix(string(column), prefix))
		if err == nil {
			_, valid := cohortAges[CohortAge(age)]
			return valid
		}
	}
	return false
}

func prepareCohortRequest(input CohortReportRequest, maximumLimit int, now time.Time) (preparedReportQuery, error) {
	const operation = "cohort_report"
	if _, found := cohortMetrics[input.Kind]; !found || !validReportWindow(input.Start, input.End, now) ||
		len(input.Columns) == 0 || len(input.Columns) > 64 || input.Limit < 0 || input.Limit > maximumLimit ||
		input.Offset < 0 || input.Offset > 1_000_000_000 || len(input.Filters) > 32 || len(input.Sorts) > 32 {
		return preparedReportQuery{}, invalidArgument(operation, "cohort kind, date window, columns, pagination, filters, or sorts are invalid")
	}
	seenColumns := make(map[CohortColumn]struct{}, len(input.Columns))
	columnNames := make([]string, len(input.Columns))
	for index, column := range input.Columns {
		if !validCohortColumn(input.Kind, column) {
			return preparedReportQuery{}, invalidArgument(operation, "cohort report contains an unknown column")
		}
		if _, exists := seenColumns[column]; exists {
			return preparedReportQuery{}, invalidArgument(operation, "cohort report columns must be unique")
		}
		seenColumns[column], columnNames[index] = struct{}{}, string(column)
	}
	query := url.Values{
		"start":   {dateString(input.Start)},
		"end":     {dateString(input.End)},
		"columns": {strings.Join(columnNames, ",")},
	}
	setPagination(query, input.Limit, input.Offset, input.NotZero)
	seenFilters := make(map[CohortColumn]struct{}, len(input.Filters))
	for _, filter := range input.Filters {
		if !validCohortColumn(input.Kind, filter.Column) || !validText(filter.Value, 4096) {
			return preparedReportQuery{}, invalidArgument(operation, "cohort filter is invalid")
		}
		if _, exists := seenFilters[filter.Column]; exists {
			return preparedReportQuery{}, invalidArgument(operation, "cohort filters must use unique columns")
		}
		seenFilters[filter.Column] = struct{}{}
		query.Set("filter_"+string(filter.Column), filter.Value)
	}
	seenSorts := make(map[CohortColumn]struct{}, len(input.Sorts))
	ordered := make([]orderedSort, 0, len(input.Sorts))
	for _, sort := range input.Sorts {
		if !validCohortColumn(input.Kind, sort.Column) || sort.Order != SortAscending && sort.Order != SortDescending {
			return preparedReportQuery{}, invalidArgument(operation, "cohort sort is invalid")
		}
		if _, exists := seenSorts[sort.Column]; exists {
			return preparedReportQuery{}, invalidArgument(operation, "cohort sorts must use unique columns")
		}
		seenSorts[sort.Column] = struct{}{}
		ordered = append(ordered, orderedSort{column: string(sort.Column), order: sort.Order})
	}
	return preparedReportQuery{values: query, sorts: ordered}, nil
}

func setPagination(query url.Values, limit, offset int, notZero bool) {
	if limit == 0 {
		limit = DefaultReportLimit
	}
	query.Set("limit", strconv.Itoa(limit))
	query.Set("offset", strconv.Itoa(offset))
	if notZero {
		query.Set("not_zero", "1")
	}
}

func prepareUserLevelRequest(input UserLevelReportRequest, now time.Time) (url.Values, error) {
	const operation = "user_level_report"
	if !validReportWindow(input.Date, input.Date, now) || input.Platform != PlatformAndroid && input.Platform != PlatformFireOS && input.Platform != PlatformIOS ||
		(input.Application == "") == (input.StoreID == "") || input.Application != "" && !validText(input.Application, 512) ||
		input.StoreID != "" && !validText(input.StoreID, 512) {
		return nil, invalidArgument(operation, "date, platform, and exactly one application identifier are required")
	}
	query := url.Values{
		"date":       {dateString(input.Date)},
		"platform":   {string(input.Platform)},
		"aggregated": {strconv.FormatBool(input.AggregateByUser)},
	}
	if input.Application != "" {
		query.Set("application", input.Application)
	} else {
		query.Set("store_id", input.StoreID)
	}
	return query, nil
}

func revenueRowsMatch(rows []RevenueRow, columns []RevenueColumn) bool {
	expected := make(map[RevenueColumn]struct{}, len(columns))
	for _, column := range columns {
		expected[column] = struct{}{}
	}
	for _, row := range rows {
		if len(row) != len(expected) {
			return false
		}
		for column, value := range row {
			if _, found := expected[column]; !found || len(value) > 65_536 {
				return false
			}
		}
	}
	return true
}

func cohortRowsMatch(rows []CohortRow, columns []CohortColumn) bool {
	expected := make(map[CohortColumn]struct{}, len(columns))
	for _, column := range columns {
		expected[column] = struct{}{}
	}
	for _, row := range rows {
		if len(row) != len(expected) {
			return false
		}
		for column, value := range row {
			if _, found := expected[column]; !found || len(value) > 65_536 {
				return false
			}
		}
	}
	return true
}

func cohortPath(kind CohortKind) string {
	switch kind {
	case CohortImpressions:
		return "/maxCohort/imp"
	case CohortSessions:
		return "/maxCohort/session"
	default:
		return "/maxCohort"
	}
}

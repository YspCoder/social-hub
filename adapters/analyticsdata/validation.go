package analyticsdata

import (
	"errors"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	maximumDimensions     = 9
	maximumMetrics        = 10
	maximumFilterDepth    = 8
	maximumFilterNodes    = 100
	maximumMinuteRanges   = 2
	maximumComparisons    = 10
	defaultReportRowLimit = 10_000
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

func validPropertyID(value string) bool {
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

func validServiceAccountEmail(value string) bool {
	if value == "" || len(value) > 254 || value != strings.TrimSpace(value) || strings.Count(value, "@") != 1 || strings.ContainsAny(value, "\x00\r\n ") {
		return false
	}
	parts := strings.Split(value, "@")
	return parts[0] != "" && strings.HasSuffix(strings.ToLower(parts[1]), ".gserviceaccount.com")
}

func validOAuthScopes(scopes []string) bool {
	if len(scopes) == 0 || len(scopes) > 2 {
		return false
	}
	seen := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		if scope != fullScope && scope != readOnlyScope {
			return false
		}
		if _, exists := seen[scope]; exists {
			return false
		}
		seen[scope] = struct{}{}
	}
	return true
}

func validText(value string, maximumBytes int, required bool) bool {
	if required && strings.TrimSpace(value) == "" || value != strings.TrimSpace(value) || !utf8.ValidString(value) ||
		len([]byte(value)) > maximumBytes || strings.ContainsAny(value, "\x00\r\n") {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validFieldName(value string) bool {
	if value == "" || len(value) > 256 {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || strings.ContainsRune("_:.-", character) {
			continue
		}
		return false
	}
	return true
}

func validCurrency(value string) bool {
	if value == "" {
		return true
	}
	if len(value) != 3 {
		return false
	}
	for _, character := range value {
		if character < 'A' || character > 'Z' {
			return false
		}
	}
	return true
}

func validDateExpression(value string) bool {
	if value == "today" || value == "yesterday" {
		return true
	}
	if strings.HasSuffix(value, "daysAgo") {
		days := strings.TrimSuffix(value, "daysAgo")
		parsed, err := strconv.ParseInt(days, 10, 32)
		return err == nil && parsed >= 0 && parsed <= 100_000 && strconv.FormatInt(parsed, 10) == days
	}
	parsed, err := time.Parse("2006-01-02", value)
	return err == nil && parsed.Format("2006-01-02") == value
}

func validDateRange(value DateRange) bool {
	if !validDateExpression(value.StartDate) || !validDateExpression(value.EndDate) || !validText(value.Name, 256, false) ||
		strings.HasPrefix(value.Name, "date_range_") || strings.HasPrefix(value.Name, "RESERVED_") {
		return false
	}
	start, startErr := time.Parse("2006-01-02", value.StartDate)
	end, endErr := time.Parse("2006-01-02", value.EndDate)
	if startErr == nil && endErr == nil {
		return !end.Before(start)
	}
	startAgo, startRelative := relativeDaysAgo(value.StartDate)
	endAgo, endRelative := relativeDaysAgo(value.EndDate)
	return !startRelative || !endRelative || startAgo >= endAgo
}

func relativeDaysAgo(value string) (int64, bool) {
	switch value {
	case "today":
		return 0, true
	case "yesterday":
		return 1, true
	}
	if !strings.HasSuffix(value, "daysAgo") {
		return 0, false
	}
	days, err := strconv.ParseInt(strings.TrimSuffix(value, "daysAgo"), 10, 32)
	return days, err == nil
}

func validDateRanges(values []DateRange, required bool) bool {
	if required && len(values) == 0 || len(values) > DefaultQuotaPolicy().MaximumDateRanges {
		return false
	}
	names := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validDateRange(value) {
			return false
		}
		if value.Name != "" {
			if _, found := names[value.Name]; found {
				return false
			}
			names[value.Name] = struct{}{}
		}
	}
	return true
}

func dimensionNames(values []Dimension, required bool) (map[string]struct{}, bool) {
	if required && len(values) == 0 || len(values) > maximumDimensions {
		return nil, false
	}
	names := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validFieldName(value.Name) {
			return nil, false
		}
		if _, exists := names[value.Name]; exists {
			return nil, false
		}
		names[value.Name] = struct{}{}
	}
	for _, value := range values {
		if value.Expression != nil && !validDimensionExpression(*value.Expression, names) {
			return nil, false
		}
	}
	return names, true
}

func validDimensionExpression(value DimensionExpression, dimensions map[string]struct{}) bool {
	count := boolInt(value.LowerCase != nil) + boolInt(value.UpperCase != nil) + boolInt(value.Concatenate != nil)
	if count != 1 {
		return false
	}
	if value.LowerCase != nil {
		_, found := dimensions[value.LowerCase.DimensionName]
		return found
	}
	if value.UpperCase != nil {
		_, found := dimensions[value.UpperCase.DimensionName]
		return found
	}
	if len(value.Concatenate.DimensionNames) < 2 || len(value.Concatenate.DimensionNames) > maximumDimensions || !validText(value.Concatenate.Delimiter, 128, false) {
		return false
	}
	for _, name := range value.Concatenate.DimensionNames {
		if _, found := dimensions[name]; !found {
			return false
		}
	}
	return true
}

func metricNames(values []Metric, required bool) (map[string]struct{}, bool) {
	if required && len(values) == 0 || len(values) > maximumMetrics {
		return nil, false
	}
	names := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validFieldName(value.Name) || !validText(value.Expression, 1024, false) {
			return nil, false
		}
		if _, exists := names[value.Name]; exists {
			return nil, false
		}
		names[value.Name] = struct{}{}
	}
	return names, true
}

func validNumericValue(value NumericValue) bool {
	if (value.Int64Value == nil) == (value.DoubleValue == nil) {
		return false
	}
	return value.DoubleValue == nil || !math.IsNaN(*value.DoubleValue) && !math.IsInf(*value.DoubleValue, 0)
}

func validFilterExpression(value *FilterExpression) bool {
	if value == nil {
		return true
	}
	nodes := 0
	return validateFilterNode(value, 1, &nodes)
}

func validateFilterNode(value *FilterExpression, depth int, nodes *int) bool {
	if value == nil || depth > maximumFilterDepth || *nodes >= maximumFilterNodes {
		return false
	}
	(*nodes)++
	count := boolInt(value.AndGroup != nil) + boolInt(value.OrGroup != nil) + boolInt(value.NotExpression != nil) + boolInt(value.Filter != nil)
	if count != 1 {
		return false
	}
	if value.NotExpression != nil {
		return validateFilterNode(value.NotExpression, depth+1, nodes)
	}
	if value.Filter != nil {
		return validFilter(*value.Filter)
	}
	group := value.AndGroup
	if group == nil {
		group = value.OrGroup
	}
	if len(group.Expressions) == 0 || len(group.Expressions) > maximumFilterNodes {
		return false
	}
	for index := range group.Expressions {
		if !validateFilterNode(&group.Expressions[index], depth+1, nodes) {
			return false
		}
	}
	return true
}

func validFilter(value Filter) bool {
	if !validFieldName(value.FieldName) {
		return false
	}
	count := boolInt(value.StringFilter != nil) + boolInt(value.InListFilter != nil) + boolInt(value.NumericFilter != nil) +
		boolInt(value.BetweenFilter != nil) + boolInt(value.EmptyFilter != nil)
	if count != 1 {
		return false
	}
	if value.StringFilter != nil {
		return validStringMatch(value.StringFilter.MatchType) && validText(value.StringFilter.Value, 4096, false)
	}
	if value.InListFilter != nil {
		if len(value.InListFilter.Values) == 0 || len(value.InListFilter.Values) > 100 {
			return false
		}
		for _, item := range value.InListFilter.Values {
			if !validText(item, 4096, false) {
				return false
			}
		}
		return true
	}
	if value.NumericFilter != nil {
		return validNumericOperation(value.NumericFilter.Operation) && validNumericValue(value.NumericFilter.Value)
	}
	if value.BetweenFilter != nil {
		return validNumericValue(value.BetweenFilter.FromValue) && validNumericValue(value.BetweenFilter.ToValue)
	}
	return true
}

func validStringMatch(value StringMatchType) bool {
	switch value {
	case "", StringMatchExact, StringMatchBeginsWith, StringMatchEndsWith, StringMatchContains, StringMatchFullRegexp, StringMatchPartialRegexp:
		return true
	default:
		return false
	}
}

func validNumericOperation(value NumericOperation) bool {
	switch value {
	case NumericEqual, NumericLessThan, NumericLessThanOrEqual, NumericGreaterThan, NumericGreaterThanOrEqual:
		return true
	default:
		return false
	}
}

func validMetricAggregations(values []MetricAggregation) bool {
	if len(values) > 4 {
		return false
	}
	seen := make(map[MetricAggregation]struct{}, len(values))
	for _, value := range values {
		switch value {
		case AggregationTotal, AggregationMinimum, AggregationMaximum, AggregationCount:
		default:
			return false
		}
		if _, found := seen[value]; found {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validOrderBys(values []OrderBy) bool {
	if len(values) > 100 {
		return false
	}
	for _, value := range values {
		count := boolInt(value.Metric != nil) + boolInt(value.Dimension != nil) + boolInt(value.Pivot != nil)
		if count != 1 {
			return false
		}
		if value.Metric != nil && !validFieldName(value.Metric.MetricName) {
			return false
		}
		if value.Dimension != nil && (!validFieldName(value.Dimension.DimensionName) || !validDimensionOrderType(value.Dimension.OrderType)) {
			return false
		}
		if value.Pivot != nil {
			if !validFieldName(value.Pivot.MetricName) || len(value.Pivot.PivotSelections) == 0 || len(value.Pivot.PivotSelections) > maximumDimensions {
				return false
			}
			for _, selection := range value.Pivot.PivotSelections {
				if !validFieldName(selection.DimensionName) || !validText(selection.DimensionValue, 4096, false) {
					return false
				}
			}
		}
	}
	return true
}

func validOrderByReferences(values []OrderBy, dimensions, metrics map[string]struct{}, allowPivot bool) bool {
	for _, value := range values {
		if value.Metric != nil {
			if _, found := metrics[value.Metric.MetricName]; !found {
				return false
			}
		}
		if value.Dimension != nil {
			if _, found := dimensions[value.Dimension.DimensionName]; !found {
				return false
			}
		}
		if value.Pivot != nil {
			if !allowPivot {
				return false
			}
			if _, found := metrics[value.Pivot.MetricName]; !found {
				return false
			}
			seen := make(map[string]struct{}, len(value.Pivot.PivotSelections))
			for _, selection := range value.Pivot.PivotSelections {
				if _, found := dimensions[selection.DimensionName]; !found {
					return false
				}
				if _, found := seen[selection.DimensionName]; found {
					return false
				}
				seen[selection.DimensionName] = struct{}{}
			}
		}
	}
	return true
}

func validFilterReferences(value *FilterExpression, fields map[string]struct{}) bool {
	if value == nil {
		return true
	}
	if value.Filter != nil {
		_, found := fields[value.Filter.FieldName]
		return found
	}
	if value.NotExpression != nil {
		return validFilterReferences(value.NotExpression, fields)
	}
	group := value.AndGroup
	if group == nil {
		group = value.OrGroup
	}
	for index := range group.Expressions {
		if !validFilterReferences(&group.Expressions[index], fields) {
			return false
		}
	}
	return true
}

func validDimensionOrderType(value DimensionOrderType) bool {
	return value == "" || value == DimensionOrderAlphanumeric || value == DimensionOrderCaseInsensitiveAlphanumeric || value == DimensionOrderNumeric
}

func validComparisons(values []Comparison) bool {
	if len(values) > maximumComparisons {
		return false
	}
	names := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validText(value.Name, 256, false) || (value.DimensionFilter == nil) == (value.Comparison == "") {
			return false
		}
		if value.DimensionFilter != nil && !validFilterExpression(value.DimensionFilter) {
			return false
		}
		if value.Comparison != "" && !validComparisonName(value.Comparison) {
			return false
		}
		if value.Name != "" {
			if _, found := names[value.Name]; found {
				return false
			}
			names[value.Name] = struct{}{}
		}
	}
	return true
}

func validComparisonName(value string) bool {
	if !strings.HasPrefix(value, "comparisons/") {
		return false
	}
	return validPropertyID(strings.TrimPrefix(value, "comparisons/"))
}

func validCohortSpec(value *CohortSpec) bool {
	if value == nil {
		return true
	}
	if len(value.Cohorts) == 0 || len(value.Cohorts) > 12 || value.CohortsRange.StartOffset < 0 ||
		value.CohortsRange.EndOffset < value.CohortsRange.StartOffset {
		return false
	}
	switch value.CohortsRange.Granularity {
	case CohortDaily, CohortWeekly, CohortMonthly:
	default:
		return false
	}
	names := make(map[string]struct{}, len(value.Cohorts))
	for _, cohort := range value.Cohorts {
		if cohort.Dimension != "firstSessionDate" || !validDateRange(cohort.DateRange) || !validText(cohort.Name, 256, false) ||
			strings.HasPrefix(cohort.Name, "cohort_") || strings.HasPrefix(cohort.Name, "RESERVED_") {
			return false
		}
		if cohort.Name != "" {
			if _, found := names[cohort.Name]; found {
				return false
			}
			names[cohort.Name] = struct{}{}
		}
	}
	return true
}

func validRunReport(value RunReportRequest) bool {
	dimensions, dimensionsValid := dimensionNames(value.Dimensions, false)
	metrics, metricsValid := metricNames(value.Metrics, true)
	dateRangesRequired := value.CohortSpec == nil
	_, hasCohortDimension := dimensions["cohort"]
	cohortValid := value.CohortSpec == nil || hasCohortDimension &&
		(value.CohortSpec.CohortReportSettings == nil || !value.CohortSpec.CohortReportSettings.Accumulate)
	if dimensionsValid && len(value.DateRanges) > 1 {
		dimensions["dateRange"] = struct{}{}
	}
	if dimensionsValid && len(value.Comparisons) > 0 {
		dimensions["comparison"] = struct{}{}
	}
	return dimensionsValid && metricsValid && validDateRanges(value.DateRanges, dateRangesRequired) &&
		(value.CohortSpec == nil || len(value.DateRanges) == 0) && cohortValid && validCohortSpec(value.CohortSpec) &&
		validFilterExpression(value.DimensionFilter) && validFilterExpression(value.MetricFilter) &&
		value.Offset >= 0 && value.Limit >= 0 && value.Limit <= DefaultQuotaPolicy().MaximumRowsPerRequest &&
		validMetricAggregations(value.MetricAggregations) && validOrderBys(value.OrderBys) &&
		validOrderByReferences(value.OrderBys, dimensions, metrics, false) && validCurrency(value.CurrencyCode) &&
		validComparisons(value.Comparisons)
}

func validBatchReports(value BatchRunReportsRequest) bool {
	if len(value.Requests) == 0 || len(value.Requests) > DefaultQuotaPolicy().MaximumBatchRequests {
		return false
	}
	for _, request := range value.Requests {
		if !validRunReport(request) {
			return false
		}
	}
	return true
}

func validRealtimeReport(value RunRealtimeReportRequest) bool {
	dimensions, dimensionsValid := dimensionNames(value.Dimensions, false)
	metrics, metricsValid := metricNames(value.Metrics, true)
	if dimensionsValid && len(value.MinuteRanges) > 1 {
		dimensions["dateRange"] = struct{}{}
	}
	if !dimensionsValid || !metricsValid || !validFilterExpression(value.DimensionFilter) || !validFilterExpression(value.MetricFilter) ||
		value.Limit < 0 || value.Limit > DefaultQuotaPolicy().MaximumRowsPerRequest || !validMetricAggregations(value.MetricAggregations) ||
		!validOrderBys(value.OrderBys) || !validOrderByReferences(value.OrderBys, dimensions, metrics, false) || len(value.MinuteRanges) > maximumMinuteRanges {
		return false
	}
	names := make(map[string]struct{}, len(value.MinuteRanges))
	for _, minuteRange := range value.MinuteRanges {
		if minuteRange.StartMinutesAgo < 0 || minuteRange.StartMinutesAgo > 59 || minuteRange.EndMinutesAgo < 0 ||
			minuteRange.EndMinutesAgo > minuteRange.StartMinutesAgo || !validText(minuteRange.Name, 256, false) ||
			strings.HasPrefix(minuteRange.Name, "date_range_") || strings.HasPrefix(minuteRange.Name, "RESERVED_") {
			return false
		}
		if minuteRange.Name != "" {
			if _, found := names[minuteRange.Name]; found {
				return false
			}
			names[minuteRange.Name] = struct{}{}
		}
	}
	return true
}

func validPivotReport(value RunPivotReportRequest) bool {
	dimensions, dimensionsValid := dimensionNames(value.Dimensions, true)
	metrics, metricsValid := metricNames(value.Metrics, true)
	_, hasCohortDimension := dimensions["cohort"]
	_, hasComparisonDimension := dimensions["comparison"]
	if !dimensionsValid || !metricsValid || !validDateRanges(value.DateRanges, value.CohortSpec == nil) ||
		(value.CohortSpec != nil && len(value.DateRanges) != 0) ||
		(value.CohortSpec != nil && !hasCohortDimension) ||
		(hasComparisonDimension != (len(value.Comparisons) > 0)) ||
		!validCohortSpec(value.CohortSpec) || !validFilterExpression(value.DimensionFilter) || !validFilterExpression(value.MetricFilter) ||
		!validFilterReferences(value.DimensionFilter, dimensions) || !validFilterReferences(value.MetricFilter, metrics) ||
		!validCurrency(value.CurrencyCode) || !validComparisons(value.Comparisons) || len(value.Pivots) == 0 || len(value.Pivots) > maximumDimensions {
		return false
	}
	used := make(map[string]struct{}, len(dimensions))
	product := int64(1)
	for _, pivot := range value.Pivots {
		if len(pivot.FieldNames) == 0 || pivot.Limit <= 0 || pivot.Limit > DefaultQuotaPolicy().MaximumRowsPerRequest || pivot.Offset < 0 ||
			!validMetricAggregations(pivot.MetricAggregations) || !validOrderBys(pivot.OrderBys) ||
			!validOrderByReferences(pivot.OrderBys, dimensions, metrics, true) {
			return false
		}
		fields := make(map[string]struct{}, len(pivot.FieldNames))
		for _, field := range pivot.FieldNames {
			_, dimension := dimensions[field]
			synthetic := field == "dateRange" && len(value.DateRanges) > 1
			if !dimension && !synthetic {
				return false
			}
			if _, found := fields[field]; found {
				return false
			}
			if _, found := used[field]; found {
				return false
			}
			used[field] = struct{}{}
			fields[field] = struct{}{}
		}
		for _, order := range pivot.OrderBys {
			if order.Dimension != nil {
				if _, found := fields[order.Dimension.DimensionName]; !found {
					return false
				}
			}
		}
		if product > DefaultQuotaPolicy().MaximumRowsPerRequest/pivot.Limit {
			return false
		}
		product *= pivot.Limit
	}
	_, comparisonVisible := used["comparison"]
	return len(value.Comparisons) == 0 || comparisonVisible
}

func validBatchPivotReports(value BatchRunPivotReportsRequest) bool {
	if len(value.Requests) == 0 || len(value.Requests) > DefaultQuotaPolicy().MaximumBatchRequests {
		return false
	}
	for _, request := range value.Requests {
		if !validPivotReport(request) {
			return false
		}
	}
	return true
}

func validCompatibilityRequest(value CheckCompatibilityRequest) bool {
	_, dimensionsValid := dimensionNames(value.Dimensions, false)
	_, metricsValid := metricNames(value.Metrics, false)
	return dimensionsValid && metricsValid && len(value.Dimensions)+len(value.Metrics) > 0 && validFilterExpression(value.DimensionFilter) &&
		validFilterExpression(value.MetricFilter) && (value.CompatibilityFilter == "" || value.CompatibilityFilter == Compatible || value.CompatibilityFilter == Incompatible)
}

func validMetadataResponse(value *MetadataResponse, name string) bool {
	if value == nil || value.Name != name || len(value.Raw) == 0 {
		return false
	}
	dimensions := make(map[string]struct{}, len(value.Dimensions))
	for _, metadata := range value.Dimensions {
		if !validFieldName(metadata.APIName) {
			return false
		}
		if _, found := dimensions[metadata.APIName]; found {
			return false
		}
		dimensions[metadata.APIName] = struct{}{}
	}
	metrics := make(map[string]struct{}, len(value.Metrics))
	for _, metadata := range value.Metrics {
		if !validFieldName(metadata.APIName) || !validMetricType(metadata.Type) || !validBlockedReasons(metadata.BlockedReasons) {
			return false
		}
		if _, found := metrics[metadata.APIName]; found {
			return false
		}
		metrics[metadata.APIName] = struct{}{}
	}
	comparisons := make(map[string]struct{}, len(value.Comparisons))
	for _, metadata := range value.Comparisons {
		if !validComparisonName(metadata.APIName) {
			return false
		}
		if _, found := comparisons[metadata.APIName]; found {
			return false
		}
		comparisons[metadata.APIName] = struct{}{}
	}
	return true
}

func validCompatibilityResponse(value *CompatibilityResponse, filter Compatibility) bool {
	if value == nil || len(value.Raw) == 0 {
		return false
	}
	for _, item := range value.DimensionCompatibilities {
		if !validFieldName(item.DimensionMetadata.APIName) || !validCompatibility(item.Compatibility, filter) {
			return false
		}
	}
	for _, item := range value.MetricCompatibilities {
		if !validFieldName(item.MetricMetadata.APIName) || !validMetricType(item.MetricMetadata.Type) ||
			!validBlockedReasons(item.MetricMetadata.BlockedReasons) || !validCompatibility(item.Compatibility, filter) {
			return false
		}
	}
	return true
}

func validCompatibility(value, filter Compatibility) bool {
	return (value == Compatible || value == Incompatible) && (filter == "" || value == filter)
}

func validBlockedReasons(values []MetricBlockedReason) bool {
	seen := make(map[MetricBlockedReason]struct{}, len(values))
	for _, value := range values {
		if value != MetricBlockedNoRevenue && value != MetricBlockedNoCost {
			return false
		}
		if _, found := seen[value]; found {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func effectiveLimit(value int64) int64 {
	if value == 0 {
		return defaultReportRowLimit
	}
	return value
}

func requestedDimensionHeaders(dimensions []Dimension, dateRanges int, comparisons int) []string {
	result := make([]string, 0, len(dimensions)+2)
	for _, dimension := range dimensions {
		result = append(result, dimension.Name)
	}
	if dateRanges > 1 && !containsString(result, "dateRange") {
		result = append(result, "dateRange")
	}
	if comparisons > 0 && !containsString(result, "comparison") {
		result = append(result, "comparison")
	}
	return result
}

func requestedPivotDimensionHeaders(pivots []Pivot) []string {
	result := make([]string, 0, maximumDimensions+1)
	for _, pivot := range pivots {
		result = append(result, pivot.FieldNames...)
	}
	return result
}

func visibleMetricHeaders(metrics []Metric) []string {
	result := make([]string, 0, len(metrics))
	for _, metric := range metrics {
		if !metric.Invisible {
			result = append(result, metric.Name)
		}
	}
	return result
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func validReportResponse(value *ReportResponse, dimensions []string, metrics []string, limit int64, kind string) bool {
	if value == nil || value.Kind != kind || value.RowCount < 0 || int64(len(value.Rows)) > limit || int64(value.RowCount) < int64(len(value.Rows)) ||
		len(value.DimensionHeaders) != len(dimensions) || len(value.MetricHeaders) != len(metrics) {
		return false
	}
	for index, header := range value.DimensionHeaders {
		if header.Name != dimensions[index] {
			return false
		}
	}
	for index, header := range value.MetricHeaders {
		if header.Name != metrics[index] || !validMetricType(header.Type) {
			return false
		}
	}
	for _, rows := range [][]Row{value.Rows, value.Totals, value.Maximums, value.Minimums} {
		for _, row := range rows {
			if len(row.DimensionValues) != len(dimensions) || len(row.MetricValues) != len(metrics) {
				return false
			}
		}
	}
	return validResponseMetadata(value.Metadata) && validPropertyQuota(value.PropertyQuota)
}

func validMetricType(value MetricType) bool {
	switch value {
	case MetricTypeInteger, MetricTypeFloat, MetricTypeSeconds, MetricTypeMilliseconds, MetricTypeMinutes, MetricTypeHours,
		MetricTypeStandard, MetricTypeCurrency, MetricTypeFeet, MetricTypeMiles, MetricTypeMeters, MetricTypeKilometers:
		return true
	default:
		return false
	}
}

func validResponseMetadata(value *ResponseMetadata) bool {
	if value == nil {
		return true
	}
	if !validCurrency(value.CurrencyCode) || !validText(value.TimeZone, 256, false) || !validText(value.EmptyReason, 1024, false) {
		return false
	}
	for _, sample := range value.SamplingMetadatas {
		if sample.SamplesReadCount < 0 || sample.SamplingSpaceSize < 0 || sample.SamplesReadCount > sample.SamplingSpaceSize {
			return false
		}
	}
	return true
}

func validPropertyQuota(value *PropertyQuota) bool {
	if value == nil {
		return true
	}
	for _, status := range []*QuotaStatus{value.TokensPerDay, value.TokensPerHour, value.TokensPerProjectPerHour, value.ConcurrentRequests, value.ServerErrorsPerProjectPerHour, value.PotentiallyThresholdedRequestsPerHour} {
		if status != nil && (status.Consumed < 0 || status.Remaining < 0) {
			return false
		}
	}
	return true
}

func validPivotResponse(value *PivotReportResponse, request RunPivotReportRequest) bool {
	dimensions := requestedPivotDimensionHeaders(request.Pivots)
	metrics := visibleMetricHeaders(request.Metrics)
	if value == nil || value.Kind != "analyticsData#runPivotReport" || len(value.PivotHeaders) != len(request.Pivots) ||
		len(value.DimensionHeaders) != len(dimensions) || len(value.MetricHeaders) != len(metrics) {
		return false
	}
	for index, header := range value.DimensionHeaders {
		if header.Name != dimensions[index] {
			return false
		}
	}
	for index, header := range value.MetricHeaders {
		if header.Name != metrics[index] || !validMetricType(header.Type) {
			return false
		}
	}
	for index, header := range value.PivotHeaders {
		if header.RowCount < 0 || int64(header.RowCount) < int64(len(header.PivotDimensionHeaders)) ||
			int64(len(header.PivotDimensionHeaders)) > request.Pivots[index].Limit {
			return false
		}
		width := len(request.Pivots[index].FieldNames)
		for _, dimensionHeader := range header.PivotDimensionHeaders {
			if len(dimensionHeader.DimensionValues) != width {
				return false
			}
		}
	}
	for _, rows := range [][]Row{value.Rows, value.Aggregates} {
		for _, row := range rows {
			if len(row.DimensionValues) != len(dimensions) || len(row.MetricValues) != len(metrics) {
				return false
			}
		}
	}
	return validResponseMetadata(value.Metadata) && validPropertyQuota(value.PropertyQuota)
}

func sanitizeCause(err error) error {
	if err == nil {
		return nil
	}
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}

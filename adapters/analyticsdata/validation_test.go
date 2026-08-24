package analyticsdata

import (
	"math"
	"strings"
	"testing"
)

func TestFilterValidationBranches(t *testing.T) {
	integer := int64(10)
	double := 1.5
	validFilters := []*FilterExpression{
		{Filter: &Filter{FieldName: "country", StringFilter: &StringFilter{MatchType: StringMatchExact, Value: "US"}}},
		{Filter: &Filter{FieldName: "country", InListFilter: &InListFilter{Values: []string{"US", "CN"}}}},
		{Filter: &Filter{FieldName: "eventCount", NumericFilter: &NumericFilter{Operation: NumericGreaterThan, Value: NumericValue{Int64Value: &integer}}}},
		{Filter: &Filter{FieldName: "engagementRate", BetweenFilter: &BetweenFilter{FromValue: NumericValue{DoubleValue: &double}, ToValue: NumericValue{Int64Value: &integer}}}},
		{Filter: &Filter{FieldName: "country", EmptyFilter: &EmptyFilter{}}},
		{NotExpression: &FilterExpression{Filter: &Filter{FieldName: "country", StringFilter: &StringFilter{Value: "US"}}}},
		{AndGroup: &FilterExpressionList{Expressions: []FilterExpression{
			{Filter: &Filter{FieldName: "country", StringFilter: &StringFilter{Value: "US"}}},
			{Filter: &Filter{FieldName: "city", StringFilter: &StringFilter{MatchType: StringMatchContains, Value: "York"}}},
		}}},
	}
	for index, filter := range validFilters {
		if !validFilterExpression(filter) {
			t.Errorf("valid filter %d rejected", index)
		}
	}
	nan := math.NaN()
	invalidFilters := []*FilterExpression{
		{},
		{Filter: &Filter{FieldName: "country", StringFilter: &StringFilter{Value: "US"}}, NotExpression: &FilterExpression{}},
		{AndGroup: &FilterExpressionList{}},
		{Filter: &Filter{FieldName: "bad field", EmptyFilter: &EmptyFilter{}}},
		{Filter: &Filter{FieldName: "country", StringFilter: &StringFilter{MatchType: "UNKNOWN", Value: "US"}}},
		{Filter: &Filter{FieldName: "country", InListFilter: &InListFilter{}}},
		{Filter: &Filter{FieldName: "eventCount", NumericFilter: &NumericFilter{Operation: "UNKNOWN", Value: NumericValue{Int64Value: &integer}}}},
		{Filter: &Filter{FieldName: "eventCount", NumericFilter: &NumericFilter{Operation: NumericEqual, Value: NumericValue{DoubleValue: &nan}}}},
		{Filter: &Filter{FieldName: "eventCount", BetweenFilter: &BetweenFilter{}}},
		{Filter: &Filter{FieldName: "country", EmptyFilter: &EmptyFilter{}, StringFilter: &StringFilter{Value: "US"}}},
	}
	for index, filter := range invalidFilters {
		if validFilterExpression(filter) {
			t.Errorf("invalid filter %d accepted", index)
		}
	}
	deep := &FilterExpression{Filter: &Filter{FieldName: "country", EmptyFilter: &EmptyFilter{}}}
	for range maximumFilterDepth {
		deep = &FilterExpression{NotExpression: deep}
	}
	if validFilterExpression(deep) {
		t.Fatal("over-depth filter accepted")
	}
}

func TestDimensionOrderComparisonAndCohortValidation(t *testing.T) {
	dimensions := []Dimension{
		{Name: "country"}, {Name: "city"},
		{Name: "countryLower", Expression: &DimensionExpression{LowerCase: &CaseExpression{DimensionName: "country"}}},
		{Name: "cityUpper", Expression: &DimensionExpression{UpperCase: &CaseExpression{DimensionName: "city"}}},
		{Name: "location", Expression: &DimensionExpression{Concatenate: &ConcatenateExpression{DimensionNames: []string{"country", "city"}, Delimiter: "|"}}},
	}
	if _, valid := dimensionNames(dimensions, true); !valid {
		t.Fatal("valid dimension expressions rejected")
	}
	invalidDimensions := [][]Dimension{
		{{Name: "derived", Expression: &DimensionExpression{}}},
		{{Name: "derived", Expression: &DimensionExpression{LowerCase: &CaseExpression{DimensionName: "missing"}}}},
		{{Name: "derived", Expression: &DimensionExpression{Concatenate: &ConcatenateExpression{DimensionNames: []string{"one"}}}}},
		{{Name: "dup"}, {Name: "dup"}},
	}
	for index, values := range invalidDimensions {
		if _, valid := dimensionNames(values, false); valid {
			t.Errorf("invalid dimensions %d accepted", index)
		}
	}
	orders := []OrderBy{
		{Metric: &MetricOrderBy{MetricName: "eventCount"}},
		{Dimension: &DimensionOrderBy{DimensionName: "country", OrderType: DimensionOrderNumeric}},
		{Pivot: &PivotOrderBy{MetricName: "eventCount", PivotSelections: []PivotSelection{{DimensionName: "country", DimensionValue: "US"}}}},
	}
	if !validOrderBys(orders) || validOrderBys([]OrderBy{{}}) || validOrderBys([]OrderBy{{Dimension: &DimensionOrderBy{DimensionName: "country", OrderType: "BROKEN"}}}) {
		t.Fatal("order validation contract failed")
	}
	dimensionSet := map[string]struct{}{"country": {}}
	metricSet := map[string]struct{}{"eventCount": {}}
	if !validOrderByReferences(orders[:1], dimensionSet, metricSet, false) ||
		validOrderByReferences([]OrderBy{{Metric: &MetricOrderBy{MetricName: "missing"}}}, dimensionSet, metricSet, false) ||
		validOrderByReferences(orders[2:], dimensionSet, metricSet, false) {
		t.Fatal("order reference validation contract failed")
	}
	comparisonFilter := &FilterExpression{Filter: &Filter{FieldName: "country", StringFilter: &StringFilter{Value: "US"}}}
	if !validComparisons([]Comparison{{Name: "US", DimensionFilter: comparisonFilter}, {Name: "Saved", Comparison: "comparisons/42"}}) ||
		validComparisons([]Comparison{{DimensionFilter: comparisonFilter, Comparison: "comparisons/42"}}) ||
		validComparisons([]Comparison{{Comparison: "properties/42"}}) {
		t.Fatal("comparison validation contract failed")
	}
	cohort := &CohortSpec{
		Cohorts:      []Cohort{{Name: "August", Dimension: "firstSessionDate", DateRange: DateRange{StartDate: "2026-08-01", EndDate: "2026-08-07"}}},
		CohortsRange: CohortsRange{Granularity: CohortWeekly, EndOffset: 4},
	}
	cohortReport := RunReportRequest{Dimensions: []Dimension{{Name: "cohort"}}, Metrics: []Metric{{Name: "cohortActiveUsers"}}, CohortSpec: cohort}
	if !validRunReport(cohortReport) {
		t.Fatal("valid cohort report rejected")
	}
	cohortReport.Dimensions = nil
	if validRunReport(cohortReport) {
		t.Fatal("cohort report without cohort dimension accepted")
	}
	cohortReport.Dimensions = []Dimension{{Name: "cohort"}}
	cohort.CohortReportSettings = &CohortReportSettings{Accumulate: true}
	if validRunReport(cohortReport) {
		t.Fatal("core accumulate cohort accepted")
	}
	cohort.CohortReportSettings = nil
	cohort.Cohorts[0].Name = "cohort_0"
	if validCohortSpec(cohort) {
		t.Fatal("reserved cohort name accepted")
	}
}

func TestReportAndPivotBoundaryValidation(t *testing.T) {
	report := reportRequest()
	report.Dimensions = append(report.Dimensions, Dimension{Name: "dateRange"})
	report.DateRanges = append(report.DateRanges, DateRange{StartDate: "2026-07-01", EndDate: "2026-07-31", Name: "July"})
	report.Metrics = append(report.Metrics, Metric{Name: "hidden", Invisible: true})
	report.MetricAggregations = []MetricAggregation{AggregationTotal, AggregationCount}
	report.OrderBys = []OrderBy{{Metric: &MetricOrderBy{MetricName: "eventCount"}}}
	report.Comparisons = []Comparison{{DimensionFilter: &FilterExpression{Filter: &Filter{FieldName: "country", EmptyFilter: &EmptyFilter{}}}}}
	if !validRunReport(report) {
		t.Fatal("full report rejected")
	}
	report.Limit = DefaultQuotaPolicy().MaximumRowsPerRequest + 1
	if validRunReport(report) {
		t.Fatal("oversized report accepted")
	}
	realtime := RunRealtimeReportRequest{Metrics: []Metric{{Name: "activeUsers"}}, MinuteRanges: []MinuteRange{{StartMinutesAgo: 59, Name: "lastHour"}}}
	if !validRealtimeReport(realtime) {
		t.Fatal("Analytics 360 realtime range rejected")
	}
	realtime.MinuteRanges[0].Name = "date_range_0"
	if validRealtimeReport(realtime) {
		t.Fatal("reserved minute range name accepted")
	}
	pivot := pivotRequest()
	pivot.Pivots[0].Limit = 1000
	pivot.Pivots[1].Limit = 1000
	if validPivotReport(pivot) {
		t.Fatal("pivot limit product overflow accepted")
	}
	pivot = pivotRequest()
	pivot.Pivots[1].FieldNames = []string{"country"}
	if validPivotReport(pivot) {
		t.Fatal("duplicate pivot dimension accepted")
	}
	pivot = pivotRequest()
	pivot.Pivots[1].FieldNames = []string{"missing"}
	if validPivotReport(pivot) {
		t.Fatal("unknown pivot dimension accepted")
	}
	pivot = pivotRequest()
	pivot.Dimensions = append(pivot.Dimensions, Dimension{Name: "city"})
	pivot.DimensionFilter = &FilterExpression{Filter: &Filter{FieldName: "city", EmptyFilter: &EmptyFilter{}}}
	if !validPivotReport(pivot) {
		t.Fatal("filter-only pivot dimension rejected")
	}
	pivot = pivotRequest()
	pivot.Comparisons = []Comparison{{DimensionFilter: &FilterExpression{Filter: &Filter{FieldName: "country", EmptyFilter: &EmptyFilter{}}}}}
	if validPivotReport(pivot) {
		t.Fatal("pivot comparison without visible comparison dimension accepted")
	}
}

func TestPrimitiveValidationBoundaries(t *testing.T) {
	if validPropertyID("0") || validPropertyID("12x") || !validPropertyID("42") ||
		validEndpoint("https://user:pass@example.com") || validEndpoint("https://example.com/") || !validEndpoint("http://localhost") ||
		validCallbackURL("ftp://example.com") || validCallbackURL("https://user:pass@example.com") || !validCallbackURL("http://localhost/callback") ||
		validOAuthScopes(nil) || validOAuthScopes([]string{readOnlyScope, readOnlyScope}) || !validOAuthScopes([]string{readOnlyScope, fullScope}) ||
		validCurrency("usd") || validCurrency("US") || !validCurrency("USD") ||
		validFieldName("bad field") || !validFieldName("customEvent:event-name") ||
		validText("bad\ntext", 20, false) || validText(strings.Repeat("x", 21), 20, false) {
		t.Fatal("primitive validation contract failed")
	}
	if !validMetricAggregations([]MetricAggregation{AggregationTotal, AggregationMinimum, AggregationMaximum, AggregationCount}) ||
		validMetricAggregations([]MetricAggregation{AggregationTotal, AggregationTotal}) || validMetricAggregations([]MetricAggregation{"UNKNOWN"}) {
		t.Fatal("aggregation validation contract failed")
	}
	if DefaultQuotaPolicy().MaximumRowsPerRequest != 250_000 || effectiveLimit(0) != defaultReportRowLimit || effectiveLimit(50) != 50 {
		t.Fatal("quota or effective limit changed")
	}
	if validDateRange(DateRange{StartDate: "today", EndDate: "yesterday"}) ||
		!validDateRange(DateRange{StartDate: "7daysAgo", EndDate: "yesterday"}) {
		t.Fatal("relative date ordering failed")
	}
}

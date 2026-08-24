package analyticsdata

import "encoding/json"

type DateRange struct {
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
	Name      string `json:"name,omitempty"`
}

type Dimension struct {
	Name       string               `json:"name"`
	Expression *DimensionExpression `json:"dimensionExpression,omitempty"`
}

type DimensionExpression struct {
	LowerCase   *CaseExpression        `json:"lowerCase,omitempty"`
	UpperCase   *CaseExpression        `json:"upperCase,omitempty"`
	Concatenate *ConcatenateExpression `json:"concatenate,omitempty"`
}

type CaseExpression struct {
	DimensionName string `json:"dimensionName"`
}

type ConcatenateExpression struct {
	DimensionNames []string `json:"dimensionNames"`
	Delimiter      string   `json:"delimiter,omitempty"`
}

type Metric struct {
	Name       string `json:"name"`
	Expression string `json:"expression,omitempty"`
	Invisible  bool   `json:"invisible,omitempty"`
}

type FilterExpression struct {
	AndGroup      *FilterExpressionList `json:"andGroup,omitempty"`
	OrGroup       *FilterExpressionList `json:"orGroup,omitempty"`
	NotExpression *FilterExpression     `json:"notExpression,omitempty"`
	Filter        *Filter               `json:"filter,omitempty"`
}

type FilterExpressionList struct {
	Expressions []FilterExpression `json:"expressions"`
}

type Filter struct {
	FieldName     string         `json:"fieldName"`
	StringFilter  *StringFilter  `json:"stringFilter,omitempty"`
	InListFilter  *InListFilter  `json:"inListFilter,omitempty"`
	NumericFilter *NumericFilter `json:"numericFilter,omitempty"`
	BetweenFilter *BetweenFilter `json:"betweenFilter,omitempty"`
	EmptyFilter   *EmptyFilter   `json:"emptyFilter,omitempty"`
}

type StringMatchType string

const (
	StringMatchExact         StringMatchType = "EXACT"
	StringMatchBeginsWith    StringMatchType = "BEGINS_WITH"
	StringMatchEndsWith      StringMatchType = "ENDS_WITH"
	StringMatchContains      StringMatchType = "CONTAINS"
	StringMatchFullRegexp    StringMatchType = "FULL_REGEXP"
	StringMatchPartialRegexp StringMatchType = "PARTIAL_REGEXP"
)

type StringFilter struct {
	MatchType     StringMatchType `json:"matchType,omitempty"`
	Value         string          `json:"value"`
	CaseSensitive bool            `json:"caseSensitive,omitempty"`
}

type InListFilter struct {
	Values        []string `json:"values"`
	CaseSensitive bool     `json:"caseSensitive,omitempty"`
}

type NumericOperation string

const (
	NumericEqual              NumericOperation = "EQUAL"
	NumericLessThan           NumericOperation = "LESS_THAN"
	NumericLessThanOrEqual    NumericOperation = "LESS_THAN_OR_EQUAL"
	NumericGreaterThan        NumericOperation = "GREATER_THAN"
	NumericGreaterThanOrEqual NumericOperation = "GREATER_THAN_OR_EQUAL"
)

type NumericFilter struct {
	Operation NumericOperation `json:"operation"`
	Value     NumericValue     `json:"value"`
}

type BetweenFilter struct {
	FromValue NumericValue `json:"fromValue"`
	ToValue   NumericValue `json:"toValue"`
}

type NumericValue struct {
	Int64Value  *int64   `json:"int64Value,omitempty,string"`
	DoubleValue *float64 `json:"doubleValue,omitempty"`
}

type EmptyFilter struct{}

type MetricAggregation string

const (
	AggregationTotal   MetricAggregation = "TOTAL"
	AggregationMinimum MetricAggregation = "MINIMUM"
	AggregationMaximum MetricAggregation = "MAXIMUM"
	AggregationCount   MetricAggregation = "COUNT"
)

type OrderBy struct {
	Metric    *MetricOrderBy    `json:"metric,omitempty"`
	Dimension *DimensionOrderBy `json:"dimension,omitempty"`
	Pivot     *PivotOrderBy     `json:"pivot,omitempty"`
	Desc      bool              `json:"desc,omitempty"`
}

type MetricOrderBy struct {
	MetricName string `json:"metricName"`
}

type DimensionOrderType string

const (
	DimensionOrderAlphanumeric                DimensionOrderType = "ALPHANUMERIC"
	DimensionOrderCaseInsensitiveAlphanumeric DimensionOrderType = "CASE_INSENSITIVE_ALPHANUMERIC"
	DimensionOrderNumeric                     DimensionOrderType = "NUMERIC"
)

type DimensionOrderBy struct {
	DimensionName string             `json:"dimensionName"`
	OrderType     DimensionOrderType `json:"orderType,omitempty"`
}

type PivotOrderBy struct {
	MetricName      string           `json:"metricName"`
	PivotSelections []PivotSelection `json:"pivotSelections"`
}

type PivotSelection struct {
	DimensionName  string `json:"dimensionName"`
	DimensionValue string `json:"dimensionValue"`
}

type Comparison struct {
	Name            string            `json:"name,omitempty"`
	DimensionFilter *FilterExpression `json:"dimensionFilter,omitempty"`
	Comparison      string            `json:"comparison,omitempty"`
}

type CohortSpec struct {
	Cohorts              []Cohort              `json:"cohorts"`
	CohortsRange         CohortsRange          `json:"cohortsRange"`
	CohortReportSettings *CohortReportSettings `json:"cohortReportSettings,omitempty"`
}

type Cohort struct {
	Name      string    `json:"name,omitempty"`
	Dimension string    `json:"dimension"`
	DateRange DateRange `json:"dateRange"`
}

type CohortGranularity string

const (
	CohortDaily   CohortGranularity = "DAILY"
	CohortWeekly  CohortGranularity = "WEEKLY"
	CohortMonthly CohortGranularity = "MONTHLY"
)

type CohortsRange struct {
	Granularity CohortGranularity `json:"granularity"`
	StartOffset int32             `json:"startOffset,omitempty"`
	EndOffset   int32             `json:"endOffset,omitempty"`
}

type CohortReportSettings struct {
	Accumulate bool `json:"accumulate,omitempty"`
}

type RunReportRequest struct {
	DateRanges          []DateRange         `json:"dateRanges,omitempty"`
	Dimensions          []Dimension         `json:"dimensions,omitempty"`
	Metrics             []Metric            `json:"metrics"`
	DimensionFilter     *FilterExpression   `json:"dimensionFilter,omitempty"`
	MetricFilter        *FilterExpression   `json:"metricFilter,omitempty"`
	Offset              int64               `json:"offset,omitempty,string"`
	Limit               int64               `json:"limit,omitempty,string"`
	MetricAggregations  []MetricAggregation `json:"metricAggregations,omitempty"`
	OrderBys            []OrderBy           `json:"orderBys,omitempty"`
	CurrencyCode        string              `json:"currencyCode,omitempty"`
	CohortSpec          *CohortSpec         `json:"cohortSpec,omitempty"`
	KeepEmptyRows       bool                `json:"keepEmptyRows,omitempty"`
	ReturnPropertyQuota bool                `json:"returnPropertyQuota,omitempty"`
	Comparisons         []Comparison        `json:"comparisons,omitempty"`
}

type BatchRunReportsRequest struct {
	Requests []RunReportRequest `json:"requests"`
}

type MinuteRange struct {
	StartMinutesAgo int32  `json:"startMinutesAgo"`
	EndMinutesAgo   int32  `json:"endMinutesAgo,omitempty"`
	Name            string `json:"name,omitempty"`
}

type RunRealtimeReportRequest struct {
	Dimensions          []Dimension         `json:"dimensions,omitempty"`
	Metrics             []Metric            `json:"metrics"`
	DimensionFilter     *FilterExpression   `json:"dimensionFilter,omitempty"`
	MetricFilter        *FilterExpression   `json:"metricFilter,omitempty"`
	Limit               int64               `json:"limit,omitempty,string"`
	MetricAggregations  []MetricAggregation `json:"metricAggregations,omitempty"`
	OrderBys            []OrderBy           `json:"orderBys,omitempty"`
	ReturnPropertyQuota bool                `json:"returnPropertyQuota,omitempty"`
	MinuteRanges        []MinuteRange       `json:"minuteRanges,omitempty"`
}

type Pivot struct {
	FieldNames         []string            `json:"fieldNames"`
	OrderBys           []OrderBy           `json:"orderBys,omitempty"`
	Offset             int64               `json:"offset,omitempty,string"`
	Limit              int64               `json:"limit,string"`
	MetricAggregations []MetricAggregation `json:"metricAggregations,omitempty"`
}

type RunPivotReportRequest struct {
	DateRanges          []DateRange       `json:"dateRanges,omitempty"`
	Dimensions          []Dimension       `json:"dimensions"`
	Metrics             []Metric          `json:"metrics"`
	Pivots              []Pivot           `json:"pivots"`
	DimensionFilter     *FilterExpression `json:"dimensionFilter,omitempty"`
	MetricFilter        *FilterExpression `json:"metricFilter,omitempty"`
	CurrencyCode        string            `json:"currencyCode,omitempty"`
	CohortSpec          *CohortSpec       `json:"cohortSpec,omitempty"`
	KeepEmptyRows       bool              `json:"keepEmptyRows,omitempty"`
	ReturnPropertyQuota bool              `json:"returnPropertyQuota,omitempty"`
	Comparisons         []Comparison      `json:"comparisons,omitempty"`
}

type BatchRunPivotReportsRequest struct {
	Requests []RunPivotReportRequest `json:"requests"`
}

type Compatibility string

const (
	Compatible   Compatibility = "COMPATIBLE"
	Incompatible Compatibility = "INCOMPATIBLE"
)

type CheckCompatibilityRequest struct {
	Dimensions          []Dimension       `json:"dimensions,omitempty"`
	Metrics             []Metric          `json:"metrics,omitempty"`
	DimensionFilter     *FilterExpression `json:"dimensionFilter,omitempty"`
	MetricFilter        *FilterExpression `json:"metricFilter,omitempty"`
	CompatibilityFilter Compatibility     `json:"compatibilityFilter,omitempty"`
}

type MetricType string

const (
	MetricTypeInteger      MetricType = "TYPE_INTEGER"
	MetricTypeFloat        MetricType = "TYPE_FLOAT"
	MetricTypeSeconds      MetricType = "TYPE_SECONDS"
	MetricTypeMilliseconds MetricType = "TYPE_MILLISECONDS"
	MetricTypeMinutes      MetricType = "TYPE_MINUTES"
	MetricTypeHours        MetricType = "TYPE_HOURS"
	MetricTypeStandard     MetricType = "TYPE_STANDARD"
	MetricTypeCurrency     MetricType = "TYPE_CURRENCY"
	MetricTypeFeet         MetricType = "TYPE_FEET"
	MetricTypeMiles        MetricType = "TYPE_MILES"
	MetricTypeMeters       MetricType = "TYPE_METERS"
	MetricTypeKilometers   MetricType = "TYPE_KILOMETERS"
)

type DimensionHeader struct {
	Name string `json:"name"`
}

type MetricHeader struct {
	Name string     `json:"name"`
	Type MetricType `json:"type,omitempty"`
}

type DimensionValue struct {
	Value string `json:"value,omitempty"`
}

type MetricValue struct {
	Value string `json:"value,omitempty"`
}

type Row struct {
	DimensionValues []DimensionValue `json:"dimensionValues,omitempty"`
	MetricValues    []MetricValue    `json:"metricValues,omitempty"`
}

type SamplingMetadata struct {
	SamplesReadCount  int64 `json:"samplesReadCount,omitempty,string"`
	SamplingSpaceSize int64 `json:"samplingSpaceSize,omitempty,string"`
}

type RestrictedMetricType string

const (
	RestrictedMetricCostData    RestrictedMetricType = "COST_DATA"
	RestrictedMetricRevenueData RestrictedMetricType = "REVENUE_DATA"
)

type ActiveMetricRestriction struct {
	MetricName            string                 `json:"metricName,omitempty"`
	RestrictedMetricTypes []RestrictedMetricType `json:"restrictedMetricTypes,omitempty"`
}

type SchemaRestrictionResponse struct {
	ActiveMetricRestrictions []ActiveMetricRestriction `json:"activeMetricRestrictions,omitempty"`
}

type ResponseMetadata struct {
	DataLossFromOtherRow      bool                       `json:"dataLossFromOtherRow,omitempty"`
	SchemaRestrictionResponse *SchemaRestrictionResponse `json:"schemaRestrictionResponse,omitempty"`
	CurrencyCode              string                     `json:"currencyCode,omitempty"`
	TimeZone                  string                     `json:"timeZone,omitempty"`
	EmptyReason               string                     `json:"emptyReason,omitempty"`
	SubjectToThresholding     bool                       `json:"subjectToThresholding,omitempty"`
	SamplingMetadatas         []SamplingMetadata         `json:"samplingMetadatas,omitempty"`
}

type QuotaStatus struct {
	Consumed  int32 `json:"consumed,omitempty"`
	Remaining int32 `json:"remaining,omitempty"`
}

type PropertyQuota struct {
	TokensPerDay                          *QuotaStatus `json:"tokensPerDay,omitempty"`
	TokensPerHour                         *QuotaStatus `json:"tokensPerHour,omitempty"`
	TokensPerProjectPerHour               *QuotaStatus `json:"tokensPerProjectPerHour,omitempty"`
	ConcurrentRequests                    *QuotaStatus `json:"concurrentRequests,omitempty"`
	ServerErrorsPerProjectPerHour         *QuotaStatus `json:"serverErrorsPerProjectPerHour,omitempty"`
	PotentiallyThresholdedRequestsPerHour *QuotaStatus `json:"potentiallyThresholdedRequestsPerHour,omitempty"`
}

type ReportResponse struct {
	DimensionHeaders []DimensionHeader `json:"dimensionHeaders,omitempty"`
	MetricHeaders    []MetricHeader    `json:"metricHeaders,omitempty"`
	Rows             []Row             `json:"rows,omitempty"`
	Totals           []Row             `json:"totals,omitempty"`
	Maximums         []Row             `json:"maximums,omitempty"`
	Minimums         []Row             `json:"minimums,omitempty"`
	RowCount         int32             `json:"rowCount,omitempty"`
	Metadata         *ResponseMetadata `json:"metadata,omitempty"`
	PropertyQuota    *PropertyQuota    `json:"propertyQuota,omitempty"`
	Kind             string            `json:"kind,omitempty"`
	Raw              json.RawMessage   `json:"-"`
}

type BatchReportResponse struct {
	Reports []ReportResponse `json:"reports,omitempty"`
	Kind    string           `json:"kind,omitempty"`
	Raw     json.RawMessage  `json:"-"`
}

type PivotDimensionHeader struct {
	DimensionValues []DimensionValue `json:"dimensionValues,omitempty"`
}

type PivotHeader struct {
	PivotDimensionHeaders []PivotDimensionHeader `json:"pivotDimensionHeaders,omitempty"`
	RowCount              int32                  `json:"rowCount,omitempty"`
}

type PivotReportResponse struct {
	PivotHeaders     []PivotHeader     `json:"pivotHeaders,omitempty"`
	DimensionHeaders []DimensionHeader `json:"dimensionHeaders,omitempty"`
	MetricHeaders    []MetricHeader    `json:"metricHeaders,omitempty"`
	Rows             []Row             `json:"rows,omitempty"`
	Aggregates       []Row             `json:"aggregates,omitempty"`
	Metadata         *ResponseMetadata `json:"metadata,omitempty"`
	PropertyQuota    *PropertyQuota    `json:"propertyQuota,omitempty"`
	Kind             string            `json:"kind,omitempty"`
	Raw              json.RawMessage   `json:"-"`
}

type BatchPivotReportResponse struct {
	PivotReports []PivotReportResponse `json:"pivotReports,omitempty"`
	Kind         string                `json:"kind,omitempty"`
	Raw          json.RawMessage       `json:"-"`
}

type DimensionMetadata struct {
	APIName            string   `json:"apiName,omitempty"`
	UIName             string   `json:"uiName,omitempty"`
	Description        string   `json:"description,omitempty"`
	DeprecatedAPINames []string `json:"deprecatedApiNames,omitempty"`
	CustomDefinition   bool     `json:"customDefinition,omitempty"`
	Category           string   `json:"category,omitempty"`
}

type MetricBlockedReason string

const (
	MetricBlockedNoRevenue MetricBlockedReason = "NO_REVENUE_METRICS"
	MetricBlockedNoCost    MetricBlockedReason = "NO_COST_METRICS"
)

type MetricMetadata struct {
	APIName            string                `json:"apiName,omitempty"`
	UIName             string                `json:"uiName,omitempty"`
	Description        string                `json:"description,omitempty"`
	DeprecatedAPINames []string              `json:"deprecatedApiNames,omitempty"`
	Type               MetricType            `json:"type,omitempty"`
	Expression         string                `json:"expression,omitempty"`
	CustomDefinition   bool                  `json:"customDefinition,omitempty"`
	BlockedReasons     []MetricBlockedReason `json:"blockedReasons,omitempty"`
	Category           string                `json:"category,omitempty"`
}

type ComparisonMetadata struct {
	APIName     string `json:"apiName,omitempty"`
	UIName      string `json:"uiName,omitempty"`
	Description string `json:"description,omitempty"`
}

type MetadataResponse struct {
	Name        string               `json:"name"`
	Dimensions  []DimensionMetadata  `json:"dimensions,omitempty"`
	Metrics     []MetricMetadata     `json:"metrics,omitempty"`
	Comparisons []ComparisonMetadata `json:"comparisons,omitempty"`
	Raw         json.RawMessage      `json:"-"`
}

type DimensionCompatibility struct {
	DimensionMetadata DimensionMetadata `json:"dimensionMetadata"`
	Compatibility     Compatibility     `json:"compatibility"`
}

type MetricCompatibility struct {
	MetricMetadata MetricMetadata `json:"metricMetadata"`
	Compatibility  Compatibility  `json:"compatibility"`
}

type CompatibilityResponse struct {
	DimensionCompatibilities []DimensionCompatibility `json:"dimensionCompatibilities,omitempty"`
	MetricCompatibilities    []MetricCompatibility    `json:"metricCompatibilities,omitempty"`
	Raw                      json.RawMessage          `json:"-"`
}

func captureRaw(data []byte, target any, raw *json.RawMessage) error {
	if err := json.Unmarshal(data, target); err != nil {
		return err
	}
	*raw = append((*raw)[:0], data...)
	return nil
}

func (value *ReportResponse) UnmarshalJSON(data []byte) error {
	type alias ReportResponse
	return captureRaw(data, (*alias)(value), &value.Raw)
}

func (value *BatchReportResponse) UnmarshalJSON(data []byte) error {
	type alias BatchReportResponse
	return captureRaw(data, (*alias)(value), &value.Raw)
}

func (value *PivotReportResponse) UnmarshalJSON(data []byte) error {
	type alias PivotReportResponse
	return captureRaw(data, (*alias)(value), &value.Raw)
}

func (value *BatchPivotReportResponse) UnmarshalJSON(data []byte) error {
	type alias BatchPivotReportResponse
	return captureRaw(data, (*alias)(value), &value.Raw)
}

func (value *MetadataResponse) UnmarshalJSON(data []byte) error {
	type alias MetadataResponse
	return captureRaw(data, (*alias)(value), &value.Raw)
}

func (value *CompatibilityResponse) UnmarshalJSON(data []byte) error {
	type alias CompatibilityResponse
	return captureRaw(data, (*alias)(value), &value.Raw)
}

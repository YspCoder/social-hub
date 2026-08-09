package admanager

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

type Dimension string
type Metric string
type ReportType string
type RelativeDateRange string
type TimeZoneSource string
type ReportVisibility string

const (
	DimensionDate         Dimension = "DATE"
	DimensionAdUnitID     Dimension = "AD_UNIT_ID"
	DimensionAdUnitName   Dimension = "AD_UNIT_NAME"
	DimensionOrderID      Dimension = "ORDER_ID"
	DimensionOrderName    Dimension = "ORDER_NAME"
	DimensionLineItemID   Dimension = "LINE_ITEM_ID"
	DimensionLineItemName Dimension = "LINE_ITEM_NAME"

	MetricImpressions         Metric = "IMPRESSIONS"
	MetricClicks              Metric = "CLICKS"
	MetricRevenue             Metric = "REVENUE"
	MetricAdServerImpressions Metric = "AD_SERVER_IMPRESSIONS"
	MetricAdServerClicks      Metric = "AD_SERVER_CLICKS"

	ReportHistorical          ReportType = "HISTORICAL"
	ReportFutureSellThrough   ReportType = "FUTURE_SELL_THROUGH"
	ReportReach               ReportType = "REACH"
	ReportRevenueVerification ReportType = "REVENUE_VERIFICATION"
	ReportAdSpeed             ReportType = "AD_SPEED"
	ReportRealTimeVideo       ReportType = "REAL_TIME_VIDEO"

	RelativeToday        RelativeDateRange = "TODAY"
	RelativeYesterday    RelativeDateRange = "YESTERDAY"
	RelativeLast7Days    RelativeDateRange = "LAST_7_DAYS"
	RelativeLast30Days   RelativeDateRange = "LAST_30_DAYS"
	RelativeLast90Days   RelativeDateRange = "LAST_90_DAYS"
	RelativeAllAvailable RelativeDateRange = "ALL_AVAILABLE"

	TimeZonePublisher  TimeZoneSource = "PUBLISHER"
	TimeZoneAdExchange TimeZoneSource = "AD_EXCHANGE"
	TimeZoneUTC        TimeZoneSource = "UTC"
	TimeZoneProvided   TimeZoneSource = "PROVIDED"

	ReportHidden  ReportVisibility = "HIDDEN"
	ReportDraft   ReportVisibility = "DRAFT"
	ReportVisible ReportVisibility = "VISIBLE"
)

type Date struct {
	Year  int32 `json:"year"`
	Month int32 `json:"month"`
	Day   int32 `json:"day"`
}

type FixedDateRange struct {
	StartDate Date `json:"startDate"`
	EndDate   Date `json:"endDate"`
}

// DateRange is a protobuf oneof: exactly one of Fixed or Relative must be set.
type DateRange struct {
	Fixed    *FixedDateRange   `json:"fixed,omitempty"`
	Relative RelativeDateRange `json:"relative,omitempty"`
}

type ReportDefinition struct {
	Dimensions            []Dimension    `json:"dimensions"`
	Metrics               []Metric       `json:"metrics"`
	DateRange             DateRange      `json:"dateRange"`
	ComparisonDateRange   *DateRange     `json:"comparisonDateRange,omitempty"`
	ReportType            ReportType     `json:"reportType"`
	TimeZoneSource        TimeZoneSource `json:"timeZoneSource,omitempty"`
	TimeZone              string         `json:"timeZone,omitempty"`
	CurrencyCode          string         `json:"currencyCode,omitempty"`
	ExpandedCompatibility bool           `json:"expandedCompatibility,omitempty"`
}

type Report struct {
	Name             string           `json:"name"`
	ReportID         string           `json:"reportId,omitempty"`
	DisplayName      string           `json:"displayName,omitempty"`
	ReportDefinition ReportDefinition `json:"reportDefinition"`
	Visibility       ReportVisibility `json:"visibility,omitempty"`
	Locale           string           `json:"locale,omitempty"`
	CreateTime       string           `json:"createTime,omitempty"`
	UpdateTime       string           `json:"updateTime,omitempty"`
	Raw              json.RawMessage  `json:"-"`
}

func (value *Report) UnmarshalJSON(data []byte) error {
	type alias Report
	return captureRaw(data, (*alias)(value), &value.Raw)
}

type CreateReportRequest struct {
	DisplayName string
	Definition  ReportDefinition
}

type RunReportMetadata struct {
	Type            string `json:"@type,omitempty"`
	Report          string `json:"report,omitempty"`
	PercentComplete int32  `json:"percentComplete,omitempty"`
}

type RunReportResponse struct {
	Type         string `json:"@type,omitempty"`
	ReportResult string `json:"reportResult,omitempty"`
}

type RPCStatus struct {
	Code    int32  `json:"code"`
	Message string `json:"message,omitempty"`
}

type ReportOperation struct {
	Name     string
	Done     bool
	Metadata RunReportMetadata
	Result   *RunReportResponse
	Failure  *RPCStatus
	Raw      json.RawMessage
}

type ReportValue struct {
	StringValue     *string     `json:"stringValue,omitempty"`
	IntValue        *string     `json:"intValue,omitempty"`
	DoubleValue     *float64    `json:"doubleValue,omitempty"`
	BoolValue       *bool       `json:"boolValue,omitempty"`
	BytesValue      *string     `json:"bytesValue,omitempty"`
	StringListValue *StringList `json:"stringListValue,omitempty"`
	IntListValue    *IntList    `json:"intListValue,omitempty"`
	DoubleListValue *DoubleList `json:"doubleListValue,omitempty"`
}

type StringList struct {
	Values []string `json:"values"`
}
type IntList struct {
	Values []string `json:"values"`
}
type DoubleList struct {
	Values []float64 `json:"values"`
}

type MetricValueGroup struct {
	PrimaryValues                  []ReportValue `json:"primaryValues,omitempty"`
	PrimaryPercentOfTotalValues    []ReportValue `json:"primaryPercentOfTotalValues,omitempty"`
	ComparisonValues               []ReportValue `json:"comparisonValues,omitempty"`
	ComparisonPercentOfTotalValues []ReportValue `json:"comparisonPercentOfTotalValues,omitempty"`
	AbsoluteChangeValues           []ReportValue `json:"absoluteChangeValues,omitempty"`
	RelativeChangeValues           []ReportValue `json:"relativeChangeValues,omitempty"`
	FlagValues                     []bool        `json:"flagValues,omitempty"`
}

type ReportRow struct {
	DimensionValues   []ReportValue      `json:"dimensionValues"`
	MetricValueGroups []MetricValueGroup `json:"metricValueGroups"`
}

type ReportRowsPage struct {
	Rows                 []ReportRow
	RunTime              string
	DateRanges           []FixedDateRange
	ComparisonDateRanges []FixedDateRange
	TotalRowCount        int32
	NextPageToken        string
}

type FetchReportRowsRequest struct {
	ResultName string
	PageSize   int32
	PageToken  string
}

type ReportingWorkflow interface {
	GetReport(context.Context, string, ...socialhub.CallOption) (*Report, error)
	ListReports(context.Context, ListRequest, ...socialhub.CallOption) (Page[Report], error)
	CreateHiddenReport(context.Context, CreateReportRequest, ...socialhub.CallOption) (*Report, error)
	RunReport(context.Context, string, ...socialhub.CallOption) (*ReportOperation, error)
	GetReportOperation(context.Context, string, ...socialhub.CallOption) (*ReportOperation, error)
	FetchReportRows(context.Context, FetchReportRowsRequest, ...socialhub.CallOption) (ReportRowsPage, error)
}

func (client *Client) GetReport(ctx context.Context, reportID string, options ...socialhub.CallOption) (*Report, error) {
	const operation = "report_get"
	name, err := client.resourceName(operation, "reports", reportID)
	if err != nil {
		return nil, err
	}
	var output Report
	if err := client.getJSON(ctx, operation, "/v1/"+name, nil, &output, options...); err != nil {
		return nil, err
	}
	if output.Name != name {
		return nil, ownershipError(operation, "report")
	}
	return &output, nil
}

func (client *Client) ListReports(ctx context.Context, input ListRequest, options ...socialhub.CallOption) (Page[Report], error) {
	const operation = "reports_list"
	query, err := listQuery(operation, input, 1000)
	if err != nil {
		return Page[Report]{}, err
	}
	query.Set("fields", "reports,nextPageToken,totalSize")
	var output struct {
		Reports       []Report `json:"reports"`
		NextPageToken string   `json:"nextPageToken"`
		TotalSize     int32    `json:"totalSize"`
	}
	if err := client.getJSON(ctx, operation, "/v1/"+client.networkName()+"/reports", query, &output, options...); err != nil {
		return Page[Report]{}, err
	}
	for _, item := range output.Reports {
		if !client.ownsResource(item.Name, "reports") {
			return Page[Report]{}, ownershipError(operation, "report")
		}
	}
	return Page[Report]{Items: output.Reports, NextPageToken: output.NextPageToken, TotalSize: output.TotalSize}, nil
}

func (client *Client) CreateHiddenReport(ctx context.Context, input CreateReportRequest, options ...socialhub.CallOption) (*Report, error) {
	const operation = "report_create_hidden"
	if !validName(input.DisplayName, 255) || !validReportDefinition(input.Definition) {
		return nil, invalidArgument(operation, "display name or report definition is invalid")
	}
	payload := struct {
		DisplayName string           `json:"displayName"`
		Definition  ReportDefinition `json:"reportDefinition"`
		Visibility  ReportVisibility `json:"visibility"`
	}{DisplayName: input.DisplayName, Definition: input.Definition, Visibility: ReportHidden}
	var output Report
	if err := client.postWriteJSON(ctx, operation, "/v1/"+client.networkName()+"/reports", payload, &output, options...); err != nil {
		return nil, err
	}
	if !client.ownsResource(output.Name, "reports") {
		return nil, ownershipError(operation, "report")
	}
	if output.Visibility != ReportHidden {
		return nil, platformContractError(operation, "created report was not hidden")
	}
	return &output, nil
}

func (client *Client) RunReport(ctx context.Context, reportID string, options ...socialhub.CallOption) (*ReportOperation, error) {
	const operation = "report_run"
	name, err := client.resourceName(operation, "reports", reportID)
	if err != nil {
		return nil, err
	}
	var wire operationWire
	if err := client.postReadJSON(ctx, operation, "/v1/"+name+":run", struct{}{}, &wire, options...); err != nil {
		return nil, err
	}
	result, err := client.decodeReportOperation(operation, wire)
	if err != nil {
		return nil, err
	}
	if result.Metadata.Report != "" && result.Metadata.Report != name {
		return nil, ownershipError(operation, "report operation")
	}
	return result, nil
}

func (client *Client) GetReportOperation(ctx context.Context, operationName string, options ...socialhub.CallOption) (*ReportOperation, error) {
	const operation = "report_operation_get"
	if !client.ownsReportOperation(operationName) {
		return nil, invalidArgument(operation, "operation name is invalid or belongs to another network")
	}
	var wire operationWire
	if err := client.getJSON(ctx, operation, "/v1/"+operationName, nil, &wire, options...); err != nil {
		return nil, err
	}
	return client.decodeReportOperation(operation, wire)
}

func (client *Client) FetchReportRows(ctx context.Context, input FetchReportRowsRequest, options ...socialhub.CallOption) (ReportRowsPage, error) {
	const operation = "report_rows_fetch"
	if !client.ownsReportResult(input.ResultName) || input.PageSize < 0 || input.PageSize > 10000 || !validPageToken(input.PageToken) {
		return ReportRowsPage{}, invalidArgument(operation, "result name or pagination is invalid")
	}
	query := make(url.Values)
	if input.PageSize > 0 {
		query.Set("pageSize", int32String(input.PageSize))
	}
	if input.PageToken != "" {
		query.Set("pageToken", input.PageToken)
	}
	var output struct {
		Rows                 []ReportRow      `json:"rows"`
		RunTime              string           `json:"runTime"`
		DateRanges           []FixedDateRange `json:"dateRanges"`
		ComparisonDateRanges []FixedDateRange `json:"comparisonDateRanges"`
		TotalRowCount        int32            `json:"totalRowCount"`
		NextPageToken        string           `json:"nextPageToken"`
	}
	if err := client.getJSON(ctx, operation, "/v1/"+input.ResultName+":fetchRows", query, &output, options...); err != nil {
		return ReportRowsPage{}, err
	}
	return ReportRowsPage{
		Rows: output.Rows, RunTime: output.RunTime, DateRanges: output.DateRanges,
		ComparisonDateRanges: output.ComparisonDateRanges, TotalRowCount: output.TotalRowCount,
		NextPageToken: output.NextPageToken,
	}, nil
}

type operationWire struct {
	Name     string          `json:"name"`
	Metadata json.RawMessage `json:"metadata"`
	Done     bool            `json:"done"`
	Error    *RPCStatus      `json:"error"`
	Response json.RawMessage `json:"response"`
}

func (client *Client) decodeReportOperation(operation string, wire operationWire) (*ReportOperation, error) {
	if !client.ownsReportOperation(wire.Name) {
		return nil, ownershipError(operation, "report operation")
	}
	if wire.Error != nil {
		wire.Error.Message = boundedMessage(redactSensitive(wire.Error.Message), 1024)
	}
	result := &ReportOperation{Name: wire.Name, Done: wire.Done, Failure: wire.Error}
	if len(wire.Metadata) > 0 {
		if err := json.Unmarshal(wire.Metadata, &result.Metadata); err != nil {
			return nil, platformContractError(operation, "invalid report operation metadata")
		}
		if result.Metadata.Report != "" && !client.ownsResource(result.Metadata.Report, "reports") {
			return nil, ownershipError(operation, "report operation")
		}
		result.Metadata.Type = boundedMessage(result.Metadata.Type, 256)
	}
	if len(wire.Response) > 0 {
		var response RunReportResponse
		if err := json.Unmarshal(wire.Response, &response); err != nil {
			return nil, platformContractError(operation, "invalid report operation response")
		}
		if response.ReportResult == "" || !client.ownsReportResult(response.ReportResult) {
			return nil, ownershipError(operation, "report result")
		}
		if result.Metadata.Report != "" && !strings.HasPrefix(response.ReportResult, result.Metadata.Report+"/results/") {
			return nil, ownershipError(operation, "report result")
		}
		result.Result = &response
	}
	if wire.Done && wire.Error == nil && result.Result == nil {
		return nil, platformContractError(operation, "completed report operation omitted both result and failure")
	}
	encoded, _ := json.Marshal(wire)
	result.Raw = encoded
	return result, nil
}

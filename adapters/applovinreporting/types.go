package applovinreporting

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	DefaultReportLimit       = 100
	MaximumReportLimit       = 500
	DefaultMaxCSVReportBytes = int64(256 << 20)

	assetDocumentationURL    = "https://support.applovin.com/en/growth/promoting-your-apps/api/asset-reporting-api"
	playableDocumentationURL = "https://support.applovin.com/en/growth/promoting-your-apps/api/html-metrics-api"
)

type AccountType string

const (
	AccountTypeApp AccountType = "APP"
	AccountTypeWeb AccountType = "WEB"
)

type ReportType string

const (
	ReportAdvertiser ReportType = "advertiser"
	ReportPublisher  ReportType = "publisher"
)

// ReportTime is a YYYY-MM-DD date, a Unix timestamp in seconds, or ReportNow
// when used as an end value for a CampaignReportRequest.
type ReportTime string

const ReportNow ReportTime = "now"

func ReportDate(value time.Time) ReportTime { return ReportTime(value.UTC().Format("2006-01-02")) }
func ReportUnix(value time.Time) ReportTime { return ReportTime(fmt.Sprintf("%d", value.Unix())) }

// Date is a UTC calendar date used by Asset and Playable reports.
type Date string

func DateFromTime(value time.Time) Date { return Date(value.UTC().Format("2006-01-02")) }

type CampaignColumn string
type AssetColumn string
type PlayableColumn string

type SortOrder string

const (
	SortAscending  SortOrder = "ASC"
	SortDescending SortOrder = "DESC"
)

type AttributionMode string

const (
	AttributionCohort   AttributionMode = "cohort"
	AttributionRealtime AttributionMode = "realtime"
)

type ComparisonOperator string

const (
	CompareGreaterThan        ComparisonOperator = ">"
	CompareLessThan           ComparisonOperator = "<"
	CompareGreaterThanOrEqual ComparisonOperator = ">="
	CompareLessThanOrEqual    ComparisonOperator = "<="
	CompareEqual              ComparisonOperator = "="
	CompareNotEqual           ComparisonOperator = "!="
)

type HavingCombine string

const (
	HavingAND HavingCombine = "AND"
	HavingOR  HavingCombine = "OR"
)

type CustomPageFilter string

const (
	CustomPageNull     CustomPageFilter = "filter_null_custom_page_id"
	CustomPageBlank    CustomPageFilter = "filter_blank_custom_page_id"
	CustomPageNotNull  CustomPageFilter = "filter_not_null_custom_page_id"
	CustomPageNotBlank CustomPageFilter = "filter_not_blank_custom_page_id"
)

type Pagination struct {
	Offset int
	Limit  int
}

type CampaignFilter struct {
	Column CampaignColumn
	Values []string
	Negate bool
}

type CampaignSort struct {
	Column CampaignColumn
	Order  SortOrder
}

type HavingCondition struct {
	Column   CampaignColumn
	Operator ComparisonOperator
	Value    string
}

type Having struct {
	Combine    HavingCombine
	Conditions []HavingCondition
}

type CampaignReportRequest struct {
	Type              ReportType
	Start             ReportTime
	End               ReportTime
	Columns           []CampaignColumn
	Filters           []CampaignFilter
	Sorts             []CampaignSort
	Having            *Having
	CustomPageFilters []CustomPageFilter
	NotZero           bool
	Attribution       AttributionMode
	Pagination        Pagination
}

type AssetRange string

const (
	AssetYesterday AssetRange = "yesterday"
	AssetLast7Days AssetRange = "last_7d"
	AssetLastMonth AssetRange = "last_month"
)

type AssetFilter struct {
	Column AssetColumn
	Values []string
	Negate bool
}

type MetricFilter struct {
	Column      AssetColumn
	GreaterThan string
	LessThan    string
}

type AssetSort struct {
	Column AssetColumn
	Order  SortOrder
}

type AssetReportRequest struct {
	Range      AssetRange
	Start      Date
	End        Date
	Columns    []AssetColumn
	Filters    []AssetFilter
	Metrics    []MetricFilter
	Sorts      []AssetSort
	NotZero    bool
	Pagination Pagination
}

type PlayableFilter struct {
	Column PlayableColumn
	Values []string
	Negate bool
}

type PlayableSort struct {
	Column PlayableColumn
	Order  SortOrder
}

type PlayableReportRequest struct {
	Start       Date
	End         Date
	Columns     []PlayableColumn
	Filters     []PlayableFilter
	Sorts       []PlayableSort
	Attribution AttributionMode
	Pagination  Pagination
}

// ReportValue preserves strings, exact decimal representations, and JSON null.
type ReportValue struct {
	Text string
	Null bool
}

func (value ReportValue) String() string { return value.Text }
func (value ReportValue) IsNull() bool   { return value.Null }

func (value *ReportValue) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		value.Text, value.Null = "", true
		return nil
	}
	if len(data) > 0 && data[0] == '"' {
		if err := json.Unmarshal(data, &value.Text); err != nil {
			return err
		}
		value.Null = false
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var number json.Number
	if err := decoder.Decode(&number); err != nil || number.String() == "" {
		return fmt.Errorf("report value must be a string, number, or null")
	}
	value.Text, value.Null = number.String(), false
	return nil
}

func (value ReportValue) MarshalJSON() ([]byte, error) {
	if value.Null {
		return []byte("null"), nil
	}
	return json.Marshal(value.Text)
}

type CampaignRow map[CampaignColumn]ReportValue
type AssetRow map[AssetColumn]ReportValue
type PlayableRow map[PlayableColumn]ReportValue

type CampaignReport struct {
	Count int
	Rows  []CampaignRow
}

type AssetReport struct {
	Count int
	Rows  []AssetRow
}

type PlayableReport struct {
	Count int
	Rows  []PlayableRow
}

type DownloadOptions struct {
	// MaxBytes bounds bytes written to Output. Zero uses DefaultMaxCSVReportBytes.
	MaxBytes int64
}

type DownloadResult struct {
	StatusCode   int
	BytesWritten int64
	DataRows     int64
	ContentType  string
}

type AccountInfo struct {
	AccountID         string
	AccountType       AccountType
	AccountIDVerified bool
}

type ReportsWorkflow interface {
	AccountInfo(context.Context, ...socialhub.CallOption) (AccountInfo, error)
	CampaignReport(context.Context, CampaignReportRequest, ...socialhub.CallOption) (CampaignReport, error)
	DownloadCampaignCSV(context.Context, CampaignReportRequest, io.Writer, DownloadOptions, ...socialhub.CallOption) (DownloadResult, error)
	AssetReport(context.Context, AssetReportRequest, ...socialhub.CallOption) (AssetReport, error)
	DownloadAssetCSV(context.Context, AssetReportRequest, io.Writer, DownloadOptions, ...socialhub.CallOption) (DownloadResult, error)
	PlayableReport(context.Context, PlayableReportRequest, ...socialhub.CallOption) (PlayableReport, error)
	DownloadPlayableCSV(context.Context, PlayableReportRequest, io.Writer, DownloadOptions, ...socialhub.CallOption) (DownloadResult, error)
}

var _ ReportsWorkflow = (*Client)(nil)

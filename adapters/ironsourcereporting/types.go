package ironsourcereporting

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
	DefaultCount             = 10_000
	MaximumCount             = 250_000
	DefaultMaxCSVReportBytes = int64(256 << 20)

	costDocumentationURL = "https://docs.unity.com/en-us/grow/is-ads/user-acquisition/apis/cost-api"
	skanDocumentationURL = "https://docs.unity.com/en-us/grow/is-ads/user-acquisition/apis/skan-reporting-api"
)

type Date string

func DateFromTime(value time.Time) Date { return Date(value.UTC().Format("2006-01-02")) }

type AdvertiserMetric string
type CostMetric string
type SKANMetric string

const (
	AdvertiserMetricImpressions AdvertiserMetric = "impressions"
	AdvertiserMetricClicks      AdvertiserMetric = "clicks"
	AdvertiserMetricCompletions AdvertiserMetric = "completions"
	AdvertiserMetricInstalls    AdvertiserMetric = "installs"
	AdvertiserMetricSpend       AdvertiserMetric = "spend"

	CostMetricImpressions   CostMetric = "impressions"
	CostMetricClicks        CostMetric = "clicks"
	CostMetricInstalls      CostMetric = "installs"
	CostMetricBillableSpend CostMetric = "billable_spend"
	CostMetricECPI          CostMetric = "ecpi"

	SKANMetricImpressions SKANMetric = "impressions"
	SKANMetricStoreOpens  SKANMetric = "storeOpens"
	SKANMetricInstalls    SKANMetric = "installs"
	SKANMetricSpend       SKANMetric = "spend"
)

type AdvertiserBreakdown string
type CostBreakdown string
type SKANBreakdown string
type SKANCVBreakdown string

const (
	AdvertiserBreakdownDay            AdvertiserBreakdown = "day"
	AdvertiserBreakdownCampaign       AdvertiserBreakdown = "campaign"
	AdvertiserBreakdownTitle          AdvertiserBreakdown = "title"
	AdvertiserBreakdownApplication    AdvertiserBreakdown = "application"
	AdvertiserBreakdownCountry        AdvertiserBreakdown = "country"
	AdvertiserBreakdownDeviceType     AdvertiserBreakdown = "deviceType"
	AdvertiserBreakdownCreative       AdvertiserBreakdown = "creative"
	AdvertiserBreakdownAdUnit         AdvertiserBreakdown = "adUnit"
	AdvertiserBreakdownOptimizedEvent AdvertiserBreakdown = "optimized_event"

	CostBreakdownDay      CostBreakdown = "day"
	CostBreakdownCampaign CostBreakdown = "campaign"
	CostBreakdownTitle    CostBreakdown = "title"
	CostBreakdownOS       CostBreakdown = "os"
	CostBreakdownCountry  CostBreakdown = "country"

	SKANBreakdownDay         SKANBreakdown = "day"
	SKANBreakdownCampaign    SKANBreakdown = "campaign"
	SKANBreakdownTitle       SKANBreakdown = "title"
	SKANBreakdownApplication SKANBreakdown = "application"
	SKANBreakdownAdUnit      SKANBreakdown = "adUnit"
	SKANBreakdownCountry     SKANBreakdown = "country"

	SKANCVBreakdownDay         SKANCVBreakdown = "day"
	SKANCVBreakdownCampaign    SKANCVBreakdown = "campaign"
	SKANCVBreakdownTitle       SKANCVBreakdown = "title"
	SKANCVBreakdownApplication SKANCVBreakdown = "application"
)

type OperatingSystem string
type DeviceType string
type AdUnit string
type Direction string
type Order string

const (
	OSIOS     OperatingSystem = "ios"
	OSAndroid OperatingSystem = "android"

	DevicePhone  DeviceType = "phone"
	DeviceTablet DeviceType = "tablet"

	AdUnitRewardedVideo AdUnit = "rewardedVideo"
	AdUnitInterstitial  AdUnit = "interstitial"
	AdUnitOfferWall     AdUnit = "offerWall"
	AdUnitBanner        AdUnit = "banner"

	DirectionAscending  Direction = "asc"
	DirectionDescending Direction = "desc"

	OrderDay           Order = "day"
	OrderCampaign      Order = "campaign"
	OrderTitle         Order = "title"
	OrderApplication   Order = "application"
	OrderCreative      Order = "creative"
	OrderCountry       Order = "country"
	OrderOS            Order = "os"
	OrderImpressions   Order = "impressions"
	OrderClicks        Order = "clicks"
	OrderCompletions   Order = "completions"
	OrderInstalls      Order = "installs"
	OrderSpend         Order = "spend"
	OrderBillableSpend Order = "billable_spend"
)

type ReportFilters struct {
	CampaignIDs []int64
	BundleIDs   []string
	Countries   []string
	OS          OperatingSystem
}

type AdvertiserFilters struct {
	ReportFilters
	CreativeIDs        []int64
	DeviceType         DeviceType
	AdUnit             AdUnit
	ExcludeCampaignIDs []int64
	ExcludeBundleIDs   []string
	ExcludeCreativeIDs []int64
	ExcludeCountries   []string
}

type SKANFilters struct {
	CampaignIDs []int64
	BundleIDs   []string
	Countries   []string
}

type AdvertiserReportRequest struct {
	Start      Date
	End        Date
	Metrics    []AdvertiserMetric
	Breakdowns []AdvertiserBreakdown
	Filters    AdvertiserFilters
	Order      Order
	Direction  Direction
	Count      int
	Cursor     string
}

type CostReportRequest struct {
	Start      Date
	End        Date
	Metrics    []CostMetric
	Breakdowns []CostBreakdown
	Filters    ReportFilters
	Order      Order
	Direction  Direction
	Count      int
	Cursor     string
}

type SKANReportRequest struct {
	Start      Date
	End        Date
	Metrics    []SKANMetric
	Breakdowns []SKANBreakdown
	Filters    SKANFilters
	AdUnit     AdUnit
	Order      Order
	Direction  Direction
	Count      int
	Cursor     string
}

type SKANConversionValueRequest struct {
	Start       Date
	End         Date
	Breakdowns  []SKANCVBreakdown
	CampaignIDs []int64
	BundleIDs   []string
	Order       Order
	Direction   Direction
	Count       int
	Cursor      string
}

// ReportValue preserves strings, exact JSON numbers, and null without using
// float64. Report rows are dynamic because their columns depend on breakdowns.
type ReportValue struct {
	Text string
	Null bool
}

func (value ReportValue) String() string { return value.Text }
func (value ReportValue) IsNull() bool   { return value.Null }

func (value *ReportValue) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if bytes.Equal(trimmed, []byte("null")) {
		value.Text, value.Null = "", true
		return nil
	}
	if len(trimmed) > 0 && trimmed[0] == '"' {
		if err := json.Unmarshal(trimmed, &value.Text); err != nil {
			return err
		}
		value.Null = false
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
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

type ReportRow map[string]ReportValue

type ReportPage struct {
	Rows       []ReportRow
	NextCursor string
	HasMore    bool
}

type ConversionValueRow struct {
	Fields           ReportRow
	ConversionValues map[uint8]int64
}

type ConversionValuePage struct {
	Rows       []ConversionValueRow
	NextCursor string
	HasMore    bool
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
	NoData       bool
	NextCursor   string
	HasMore      bool
}

// QuotaPolicy records the documented per-endpoint advertiser reporting limit.
type QuotaPolicy struct {
	Requests int
	Window   time.Duration
}

var DefaultQuotaPolicy = QuotaPolicy{Requests: 100, Window: time.Minute}

type ReportsWorkflow interface {
	AdvertiserReport(context.Context, AdvertiserReportRequest, ...socialhub.CallOption) (ReportPage, error)
	CostReport(context.Context, CostReportRequest, ...socialhub.CallOption) (ReportPage, error)
	SKANReport(context.Context, SKANReportRequest, ...socialhub.CallOption) (ReportPage, error)
	SKANConversionValues(context.Context, SKANConversionValueRequest, ...socialhub.CallOption) (ConversionValuePage, error)
	DownloadAdvertiserCSV(context.Context, AdvertiserReportRequest, io.Writer, DownloadOptions, ...socialhub.CallOption) (DownloadResult, error)
	DownloadCostCSV(context.Context, CostReportRequest, io.Writer, DownloadOptions, ...socialhub.CallOption) (DownloadResult, error)
	DownloadSKANCSV(context.Context, SKANReportRequest, io.Writer, DownloadOptions, ...socialhub.CallOption) (DownloadResult, error)
	DownloadSKANConversionValuesCSV(context.Context, SKANConversionValueRequest, io.Writer, DownloadOptions, ...socialhub.CallOption) (DownloadResult, error)
}

var _ ReportsWorkflow = (*Client)(nil)

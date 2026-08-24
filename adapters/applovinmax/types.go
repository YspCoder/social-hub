package applovinmax

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
	DefaultReportLimit             = 1_000
	MaximumJSONReportLimit         = 10_000
	MaximumStreamReportLimit       = 1_000_000
	DefaultMaxReportBytes    int64 = 256 << 20
)

// Date is a calendar date in the UTC reporting timezone.
type Date struct {
	Year  int
	Month time.Month
	Day   int
}

func DateFromTime(value time.Time) Date {
	year, month, day := value.UTC().Date()
	return Date{Year: year, Month: month, Day: day}
}

type SortOrder string

const (
	SortAscending  SortOrder = "ASC"
	SortDescending SortOrder = "DESC"
)

type RevenueColumn string

const (
	RevenueAdFormat            RevenueColumn = "ad_format"
	RevenueAdUnitWaterfallName RevenueColumn = "ad_unit_waterfall_name"
	RevenueApplication         RevenueColumn = "application"
	RevenueAttempts            RevenueColumn = "attempts"
	RevenueCountry             RevenueColumn = "country"
	RevenueCustomNetworkName   RevenueColumn = "custom_network_name"
	RevenueDay                 RevenueColumn = "day"
	RevenueDeviceType          RevenueColumn = "device_type"
	RevenueECPM                RevenueColumn = "ecpm"
	RevenueEstimatedRevenue    RevenueColumn = "estimated_revenue"
	RevenueFillRate            RevenueColumn = "fill_rate"
	RevenueHasIDFA             RevenueColumn = "has_idfa"
	RevenueHour                RevenueColumn = "hour"
	RevenueImpressions         RevenueColumn = "impressions"
	RevenueMAXAdUnit           RevenueColumn = "max_ad_unit"
	RevenueMAXAdUnitID         RevenueColumn = "max_ad_unit_id"
	RevenueMAXAdUnitTest       RevenueColumn = "max_ad_unit_test"
	RevenueMAXPlacement        RevenueColumn = "max_placement"
	RevenueNetwork             RevenueColumn = "network"
	RevenueNetworkPlacement    RevenueColumn = "network_placement"
	RevenuePackageName         RevenueColumn = "package_name"
	RevenuePlatform            RevenueColumn = "platform"
	RevenueRequests            RevenueColumn = "requests"
	RevenueResponses           RevenueColumn = "responses"
	RevenueStoreID             RevenueColumn = "store_id"
)

type RevenueFilter struct {
	Column RevenueColumn
	Value  string
}

type RevenueSort struct {
	Column RevenueColumn
	Order  SortOrder
}

type RevenueReportRequest struct {
	Start, End Date
	Columns    []RevenueColumn
	Filters    []RevenueFilter
	Sorts      []RevenueSort
	Limit      int
	Offset     int
	NotZero    bool
}

// ReportValue preserves the exact decimal or string representation returned
// by AppLovin instead of converting money and ratios to float64.
type ReportValue string

func (value *ReportValue) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("empty report value")
	}
	if data[0] == '"' {
		var decoded string
		if err := json.Unmarshal(data, &decoded); err != nil {
			return err
		}
		*value = ReportValue(decoded)
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var number json.Number
	if err := decoder.Decode(&number); err != nil {
		return fmt.Errorf("report value must be a string or number: %w", err)
	}
	if number.String() == "" {
		return fmt.Errorf("report value must be a string or number")
	}
	*value = ReportValue(number.String())
	return nil
}

type RevenueRow map[RevenueColumn]ReportValue

type RevenueReport struct {
	Count int
	Rows  []RevenueRow
}

type CohortKind string

const (
	CohortRevenue     CohortKind = "revenue"
	CohortImpressions CohortKind = "impressions"
	CohortSessions    CohortKind = "sessions"
)

type CohortColumn string

const (
	CohortApplication CohortColumn = "application"
	CohortCountry     CohortColumn = "country"
	CohortDayColumn   CohortColumn = "day"
	CohortInstalls    CohortColumn = "installs"
	CohortPackageName CohortColumn = "package_name"
	CohortPlatform    CohortColumn = "platform"
)

type CohortMetric string

const (
	CohortAdsPublisherRevenue       CohortMetric = "ads_pub_revenue"
	CohortAdsRevenuePerInstall      CohortMetric = "ads_rpi"
	CohortBannerPublisherRevenue    CohortMetric = "banner_pub_revenue"
	CohortBannerRevenuePerInstall   CohortMetric = "banner_rpi"
	CohortIAPPublisherRevenue       CohortMetric = "iap_pub_revenue"
	CohortIAPRevenuePerInstall      CohortMetric = "iap_rpi"
	CohortInterstitialRevenue       CohortMetric = "inter_pub_revenue"
	CohortInterstitialRPI           CohortMetric = "inter_rpi"
	CohortMRECPublisherRevenue      CohortMetric = "mrec_pub_revenue"
	CohortMRECRevenuePerInstall     CohortMetric = "mrec_rpi"
	CohortPublisherRevenue          CohortMetric = "pub_revenue"
	CohortRewardedPublisherRevenue  CohortMetric = "reward_pub_revenue"
	CohortRewardedRevenuePerInstall CohortMetric = "reward_rpi"
	CohortRevenuePerInstall         CohortMetric = "rpi"

	CohortBannerImpressions        CohortMetric = "banner_imp"
	CohortBannerImpressionsPerUser CohortMetric = "banner_imp_per_user"
	CohortImpressionCount          CohortMetric = "imp"
	CohortImpressionsPerUser       CohortMetric = "imp_per_user"
	CohortInterstitialImpressions  CohortMetric = "inter_imp"
	CohortInterstitialImpsPerUser  CohortMetric = "inter_imp_per_user"
	CohortMRECImpressions          CohortMetric = "mrec_imp"
	CohortMRECImpressionsPerUser   CohortMetric = "mrec_imp_per_user"
	CohortRewardedImpressions      CohortMetric = "reward_imp"
	CohortRewardedImpsPerUser      CohortMetric = "reward_imp_per_user"

	CohortDailyUsage    CohortMetric = "daily_usage"
	CohortRetention     CohortMetric = "retention"
	CohortSessionCount  CohortMetric = "session_count"
	CohortSessionLength CohortMetric = "session_length"
	CohortUserCount     CohortMetric = "user_count"
)

type CohortAge int

func CohortMetricAt(metric CohortMetric, age CohortAge) CohortColumn {
	return CohortColumn(fmt.Sprintf("%s_%d", metric, age))
}

type CohortFilter struct {
	Column CohortColumn
	Value  string
}

type CohortSort struct {
	Column CohortColumn
	Order  SortOrder
}

type CohortReportRequest struct {
	Kind       CohortKind
	Start, End Date
	Columns    []CohortColumn
	Filters    []CohortFilter
	Sorts      []CohortSort
	Limit      int
	Offset     int
	NotZero    bool
}

type CohortRow map[CohortColumn]ReportValue

type CohortReport struct {
	Count int
	Rows  []CohortRow
}

type AppPlatform string

const (
	PlatformAndroid AppPlatform = "android"
	PlatformFireOS  AppPlatform = "fireos"
	PlatformIOS     AppPlatform = "ios"
)

type UserLevelReportRequest struct {
	Date            Date
	Platform        AppPlatform
	Application     string
	StoreID         string
	AggregateByUser bool
}

type UserLevelReport struct {
	Status                int    `json:"status"`
	URL                   string `json:"url"`
	AdRevenueReportURL    string `json:"ad_revenue_report_url"`
	FBEstimatedRevenueURL string `json:"fb_estimated_revenue_url"`
}

type UserLevelReportVariant string

const (
	UserLevelWithoutMetaEstimate UserLevelReportVariant = "without_meta_estimate"
	UserLevelWithMetaEstimate    UserLevelReportVariant = "with_meta_estimate"
	UserLevelMetaEstimateOnly    UserLevelReportVariant = "meta_estimate_only"
)

type DownloadOptions struct {
	// MaxBytes bounds bytes written to Output. Zero uses DefaultMaxReportBytes.
	MaxBytes int64
}

type DownloadResult struct {
	StatusCode   int
	BytesWritten int64
	ContentType  string
	ETag         string
	LastModified string
}

type ReportsWorkflow interface {
	RevenueReport(context.Context, RevenueReportRequest, ...socialhub.CallOption) (RevenueReport, error)
	DownloadRevenueReport(context.Context, RevenueReportRequest, io.Writer, DownloadOptions, ...socialhub.CallOption) (DownloadResult, error)
	CohortReport(context.Context, CohortReportRequest, ...socialhub.CallOption) (CohortReport, error)
	DownloadCohortReport(context.Context, CohortReportRequest, io.Writer, DownloadOptions, ...socialhub.CallOption) (DownloadResult, error)
	RequestUserLevelReport(context.Context, UserLevelReportRequest, ...socialhub.CallOption) (UserLevelReport, error)
	DownloadUserLevelReport(context.Context, UserLevelReport, UserLevelReportVariant, io.Writer, DownloadOptions, ...socialhub.CallOption) (DownloadResult, error)
}

var _ ReportsWorkflow = (*Client)(nil)

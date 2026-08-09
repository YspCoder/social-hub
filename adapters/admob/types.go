package admob

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
)

type ListRequest struct {
	PageSize  int32
	PageToken string
}

type Page[T any] struct {
	Items         []T
	NextPageToken string
}

type PublisherAccount struct {
	Name              string          `json:"name"`
	PublisherID       string          `json:"publisherId"`
	ReportingTimeZone string          `json:"reportingTimeZone"`
	CurrencyCode      string          `json:"currencyCode"`
	Raw               json.RawMessage `json:"-"`
}

type AppPlatform string

const (
	AppPlatformIOS     AppPlatform = "IOS"
	AppPlatformAndroid AppPlatform = "ANDROID"
)

type AppApprovalState string

const (
	AppApprovalUnspecified    AppApprovalState = "APP_APPROVAL_STATE_UNSPECIFIED"
	AppApprovalActionRequired AppApprovalState = "ACTION_REQUIRED"
	AppApprovalInReview       AppApprovalState = "IN_REVIEW"
	AppApprovalApproved       AppApprovalState = "APPROVED"
)

type ManualAppInfo struct {
	DisplayName string `json:"displayName,omitempty"`
}

type LinkedAppInfo struct {
	AppStoreID  string `json:"appStoreId,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
}

type App struct {
	Name             string           `json:"name"`
	AppID            string           `json:"appId"`
	Platform         AppPlatform      `json:"platform"`
	ManualAppInfo    *ManualAppInfo   `json:"manualAppInfo,omitempty"`
	LinkedAppInfo    *LinkedAppInfo   `json:"linkedAppInfo,omitempty"`
	AppApprovalState AppApprovalState `json:"appApprovalState"`
	Raw              json.RawMessage  `json:"-"`
}

type AdFormat string

const (
	AdFormatAppOpen              AdFormat = "APP_OPEN"
	AdFormatBanner               AdFormat = "BANNER"
	AdFormatBannerInterstitial   AdFormat = "BANNER_INTERSTITIAL"
	AdFormatInterstitial         AdFormat = "INTERSTITIAL"
	AdFormatNative               AdFormat = "NATIVE"
	AdFormatRewarded             AdFormat = "REWARDED"
	AdFormatRewardedInterstitial AdFormat = "REWARDED_INTERSTITIAL"
)

type AdType string

const (
	AdTypeRichMedia AdType = "RICH_MEDIA"
	AdTypeVideo     AdType = "VIDEO"
)

type AdUnit struct {
	Name        string          `json:"name"`
	AdUnitID    string          `json:"adUnitId"`
	AppID       string          `json:"appId"`
	DisplayName string          `json:"displayName"`
	AdFormat    AdFormat        `json:"adFormat"`
	AdTypes     []AdType        `json:"adTypes"`
	Raw         json.RawMessage `json:"-"`
}

type Date struct {
	Year  int32 `json:"year"`
	Month int32 `json:"month"`
	Day   int32 `json:"day"`
}

type DateRange struct {
	StartDate Date `json:"startDate"`
	EndDate   Date `json:"endDate"`
}

type LocalizationSettings struct {
	CurrencyCode string `json:"currencyCode,omitempty"`
	LanguageCode string `json:"languageCode,omitempty"`
}

type Dimension string

const (
	DimensionDate               Dimension = "DATE"
	DimensionMonth              Dimension = "MONTH"
	DimensionWeek               Dimension = "WEEK"
	DimensionAdSource           Dimension = "AD_SOURCE"
	DimensionAdSourceInstance   Dimension = "AD_SOURCE_INSTANCE"
	DimensionAdUnit             Dimension = "AD_UNIT"
	DimensionApp                Dimension = "APP"
	DimensionMediationGroup     Dimension = "MEDIATION_GROUP"
	DimensionAdType             Dimension = "AD_TYPE"
	DimensionCountry            Dimension = "COUNTRY"
	DimensionFormat             Dimension = "FORMAT"
	DimensionPlatform           Dimension = "PLATFORM"
	DimensionMobileOSVersion    Dimension = "MOBILE_OS_VERSION"
	DimensionGMASDKVersion      Dimension = "GMA_SDK_VERSION"
	DimensionAppVersionName     Dimension = "APP_VERSION_NAME"
	DimensionServingRestriction Dimension = "SERVING_RESTRICTION"
)

type Metric string

const (
	MetricAdRequests        Metric = "AD_REQUESTS"
	MetricClicks            Metric = "CLICKS"
	MetricEstimatedEarnings Metric = "ESTIMATED_EARNINGS"
	MetricImpressions       Metric = "IMPRESSIONS"
	MetricImpressionCTR     Metric = "IMPRESSION_CTR"
	MetricImpressionRPM     Metric = "IMPRESSION_RPM"
	MetricMatchedRequests   Metric = "MATCHED_REQUESTS"
	MetricMatchRate         Metric = "MATCH_RATE"
	MetricShowRate          Metric = "SHOW_RATE"
	MetricObservedECPM      Metric = "OBSERVED_ECPM"
)

type SortOrder string

const (
	SortAscending  SortOrder = "ASCENDING"
	SortDescending SortOrder = "DESCENDING"
)

type DimensionFilter struct {
	Dimension  Dimension  `json:"dimension"`
	MatchesAny StringList `json:"matchesAny"`
}

type StringList struct {
	Values []string `json:"values"`
}

// SortCondition sets exactly one of Dimension or Metric.
type SortCondition struct {
	Dimension Dimension `json:"dimension,omitempty"`
	Metric    Metric    `json:"metric,omitempty"`
	Order     SortOrder `json:"order"`
}

type NetworkReportSpec struct {
	DateRange            DateRange             `json:"dateRange"`
	Dimensions           []Dimension           `json:"dimensions,omitempty"`
	Metrics              []Metric              `json:"metrics"`
	DimensionFilters     []DimensionFilter     `json:"dimensionFilters,omitempty"`
	SortConditions       []SortCondition       `json:"sortConditions,omitempty"`
	LocalizationSettings *LocalizationSettings `json:"localizationSettings,omitempty"`
	TimeZone             string                `json:"timeZone,omitempty"`
	MaxReportRows        int32                 `json:"maxReportRows,omitempty"`
}

type MediationReportSpec struct {
	DateRange            DateRange             `json:"dateRange"`
	Dimensions           []Dimension           `json:"dimensions,omitempty"`
	Metrics              []Metric              `json:"metrics"`
	DimensionFilters     []DimensionFilter     `json:"dimensionFilters,omitempty"`
	SortConditions       []SortCondition       `json:"sortConditions,omitempty"`
	LocalizationSettings *LocalizationSettings `json:"localizationSettings,omitempty"`
	TimeZone             string                `json:"timeZone,omitempty"`
	MaxReportRows        int32                 `json:"maxReportRows,omitempty"`
}

type GenerateNetworkReportRequest struct {
	ReportSpec NetworkReportSpec `json:"reportSpec"`
}

type GenerateMediationReportRequest struct {
	ReportSpec MediationReportSpec `json:"reportSpec"`
}

type ReportHeader struct {
	DateRange            DateRange            `json:"dateRange"`
	LocalizationSettings LocalizationSettings `json:"localizationSettings"`
	ReportingTimeZone    string               `json:"reportingTimeZone"`
}

type DimensionValue struct {
	Value        string `json:"value"`
	DisplayLabel string `json:"displayLabel,omitempty"`
}

// MetricValue preserves the AdMob metric oneof as exactly one typed pointer.
type MetricValue struct {
	IntegerValue *int64          `json:"integerValue,omitempty,string"`
	MicrosValue  *int64          `json:"microsValue,omitempty,string"`
	DoubleValue  *float64        `json:"doubleValue,omitempty"`
	Raw          json.RawMessage `json:"-"`
}

func (value *MetricValue) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	if len(fields) != 1 {
		return fmt.Errorf("metric value must contain exactly one field")
	}
	result := MetricValue{Raw: append(json.RawMessage(nil), data...)}
	for name, raw := range fields {
		switch name {
		case "integerValue", "microsValue":
			var encoded string
			if err := json.Unmarshal(raw, &encoded); err != nil {
				return fmt.Errorf("%s must be a decimal string", name)
			}
			parsed, err := strconv.ParseInt(encoded, 10, 64)
			if err != nil {
				return fmt.Errorf("%s is outside int64", name)
			}
			if name == "integerValue" {
				result.IntegerValue = &parsed
			} else {
				result.MicrosValue = &parsed
			}
		case "doubleValue":
			var parsed float64
			if err := json.Unmarshal(raw, &parsed); err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
				return fmt.Errorf("doubleValue must be finite")
			}
			result.DoubleValue = &parsed
		default:
			return fmt.Errorf("unknown metric value field %q", name)
		}
	}
	*value = result
	return nil
}

type ReportRow struct {
	DimensionValues map[Dimension]DimensionValue `json:"dimensionValues,omitempty"`
	MetricValues    map[Metric]MetricValue       `json:"metricValues"`
}

type ReportWarningType string

const (
	ReportWarningUnspecified          ReportWarningType = "TYPE_UNSPECIFIED"
	ReportWarningBeforeTimeZoneChange ReportWarningType = "DATA_BEFORE_ACCOUNT_TIMEZONE_CHANGE"
	ReportWarningDataDelayed          ReportWarningType = "DATA_DELAYED"
	ReportWarningOther                ReportWarningType = "OTHER"
	ReportWarningCurrencyDiffers      ReportWarningType = "REPORT_CURRENCY_NOT_ACCOUNT_CURRENCY"
)

type ReportWarning struct {
	Type        ReportWarningType `json:"type"`
	Description string            `json:"description"`
}

type ReportFooter struct {
	Warnings         []ReportWarning `json:"warnings,omitempty"`
	MatchingRowCount int64           `json:"matchingRowCount,string"`
	Raw              json.RawMessage `json:"-"`
	matchingPresent  bool
}

func (footer *ReportFooter) UnmarshalJSON(data []byte) error {
	var wire struct {
		Warnings         []ReportWarning `json:"warnings"`
		MatchingRowCount *string         `json:"matchingRowCount"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	if wire.MatchingRowCount == nil {
		return fmt.Errorf("matchingRowCount is missing")
	}
	count, err := strconv.ParseInt(*wire.MatchingRowCount, 10, 64)
	if err != nil || count < 0 {
		return fmt.Errorf("matchingRowCount is invalid")
	}
	*footer = ReportFooter{
		Warnings: wire.Warnings, MatchingRowCount: count,
		Raw: append(json.RawMessage(nil), data...), matchingPresent: true,
	}
	return nil
}

type Report struct {
	Header ReportHeader `json:"header"`
	Rows   []ReportRow  `json:"rows"`
	Footer ReportFooter `json:"footer"`
}

func captureRaw(data []byte, target any, raw *json.RawMessage) error {
	if err := json.Unmarshal(data, target); err != nil {
		return err
	}
	*raw = append((*raw)[:0], data...)
	return nil
}

func (value *PublisherAccount) UnmarshalJSON(data []byte) error {
	type alias PublisherAccount
	return captureRaw(data, (*alias)(value), &value.Raw)
}

func (value *App) UnmarshalJSON(data []byte) error {
	type alias App
	return captureRaw(data, (*alias)(value), &value.Raw)
}

func (value *AdUnit) UnmarshalJSON(data []byte) error {
	type alias AdUnit
	return captureRaw(data, (*alias)(value), &value.Raw)
}

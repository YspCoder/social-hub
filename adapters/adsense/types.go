package adsense

import (
	"encoding/json"
	"strconv"
)

// ListRequest controls AdSense collection pagination.
type ListRequest struct {
	PageSize  int32
	PageToken string
}

type Page[T any] struct {
	Items         []T
	NextPageToken string
}

type Date struct {
	Year  int32 `json:"year,omitempty"`
	Month int32 `json:"month,omitempty"`
	Day   int32 `json:"day,omitempty"`
}

type TimeZone struct {
	ID      string `json:"id,omitempty"`
	Version string `json:"version,omitempty"`
}

type AccountState string

const (
	AccountReady          AccountState = "READY"
	AccountNeedsAttention AccountState = "NEEDS_ATTENTION"
	AccountClosed         AccountState = "CLOSED"
)

type Account struct {
	Name         string          `json:"name"`
	DisplayName  string          `json:"displayName,omitempty"`
	Premium      bool            `json:"premium,omitempty"`
	TimeZone     TimeZone        `json:"timeZone,omitempty"`
	CreateTime   string          `json:"createTime,omitempty"`
	PendingTasks []string        `json:"pendingTasks,omitempty"`
	State        AccountState    `json:"state,omitempty"`
	Raw          json.RawMessage `json:"-"`
}

type AdBlockingRecoveryTag struct {
	Tag                 string          `json:"tag,omitempty"`
	ErrorProtectionCode string          `json:"errorProtectionCode,omitempty"`
	Raw                 json.RawMessage `json:"-"`
}

type AdClientState string

const (
	AdClientReady          AdClientState = "READY"
	AdClientGettingReady   AdClientState = "GETTING_READY"
	AdClientRequiresReview AdClientState = "REQUIRES_REVIEW"
)

type AdClient struct {
	Name                 string          `json:"name"`
	ReportingDimensionID string          `json:"reportingDimensionId,omitempty"`
	ProductCode          string          `json:"productCode,omitempty"`
	State                AdClientState   `json:"state,omitempty"`
	Raw                  json.RawMessage `json:"-"`
}

type AdClientAdCode struct {
	AdCode  string          `json:"adCode,omitempty"`
	AMPHead string          `json:"ampHead,omitempty"`
	AMPBody string          `json:"ampBody,omitempty"`
	Raw     json.RawMessage `json:"-"`
}

type ContentAdType string

const (
	ContentAdDisplay        ContentAdType = "DISPLAY"
	ContentAdFeed           ContentAdType = "FEED"
	ContentAdArticle        ContentAdType = "ARTICLE"
	ContentAdMatchedContent ContentAdType = "MATCHED_CONTENT"
	ContentAdLink           ContentAdType = "LINK"
)

type ContentAdsSettings struct {
	Size string        `json:"size,omitempty"`
	Type ContentAdType `json:"type,omitempty"`
}

type AdUnitState string

const (
	AdUnitActive   AdUnitState = "ACTIVE"
	AdUnitArchived AdUnitState = "ARCHIVED"
)

type AdUnit struct {
	Name                 string             `json:"name"`
	ReportingDimensionID string             `json:"reportingDimensionId,omitempty"`
	DisplayName          string             `json:"displayName,omitempty"`
	State                AdUnitState        `json:"state,omitempty"`
	ContentAdsSettings   ContentAdsSettings `json:"contentAdsSettings,omitempty"`
	Raw                  json.RawMessage    `json:"-"`
}

type AdUnitAdCode struct {
	AdCode string          `json:"adCode,omitempty"`
	Raw    json.RawMessage `json:"-"`
}

type CustomChannel struct {
	Name                 string          `json:"name"`
	ReportingDimensionID string          `json:"reportingDimensionId,omitempty"`
	DisplayName          string          `json:"displayName,omitempty"`
	Active               bool            `json:"active,omitempty"`
	Raw                  json.RawMessage `json:"-"`
}

type URLChannel struct {
	Name                 string          `json:"name"`
	ReportingDimensionID string          `json:"reportingDimensionId,omitempty"`
	URIPattern           string          `json:"uriPattern,omitempty"`
	Raw                  json.RawMessage `json:"-"`
}

type SiteState string

const (
	SiteRequiresReview SiteState = "REQUIRES_REVIEW"
	SiteGettingReady   SiteState = "GETTING_READY"
	SiteReady          SiteState = "READY"
	SiteNeedsAttention SiteState = "NEEDS_ATTENTION"
)

type Site struct {
	Name                 string          `json:"name"`
	ReportingDimensionID string          `json:"reportingDimensionId,omitempty"`
	Domain               string          `json:"domain,omitempty"`
	State                SiteState       `json:"state,omitempty"`
	AutoAdsEnabled       bool            `json:"autoAdsEnabled,omitempty"`
	Raw                  json.RawMessage `json:"-"`
}

type AlertSeverity string

const (
	AlertInfo    AlertSeverity = "INFO"
	AlertWarning AlertSeverity = "WARNING"
	AlertSevere  AlertSeverity = "SEVERE"
)

type Alert struct {
	Name     string          `json:"name"`
	Severity AlertSeverity   `json:"severity,omitempty"`
	Message  string          `json:"message,omitempty"`
	Type     string          `json:"type,omitempty"`
	Raw      json.RawMessage `json:"-"`
}

type Payment struct {
	Name   string          `json:"name"`
	Date   Date            `json:"date,omitempty"`
	Amount string          `json:"amount,omitempty"`
	Raw    json.RawMessage `json:"-"`
}

type PolicyEntityType string

const (
	PolicyEntitySite        PolicyEntityType = "SITE"
	PolicyEntitySiteSection PolicyEntityType = "SITE_SECTION"
	PolicyEntityPage        PolicyEntityType = "PAGE"
)

type EnforcementAction string

const (
	EnforcementWarned                   EnforcementAction = "WARNED"
	EnforcementAdServingRestricted      EnforcementAction = "AD_SERVING_RESTRICTED"
	EnforcementAdServingDisabled        EnforcementAction = "AD_SERVING_DISABLED"
	EnforcementClickConfirmation        EnforcementAction = "AD_SERVED_WITH_CLICK_CONFIRMATION"
	EnforcementAdPersonalizationLimited EnforcementAction = "AD_PERSONALIZATION_RESTRICTED"
)

type PolicyTopicType string

const (
	PolicyTopicPolicy               PolicyTopicType = "POLICY"
	PolicyTopicAdvertiserPreference PolicyTopicType = "ADVERTISER_PREFERENCE"
	PolicyTopicRegulatory           PolicyTopicType = "REGULATORY"
)

type PolicyTopic struct {
	Topic   string          `json:"topic,omitempty"`
	Type    PolicyTopicType `json:"type,omitempty"`
	MustFix bool            `json:"mustFix,omitempty"`
}

type PolicyIssue struct {
	Name                  string            `json:"name"`
	EntityType            PolicyEntityType  `json:"entityType,omitempty"`
	Site                  string            `json:"site,omitempty"`
	SiteSection           string            `json:"siteSection,omitempty"`
	URI                   string            `json:"uri,omitempty"`
	AdClients             []string          `json:"adClients,omitempty"`
	PolicyTopics          []PolicyTopic     `json:"policyTopics,omitempty"`
	AdRequestCount        string            `json:"adRequestCount,omitempty"`
	Action                EnforcementAction `json:"action,omitempty"`
	FirstDetectedDate     Date              `json:"firstDetectedDate,omitempty"`
	LastDetectedDate      Date              `json:"lastDetectedDate,omitempty"`
	WarningEscalationDate Date              `json:"warningEscalationDate,omitempty"`
	Raw                   json.RawMessage   `json:"-"`
}

type SavedReport struct {
	Name  string          `json:"name"`
	Title string          `json:"title,omitempty"`
	Raw   json.RawMessage `json:"-"`
}

type Dimension string
type Metric string

const (
	DimensionDate              Dimension = "DATE"
	DimensionWeek              Dimension = "WEEK"
	DimensionMonth             Dimension = "MONTH"
	DimensionAccountName       Dimension = "ACCOUNT_NAME"
	DimensionAdClientID        Dimension = "AD_CLIENT_ID"
	DimensionProductName       Dimension = "PRODUCT_NAME"
	DimensionProductCode       Dimension = "PRODUCT_CODE"
	DimensionAdUnitName        Dimension = "AD_UNIT_NAME"
	DimensionAdUnitID          Dimension = "AD_UNIT_ID"
	DimensionCustomChannelName Dimension = "CUSTOM_CHANNEL_NAME"
	DimensionCustomChannelID   Dimension = "CUSTOM_CHANNEL_ID"
	DimensionOwnedSiteDomain   Dimension = "OWNED_SITE_DOMAIN_NAME"
	DimensionOwnedSiteID       Dimension = "OWNED_SITE_ID"
	DimensionPageURL           Dimension = "PAGE_URL"
	DimensionURLChannelName    Dimension = "URL_CHANNEL_NAME"
	DimensionURLChannelID      Dimension = "URL_CHANNEL_ID"
	DimensionCountryName       Dimension = "COUNTRY_NAME"
	DimensionCountryCode       Dimension = "COUNTRY_CODE"
	DimensionPlatformTypeName  Dimension = "PLATFORM_TYPE_NAME"
	DimensionPlatformTypeCode  Dimension = "PLATFORM_TYPE_CODE"
)

const (
	MetricPageViews               Metric = "PAGE_VIEWS"
	MetricAdRequests              Metric = "AD_REQUESTS"
	MetricMatchedAdRequests       Metric = "MATCHED_AD_REQUESTS"
	MetricTotalImpressions        Metric = "TOTAL_IMPRESSIONS"
	MetricImpressions             Metric = "IMPRESSIONS"
	MetricIndividualAdImpressions Metric = "INDIVIDUAL_AD_IMPRESSIONS"
	MetricClicks                  Metric = "CLICKS"
	MetricAdRequestsCoverage      Metric = "AD_REQUESTS_COVERAGE"
	MetricPageViewsCTR            Metric = "PAGE_VIEWS_CTR"
	MetricAdRequestsCTR           Metric = "AD_REQUESTS_CTR"
	MetricMatchedAdRequestsCTR    Metric = "MATCHED_AD_REQUESTS_CTR"
	MetricImpressionsCTR          Metric = "IMPRESSIONS_CTR"
	MetricActiveViewMeasurability Metric = "ACTIVE_VIEW_MEASURABILITY"
	MetricActiveViewViewability   Metric = "ACTIVE_VIEW_VIEWABILITY"
	MetricEstimatedEarnings       Metric = "ESTIMATED_EARNINGS"
	MetricPageViewsRPM            Metric = "PAGE_VIEWS_RPM"
	MetricAdRequestsRPM           Metric = "AD_REQUESTS_RPM"
	MetricMatchedAdRequestsRPM    Metric = "MATCHED_AD_REQUESTS_RPM"
	MetricImpressionsRPM          Metric = "IMPRESSIONS_RPM"
	MetricCostPerClick            Metric = "COST_PER_CLICK"
	MetricTotalEarnings           Metric = "TOTAL_EARNINGS"
)

type ReportDateRange string

const (
	ReportDateCustom      ReportDateRange = "CUSTOM"
	ReportDateToday       ReportDateRange = "TODAY"
	ReportDateYesterday   ReportDateRange = "YESTERDAY"
	ReportDateMonthToDate ReportDateRange = "MONTH_TO_DATE"
	ReportDateYearToDate  ReportDateRange = "YEAR_TO_DATE"
	ReportDateLast7Days   ReportDateRange = "LAST_7_DAYS"
	ReportDateLast30Days  ReportDateRange = "LAST_30_DAYS"
)

type ReportingTimeZone string

const (
	ReportingTimeZoneAccount ReportingTimeZone = "ACCOUNT_TIME_ZONE"
	ReportingTimeZoneGoogle  ReportingTimeZone = "GOOGLE_TIME_ZONE"
)

type GenerateReportRequest struct {
	Dimensions        []Dimension
	Metrics           []Metric
	DateRange         ReportDateRange
	StartDate         Date
	EndDate           Date
	ReportingTimeZone ReportingTimeZone
	CurrencyCode      string
	LanguageCode      string
	Filters           []string
	OrderBy           []string
	Limit             int32
}

type GenerateSavedReportRequest struct {
	DateRange         ReportDateRange
	StartDate         Date
	EndDate           Date
	ReportingTimeZone ReportingTimeZone
	CurrencyCode      string
	LanguageCode      string
}

type HeaderType string

const (
	HeaderDimension          HeaderType = "DIMENSION"
	HeaderMetricTally        HeaderType = "METRIC_TALLY"
	HeaderMetricRatio        HeaderType = "METRIC_RATIO"
	HeaderMetricCurrency     HeaderType = "METRIC_CURRENCY"
	HeaderMetricMilliseconds HeaderType = "METRIC_MILLISECONDS"
	HeaderMetricDecimal      HeaderType = "METRIC_DECIMAL"
)

type ReportHeader struct {
	Name         string     `json:"name,omitempty"`
	Type         HeaderType `json:"type,omitempty"`
	CurrencyCode string     `json:"currencyCode,omitempty"`
}

type ReportCell struct {
	Value string `json:"value,omitempty"`
}

type ReportRow struct {
	Cells []ReportCell `json:"cells,omitempty"`
}

type ReportResult struct {
	Headers          []ReportHeader  `json:"headers,omitempty"`
	Rows             []ReportRow     `json:"rows,omitempty"`
	Totals           *ReportRow      `json:"totals,omitempty"`
	Averages         *ReportRow      `json:"averages,omitempty"`
	Warnings         []string        `json:"warnings,omitempty"`
	StartDate        Date            `json:"startDate,omitempty"`
	EndDate          Date            `json:"endDate,omitempty"`
	TotalMatchedRows int64           `json:"totalMatchedRows"`
	Raw              json.RawMessage `json:"-"`
	totalRowsPresent bool
}

func (result *ReportResult) UnmarshalJSON(data []byte) error {
	var wire struct {
		Headers          []ReportHeader `json:"headers"`
		Rows             []ReportRow    `json:"rows"`
		Totals           *ReportRow     `json:"totals"`
		Averages         *ReportRow     `json:"averages"`
		Warnings         []string       `json:"warnings"`
		StartDate        Date           `json:"startDate"`
		EndDate          Date           `json:"endDate"`
		TotalMatchedRows *string        `json:"totalMatchedRows"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	var total int64
	if wire.TotalMatchedRows != nil {
		parsed, err := strconv.ParseInt(*wire.TotalMatchedRows, 10, 64)
		if err != nil {
			return err
		}
		total = parsed
	}
	*result = ReportResult{
		Headers: wire.Headers, Rows: wire.Rows, Totals: wire.Totals, Averages: wire.Averages,
		Warnings: wire.Warnings, StartDate: wire.StartDate, EndDate: wire.EndDate,
		TotalMatchedRows: total, Raw: append(json.RawMessage(nil), data...), totalRowsPresent: wire.TotalMatchedRows != nil,
	}
	return nil
}

func captureRaw(data []byte, target any, raw *json.RawMessage) error {
	if err := json.Unmarshal(data, target); err != nil {
		return err
	}
	*raw = append((*raw)[:0], data...)
	return nil
}

func (value *Account) UnmarshalJSON(data []byte) error {
	type alias Account
	return captureRaw(data, (*alias)(value), &value.Raw)
}
func (value *AdBlockingRecoveryTag) UnmarshalJSON(data []byte) error {
	type alias AdBlockingRecoveryTag
	return captureRaw(data, (*alias)(value), &value.Raw)
}
func (value *AdClient) UnmarshalJSON(data []byte) error {
	type alias AdClient
	return captureRaw(data, (*alias)(value), &value.Raw)
}
func (value *AdClientAdCode) UnmarshalJSON(data []byte) error {
	type alias AdClientAdCode
	return captureRaw(data, (*alias)(value), &value.Raw)
}
func (value *AdUnit) UnmarshalJSON(data []byte) error {
	type alias AdUnit
	return captureRaw(data, (*alias)(value), &value.Raw)
}
func (value *AdUnitAdCode) UnmarshalJSON(data []byte) error {
	type alias AdUnitAdCode
	return captureRaw(data, (*alias)(value), &value.Raw)
}
func (value *CustomChannel) UnmarshalJSON(data []byte) error {
	type alias CustomChannel
	return captureRaw(data, (*alias)(value), &value.Raw)
}
func (value *URLChannel) UnmarshalJSON(data []byte) error {
	type alias URLChannel
	return captureRaw(data, (*alias)(value), &value.Raw)
}
func (value *Site) UnmarshalJSON(data []byte) error {
	type alias Site
	return captureRaw(data, (*alias)(value), &value.Raw)
}
func (value *Alert) UnmarshalJSON(data []byte) error {
	type alias Alert
	return captureRaw(data, (*alias)(value), &value.Raw)
}
func (value *Payment) UnmarshalJSON(data []byte) error {
	type alias Payment
	return captureRaw(data, (*alias)(value), &value.Raw)
}
func (value *PolicyIssue) UnmarshalJSON(data []byte) error {
	type alias PolicyIssue
	return captureRaw(data, (*alias)(value), &value.Raw)
}
func (value *SavedReport) UnmarshalJSON(data []byte) error {
	type alias SavedReport
	return captureRaw(data, (*alias)(value), &value.Raw)
}

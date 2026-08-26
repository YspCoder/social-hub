package moloco

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"social-hub/pkg/socialhub"
)

const maxProviderObjectBytes = 8 << 20

// ResponseMeta preserves Moloco's documented account-level quota headers.
type ResponseMeta struct {
	RequestID          string
	RateLimitQuota     string
	RateLimitRemaining string
	RateLimitReset     string
}

type ReportDimension string

const (
	DimensionUnknown       ReportDimension = "UNKNOWN_REPORT_DIMENSION"
	DimensionDate          ReportDimension = "DATE"
	DimensionAppOrSite     ReportDimension = "APP_OR_SITE"
	DimensionCampaign      ReportDimension = "CAMPAIGN"
	DimensionAdGroup       ReportDimension = "AD_GROUP"
	DimensionCreativeGroup ReportDimension = "CREATIVE_GROUP"
	DimensionCreative      ReportDimension = "CREATIVE"
	DimensionExchange      ReportDimension = "EXCHANGE"
	DimensionSubPublisher  ReportDimension = "SUB_PUBLISHER"
	DimensionTraffic       ReportDimension = "TRAFFIC"
	DimensionSKAN          ReportDimension = "SKAN"
)

type OptionalMetric string

const (
	OptionalMetricUnknown           OptionalMetric = "UNKNOWN_REPORT_METRIC"
	OptionalMetricVideoPlayProgress OptionalMetric = "VIDEO_PLAY_PROGRESS"
	OptionalMetricEngagedViews      OptionalMetric = "ENGAGED_VIEWS"
	OptionalMetricEngagedClicks     OptionalMetric = "ENGAGED_CLICKS"
)

type DateInterval struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type Report struct {
	ID              string            `json:"id"`
	AdAccountID     string            `json:"ad_account_id"`
	ProductID       string            `json:"product_id,omitempty"`
	DateRange       DateInterval      `json:"date_range"`
	Dimensions      []ReportDimension `json:"dimensions"`
	OptionalMetrics []OptionalMetric  `json:"optional_metrics,omitempty"`
	ExpireAt        *time.Time        `json:"expire_at,omitempty"`
	CreatedAt       *time.Time        `json:"created_at,omitempty"`
	UpdatedAt       *time.Time        `json:"updated_at,omitempty"`
	Meta            ResponseMeta      `json:"-"`
	Raw             json.RawMessage   `json:"-"`
}

func (value *Report) UnmarshalJSON(data []byte) error {
	type wire Report
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Report(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type ListReportsResponse struct {
	Reports []Report        `json:"reports"`
	Meta    ResponseMeta    `json:"-"`
	Raw     json.RawMessage `json:"-"`
}

func (value *ListReportsResponse) UnmarshalJSON(data []byte) error {
	type wire ListReportsResponse
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = ListReportsResponse(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type ReportStatus string

const (
	ReportStatusUnknown  ReportStatus = "UNKNOWN_REPORT_STATUS"
	ReportStatusAccepted ReportStatus = "ACCEPTED"
	ReportStatusReady    ReportStatus = "READY"
	ReportStatusFailed   ReportStatus = "FAILED"
)

type ReportStatusResponse struct {
	ID           string       `json:"id,omitempty"`
	Status       ReportStatus `json:"status"`
	LocationJSON string       `json:"location_json,omitempty"`
	LocationCSV  string       `json:"location_csv,omitempty"`
	Meta         ResponseMeta `json:"-"`
}

func (value *ReportStatusResponse) UnmarshalJSON(data []byte) error {
	type wire ReportStatusResponse
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = ReportStatusResponse(decoded)
	return nil
}

// String prevents presigned report locations from entering ordinary logs.
func (ReportStatusResponse) String() string {
	return "moloco.ReportStatusResponse(<redacted locations>)"
}

// GoString prevents presigned report locations from entering Go-syntax logs.
func (ReportStatusResponse) GoString() string {
	return "moloco.ReportStatusResponse(<redacted locations>)"
}

// ReportWorkflow is the bounded, read-only Moloco Report API surface. The
// official list endpoint defines no pagination parameters.
type ReportWorkflow interface {
	ListReports(context.Context, ...socialhub.CallOption) (ListReportsResponse, error)
	GetReport(context.Context, string, ...socialhub.CallOption) (Report, error)
	GetReportStatus(context.Context, string, ...socialhub.CallOption) (ReportStatusResponse, error)
}

func decodeProviderObject(data []byte, target any) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxProviderObjectBytes || trimmed[0] != '{' || !json.Valid(trimmed) {
		return fmt.Errorf("moloco: invalid provider object")
	}
	return json.Unmarshal(trimmed, target)
}

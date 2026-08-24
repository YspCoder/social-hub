package lineads

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"social-hub/pkg/socialhub"
)

const (
	maxExactValueBytes     = 256
	maxRawValueBytes       = 1 << 20
	maxProviderObjectBytes = 8 << 20
)

// PartnerType is an entitlement published by the current LINE Ads docs.
type PartnerType string

const (
	PartnerDataGeneral            PartnerType = "data-general-partner"
	PartnerCertifiedAdTechGeneral PartnerType = "certificated-ad-tech-general-partner"
	PartnerReportingGeneral       PartnerType = "reporting-general-partner"
)

type SortDirection string

const (
	SortAscending  SortDirection = "asc"
	SortDescending SortDirection = "desc"
)

// Sort represents LINE Ads' property(,asc|desc) query syntax.
type Sort struct {
	Field     string
	Direction SortDirection
}

// Date is a YYYY-MM-DD calendar date.
type Date string

type ReportLevel string

const (
	ReportLevelCampaign ReportLevel = "campaign"
	ReportLevelAdGroup  ReportLevel = "adgroup"
	ReportLevelAd       ReportLevel = "ad"
)

type ResponseMeta struct {
	RequestQuotaLimit string
	RequestQuotaUsed  string
}

type Paging struct {
	Page          int      `json:"page"`
	Size          int      `json:"size"`
	TotalElements int64    `json:"totalElements"`
	Sorts         []string `json:"sorts"`
}

// ExactValue preserves a provider JSON string, number, or null without
// float64 coercion.
type ExactValue struct {
	raw json.RawMessage
}

func (value *ExactValue) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxExactValueBytes || !json.Valid(trimmed) {
		return fmt.Errorf("lineads: invalid exact value")
	}
	first := trimmed[0]
	if first != '"' && first != '-' && (first < '0' || first > '9') && !bytes.Equal(trimmed, []byte("null")) {
		return fmt.Errorf("lineads: exact value must be a JSON string, number, or null")
	}
	value.raw = append(value.raw[:0], trimmed...)
	return nil
}

func (value ExactValue) MarshalJSON() ([]byte, error) {
	if len(value.raw) == 0 {
		return []byte("null"), nil
	}
	return append([]byte(nil), value.raw...), nil
}

func (value ExactValue) Bytes() []byte { return append([]byte(nil), value.raw...) }
func (value ExactValue) IsSet() bool   { return len(value.raw) > 0 }
func (value ExactValue) IsNull() bool  { return bytes.Equal(bytes.TrimSpace(value.raw), []byte("null")) }

func (value ExactValue) String() string {
	trimmed := bytes.TrimSpace(value.raw)
	if len(trimmed) > 0 && trimmed[0] == '"' {
		var text string
		if json.Unmarshal(trimmed, &text) == nil {
			return text
		}
	}
	return string(trimmed)
}

func (value ExactValue) Decode(target any) error {
	if target == nil || len(value.raw) == 0 {
		return fmt.Errorf("lineads: decode target and exact value are required")
	}
	return json.Unmarshal(value.raw, target)
}

// RawValue preserves one dynamic online-report metric without coercion.
type RawValue struct {
	raw json.RawMessage
}

func (value *RawValue) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxRawValueBytes || !json.Valid(trimmed) {
		return fmt.Errorf("lineads: invalid report value")
	}
	value.raw = append(value.raw[:0], trimmed...)
	return nil
}

func (value RawValue) MarshalJSON() ([]byte, error) {
	if len(value.raw) == 0 {
		return []byte("null"), nil
	}
	return append([]byte(nil), value.raw...), nil
}

func (value RawValue) Bytes() []byte { return append([]byte(nil), value.raw...) }
func (value RawValue) IsNull() bool  { return bytes.Equal(bytes.TrimSpace(value.raw), []byte("null")) }

func (value RawValue) String() string {
	trimmed := bytes.TrimSpace(value.raw)
	if len(trimmed) > 0 && trimmed[0] == '"' {
		var text string
		if json.Unmarshal(trimmed, &text) == nil {
			return text
		}
	}
	return string(trimmed)
}

func (value RawValue) Decode(target any) error {
	if target == nil || len(value.raw) == 0 {
		return fmt.Errorf("lineads: decode target and report value are required")
	}
	return json.Unmarshal(value.raw, target)
}

type ListAdAccountsRequest struct {
	IncludeLinked  *bool
	IncludeRemoved *bool
	Name           string
	Page           int
	Size           int
	Sort           Sort
}

type ListCampaignsRequest struct {
	AdAccountID    string
	IDs            []int64
	IncludeRemoved *bool
	Page           int
	Size           int
	Sort           Sort
}

type ListPerformanceReportsRequest struct {
	AdAccountID string
	IDs         []int64
	Page        int
	Size        int
	Sort        Sort
}

type GetOnlineReportRequest struct {
	AdAccountID    string
	Level          ReportLevel
	AdGroupID      int64
	CampaignID     int64
	IncludeRemoved *bool
	LandingPageID  int64
	Page           int
	SearchKey      string
	Since          Date
	Size           int
	Until          Date
}

// ManagementWorkflow is the bounded, read-only LINE Ads API v3 surface.
type ManagementWorkflow interface {
	ListAdAccounts(context.Context, ListAdAccountsRequest, ...socialhub.CallOption) (AdAccountsResponse, error)
	ListCampaigns(context.Context, ListCampaignsRequest, ...socialhub.CallOption) (CampaignsResponse, error)
	ListPerformanceReports(context.Context, ListPerformanceReportsRequest, ...socialhub.CallOption) (PerformanceReportsResponse, error)
	GetOnlineReport(context.Context, GetOnlineReportRequest, ...socialhub.CallOption) (OnlineReportResponse, error)
}

type LineAccount struct {
	Name   string `json:"name"`
	LineID string `json:"lineId"`
}

type DeliveryStatusReason struct {
	Code string `json:"code"`
}

type AdAccount struct {
	ID                          ExactValue             `json:"id"`
	Name                        string                 `json:"name"`
	ConfiguredStatus            string                 `json:"configuredStatus"`
	ProductType                 string                 `json:"productType"`
	AvailableCampaignObjectives []string               `json:"availableCampaignObjective"`
	Currency                    string                 `json:"currency"`
	Timezone                    string                 `json:"timezone"`
	Country                     string                 `json:"country"`
	LineAccount                 *LineAccount           `json:"lineAccount"`
	DeliveryStatus              string                 `json:"deliveryStatus"`
	DeliveryStatusReasons       []DeliveryStatusReason `json:"deliveryStatusReasons"`
	CreatedDate                 string                 `json:"createdDate"`
	ModifiedDate                string                 `json:"modifiedDate"`
	RemovedDate                 string                 `json:"removedDate"`
	Raw                         json.RawMessage        `json:"-"`
}

func (value *AdAccount) UnmarshalJSON(data []byte) error {
	type wire AdAccount
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = AdAccount(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type AdAccountsResponse struct {
	Paging     Paging          `json:"paging"`
	AdAccounts []AdAccount     `json:"datas"`
	Meta       ResponseMeta    `json:"-"`
	Raw        json.RawMessage `json:"-"`
}

func (value *AdAccountsResponse) UnmarshalJSON(data []byte) error {
	type wire AdAccountsResponse
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = AdAccountsResponse(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type Campaign struct {
	ID                    ExactValue             `json:"id"`
	Name                  string                 `json:"name"`
	CampaignObjective     string                 `json:"campaignObjective"`
	ConfiguredStatus      string                 `json:"configuredStatus"`
	SpendingLimitType     string                 `json:"spendingLimitType"`
	SpendingLimitMicro    ExactValue             `json:"spendingLimitMicro"`
	StartDate             string                 `json:"startDate"`
	EndDate               string                 `json:"endDate"`
	ActiveCBO             bool                   `json:"activeCbo"`
	BidStrategy           string                 `json:"bidStrategy"`
	DailyBudgetMicro      ExactValue             `json:"dailyBudgetMicro"`
	DeliveryStatus        string                 `json:"deliveryStatus"`
	DeliveryStatusReasons []DeliveryStatusReason `json:"deliveryStatusReasons"`
	CreatedDate           string                 `json:"createdDate"`
	ModifiedDate          string                 `json:"modifiedDate"`
	RemovedDate           string                 `json:"removedDate"`
	Raw                   json.RawMessage        `json:"-"`
}

func (value *Campaign) UnmarshalJSON(data []byte) error {
	type wire Campaign
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Campaign(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type CampaignsResponse struct {
	Paging    Paging          `json:"paging"`
	Campaigns []Campaign      `json:"datas"`
	Meta      ResponseMeta    `json:"-"`
	Raw       json.RawMessage `json:"-"`
}

func (value *CampaignsResponse) UnmarshalJSON(data []byte) error {
	type wire CampaignsResponse
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = CampaignsResponse(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type ReportQueryParams struct {
	Level            string           `json:"level"`
	Since            string           `json:"since"`
	Until            string           `json:"until"`
	Breakdown        *ReportBreakdown `json:"breakdown"`
	Filtering        *ReportFiltering `json:"filtering"`
	IncludeRemove    bool             `json:"includeRemove"`
	OnlyCarouselSlot bool             `json:"onlyCarouselSlot"`
	FileFormat       string           `json:"fileFormat"`
	Locale           string           `json:"locale"`
}

type ReportBreakdown struct {
	Delivery       string `json:"delivery"`
	Time           string `json:"time"`
	ByServiceGroup bool   `json:"byServiceGroup"`
	Specific       string `json:"specific"`
}

type ReportFiltering struct {
	IDType string   `json:"idType"`
	IDs    []string `json:"ids"`
}

type PerformanceReport struct {
	ID          ExactValue         `json:"id"`
	Name        string             `json:"name"`
	Status      string             `json:"status"`
	QueryParams *ReportQueryParams `json:"queryParams"`
	Raw         json.RawMessage    `json:"-"`
}

func (value *PerformanceReport) UnmarshalJSON(data []byte) error {
	type wire PerformanceReport
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = PerformanceReport(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type PerformanceReportsResponse struct {
	Paging  Paging              `json:"paging"`
	Reports []PerformanceReport `json:"datas"`
	Meta    ResponseMeta        `json:"-"`
	Raw     json.RawMessage     `json:"-"`
}

func (value *PerformanceReportsResponse) UnmarshalJSON(data []byte) error {
	type wire PerformanceReportsResponse
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = PerformanceReportsResponse(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

// ReportEntity exposes fields common to partial Ad Group and Ad objects while
// retaining the complete provider object in Raw.
type ReportEntity struct {
	ID               ExactValue      `json:"id"`
	Name             string          `json:"name"`
	AdAccountID      ExactValue      `json:"adaccountId"`
	CampaignID       ExactValue      `json:"campaignId"`
	AdGroupID        ExactValue      `json:"adgroupId"`
	ConfiguredStatus string          `json:"configuredStatus"`
	DeliveryStatus   string          `json:"deliveryStatus"`
	Raw              json.RawMessage `json:"-"`
}

func (value *ReportEntity) UnmarshalJSON(data []byte) error {
	type wire ReportEntity
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = ReportEntity(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type OnlineReportRow struct {
	AdAccount  *AdAccount          `json:"adaccount"`
	Campaign   *Campaign           `json:"campaign"`
	AdGroup    *ReportEntity       `json:"adgroup"`
	Ad         *ReportEntity       `json:"ad"`
	Statistics map[string]RawValue `json:"statistics"`
	Raw        json.RawMessage     `json:"-"`
}

func (value *OnlineReportRow) UnmarshalJSON(data []byte) error {
	type wire OnlineReportRow
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = OnlineReportRow(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type TimeRange struct {
	Since string `json:"since"`
	Until string `json:"until"`
}

type OnlineReportResponse struct {
	Paging    Paging            `json:"paging"`
	Rows      []OnlineReportRow `json:"datas"`
	TimeRange TimeRange         `json:"timeRange"`
	Meta      ResponseMeta      `json:"-"`
	Raw       json.RawMessage   `json:"-"`
}

func (value *OnlineReportResponse) UnmarshalJSON(data []byte) error {
	type wire OnlineReportResponse
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = OnlineReportResponse(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

func decodeProviderObject(data []byte, target any) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxProviderObjectBytes || trimmed[0] != '{' || !json.Valid(trimmed) {
		return fmt.Errorf("lineads: invalid provider object")
	}
	return json.Unmarshal(trimmed, target)
}

var _ ManagementWorkflow = (*Client)(nil)

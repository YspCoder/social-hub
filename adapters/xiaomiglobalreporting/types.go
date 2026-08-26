package xiaomiglobalreporting

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	maximumPageSize         = 1000
	maximumReportRows       = 1000
	maximumNameIDs          = 8000
	maximumReportValueBytes = 1 << 20
)

// TimestampUnit selects the epoch unit for Xiaomi's undocumented Long-valued
// timestamp Cookie. It must be configured explicitly.
type TimestampUnit string

const (
	TimestampUnixSeconds      TimestampUnit = "unix_seconds"
	TimestampUnixMilliseconds TimestampUnit = "unix_milliseconds"
)

// Date is one UTC Reporting API calendar day in YYYY-MM-DD form.
type Date string

func DateFromTime(value time.Time) Date { return Date(value.UTC().Format("2006-01-02")) }

type AdType int

const (
	AdTypeEffect AdType = 1
	AdTypeBrand  AdType = 2
)

type Dimension int

const (
	DimensionCampaign  Dimension = 1
	DimensionAdGroup   Dimension = 2
	DimensionAd        Dimension = 3
	DimensionRegion    Dimension = 4
	DimensionDate      Dimension = 5
	DimensionPlacement Dimension = 9
	DimensionPublisher Dimension = 10
)

type Language string

const (
	LanguageSimplifiedChinese Language = "zh_CN"
	LanguageEnglish           Language = "en_US"
)

// ReportQuery requests one page of daily Xiaomi Global delivery data. Empty
// AccountIDs are replaced with the configured account whitelist.
type ReportQuery struct {
	Page         int
	PageSize     int
	AdType       AdType
	AccountIDs   []int64
	CampaignIDs  []int64
	AdGroupIDs   []int64
	CreativeIDs  []int64
	PlacementIDs []string
	Regions      []string
	PublisherIDs []string
	Dimensions   []Dimension
	Begin        Date
	End          Date
	Language     Language
}

// NameQuery resolves account and optional campaign, ad group, and creative
// names. Empty AccountIDs are replaced with the configured whitelist.
type NameQuery struct {
	AccountIDs  []int64
	CampaignIDs []int64
	AdGroupIDs  []int64
	CreativeIDs []int64
}

// ReportValue preserves exact JSON instead of converting numbers through
// float64. Dynamic extension fields may also be decoded into caller types.
type ReportValue struct {
	raw json.RawMessage
}

func (value *ReportValue) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || len(data) > maximumReportValueBytes || !json.Valid(data) {
		return fmt.Errorf("xiaomiglobalreporting: invalid report value")
	}
	value.raw = append(value.raw[:0], data...)
	return nil
}

func (value ReportValue) MarshalJSON() ([]byte, error) {
	if len(value.raw) == 0 {
		return []byte("null"), nil
	}
	return append([]byte(nil), value.raw...), nil
}

func (value ReportValue) Bytes() []byte { return append([]byte(nil), value.raw...) }
func (value ReportValue) IsNull() bool {
	return bytes.Equal(bytes.TrimSpace(value.raw), []byte("null"))
}

func (value ReportValue) String() string {
	trimmed := bytes.TrimSpace(value.raw)
	if len(trimmed) > 0 && trimmed[0] == '"' {
		var text string
		if json.Unmarshal(trimmed, &text) == nil {
			return text
		}
	}
	return string(trimmed)
}

func (value ReportValue) Decode(target any) error {
	if target == nil || len(value.raw) == 0 {
		return fmt.Errorf("xiaomiglobalreporting: decode target and report value are required")
	}
	return json.Unmarshal(value.raw, target)
}

type ReportRow map[string]ReportValue

type ReportPage struct {
	Current    int64
	Pages      int64
	Size       int64
	Total      int64
	Records    []ReportRow
	RequestUID string
}

type CampaignName struct {
	ID   int64  `json:"adCampaignId"`
	Name string `json:"campaignName"`
}

type AdGroupName struct {
	ID   int64  `json:"adGroupId"`
	Name string `json:"groupName"`
}

type CreativeName struct {
	ID   int64  `json:"adCreativeId"`
	Name string `json:"creativeName"`
}

type AccountNames struct {
	AccountID   int64          `json:"accountId"`
	AccountName string         `json:"accountName"`
	Campaigns   []CampaignName `json:"adCampaigns"`
	AdGroups    []AdGroupName  `json:"adGroups"`
	Creatives   []CreativeName `json:"adCreatives"`
}

type NameDirectory struct {
	Accounts   []AccountNames
	RequestUID string
}

// TokenBundle contains the parsed Xiaomi credential expiries and the original
// date strings returned by the token endpoint.
type TokenBundle struct {
	Token             socialhub.Token
	ExpireDate        string
	RefreshExpireDate string
	RefreshExpiresAt  time.Time
}

func (TokenBundle) String() string {
	return "xiaomiglobalreporting.TokenBundle(<redacted credentials>)"
}
func (TokenBundle) GoString() string {
	return "xiaomiglobalreporting.TokenBundle(<redacted credentials>)"
}

// QuotaPolicy is metadata for a shared limiter. The documented intervals are
// strict lower bounds; this adapter does not sleep or keep an in-process quota.
type QuotaPolicy struct {
	ReportMinimumInterval time.Duration
	NameMinimumInterval   time.Duration
	StrictlyGreater       bool
}

func DefaultQuotaPolicy() QuotaPolicy {
	return QuotaPolicy{
		ReportMinimumInterval: 2 * time.Second,
		NameMinimumInterval:   300 * time.Millisecond,
		StrictlyGreater:       true,
	}
}

type ReportsWorkflow interface {
	Query(context.Context, ReportQuery, ...socialhub.CallOption) (ReportPage, error)
	QueryNames(context.Context, NameQuery, ...socialhub.CallOption) (NameDirectory, error)
}

var _ ReportsWorkflow = (*Client)(nil)

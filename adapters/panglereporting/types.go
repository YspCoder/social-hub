package panglereporting

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"

	"social-hub/pkg/socialhub"
)

const MaximumReportRows = 100_000

type Date string

func DateFromTime(value time.Time) Date { return Date(value.UTC().Format("2006-01-02")) }

type TimeZone int

const (
	TimeZoneUTC  TimeZone = 0
	TimeZoneUTC8 TimeZone = 8
)

type Currency string

const (
	CurrencyUSD Currency = "usd"
	CurrencyCNY Currency = "cny"
)

type Dimension string

const (
	DimensionUserID     Dimension = "user_id"
	DimensionSiteID     Dimension = "site_id"
	DimensionAdSlotType Dimension = "ad_slot_type"
	DimensionRegion     Dimension = "region"
	DimensionIsBidding  Dimension = "is_bidding"
)

type ID string

func (id *ID) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	var value string
	if len(trimmed) > 0 && trimmed[0] == '"' {
		if err := json.Unmarshal(trimmed, &value); err != nil {
			return err
		}
	} else {
		decoder := json.NewDecoder(bytes.NewReader(trimmed))
		decoder.UseNumber()
		var number json.Number
		if err := decoder.Decode(&number); err != nil {
			return fmt.Errorf("panglereporting: identifier must be a decimal string or number")
		}
		value = number.String()
	}
	if !validNumericID(value) {
		return fmt.Errorf("panglereporting: invalid identifier")
	}
	*id = ID(value)
	return nil
}

func (id ID) MarshalJSON() ([]byte, error) { return json.Marshal(string(id)) }

// Decimal preserves Pangle's JSON number representation without converting
// revenue, eCPM, or ratios through float64.
type Decimal string

func (decimal *Decimal) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	var value string
	if len(trimmed) > 0 && trimmed[0] == '"' {
		if err := json.Unmarshal(trimmed, &value); err != nil {
			return err
		}
	} else {
		value = string(trimmed)
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) || value == "" {
		return fmt.Errorf("panglereporting: invalid decimal")
	}
	*decimal = Decimal(value)
	return nil
}

func (decimal Decimal) String() string { return string(decimal) }

func (decimal Decimal) MarshalJSON() ([]byte, error) {
	if !validNonnegativeDecimal(decimal) {
		return nil, fmt.Errorf("panglereporting: invalid decimal")
	}
	return []byte(decimal), nil
}

type ReportRequest struct {
	Date       Date
	TimeZone   *TimeZone
	Currency   Currency
	Region     string
	AppIDs     []ID
	Dimensions []Dimension
}

type IncomeRow struct {
	Date             Date     `json:"date,omitempty"`
	TimeZone         TimeZone `json:"time_zone"`
	Currency         Currency `json:"currency"`
	Region           string   `json:"region,omitempty"`
	UserID           ID       `json:"user_id,omitempty"`
	SiteID           ID       `json:"site_id,omitempty"`
	AppID            ID       `json:"app_id,omitempty"`
	AppName          string   `json:"app_name,omitempty"`
	AdSlotID         ID       `json:"ad_slot_id,omitempty"`
	AdSlotType       int      `json:"ad_slot_type,omitempty"`
	PackageName      string   `json:"package_name,omitempty"`
	Requests         int64    `json:"request"`
	Returned         int64    `json:"return"`
	FillRate         Decimal  `json:"fill_rate,omitempty"`
	Impressions      int64    `json:"show"`
	Clicks           int64    `json:"click"`
	ClickRate        Decimal  `json:"click_rate,omitempty"`
	Revenue          Decimal  `json:"revenue,omitempty"`
	ECPM             Decimal  `json:"ecpm,omitempty"`
	MediaName        string   `json:"media_name,omitempty"`
	CodeName         string   `json:"code_name,omitempty"`
	OS               string   `json:"os,omitempty"`
	UseMediation     int      `json:"use_mediation,omitempty"`
	BiddingType      int      `json:"bidding_type,omitempty"`
	AdRequests       int64    `json:"ad_request"`
	Responses        int64    `json:"response"`
	AdFillRate       Decimal  `json:"ad_fill_rate,omitempty"`
	AdImpressionRate Decimal  `json:"ad_impression_rate,omitempty"`
	AppCodeType      int      `json:"app_code_type,omitempty"`
}

type Report struct {
	Date           Date
	Rows           []IncomeRow
	NoData         bool
	MayBeTruncated bool
}

type ReportsWorkflow interface {
	IncomeReport(context.Context, ReportRequest, ...socialhub.CallOption) (Report, error)
}

var _ ReportsWorkflow = (*Client)(nil)

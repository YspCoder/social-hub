package skimlinks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	maxExactValueBytes     = 256
	maxProviderObjectBytes = 8 << 20
)

// ExactValue preserves a provider JSON scalar without float64 coercion.
type ExactValue struct {
	raw json.RawMessage
}

func (value *ExactValue) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxExactValueBytes || !json.Valid(trimmed) {
		return fmt.Errorf("skimlinks: invalid exact value")
	}
	first := trimmed[0]
	if first != '"' && first != '-' && (first < '0' || first > '9') &&
		!bytes.Equal(trimmed, []byte("true")) && !bytes.Equal(trimmed, []byte("false")) &&
		!bytes.Equal(trimmed, []byte("null")) {
		return fmt.Errorf("skimlinks: exact value must be a JSON scalar")
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
		return fmt.Errorf("skimlinks: decode target and exact value are required")
	}
	return json.Unmarshal(value.raw, target)
}

// Date is a YYYY-MM-DD calendar date.
type Date string

func DateFromTime(value time.Time) Date { return Date(value.UTC().Format("2006-01-02")) }

type MerchantSortField string
type SortDirection string
type ReportSortDirection string
type CommissionSortField string
type CommissionStatus string
type CommissionType string
type ReportBy string
type TimePeriod string
type ReportSortField string
type PaymentType string

const (
	MerchantSortName                     MerchantSortField = "name"
	MerchantSortPartnerType              MerchantSortField = "partner_type"
	MerchantSortCalculatedCommissionRate MerchantSortField = "calculated_commission_rate"
	MerchantSortCalculatedECPC           MerchantSortField = "calculated_ecpc"
	MerchantSortPopularity               MerchantSortField = "popularity"
	SortAscending                        SortDirection     = "asc"
	SortDescending                       SortDirection     = "desc"

	ReportSortAscending  ReportSortDirection = "ASC"
	ReportSortDescending ReportSortDirection = "DESC"

	CommissionSortID              CommissionSortField = "id"
	CommissionSortTransactionDate CommissionSortField = "transaction_date"

	CommissionActive    CommissionStatus = "active"
	CommissionCancelled CommissionStatus = "cancelled"

	CommissionCPA         CommissionType = "CPA"
	CommissionCPC         CommissionType = "CPC"
	CommissionCPL         CommissionType = "CPL"
	CommissionFlatFee     CommissionType = "Flat-fee"
	CommissionPerformance CommissionType = "Performance"

	ReportByPage              ReportBy = "page"
	ReportByDate              ReportBy = "date"
	ReportByDevice            ReportBy = "device"
	ReportByCountry           ReportBy = "country"
	ReportByDomain            ReportBy = "domain"
	ReportByLink              ReportBy = "link"
	ReportByMerchant          ReportBy = "merchant"
	ReportByNetworkPayoutType ReportBy = "network_payout_type"

	TimePeriodDay   TimePeriod = "day"
	TimePeriodWeek  TimePeriod = "week"
	TimePeriodMonth TimePeriod = "month"

	ReportSortImpressions         ReportSortField = "impressions"
	ReportSortAffiliatedClicks    ReportSortField = "clicks_affiliated"
	ReportSortOrderAmount         ReportSortField = "order_amount"
	ReportSortPublisherCommission ReportSortField = "publisher_commission_amount"
	ReportSortSales               ReportSortField = "sales"
	ReportSortPageURL             ReportSortField = "page_url"
	ReportSortISODate             ReportSortField = "isodate"
	ReportSortDeviceType          ReportSortField = "device_type"
	ReportSortUserCountry         ReportSortField = "user_ip_country"
	ReportSortDomain              ReportSortField = "domain"
	ReportSortMerchantName        ReportSortField = "merchant_name"
	ReportSortTargetURL           ReportSortField = "target_url"

	PaymentTypeAffiliate PaymentType = "affiliate"
	PaymentTypeFlatFee   PaymentType = "flatfee"
)

type ListMerchantsRequest struct {
	PublisherDomainID           int64
	Search                      string
	AdvertiserID                int64
	MerchantID                  int64
	VerticalID                  int64
	Country                     string
	FavouritesOnly              bool
	Limit                       int
	Offset                      int
	SortBy                      MerchantSortField
	SortDirection               SortDirection
	AlternativeVerticalID       int64
	AlternativeVerticalTaxonomy string
	AlternativeVerticalCountry  string
}

type WrapLinkRequest struct {
	DestinationURL string
	SourceURL      string
	CustomID       string
}

type ListCommissionsRequest struct {
	Limit          int
	Offset         int
	StartDate      time.Time
	EndDate        time.Time
	UpdatedSince   time.Time
	CustomID       string
	MerchantID     int64
	AdvertiserID   int64
	DomainID       int64
	SortDirection  ReportSortDirection
	SortBy         CommissionSortField
	CommissionID   string
	Status         CommissionStatus
	CommissionType CommissionType
}

type PerformanceReportRequest struct {
	ReportBy       ReportBy
	StartDate      Date
	EndDate        Date
	Limit          int
	Offset         int
	SortBy         ReportSortField
	SortDirection  ReportSortDirection
	TimePeriod     TimePeriod
	Currency       string
	AdvertiserID   int64
	DomainID       int64
	PageSearch     string
	LinkSearch     string
	MerchantSearch string
	UserCountries  []string
	PaymentType    PaymentType
	Timezone       string
}

// PublisherWorkflow is the bounded current Skimlinks publisher surface.
type PublisherWorkflow interface {
	ListMerchants(context.Context, ListMerchantsRequest, ...socialhub.CallOption) (MerchantsResponse, error)
	ListDomains(context.Context, ...socialhub.CallOption) (DomainsResponse, error)
	WrapLink(context.Context, WrapLinkRequest, ...socialhub.CallOption) (WrappedLink, error)
	ListCommissions(context.Context, ListCommissionsRequest, ...socialhub.CallOption) (CommissionsResponse, error)
	GetPerformanceReport(context.Context, PerformanceReportRequest, ...socialhub.CallOption) (PerformanceReportResponse, error)
}

type Rate struct {
	AggregationType string     `json:"aggregation_type"`
	RateType        string     `json:"rate_type"`
	BaseRate        ExactValue `json:"base_rate"`
	IncreasedRate   ExactValue `json:"increased_rate"`
	Currency        string     `json:"currency"`
	PayoutType      string     `json:"payout_type"`
	Description     string     `json:"description"`
}

type MerchantMetadata struct {
	Logo              string `json:"logo"`
	Description       string `json:"description"`
	SpecialConditions string `json:"special_conditions"`
}

type AlternativeVertical struct {
	Taxonomy    string                `json:"taxonomy"`
	CountryCode string                `json:"country_code"`
	ID          ExactValue            `json:"id"`
	Name        string                `json:"name"`
	Verticals   []AlternativeVertical `json:"verticals"`
}

type Merchant struct {
	ID                          ExactValue            `json:"id"`
	MerchantID                  ExactValue            `json:"merchant_id"`
	AdvertiserID                ExactValue            `json:"advertiser_id"`
	MerchantIDs                 []ExactValue          `json:"merchant_ids"`
	Name                        string                `json:"name"`
	Domains                     []string              `json:"domains"`
	PartnerType                 string                `json:"partner_type"`
	Countries                   []string              `json:"countries"`
	CountryCode                 string                `json:"country_code"`
	Favourite                   bool                  `json:"favourite"`
	CalculatedConversionRate    ExactValue            `json:"calculated_conversion_rate"`
	CalculatedAverageDailySales ExactValue            `json:"calculated_average_daily_sales"`
	CalculatedAverageOrderValue ExactValue            `json:"calculated_average_order_value"`
	CalculatedCommissionRate    ExactValue            `json:"calculated_commission_rate"`
	CalculatedECPC              ExactValue            `json:"calculated_ecpc"`
	EstimatedPaymentDays        string                `json:"estimated_payment_days"`
	EstimatedAttributionWindow  string                `json:"estimated_attribution_window"`
	MinimumRate                 *Rate                 `json:"minimum_rate"`
	MaximumRate                 *Rate                 `json:"maximum_rate"`
	Metadata                    MerchantMetadata      `json:"metadata"`
	AlternativeVerticals        []AlternativeVertical `json:"alternative_verticals"`
	Raw                         json.RawMessage       `json:"-"`
}

func (value *Merchant) UnmarshalJSON(data []byte) error {
	type wire Merchant
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Merchant(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type PublisherDomainStat struct {
	PublisherDomainID           ExactValue `json:"publisher_domain_id"`
	CalculatedAverageOrderValue ExactValue `json:"calculated_average_order_value"`
	CalculatedCommissionRate    ExactValue `json:"calculated_commission_rate"`
	CalculatedConversionRate    ExactValue `json:"calculated_conversion_rate"`
	CalculatedECPC              ExactValue `json:"calculated_ecpc"`
	EstimatedReversalRate       ExactValue `json:"estimated_reversal_rate"`
}

type MerchantsResponse struct {
	UserCurrency         string                `json:"user_currency"`
	Merchants            []Merchant            `json:"merchants"`
	LastValue            ExactValue            `json:"last_val"`
	HasMore              bool                  `json:"has_more"`
	NextValue            ExactValue            `json:"next_val"`
	NumberReturned       int64                 `json:"num_returned"`
	PublisherDomainStats []PublisherDomainStat `json:"publisher_domain_stats"`
	Meta                 ResponseMeta          `json:"-"`
	Raw                  json.RawMessage       `json:"-"`
}

type MerchantDomain struct {
	ID           ExactValue      `json:"id"`
	Domain       string          `json:"domain"`
	MerchantID   ExactValue      `json:"merchant_id"`
	AdvertiserID ExactValue      `json:"advertiser_id"`
	Raw          json.RawMessage `json:"-"`
}

func (value *MerchantDomain) UnmarshalJSON(data []byte) error {
	type wire MerchantDomain
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = MerchantDomain(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type DomainsResponse struct {
	HasMore        bool             `json:"has_more"`
	LastValue      ExactValue       `json:"last_val"`
	NextValue      ExactValue       `json:"next_val"`
	NextPrefix     ExactValue       `json:"next_prefix"`
	NumberReturned int64            `json:"num_returned"`
	Domains        []MerchantDomain `json:"domains"`
	Meta           ResponseMeta     `json:"-"`
	Raw            json.RawMessage  `json:"-"`
}

type WrappedLink struct {
	URL string
}

type CommissionPagination struct {
	HasNext    bool  `json:"has_next"`
	TotalCount int64 `json:"total_count"`
	Offset     int64 `json:"offset"`
	Limit      int64 `json:"limit"`
}

type CommissionBasket struct {
	Items           ExactValue `json:"items"`
	PublisherAmount ExactValue `json:"publisher_amount"`
	OrderAmount     ExactValue `json:"order_amount"`
	Currency        string     `json:"currency"`
	CommissionType  string     `json:"commission_type"`
}

type CommissionTransactionDetails struct {
	AggregationID   string           `json:"aggregation_id"`
	Basket          CommissionBasket `json:"basket"`
	Status          string           `json:"status"`
	PaymentStatus   string           `json:"payment_status"`
	TransactionDate string           `json:"transaction_date"`
	LastUpdated     string           `json:"last_updated"`
	InvoiceID       string           `json:"invoice_id"`
}

type CommissionMerchantDetails struct {
	Name           string     `json:"name"`
	ID             ExactValue `json:"id"`
	MerchantName   string     `json:"merchant_name"`
	MerchantID     ExactValue `json:"merchant_id"`
	AdvertiserName string     `json:"advertiser_name"`
	AdvertiserID   ExactValue `json:"advertiser_id"`
}

type CommissionClickDetails struct {
	Date                   string `json:"date"`
	CustomID               string `json:"custom_id"`
	UserCountry            string `json:"user_country"`
	PageURL                string `json:"page_url"`
	NormalizedPageURL      string `json:"normalized_page_url"`
	ClickedURL             string `json:"clicked_url"`
	NormalizedPageReferrer string `json:"normalized_page_referrer"`
	Platform               string `json:"platform"`
}

type Commission struct {
	CommissionID       ExactValue                   `json:"commission_id"`
	TransactionDetails CommissionTransactionDetails `json:"transaction_details"`
	MerchantDetails    CommissionMerchantDetails    `json:"merchant_details"`
	ClickDetails       CommissionClickDetails       `json:"click_details"`
	PublisherID        ExactValue                   `json:"publisher_id"`
	PublisherDomainID  ExactValue                   `json:"publisher_domain_id"`
	Raw                json.RawMessage              `json:"-"`
}

func (value *Commission) UnmarshalJSON(data []byte) error {
	type wire Commission
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Commission(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type CommissionsResponse struct {
	Pagination  CommissionPagination `json:"pagination"`
	Commissions []Commission         `json:"commissions"`
	Meta        ResponseMeta         `json:"-"`
	Raw         json.RawMessage      `json:"-"`
}

type PerformanceReportRow struct {
	PublisherCommissionAmount ExactValue      `json:"publisher_commission_amount"`
	ClicksAffiliated          ExactValue      `json:"clicks_affiliated"`
	Sales                     ExactValue      `json:"sales"`
	CTRAffiliated             ExactValue      `json:"ctr_affiliated"`
	ConversionRate            ExactValue      `json:"conversion_rate"`
	Impressions               ExactValue      `json:"impressions"`
	OrderAmount               ExactValue      `json:"order_amount"`
	RPM                       ExactValue      `json:"rpm"`
	EPC                       ExactValue      `json:"epc"`
	ISODate                   string          `json:"isodate"`
	PageURL                   string          `json:"page_url"`
	DeviceType                string          `json:"device_type"`
	UserCountry               string          `json:"user_ip_country"`
	Domain                    string          `json:"domain"`
	MerchantName              string          `json:"merchant_name"`
	TargetURL                 string          `json:"target_url"`
	NetworkPayoutType         string          `json:"network_payout_type"`
	Raw                       json.RawMessage `json:"-"`
}

func (value *PerformanceReportRow) UnmarshalJSON(data []byte) error {
	type wire PerformanceReportRow
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = PerformanceReportRow(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type PerformanceReportResponse struct {
	Count   int64                  `json:"count"`
	Reports []PerformanceReportRow `json:"reports"`
	Totals  *PerformanceReportRow  `json:"totals"`
	Meta    ResponseMeta           `json:"-"`
	Raw     json.RawMessage        `json:"-"`
}

func decodeProviderObject(data []byte, target any) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxProviderObjectBytes || trimmed[0] != '{' || !json.Valid(trimmed) {
		return fmt.Errorf("skimlinks: invalid provider object")
	}
	return json.Unmarshal(trimmed, target)
}

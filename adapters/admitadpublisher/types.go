package admitadpublisher

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

// ExactValue preserves a provider JSON string, number, or null without
// float64 coercion.
type ExactValue struct {
	raw json.RawMessage
}

func (value *ExactValue) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxExactValueBytes || !json.Valid(trimmed) {
		return fmt.Errorf("admitadpublisher: invalid exact value")
	}
	first := trimmed[0]
	if first != '"' && first != '-' && (first < '0' || first > '9') && !bytes.Equal(trimmed, []byte("null")) {
		return fmt.Errorf("admitadpublisher: exact value must be a JSON string, number, or null")
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
		return fmt.Errorf("admitadpublisher: decode target and exact value are required")
	}
	return json.Unmarshal(value.raw, target)
}

type ResponseMeta struct {
	RequestID string
}

type PageMeta struct {
	Count  ExactValue `json:"count"`
	Limit  ExactValue `json:"limit"`
	Offset ExactValue `json:"offset"`
}

type ConnectionStatus string

const (
	ConnectionActive   ConnectionStatus = "active"
	ConnectionPending  ConnectionStatus = "pending"
	ConnectionDeclined ConnectionStatus = "declined"
)

type ProgramTool string

const (
	ProgramToolDeeplink           ProgramTool = "deeplink"
	ProgramToolProducts           ProgramTool = "products"
	ProgramToolRetag              ProgramTool = "retag"
	ProgramToolLostOrders         ProgramTool = "lost_orders"
	ProgramToolCoupons            ProgramTool = "coupons"
	ProgramToolBasketTracking     ProgramTool = "basket_tracking"
	ProgramToolMobileSiteTracking ProgramTool = "tracking_in_mobile_site"
	ProgramToolMobileAppTracking  ProgramTool = "tracking_in_mobile_app"
)

type CustomerType string

const (
	CustomerNew CustomerType = "new_customers"
	CustomerAll CustomerType = "all_customers"
)

type StatisticsOrder string

const (
	StatisticsOrderAction          StatisticsOrder = "action"
	StatisticsOrderName            StatisticsOrder = "name"
	StatisticsOrderLeads           StatisticsOrder = "leads"
	StatisticsOrderSales           StatisticsOrder = "sales"
	StatisticsOrderPayment         StatisticsOrder = "payment_sum"
	StatisticsOrderPaymentApproved StatisticsOrder = "payment_sum_approved"
	StatisticsOrderPaymentDeclined StatisticsOrder = "payment_sum_declined"
	StatisticsOrderPaymentOpen     StatisticsOrder = "payment_sum_open"
	StatisticsOrderViews           StatisticsOrder = "views"
	StatisticsOrderClicks          StatisticsOrder = "clicks"
	StatisticsOrderCTR             StatisticsOrder = "ctr"
	StatisticsOrderECPC            StatisticsOrder = "ecpc"
	StatisticsOrderCR              StatisticsOrder = "cr"
	StatisticsOrderECPM            StatisticsOrder = "ecpm"
)

type ListProgramsRequest struct {
	WebsiteID        int64
	ConnectionStatus ConnectionStatus
	HasTool          ProgramTool
	Offset           int
	Limit            int
}

type GetProgramRequest struct {
	WebsiteID  int64
	CampaignID int64
}

type GenerateDeeplinksRequest struct {
	WebsiteID  int64
	CampaignID int64
	TargetURLs []string
	SubID      string
	SubID1     string
	SubID2     string
	SubID3     string
	SubID4     string
}

type ListCouponsRequest struct {
	WebsiteID           int64
	CampaignID          int64
	CategoryID          int64
	CampaignCategoryID  int64
	TypeID              int64
	Search              string
	DateStart           time.Time
	DateEnd             time.Time
	Region              string
	Language            string
	OrderBy             []string
	IsTrackingPromocode *bool
	HasAffiliateLink    *bool
	CustomerType        CustomerType
	Offset              int
	Limit               int
}

type ListCampaignStatisticsRequest struct {
	DateStart   time.Time
	DateEnd     time.Time
	WebsiteIDs  []int64
	CampaignIDs []int64
	SubID       string
	OrderBy     []StatisticsOrder
	Offset      int
	Limit       int
}

// PublisherWorkflow exposes the bounded Admitad Publisher API surface.
type PublisherWorkflow interface {
	ListProgramsForWebsite(context.Context, ListProgramsRequest, ...socialhub.CallOption) (ProgramsResponse, error)
	GetProgramForWebsite(context.Context, GetProgramRequest, ...socialhub.CallOption) (Program, error)
	GenerateDeeplinks(context.Context, GenerateDeeplinksRequest, ...socialhub.CallOption) (DeeplinksResponse, error)
	ListCouponsForWebsite(context.Context, ListCouponsRequest, ...socialhub.CallOption) (CouponsResponse, error)
	ListCampaignStatistics(context.Context, ListCampaignStatisticsRequest, ...socialhub.CallOption) (CampaignStatisticsResponse, error)
}

type Region struct {
	Region string `json:"region"`
}

type Category struct {
	ID       ExactValue `json:"id"`
	Name     string     `json:"name"`
	Language string     `json:"language"`
	Parent   *Category  `json:"parent"`
}

type ProgramAction struct {
	ID          ExactValue `json:"id"`
	Name        string     `json:"name"`
	Type        string     `json:"type"`
	PaymentSize string     `json:"payment_size"`
}

type Traffic struct {
	ID      ExactValue `json:"id"`
	Name    string     `json:"name"`
	Type    string     `json:"type"`
	Enabled *bool      `json:"enabled"`
}

type FeedInfo struct {
	Name                 string `json:"name"`
	AdmitadLastUpdate    string `json:"admitad_last_update"`
	AdvertiserLastUpdate string `json:"advertiser_last_update"`
	CSVLink              string `json:"csv_link"`
	XMLLink              string `json:"xml_link"`
}

type Program struct {
	ID                       ExactValue       `json:"id"`
	Name                     string           `json:"name"`
	Image                    string           `json:"image"`
	Status                   string           `json:"status"`
	ConnectionStatus         ConnectionStatus `json:"connection_status"`
	Rating                   ExactValue       `json:"rating"`
	Description              string           `json:"description"`
	RawDescription           string           `json:"raw_description"`
	SiteURL                  string           `json:"site_url"`
	Exclusive                *bool            `json:"exclusive"`
	Currency                 string           `json:"currency"`
	Regions                  []Region         `json:"regions"`
	Categories               []Category       `json:"categories"`
	Actions                  []ProgramAction  `json:"actions"`
	ActionsDetail            json.RawMessage  `json:"actions_detail"`
	CR                       ExactValue       `json:"cr"`
	CRTrend                  ExactValue       `json:"cr_trend"`
	ECPC                     ExactValue       `json:"ecpc"`
	ECPCTrend                ExactValue       `json:"ecpc_trend"`
	EPC                      ExactValue       `json:"epc"`
	EPCTrend                 ExactValue       `json:"epc_trend"`
	RateOfApprove            ExactValue       `json:"rate_of_approve"`
	MoreRules                string           `json:"more_rules"`
	Geotargeting             *bool            `json:"geotargeting"`
	CouponIframeDenied       *bool            `json:"coupon_iframe_denied"`
	ActivationDate           string           `json:"activation_date"`
	ModifiedDate             string           `json:"modified_date"`
	Connected                *bool            `json:"connected"`
	Moderation               *bool            `json:"moderation"`
	AverageHoldTime          ExactValue       `json:"avg_hold_time"`
	AverageMoneyTransferTime ExactValue       `json:"avg_money_transfer_time"`
	DenyNewWebmasters        *bool            `json:"denynewwms"`
	Retag                    *bool            `json:"retag"`
	ShowProductLinks         *bool            `json:"show_products_links"`
	Traffics                 []Traffic        `json:"traffics"`
	LandingCode              string           `json:"landing_code"`
	LandingTitle             string           `json:"landing_title"`
	ActionType               string           `json:"action_type"`
	IndividualTerms          *bool            `json:"individual_terms"`
	AllowDeeplink            *bool            `json:"allow_deeplink"`
	GotoLink                 string           `json:"gotolink"`
	ProductsCSVLink          string           `json:"products_csv_link"`
	ProductsXMLLink          string           `json:"products_xml_link"`
	FeedsInfo                []FeedInfo       `json:"feeds_info"`
	AdvertiserLegalInfo      string           `json:"advertiser_legal_info"`
	ActionCountries          []string         `json:"action_countries"`
	AllowActionsAllCountries *bool            `json:"allow_actions_all_countries"`
	ActionTestingLimit       ExactValue       `json:"action_testing_limit"`
	ActionsLimit             ExactValue       `json:"actions_limit"`
	ActionsLimit24           ExactValue       `json:"actions_limit_24"`
	MobileDeviceType         string           `json:"mobile_device_type"`
	MobileOSType             string           `json:"mobile_os_type"`
	MobileOS                 string           `json:"mobile_os"`
	KKTUCode                 string           `json:"kktu_code"`
	Meta                     ResponseMeta     `json:"-"`
	Raw                      json.RawMessage  `json:"-"`
}

func (value *Program) UnmarshalJSON(data []byte) error {
	type wire Program
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Program(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type ProgramsResponse struct {
	Results []Program       `json:"results"`
	Page    PageMeta        `json:"_meta"`
	Meta    ResponseMeta    `json:"-"`
	Raw     json.RawMessage `json:"-"`
}

func (value *ProgramsResponse) UnmarshalJSON(data []byte) error {
	type wire ProgramsResponse
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = ProgramsResponse(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type Deeplink struct {
	Link               string          `json:"link"`
	IsAffiliateProduct *bool           `json:"is_affiliate_product"`
	Raw                json.RawMessage `json:"-"`
}

func (value *Deeplink) UnmarshalJSON(data []byte) error {
	type wire Deeplink
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Deeplink(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type DeeplinksResponse struct {
	Links []Deeplink
	Meta  ResponseMeta
	Raw   json.RawMessage
}

func (value *DeeplinksResponse) UnmarshalJSON(data []byte) error {
	if err := decodeProviderArray(data, &value.Links); err != nil {
		return err
	}
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type Coupon struct {
	ID                  ExactValue      `json:"id"`
	Name                string          `json:"name"`
	Image               string          `json:"image"`
	Status              string          `json:"status"`
	Rating              ExactValue      `json:"rating"`
	Description         string          `json:"description"`
	ShortName           string          `json:"short_name"`
	Campaign            json.RawMessage `json:"campaign"`
	DateStart           string          `json:"date_start"`
	DateEnd             *string         `json:"date_end"`
	Categories          json.RawMessage `json:"categories"`
	Types               json.RawMessage `json:"types"`
	FramesetLink        string          `json:"frameset_link"`
	GotoLink            string          `json:"goto_link"`
	Promocode           string          `json:"promocode"`
	Exclusive           *bool           `json:"exclusive"`
	Discount            string          `json:"discount"`
	Species             string          `json:"species"`
	IsPersonal          *bool           `json:"is_personal"`
	IsUnique            *bool           `json:"is_unique"`
	Regions             json.RawMessage `json:"regions"`
	Language            string          `json:"language"`
	IsTrackingPromocode *bool           `json:"is_tracking_promo_code"`
	HasAffiliateLink    *bool           `json:"has_affiliate_link"`
	CustomerType        CustomerType    `json:"customer_type"`
	Merchant            json.RawMessage `json:"merchant"`
	Raw                 json.RawMessage `json:"-"`
}

func (value *Coupon) UnmarshalJSON(data []byte) error {
	type wire Coupon
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Coupon(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type CouponsResponse struct {
	Results []Coupon        `json:"results"`
	Page    PageMeta        `json:"_meta"`
	Meta    ResponseMeta    `json:"-"`
	Raw     json.RawMessage `json:"-"`
}

func (value *CouponsResponse) UnmarshalJSON(data []byte) error {
	type wire CouponsResponse
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = CouponsResponse(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type CampaignStatistic struct {
	CampaignName    string          `json:"advcampaign_name"`
	CampaignID      ExactValue      `json:"advcampaign_id"`
	Currency        string          `json:"currency"`
	Leads           ExactValue      `json:"leads_sum"`
	Sales           ExactValue      `json:"sales_sum"`
	Payment         ExactValue      `json:"payment_sum"`
	PaymentDeclined ExactValue      `json:"payment_sum_declined"`
	PaymentApproved ExactValue      `json:"payment_sum_approved"`
	PaymentOpen     ExactValue      `json:"payment_sum_open"`
	Views           ExactValue      `json:"views"`
	Clicks          ExactValue      `json:"clicks"`
	CTR             ExactValue      `json:"ctr"`
	ECPC            ExactValue      `json:"ecpc"`
	CR              ExactValue      `json:"cr"`
	ECPM            ExactValue      `json:"ecpm"`
	Raw             json.RawMessage `json:"-"`
}

func (value *CampaignStatistic) UnmarshalJSON(data []byte) error {
	type wire CampaignStatistic
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = CampaignStatistic(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type CampaignStatisticsResponse struct {
	Results []CampaignStatistic `json:"results"`
	Page    PageMeta            `json:"_meta"`
	Meta    ResponseMeta        `json:"-"`
	Raw     json.RawMessage     `json:"-"`
}

func (value *CampaignStatisticsResponse) UnmarshalJSON(data []byte) error {
	type wire CampaignStatisticsResponse
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = CampaignStatisticsResponse(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

func decodeProviderObject(data []byte, target any) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxProviderObjectBytes || trimmed[0] != '{' || !json.Valid(trimmed) {
		return fmt.Errorf("admitadpublisher: invalid provider object")
	}
	return json.Unmarshal(trimmed, target)
}

func decodeProviderArray(data []byte, target any) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxProviderObjectBytes || trimmed[0] != '[' || !json.Valid(trimmed) {
		return fmt.Errorf("admitadpublisher: invalid provider array")
	}
	return json.Unmarshal(trimmed, target)
}

package applovinconversion

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	MaximumBatchSize = 100

	leadGenDocumentationURL           = "https://support.applovin.com/en/growth/promoting-your-websites/api/conversion-api-for-lead-gen"
	restrictedLeadGenDocumentationURL = "https://support.applovin.com/en/growth/promoting-your-websites/api/restricted-lead-gen-capi"
	deduplicationDocumentationURL     = "https://support.applovin.com/en/growth/promoting-your-websites/track-and-optimize/deduplicating-events"
)

type AccountPolicy string

const (
	PolicyStandard          AccountPolicy = "STANDARD"
	PolicyLeadGen           AccountPolicy = "LEAD_GEN"
	PolicyRestrictedLeadGen AccountPolicy = "RESTRICTED_LEAD_GEN"
)

type EventName string

const (
	EventPageView       EventName = "page_view"
	EventViewItem       EventName = "view_item"
	EventAddToCart      EventName = "add_to_cart"
	EventBeginCheckout  EventName = "begin_checkout"
	EventPurchase       EventName = "purchase"
	EventAddPaymentInfo EventName = "add_payment_info"
	EventRemoveFromCart EventName = "remove_from_cart"
	EventSearch         EventName = "search"
	EventViewCart       EventName = "view_cart"
	EventGenerateLead   EventName = "generate_lead"
	EventLogin          EventName = "login"
	EventSignUp         EventName = "sign_up"
	EventSubscribe      EventName = "subscribe"
	EventAppOpen        EventName = "app_open"
)

type EventSourceIndicator string

const (
	SourceApp EventSourceIndicator = "app"
	SourceWeb EventSourceIndicator = "web"
)

type OperatingSystem string

const (
	OSIOS     OperatingSystem = "ios"
	OSAndroid OperatingSystem = "android"
	OSDesktop OperatingSystem = "desktop_os"
)

type PaymentType string

const (
	PaymentCreditCard PaymentType = "credit_card"
	PaymentDeferred   PaymentType = "deferred"
	PaymentRedeemable PaymentType = "redeemable"
	PaymentOnDelivery PaymentType = "payment_on_delivery"
	PaymentWallet     PaymentType = "wallet"
	PaymentOther      PaymentType = "other"
)

type AccountingMode string

const (
	AccountingCash    AccountingMode = "CASH"
	AccountingAccrual AccountingMode = "ACCRUAL"
)

type AttributionModel string

const (
	AttributionLastClick          AttributionModel = "LAST_CLICK"
	AttributionFirstClick         AttributionModel = "FIRST_CLICK"
	AttributionLinear             AttributionModel = "LINEAR"
	AttributionTimeDecay          AttributionModel = "TIME_DECAY"
	AttributionCustomMultiTouch   AttributionModel = "CUSTOM_MULTI_TOUCH"
	AttributionLastNonDirectTouch AttributionModel = "LAST_NON_DIRECT_TOUCH"
	AttributionClicksViews        AttributionModel = "CLICKS_AND_VIEWS_ENHANCED"
	AttributionAnyClick           AttributionModel = "ANY_CLICK"
)

// Decimal preserves an exact non-negative JSON number without float64
// rounding. Construct it from a base-10 string such as "19.95".
type Decimal string

func (value Decimal) String() string { return string(value) }

func (value Decimal) MarshalJSON() ([]byte, error) {
	if !validDecimal(value, true) {
		return nil, fmt.Errorf("applovinconversion: invalid decimal")
	}
	return json.Marshal(json.Number(value))
}

func UnixMilliseconds(value time.Time) int64 { return value.UnixMilli() }

type UserData struct {
	Alart           string               `json:"alart,omitempty"`
	ALEID           string               `json:"aleid,omitempty"`
	Axwrt           string               `json:"axwrt,omitempty"`
	ClientID        string               `json:"client_id,omitempty"`
	ClientIPAddress string               `json:"client_ip_address"`
	ClientUserAgent string               `json:"client_user_agent"`
	Email           string               `json:"email,omitempty"`
	ESI             EventSourceIndicator `json:"esi"`
	Phone           string               `json:"phone,omitempty"`
	UserID          string               `json:"user_id,omitempty"`
	CountryCode     string               `json:"country_code,omitempty"`
	IFA             string               `json:"ifa,omitempty"`
	IDFV            string               `json:"idfv,omitempty"`
	OS              OperatingSystem      `json:"os,omitempty"`
	SID             string               `json:"sid,omitempty"`
	Zip             string               `json:"zip,omitempty"`
}

type MeasurementPartnerData struct {
	AccountingMode                 AccountingMode   `json:"accounting_mode"`
	AttributionModel               AttributionModel `json:"attribution_model"`
	AttributionLookbackWindowHours *int64           `json:"attribution_lookback_window_hours,omitempty"`
	AttributionShare               Decimal          `json:"attribution_share"`
	IsClaimable                    bool             `json:"is_claimable"`
	CampaignID                     string           `json:"campaign_id,omitempty"`
	CreativeSetID                  string           `json:"creative_set_id,omitempty"`
	FirstPurchaseTimestamp         *int64           `json:"first_purchase_ts,omitempty"`
	FirstVisitTimestamp            *int64           `json:"first_visit_ts,omitempty"`
	IsNewCustomer                  *bool            `json:"is_new_customer,omitempty"`
	IsNewVisitor                   *bool            `json:"is_new_visitor,omitempty"`
	LastPurchaseTimestamp          *int64           `json:"last_purchase_ts,omitempty"`
	LastVisitTimestamp             *int64           `json:"last_visit_ts,omitempty"`
}

type Item struct {
	ItemID         string  `json:"item_id"`
	ImageURL       string  `json:"image_url,omitempty"`
	ItemCategoryID int64   `json:"item_category_id,omitempty"`
	ItemName       string  `json:"item_name,omitempty"`
	ItemVariantID  string  `json:"item_variant_id,omitempty"`
	Price          Decimal `json:"price,omitempty"`
	Quantity       Decimal `json:"quantity,omitempty"`
	Affiliation    string  `json:"affiliation,omitempty"`
	Discount       Decimal `json:"discount,omitempty"`
	ItemBrand      string  `json:"item_brand,omitempty"`
	ItemCategory   string  `json:"item_category,omitempty"`
	ItemCategory2  string  `json:"item_category2,omitempty"`
}

// EventData is closed to the event-specific data types in this package so an
// unsupported custom object cannot be sent accidentally.
type EventData interface {
	eventData()
}

type ViewItemData struct {
	Items    []Item  `json:"items"`
	Currency string  `json:"currency,omitempty"`
	Value    Decimal `json:"value,omitempty"`
}

func (*ViewItemData) eventData() {}

type AddToCartData struct {
	Items    []Item  `json:"items"`
	Currency string  `json:"currency,omitempty"`
	Value    Decimal `json:"value,omitempty"`
}

func (*AddToCartData) eventData() {}

type BeginCheckoutData struct {
	Currency string  `json:"currency"`
	Items    []Item  `json:"items"`
	Value    Decimal `json:"value"`
}

func (*BeginCheckoutData) eventData() {}

type PurchaseData struct {
	Currency      string  `json:"currency"`
	Items         []Item  `json:"items"`
	Shipping      Decimal `json:"shipping"`
	Tax           Decimal `json:"tax"`
	TransactionID string  `json:"transaction_id"`
	Value         Decimal `json:"value"`
}

func (*PurchaseData) eventData() {}

type AddPaymentInfoData struct {
	Currency    string      `json:"currency,omitempty"`
	Items       []Item      `json:"items,omitempty"`
	PaymentType PaymentType `json:"payment_type,omitempty"`
	Value       Decimal     `json:"value,omitempty"`
}

func (*AddPaymentInfoData) eventData() {}

type RemoveFromCartData struct {
	Items    []Item  `json:"items"`
	Currency string  `json:"currency,omitempty"`
	Value    Decimal `json:"value,omitempty"`
}

func (*RemoveFromCartData) eventData() {}

type SearchData struct {
	SearchTerm string `json:"search_term"`
	Results    []Item `json:"results,omitempty"`
}

func (*SearchData) eventData() {}

type ViewCartData struct {
	Items    []Item  `json:"items"`
	Currency string  `json:"currency,omitempty"`
	Value    Decimal `json:"value,omitempty"`
}

func (*ViewCartData) eventData() {}

type GenerateLeadData struct {
	Currency string  `json:"currency"`
	Value    Decimal `json:"value"`
}

func (*GenerateLeadData) eventData() {}

type LoginData struct{}

func (*LoginData) eventData() {}

type SignUpData struct {
	Method string `json:"method,omitempty"`
}

func (*SignUpData) eventData() {}

type SubscribeData struct {
	Currency string  `json:"currency,omitempty"`
	Value    Decimal `json:"value,omitempty"`
}

func (*SubscribeData) eventData() {}

type ServerEvent struct {
	EventTime              int64                   `json:"event_time"`
	EventSourceURL         string                  `json:"event_source_url"`
	Name                   EventName               `json:"name"`
	UserData               UserData                `json:"user_data"`
	Data                   EventData               `json:"data"`
	MeasurementPartnerData *MeasurementPartnerData `json:"measurement_partner_data,omitempty"`
	DedupeID               string                  `json:"dedupe_id,omitempty"`
}

type SubmitResult struct {
	StatusCode int
	EventCount int
}

type ConversionWorkflow interface {
	SubmitEvents(context.Context, []ServerEvent, ...socialhub.CallOption) (SubmitResult, error)
}

var _ ConversionWorkflow = (*Client)(nil)

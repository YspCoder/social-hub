package partnerize

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

// ExactValue preserves provider strings, numbers, and nulls without float64
// coercion.
type ExactValue struct {
	raw json.RawMessage
}

func (value *ExactValue) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxExactValueBytes || !json.Valid(trimmed) {
		return fmt.Errorf("partnerize: invalid exact value")
	}
	first := trimmed[0]
	if first != '"' && first != '-' && (first < '0' || first > '9') && !bytes.Equal(trimmed, []byte("null")) {
		return fmt.Errorf("partnerize: exact value must be a JSON string, number, or null")
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
		return fmt.Errorf("partnerize: decode target and exact value are required")
	}
	return json.Unmarshal(value.raw, target)
}

type ResponseMeta struct {
	RequestID           string
	RateLimitLimit      string
	RateLimitRemaining  string
	RateLimitReset      string
	RateLimitRetryAfter string
}

type YesNo string

const (
	Yes YesNo = "y"
	No  YesNo = "n"
)

type CampaignStatus string

const (
	CampaignApproved CampaignStatus = "a"
	CampaignPending  CampaignStatus = "p"
	CampaignRejected CampaignStatus = "r"
)

type Currency string

const (
	CurrencyGBP Currency = "GBP"
	CurrencyUSD Currency = "USD"
	CurrencyEUR Currency = "EUR"
	CurrencyJPY Currency = "JPY"
)

type ConversionStatus string

const (
	ConversionPending  ConversionStatus = "pending"
	ConversionApproved ConversionStatus = "approved"
	ConversionRejected ConversionStatus = "rejected"
)

type ConversionPivot string

const (
	ConversionPivotCampaign           ConversionPivot = "campaign"
	ConversionPivotProduct            ConversionPivot = "product"
	ConversionPivotPublisherReference ConversionPivot = "publisher_reference"
)

type ListCampaignsRequest struct {
	Status CampaignStatus
}

type ListCreativesRequest struct {
	CampaignID      string
	Active          YesNo
	Tags            string
	CreativeTypeIDs []string
}

type KeyValuePair struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type CreateTrackingLinkRequest struct {
	CampaignID     string
	Description    string
	DestinationURL string
	Params         []KeyValuePair
	Active         *bool
}

type ListConversionsRequest struct {
	StartDate          time.Time
	EndDate            time.Time
	TextDate           string
	Timezone           string
	Currency           Currency
	DateType           string
	Pivot              ConversionPivot
	PivotValues        []string
	Statuses           []ConversionStatus
	Limit              int
	CursorID           int64
	Offset             int
	InvoiceCreatedDate time.Time
	IncludePaymentInfo bool
}

// PartnerWorkflow is the bounded current Partnerize Partners API surface.
type PartnerWorkflow interface {
	GetPartner(context.Context, ...socialhub.CallOption) (PartnerResponse, error)
	ListCampaigns(context.Context, ListCampaignsRequest, ...socialhub.CallOption) (CampaignsResponse, error)
	ListCreatives(context.Context, ListCreativesRequest, ...socialhub.CallOption) (CreativesResponse, error)
	CreateTrackingLink(context.Context, CreateTrackingLinkRequest, ...socialhub.CallOption) (TrackingLinkResponse, error)
	ListConversions(context.Context, ListConversionsRequest, ...socialhub.CallOption) (ConversionsResponse, error)
}

type Partner struct {
	ABN                   string            `json:"abn"`
	AccountName           string            `json:"account_name"`
	CampaignSelect        string            `json:"campaign_select"`
	CompanyName           string            `json:"company_name"`
	CompanyDivision       string            `json:"company_division"`
	CompanyLogo           string            `json:"company_logo"`
	ContactEmail          string            `json:"contact_email"`
	ContactLocale         string            `json:"contact_locale"`
	ContactName           string            `json:"contact_name"`
	Created               string            `json:"created"`
	CreatedBy             string            `json:"created_by"`
	DefaultCurrency       Currency          `json:"default_currency"`
	Description           string            `json:"description"`
	ForeignIdentifier     string            `json:"foreign_identifier"`
	GSTRegistered         string            `json:"gst_registered"`
	IMProvider            string            `json:"im_provider"`
	IMUsername            string            `json:"im_username"`
	IsAffiliateUser       YesNo             `json:"is_affiliate_user"`
	IsForeignNetwork      YesNo             `json:"is_foreign_network"`
	IsLeadUser            YesNo             `json:"is_lead_user"`
	LegalEntity           string            `json:"legal_entity"`
	NetworkID             string            `json:"network_id"`
	NetworkNotes          string            `json:"network_notes"`
	NetworkStatus         string            `json:"network_status"`
	OperatingCountry      string            `json:"operating_country"`
	Phone                 ExactValue        `json:"phone"`
	PhoneArea             ExactValue        `json:"phone_area"`
	PromotionalMethod     ExactValue        `json:"promotional_method"`
	PromotionalMethodName string            `json:"promotional_method_name"`
	PublisherID           string            `json:"publisher_id"`
	ReportingIdentifier   string            `json:"reporting_identifier"`
	ReportingTimezone     string            `json:"reporting_timezone"`
	SignupIP              string            `json:"signup_ip"`
	TermsAndConditionsID  ExactValue        `json:"terms_and_conditions_id"`
	UKVATRegistered       YesNo             `json:"uk_vat_registered"`
	USTaxState            string            `json:"us_tax_state"`
	VATNumber             ExactValue        `json:"vat_number"`
	Vertical              ExactValue        `json:"vertical"`
	VerticalName          string            `json:"vertical_name"`
	WeekStart             string            `json:"week_start"`
	Websites              []PartnerWebsite  `json:"websites"`
	Databases             []PartnerDatabase `json:"databases"`
	Raw                   json.RawMessage   `json:"-"`
}

func (value *Partner) UnmarshalJSON(data []byte) error {
	type wire Partner
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Partner(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type PartnerWebsite struct {
	PublisherID  string     `json:"publisher_id"`
	Active       YesNo      `json:"active"`
	Primary      YesNo      `json:"primary"`
	Country      string     `json:"website_country"`
	Name         string     `json:"website_name"`
	Type         ExactValue `json:"website_type"`
	TypeName     string     `json:"website_type_name"`
	URL          string     `json:"website_url"`
	Vertical     ExactValue `json:"website_vertical"`
	VerticalName string     `json:"website_vertical_name"`
}

type PartnerDatabase struct {
	PublisherID    string     `json:"publisher_id"`
	Active         YesNo      `json:"active"`
	CreationMethod string     `json:"creation_method"`
	Name           string     `json:"database_name"`
	Female         ExactValue `json:"female"`
	Male           ExactValue `json:"male"`
	MaximumAge     ExactValue `json:"max_age"`
	MinimumAge     ExactValue `json:"min_age"`
	Size           ExactValue `json:"size"`
}

type PartnerResponse struct {
	Partner Partner         `json:"publisher"`
	Meta    ResponseMeta    `json:"-"`
	Raw     json.RawMessage `json:"-"`
}

func (value *PartnerResponse) UnmarshalJSON(data []byte) error {
	type wire PartnerResponse
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = PartnerResponse(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type Campaign struct {
	AdvertiserID           string            `json:"advertiser_id"`
	CampaignID             string            `json:"campaign_id"`
	Title                  string            `json:"title"`
	Description            map[string]string `json:"description"`
	DestinationURL         string            `json:"destination_url"`
	CampaignLogo           string            `json:"campaign_logo"`
	PublisherStatus        string            `json:"publisher_status"`
	Status                 string            `json:"status"`
	ConversionType         string            `json:"conversion_type"`
	DefaultCurrency        Currency          `json:"default_currency"`
	DefaultCommissionRate  string            `json:"default_commission_rate"`
	DefaultCommissionValue string            `json:"default_commission_value"`
	AllowDeepLinking       YesNo             `json:"allow_deep_linking"`
	CookiePeriod           ExactValue        `json:"cookie_period"`
	TrackingMethod         string            `json:"tracking_method"`
	ReportingTimezone      string            `json:"reporting_timezone"`
	VerticalID             ExactValue        `json:"vertical_id"`
	VerticalName           string            `json:"vertical_name"`
	CustomTermsID          string            `json:"campaign_custom_terms_and_conditions_id"`
	CustomTermsTitle       string            `json:"campaign_custom_terms_and_conditions_title"`
	Terms                  json.RawMessage   `json:"terms"`
	Raw                    json.RawMessage   `json:"-"`
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

type CampaignWrapper struct {
	Campaign Campaign `json:"campaign"`
}

type CampaignsResponse struct {
	Campaigns []CampaignWrapper `json:"campaigns"`
	Meta      ResponseMeta      `json:"-"`
	Raw       json.RawMessage   `json:"-"`
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

type Creative struct {
	CreativeID                 string          `json:"creative_id"`
	AdvertiserReference        string          `json:"advertiser_reference"`
	CreativeItems              []CreativeItem  `json:"creative_items"`
	CreativeTrackingLink       string          `json:"creative_tracking_link"`
	CreativeTrackingLinkRotate string          `json:"creative_tracking_link_rotate"`
	DefaultSpecificCreativeID  string          `json:"default_specific_creative_id"`
	Description                string          `json:"description"`
	Destination                string          `json:"destination"`
	DynamicTrackingLink        string          `json:"dynamic_tracking_link"`
	Height                     ExactValue      `json:"height"`
	HTMLTrackingLink           string          `json:"html_tracking_link"`
	Limits                     string          `json:"limits"`
	Preview                    string          `json:"preview"`
	Width                      ExactValue      `json:"width"`
	Tags                       string          `json:"tags"`
	Active                     YesNo           `json:"active"`
	CampaignID                 string          `json:"campaign_id"`
	CreativeTypeID             ExactValue      `json:"creative_type_id"`
	CustomAppendURLParameters  string          `json:"custom_append_url_parameters"`
	CustomPrependURLParameters string          `json:"custom_prepend_url_parameters"`
	StartDateTime              string          `json:"start_date_time"`
	EndDateTime                string          `json:"end_date_time"`
	CreatedBy                  string          `json:"created_by"`
	CreatedAt                  string          `json:"created_at"`
	Raw                        json.RawMessage `json:"-"`
}

func (value *Creative) UnmarshalJSON(data []byte) error {
	type wire Creative
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Creative(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type CreativeItem struct {
	ContentType                string     `json:"content_type"`
	CreativeItemID             string     `json:"creative_item_id"`
	Description                string     `json:"description"`
	Filename                   string     `json:"filename"`
	ImageURL                   string     `json:"img_url"`
	Preview                    string     `json:"preview"`
	SpecificDestination        string     `json:"specific_destination"`
	CreatedBy                  string     `json:"created_by"`
	CreatedAt                  string     `json:"created_at"`
	Tags                       string     `json:"tags"`
	Active                     YesNo      `json:"active"`
	CampaignID                 string     `json:"campaign_id"`
	CreativeTypeID             ExactValue `json:"creative_type_id"`
	CustomAppendURLParameters  string     `json:"custom_append_url_parameters"`
	CustomPrependURLParameters string     `json:"custom_prepend_url_parameters"`
}

type CreativeWrapper struct {
	Creative Creative `json:"creative"`
}

type CreativesResponse struct {
	Creatives []CreativeWrapper `json:"creatives"`
	Meta      ResponseMeta      `json:"-"`
	Raw       json.RawMessage   `json:"-"`
}

func (value *CreativesResponse) UnmarshalJSON(data []byte) error {
	type wire CreativesResponse
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = CreativesResponse(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type TrackingLink struct {
	ID               string          `json:"id"`
	CampaignID       string          `json:"campaign_id"`
	Description      *string         `json:"description"`
	DestinationURL   string          `json:"destination_url"`
	TrackingShortURL string          `json:"tracking_short_url"`
	TrackingURL      string          `json:"tracking_url"`
	Params           []KeyValuePair  `json:"params"`
	Active           bool            `json:"active"`
	Raw              json.RawMessage `json:"-"`
}

func (value *TrackingLink) UnmarshalJSON(data []byte) error {
	type wire TrackingLink
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = TrackingLink(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type TrackingLinkResponse struct {
	Link          TrackingLink    `json:"link"`
	ExecutionTime string          `json:"execution_time"`
	Meta          ResponseMeta    `json:"-"`
	Raw           json.RawMessage `json:"-"`
}

func (value *TrackingLinkResponse) UnmarshalJSON(data []byte) error {
	type wire TrackingLinkResponse
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = TrackingLinkResponse(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type ConversionValue struct {
	Status              ConversionStatus `json:"conversion_status"`
	Value               ExactValue       `json:"value"`
	PublisherCommission ExactValue       `json:"publisher_commission"`
}

type Click struct {
	CampaignID         string     `json:"campaign_id"`
	PublisherID        string     `json:"publisher_id"`
	Type               string     `json:"type"`
	Status             string     `json:"status"`
	SetTime            string     `json:"set_time"`
	SetIP              string     `json:"set_ip"`
	LastUsed           ExactValue `json:"last_used"`
	LastIP             string     `json:"last_ip"`
	PublisherReference string     `json:"publisher_reference"`
	Referer            string     `json:"referer"`
	CreativeID         ExactValue `json:"creative_id"`
	CreativeType       ExactValue `json:"creative_type"`
	SpecificCreativeID ExactValue `json:"specific_creative_id"`
	DeviceID           ExactValue `json:"ref_device_id"`
	TrafficSourceID    ExactValue `json:"ref_traffic_source_id"`
	PartnershipModelID ExactValue `json:"ref_partnership_model_id"`
	UserContextID      ExactValue `json:"ref_user_context_id"`
	Device             string     `json:"ref_device"`
	TrafficSource      string     `json:"ref_traffic_source"`
	PartnershipModel   string     `json:"ref_partnership_model"`
	UserContext        string     `json:"ref_user_context"`
	ClickReference     string     `json:"clickref"`
}

type ConversionItem struct {
	ConversionItemID    string          `json:"conversion_item_id"`
	SKU                 ExactValue      `json:"sku"`
	Category            string          `json:"category"`
	ItemValue           ExactValue      `json:"item_value"`
	PublisherCommission ExactValue      `json:"item_publisher_commission"`
	Status              string          `json:"item_status"`
	LastUpdate          string          `json:"last_update"`
	PublisherSelfBillID ExactValue      `json:"publisher_self_bill_id"`
	ApprovedAt          string          `json:"approved_at"`
	StatusID            ExactValue      `json:"item_status_id"`
	RejectReason        string          `json:"reject_reason"`
	VoucherCodes        json.RawMessage `json:"voucher_codes"`
	Metadata            json.RawMessage `json:"meta_data"`
	Payable             bool            `json:"payable"`
	Raw                 json.RawMessage `json:"-"`
}

func (value *ConversionItem) UnmarshalJSON(data []byte) error {
	type wire ConversionItem
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = ConversionItem(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type Conversion struct {
	ConversionID        string           `json:"conversion_id"`
	CampaignID          string           `json:"campaign_id"`
	PublisherID         string           `json:"publisher_id"`
	ConversionTime      string           `json:"conversion_time"`
	CreativeType        ExactValue       `json:"creative_type"`
	CreativeID          ExactValue       `json:"creative_id"`
	SpecificCreativeID  ExactValue       `json:"specific_creative_id"`
	Currency            Currency         `json:"currency"`
	PublisherReference  string           `json:"publisher_reference"`
	AdvertiserReference string           `json:"advertiser_reference"`
	ConversionReference string           `json:"conversion_reference"`
	CustomerType        string           `json:"customer_type"`
	RefererIP           string           `json:"referer_ip"`
	SourceReferer       string           `json:"source_referer"`
	LastModified        string           `json:"last_modified"`
	ConversionType      ExactValue       `json:"conversion_type"`
	Country             string           `json:"country"`
	CustomerReference   string           `json:"customer_reference"`
	DeviceID            ExactValue       `json:"ref_device_id"`
	PartnershipModelID  ExactValue       `json:"ref_partnership_model_id"`
	TrafficSourceID     ExactValue       `json:"ref_traffic_source_id"`
	ConversionMetricID  ExactValue       `json:"ref_conversion_metric_id"`
	UserContextID       ExactValue       `json:"ref_user_context_id"`
	CampaignTitle       string           `json:"campaign_title"`
	PublisherName       string           `json:"publisher_name"`
	Click               Click            `json:"click"`
	ConversionMetric    string           `json:"ref_conversion_metric"`
	Device              string           `json:"ref_device"`
	PartnershipModel    string           `json:"ref_partnership_model"`
	TrafficSource       string           `json:"ref_traffic_source"`
	UserContext         string           `json:"ref_user_context"`
	ConversionValue     ConversionValue  `json:"conversion_value"`
	PublisherCommission ExactValue       `json:"publisher_commission"`
	Metadata            json.RawMessage  `json:"meta_data"`
	Items               []ConversionItem `json:"conversion_items"`
	WasDisputed         bool             `json:"was_disputed"`
	ConversionLag       ExactValue       `json:"conversion_lag"`
	ClickReference      string           `json:"clickref"`
	Raw                 json.RawMessage  `json:"-"`
}

func (value *Conversion) UnmarshalJSON(data []byte) error {
	type wire Conversion
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Conversion(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

// ConversionWrapper accepts both the wrapper shown in Partnerize's official
// example and the direct object declared by the OpenAPI schema.
type ConversionWrapper struct {
	Conversion Conversion      `json:"conversion_data"`
	Raw        json.RawMessage `json:"-"`
}

func (value *ConversionWrapper) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := decodeProviderObject(data, &fields); err != nil {
		return err
	}
	if conversion, wrapped := fields["conversion_data"]; wrapped {
		if len(bytes.TrimSpace(conversion)) == 0 || bytes.Equal(bytes.TrimSpace(conversion), []byte("null")) {
			return fmt.Errorf("partnerize: conversion_data must be an object")
		}
		if err := json.Unmarshal(conversion, &value.Conversion); err != nil {
			return err
		}
	} else if err := json.Unmarshal(data, &value.Conversion); err != nil {
		return err
	}
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type Pagination struct {
	TotalPageCount ExactValue `json:"total_page_count"`
	TotalItemCount ExactValue `json:"total_item_count"`
	FirstPage      string     `json:"first_page"`
	LastPage       string     `json:"last_page"`
	NextPage       string     `json:"next_page"`
	PreviousPage   string     `json:"previous_page"`
}

type Hypermedia struct {
	Pagination Pagination `json:"pagination"`
}

type ConversionsResponse struct {
	TotalConversionCount     map[string]ExactValue `json:"total_conversion_count"`
	TotalPublisherCommission map[string]ExactValue `json:"total_publisher_commission"`
	TotalValue               map[string]ExactValue `json:"total_value"`
	StartDateTimeUTC         string                `json:"start_date_time_utc"`
	EndDateTimeUTC           string                `json:"end_date_time_utc"`
	StartDateTime            string                `json:"start_date_time"`
	EndDateTime              string                `json:"end_date_time"`
	Limit                    ExactValue            `json:"limit"`
	Metadata                 json.RawMessage       `json:"meta_data"`
	Count                    ExactValue            `json:"count"`
	CursorID                 ExactValue            `json:"cursor_id"`
	Hypermedia               Hypermedia            `json:"hypermedia"`
	ExecutionTime            string                `json:"execution_time"`
	Conversions              []ConversionWrapper   `json:"conversions"`
	Meta                     ResponseMeta          `json:"-"`
	Raw                      json.RawMessage       `json:"-"`
}

func (value *ConversionsResponse) UnmarshalJSON(data []byte) error {
	type wire ConversionsResponse
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = ConversionsResponse(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

func decodeProviderObject(data []byte, target any) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxProviderObjectBytes || trimmed[0] != '{' || !json.Valid(trimmed) {
		return fmt.Errorf("partnerize: invalid provider object")
	}
	return json.Unmarshal(trimmed, target)
}

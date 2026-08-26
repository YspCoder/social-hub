package rakutenadvertising

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	maxExactValueBytes     = 256
	maxProviderObjectBytes = 8 << 20
)

// ExactValue preserves a provider JSON string, number, boolean, or null
// without float64 coercion.
type ExactValue struct {
	raw json.RawMessage
}

func (value *ExactValue) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxExactValueBytes || !json.Valid(trimmed) {
		return fmt.Errorf("rakutenadvertising: invalid exact value")
	}
	first := trimmed[0]
	if first != '"' && first != '-' && first != 't' && first != 'f' &&
		(first < '0' || first > '9') && !bytes.Equal(trimmed, []byte("null")) {
		return fmt.Errorf("rakutenadvertising: exact value must be a JSON scalar or null")
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
		return fmt.Errorf("rakutenadvertising: decode target and exact value are required")
	}
	return json.Unmarshal(value.raw, target)
}

type ResponseMeta struct {
	RequestID string
}

type PageLinks struct {
	Previous string `json:"prev"`
	Next     string `json:"next"`
	Self     string `json:"self"`
}

type PageMetadata struct {
	APINameVersion string    `json:"api_name_version"`
	Page           int       `json:"page"`
	Limit          int       `json:"limit"`
	Total          int64     `json:"total"`
	Links          PageLinks `json:"_links"`
}

func (value *PageMetadata) UnmarshalJSON(data []byte) error {
	type wire PageMetadata
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var aliases struct {
		Links PageLinks `json:"links"`
	}
	if err := json.Unmarshal(data, &aliases); err != nil {
		return err
	}
	*value = PageMetadata(decoded)
	if value.Links == (PageLinks{}) {
		value.Links = aliases.Links
	}
	return nil
}

type SearchAdvertisersRequest struct {
	Page      int
	Limit     int
	ShipsTo   string
	DeepLinks *bool
	Network   int
}

type PartnerStatus string

const (
	PartnerActive           PartnerStatus = "active"
	PartnerPending          PartnerStatus = "pending"
	PartnerSelfRemoved      PartnerStatus = "self-removed"
	PartnerPermanentDecline PartnerStatus = "permanent-decline"
	PartnerPermanentRemove  PartnerStatus = "permanent-remove"
	PartnerTemporaryDecline PartnerStatus = "temp-decline"
	PartnerTemporaryRemove  PartnerStatus = "temp-remove"
	PartnerExtended         PartnerStatus = "extended"
)

type AdvertiserStatus string

const (
	AdvertiserActive   AdvertiserStatus = "active"
	AdvertiserInactive AdvertiserStatus = "inactive"
)

type DateRange string

const (
	DateRangeOneDay     DateRange = "1d"
	DateRangeSevenDays  DateRange = "7d"
	DateRangeThirtyDays DateRange = "30d"
)

type PartnershipSortField string

const (
	SortByApplyDate        PartnershipSortField = "apply_datetime"
	SortByApproveDate      PartnershipSortField = "approve_datetime"
	SortByStatusUpdateDate PartnershipSortField = "status_update_datetime"
)

type SortDirection string

const (
	SortAscending  SortDirection = "asc"
	SortDescending SortDirection = "dsc"
)

type ListPartnershipsRequest struct {
	PartnerStatus     PartnerStatus
	Network           int
	AdvertiserStatus  AdvertiserStatus
	Category          string
	StatusUpdateRange DateRange
	ApproveDateRange  DateRange
	ApplyDateRange    DateRange
	SortBy            PartnershipSortField
	OrderBy           SortDirection
	Limit             int
	Page              int
}

type ProductLanguage string

const (
	LanguageEnglishUS     ProductLanguage = "en_US"
	LanguageFrenchFrance  ProductLanguage = "fr_FR"
	LanguageGermanGermany ProductLanguage = "de_DE"
	LanguagePortugueseBR  ProductLanguage = "pt_BR"
)

type ProductSortField string

const (
	ProductSortRetailPrice ProductSortField = "retailprice"
	ProductSortName        ProductSortField = "productname"
	ProductSortCategory    ProductSortField = "categoryname"
	ProductSortAdvertiser  ProductSortField = "mid"
)

type ProductSort struct {
	Field     ProductSortField
	Direction SortDirection
}

type SearchProductsRequest struct {
	Keyword      string
	Exact        string
	One          string
	None         string
	Category     string
	Language     ProductLanguage
	Max          int
	PageNumber   int
	AdvertiserID int64
	Sort         []ProductSort
}

type CreateDeepLinkRequest struct {
	URL          string `json:"url"`
	AdvertiserID int64  `json:"advertiser_id"`
	U1           string `json:"u1,omitempty"`
}

type Currency string

const (
	CurrencyUSD Currency = "USD"
	CurrencyCAD Currency = "CAD"
	CurrencyGBP Currency = "GBP"
	CurrencyEUR Currency = "EUR"
	CurrencyBRL Currency = "BRL"
	CurrencyAUD Currency = "AUD"
	CurrencyJPY Currency = "JPY"
)

type TransactionType string

const (
	TransactionBatch    TransactionType = "batch"
	TransactionRealtime TransactionType = "realtime"
)

type ListTransactionsRequest struct {
	ProcessDateStart     time.Time
	ProcessDateEnd       time.Time
	TransactionDateStart time.Time
	TransactionDateEnd   time.Time
	Limit                int
	Page                 int
	Currency             Currency
	Type                 TransactionType
}

// PublisherWorkflow exposes the bounded Rakuten Advertising Affiliate APIs
// publisher surface.
type PublisherWorkflow interface {
	SearchAdvertisers(context.Context, SearchAdvertisersRequest, ...socialhub.CallOption) (AdvertisersResponse, error)
	ListPartnerships(context.Context, ListPartnershipsRequest, ...socialhub.CallOption) (PartnershipsResponse, error)
	SearchProducts(context.Context, SearchProductsRequest, ...socialhub.CallOption) (ProductSearchResponse, error)
	CreateDeepLink(context.Context, CreateDeepLinkRequest, ...socialhub.CallOption) (DeepLinkResponse, error)
	ListTransactions(context.Context, ListTransactionsRequest, ...socialhub.CallOption) (TransactionsResponse, error)
}

type InternationalCapabilities struct {
	ShipsTo []string `json:"ships_to"`
}

type SoftwarePolicy struct {
	DSA      bool   `json:"dsa"`
	DSASpecs string `json:"dsa_specs"`
}

type AdvertiserPolicies struct {
	InternationalCapabilities InternationalCapabilities `json:"international_capabilities"`
	Software                  SoftwarePolicy            `json:"software"`
}

type AdvertiserFeatures struct {
	PremiumAdvertiser   bool `json:"premium_advertiser"`
	ProductFeed         bool `json:"product_feed"`
	MediaOptimization   bool `json:"media_opt_report"`
	CrossDeviceTracking bool `json:"cross_device_tracking"`
	DeepLinks           bool `json:"deep_links"`
	ITPCompliant        bool `json:"itp_compliant"`
}

type AdvertiserContact struct {
	Name     string `json:"name"`
	JobTitle string `json:"job_title"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Country  string `json:"country"`
}

type Advertiser struct {
	ID          int64              `json:"id"`
	Name        string             `json:"name"`
	URL         string             `json:"url"`
	Description string             `json:"description"`
	Policies    AdvertiserPolicies `json:"policies"`
	Features    AdvertiserFeatures `json:"features"`
	Contact     AdvertiserContact  `json:"contact"`
	Raw         json.RawMessage    `json:"-"`
}

func (value *Advertiser) UnmarshalJSON(data []byte) error {
	type wire Advertiser
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Advertiser(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type AdvertisersResponse struct {
	Metadata    PageMetadata
	Advertisers []Advertiser
	Meta        ResponseMeta
	Raw         json.RawMessage
}

func (value *AdvertisersResponse) UnmarshalJSON(data []byte) error {
	var decoded struct {
		Metadata      PageMetadata `json:"_metadata"`
		MetadataAlias PageMetadata `json:"metadata"`
		Advertisers   []Advertiser `json:"advertisers"`
	}
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	value.Metadata = chooseMetadata(decoded.Metadata, decoded.MetadataAlias)
	value.Advertisers = decoded.Advertisers
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type PartnershipAdvertiser struct {
	ID         int64    `json:"id"`
	Network    int      `json:"network"`
	Name       string   `json:"name"`
	Status     string   `json:"status"`
	Categories []string `json:"categories"`
	Details    string   `json:"details"`
}

type Partnership struct {
	Advertiser      PartnershipAdvertiser `json:"advertiser"`
	Status          string                `json:"status"`
	StatusUpdatedAt string                `json:"status_update_datetime"`
	ApprovedAt      string                `json:"approve_datetime"`
	AppliedAt       string                `json:"apply_datetime"`
	Offers          string                `json:"offers"`
	Raw             json.RawMessage       `json:"-"`
}

func (value *Partnership) UnmarshalJSON(data []byte) error {
	type wire Partnership
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Partnership(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type PartnershipsResponse struct {
	Metadata     PageMetadata
	Partnerships []Partnership
	Meta         ResponseMeta
	Raw          json.RawMessage
}

func (value *PartnershipsResponse) UnmarshalJSON(data []byte) error {
	var decoded struct {
		Metadata      PageMetadata  `json:"_metadata"`
		MetadataAlias PageMetadata  `json:"metadata"`
		Partnerships  []Partnership `json:"partnerships"`
	}
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	value.Metadata = chooseMetadata(decoded.Metadata, decoded.MetadataAlias)
	value.Partnerships = decoded.Partnerships
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type ProductCategory struct {
	Primary   string `xml:"primary" json:"primary"`
	Secondary string `xml:"secondary" json:"secondary"`
}

type ProductPrice struct {
	Currency string `xml:"currency,attr" json:"currency"`
	Amount   string `xml:",chardata" json:"amount"`
}

type ProductDescription struct {
	Short string `xml:"short" json:"short"`
	Long  string `xml:"long" json:"long"`
}

type Product struct {
	AdvertiserID   int64              `xml:"mid" json:"advertiser_id"`
	AdvertiserName string             `xml:"merchantname" json:"advertiser_name"`
	LinkID         string             `xml:"linkid" json:"link_id"`
	CreatedOn      string             `xml:"createdon" json:"created_on"`
	SKU            string             `xml:"sku" json:"sku"`
	Name           string             `xml:"productname" json:"name"`
	Category       ProductCategory    `xml:"category" json:"category"`
	Price          ProductPrice       `xml:"price" json:"price"`
	SalePrice      ProductPrice       `xml:"saleprice" json:"sale_price"`
	UPCCode        string             `xml:"upccode" json:"upc_code"`
	Description    ProductDescription `xml:"description" json:"description"`
	Keywords       string             `xml:"keywords" json:"keywords"`
	LinkURL        string             `xml:"linkurl" json:"link_url"`
	ImageURL       string             `xml:"imageurl" json:"image_url"`
}

type ProductSearchResponse struct {
	XMLName      xml.Name     `xml:"result" json:"-"`
	TotalMatches int64        `xml:"TotalMatches" json:"total_matches"`
	TotalPages   int64        `xml:"TotalPages" json:"total_pages"`
	PageNumber   int64        `xml:"PageNumber" json:"page_number"`
	Products     []Product    `xml:"item" json:"products"`
	Meta         ResponseMeta `xml:"-" json:"-"`
	Raw          []byte       `xml:"-" json:"-"`
}

type DeepLink struct {
	URL         string `json:"deep_link_url"`
	U1          string `json:"u1"`
	OriginalURL string `json:"url"`
}

type DeepLinkAdvertiser struct {
	ID          int64           `json:"id"`
	Name        string          `json:"name"`
	URL         string          `json:"url"`
	Description string          `json:"description"`
	DeepLink    DeepLink        `json:"deep_link"`
	Raw         json.RawMessage `json:"-"`
}

func (value *DeepLinkAdvertiser) UnmarshalJSON(data []byte) error {
	type wire DeepLinkAdvertiser
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = DeepLinkAdvertiser(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type DeepLinkResponse struct {
	Metadata   PageMetadata       `json:"_metadata"`
	Advertiser DeepLinkAdvertiser `json:"advertiser"`
	Meta       ResponseMeta       `json:"-"`
	Raw        json.RawMessage    `json:"-"`
}

func (value *DeepLinkResponse) UnmarshalJSON(data []byte) error {
	type wire DeepLinkResponse
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = DeepLinkResponse(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type Transaction struct {
	ETransactionID    string          `json:"etransaction_id"`
	AdvertiserID      ExactValue      `json:"advertiser_id"`
	PublisherID       ExactValue      `json:"sid"`
	OrderID           ExactValue      `json:"order_id"`
	OfferID           ExactValue      `json:"offer_id"`
	SKUNumber         string          `json:"sku_number"`
	SaleAmount        ExactValue      `json:"sale_amount"`
	Quantity          ExactValue      `json:"quantity"`
	Commissions       ExactValue      `json:"commissions"`
	ProcessDate       string          `json:"process_date"`
	TransactionDate   string          `json:"transaction_date"`
	TransactionType   string          `json:"transaction_type"`
	ProductName       string          `json:"product_name"`
	U1                string          `json:"u1"`
	Currency          string          `json:"currency"`
	IsEvent           ExactValue      `json:"is_event"`
	CommissionListID  ExactValue      `json:"commissions_list_id"`
	OrderAutoLockDate ExactValue      `json:"order_auto_lock_date"`
	LockStatus        ExactValue      `json:"lock_status"`
	Raw               json.RawMessage `json:"-"`
}

func (value *Transaction) UnmarshalJSON(data []byte) error {
	type wire Transaction
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	var aliases struct {
		OfferIDAlias          ExactValue `json:"offer"`
		CommissionListIDAlias ExactValue `json:"commission_list_id"`
	}
	if err := json.Unmarshal(data, &aliases); err != nil {
		return err
	}
	*value = Transaction(decoded)
	if !value.OfferID.IsSet() {
		value.OfferID = aliases.OfferIDAlias
	}
	if !value.CommissionListID.IsSet() {
		value.CommissionListID = aliases.CommissionListIDAlias
	}
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type TransactionsResponse struct {
	Transactions []Transaction
	Meta         ResponseMeta
	Raw          json.RawMessage
}

func (value *TransactionsResponse) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxProviderObjectBytes || !json.Valid(trimmed) {
		return fmt.Errorf("rakutenadvertising: invalid transactions response")
	}
	switch trimmed[0] {
	case '[':
		if err := json.Unmarshal(trimmed, &value.Transactions); err != nil {
			return err
		}
	case '{':
		var transaction Transaction
		if err := json.Unmarshal(trimmed, &transaction); err != nil {
			return err
		}
		value.Transactions = []Transaction{transaction}
	default:
		return fmt.Errorf("rakutenadvertising: transactions response must be an array or object")
	}
	value.Raw = append(value.Raw[:0], trimmed...)
	return nil
}

func chooseMetadata(primary, alias PageMetadata) PageMetadata {
	if primary.APINameVersion != "" || primary.Page != 0 || primary.Limit != 0 || primary.Total != 0 || primary.Links != (PageLinks{}) {
		return primary
	}
	return alias
}

func decodeProviderObject(data []byte, target any) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxProviderObjectBytes || trimmed[0] != '{' || !json.Valid(trimmed) {
		return fmt.Errorf("rakutenadvertising: invalid provider object")
	}
	return json.Unmarshal(trimmed, target)
}

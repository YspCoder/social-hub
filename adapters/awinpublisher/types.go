package awinpublisher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	DefaultMaxFeedBytes     int64 = 256 << 20
	DefaultMaxFeedLineBytes int64 = 16 << 20
	MaximumFeedLineBytes    int64 = 64 << 20
	maxExactValueBytes            = 256
	maxProviderObjectBytes        = MaximumFeedLineBytes
)

type ResponseMeta struct {
	RequestID string
}

// ExactValue preserves a provider JSON string, number, or null without
// float64 coercion.
type ExactValue struct {
	raw json.RawMessage
}

func (value *ExactValue) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxExactValueBytes || !json.Valid(trimmed) {
		return fmt.Errorf("awinpublisher: invalid exact value")
	}
	first := trimmed[0]
	if first != '"' && first != '-' && (first < '0' || first > '9') && !bytes.Equal(trimmed, []byte("null")) {
		return fmt.Errorf("awinpublisher: exact value must be a JSON string, number, or null")
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
		return fmt.Errorf("awinpublisher: decode target and exact value are required")
	}
	return json.Unmarshal(value.raw, target)
}

type ProgramRelationship string

const (
	RelationshipJoined    ProgramRelationship = "joined"
	RelationshipPending   ProgramRelationship = "pending"
	RelationshipSuspended ProgramRelationship = "suspended"
	RelationshipRejected  ProgramRelationship = "rejected"
	RelationshipNotJoined ProgramRelationship = "notjoined"
)

type TransactionDateType string

const (
	DateTypeTransaction TransactionDateType = "transaction"
	DateTypeValidation  TransactionDateType = "validation"
	DateTypeAmendment   TransactionDateType = "amendment"
)

type TransactionStatus string

const (
	TransactionPending  TransactionStatus = "pending"
	TransactionApproved TransactionStatus = "approved"
	TransactionDeclined TransactionStatus = "declined"
	TransactionDeleted  TransactionStatus = "deleted"
)

type Region string

const (
	RegionAT Region = "AT"
	RegionAU Region = "AU"
	RegionBE Region = "BE"
	RegionBR Region = "BR"
	RegionBU Region = "BU"
	RegionCA Region = "CA"
	RegionCH Region = "CH"
	RegionDE Region = "DE"
	RegionDK Region = "DK"
	RegionES Region = "ES"
	RegionFI Region = "FI"
	RegionFR Region = "FR"
	RegionGB Region = "GB"
	RegionIE Region = "IE"
	RegionIT Region = "IT"
	RegionNL Region = "NL"
	RegionNO Region = "NO"
	RegionPL Region = "PL"
	RegionSE Region = "SE"
	RegionUS Region = "US"
)

// Date is a YYYY-MM-DD calendar date.
type Date string

func DateFromTime(value time.Time) Date { return Date(value.UTC().Format("2006-01-02")) }

type ListProgramsRequest struct {
	CountryCode   string
	IncludeHidden bool
	Relationship  ProgramRelationship
}

type DownloadEnhancedFeedRequest struct {
	AdvertiserID int64
	Locale       string
}

type FeedDownloadOptions struct {
	// MaxBytes bounds bytes written to output. Zero uses DefaultMaxFeedBytes.
	MaxBytes int64
	// MaxLineBytes bounds one JSONL product record. Zero uses DefaultMaxFeedLineBytes.
	MaxLineBytes int64
}

type TrackingLinkParameters struct {
	Campaign  string `json:"campaign,omitempty"`
	ClickRef  string `json:"clickref,omitempty"`
	ClickRef2 string `json:"clickref2,omitempty"`
	ClickRef3 string `json:"clickref3,omitempty"`
	ClickRef4 string `json:"clickref4,omitempty"`
	ClickRef5 string `json:"clickref5,omitempty"`
	ClickRef6 string `json:"clickref6,omitempty"`
}

type GenerateTrackingLinkRequest struct {
	AdvertiserID   int64                  `json:"advertiserId"`
	DestinationURL string                 `json:"destinationUrl,omitempty"`
	Parameters     TrackingLinkParameters `json:"parameters,omitempty"`
	Shorten        bool                   `json:"shorten,omitempty"`
}

type ListTransactionsRequest struct {
	StartDate          time.Time
	EndDate            time.Time
	AdvertiserIDs      []int64
	DateType           TransactionDateType
	Status             TransactionStatus
	ShowBasketProducts bool
	Timezone           string
}

type GetAdvertiserPerformanceRequest struct {
	StartDate Date
	EndDate   Date
	Region    Region
	DateType  TransactionDateType
	Timezone  string
}

// PublisherWorkflow exposes the bounded Awin Publisher API 1.0 surface.
type PublisherWorkflow interface {
	ListPrograms(context.Context, ListProgramsRequest, ...socialhub.CallOption) (ProgramsResponse, error)
	DownloadEnhancedFeed(context.Context, DownloadEnhancedFeedRequest, io.Writer, FeedDownloadOptions, ...socialhub.CallOption) (FeedDownloadResult, error)
	GenerateTrackingLink(context.Context, GenerateTrackingLinkRequest, ...socialhub.CallOption) (TrackingLink, error)
	ListTransactions(context.Context, ListTransactionsRequest, ...socialhub.CallOption) (TransactionsResponse, error)
	GetAdvertiserPerformance(context.Context, GetAdvertiserPerformanceRequest, ...socialhub.CallOption) (AdvertiserPerformanceResponse, error)
}

type PrimaryRegion struct {
	CountryCode string `json:"countryCode"`
	Name        string `json:"name"`
}

type ValidDomain struct {
	Domain string `json:"domain"`
}

type Program struct {
	ID              ExactValue      `json:"id"`
	Name            string          `json:"name"`
	Description     string          `json:"description"`
	DisplayURL      string          `json:"displayUrl"`
	ClickThroughURL string          `json:"clickThroughUrl"`
	LogoURL         string          `json:"logoUrl"`
	CurrencyCode    string          `json:"currencyCode"`
	PrimaryRegion   PrimaryRegion   `json:"primaryRegion"`
	PrimarySector   string          `json:"primarySector"`
	Status          string          `json:"status"`
	LinkStatus      string          `json:"linkStatus"`
	ValidDomains    []ValidDomain   `json:"validDomains"`
	Raw             json.RawMessage `json:"-"`
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
	Programs []Program
	Meta     ResponseMeta
	Raw      json.RawMessage
}

type EnhancedFeedMeta struct {
	AdvertiserID   ExactValue `json:"advertiser_id"`
	AdvertiserName string     `json:"advertiser_name"`
}

type EnhancedFeedProductBasic struct {
	ID                  ExactValue      `json:"id"`
	Title               string          `json:"title"`
	Description         string          `json:"description"`
	Link                string          `json:"link"`
	ImageLink           string          `json:"image_link"`
	AdditionalImageLink json.RawMessage `json:"additional_image_link"`
	MobileLink          string          `json:"mobile_link"`
	AwinDeepLink        string          `json:"aw_deep_link"`
	AwinMobileLink      string          `json:"aw_mobile_link"`
	VirtualModelLink    string          `json:"virtual_model_link"`
}

// EnhancedFeedProduct keeps unstable and vertical-specific sections raw while
// exposing the stable product identity and link section.
type EnhancedFeedProduct struct {
	Meta                 EnhancedFeedMeta         `json:"meta"`
	ProductBasic         EnhancedFeedProductBasic `json:"product_basic"`
	PriceAndAvailability json.RawMessage          `json:"price_and_availability"`
	ProductCategory      json.RawMessage          `json:"product_category"`
	ProductIdentifiers   json.RawMessage          `json:"product_identifiers"`
	ProductDetailed      json.RawMessage          `json:"product_detailed"`
	ShoppingCampaign     json.RawMessage          `json:"shopping_campaign"`
	Delivery             json.RawMessage          `json:"delivery"`
	Tax                  json.RawMessage          `json:"tax"`
	Raw                  json.RawMessage          `json:"-"`
}

func (value *EnhancedFeedProduct) UnmarshalJSON(data []byte) error {
	type wire EnhancedFeedProduct
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = EnhancedFeedProduct(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type FeedDownloadResult struct {
	Products      int64
	BytesWritten  int64
	ContentType   string
	ContentLength int64
	RequestID     string
}

type TrackingLink struct {
	URL         string          `json:"url"`
	ShortURL    string          `json:"shortUrl"`
	Description string          `json:"description"`
	Meta        ResponseMeta    `json:"-"`
	Raw         json.RawMessage `json:"-"`
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

type Money struct {
	Amount   ExactValue `json:"amount"`
	Currency string     `json:"currency"`
}

type ClickReferences struct {
	ClickRef  string `json:"clickRef"`
	ClickRef2 string `json:"clickRef2"`
	ClickRef3 string `json:"clickRef3"`
	ClickRef4 string `json:"clickRef4"`
	ClickRef5 string `json:"clickRef5"`
	ClickRef6 string `json:"clickRef6"`
}

type CustomParameter struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type TrackedPart struct {
	Amount   ExactValue `json:"amount"`
	Code     string     `json:"code"`
	Currency string     `json:"currency"`
}

type TransactionPart struct {
	Amount              ExactValue    `json:"amount"`
	CommissionAmount    ExactValue    `json:"commissionAmount"`
	CommissionGroupCode string        `json:"commissionGroupCode"`
	CommissionGroupID   ExactValue    `json:"commissionGroupId"`
	CommissionGroupName string        `json:"commissionGroupName"`
	TrackedParts        []TrackedPart `json:"trackedParts"`
}

type BasketProduct struct {
	ProductID           string     `json:"productId"`
	ProductName         string     `json:"productName"`
	UnitPrice           ExactValue `json:"unitPrice"`
	Quantity            ExactValue `json:"quantity"`
	SKUCode             string     `json:"skuCode"`
	CommissionGroupCode string     `json:"commissionGroupCode"`
	Category            string     `json:"category"`
}

type Transaction struct {
	ID                                       ExactValue        `json:"id"`
	URL                                      string            `json:"url"`
	AdvertiserID                             ExactValue        `json:"advertiserId"`
	PublisherID                              ExactValue        `json:"publisherId"`
	CommissionSharingPublisherID             ExactValue        `json:"commissionSharingPublisherId"`
	CommissionSharingSelectedRatePublisherID ExactValue        `json:"commissionSharingSelectedRatePublisherId"`
	SiteName                                 string            `json:"siteName"`
	Campaign                                 string            `json:"campaign"`
	CommissionStatus                         string            `json:"commissionStatus"`
	CommissionAmount                         Money             `json:"commissionAmount"`
	SaleAmount                               Money             `json:"saleAmount"`
	IPHash                                   string            `json:"ipHash"`
	CustomerCountry                          string            `json:"customerCountry"`
	ClickReferences                          ClickReferences   `json:"clickRefs"`
	ClickDate                                string            `json:"clickDate"`
	TransactionDate                          string            `json:"transactionDate"`
	ValidationDate                           string            `json:"validationDate"`
	Type                                     string            `json:"type"`
	DeclineReason                            string            `json:"declineReason"`
	VoucherCodeUsed                          bool              `json:"voucherCodeUsed"`
	VoucherCode                              string            `json:"voucherCode"`
	LapseTime                                ExactValue        `json:"lapseTime"`
	Amended                                  bool              `json:"amended"`
	AmendReason                              string            `json:"amendReason"`
	OldSaleAmount                            Money             `json:"oldSaleAmount"`
	OldCommissionAmount                      Money             `json:"oldCommissionAmount"`
	ClickDevice                              string            `json:"clickDevice"`
	TransactionDevice                        string            `json:"transactionDevice"`
	PublisherURL                             string            `json:"publisherUrl"`
	AdvertiserCountry                        string            `json:"advertiserCountry"`
	OrderRef                                 string            `json:"orderRef"`
	CustomParameters                         []CustomParameter `json:"customParameters"`
	TransactionParts                         []TransactionPart `json:"transactionParts"`
	PaidToPublisher                          bool              `json:"paidToPublisher"`
	PaymentID                                ExactValue        `json:"paymentId"`
	TransactionQueryID                       ExactValue        `json:"transactionQueryId"`
	OriginalSaleAmount                       ExactValue        `json:"originalSaleAmount"`
	AdvertiserCost                           Money             `json:"advertiserCost"`
	BasketProducts                           []BasketProduct   `json:"basketProducts"`
	Raw                                      json.RawMessage   `json:"-"`
}

func (value *Transaction) UnmarshalJSON(data []byte) error {
	type wire Transaction
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Transaction(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type TransactionsResponse struct {
	Transactions []Transaction
	Meta         ResponseMeta
	Raw          json.RawMessage
}

type AdvertiserPerformance struct {
	AdvertiserID   ExactValue      `json:"advertiserId"`
	AdvertiserName string          `json:"advertiserName"`
	PublisherID    ExactValue      `json:"publisherId"`
	PublisherName  string          `json:"publisherName"`
	Region         string          `json:"region"`
	Currency       string          `json:"currency"`
	Impressions    ExactValue      `json:"impressions"`
	Clicks         ExactValue      `json:"clicks"`
	PendingNo      ExactValue      `json:"pendingNo"`
	PendingValue   ExactValue      `json:"pendingValue"`
	PendingComm    ExactValue      `json:"pendingComm"`
	ConfirmedNo    ExactValue      `json:"confirmedNo"`
	ConfirmedValue ExactValue      `json:"confirmedValue"`
	ConfirmedComm  ExactValue      `json:"confirmedComm"`
	BonusNo        ExactValue      `json:"bonusNo"`
	BonusValue     ExactValue      `json:"bonusValue"`
	BonusComm      ExactValue      `json:"bonusComm"`
	DeclinedNo     ExactValue      `json:"declinedNo"`
	DeclinedValue  ExactValue      `json:"declinedValue"`
	DeclinedComm   ExactValue      `json:"declinedComm"`
	TotalNo        ExactValue      `json:"totalNo"`
	TotalValue     ExactValue      `json:"totalValue"`
	TotalComm      ExactValue      `json:"totalComm"`
	Raw            json.RawMessage `json:"-"`
}

func (value *AdvertiserPerformance) UnmarshalJSON(data []byte) error {
	type wire AdvertiserPerformance
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = AdvertiserPerformance(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type AdvertiserPerformanceResponse struct {
	Rows []AdvertiserPerformance
	Meta ResponseMeta
	Raw  json.RawMessage
}

func decodeProviderObject(data []byte, target any) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || int64(len(trimmed)) > maxProviderObjectBytes || trimmed[0] != '{' || !json.Valid(trimmed) {
		return fmt.Errorf("awinpublisher: invalid provider object")
	}
	return json.Unmarshal(trimmed, target)
}

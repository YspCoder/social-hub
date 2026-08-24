package conversions

import (
	"context"
	"fmt"

	"social-hub/pkg/socialhub"
)

const (
	MaximumBatchSize    = 1000
	MaximumEventAgeDays = 7
)

type Decimal string

func (value Decimal) String() string { return string(value) }

func (value Decimal) MarshalJSON() ([]byte, error) {
	if !validDecimal(value) {
		return nil, fmt.Errorf("microsoftads conversions: invalid decimal")
	}
	return []byte(value), nil
}

type EventType string

const (
	EventTypePageLoad EventType = "pageLoad"
	EventTypeCustom   EventType = "custom"
)

type AdStorageConsent string

const (
	ConsentGranted AdStorageConsent = "G"
	ConsentDenied  AdStorageConsent = "D"
)

type PageType string

const (
	PageTypeCart          PageType = "cart"
	PageTypeCategory      PageType = "category"
	PageTypeHome          PageType = "home"
	PageTypeOther         PageType = "other"
	PageTypeProduct       PageType = "product"
	PageTypePurchase      PageType = "purchase"
	PageTypeSearchResults PageType = "searchresults"
)

type UserData struct {
	MicrosoftClickID string
	Email            string
	Phone            string
	AnonymousID      string
	ExternalID       string
	ClientUserAgent  string
	ClientIPAddress  string
	IDFA             string
	GAID             string
}

type Item struct {
	ID       string
	Quantity *int64
	Price    Decimal
	Name     string
}

type HotelData struct {
	TotalPrice     Decimal
	BasePrice      Decimal
	CheckinDate    string
	CheckoutDate   string
	LengthOfStay   *int64
	PartnerHotelID string
	BookingHref    string
}

type CustomData struct {
	EventCategory   string
	EventLabel      string
	EventValue      Decimal
	SearchTerm      string
	TransactionID   string
	Value           Decimal
	Currency        string
	Items           []Item
	ItemIDs         []string
	PageType        PageType
	EcommTotalValue Decimal
	EcommCategory   string
	HotelData       *HotelData
}

type ConversionEvent struct {
	EventType        EventType
	EventTime        int64
	EventID          string
	EventName        string
	EventSourceURL   string
	PageLoadID       string
	ReferrerURL      string
	PageTitle        string
	Keywords         string
	AdStorageConsent AdStorageConsent
	UserData         UserData
	CustomData       *CustomData
}

type SubmitEventsRequest struct {
	DataProvider string
	Events       []ConversionEvent
}

type SubmitResult struct {
	StatusCode     int
	EventsAccepted int
	HasWarnings    bool
}

type EventWorkflow interface {
	SubmitEvents(context.Context, SubmitEventsRequest, ...socialhub.CallOption) (SubmitResult, error)
}

var _ EventWorkflow = (*Client)(nil)

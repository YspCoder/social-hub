package conversions

import (
	"context"
	"encoding/json"
	"fmt"

	"social-hub/pkg/socialhub"
)

const (
	MaximumBatchSize        = 1000
	TestEventQuotaPerSecond = 10
)

type Decimal string

func (value Decimal) String() string { return string(value) }

func (value Decimal) MarshalJSON() ([]byte, error) {
	if !validNonnegativeDecimal(value) {
		return nil, fmt.Errorf("reddit conversions: invalid decimal")
	}
	return json.Marshal(json.Number(value))
}

type TrackingType string

const (
	TrackingPageVisit     TrackingType = "PAGE_VISIT"
	TrackingViewContent   TrackingType = "VIEW_CONTENT"
	TrackingSearch        TrackingType = "SEARCH"
	TrackingAddToCart     TrackingType = "ADD_TO_CART"
	TrackingAddToWishlist TrackingType = "ADD_TO_WISHLIST"
	TrackingPurchase      TrackingType = "PURCHASE"
	TrackingLead          TrackingType = "LEAD"
	TrackingSignUp        TrackingType = "SIGN_UP"
	TrackingCustom        TrackingType = "CUSTOM"
)

type ActionSource string

const (
	ActionSourceWebsite       ActionSource = "WEBSITE"
	ActionSourceApp           ActionSource = "APP"
	ActionSourceOther         ActionSource = "OTHER"
	ActionSourcePhysicalStore ActionSource = "PHYSICAL_STORE"
)

type EventType struct {
	TrackingType    TrackingType
	CustomEventName string
}

type Metadata struct {
	ConversionID string
	Currency     string
	ItemCount    *int32
	Value        Decimal
	Products     []Product
}

type Product struct {
	Category  string
	ID        string
	Name      string
	Quantity  *int64
	ItemPrice Decimal
}

type UserData struct {
	Email       string
	ExternalID  string
	IPAddress   string
	PhoneNumber string
	UserAgent   string
	AAID        string
	IDFA        string
	UUID        string

	DataProcessingOptions *DataProcessingOptions
	ScreenDimensions      *ScreenDimensions
}

type DataProcessingOptions struct {
	Country string   `json:"country"`
	Region  string   `json:"region,omitempty"`
	Modes   []string `json:"modes"`
}

type ScreenDimensions struct {
	Height int `json:"height"`
	Width  int `json:"width"`
}

type ConversionEvent struct {
	ClickID        string
	EventAt        int64
	ActionSource   ActionSource
	EventSourceURL string
	Type           EventType
	Metadata       *Metadata
	User           *UserData
}

type SubmitEventsRequest struct {
	TestID string
	Events []ConversionEvent
}

type SubmitResult struct {
	StatusCode int
	Message    string
}

type ConversionWorkflow interface {
	SubmitEvents(context.Context, SubmitEventsRequest, ...socialhub.CallOption) (SubmitResult, error)
}

var _ ConversionWorkflow = (*Client)(nil)

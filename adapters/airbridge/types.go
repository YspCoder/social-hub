package airbridge

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"social-hub/pkg/socialhub"
)

// Decimal preserves an exact finite JSON number without float64 rounding or
// exponent normalization.
type Decimal string

func (value Decimal) String() string { return string(value) }

func (value Decimal) MarshalJSON() ([]byte, error) {
	if !validDecimal(value) {
		return nil, fmt.Errorf("airbridge: invalid decimal")
	}
	return json.Marshal(json.Number(value))
}

// EventCategory is an Airbridge standard or custom event category. Custom
// categories remain valid and are not restricted to the constants below.
type EventCategory string

const (
	EventUserSignup          EventCategory = "airbridge.user.signup"
	EventUserSignin          EventCategory = "airbridge.user.signin"
	EventUserSignout         EventCategory = "airbridge.user.signout"
	EventHomeViewed          EventCategory = "airbridge.ecommerce.home.viewed"
	EventProductListViewed   EventCategory = "airbridge.ecommerce.productList.viewed"
	EventSearchResultsViewed EventCategory = "airbridge.ecommerce.searchResults.viewed"
	EventProductViewed       EventCategory = "airbridge.ecommerce.product.viewed"
	EventAddPaymentInfo      EventCategory = "airbridge.addPaymentInfo"
	EventAddToWishlist       EventCategory = "airbridge.addToWishlist"
	EventProductAddedToCart  EventCategory = "airbridge.ecommerce.product.addedToCart"
	EventInitiateCheckout    EventCategory = "airbridge.initiateCheckout"
	EventOrderCompleted      EventCategory = "airbridge.ecommerce.order.completed"
	EventOrderCanceled       EventCategory = "airbridge.ecommerce.order.canceled"
	EventStartTrial          EventCategory = "airbridge.startTrial"
	EventSubscribe           EventCategory = "airbridge.subscribe"
	EventUnsubscribe         EventCategory = "airbridge.unsubscribe"
	EventAdImpression        EventCategory = "airbridge.adImpression"
	EventAdClick             EventCategory = "airbridge.adClick"
	EventCompleteTutorial    EventCategory = "airbridge.completeTutorial"
	EventAchieveLevel        EventCategory = "airbridge.achieveLevel"
	EventUnlockAchievement   EventCategory = "airbridge.unlockAchievement"
	EventRate                EventCategory = "airbridge.rate"
	EventShare               EventCategory = "airbridge.share"
	EventSchedule            EventCategory = "airbridge.schedule"
	EventSpendCredits        EventCategory = "airbridge.spendCredits"
)

type OSName string

const (
	OSAndroid OSName = "Android"
	OSIOS     OSName = "iOS"
)

// Properties is a flat scalar JSON object. Number values use Decimal so the
// adapter never silently rounds identifiers or monetary values through float64.
type Properties struct {
	Strings  map[string]string
	Numbers  map[string]Decimal
	Booleans map[string]bool
}

type User struct {
	ExternalUserID    string
	ExternalUserEmail string
	ExternalUserPhone string
	Attributes        Properties
}

type Screen struct {
	Density string
	Height  *int64
	Width   *int64
}

type Location struct {
	Latitude  Decimal
	Longitude Decimal
	Speed     string
}

type Network struct {
	Carrier  string
	Cellular *bool
	WiFi     *bool
}

// DMAConsent uses booleans publicly and is encoded as Airbridge's documented
// "0"/"1" alias values on the wire.
type DMAConsent struct {
	EEA               *bool
	AdPersonalization *bool
	AdUserData        *bool
}

type Device struct {
	DeviceUUID              string
	GAID                    string
	IFA                     string
	AppSetID                string
	IFV                     string
	ClientIP                string
	LimitAdTracking         *bool
	DeviceModel             string
	AppTrackingTransparency *int
	DeviceIdentifier        string
	Manufacturer            string
	OSName                  OSName
	OSVersion               string
	Locale                  string
	Timezone                string
	Orientation             string
	Screen                  *Screen
	Location                *Location
	Network                 *Network
	DMA                     *DMAConsent
}

type Browser struct {
	ClientID  string
	UserAgent string
}

type App struct {
	PackageName string
	Version     string
}

type Product struct {
	ProductID    string
	Name         string
	Price        Decimal
	Quantity     *int64
	Currency     string
	Position     *int64
	CategoryID   string
	CategoryName string
	BrandID      string
	BrandName    string
}

// SemanticAttributes contains the currently documented Airbridge semantic
// event attributes. Zero values are omitted from the request.
type SemanticAttributes struct {
	Action                          string
	Label                           string
	TotalValue                      Decimal
	OriginalTotalValue              Decimal
	Currency                        string
	OriginalCurrency                string
	Products                        []Product
	Period                          string
	IsRenewal                       *bool
	RenewalCount                    *int64
	ProductListID                   string
	CartID                          string
	TransactionID                   string
	TransactionType                 string
	TransactionPairedEventCategory  string
	TransactionPairedEventTimestamp *int64
	TotalQuantity                   *int64
	Query                           string
	WishListID                      string
	ContentID                       string
	ContentName                     string
	InAppPurchased                  *bool
	ContributionMargin              Decimal
	OriginalContributionMargin      Decimal
	ListID                          string
	RateID                          string
	Rate                            Decimal
	MaxRate                         Decimal
	RatingValue                     Decimal
	MaxRatingValue                  Decimal
	AchievementID                   string
	SharedChannel                   string
	Datetime                        string
	Description                     string
	IsRevenue                       *bool
	Place                           string
	ScheduleID                      string
	Type                            string
	Level                           string
	Score                           Decimal
	AdPartners                      map[string]Properties
	IsFirstPerUser                  *bool
}

type Goal struct {
	Category           EventCategory
	Value              Decimal
	CustomAttributes   Properties
	SemanticAttributes SemanticAttributes
}

type TrackingData struct {
	Channel string
	Params  Properties
}

type MobileEventRequest struct {
	EventUUID      string
	EventTimestamp *time.Time
	User           User
	Device         Device
	App            App
	Goal           Goal
	ForwardedFor   string
	AcceptLanguage string
}

type WebEventRequest struct {
	EventUUID      string
	EventTimestamp *time.Time
	User           User
	Browser        Browser
	ShortID        string
	Tracking       *TrackingData
	Goal           Goal
	ForwardedFor   string
	AcceptLanguage string
}

type SubmitResult struct {
	StatusCode int
	At         string
	Data       string
}

type EventWorkflow interface {
	SendMobileEvent(context.Context, MobileEventRequest, ...socialhub.CallOption) (SubmitResult, error)
	SendWebEvent(context.Context, WebEventRequest, ...socialhub.CallOption) (SubmitResult, error)
}

var _ EventWorkflow = (*Client)(nil)

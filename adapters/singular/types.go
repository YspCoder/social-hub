package singular

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"social-hub/pkg/socialhub"
)

// Decimal preserves an exact finite form value without float64 rounding or
// exponent normalization.
type Decimal string

func (value Decimal) String() string { return string(value) }

func (value Decimal) MarshalJSON() ([]byte, error) {
	if !validDecimal(value) {
		return nil, fmt.Errorf("singular: invalid decimal")
	}
	return json.Marshal(json.Number(value))
}

type Platform string

const (
	PlatformAndroid     Platform = "Android"
	PlatformIOS         Platform = "iOS"
	PlatformWeb         Platform = "Web"
	PlatformPC          Platform = "PC"
	PlatformXbox        Platform = "Xbox"
	PlatformPlayStation Platform = "PlayStation"
	PlatformNintendo    Platform = "Nintendo"
	PlatformMetaQuest   Platform = "MetaQuest"
	PlatformCTV         Platform = "CTV"
)

type EventName string

const (
	EventRate                 EventName = "sng_rate"
	EventSpentCredits         EventName = "sng_spent_credits"
	EventTutorialComplete     EventName = "sng_tutorial_complete"
	EventLogin                EventName = "sng_login"
	EventStartTrial           EventName = "sng_start_trial"
	EventSubscribe            EventName = "sng_subscribe"
	EventContentViewList      EventName = "sng_content_view_list"
	EventInvite               EventName = "sng_invite"
	EventShare                EventName = "sng_share"
	EventEcommercePurchase    EventName = "sng_ecommerce_purchase"
	EventViewCart             EventName = "sng_view_cart"
	EventAchievementUnlocked  EventName = "sng_achievement_unlocked"
	EventAddPaymentInfo       EventName = "sng_add_payment_info"
	EventAddToCart            EventName = "sng_add_to_cart"
	EventAddToWishlist        EventName = "sng_add_to_wishlist"
	EventCheckoutInitiated    EventName = "sng_checkout_initiated"
	EventCompleteRegistration EventName = "sng_complete_registration"
	EventContentView          EventName = "sng_content_view"
	EventLevelAchieved        EventName = "sng_level_achieved"
	EventSearch               EventName = "sng_search"

	EventPageVisit             EventName = "__PAGE_VISIT__"
	EventAdMonetizationRevenue EventName = "__ADMON_USER_LEVEL_REVENUE__"
)

type AttributeKey string

const (
	AttributeAchievementID        AttributeKey = "sng_attr_achievement_id"
	AttributeContent              AttributeKey = "sng_attr_content"
	AttributeContentID            AttributeKey = "sng_attr_content_id"
	AttributeContentList          AttributeKey = "sng_attr_content_list"
	AttributeContentType          AttributeKey = "sng_attr_content_type"
	AttributeCountry              AttributeKey = "sng_attr_country"
	AttributeCouponCode           AttributeKey = "sng_attr_coupon_code"
	AttributeDeepLink             AttributeKey = "sng_attr_deep_link"
	AttributeEventEnd             AttributeKey = "sng_attr_event_end"
	AttributeEventStart           AttributeKey = "sng_attr_event_start"
	AttributeFromDate             AttributeKey = "sng_attr_from_date"
	AttributeHotelScore           AttributeKey = "sng_attr_hotel_score"
	AttributeItemDescription      AttributeKey = "sng_attr_item_description"
	AttributeItemPrice            AttributeKey = "sng_attr_item_price"
	AttributeLatitude             AttributeKey = "sng_attr_latitude"
	AttributeLevel                AttributeKey = "sng_attr_level"
	AttributeLocation             AttributeKey = "sng_attr_location"
	AttributeAddressCountry       AttributeKey = "sng_attr_location_address_country"
	AttributeAddressRegion        AttributeKey = "sng_attr_location_address_region_or_province"
	AttributeAddressStreet        AttributeKey = "sng_attr_location_address_street"
	AttributeLongitude            AttributeKey = "sng_attr_longitude"
	AttributeMax                  AttributeKey = "sng_attr_max"
	AttributeNewVersion           AttributeKey = "sng_attr_new_version"
	AttributeOrigin               AttributeKey = "sng_attr_origin"
	AttributePaymentInfoAvailable AttributeKey = "sng_attr_payment_info_available"
	AttributeQuantity             AttributeKey = "sng_attr_quantity"
	AttributeRating               AttributeKey = "sng_attr_rating"
	AttributeRegion               AttributeKey = "sng_attr_region"
	AttributeRegistrationMethod   AttributeKey = "sng_attr_registration_method"
	AttributeReviewText           AttributeKey = "sng_attr_review_text"
	AttributeScore                AttributeKey = "sng_attr_score"
	AttributeSearchString         AttributeKey = "sng_attr_search_string"
	AttributeSubscriptionID       AttributeKey = "sng_attr_subscription_id"
	AttributeSuccess              AttributeKey = "sng_attr_success"
	AttributeToDate               AttributeKey = "sng_attr_to_date"
	AttributeTransactionID        AttributeKey = "sng_attr_transaction_id"
	AttributeTutorialID           AttributeKey = "sng_attr_tutorial_id"
	AttributeValid                AttributeKey = "sng_attr_valid"

	AttributePartnerEventID AttributeKey = "eventId"
	AttributeEmailHash      AttributeKey = "ehash"
	AttributePhoneHash      AttributeKey = "phash"
	AttributeFirstNameHash  AttributeKey = "fnamehash"
	AttributeLastNameHash   AttributeKey = "lnamehash"
	AttributeExternalID     AttributeKey = "external_id"
	AttributePhoneE164Hash  AttributeKey = "phashE164"
	AttributeRedditUUID     AttributeKey = "rdt_uuid"
	AttributeTag            AttributeKey = "etag"
	AttributePageURL        AttributeKey = "page_url"
)

// Properties is a type-safe JSON object for event attributes.
type Properties struct {
	Strings     map[string]string
	Numbers     map[string]Decimal
	Booleans    map[string]bool
	StringLists map[string][]string
}

type ConnectionType string

const (
	ConnectionWiFi    ConnectionType = "wifi"
	ConnectionCarrier ConnectionType = "carrier"
)

type SKANData struct {
	ConversionValue    *int
	FirstCallTimestamp *int64
	LastCallTimestamp  *int64
}

type Revenue struct {
	Amount           Decimal
	Currency         string
	IsRevenueEvent   *bool
	PurchaseReceipt  string
	ReceiptSignature string
	ProductID        string
	TransactionID    string
}

type AdRevenue struct {
	Amount            Decimal
	Currency          string
	AdPlatform        string
	MediationPlatform string
	AdType            string
	AdGroupType       string
	AdImpressionID    string
	AdPlacementName   string
	AdUnitID          string
	AdUnitName        string
	AdGroupID         string
	AdGroupName       string
	AdGroupPriority   string
	AdPlacementID     string
}

type WebData struct {
	ConversionEvent bool
	AttributionData map[string]string
	LandingPageURL  string
	DeviceUserAgent string
	PageReferrer    string
	Timezone        string
	OS              string
	ScreenWidth     *int64
	ScreenHeight    *int64
}

type EventRequest struct {
	Platform         Platform
	SDID             string
	Name             EventName
	IPAddress        string
	UseRequestIP     bool
	Country          string
	OSVersion        string
	Manufacturer     string
	Model            string
	Locale           string
	Build            string
	AppVersion       string
	ATTStatus        *int
	OccurredAt       *time.Time
	Attributes       Properties
	GlobalProperties map[string]string
	UserAgent        string
	Connection       ConnectionType
	CarrierName      string
	LimitDataSharing *bool
	DoNotTrack       *bool
	CustomUserID     string
	SKAN             *SKANData
	Revenue          *Revenue
	AdRevenue        *AdRevenue
	Web              *WebData
}

type SubmitResult struct {
	StatusCode int
}

type EventWorkflow interface {
	SendEvent(context.Context, EventRequest, ...socialhub.CallOption) (SubmitResult, error)
}

var _ EventWorkflow = (*Client)(nil)

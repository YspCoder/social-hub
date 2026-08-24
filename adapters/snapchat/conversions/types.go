package conversions

import (
	"context"
	"encoding/json"
	"fmt"

	"social-hub/pkg/socialhub"
)

const MaximumBatchSize = 2000

type Decimal string

func (value Decimal) String() string { return string(value) }

func (value Decimal) MarshalJSON() ([]byte, error) {
	if !validDecimal(value) {
		return nil, fmt.Errorf("snapchat conversions: invalid decimal")
	}
	return json.Marshal(json.Number(value))
}

type EventName string

const (
	EventPurchase            EventName = "PURCHASE"
	EventSave                EventName = "SAVE"
	EventStartCheckout       EventName = "START_CHECKOUT"
	EventAddCart             EventName = "ADD_CART"
	EventViewContent         EventName = "VIEW_CONTENT"
	EventAddBilling          EventName = "ADD_BILLING"
	EventSignUp              EventName = "SIGN_UP"
	EventSearch              EventName = "SEARCH"
	EventPageView            EventName = "PAGE_VIEW"
	EventSubscribe           EventName = "SUBSCRIBE"
	EventAdClick             EventName = "AD_CLICK"
	EventAdView              EventName = "AD_VIEW"
	EventCompleteTutorial    EventName = "COMPLETE_TUTORIAL"
	EventLevelComplete       EventName = "LEVEL_COMPLETE"
	EventInvite              EventName = "INVITE"
	EventLogin               EventName = "LOGIN"
	EventShare               EventName = "SHARE"
	EventReserve             EventName = "RESERVE"
	EventAchievementUnlocked EventName = "ACHIEVEMENT_UNLOCKED"
	EventAddToWishlist       EventName = "ADD_TO_WISHLIST"
	EventSpentCredits        EventName = "SPENT_CREDITS"
	EventRate                EventName = "RATE"
	EventStartTrial          EventName = "START_TRIAL"
	EventListView            EventName = "LIST_VIEW"
	EventAppInstall          EventName = "APP_INSTALL"
	EventAppOpen             EventName = "APP_OPEN"
	EventCustom1             EventName = "CUSTOM_EVENT_1"
	EventCustom2             EventName = "CUSTOM_EVENT_2"
	EventCustom3             EventName = "CUSTOM_EVENT_3"
	EventCustom4             EventName = "CUSTOM_EVENT_4"
	EventCustom5             EventName = "CUSTOM_EVENT_5"
)

type ActionSource string

const (
	ActionSourceWeb             ActionSource = "WEB"
	ActionSourceOffline         ActionSource = "OFFLINE"
	ActionSourceMobileApp       ActionSource = "MOBILE_APP"
	ActionSourcePhysicalStore   ActionSource = "physical_store"
	ActionSourcePhoneCall       ActionSource = "phone_call"
	ActionSourcePhone           ActionSource = "phone"
	ActionSourceEmail           ActionSource = "email"
	ActionSourceChat            ActionSource = "chat"
	ActionSourceSystemGenerated ActionSource = "system_generated"
)

type DataProcessingOption string

const (
	DataProcessingLMU    DataProcessingOption = "LMU"
	DataProcessingDelete DataProcessingOption = "DELETE"
)

type DeliveryCategory string

const (
	DeliveryCategoryInStore      DeliveryCategory = "in_store"
	DeliveryCategoryCurbside     DeliveryCategory = "curbside"
	DeliveryCategoryHomeDelivery DeliveryCategory = "home_delivery"
)

type ContentType string

const (
	ContentTypeProduct      ContentType = "product"
	ContentTypeProductGroup ContentType = "product_group"
)

// UserData accepts plaintext or lowercase SHA-256 for fields Snap requires to
// be hashed. MADID remains plaintext on the v3 wire after lowercase
// normalization, matching the current parameter reference.
type UserData struct {
	Emails      []string
	Phones      []string
	FirstNames  []string
	LastNames   []string
	Genders     []string
	Cities      []string
	States      []string
	Zips        []string
	Countries   []string
	ExternalIDs []string

	ClientIPAddress string
	ClientUserAgent string
	SubscriptionID  string
	LeadID          string
	AnonymousID     string
	MobileAdID      string
	DownloadID      string
	SnapClickID     string
	SnapCookie1     string
	IDFV            string
	PartnerID       string
}

type CustomData struct {
	ContentCategories []string
	ContentIDs        []string
	ContentName       string
	ContentType       ContentType
	Contents          []Content
	Currency          string
	NumItems          string
	OrderID           string
	PredictedLTV      Decimal
	Value             Decimal
	SearchString      string
	Status            string
	EventTag          string
	CustomFields      CustomFields
}

type CustomFields struct {
	Strings  map[string]string
	Numbers  map[string]Decimal
	Booleans map[string]bool
}

type Content struct {
	ID               string
	Quantity         int64
	ItemPrice        Decimal
	Brand            string
	DeliveryCategory DeliveryCategory
}

type AppData struct {
	AdvertiserTrackingEnabled *bool
	AppID                     string
	ExtendedDeviceInfo        ExtendedDeviceInfo
}

type ExtendedDeviceInfo struct {
	Version              string
	AppPackageName       string
	ShortVersion         string
	LongVersion          string
	OSVersion            string
	DeviceModelName      string
	Locale               string
	TimezoneAbbreviation string
	Carrier              string
	ScreenWidth          int64
	ScreenHeight         int64
	ScreenDensity        string
	CPUCoreCount         int64
	ExternalStorageGB    int64
	FreeStorageGB        int64
	DeviceTimeZone       string
}

type ServerEvent struct {
	EventName             EventName
	EventTime             int64
	EventSourceURL        string
	EventID               string
	ActionSource          ActionSource
	Integration           string
	DataProcessingOptions []DataProcessingOption
	TestEventCode         string
	UserData              UserData
	CustomData            *CustomData
	AppData               *AppData
}

type SubmitResult struct {
	StatusCode int
	Status     string
	Reason     string
}

type ValidationEventLog struct {
	Event  int                `json:"event"`
	Status string             `json:"status"`
	Errors ValidationMessages `json:"errors,omitempty"`
}

type ValidationMessages struct {
	Codes    []string `json:"codes,omitempty"`
	Messages []string `json:"error_msgs,omitempty"`
}

type ValidationResult struct {
	StatusCode int
	Status     string
	TestEvent  bool
	Reason     string
	EventLogs  []ValidationEventLog
}

type ValidationLog struct {
	EventName      string   `json:"event_name"`
	EventTime      string   `json:"event_time"`
	ActionSource   string   `json:"action_source"`
	Status         string   `json:"status"`
	ErrorRecords   []string `json:"error_records,omitempty"`
	WarningRecords []string `json:"warning_records,omitempty"`
	AssetID        string   `json:"asset_id"`
	RawEventName   string   `json:"raw_event_name"`
}

type ValidationLogsResult struct {
	Status string          `json:"status"`
	Reason string          `json:"reason"`
	Logs   []ValidationLog `json:"logs"`
}

type ValidationStats struct {
	LatestEventTimestamp int64 `json:"latest_event_ts"`
	EventCountPastHour   int64 `json:"event_count_past_hour"`
}

type ValidationStatsResult struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
	Stats  struct {
		Test ValidationStats `json:"test"`
	} `json:"stats"`
}

type ConversionWorkflow interface {
	SubmitEvents(context.Context, []ServerEvent, ...socialhub.CallOption) (SubmitResult, error)
	ValidateEvents(context.Context, []ServerEvent, ...socialhub.CallOption) (ValidationResult, error)
	GetValidationLogs(context.Context, ...socialhub.CallOption) (ValidationLogsResult, error)
	GetValidationStats(context.Context, ...socialhub.CallOption) (ValidationStatsResult, error)
}

var _ ConversionWorkflow = (*Client)(nil)

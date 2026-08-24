package conversions

import (
	"context"

	"social-hub/pkg/socialhub"
)

const (
	MaximumBatchSize          = 1000
	MaximumTestBatchSize      = 20
	TestRequestQuotaPerSecond = 10
)

// Decimal stores an exact base-10 value and is encoded as a JSON string, as
// required by Pinterest's conversion event contract.
type Decimal string

func (value Decimal) String() string { return string(value) }

type ActionSource string

const (
	ActionSourceAndroid ActionSource = "app_android"
	ActionSourceIOS     ActionSource = "app_ios"
	ActionSourceWeb     ActionSource = "web"
	ActionSourceOffline ActionSource = "offline"
)

type EventName string

const (
	EventAddPaymentInfo    EventName = "add_payment_info"
	EventAddToCart         EventName = "add_to_cart"
	EventAddToWishlist     EventName = "add_to_wishlist"
	EventAppInstall        EventName = "app_install"
	EventAppOpen           EventName = "app_open"
	EventCheckout          EventName = "checkout"
	EventContact           EventName = "contact"
	EventCustom            EventName = "custom"
	EventCustomizeProduct  EventName = "customize_product"
	EventFindLocation      EventName = "find_location"
	EventInitiateCheckout  EventName = "initiate_checkout"
	EventLead              EventName = "lead"
	EventPageVisit         EventName = "page_visit"
	EventSchedule          EventName = "schedule"
	EventSearch            EventName = "search"
	EventSignup            EventName = "signup"
	EventStartTrial        EventName = "start_trial"
	EventSubmitApplication EventName = "submit_application"
	EventSubscribe         EventName = "subscribe"
	EventViewCategory      EventName = "view_category"
	EventViewContent       EventName = "view_content"
	EventWatchVideo        EventName = "watch_video"
)

type OptOutType string

const OptOutLDP OptOutType = "LDP"

type FormFactor string

const (
	FormFactorDesktop    FormFactor = "desktop"
	FormFactorLaptop     FormFactor = "laptop"
	FormFactorCellphone  FormFactor = "cellphone"
	FormFactorTablet     FormFactor = "tablet"
	FormFactorSmartwatch FormFactor = "smartwatch"
	FormFactorTV         FormFactor = "tv"
	FormFactorVR         FormFactor = "vr"
	FormFactorConsole    FormFactor = "console"
	FormFactorOther      FormFactor = "other"
)

type NetworkType string

const (
	NetworkWiFi       NetworkType = "wifi"
	NetworkCellular2G NetworkType = "cellular_2g"
	NetworkCellular3G NetworkType = "cellular_3g"
	NetworkCellular4G NetworkType = "cellular_4g"
	NetworkCellular5G NetworkType = "cellular_5g"
	NetworkCellular6G NetworkType = "cellular_6g"
	NetworkEthernet   NetworkType = "ethernet"
	NetworkUnknown    NetworkType = "unknown"
)

type OSFamily string

const (
	OSFamilyIOS     OSFamily = "ios"
	OSFamilyAndroid OSFamily = "android"
	OSFamilyMacOS   OSFamily = "macos"
	OSFamilyWindows OSFamily = "windows"
	OSFamilyLinux   OSFamily = "linux"
	OSFamilyBSD     OSFamily = "bsd"
	OSFamilyOther   OSFamily = "other"
)

// UserData accepts plaintext or exact lowercase SHA-256 values. Plaintext
// identifiers are normalized and hashed in a temporary wire copy.
type UserData struct {
	Emails               []string
	Phones               []string
	Genders              []string
	DatesOfBirth         []string
	LastNames            []string
	FirstNames           []string
	Cities               []string
	States               []string
	Zips                 []string
	Countries            []string
	ExternalIDs          []string
	MobileAdvertisingIDs []string

	ClientIPAddress string
	ClientUserAgent string
	ClickID         string
	PartnerID       string
}

type Content struct {
	ID           string
	ItemBrand    string
	ItemBrandID  string
	ItemCategory string
	ItemName     string
	ItemPrice    Decimal
	Quantity     *int64
}

type CustomData struct {
	ContentBrand    string
	ContentCategory string
	ContentIDs      []string
	ContentName     string
	Contents        []Content
	Currency        string
	NumItems        *int64
	OptOutType      OptOutType
	OrderID         string
	PredictedLTV    Decimal
	SearchString    string
	Value           Decimal
}

type AppInfo struct {
	AppID          string `json:"app_id,omitempty"`
	AppName        string `json:"app_name,omitempty"`
	AppPackageName string `json:"app_package_name,omitempty"`
	AppStore       string `json:"app_store,omitempty"`
	AppVersion     string `json:"app_version,omitempty"`
	InstallTime    *int64 `json:"install_time,omitempty"`
	UserAgent      string `json:"user_agent,omitempty"`
	WindowHeight   *int   `json:"window_height,omitempty"`
	WindowWidth    *int   `json:"window_width,omitempty"`
}

type DeviceInfo struct {
	BatteryLevel             *int        `json:"battery_level,omitempty"`
	Brand                    string      `json:"brand,omitempty"`
	Carrier                  string      `json:"carrier,omitempty"`
	CPUCores                 *int        `json:"cpu_cores,omitempty"`
	ExternalStorageFreeSpace *int        `json:"external_storage_free_space,omitempty"`
	ExternalStorageSize      *int        `json:"external_storage_size,omitempty"`
	FormFactor               FormFactor  `json:"form_factor,omitempty"`
	KernelVersion            string      `json:"kernel_version,omitempty"`
	Languages                []string    `json:"languages,omitempty"`
	Locale                   string      `json:"locale,omitempty"`
	Model                    string      `json:"model,omitempty"`
	NetworkType              NetworkType `json:"network_type,omitempty"`
	OSFamily                 OSFamily    `json:"os_family,omitempty"`
	OSName                   string      `json:"os_name,omitempty"`
	OSReleaseName            string      `json:"os_release_name,omitempty"`
	OSVersion                string      `json:"os_version,omitempty"`
	ScreenDensity            *int        `json:"screen_density,omitempty"`
	ScreenHeight             *int        `json:"screen_height,omitempty"`
	ScreenWidth              *int        `json:"screen_width,omitempty"`
	StorageFreeSpace         *int        `json:"storage_free_space,omitempty"`
	StorageSize              *int        `json:"storage_size,omitempty"`
	Timezone                 string      `json:"timezone,omitempty"`
	TimezoneAbbreviation     string      `json:"timezone_abbr,omitempty"`
	Type                     string      `json:"type,omitempty"`
}

type ConversionEvent struct {
	ActionSource   ActionSource
	EventID        string
	EventName      EventName
	EventTime      int64
	EventSourceURL string
	OptOut         *bool
	PartnerName    string
	UserData       UserData
	CustomData     *CustomData

	AppID         string
	AppName       string
	AppVersion    string
	DeviceBrand   string
	DeviceCarrier string
	DeviceModel   string
	DeviceType    string
	OSVersion     string
	WiFi          *bool
	Language      string
	AppInfo       *AppInfo
	DeviceInfo    *DeviceInfo
}

type SubmitEventsRequest struct {
	Test   bool
	Events []ConversionEvent
}

type EventStatus string

const (
	EventStatusProcessed EventStatus = "processed"
	EventStatusFailed    EventStatus = "failed"
)

// EventResult intentionally exposes only status flags. Pinterest's free-form
// messages may echo customer data and are discarded after validation.
type EventResult struct {
	Index      int
	Status     EventStatus
	HasError   bool
	HasWarning bool
}

type SubmitResult struct {
	StatusCode      int
	EventsReceived  int
	EventsProcessed int
	Events          []EventResult
}

type ConversionWorkflow interface {
	SubmitEvents(context.Context, SubmitEventsRequest, ...socialhub.CallOption) (SubmitResult, error)
}

var _ ConversionWorkflow = (*Client)(nil)

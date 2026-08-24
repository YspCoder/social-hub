package conversions

import (
	"context"
	"encoding/json"
	"fmt"

	"social-hub/pkg/socialhub"
)

// Decimal preserves an exact finite JSON number without float64 rounding.
// Construct it from a base-10 string such as "19.95".
type Decimal string

func (value Decimal) String() string { return string(value) }

func (value Decimal) MarshalJSON() ([]byte, error) {
	if !validDecimal(value) {
		return nil, fmt.Errorf("conversions: invalid decimal")
	}
	return json.Marshal(json.Number(value))
}

type ActionSource string

const (
	ActionSourceWebsite           ActionSource = "website"
	ActionSourceApp               ActionSource = "app"
	ActionSourcePhysicalStore     ActionSource = "physical_store"
	ActionSourceSystemGenerated   ActionSource = "system_generated"
	ActionSourceBusinessMessaging ActionSource = "business_messaging"
	ActionSourceChat              ActionSource = "chat"
	ActionSourceEmail             ActionSource = "email"
	ActionSourceOther             ActionSource = "other"
	ActionSourcePhoneCall         ActionSource = "phone_call"
)

type MessagingChannel string

const (
	MessagingChannelMessenger MessagingChannel = "messenger"
	MessagingChannelWhatsApp  MessagingChannel = "whatsapp"
	MessagingChannelInstagram MessagingChannel = "instagram"
)

type DeliveryCategory string

const (
	DeliveryCategoryInStore      DeliveryCategory = "in_store"
	DeliveryCategoryCurbside     DeliveryCategory = "curbside"
	DeliveryCategoryHomeDelivery DeliveryCategory = "home_delivery"
)

type DataProcessingOption string

const DataProcessingOptionLDU DataProcessingOption = "LDU"

// UserData accepts plaintext customer information or lowercase SHA-256
// digests. Plaintext hash-required fields are normalized and hashed only in
// the request wire copy; the caller's value is never mutated.
type UserData struct {
	Emails       []string
	Phones       []string
	Genders      []string
	DatesOfBirth []string
	FirstNames   []string
	LastNames    []string
	Cities       []string
	States       []string
	Zips         []string
	Countries    []string
	ExternalIDs  []string

	ClientIPAddress string
	ClientUserAgent string
	FBC             string
	FBP             string
	SubscriptionID  string
	FBLoginID       string
	LeadID          string
	F5First         string
	F5Last          string
	FirstInitial    string
	BirthDay        string
	BirthMonth      string
	BirthYear       string
	MobileAdID      string
	AnonymousID     string
	AppUserID       string
	CTWAClID        string
	PageID          string
}

// CustomData contains standard CAPI commerce fields and scalar-only custom
// properties. Separate maps prevent arbitrary JSON values from entering the
// event contract.
type CustomData struct {
	Value             Decimal
	NetRevenue        Decimal
	Currency          string
	ContentName       string
	ContentCategory   string
	ContentIDs        []string
	Contents          []Content
	ContentType       string
	OrderID           string
	PredictedLTV      Decimal
	NumItems          *int64
	SearchString      string
	Status            string
	ItemNumber        string
	DeliveryCategory  DeliveryCategory
	StringProperties  map[string]string
	NumberProperties  map[string]Decimal
	BooleanProperties map[string]bool
}

type Content struct {
	ID               string
	Quantity         int64
	ItemPrice        Decimal
	Title            string
	Description      string
	Category         string
	Brand            string
	DeliveryCategory DeliveryCategory
}

// AppData mirrors the stable v26 Business SDK app_data contract. CampaignIDs
// and URLSchemes are wire strings in Meta's contract, not JSON arrays.
type AppData struct {
	ApplicationTrackingEnabled *bool
	AdvertiserTrackingEnabled  *bool
	CampaignIDs                string
	ConsiderViews              *bool
	ExtendedDeviceInfo         *ExtendedDeviceInfo
	IncludeDwellData           *bool
	IncludeVideoData           *bool
	InstallReferrer            string
	InstallerPackage           string
	ReceiptData                string
	URLSchemes                 string
	WindowsAttributionID       string
}

// ExtendedDeviceInfo is deterministically encoded as Meta's fixed 16-slot
// extinfo array.
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
	TotalDiskSpaceGB     int64
	FreeDiskSpaceGB      int64
	DeviceTimeZone       string
}

type ServerEvent struct {
	EventName                    string
	EventTime                    int64
	EventSourceURL               string
	EventID                      string
	ActionSource                 ActionSource
	OptOut                       *bool
	UserData                     UserData
	CustomData                   *CustomData
	AppData                      *AppData
	DataProcessingOptions        []DataProcessingOption
	DataProcessingOptionsCountry *int
	DataProcessingOptionsState   *int
	AdvertiserTrackingEnabled    *bool
	MessagingChannel             MessagingChannel
	ReferrerURL                  string
}

// SubmitEventsRequest contains one atomic request envelope. Upload fields are
// retained for offline dataset ingestion and TestEventCode targets Events
// Manager's test-event view.
type SubmitEventsRequest struct {
	Events        []ServerEvent
	PartnerAgent  string
	TestEventCode string
	NamespaceID   string
	UploadID      string
	UploadTag     string
	UploadSource  string
}

// SubmitResult preserves optional response counters because Meta does not
// return every counter for every ingestion path.
type SubmitResult struct {
	StatusCode          int
	EventsReceived      *int
	MessageCount        int
	TraceID             string
	DatasetID           string
	NumProcessedEntries *int
}

type ConversionWorkflow interface {
	SubmitEvents(context.Context, SubmitEventsRequest, ...socialhub.CallOption) (SubmitResult, error)
}

var _ ConversionWorkflow = (*Client)(nil)

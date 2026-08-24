package conversions

import (
	"context"
	"fmt"

	"social-hub/pkg/socialhub"
)

const MaximumBatchSize = 1000

// Decimal stores an exact non-negative base-10 number. It is encoded as a JSON
// number without conversion through float64.
type Decimal string

func (value Decimal) String() string { return string(value) }

func (value Decimal) MarshalJSON() ([]byte, error) {
	if !validDecimal(value) {
		return nil, fmt.Errorf("tiktok conversions: invalid decimal")
	}
	return []byte(value), nil
}

type EventSource string

const (
	EventSourceWeb     EventSource = "web"
	EventSourceApp     EventSource = "app"
	EventSourceOffline EventSource = "offline"
	EventSourceCRM     EventSource = "crm"
)

type EventName string

const (
	EventAddPaymentInfo       EventName = "AddPaymentInfo"
	EventAddToCart            EventName = "AddToCart"
	EventAddToWishlist        EventName = "AddToWishlist"
	EventApplicationApproval  EventName = "ApplicationApproval"
	EventCompleteRegistration EventName = "CompleteRegistration"
	EventContact              EventName = "Contact"
	EventCustomizeProduct     EventName = "CustomizeProduct"
	EventDownload             EventName = "Download"
	EventFindLocation         EventName = "FindLocation"
	EventInitiateCheckout     EventName = "InitiateCheckout"
	EventLead                 EventName = "Lead"
	EventPurchase             EventName = "Purchase"
	EventSchedule             EventName = "Schedule"
	EventSearch               EventName = "Search"
	EventStartTrial           EventName = "StartTrial"
	EventSubmitApplication    EventName = "SubmitApplication"
	EventSubscribe            EventName = "Subscribe"
	EventViewContent          EventName = "ViewContent"

	EventAchieveLevel      EventName = "AchieveLevel"
	EventCheckout          EventName = "Checkout"
	EventCompleteTutorial  EventName = "CompleteTutorial"
	EventCreateGroup       EventName = "CreateGroup"
	EventCreateRole        EventName = "CreateRole"
	EventGenerateLead      EventName = "GenerateLead"
	EventInAppADClick      EventName = "InAppADClick"
	EventInAppADImpression EventName = "InAppADImpr"
	EventInstallApp        EventName = "InstallApp"
	EventJoinGroup         EventName = "JoinGroup"
	EventLaunchApp         EventName = "LaunchAPP"
	EventLoanApplication   EventName = "LoanApplication"
	EventLoanApproval      EventName = "LoanApproval"
	EventLoanDisbursal     EventName = "LoanDisbursal"
	EventLogin             EventName = "Login"
	EventRate              EventName = "Rate"
	EventRegistration      EventName = "Registration"
	EventSpendCredits      EventName = "SpendCredits"
	EventUnlockAchievement EventName = "UnlockAchievement"
)

type ContentType string

const (
	ContentTypeProduct      ContentType = "product"
	ContentTypeProductGroup ContentType = "product_group"
)

type CustomerType string

const (
	CustomerNew       CustomerType = "new"
	CustomerReturning CustomerType = "returning"
)

type ATTStatus string

const (
	ATTAuthorized    ATTStatus = "AUTHORIZED"
	ATTDenied        ATTStatus = "DENIED"
	ATTNotDetermined ATTStatus = "NOT_DETERMINED"
	ATTRestricted    ATTStatus = "RESTRICTED"
	ATTNotApplicable ATTStatus = "NOT_APPLICABLE"
)

// User accepts plaintext for fields TikTok requires to be SHA-256 hashed.
// Lowercase SHA-256 digests are also accepted. Normalization never mutates it.
type User struct {
	TikTokClickID string
	Emails        []string
	Phones        []string
	ExternalIDs   []string
	TikTokCookie  string
	IP            string
	UserAgent     string
	FirstName     string
	LastName      string
	City          string
	State         string
	Country       string
	ZipCode       string
	IDFA          string
	IDFV          string
	GAID          string
	Locale        string
	ATTStatus     ATTStatus
}

type Content struct {
	Price           Decimal
	Quantity        *int64
	ContentID       string
	ContentCategory string
	ContentName     string
	Brand           string
}

type Properties struct {
	ContentIDs   []string
	Contents     []Content
	ContentType  ContentType
	Currency     string
	Value        Decimal
	NumItems     *int64
	SearchString string
	Description  string
	OrderID      string
	ShopID       string
	CustomerType CustomerType
}

type Page struct {
	URL      string `json:"url"`
	Referrer string `json:"referrer,omitempty"`
}

type App struct {
	AppID      string `json:"app_id"`
	AppName    string `json:"app_name,omitempty"`
	AppVersion string `json:"app_version,omitempty"`
}

// Ad describes optional first-party attribution information. String fields
// intentionally remain open where TikTok documents examples rather than enums.
type Ad struct {
	Callback                 string  `json:"callback,omitempty"`
	CampaignID               string  `json:"campaign_id,omitempty"`
	AdID                     string  `json:"ad_id,omitempty"`
	CreativeID               string  `json:"creative_id,omitempty"`
	IsRetargeting            *bool   `json:"is_retargeting,omitempty"`
	Attributed               *bool   `json:"attributed,omitempty"`
	AttributionType          string  `json:"attribution_type,omitempty"`
	AttributionProvider      string  `json:"attribution_provider,omitempty"`
	AttributionShare         Decimal `json:"attribution_share,omitempty"`
	AttributionValue         Decimal `json:"attribution_value,omitempty"`
	AttributionModel         string  `json:"attribution_model,omitempty"`
	TouchpointType           string  `json:"touchpoint_type,omitempty"`
	TouchpointTime           *int64  `json:"touchpoint_ts,omitempty"`
	TouchpointURL            string  `json:"touchpoint_url,omitempty"`
	ClickAttributionWindowHR *int64  `json:"attribution_window_hr_click,omitempty"`
	ViewAttributionWindowHR  *int64  `json:"attribution_window_hr_view,omitempty"`
	AttributionMethod        string  `json:"attribution_method,omitempty"`
	DeclineReason            string  `json:"decline_reason,omitempty"`
	UTMID                    string  `json:"utm_id,omitempty"`
	UTMSource                string  `json:"utm_source,omitempty"`
	UTMMedium                string  `json:"utm_medium,omitempty"`
	UTMCampaign              string  `json:"utm_campaign,omitempty"`
}

type Lead struct {
	LeadID          string `json:"lead_id"`
	LeadEventSource string `json:"lead_event_source,omitempty"`
}

type ConversionEvent struct {
	Event          EventName
	EventTime      int64
	EventID        string
	User           *User
	Properties     *Properties
	Page           *Page
	App            *App
	Ad             *Ad
	LimitedDataUse *bool
	Lead           *Lead
}

type SubmitEventsRequest struct {
	TestEventCode string
	Events        []ConversionEvent
}

// SubmitResult deliberately excludes TikTok's free-text response message.
type SubmitResult struct {
	StatusCode     int
	RequestID      string
	EventsAccepted int
}

type EventWorkflow interface {
	SubmitEvents(context.Context, SubmitEventsRequest, ...socialhub.CallOption) (SubmitResult, error)
}

var _ EventWorkflow = (*Client)(nil)

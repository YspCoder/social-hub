package appsflyer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"social-hub/pkg/socialhub"
)

const MaximumRequestBytes = 1024

type Platform string

const (
	PlatformIOS     Platform = "ios"
	PlatformAndroid Platform = "android"
	PlatformWindows Platform = "windows"
)

// Decimal preserves an exact finite JSON number without float64 rounding.
type Decimal string

func (value Decimal) String() string { return string(value) }

func (value Decimal) MarshalJSON() ([]byte, error) {
	if !validDecimal(value) {
		return nil, fmt.Errorf("appsflyer: invalid decimal")
	}
	return json.Marshal(json.Number(value))
}

// EventValues are stringified into AppsFlyer's eventValue JSON string. An
// empty map is sent as the required empty string.
type EventValues map[string]string

// CustomData is stringified into AppsFlyer's custom_data JSON string. Objects
// may be nested; exact numbers avoid float64 rounding.
type CustomData struct {
	Strings  map[string]string
	Numbers  map[string]Decimal
	Booleans map[string]bool
	Objects  map[string]CustomData
}

type DeviceIdentifiers struct {
	// Android identifiers.
	AdvertisingID string
	OAID          string
	AmazonAID     string
	IMEI          string
	// Apple identifiers.
	IDFA string
	IDFV string
	// FBLoginID is accepted by both Android and iOS event schemas.
	FBLoginID string
}

// HashedUserData accepts only pre-normalized lowercase SHA-256 digests.
type HashedUserData struct {
	Email     string
	Phone     string
	PhoneE164 string
	FirstName string
	LastName  string
}

type SharingFilter struct {
	BlockAll bool
	Partners []string
}

type AppType string

const AppTypeAppClip AppType = "app_clip"

type AppSetIDScope int

const (
	AppSetIDScopeApp       AppSetIDScope = 1
	AppSetIDScopeDeveloper AppSetIDScope = 2
)

type AppSetID struct {
	Scope AppSetIDScope
	ID    string
}

type ConsentData struct {
	Manual *ManualConsent
	TCF    *TCFConsent
}

type ManualConsent struct {
	GDPRApplies              *bool
	AdUserDataEnabled        *bool
	AdPersonalizationEnabled *bool
}

type TCFConsent struct {
	PolicyVersion int
	CMPSDKID      int
	CMPSDKVersion int
	GDPRApplies   int
	TCString      string
}

type EventRequest struct {
	AppsFlyerID      string
	EventName        string
	EventValue       EventValues
	EventTime        *time.Time
	EventCurrency    string
	BundleIdentifier string
	AppVersionName   string
	AppStore         string
	OS               string
	UserAgent        string
	IPAddress        string
	CustomerUserID   string
	Device           DeviceIdentifiers
	HashedUser       HashedUserData
	SharingFilter    SharingFilter
	CustomData       CustomData
	AppType          AppType
	AIE              *bool
	ATT              *int
	Consent          *ConsentData
	AppSetID         *AppSetID
}

type SubmitResult struct {
	StatusCode int
}

type EventWorkflow interface {
	SendEvent(context.Context, EventRequest, ...socialhub.CallOption) (SubmitResult, error)
}

var _ EventWorkflow = (*Client)(nil)

package adjust

import (
	"context"
	"time"

	"social-hub/pkg/socialhub"
)

const MaximumRequestBytes = 1 << 20

// Decimal preserves a base-10 monetary value without float64 rounding.
type Decimal string

func (value Decimal) String() string { return string(value) }

type Environment string

const (
	EnvironmentSandbox    Environment = "sandbox"
	EnvironmentProduction Environment = "production"
)

type OSName string

const (
	OSIOS         OSName = "ios"
	OSAndroid     OSName = "android"
	OSAndroidTV   OSName = "android-tv"
	OSFireTV      OSName = "fire-tv"
	OSRoku        OSName = "roku-os"
	OSTizen       OSName = "tizen"
	OSWindows     OSName = "windows"
	OSXbox        OSName = "xbox"
	OSPlayStation OSName = "playstation"
	OSServer      OSName = "server"
)

// DeviceIdentifiers contains the public preferred and fallback identifiers
// accepted by the S2S API. At least one must be present on every request.
type DeviceIdentifiers struct {
	VIDA               string
	RIDA               string
	TIFA               string
	IDFA               string
	GPSADID            string
	FireADID           string
	OAID               string
	WebUUID            string
	ADID               string
	IDFV               string
	AndroidID          string
	ExternalDeviceID   string
	PersistentIOSUUID  string
	AndroidIDLowerMD5  string
	AndroidIDLowerSHA1 string
	AndroidIDUpperMD5  string
	AndroidIDUpperSHA1 string
	IMEI               string
	IMEILowerMD5       string
	MEID               string
	WindowsNAID        string
	WindowsHardwareID  string
}

type EventRequest struct {
	EventToken     string
	Device         DeviceIdentifiers
	CreatedAt      *time.Time
	Environment    Environment
	IPAddress      string
	UserAgent      string
	Revenue        Decimal
	Currency       string
	CallbackParams map[string]string
	PartnerParams  map[string]string
}

type SessionRequest struct {
	Device             DeviceIdentifiers
	OSName             OSName
	CreatedAt          *time.Time
	SentAt             *time.Time
	Environment        Environment
	IPAddress          string
	ForwardedFor       string
	UserAgent          string
	AppVersion         string
	AppVersionShort    string
	SessionCount       *int64
	SubsessionCount    *int64
	SessionLength      *int64
	TimeSpent          *int64
	TrackingEnabled    *bool
	ATTStatus          *int
	BundleID           string
	PackageName        string
	Country            string
	Language           string
	OSVersion          string
	CPUType            string
	DeviceType         string
	DeviceName         string
	HardwareName       string
	InstallReceipt     string
	PrimaryDedupeToken string
	GoogleAppSetID     string
	EEA                *bool
	AdPersonalization  *bool
	AdUserData         *bool
	NPA                *bool
	AmazonDMA          *AmazonDMAConsent
	CallbackParams     map[string]string
	PartnerParams      map[string]string
}

type AmazonDMAConsent struct {
	AdUserData *bool
	AdStorage  *bool
}

type AdRevenueRequest struct {
	Device             DeviceIdentifiers
	Revenue            Decimal
	Currency           string
	AdImpressionsCount int64
	CreatedAt          *time.Time
	Environment        Environment
	CallbackParams     map[string]string
	Network            string
	Unit               string
	Placement          string
}

type SubmitResult struct {
	StatusCode int
}

type SessionResult struct {
	ADID      string `json:"adid"`
	Timestamp string `json:"timestamp"`
	Message   string `json:"message"`
	AskIn     int64  `json:"ask_in"`
}

type Workflow interface {
	TrackEvent(context.Context, EventRequest, ...socialhub.CallOption) (SubmitResult, error)
	TrackSession(context.Context, SessionRequest, ...socialhub.CallOption) (*SessionResult, error)
	TrackAdRevenue(context.Context, AdRevenueRequest, ...socialhub.CallOption) (SubmitResult, error)
}

var _ Workflow = (*Client)(nil)

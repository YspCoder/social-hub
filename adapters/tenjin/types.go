package tenjin

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
		return nil, fmt.Errorf("tenjin: invalid decimal")
	}
	return json.Marshal(json.Number(value))
}

type Platform string

const (
	PlatformIOS          Platform = "ios"
	PlatformAndroid      Platform = "android"
	PlatformAmazon       Platform = "amazon"
	PlatformAndroidOther Platform = "android_other"
)

type ATTStatus int

const (
	ATTNotDetermined ATTStatus = iota
	ATTRestricted
	ATTDenied
	ATTAuthorized
)

// DeviceIdentity contains the identifiers accepted by Tenjin. The analytics
// installation ID must remain stable across opens, events, and purchases.
type DeviceIdentity struct {
	AnalyticsInstallationID string
	AdvertisingID           string
	DeveloperDeviceID       string
}

// EventContext contains the device and consent fields shared by Tenjin's
// open, custom-event, and purchase endpoints.
type EventContext struct {
	Identity          DeviceIdentity
	OSVersion         string
	AppVersion        string
	LimitAdTracking   *bool
	Country           string
	IPAddress         string
	AdUserData        *bool
	AdPersonalization *bool
	OSVersionRelease  string
	BuildID           string
	Locale            string
	DeviceModel       string
	CustomerUserID    string
	TrackingStatus    *ATTStatus
}

type OpenRequest struct {
	Context  EventContext
	Referrer string
	ODMInfo  string
}

type EventName string

type CustomEventRequest struct {
	Context EventContext
	Name    EventName
	Value   *int64
}

type PurchaseRequest struct {
	Context          EventContext
	ProductID        string
	Price            Decimal
	Quantity         int64
	Currency         string
	AfterPlatformCut *bool
}

type Mediation string

const (
	MediationMAX        Mediation = "max"
	MediationIronSource Mediation = "ironsource"
	MediationAdMob      Mediation = "admob"
	MediationTopOn      Mediation = "topon"
	MediationCAS        Mediation = "cas"
	MediationTradPlus   Mediation = "tradplus"
	MediationCustom     Mediation = "custom"
)

type ConnectionType string

const (
	ConnectionMobile ConnectionType = "mobile"
	ConnectionWiFi   ConnectionType = "wifi"
)

type SourceAppStore string

const (
	SourceStoreUnspecified SourceAppStore = "unspecified"
	SourceStoreGooglePlay  SourceAppStore = "googleplay"
	SourceStoreAmazon      SourceAppStore = "amazon"
	SourceStoreOther       SourceAppStore = "other"
)

type AdFormat string

const (
	AdFormatBanner               AdFormat = "banner"
	AdFormatMREC                 AdFormat = "mrec"
	AdFormatCrossPromotion       AdFormat = "xpromo"
	AdFormatNative               AdFormat = "native"
	AdFormatLeaderboard          AdFormat = "leaderboard"
	AdFormatLeader               AdFormat = "leader"
	AdFormatInterstitial         AdFormat = "interstitial"
	AdFormatInter                AdFormat = "inter"
	AdFormatRewarded             AdFormat = "rewarded"
	AdFormatReward               AdFormat = "reward"
	AdFormatRewardedInterstitial AdFormat = "rewarded_interstitial"
	AdFormatRewardedInter        AdFormat = "rewarded_inter"
)

// AdImpressionContext carries Tenjin's generic ILRD device envelope.
type AdImpressionContext struct {
	Identity           DeviceIdentity
	AppVersion         string
	AppVersionCode     string
	BuildID            string
	Carrier            string
	Connection         ConnectionType
	Country            string
	Device             string
	DeviceBrand        string
	DeviceManufacturer string
	DeviceModel        string
	DeviceProduct      string
	Language           string
	LimitAdTracking    *bool
	OptIn              *bool
	Locale             string
	OSVersion          string
	OSVersionRelease   string
	Timezone           string
	UserAgent          string
	ScreenHeight       *int64
	ScreenWidth        *int64
	SentAt             *time.Time
	SessionID          string
	SourceAppStore     SourceAppStore
	IPAddress          string
	AdUserData         *bool
	AdPersonalization  *bool
}

type AdImpressionRequest struct {
	Context          AdImpressionContext
	Mediation        Mediation
	NetworkName      string
	Currency         string
	RevenueDecimal   Decimal
	RevenueCPM       Decimal
	MediationCountry string
	AdUnitID         string
	Format           AdFormat
	Precision        string
	CreativeID       string
	Placement        string
	NetworkPlacement string
	AuctionID        string
}

type SubmitResult struct {
	StatusCode int
}

type S2SWorkflow interface {
	TrackOpen(context.Context, OpenRequest, ...socialhub.CallOption) (SubmitResult, error)
	TrackCustomEvent(context.Context, CustomEventRequest, ...socialhub.CallOption) (SubmitResult, error)
	TrackPurchase(context.Context, PurchaseRequest, ...socialhub.CallOption) (SubmitResult, error)
	TrackAdImpression(context.Context, AdImpressionRequest, ...socialhub.CallOption) (SubmitResult, error)
}

var _ S2SWorkflow = (*Client)(nil)

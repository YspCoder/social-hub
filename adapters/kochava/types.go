package kochava

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"social-hub/pkg/socialhub"
)

const MaximumPayloadBytes = 2 << 20

// Decimal preserves a finite plain JSON number without float64 rounding.
type Decimal string

func (value Decimal) String() string { return string(value) }

func (value Decimal) MarshalJSON() ([]byte, error) {
	if !validDecimal(value) {
		return nil, fmt.Errorf("kochava: invalid decimal")
	}
	return json.Marshal(json.Number(value))
}

// Properties is a flat, type-safe event_data object. Kochava ignores nested
// objects and arrays for SKAdNetwork conversion processing.
type Properties struct {
	Strings     map[string]string
	Numbers     map[string]Decimal
	Booleans    map[string]bool
	StringLists map[string][]string
}

// DeviceIdentifiers contains the documented mobile and non-mobile identifiers.
// Custom identifiers should be provisioned with the Kochava account team.
type DeviceIdentifiers struct {
	IDFA      string
	IDFV      string
	ADID      string
	AndroidID string
	OpenUDID  string
	UDID      string
	Custom    map[string]string
}

type ATTDetail string

const (
	ATTAuthorized    ATTDetail = "authorized"
	ATTDenied        ATTDetail = "denied"
	ATTNotDetermined ATTDetail = "notDetermined"
	ATTRestricted    ATTDetail = "restricted"
)

// AppTrackingTransparency carries the iOS 14+ consent result. Authorized is a
// pointer so an explicit false signal remains distinguishable from omission.
type AppTrackingTransparency struct {
	Authorized        *bool
	AuthorizationTime *time.Time
	ResponseDuration  *int64
	Detail            ATTDetail
}

// GDPRPrivacyConsent models Kochava's Google DMA consent object.
type GDPRPrivacyConsent struct {
	GDPRApplies       *bool
	TCString          string
	AdUserData        *bool
	AdPersonalization *bool
}

// DeviceContext contains fields shared by install and post-install events.
type DeviceContext struct {
	DeviceIDs          DeviceIdentifiers
	OccurredAt         *time.Time
	OriginationIP      string
	DeviceUserAgent    string
	DeviceVersion      string
	AppVersion         string
	LimitTracking      *bool
	ATT                *AppTrackingTransparency
	GDPRPrivacyConsent *GDPRPrivacyConsent
}

// IAdAttribution models the Version3.1 legacy Apple Search Ads claim fields
// still accepted by Kochava alongside AdServices data.
type IAdAttribution struct {
	PurchaseDate     string
	Keyword          string
	AdGroupID        string
	CreativeSetID    string
	CreativeSetName  string
	CampaignID       string
	LineItemID       string
	OrganizationID   string
	ConversionDate   string
	KeywordID        string
	ConversionType   string
	CountryOrRegion  string
	OrganizationName string
	CampaignName     string
	ClickDate        string
	Attributed       string
	AdGroupName      string
	KeywordMatchType string
	LineItemName     string
}

// AdServicesAttribution models the current Apple Ads Attribution API result.
type AdServicesAttribution struct {
	KeywordID       *int64
	ConversionType  string
	CreativeSetID   *int64
	OrganizationID  *int64
	CampaignID      *int64
	AdGroupID       *int64
	ClickDate       string
	CountryOrRegion string
	Attributed      *bool
}

type AppleSearchAds struct {
	AdServicesToken       string
	IAd                   *IAdAttribution
	AdServicesAttribution *AdServicesAttribution
}

type InstallReferrer struct {
	Referrer  string
	ClickTime *time.Time
}

type InstallRequest struct {
	KochavaDeviceID string
	Context         DeviceContext
	AppleSearchAds  *AppleSearchAds
	InstallReferrer *InstallReferrer
}

type EventName string

const EventAdView EventName = "Ad View"

type EventRequest struct {
	KochavaDeviceID string
	Context         DeviceContext
	Name            EventName
	Currency        string
	Data            Properties
}

type UpdateIDFARequest struct {
	KochavaDeviceID string
	IDFA            string
}

type SubmitResult struct {
	StatusCode int
}

type S2SWorkflow interface {
	TrackInstall(context.Context, InstallRequest, ...socialhub.CallOption) (SubmitResult, error)
	TrackEvent(context.Context, EventRequest, ...socialhub.CallOption) (SubmitResult, error)
	UpdateIDFA(context.Context, UpdateIDFARequest, ...socialhub.CallOption) (SubmitResult, error)
}

var _ S2SWorkflow = (*Client)(nil)

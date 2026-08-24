package unitypublisher

import (
	"encoding/json"
	"time"
)

// MutationOptions controls write-only query parameters.
type MutationOptions struct {
	DryRun bool
}

type Platform string

const (
	PlatformAndroid      Platform = "Android"
	PlatformIOS          Platform = "iOS"
	PlatformOSX          Platform = "OSX"
	PlatformWindows      Platform = "Windows"
	PlatformLinux        Platform = "Linux"
	PlatformWebGL        Platform = "WebGL"
	PlatformWindowsStore Platform = "Windows_Store"
	PlatformPS4          Platform = "PS4"
	PlatformPS5          Platform = "PS5"
	PlatformXboxOne      Platform = "XboxOne"
	PlatformTVOS         Platform = "tvOS"
	PlatformSwitch       Platform = "Switch"
	PlatformVisionOS     Platform = "VisionOS"
)

type Store string

const (
	StoreGooglePlay       Store = "GooglePlay"
	StoreAppleAppStore    Store = "AppleAppStore"
	StoreSamsungGalaxy    Store = "SamsungGalaxy"
	StoreAmazonAppStore   Store = "AmazonAppStore"
	StoreMacAppStore      Store = "MacAppStore"
	StoreUDP              Store = "UDP"
	StoreMicrosoftStore   Store = "MicrosoftStore"
	StoreHuaweiAppGallery Store = "HuaweiAppGallery"
	StoreAPK              Store = "APK"
)

type TestMode string

const (
	TestModeForceAll TestMode = "forceAll"
	TestModeForceOff TestMode = "forceOff"
)

type AdFormat string

const (
	AdFormatRewarded     AdFormat = "rewarded"
	AdFormatInterstitial AdFormat = "interstitial"
	AdFormatBanner       AdFormat = "banner"
	AdFormatNative       AdFormat = "native"
)

type PlacementStatus string

const (
	PlacementActive PlacementStatus = "active"
	PlacementPaused PlacementStatus = "paused"
)

type OrganizationPlacementPlatform string

const (
	OrganizationPlatformIOS     OrganizationPlacementPlatform = "ios"
	OrganizationPlatformAndroid OrganizationPlacementPlatform = "android"
	OrganizationPlatformOther   OrganizationPlacementPlatform = ""
)

type Privacy struct {
	COPPA         bool `json:"coppa"`
	MixedAudience bool `json:"mixedAudience"`
}

type PrivacyUpdate struct {
	COPPA         *bool `json:"coppa,omitempty"`
	MixedAudience *bool `json:"mixedAudience,omitempty"`
}

type Application struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	GameID        *int64          `json:"gameId"`
	Platform      Platform        `json:"platform"`
	IconURL       *string         `json:"iconUrl"`
	ProjectID     *string         `json:"projectId"`
	StoreID       *string         `json:"storeId"`
	Store         *Store          `json:"store"`
	TestMode      *TestMode       `json:"testMode"`
	KidsSettings  bool            `json:"kidsSettings"`
	COPPA         *bool           `json:"coppa"`
	MixedAudience *bool           `json:"mixedAudience"`
	Raw           json.RawMessage `json:"-"`
}

type CreateApplicationRequest struct {
	Name        string   `json:"name"`
	Platform    Platform `json:"platform"`
	IconURL     *string  `json:"iconUrl,omitempty"`
	StoreID     *string  `json:"storeId,omitempty"`
	Store       *Store   `json:"store,omitempty"`
	Privacy     *Privacy `json:"privacy,omitempty"`
	ProjectID   *string  `json:"projectId,omitempty"`
	ProjectName *string  `json:"projectName,omitempty"`
}

type UpdateApplicationRequest struct {
	Name         *string        `json:"name,omitempty"`
	StoreID      *string        `json:"storeId,omitempty"`
	Store        *Store         `json:"store,omitempty"`
	Privacy      *PrivacyUpdate `json:"privacy,omitempty"`
	KidsSettings *bool          `json:"kidsSettings,omitempty"`
}

type ApplicationTestMode struct {
	ID       string          `json:"id"`
	GameID   *int64          `json:"gameId"`
	TestMode *TestMode       `json:"testMode"`
	Raw      json.RawMessage `json:"-"`
}

type UpdateTestModeRequest struct {
	TestMode TestMode `json:"testMode"`
}

// PlacementConfiguration is the format-specific request configuration union.
type PlacementConfiguration interface {
	isPlacementConfiguration()
}

type AdminConfigurations struct {
	AllowSkip                     bool    `json:"allowSkip"`
	AllowSkipInSeconds            float64 `json:"allowSkipInSeconds"`
	VideoPlayableSkipInSeconds    float64 `json:"videoPlayableSkipInSeconds"`
	CloseTimerDuration            float64 `json:"closeTimerDuration"`
	TapsToClose                   float64 `json:"tapsToClose"`
	MuteVideo                     bool    `json:"muteVideo"`
	DisableVideoControlsFade      bool    `json:"disableVideoControlsFade"`
	UseCloseIconInsteadOfSkipIcon bool    `json:"useCloseIconInsteadOfSkipIcon"`
}

type RewardedConfigurations struct {
	Name          string               `json:"name"`
	Value         float64              `json:"value"`
	AdminSettings *AdminConfigurations `json:"adminSettings,omitempty"`
}

func (RewardedConfigurations) isPlacementConfiguration() {}

type InterstitialConfigurations struct {
	AdminSettings *AdminConfigurations `json:"adminSettings,omitempty"`
}

func (InterstitialConfigurations) isPlacementConfiguration() {}

type BannerConfigurations struct {
	AdminSettings     *AdminConfigurations `json:"adminSettings,omitempty"`
	BannerRefreshRate float64              `json:"bannerRefreshRate"`
}

func (BannerConfigurations) isPlacementConfiguration() {}

type PlacementRequest struct {
	Name                   string                 `json:"name"`
	AdFormat               AdFormat               `json:"adFormat"`
	AdFormatConfigurations PlacementConfiguration `json:"adFormatConfigurations,omitempty"`
}

type Placement struct {
	ID                     string          `json:"id"`
	Key                    string          `json:"key"`
	Name                   string          `json:"name"`
	GameID                 *int64          `json:"gameId"`
	AdFormat               AdFormat        `json:"adFormat"`
	Status                 PlacementStatus `json:"status"`
	AdFormatConfigurations json.RawMessage `json:"adFormatConfigurations,omitempty"`
	ApplicationID          string          `json:"applicationId"`
	ArchivedAt             *time.Time      `json:"archivedAt"`
	CreatedAt              time.Time       `json:"createdAt"`
	UpdatedAt              time.Time       `json:"updatedAt"`
	Raw                    json.RawMessage `json:"-"`
}

type OrganizationPlacement struct {
	PlacementID   string                        `json:"placementId"`
	Name          string                        `json:"name"`
	PlacementType string                        `json:"placementType"`
	GameID        *int64                        `json:"gameId"`
	AdFormat      AdFormat                      `json:"adFormat"`
	Platform      OrganizationPlacementPlatform `json:"platform"`
	StoreID       *string                       `json:"storeId"`
	Raw           json.RawMessage               `json:"-"`
}

type ListApplicationPlacementsRequest struct {
	IsArchived *bool
	AdFormats  []AdFormat
}

// NullablePlatform distinguishes an omitted patch field from JSON null.
// A non-nil NullablePlatform with Value nil explicitly clears the platform.
type NullablePlatform struct {
	Value *Platform
}

func (value NullablePlatform) MarshalJSON() ([]byte, error) {
	return json.Marshal(value.Value)
}

type TestDevice struct {
	ID            string          `json:"id"`
	Platform      *Platform       `json:"platform"`
	Name          string          `json:"name"`
	AdvertisingID string          `json:"advertisingId"`
	Raw           json.RawMessage `json:"-"`
}

type CreateTestDeviceRequest struct {
	Platform      *NullablePlatform `json:"platform,omitempty"`
	Name          string            `json:"name"`
	AdvertisingID string            `json:"advertisingId"`
}

type UpdateTestDeviceRequest struct {
	Platform      *NullablePlatform `json:"platform,omitempty"`
	Name          *string           `json:"name,omitempty"`
	AdvertisingID *string           `json:"advertisingId,omitempty"`
}

func (value *Application) UnmarshalJSON(data []byte) error {
	return captureRaw(data, (*applicationAlias)(value), &value.Raw)
}

func (value *ApplicationTestMode) UnmarshalJSON(data []byte) error {
	return captureRaw(data, (*applicationTestModeAlias)(value), &value.Raw)
}

func (value *Placement) UnmarshalJSON(data []byte) error {
	return captureRaw(data, (*placementAlias)(value), &value.Raw)
}

func (value *OrganizationPlacement) UnmarshalJSON(data []byte) error {
	return captureRaw(data, (*organizationPlacementAlias)(value), &value.Raw)
}

func (value *TestDevice) UnmarshalJSON(data []byte) error {
	return captureRaw(data, (*testDeviceAlias)(value), &value.Raw)
}

type applicationAlias Application
type applicationTestModeAlias ApplicationTestMode
type placementAlias Placement
type organizationPlacementAlias OrganizationPlacement
type testDeviceAlias TestDevice

func captureRaw(data []byte, target any, raw *json.RawMessage) error {
	if err := json.Unmarshal(data, target); err != nil {
		return err
	}
	*raw = append((*raw)[:0], data...)
	return nil
}

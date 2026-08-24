package unityadvertising

import (
	"context"
	"encoding/json"

	"social-hub/pkg/socialhub"
)

type LimitedAdTracking string

const (
	UsersAllowingAdTracking    LimitedAdTracking = "USERS_ALLOWING_AD_TRACKING"
	UsersNotAllowingAdTracking LimitedAdTracking = "USERS_NOT_ALLOWING_AD_TRACKING"
)

type ConnectionType string

const (
	ConnectionWiFi     ConnectionType = "wifi"
	ConnectionCellular ConnectionType = "cellular"
)

type ScreenSize string

const (
	ScreenSmall  ScreenSize = "small"
	ScreenNormal ScreenSize = "normal"
	ScreenLarge  ScreenSize = "large"
	ScreenXLarge ScreenSize = "xlarge"
)

type ScreenDensity string

const (
	DensityLDPI    ScreenDensity = "ldpi"
	DensityMDPI    ScreenDensity = "mdpi"
	DensityHDPI    ScreenDensity = "hdpi"
	DensityXHDPI   ScreenDensity = "xhdpi"
	DensityXXHDPI  ScreenDensity = "xxhdpi"
	DensityXXXHDPI ScreenDensity = "xxxhdpi"
)

// AppTargetingOptions models Unity's mutually exclusive allowList/blockList union.
// A pointer to an empty slice sends an explicit empty list.
type AppTargetingOptions struct {
	AllowList *[]string `json:"allowList,omitempty"`
	BlockList *[]string `json:"blockList,omitempty"`
}

type CategoryTargetingOptions struct {
	AllowList *[]string `json:"allowList,omitempty"`
	BlockList *[]string `json:"blockList,omitempty"`
}

// DeviceTargetingOptions covers the Android and iOS union. Android uses screen
// fields; iOS uses allowedDevices/blockedDevices.
type DeviceTargetingOptions struct {
	LimitedAdTracking *[]LimitedAdTracking `json:"limitedAdTracking,omitempty"`
	OSMin             *string              `json:"osMin,omitempty"`
	OSMax             *string              `json:"osMax,omitempty"`
	ConnectionType    *[]ConnectionType    `json:"connectionType,omitempty"`
	ScreenSize        *[]ScreenSize        `json:"screenSize,omitempty"`
	ScreenDensity     *[]ScreenDensity     `json:"screenDensity,omitempty"`
	AllowedDevices    *[]string            `json:"allowedDevices,omitempty"`
	BlockedDevices    *[]string            `json:"blockedDevices,omitempty"`
}

type RegionalTargeting struct {
	Country      CountryCode `json:"country"`
	Subdivisions []string    `json:"subdivisions"`
}

type Targeting struct {
	AppTargeting      *AppTargetingOptions      `json:"appTargeting,omitempty"`
	DeviceTargeting   *DeviceTargetingOptions   `json:"deviceTargeting,omitempty"`
	CategoryTargeting *CategoryTargetingOptions `json:"categoryTargeting,omitempty"`
	RegionalTargeting *[]RegionalTargeting      `json:"regionalTargeting,omitempty"`
	Raw               json.RawMessage           `json:"-"`
}

func (client *Client) GetTargeting(ctx context.Context, campaignSetID, campaignID string, options ...socialhub.CallOption) (*Targeting, error) {
	path, err := client.campaignPath("targeting_get", campaignSetID, campaignID)
	if err != nil {
		return nil, err
	}
	var targeting Targeting
	if err := client.getJSON(ctx, "targeting_get", path+"/targeting", nil, &targeting, options...); err != nil {
		return nil, err
	}
	if err := validateTargeting(targeting, "targeting_get"); err != nil {
		return nil, platformContractError("targeting_get", "Unity returned invalid targeting options")
	}
	return &targeting, nil
}

func (client *Client) UpdateTargeting(ctx context.Context, campaignSetID, campaignID string, input Targeting, options ...socialhub.CallOption) (*Targeting, error) {
	path, err := client.campaignPath("targeting_update", campaignSetID, campaignID)
	if err != nil {
		return nil, err
	}
	if err := validateTargeting(input, "targeting_update"); err != nil {
		return nil, err
	}
	var targeting Targeting
	if err := client.patchJSON(ctx, "targeting_update", path+"/targeting", input, &targeting, options...); err != nil {
		return nil, err
	}
	if err := validateTargeting(targeting, "targeting_update"); err != nil {
		return nil, platformContractError("targeting_update", "Unity returned invalid targeting options")
	}
	return &targeting, nil
}

func validateTargeting(input Targeting, operation string) error {
	if input.AppTargeting != nil {
		if input.AppTargeting.AllowList != nil && input.AppTargeting.BlockList != nil {
			return invalidArgument(operation, "app targeting cannot contain both allowList and blockList")
		}
		for _, list := range []*[]string{input.AppTargeting.AllowList, input.AppTargeting.BlockList} {
			if list == nil {
				continue
			}
			for _, appID := range *list {
				if !validSourceAppID(appID) {
					return invalidArgument(operation, "app targeting contains an invalid 12-character source app ID")
				}
			}
		}
	}
	if input.CategoryTargeting != nil {
		if input.CategoryTargeting.AllowList != nil && input.CategoryTargeting.BlockList != nil {
			return invalidArgument(operation, "category targeting cannot contain both allowList and blockList")
		}
		for _, list := range []*[]string{input.CategoryTargeting.AllowList, input.CategoryTargeting.BlockList} {
			if list == nil {
				continue
			}
			for _, category := range *list {
				if !validText(category, 255) {
					return invalidArgument(operation, "category targeting contains an invalid category")
				}
			}
		}
	}
	if input.DeviceTargeting != nil {
		if err := validateDeviceTargeting(*input.DeviceTargeting, operation); err != nil {
			return err
		}
	}
	if input.RegionalTargeting != nil {
		for _, region := range *input.RegionalTargeting {
			if !validRegionalCountry(region.Country) || len(region.Subdivisions) == 0 {
				return invalidArgument(operation, "regional targeting requires a country and one or more subdivisions")
			}
			for _, subdivision := range region.Subdivisions {
				if !validOpaque(subdivision, 32) {
					return invalidArgument(operation, "regional targeting contains an invalid subdivision")
				}
			}
		}
	}
	return nil
}

func validateDeviceTargeting(input DeviceTargetingOptions, operation string) error {
	android := input.ScreenSize != nil || input.ScreenDensity != nil
	ios := input.AllowedDevices != nil || input.BlockedDevices != nil
	if android && ios {
		return invalidArgument(operation, "device targeting cannot mix Android screen fields with iOS device lists")
	}
	if input.LimitedAdTracking != nil {
		for _, value := range *input.LimitedAdTracking {
			if value != UsersAllowingAdTracking && value != UsersNotAllowingAdTracking {
				return invalidArgument(operation, "limited-ad-tracking value is invalid")
			}
		}
	}
	if input.ConnectionType != nil {
		for _, value := range *input.ConnectionType {
			if value != ConnectionWiFi && value != ConnectionCellular {
				return invalidArgument(operation, "connection type is invalid")
			}
		}
	}
	if input.ScreenSize != nil {
		for _, value := range *input.ScreenSize {
			if value != ScreenSmall && value != ScreenNormal && value != ScreenLarge && value != ScreenXLarge {
				return invalidArgument(operation, "screen size is invalid")
			}
		}
	}
	if input.ScreenDensity != nil {
		for _, value := range *input.ScreenDensity {
			if value != DensityLDPI && value != DensityMDPI && value != DensityHDPI && value != DensityXHDPI && value != DensityXXHDPI && value != DensityXXXHDPI {
				return invalidArgument(operation, "screen density is invalid")
			}
		}
	}
	for _, value := range []*string{input.OSMin, input.OSMax} {
		if value != nil && !validOpaque(*value, 32) {
			return invalidArgument(operation, "OS version is invalid")
		}
	}
	for _, list := range []*[]string{input.AllowedDevices, input.BlockedDevices} {
		if list == nil {
			continue
		}
		for _, device := range *list {
			if !validOpaque(device, 128) {
				return invalidArgument(operation, "iOS device target is invalid")
			}
		}
	}
	return nil
}

func (targeting *Targeting) UnmarshalJSON(data []byte) error {
	return captureRaw(data, (*targetingAlias)(targeting), &targeting.Raw)
}

type targetingAlias Targeting

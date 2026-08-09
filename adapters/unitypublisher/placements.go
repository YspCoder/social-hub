package unitypublisher

import (
	"context"
	"net/url"

	"social-hub/pkg/socialhub"
)

type PlacementsWorkflow interface {
	ListApplicationPlacements(context.Context, string, ListApplicationPlacementsRequest, ...socialhub.CallOption) ([]Placement, error)
	CreatePlacement(context.Context, string, PlacementRequest, MutationOptions, ...socialhub.CallOption) (*Placement, error)
	GetPlacement(context.Context, string, string, ...socialhub.CallOption) (*Placement, error)
	UpdatePlacement(context.Context, string, string, PlacementRequest, MutationOptions, ...socialhub.CallOption) (*Placement, error)
	ArchivePlacement(context.Context, string, string, MutationOptions, ...socialhub.CallOption) error
	RestorePlacement(context.Context, string, string, MutationOptions, ...socialhub.CallOption) error
	ListOrganizationPlacements(context.Context, ...socialhub.CallOption) ([]OrganizationPlacement, error)
}

func (client *Client) ListApplicationPlacements(ctx context.Context, applicationID string, input ListApplicationPlacementsRequest, options ...socialhub.CallOption) ([]Placement, error) {
	const operation = "application_placements_list"
	path, err := client.applicationPath(operation, applicationID)
	if err != nil {
		return nil, err
	}
	query, err := placementListQuery(input)
	if err != nil {
		return nil, invalidArgument(operation, "placement filter contains an invalid ad format")
	}
	var output []Placement
	if err := client.getJSON(ctx, operation, path+"/placements", query, &output, options...); err != nil {
		return nil, err
	}
	for _, placement := range output {
		if placement.ApplicationID != applicationID || !validPlacement(placement) {
			return nil, ownershipError(operation, "Placement")
		}
	}
	return output, nil
}

func (client *Client) CreatePlacement(ctx context.Context, applicationID string, input PlacementRequest, mutation MutationOptions, options ...socialhub.CallOption) (*Placement, error) {
	const operation = "placement_create"
	path, err := client.applicationPath(operation, applicationID)
	if err != nil {
		return nil, err
	}
	if !validPlacementRequest(input) {
		return nil, invalidArgument(operation, "placement name, ad format, or format-specific configuration is invalid")
	}
	var output Placement
	if err := client.postJSON(ctx, operation, path+"/placements", mutationQuery(mutation), input, &output, options...); err != nil {
		return nil, err
	}
	if output.ApplicationID != applicationID || !validPlacement(output) {
		return nil, ownershipError(operation, "Placement")
	}
	return &output, nil
}

func (client *Client) GetPlacement(ctx context.Context, applicationID, placementID string, options ...socialhub.CallOption) (*Placement, error) {
	const operation = "placement_get"
	path, err := client.placementPath(operation, applicationID, placementID)
	if err != nil {
		return nil, err
	}
	var output Placement
	if err := client.getJSON(ctx, operation, path, nil, &output, options...); err != nil {
		return nil, err
	}
	if output.ID != placementID || output.ApplicationID != applicationID || !validPlacement(output) {
		return nil, ownershipError(operation, "Placement")
	}
	return &output, nil
}

func (client *Client) UpdatePlacement(ctx context.Context, applicationID, placementID string, input PlacementRequest, mutation MutationOptions, options ...socialhub.CallOption) (*Placement, error) {
	const operation = "placement_update"
	path, err := client.placementPath(operation, applicationID, placementID)
	if err != nil {
		return nil, err
	}
	if !validPlacementRequest(input) {
		return nil, invalidArgument(operation, "placement name, ad format, or format-specific configuration is invalid")
	}
	var output Placement
	if err := client.putJSON(ctx, operation, path, mutationQuery(mutation), input, &output, options...); err != nil {
		return nil, err
	}
	if output.ID != placementID || output.ApplicationID != applicationID || !validPlacement(output) {
		return nil, ownershipError(operation, "Placement")
	}
	return &output, nil
}

func (client *Client) ArchivePlacement(ctx context.Context, applicationID, placementID string, mutation MutationOptions, options ...socialhub.CallOption) error {
	const operation = "placement_archive"
	path, err := client.placementPath(operation, applicationID, placementID)
	if err != nil {
		return err
	}
	return client.deleteJSON(ctx, operation, path, mutationQuery(mutation), options...)
}

func (client *Client) RestorePlacement(ctx context.Context, applicationID, placementID string, mutation MutationOptions, options ...socialhub.CallOption) error {
	const operation = "placement_restore"
	path, err := client.placementPath(operation, applicationID, placementID)
	if err != nil {
		return err
	}
	return client.patchJSON(ctx, operation, path+"/restore", mutationQuery(mutation), nil, nil, options...)
}

func (client *Client) ListOrganizationPlacements(ctx context.Context, options ...socialhub.CallOption) ([]OrganizationPlacement, error) {
	const operation = "organization_placements_list"
	var output []OrganizationPlacement
	if err := client.getJSON(ctx, operation, client.organizationPath()+"/placements", nil, &output, options...); err != nil {
		return nil, err
	}
	for _, placement := range output {
		if !validOrganizationPlacement(placement) {
			return nil, platformContractError(operation, "Unity returned an invalid organization Placement")
		}
	}
	return output, nil
}

func placementListQuery(input ListApplicationPlacementsRequest) (url.Values, error) {
	query := make(url.Values)
	if input.IsArchived != nil {
		query.Set("isArchived", boolString(*input.IsArchived))
	}
	for _, format := range input.AdFormats {
		if !validWritableAdFormat(format) {
			return nil, invalidArgument("application_placements_list", "ad format filter is invalid")
		}
		query.Add("adFormat", string(format))
	}
	return query, nil
}

func validPlacementRequest(input PlacementRequest) bool {
	if !validText(input.Name, 1024) || !validWritableAdFormat(input.AdFormat) {
		return false
	}
	if input.AdFormatConfigurations == nil {
		return true
	}
	switch configuration := input.AdFormatConfigurations.(type) {
	case RewardedConfigurations:
		return input.AdFormat == AdFormatRewarded && validRewardedConfiguration(configuration)
	case *RewardedConfigurations:
		return configuration != nil && input.AdFormat == AdFormatRewarded && validRewardedConfiguration(*configuration)
	case InterstitialConfigurations:
		return input.AdFormat == AdFormatInterstitial && validAdmin(configuration.AdminSettings)
	case *InterstitialConfigurations:
		return configuration != nil && input.AdFormat == AdFormatInterstitial && validAdmin(configuration.AdminSettings)
	case BannerConfigurations:
		return input.AdFormat == AdFormatBanner && validBannerConfiguration(configuration)
	case *BannerConfigurations:
		return configuration != nil && input.AdFormat == AdFormatBanner && validBannerConfiguration(*configuration)
	default:
		return false
	}
}

func validRewardedConfiguration(value RewardedConfigurations) bool {
	return validText(value.Name, 1024) && finiteNonNegative(value.Value) && validAdmin(value.AdminSettings)
}

func validBannerConfiguration(value BannerConfigurations) bool {
	return finiteNonNegative(value.BannerRefreshRate) && validAdmin(value.AdminSettings)
}

func validAdmin(value *AdminConfigurations) bool {
	return value == nil || finiteNonNegative(value.AllowSkipInSeconds) && finiteNonNegative(value.VideoPlayableSkipInSeconds) &&
		finiteNonNegative(value.CloseTimerDuration) && finiteNonNegative(value.TapsToClose)
}

func validPlacement(value Placement) bool {
	return validUUID(value.ID) && validText(value.Key, 1024) && validText(value.Name, 1024) &&
		validResponseAdFormat(value.AdFormat) && (value.Status == PlacementActive || value.Status == PlacementPaused) &&
		validPathID(value.ApplicationID) && !value.CreatedAt.IsZero() && !value.UpdatedAt.IsZero()
}

func validOrganizationPlacement(value OrganizationPlacement) bool {
	validPlatform := value.Platform == OrganizationPlatformIOS || value.Platform == OrganizationPlatformAndroid || value.Platform == OrganizationPlatformOther
	return validText(value.PlacementID, 1024) && validText(value.Name, 1024) && validText(value.PlacementType, 256) &&
		validResponseAdFormat(value.AdFormat) && validPlatform
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

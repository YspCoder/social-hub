package amazonads

import (
	"context"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListProfiles(ctx context.Context, options ...socialhub.CallOption) ([]Profile, error) {
	const operation = "profiles_list"
	var profiles []Profile
	if _, err := client.getJSON(ctx, operation, "/v2/profiles", "application/json", &profiles, options...); err != nil {
		return nil, err
	}
	for _, profile := range profiles {
		if !validID(profile.ID) {
			return nil, platformContractError(operation, "Amazon Ads returned an invalid profile ID")
		}
	}
	return profiles, nil
}

func (client *Client) GetProfile(ctx context.Context, options ...socialhub.CallOption) (*Profile, error) {
	profiles, err := client.ListProfiles(ctx, options...)
	if err != nil {
		return nil, err
	}
	for index := range profiles {
		if profiles[index].ID == client.profileID {
			return &profiles[index], nil
		}
	}
	return nil, &socialhub.Error{
		Code: socialhub.CodeNotFound, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: "profile_get",
		PlatformMessage: "configured profile is not accessible to the OAuth token",
	}
}

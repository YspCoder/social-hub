package cm360

import (
	"context"

	"social-hub/pkg/socialhub"
)

func (client *Client) GetProfile(ctx context.Context, options ...socialhub.CallOption) (UserProfile, error) {
	const operation = "profile_get"
	if err := client.requireAnyScope(operation, traffickingScope, reportingScope, conversionsScope); err != nil {
		return UserProfile{}, err
	}
	var profile UserProfile
	if err := withOperation(client.api.JSON(ctx, "GET", client.profilePath(), nil, nil, &profile, options...), operation); err != nil {
		return UserProfile{}, err
	}
	if !validID(profile.ProfileID) || profile.ProfileID != client.profileID || !validID(profile.AccountID) ||
		!validOptionalText(profile.AccountName, 512) || !validOptionalText(profile.UserName, 512) ||
		(profile.SubaccountID != "" && !validID(profile.SubaccountID)) {
		return UserProfile{}, platformContractError(operation, "CM360 returned an invalid or different user profile")
	}
	return profile, nil
}

func (client *Client) GetAdvertiser(ctx context.Context, options ...socialhub.CallOption) (Advertiser, error) {
	const operation = "advertiser_get"
	var advertiser Advertiser
	path := client.profilePath() + "/advertisers/" + client.advertiserID
	if err := client.getJSON(ctx, operation, path, nil, &advertiser, traffickingScope, options...); err != nil {
		return Advertiser{}, err
	}
	if !validID(advertiser.ID) || advertiser.ID != client.advertiserID || !validID(advertiser.AccountID) ||
		!validName(advertiser.Name, 255) || (advertiser.Status != "APPROVED" && advertiser.Status != "ON_HOLD") ||
		(advertiser.SubaccountID != "" && !validID(advertiser.SubaccountID)) ||
		(advertiser.FloodlightConfigurationID != "" && !validID(advertiser.FloodlightConfigurationID)) {
		return Advertiser{}, platformContractError(operation, "CM360 returned an invalid or different advertiser")
	}
	return advertiser, nil
}

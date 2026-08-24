package criteo

import (
	"context"
	"strings"

	"social-hub/pkg/socialhub"
)

const advertisersMePath = "/2026-01/advertisers/me"

func (client *Client) ListAdvertisers(ctx context.Context, options ...socialhub.CallOption) ([]Advertiser, error) {
	const operation = "advertiser_list"
	var response apiEnvelope[[]entityResource[advertiserAttributes]]
	if err := client.getJSON(ctx, operation, advertisersMePath, nil, &response, options...); err != nil {
		return nil, err
	}
	if err := checkProblems(operation, response.Errors); err != nil {
		return nil, err
	}
	result := make([]Advertiser, 0, len(response.Data))
	seen := make(map[string]struct{}, len(response.Data))
	for _, resource := range response.Data {
		if !validID(resource.ID) || (resource.Type != "" && !strings.EqualFold(resource.Type, "advertiser")) ||
			!validText(resource.Attributes.AdvertiserName, 1024) {
			return nil, platformContractError(operation, "Criteo returned an invalid advertiser portfolio resource")
		}
		if _, exists := seen[resource.ID]; exists {
			return nil, platformContractError(operation, "Criteo returned duplicate advertiser resources")
		}
		seen[resource.ID] = struct{}{}
		result = append(result, Advertiser{ID: resource.ID, Name: resource.Attributes.AdvertiserName})
	}
	return result, nil
}

func (client *Client) ValidateConfiguredAdvertiser(ctx context.Context, options ...socialhub.CallOption) (Advertiser, error) {
	const operation = "advertiser_validate"
	advertisers, err := client.ListAdvertisers(ctx, options...)
	if err != nil {
		return Advertiser{}, withOperation(err, operation)
	}
	for _, advertiser := range advertisers {
		if advertiser.ID == client.advertiserID {
			return advertiser, nil
		}
	}
	return Advertiser{}, ownershipError(operation)
}

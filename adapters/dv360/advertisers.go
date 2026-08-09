package dv360

import (
	"context"

	"social-hub/pkg/socialhub"
)

var advertiserOrderFields = map[string]struct{}{
	"advertiserId": {}, "displayName": {}, "entityStatus": {}, "updateTime": {},
}

func (client *Client) GetAdvertiser(ctx context.Context, options ...socialhub.CallOption) (Advertiser, error) {
	const operation = "advertiser_get"
	var advertiser Advertiser
	if err := client.getJSON(ctx, operation, "/v4/advertisers/"+client.advertiserID, nil, &advertiser, options...); err != nil {
		return Advertiser{}, err
	}
	if err := client.validateAdvertiser(operation, advertiser); err != nil {
		return Advertiser{}, err
	}
	if advertiser.AdvertiserID != client.advertiserID {
		return Advertiser{}, platformContractError(operation, "DV360 returned a different advertiser")
	}
	return advertiser, nil
}

func (client *Client) ListAdvertisers(ctx context.Context, input ListRequest, options ...socialhub.CallOption) (Page[Advertiser], error) {
	const operation = "advertiser_list"
	if !validID(client.partnerID) {
		return Page[Advertiser]{}, invalidArgument(operation, "account.settings.partner_id is required to list advertisers")
	}
	if !validPage(input, 200, advertiserOrderFields) {
		return Page[Advertiser]{}, invalidArgument(operation, "pagination, filter, or order is invalid")
	}
	query := listQuery(input)
	query.Set("partnerId", client.partnerID)
	var response listAdvertisersResponse
	if err := client.getJSON(ctx, operation, "/v4/advertisers", query, &response, options...); err != nil {
		return Page[Advertiser]{}, err
	}
	seen := make(map[string]struct{}, len(response.Advertisers))
	for _, advertiser := range response.Advertisers {
		if err := client.validateAdvertiser(operation, advertiser); err != nil {
			return Page[Advertiser]{}, err
		}
		if advertiser.PartnerID != client.partnerID {
			return Page[Advertiser]{}, ownershipError(operation, "advertiser")
		}
		if _, exists := seen[advertiser.AdvertiserID]; exists {
			return Page[Advertiser]{}, platformContractError(operation, "DV360 returned duplicate advertisers")
		}
		seen[advertiser.AdvertiserID] = struct{}{}
	}
	if !validPageToken(response.NextPageToken) {
		return Page[Advertiser]{}, platformContractError(operation, "DV360 returned an invalid page token")
	}
	return Page[Advertiser]{Items: response.Advertisers, NextPageToken: response.NextPageToken}, nil
}

func (client *Client) validateAdvertiser(operation string, advertiser Advertiser) error {
	if !validID(advertiser.AdvertiserID) || !validID(advertiser.PartnerID) || !validDisplayName(advertiser.DisplayName) ||
		!validReadEntityStatus(advertiser.EntityStatus) {
		return platformContractError(operation, "DV360 returned an invalid advertiser")
	}
	if client.partnerID != "" && advertiser.PartnerID != client.partnerID {
		return ownershipError(operation, "advertiser")
	}
	return nil
}

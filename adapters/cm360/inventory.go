package cm360

import (
	"context"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

func (client *Client) GetPlacement(ctx context.Context, placementID string, options ...socialhub.CallOption) (Placement, error) {
	const operation = "placement_get"
	if !validID(placementID) {
		return Placement{}, invalidArgument(operation, "placement ID must be a positive string-encoded integer")
	}
	var placement Placement
	path := client.profilePath() + "/placements/" + placementID
	if err := client.getJSON(ctx, operation, path, nil, &placement, traffickingScope, options...); err != nil {
		return Placement{}, err
	}
	if err := client.validatePlacement(operation, placement); err != nil {
		return Placement{}, err
	}
	if placement.ID != placementID {
		return Placement{}, platformContractError(operation, "CM360 returned a different placement")
	}
	return placement, nil
}

func (client *Client) ListPlacements(ctx context.Context, input PlacementListRequest, options ...socialhub.CallOption) (Page[Placement], error) {
	const operation = "placement_list"
	if !validListBase(input.MaxResults, input.PageToken, input.SearchString, input.SortOrder) ||
		!validIDs(input.CampaignIDs, 1000) || !validPlacementStatus(input.ActiveStatus) {
		return Page[Placement]{}, invalidArgument(operation, "placement filters, pagination, or sorting are invalid")
	}
	query := make(url.Values)
	query.Add("advertiserIds", client.advertiserID)
	setListBase(query, input.MaxResults, input.PageToken, input.SearchString, input.SortOrder)
	for _, id := range input.CampaignIDs {
		query.Add("campaignIds", id)
	}
	if input.ActiveStatus != "" {
		query.Set("activeStatus", string(input.ActiveStatus))
	}
	var response listPlacementsResponse
	if err := client.getJSON(ctx, operation, client.profilePath()+"/placements", query, &response, traffickingScope, options...); err != nil {
		return Page[Placement]{}, err
	}
	seen := make(map[string]struct{}, len(response.Placements))
	for _, placement := range response.Placements {
		if err := client.validatePlacement(operation, placement); err != nil {
			return Page[Placement]{}, err
		}
		if _, exists := seen[placement.ID]; exists {
			return Page[Placement]{}, platformContractError(operation, "CM360 returned duplicate placements")
		}
		seen[placement.ID] = struct{}{}
	}
	if !validPageToken(response.NextPageToken) {
		return Page[Placement]{}, platformContractError(operation, "CM360 returned an invalid page token")
	}
	return Page[Placement]{Items: response.Placements, NextPageToken: response.NextPageToken}, nil
}

func (client *Client) validatePlacement(operation string, placement Placement) error {
	if validID(placement.AdvertiserID) && placement.AdvertiserID != client.advertiserID {
		return ownershipError(operation, "placement")
	}
	if !validID(placement.ID) || placement.AdvertiserID != client.advertiserID || !validID(placement.CampaignID) ||
		!validName(placement.Name, 512) || !validPlacementStatus(placement.ActiveStatus) || placement.ActiveStatus == "" ||
		(placement.SiteID != "" && !validID(placement.SiteID)) ||
		(placement.StartDate != "" || placement.EndDate != "") && !validAbsoluteDateRange(placement.StartDate, placement.EndDate) {
		return platformContractError(operation, "CM360 returned an invalid placement")
	}
	return nil
}

func (client *Client) GetAd(ctx context.Context, adID string, options ...socialhub.CallOption) (Ad, error) {
	const operation = "ad_get"
	if !validID(adID) {
		return Ad{}, invalidArgument(operation, "ad ID must be a positive string-encoded integer")
	}
	var ad Ad
	path := client.profilePath() + "/ads/" + adID
	if err := client.getJSON(ctx, operation, path, nil, &ad, traffickingScope, options...); err != nil {
		return Ad{}, err
	}
	if err := client.validateAd(operation, ad); err != nil {
		return Ad{}, err
	}
	if ad.ID != adID {
		return Ad{}, platformContractError(operation, "CM360 returned a different ad")
	}
	return ad, nil
}

func (client *Client) ListAds(ctx context.Context, input AdListRequest, options ...socialhub.CallOption) (Page[Ad], error) {
	const operation = "ad_list"
	if !validListBase(input.MaxResults, input.PageToken, input.SearchString, input.SortOrder) ||
		!validIDs(input.CampaignIDs, 1000) || !validAdType(input.Type) {
		return Page[Ad]{}, invalidArgument(operation, "ad filters, pagination, or sorting are invalid")
	}
	query := make(url.Values)
	query.Set("advertiserId", client.advertiserID)
	setListBase(query, input.MaxResults, input.PageToken, input.SearchString, input.SortOrder)
	for _, id := range input.CampaignIDs {
		query.Add("campaignIds", id)
	}
	if input.Active != nil {
		query.Set("active", strconv.FormatBool(*input.Active))
	}
	if input.Archived != nil {
		query.Set("archived", strconv.FormatBool(*input.Archived))
	}
	if input.Type != "" {
		query.Set("type", string(input.Type))
	}
	var response listAdsResponse
	if err := client.getJSON(ctx, operation, client.profilePath()+"/ads", query, &response, traffickingScope, options...); err != nil {
		return Page[Ad]{}, err
	}
	seen := make(map[string]struct{}, len(response.Ads))
	for _, ad := range response.Ads {
		if err := client.validateAd(operation, ad); err != nil {
			return Page[Ad]{}, err
		}
		if _, exists := seen[ad.ID]; exists {
			return Page[Ad]{}, platformContractError(operation, "CM360 returned duplicate ads")
		}
		seen[ad.ID] = struct{}{}
	}
	if !validPageToken(response.NextPageToken) {
		return Page[Ad]{}, platformContractError(operation, "CM360 returned an invalid page token")
	}
	return Page[Ad]{Items: response.Ads, NextPageToken: response.NextPageToken}, nil
}

func (client *Client) validateAd(operation string, ad Ad) error {
	if validID(ad.AdvertiserID) && ad.AdvertiserID != client.advertiserID {
		return ownershipError(operation, "ad")
	}
	if !validID(ad.ID) || ad.AdvertiserID != client.advertiserID || !validID(ad.CampaignID) ||
		!validName(ad.Name, 255) || !validAdType(ad.Type) || ad.Type == "" || ad.Active && ad.Archived {
		return platformContractError(operation, "CM360 returned an invalid ad")
	}
	return nil
}

package appleads

import (
	"context"
	"encoding/json"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListAds(ctx context.Context, campaignID, adGroupID int64, pagination Pagination, options ...socialhub.CallOption) (Page[Ad], error) {
	const operation = "ads_list"
	if !validID(campaignID) || !validID(adGroupID) || !validPagination(pagination) {
		return Page[Ad]{}, invalidArgument(operation, "campaign ID, Ad Group ID, or pagination is invalid")
	}
	if _, err := client.GetAdGroup(ctx, campaignID, adGroupID, options...); err != nil {
		return Page[Ad]{}, err
	}
	var response responseEnvelope[[]Ad]
	if err := client.getJSON(ctx, operation, adCollectionPath(campaignID, adGroupID), listQuery(pagination), &response, options...); err != nil {
		return Page[Ad]{}, err
	}
	if err := checkEnvelopeError(operation, response.Error); err != nil {
		return Page[Ad]{}, err
	}
	for index := range response.Data {
		if err := client.validateAd(operation, &response.Data[index], campaignID, adGroupID, 0); err != nil {
			return Page[Ad]{}, err
		}
	}
	return pageResult(response.Data, response.Pagination), nil
}

func (client *Client) GetAd(ctx context.Context, campaignID, adGroupID, adID int64, options ...socialhub.CallOption) (*Ad, error) {
	const operation = "ad_get"
	if !validID(campaignID) || !validID(adGroupID) || !validID(adID) {
		return nil, invalidArgument(operation, "campaign ID, Ad Group ID, and Ad ID must be positive")
	}
	if _, err := client.GetAdGroup(ctx, campaignID, adGroupID, options...); err != nil {
		return nil, err
	}
	return client.getAd(ctx, operation, campaignID, adGroupID, adID, options...)
}

func (client *Client) getAd(ctx context.Context, operation string, campaignID, adGroupID, adID int64, options ...socialhub.CallOption) (*Ad, error) {
	var response responseEnvelope[Ad]
	path := adCollectionPath(campaignID, adGroupID) + "/" + formatID(adID)
	if err := client.getJSON(ctx, operation, path, nil, &response, options...); err != nil {
		return nil, err
	}
	if err := checkEnvelopeError(operation, response.Error); err != nil {
		return nil, err
	}
	if err := client.validateAd(operation, &response.Data, campaignID, adGroupID, adID); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

type adWrite struct {
	CreativeID int64     `json:"creativeId,omitempty"`
	Name       *string   `json:"name,omitempty"`
	Status     *AdStatus `json:"status,omitempty"`
}

func (client *Client) CreateAd(ctx context.Context, campaignID, adGroupID int64, input CreateAdRequest, options ...socialhub.CallOption) (*Ad, error) {
	const operation = "ad_create"
	if !validID(campaignID) || !validID(adGroupID) || !validID(input.CreativeID) || !validText(input.Name, 255) {
		return nil, invalidArgument(operation, "campaign ID, Ad Group ID, or Ad fields are invalid")
	}
	campaign, err := client.GetCampaign(ctx, campaignID, options...)
	if err != nil {
		return nil, err
	}
	group, err := client.getAdGroup(ctx, operation, campaignID, adGroupID, options...)
	if err != nil {
		return nil, err
	}
	if campaign.Deleted || group.Deleted || group.Status != AdGroupPaused {
		return nil, invalidArgument(operation, "Ad Group must be undeleted and paused before creating an Ad")
	}
	creative, err := client.GetCreative(ctx, input.CreativeID, options...)
	if err != nil {
		return nil, err
	}
	if creative.State != "VALID" || creative.AdamID != campaign.AdamID {
		return nil, invalidArgument(operation, "Creative must be VALID and belong to the Campaign app")
	}
	paused := AdPaused
	payload := adWrite{CreativeID: input.CreativeID, Name: &input.Name, Status: &paused}
	var response responseEnvelope[Ad]
	if err := client.postJSON(ctx, operation, adCollectionPath(campaignID, adGroupID), payload, &response, options...); err != nil {
		return nil, err
	}
	if err := checkEnvelopeError(operation, response.Error); err != nil {
		return nil, err
	}
	if err := client.validateAd(operation, &response.Data, campaignID, adGroupID, 0); err != nil {
		return nil, err
	}
	if response.Data.CreativeID != input.CreativeID || response.Data.Status != AdPaused {
		return nil, platformContractError(operation, "created Ad did not preserve the Creative assignment in a paused state")
	}
	return &response.Data, nil
}

func (client *Client) UpdateAd(ctx context.Context, campaignID, adGroupID, adID int64, input UpdateAdRequest, options ...socialhub.CallOption) (*Ad, error) {
	const operation = "ad_update"
	if !validID(campaignID) || !validID(adGroupID) || !validID(adID) || input.Name == nil || !validOptionalText(input.Name, 255) {
		return nil, invalidArgument(operation, "campaign ID, Ad Group ID, Ad ID, or name is invalid")
	}
	if _, err := client.GetAd(ctx, campaignID, adGroupID, adID, options...); err != nil {
		return nil, err
	}
	return client.writeAd(ctx, operation, campaignID, adGroupID, adID, adWrite{Name: input.Name}, nil, options...)
}

func (client *Client) SetAdEnabled(ctx context.Context, campaignID, adGroupID, adID int64, enabled bool, options ...socialhub.CallOption) (*Ad, error) {
	const operation = "ad_set_enabled"
	campaign, err := client.GetCampaign(ctx, campaignID, options...)
	if err != nil {
		return nil, err
	}
	group, err := client.getAdGroup(ctx, operation, campaignID, adGroupID, options...)
	if err != nil {
		return nil, err
	}
	if _, err := client.getAd(ctx, operation, campaignID, adGroupID, adID, options...); err != nil {
		return nil, err
	}
	if enabled && (campaign.Deleted || campaign.Status != CampaignEnabled || group.Deleted || group.Status != AdGroupEnabled) {
		return nil, invalidArgument(operation, "Campaign and Ad Group must be undeleted and enabled before enabling an Ad")
	}
	status := AdPaused
	if enabled {
		status = AdEnabled
	}
	return client.writeAd(ctx, operation, campaignID, adGroupID, adID, adWrite{Status: &status}, &status, options...)
}

func (client *Client) DeleteAd(ctx context.Context, campaignID, adGroupID, adID int64, options ...socialhub.CallOption) error {
	const operation = "ad_delete"
	current, err := client.GetAd(ctx, campaignID, adGroupID, adID, options...)
	if err != nil {
		return err
	}
	if current.Status != AdPaused {
		return invalidArgument(operation, "Ad must be paused before deletion")
	}
	var response responseEnvelope[json.RawMessage]
	path := adCollectionPath(campaignID, adGroupID) + "/" + formatID(adID)
	if err := client.deleteJSON(ctx, operation, path, &response, options...); err != nil {
		return err
	}
	return checkEnvelopeError(operation, response.Error)
}

func (client *Client) writeAd(ctx context.Context, operation string, campaignID, adGroupID, adID int64, payload adWrite, expected *AdStatus, options ...socialhub.CallOption) (*Ad, error) {
	var response responseEnvelope[Ad]
	path := adCollectionPath(campaignID, adGroupID) + "/" + formatID(adID)
	if err := client.putJSON(ctx, operation, path, payload, &response, options...); err != nil {
		return nil, err
	}
	if err := checkEnvelopeError(operation, response.Error); err != nil {
		return nil, err
	}
	if err := client.validateAd(operation, &response.Data, campaignID, adGroupID, adID); err != nil {
		return nil, err
	}
	if expected != nil && response.Data.Status != *expected {
		return nil, platformContractError(operation, "Ad status did not match the requested state")
	}
	return &response.Data, nil
}

func (client *Client) validateAd(operation string, ad *Ad, campaignID, adGroupID, expectedID int64) error {
	if ad == nil || !validID(ad.ID) || ad.OrgID != client.orgID || ad.CampaignID != campaignID || ad.AdGroupID != adGroupID || !validID(ad.CreativeID) {
		return platformContractError(operation, "Ad response has invalid ID or parent ownership")
	}
	if expectedID != 0 && ad.ID != expectedID {
		return platformContractError(operation, "Ad response ID did not match the requested Ad")
	}
	return nil
}

func adCollectionPath(campaignID, adGroupID int64) string {
	return "/campaigns/" + formatID(campaignID) + "/adgroups/" + formatID(adGroupID) + "/ads"
}

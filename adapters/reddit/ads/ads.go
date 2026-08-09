package ads

import (
	"context"
	"net/http"

	"social-hub/pkg/socialhub"
)

type createAdData struct {
	AdGroupID        string           `json:"ad_group_id"`
	Name             string           `json:"name"`
	PostID           string           `json:"post_id"`
	ClickURL         string           `json:"click_url,omitempty"`
	ConfiguredStatus ConfiguredStatus `json:"configured_status"`
}

func (client *Client) ListAds(ctx context.Context, input ListRequest, options ...socialhub.CallOption) (socialhub.Page[Ad], error) {
	const operation = "ads_list"
	if !validList(input) {
		return socialhub.Page[Ad]{}, invalidArgument(operation, "pagination is invalid")
	}
	path := client.accountPath("ads")
	var response listResponse[Ad]
	if _, err := client.getJSON(ctx, operation, path, listQuery(input), &response, options...); err != nil {
		return socialhub.Page[Ad]{}, err
	}
	for index := range response.Data {
		if err := client.validateAd(operation, &response.Data[index], "", "", ""); err != nil {
			return socialhub.Page[Ad]{}, err
		}
	}
	cursor, err := client.pageCursor(operation, path, response.Pagination.NextURL)
	if err != nil {
		return socialhub.Page[Ad]{}, err
	}
	return page(response.Data, cursor), nil
}

func (client *Client) GetAd(ctx context.Context, id string, options ...socialhub.CallOption) (*Ad, error) {
	return client.getAd(ctx, "ad_get", id, options...)
}

func (client *Client) getAd(ctx context.Context, operation, id string, options ...socialhub.CallOption) (*Ad, error) {
	if !validResourceID(id) {
		return nil, invalidArgument(operation, "Ad ID must be numeric")
	}
	var response singleResponse[Ad]
	if _, err := client.getJSON(ctx, operation, "/ads/"+id, nil, &response, options...); err != nil {
		return nil, err
	}
	if err := client.validateAd(operation, &response.Data, id, "", ""); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

func (client *Client) CreateAd(ctx context.Context, input CreateAdRequest, options ...socialhub.CallOption) (*Ad, error) {
	const operation = "ad_create"
	if !validResourceID(input.AdGroupID) || !validText(input.Name, 500) || !validPostID(input.PostID) ||
		input.ClickURL != "" && !validClickURL(input.ClickURL) {
		return nil, invalidArgument(operation, "Ad Group, name, Reddit Post ID, or click URL is invalid")
	}
	adGroup, err := client.getAdGroup(ctx, operation, input.AdGroupID, options...)
	if err != nil {
		return nil, err
	}
	if _, err := client.getCampaign(ctx, operation, adGroup.CampaignID, options...); err != nil {
		return nil, err
	}
	data := createAdData{
		AdGroupID: input.AdGroupID, Name: input.Name, PostID: input.PostID,
		ClickURL: input.ClickURL, ConfiguredStatus: StatusPaused,
	}
	var response singleResponse[Ad]
	path := client.accountPath("ads")
	if _, err := client.writeJSON(ctx, operation, http.MethodPost, path, nil, struct {
		Data createAdData `json:"data"`
	}{Data: data}, &response, options...); err != nil {
		return nil, err
	}
	if err := client.validateAd(operation, &response.Data, "", input.AdGroupID, adGroup.CampaignID); err != nil {
		return nil, err
	}
	if response.Data.ConfiguredStatus != StatusPaused || response.Data.PostID != input.PostID {
		return nil, platformContractError(operation, "Reddit did not preserve the paused existing-Post Ad safety settings")
	}
	return &response.Data, nil
}

func (client *Client) UpdateAd(ctx context.Context, id string, input UpdateAdRequest, options ...socialhub.CallOption) (*Ad, error) {
	const operation = "ad_update"
	if !validResourceID(id) || input.Name == nil && input.Status == nil && input.ClickURL == nil ||
		input.Name != nil && !validText(*input.Name, 500) || input.Status != nil && !validMutationStatus(*input.Status) ||
		input.ClickURL != nil && *input.ClickURL != "" && !validClickURL(*input.ClickURL) {
		return nil, invalidArgument(operation, "Ad ID or update fields are invalid")
	}
	current, err := client.getAd(ctx, operation, id, options...)
	if err != nil {
		return nil, err
	}
	adGroup, err := client.getAdGroup(ctx, operation, current.AdGroupID, options...)
	if err != nil {
		return nil, err
	}
	if _, err := client.getCampaign(ctx, operation, adGroup.CampaignID, options...); err != nil {
		return nil, err
	}
	data := make(map[string]any, 3)
	if input.Name != nil {
		data["name"] = *input.Name
	}
	if input.Status != nil {
		data["configured_status"] = *input.Status
	}
	if input.ClickURL != nil {
		data["click_url"] = *input.ClickURL
	}
	var response singleResponse[Ad]
	if _, err := client.writeJSON(ctx, operation, http.MethodPatch, "/ads/"+id, nil, struct {
		Data map[string]any `json:"data"`
	}{Data: data}, &response, options...); err != nil {
		return nil, err
	}
	if err := client.validateAd(operation, &response.Data, id, current.AdGroupID, adGroup.CampaignID); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

func (client *Client) validateAd(operation string, value *Ad, expectedID, expectedAdGroupID, expectedCampaignID string) error {
	if !validResourceID(value.ID) || expectedID != "" && value.ID != expectedID || !validResourceID(value.AdGroupID) ||
		expectedAdGroupID != "" && value.AdGroupID != expectedAdGroupID || !validResourceID(value.CampaignID) ||
		expectedCampaignID != "" && value.CampaignID != expectedCampaignID {
		return platformContractError(operation, "Reddit returned a missing or mismatched Ad identity")
	}
	if value.AdAccountID != "" && value.AdAccountID != client.adAccountID {
		return platformContractError(operation, "Reddit returned an Ad owned by another Ad Account")
	}
	if value.AdAccountID == "" {
		value.AdAccountID = client.adAccountID
	}
	return nil
}

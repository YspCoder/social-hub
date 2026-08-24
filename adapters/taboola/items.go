package taboola

import (
	"context"
	"net/url"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListItems(ctx context.Context, campaignID string, options ...socialhub.CallOption) ([]CampaignItem, error) {
	const operation = "items_list"
	if !validPathID(campaignID, true) {
		return nil, invalidArgument(operation, "campaign ID is invalid")
	}
	if _, err := client.GetCampaign(ctx, campaignID, options...); err != nil {
		return nil, err
	}
	return client.listItems(ctx, campaignID, options...)
}

func (client *Client) listItems(ctx context.Context, campaignID string, options ...socialhub.CallOption) ([]CampaignItem, error) {
	const operation = "items_list"
	var response pageEnvelope[CampaignItem]
	path := client.accountPath("campaigns/" + url.PathEscape(campaignID) + "/items/")
	if err := client.getJSON(ctx, operation, path, nil, &response, options...); err != nil {
		return nil, err
	}
	for index := range response.Results {
		if err := validateItem(operation, &response.Results[index], campaignID, ""); err != nil {
			return nil, err
		}
	}
	return response.Results, nil
}

func (client *Client) GetItem(ctx context.Context, campaignID, itemID string, options ...socialhub.CallOption) (*CampaignItem, error) {
	const operation = "item_get"
	if !validPathID(campaignID, true) || !validPathID(itemID, true) {
		return nil, invalidArgument(operation, "campaign ID or item ID is invalid")
	}
	if _, err := client.GetCampaign(ctx, campaignID, options...); err != nil {
		return nil, err
	}
	return client.getItem(ctx, operation, campaignID, itemID, options...)
}

func (client *Client) getItem(ctx context.Context, operation, campaignID, itemID string, options ...socialhub.CallOption) (*CampaignItem, error) {
	var response CampaignItem
	path := client.accountPath("campaigns/" + url.PathEscape(campaignID) + "/items/" + url.PathEscape(itemID) + "/")
	if err := client.getJSON(ctx, operation, path, nil, &response, options...); err != nil {
		return nil, err
	}
	if err := validateItem(operation, &response, campaignID, itemID); err != nil {
		return nil, err
	}
	return &response, nil
}

func (client *Client) CreateItem(ctx context.Context, campaignID string, input CreateItemRequest, options ...socialhub.CallOption) (*CampaignItem, error) {
	const operation = "item_create"
	if !validPathID(campaignID, true) || !validDestinationURL(input.URL) {
		return nil, invalidArgument(operation, "campaign ID or destination URL is invalid")
	}
	campaign, err := client.GetCampaign(ctx, campaignID, options...)
	if err != nil {
		return nil, err
	}
	if campaign.IsActive == nil {
		return nil, platformContractError(operation, "Campaign response omitted is_active")
	}
	if *campaign.IsActive || campaign.Status == CampaignRunning {
		return nil, invalidArgument(operation, "Campaign must be paused before creating an Item")
	}
	payload := struct {
		URL string `json:"url"`
	}{URL: input.URL}
	var response CampaignItem
	path := client.accountPath("campaigns/" + url.PathEscape(campaignID) + "/items/")
	if err := client.postJSON(ctx, operation, path, payload, &response, options...); err != nil {
		return nil, err
	}
	if err := validateItem(operation, &response, campaignID, ""); err != nil {
		return nil, err
	}
	if response.Status != ItemCrawling {
		return nil, platformContractError(operation, "new Item did not enter CRAWLING state")
	}
	return &response, nil
}

type itemWrite struct {
	URL          *string `json:"url,omitempty"`
	ThumbnailURL *string `json:"thumbnail_url,omitempty"`
	Title        *string `json:"title,omitempty"`
	Description  *string `json:"description,omitempty"`
	IsActive     *bool   `json:"is_active,omitempty"`
}

func (client *Client) UpdateItem(ctx context.Context, campaignID, itemID string, input UpdateItemRequest, options ...socialhub.CallOption) (*CampaignItem, error) {
	const operation = "item_update"
	if !validPathID(campaignID, true) || !validPathID(itemID, true) || !validUpdateItem(input) {
		return nil, invalidArgument(operation, "campaign ID, item ID, or update fields are invalid")
	}
	if _, err := client.GetCampaign(ctx, campaignID, options...); err != nil {
		return nil, err
	}
	current, err := client.getItem(ctx, operation, campaignID, itemID, options...)
	if err != nil {
		return nil, err
	}
	if current.Status == ItemCrawling {
		return nil, invalidArgument(operation, "CRAWLING Items are read-only")
	}
	payload := itemWrite{URL: input.URL, ThumbnailURL: input.ThumbnailURL, Title: input.Title, Description: input.Description}
	return client.writeItem(ctx, operation, campaignID, itemID, payload, options...)
}

func (client *Client) SetItemActive(ctx context.Context, campaignID, itemID string, active bool, options ...socialhub.CallOption) (*CampaignItem, error) {
	const operation = "item_set_active"
	if !validPathID(campaignID, true) || !validPathID(itemID, true) {
		return nil, invalidArgument(operation, "campaign ID or item ID is invalid")
	}
	if _, err := client.GetCampaign(ctx, campaignID, options...); err != nil {
		return nil, err
	}
	current, err := client.getItem(ctx, operation, campaignID, itemID, options...)
	if err != nil {
		return nil, err
	}
	switch current.Status {
	case ItemRunning, ItemPaused, ItemPendingApproval:
	default:
		return nil, invalidArgument(operation, "is_active can only be changed for RUNNING, PAUSED, or PENDING_APPROVAL Items")
	}
	return client.writeItem(ctx, operation, campaignID, itemID, itemWrite{IsActive: &active}, options...)
}

func (client *Client) writeItem(ctx context.Context, operation, campaignID, itemID string, payload itemWrite, options ...socialhub.CallOption) (*CampaignItem, error) {
	var response CampaignItem
	path := client.accountPath("campaigns/" + url.PathEscape(campaignID) + "/items/" + url.PathEscape(itemID) + "/")
	if err := client.postJSON(ctx, operation, path, payload, &response, options...); err != nil {
		return nil, err
	}
	if err := validateItem(operation, &response, campaignID, itemID); err != nil {
		return nil, err
	}
	if payload.IsActive != nil && (response.IsActive == nil || *response.IsActive != *payload.IsActive) {
		return nil, platformContractError(operation, "Item is_active did not match the requested state")
	}
	return &response, nil
}

func validateItem(operation string, item *CampaignItem, campaignID, expectedItemID string) error {
	if item == nil || !validPathID(item.ID, true) || item.CampaignID != campaignID {
		return platformContractError(operation, "Item response has invalid ID or Campaign ownership")
	}
	if expectedItemID != "" && item.ID != expectedItemID {
		return platformContractError(operation, "Item response ID did not match the requested Item")
	}
	return nil
}

func validUpdateItem(input UpdateItemRequest) bool {
	if input == (UpdateItemRequest{}) || input.URL != nil && !validDestinationURL(*input.URL) ||
		input.ThumbnailURL != nil && !validDestinationURL(*input.ThumbnailURL) ||
		!validOptionalText(input.Title, 2048) || !validOptionalText(input.Description, 4096) {
		return false
	}
	return true
}

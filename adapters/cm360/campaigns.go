package cm360

import (
	"context"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

type campaignCreatePayload struct {
	AdvertiserID       string `json:"advertiserId"`
	Name               string `json:"name"`
	Archived           bool   `json:"archived"`
	StartDate          string `json:"startDate"`
	EndDate            string `json:"endDate"`
	Comment            string `json:"comment,omitempty"`
	BillingInvoiceCode string `json:"billingInvoiceCode,omitempty"`
}

type campaignPatchPayload struct {
	Name               *string `json:"name,omitempty"`
	Archived           *bool   `json:"archived,omitempty"`
	StartDate          *string `json:"startDate,omitempty"`
	EndDate            *string `json:"endDate,omitempty"`
	Comment            *string `json:"comment,omitempty"`
	BillingInvoiceCode *string `json:"billingInvoiceCode,omitempty"`
}

func (client *Client) GetCampaign(ctx context.Context, campaignID string, options ...socialhub.CallOption) (Campaign, error) {
	const operation = "campaign_get"
	if !validID(campaignID) {
		return Campaign{}, invalidArgument(operation, "campaign ID must be a positive string-encoded integer")
	}
	var campaign Campaign
	path := client.profilePath() + "/campaigns/" + campaignID
	if err := client.getJSON(ctx, operation, path, nil, &campaign, traffickingScope, options...); err != nil {
		return Campaign{}, err
	}
	if err := client.validateCampaign(operation, campaign); err != nil {
		return Campaign{}, err
	}
	if campaign.ID != campaignID {
		return Campaign{}, platformContractError(operation, "CM360 returned a different campaign")
	}
	return campaign, nil
}

func (client *Client) ListCampaigns(ctx context.Context, input CampaignListRequest, options ...socialhub.CallOption) (Page[Campaign], error) {
	const operation = "campaign_list"
	if !validListBase(input.MaxResults, input.PageToken, input.SearchString, input.SortOrder) ||
		!validIDs(input.IDs, 1000) || (input.SortField != "" && input.SortField != CampaignSortID && input.SortField != CampaignSortName) {
		return Page[Campaign]{}, invalidArgument(operation, "campaign filters, pagination, or sorting are invalid")
	}
	query := make(url.Values)
	query.Add("advertiserIds", client.advertiserID)
	setListBase(query, input.MaxResults, input.PageToken, input.SearchString, input.SortOrder)
	if input.Archived != nil {
		query.Set("archived", strconv.FormatBool(*input.Archived))
	}
	if input.SortField != "" {
		query.Set("sortField", string(input.SortField))
	}
	for _, id := range input.IDs {
		query.Add("ids", id)
	}
	var response listCampaignsResponse
	if err := client.getJSON(ctx, operation, client.profilePath()+"/campaigns", query, &response, traffickingScope, options...); err != nil {
		return Page[Campaign]{}, err
	}
	seen := make(map[string]struct{}, len(response.Campaigns))
	for _, campaign := range response.Campaigns {
		if err := client.validateCampaign(operation, campaign); err != nil {
			return Page[Campaign]{}, err
		}
		if _, exists := seen[campaign.ID]; exists {
			return Page[Campaign]{}, platformContractError(operation, "CM360 returned duplicate campaigns")
		}
		seen[campaign.ID] = struct{}{}
	}
	if !validPageToken(response.NextPageToken) {
		return Page[Campaign]{}, platformContractError(operation, "CM360 returned an invalid page token")
	}
	return Page[Campaign]{Items: response.Campaigns, NextPageToken: response.NextPageToken}, nil
}

func (client *Client) CreateCampaign(ctx context.Context, input CreateCampaignRequest, options ...socialhub.CallOption) (Campaign, error) {
	const operation = "campaign_create"
	if !validName(input.Name, 511) || !validAbsoluteDateRange(input.StartDate, input.EndDate) ||
		!validOptionalText(input.Comment, 255) || !validOptionalText(input.BillingInvoiceCode, 512) {
		return Campaign{}, invalidArgument(operation, "campaign name, dates, comment, or billing code are invalid")
	}
	payload := campaignCreatePayload{
		AdvertiserID: client.advertiserID, Name: input.Name, Archived: true,
		StartDate: input.StartDate, EndDate: input.EndDate, Comment: input.Comment,
		BillingInvoiceCode: input.BillingInvoiceCode,
	}
	var campaign Campaign
	if err := client.postJSON(ctx, operation, client.profilePath()+"/campaigns", nil, payload, &campaign, traffickingScope, options...); err != nil {
		return Campaign{}, err
	}
	if err := client.validateCampaign(operation, campaign); err != nil {
		return Campaign{}, err
	}
	if campaign.Name != input.Name || !campaign.Archived || campaign.StartDate != input.StartDate || campaign.EndDate != input.EndDate {
		return Campaign{}, platformContractError(operation, "new CM360 campaign was not returned archived with the requested fields")
	}
	return campaign, nil
}

func (client *Client) UpdateCampaign(ctx context.Context, campaignID string, input UpdateCampaignRequest, options ...socialhub.CallOption) (Campaign, error) {
	const operation = "campaign_update"
	if err := validateCampaignPatch(campaignID, input); err != nil {
		return Campaign{}, withOperation(err, operation)
	}
	if _, err := client.GetCampaign(ctx, campaignID, options...); err != nil {
		return Campaign{}, withOperation(err, operation)
	}
	query := url.Values{"id": {campaignID}}
	payload := campaignPatchPayload(input)
	var campaign Campaign
	if err := client.patchJSON(ctx, operation, client.profilePath()+"/campaigns", query, payload, &campaign, traffickingScope, options...); err != nil {
		return Campaign{}, err
	}
	if err := client.validateCampaign(operation, campaign); err != nil {
		return Campaign{}, err
	}
	if campaign.ID != campaignID || !campaignMatchesPatch(campaign, input) {
		return Campaign{}, platformContractError(operation, "CM360 campaign update response did not match the requested patch")
	}
	return campaign, nil
}

func validateCampaignPatch(campaignID string, input UpdateCampaignRequest) error {
	if !validID(campaignID) || input == (UpdateCampaignRequest{}) {
		return invalidArgument("campaign_update", "campaign ID and at least one update field are required")
	}
	if input.Name != nil && !validName(*input.Name, 511) ||
		input.StartDate != nil && !validDate(*input.StartDate) || input.EndDate != nil && !validDate(*input.EndDate) ||
		input.StartDate != nil && input.EndDate != nil && !validAbsoluteDateRange(*input.StartDate, *input.EndDate) ||
		input.Comment != nil && !validOptionalText(*input.Comment, 255) ||
		input.BillingInvoiceCode != nil && !validOptionalText(*input.BillingInvoiceCode, 512) {
		return invalidArgument("campaign_update", "campaign update fields are invalid")
	}
	return nil
}

func campaignMatchesPatch(campaign Campaign, input UpdateCampaignRequest) bool {
	return (input.Name == nil || campaign.Name == *input.Name) &&
		(input.Archived == nil || campaign.Archived == *input.Archived) &&
		(input.StartDate == nil || campaign.StartDate == *input.StartDate) &&
		(input.EndDate == nil || campaign.EndDate == *input.EndDate) &&
		(input.Comment == nil || campaign.Comment == *input.Comment) &&
		(input.BillingInvoiceCode == nil || campaign.BillingInvoiceCode == *input.BillingInvoiceCode)
}

func (client *Client) validateCampaign(operation string, campaign Campaign) error {
	if !validID(campaign.ID) || campaign.AdvertiserID != client.advertiserID || !validName(campaign.Name, 511) ||
		!validAbsoluteDateRange(campaign.StartDate, campaign.EndDate) ||
		!validOptionalText(campaign.Comment, 255) || !validOptionalText(campaign.BillingInvoiceCode, 512) {
		if validID(campaign.ID) && validID(campaign.AdvertiserID) && campaign.AdvertiserID != client.advertiserID {
			return ownershipError(operation, "campaign")
		}
		return platformContractError(operation, "CM360 returned an invalid campaign")
	}
	return nil
}

func setListBase(query url.Values, maxResults int, pageToken, search string, sortOrder SortOrder) {
	if maxResults > 0 {
		query.Set("maxResults", strconv.Itoa(maxResults))
	}
	if pageToken != "" {
		query.Set("pageToken", pageToken)
	}
	if search != "" {
		query.Set("searchString", search)
	}
	if sortOrder != "" {
		query.Set("sortOrder", string(sortOrder))
	}
}

package amazonads

import (
	"context"
	"net/http"

	"social-hub/pkg/socialhub"
)

type campaignListEnvelope struct {
	Campaigns    []Campaign `json:"campaigns"`
	NextToken    string     `json:"nextToken"`
	TotalResults int        `json:"totalResults"`
}

type campaignMutationEnvelope struct {
	Campaigns struct {
		Success []struct {
			Index      int      `json:"index"`
			CampaignID string   `json:"campaignId"`
			Campaign   Campaign `json:"campaign"`
		} `json:"success"`
		Error []mutationFailure `json:"error"`
	} `json:"campaigns"`
}

type campaignMutationResource struct {
	ID             string          `json:"campaignId,omitempty"`
	Name           string          `json:"name,omitempty"`
	TargetingType  TargetingType   `json:"targetingType,omitempty"`
	State          State           `json:"state,omitempty"`
	StartDate      string          `json:"startDate,omitempty"`
	EndDate        *string         `json:"endDate,omitempty"`
	Budget         *Budget         `json:"budget,omitempty"`
	DynamicBidding *DynamicBidding `json:"dynamicBidding,omitempty"`
	PortfolioID    *string         `json:"portfolioId,omitempty"`
}

func (client *Client) ListCampaigns(ctx context.Context, input ListCampaignsRequest, options ...socialhub.CallOption) (Page[Campaign], error) {
	const operation = "campaigns_list"
	if !validIDs(input.IDs) || !validStates(input.States) || !validList(input.MaxResults, input.NextToken) {
		return Page[Campaign]{}, invalidArgument(operation, "IDs, states, max results, or next token are invalid")
	}
	body := struct {
		CampaignIDFilter *includeFilter[string] `json:"campaignIdFilter,omitempty"`
		StateFilter      *includeFilter[State]  `json:"stateFilter,omitempty"`
		MaxResults       int                    `json:"maxResults,omitempty"`
		NextToken        string                 `json:"nextToken,omitempty"`
	}{MaxResults: input.MaxResults, NextToken: input.NextToken}
	if len(input.IDs) > 0 {
		body.CampaignIDFilter = &includeFilter[string]{Include: input.IDs}
	}
	if len(input.States) > 0 {
		body.StateFilter = &includeFilter[State]{Include: input.States}
	}
	var response campaignListEnvelope
	if _, err := client.vendorJSON(ctx, operation, http.MethodPost, "/sp/campaigns/list", campaignMediaType, body, &response, false, options...); err != nil {
		return Page[Campaign]{}, err
	}
	for _, campaign := range response.Campaigns {
		if !validID(campaign.ID) {
			return Page[Campaign]{}, platformContractError(operation, "Amazon Ads returned an invalid Campaign ID")
		}
	}
	return Page[Campaign]{Items: response.Campaigns, NextToken: response.NextToken, TotalResults: response.TotalResults}, nil
}

func (client *Client) CreateCampaign(ctx context.Context, input CreateCampaignRequest, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_create"
	if !validText(input.Name, 128) || !validTargetingType(input.TargetingType) || !validDate(input.StartDate) ||
		input.EndDate != "" && (!validDate(input.EndDate) || input.EndDate < input.StartDate) ||
		!validDecimal(string(input.DailyBudget), true) || !validDynamicBidding(input.DynamicBidding) ||
		input.PortfolioID != "" && !validID(input.PortfolioID) {
		return nil, invalidArgument(operation, "campaign name, targeting, dates, budget, bidding, or portfolio is invalid")
	}
	resource := campaignMutationResource{
		Name: input.Name, TargetingType: input.TargetingType, State: StatePaused,
		StartDate:      input.StartDate,
		Budget:         &Budget{Type: "DAILY", Amount: input.DailyBudget},
		DynamicBidding: &input.DynamicBidding,
	}
	if input.EndDate != "" {
		resource.EndDate = &input.EndDate
	}
	if input.PortfolioID != "" {
		resource.PortfolioID = &input.PortfolioID
	}
	return client.mutateCampaign(ctx, operation, http.MethodPost, "/sp/campaigns", resource, "", options...)
}

func (client *Client) UpdateCampaign(ctx context.Context, id string, input UpdateCampaignRequest, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_update"
	if !validID(id) || input.Name == nil && input.EndDate == nil && input.DailyBudget == nil && input.DynamicBidding == nil && input.PortfolioID == nil {
		return nil, invalidArgument(operation, "campaign ID and at least one update are required")
	}
	resource := campaignMutationResource{ID: id}
	if input.Name != nil {
		if !validText(*input.Name, 128) {
			return nil, invalidArgument(operation, "campaign name is invalid")
		}
		resource.Name = *input.Name
	}
	if input.EndDate != nil {
		if *input.EndDate != "" && !validDate(*input.EndDate) {
			return nil, invalidArgument(operation, "campaign end date is invalid")
		}
		resource.EndDate = input.EndDate
	}
	if input.DailyBudget != nil {
		if !validDecimal(string(*input.DailyBudget), true) {
			return nil, invalidArgument(operation, "campaign daily budget is invalid")
		}
		resource.Budget = &Budget{Type: "DAILY", Amount: *input.DailyBudget}
	}
	if input.DynamicBidding != nil {
		if !validDynamicBidding(*input.DynamicBidding) {
			return nil, invalidArgument(operation, "campaign dynamic bidding is invalid")
		}
		resource.DynamicBidding = input.DynamicBidding
	}
	if input.PortfolioID != nil {
		if *input.PortfolioID != "" && !validID(*input.PortfolioID) {
			return nil, invalidArgument(operation, "campaign portfolio ID is invalid")
		}
		resource.PortfolioID = input.PortfolioID
	}
	return client.mutateCampaign(ctx, operation, http.MethodPut, "/sp/campaigns", resource, id, options...)
}

func (client *Client) SetCampaignState(ctx context.Context, id string, state State, options ...socialhub.CallOption) (*Campaign, error) {
	if !validID(id) || !validState(state) {
		return nil, invalidArgument("campaign_state", "campaign ID and ENABLED or PAUSED state are required")
	}
	return client.mutateCampaign(ctx, "campaign_state", http.MethodPut, "/sp/campaigns", campaignMutationResource{ID: id, State: state}, id, options...)
}

func (client *Client) ArchiveCampaign(ctx context.Context, id string, options ...socialhub.CallOption) error {
	const operation = "campaign_archive"
	if !validID(id) {
		return invalidArgument(operation, "campaign ID is invalid")
	}
	body := struct {
		Filter includeFilter[string] `json:"campaignIdFilter"`
	}{Filter: includeFilter[string]{Include: []string{id}}}
	var response campaignMutationEnvelope
	metadata, err := client.vendorJSON(ctx, operation, http.MethodPost, "/sp/campaigns/delete", campaignMediaType, body, &response, true, options...)
	if err != nil {
		return err
	}
	_, err = campaignMutationResult(operation, id, metadata.StatusCode, metadata.Header, response)
	return err
}

func (client *Client) mutateCampaign(ctx context.Context, operation, method, path string, resource campaignMutationResource, expected string, options ...socialhub.CallOption) (*Campaign, error) {
	body := struct {
		Campaigns []campaignMutationResource `json:"campaigns"`
	}{Campaigns: []campaignMutationResource{resource}}
	var response campaignMutationEnvelope
	metadata, err := client.vendorJSON(ctx, operation, method, path, campaignMediaType, body, &response, true, options...)
	if err != nil {
		return nil, err
	}
	return campaignMutationResult(operation, expected, metadata.StatusCode, metadata.Header, response)
}

func campaignMutationResult(operation, expected string, status int, header http.Header, response campaignMutationEnvelope) (*Campaign, error) {
	if len(response.Campaigns.Error) > 0 {
		return nil, mutationError(operation, status, header, response.Campaigns.Error[0])
	}
	if len(response.Campaigns.Success) != 1 {
		return nil, platformContractError(operation, "Amazon Ads did not return exactly one Campaign mutation result")
	}
	item := response.Campaigns.Success[0]
	if err := requireMutationID(operation, expected, item.CampaignID); err != nil {
		return nil, err
	}
	if item.Campaign.ID == "" {
		item.Campaign.ID = item.CampaignID
	}
	if item.Campaign.ID != item.CampaignID {
		return nil, platformContractError(operation, "Amazon Ads returned mismatched Campaign IDs")
	}
	return &item.Campaign, nil
}

type includeFilter[T any] struct {
	Include []T `json:"include"`
}

package yandexdirect

import (
	"context"

	"social-hub/pkg/socialhub"
)

var campaignFields = []string{
	"Id", "Name", "StartDate", "EndDate", "TimeZone", "Type",
	"State", "Status", "StatusPayment", "StatusClarification",
}

var textCampaignFields = []string{"BiddingStrategy"}

func (client *Client) ListCampaigns(ctx context.Context, input ListCampaignsRequest, options ...socialhub.CallOption) (Page[Campaign], error) {
	const operation = "campaign_list"
	if !validCampaignSelection(input.Selection) || !validPage(input.Page) {
		return Page[Campaign]{}, invalidArgument(operation, "campaign selection or page is invalid")
	}
	params := struct {
		SelectionCriteria      CampaignSelection `json:"SelectionCriteria"`
		FieldNames             []string          `json:"FieldNames"`
		TextCampaignFieldNames []string          `json:"TextCampaignFieldNames"`
		Page                   *PageRequest      `json:"Page,omitempty"`
	}{
		SelectionCriteria: input.Selection, FieldNames: campaignFields,
		TextCampaignFieldNames: textCampaignFields, Page: pagePointer(input.Page),
	}
	var response struct {
		Campaigns []Campaign `json:"Campaigns"`
		LimitedBy *int64     `json:"LimitedBy"`
	}
	metadata, err := client.rpc(ctx, operation, "campaigns", "get", params, &response, false, options...)
	if err != nil {
		return Page[Campaign]{}, err
	}
	if len(response.Campaigns) > maximumPageItems(input.Page) || response.LimitedBy != nil && *response.LimitedBy <= input.Page.Offset {
		return Page[Campaign]{}, platformContractError(operation, "Yandex returned invalid Campaign pagination")
	}
	for index := range response.Campaigns {
		if err := validateCampaign(operation, &response.Campaigns[index], 0); err != nil {
			return Page[Campaign]{}, err
		}
	}
	return Page[Campaign]{Items: response.Campaigns, LimitedBy: response.LimitedBy, Metadata: metadata}, nil
}

func (client *Client) GetCampaign(ctx context.Context, id int64, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_get"
	if id <= 0 {
		return nil, invalidArgument(operation, "campaign ID must be positive")
	}
	page, err := client.ListCampaigns(ctx, ListCampaignsRequest{Selection: CampaignSelection{IDs: []int64{id}}}, options...)
	if err != nil {
		return nil, withOperation(err, operation)
	}
	if len(page.Items) == 0 {
		return nil, notFound(operation, "campaign was not returned")
	}
	if len(page.Items) != 1 || page.Items[0].ID != id {
		return nil, platformContractError(operation, "Yandex returned a different Campaign")
	}
	return &page.Items[0], nil
}

func (client *Client) UpdateCampaigns(ctx context.Context, inputs []CampaignUpdate, options ...socialhub.CallOption) (BatchResult, error) {
	const operation = "campaign_update"
	if len(inputs) == 0 || len(inputs) > MaximumCampaignMutationBatch {
		return BatchResult{}, invalidArgument(operation, "1-10 Campaign updates are required")
	}
	seen := make(map[int64]struct{}, len(inputs))
	ids := make([]int64, len(inputs))
	for index, input := range inputs {
		if !validCampaignUpdate(input) {
			return BatchResult{}, invalidArgument(operation, "Campaign update is invalid")
		}
		if _, exists := seen[input.ID]; exists {
			return BatchResult{}, invalidArgument(operation, "Campaign IDs must be unique")
		}
		seen[input.ID] = struct{}{}
		ids[index] = input.ID
	}
	params := struct {
		Campaigns []CampaignUpdate `json:"Campaigns"`
	}{Campaigns: inputs}
	var response struct {
		UpdateResults []ActionResult `json:"UpdateResults"`
	}
	metadata, err := client.rpc(ctx, operation, "campaigns", "update", params, &response, true, options...)
	if err != nil {
		return BatchResult{Metadata: metadata}, err
	}
	return actionResult(operation, response.UpdateResults, len(inputs), metadata, ids...)
}

func (client *Client) SuspendCampaigns(ctx context.Context, ids []int64, options ...socialhub.CallOption) (BatchResult, error) {
	return client.campaignAction(ctx, "campaign_suspend", "suspend", ids, options...)
}

func (client *Client) ResumeCampaigns(ctx context.Context, ids []int64, options ...socialhub.CallOption) (BatchResult, error) {
	return client.campaignAction(ctx, "campaign_resume", "resume", ids, options...)
}

func (client *Client) DeleteCampaigns(ctx context.Context, ids []int64, options ...socialhub.CallOption) (BatchResult, error) {
	const operation = "campaign_delete"
	if !validIDs(ids, MaximumCampaignActionBatch, false) {
		return BatchResult{}, invalidArgument(operation, "1-1000 unique Campaign IDs are required")
	}
	callOptions, err := prepareCallOptions(operation, options)
	if err != nil {
		return BatchResult{}, err
	}
	ctx, cancel := withCallTimeout(ctx, callOptions.Timeout)
	defer cancel()
	options = nil
	page, err := client.ListCampaigns(ctx, ListCampaignsRequest{
		Selection: CampaignSelection{IDs: ids}, Page: PageRequest{Limit: int64(len(ids))},
	}, options...)
	if err != nil {
		return BatchResult{}, withOperation(err, operation)
	}
	if len(page.Items) != len(ids) {
		return BatchResult{}, notFound(operation, "one or more Campaigns were not returned")
	}
	expected := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		expected[id] = struct{}{}
	}
	for index := range page.Items {
		campaign := &page.Items[index]
		if _, found := expected[campaign.ID]; !found {
			return BatchResult{}, platformContractError(operation, "Yandex returned a duplicate Campaign or one outside the delete selection")
		}
		delete(expected, campaign.ID)
		if !safeNonServingCampaign(campaign) && campaign.State != CampaignStateEnded {
			return BatchResult{}, invalidArgument(operation, "Campaign must be suspended, draft/off, or ended before delete")
		}
	}
	if len(expected) != 0 {
		return BatchResult{}, platformContractError(operation, "Yandex omitted one or more Campaigns from the delete selection")
	}
	return client.campaignAction(ctx, operation, "delete", ids, options...)
}

func (client *Client) campaignAction(ctx context.Context, operation, method string, ids []int64, options ...socialhub.CallOption) (BatchResult, error) {
	if !validIDs(ids, MaximumCampaignActionBatch, false) {
		return BatchResult{}, invalidArgument(operation, "1-1000 unique Campaign IDs are required")
	}
	params := struct {
		SelectionCriteria struct {
			IDs []int64 `json:"Ids"`
		} `json:"SelectionCriteria"`
	}{}
	params.SelectionCriteria.IDs = ids
	var response struct {
		SuspendResults []ActionResult `json:"SuspendResults"`
		ResumeResults  []ActionResult `json:"ResumeResults"`
		DeleteResults  []ActionResult `json:"DeleteResults"`
	}
	metadata, err := client.rpc(ctx, operation, "campaigns", method, params, &response, true, options...)
	if err != nil {
		return BatchResult{Metadata: metadata}, err
	}
	items := response.SuspendResults
	if method == "resume" {
		items = response.ResumeResults
	} else if method == "delete" {
		items = response.DeleteResults
	}
	return actionResult(operation, items, len(ids), metadata, ids...)
}

func validateCampaign(operation string, campaign *Campaign, expectedID int64) error {
	if campaign == nil || campaign.ID <= 0 || !validText(campaign.Name, 255) || !validDate(campaign.StartDate) ||
		!validOptionalDate(campaign.EndDate) || campaign.Type == "" || campaign.State == "" || campaign.Status == "" {
		return platformContractError(operation, "Yandex returned an invalid Campaign")
	}
	if expectedID > 0 && campaign.ID != expectedID {
		return platformContractError(operation, "Campaign ID did not match the request")
	}
	if campaign.Type == CampaignText {
		if campaign.TextCampaign == nil || campaign.TextCampaign.BiddingStrategy.Search.BiddingStrategyType == "" ||
			campaign.TextCampaign.BiddingStrategy.Network.BiddingStrategyType == "" {
			return platformContractError(operation, "Yandex returned incomplete Text Campaign strategy data")
		}
	}
	return nil
}

func safeNonServingCampaign(campaign *Campaign) bool {
	return campaign != nil && (campaign.State == CampaignStateSuspended ||
		campaign.State == CampaignStateOff && campaign.Status == StatusDraft)
}

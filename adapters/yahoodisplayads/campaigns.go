package yahoodisplayads

import (
	"context"

	"social-hub/pkg/socialhub"
)

const campaignServicePath = "CampaignService"

type campaignSelectorRequest struct {
	AccountID     int64        `json:"accountId"`
	CampaignIDs   []int64      `json:"campaignIds,omitempty"`
	UserStatuses  []UserStatus `json:"userStatuses,omitempty"`
	StartIndex    int32        `json:"startIndex,omitempty"`
	NumberResults int32        `json:"numberResults,omitempty"`
}

type campaignOperation struct {
	AccountID int64      `json:"accountId"`
	Operand   []Campaign `json:"operand"`
}

func (client *Client) ListCampaigns(ctx context.Context, input CampaignSelector, options ...socialhub.CallOption) (Page[Campaign], error) {
	const operation = "campaign_list"
	if !validCampaignSelector(input) {
		return Page[Campaign]{}, invalidArgument(operation, "campaign IDs, statuses, or pagination are invalid")
	}
	request := campaignSelectorRequest{
		AccountID: client.advertiserAccountID, CampaignIDs: input.CampaignIDs,
		UserStatuses: input.UserStatuses, StartIndex: input.StartIndex, NumberResults: input.NumberResults,
	}
	return postPage(ctx, client, operation, campaignServicePath+"/get", request, input.PageRequest,
		MaximumCampaignPageSize, campaignEntity, func(value *Campaign) error {
			return client.validateCampaign(operation, value, 0)
		}, options...)
}

func (client *Client) GetCampaign(ctx context.Context, id int64, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_get"
	if id <= 0 {
		return nil, invalidArgument(operation, "campaign ID must be positive")
	}
	page, err := client.ListCampaigns(ctx, CampaignSelector{
		CampaignIDs: []int64{id}, PageRequest: PageRequest{StartIndex: 1, NumberResults: 1},
	}, options...)
	if err != nil {
		return nil, withOperation(err, operation)
	}
	if len(page.Items) == 0 {
		return nil, notFound(operation, "campaign was not returned")
	}
	if len(page.Items) != 1 || page.Items[0].CampaignID != id {
		return nil, platformContractError(operation, "LINE Yahoo returned a different campaign")
	}
	return &page.Items[0], nil
}

func (client *Client) CreateCampaign(ctx context.Context, input CampaignAdd, options ...socialhub.CallOption) (*Campaign, MutationResult[Campaign], error) {
	const operation = "campaign_create"
	if !validCampaignAdd(input) {
		return nil, MutationResult[Campaign]{}, invalidArgument(operation, "campaign name, goal, CPC, budget, or dates are invalid")
	}
	prepared, err := prepareCallOptions(operation, options)
	if err != nil {
		return nil, MutationResult[Campaign]{}, err
	}
	budget, cpc := input.BudgetAmount, input.CPC
	operand := Campaign{
		CampaignName: input.Name, CampaignGoal: input.Goal, UserStatus: StatusPaused,
		Budget: &CampaignBudget{Amount: &budget},
		BiddingStrategyConfiguration: &CampaignBiddingStrategy{BiddingScheme: &CampaignBiddingScheme{
			BiddingStrategyType: BiddingCPC, CPC: &CampaignCPCScheme{CPC: &cpc},
		}},
		StartDate: input.StartDate, EndDate: input.EndDate,
	}
	result, err := postMutation(ctx, client, operation, campaignServicePath+"/add",
		campaignOperation{AccountID: client.advertiserAccountID, Operand: []Campaign{operand}},
		1, campaignEntity, func(value *Campaign) error {
			return client.validateCampaignMutation(operation, value)
		}, prepared...)
	if err != nil {
		return nil, result, err
	}
	created, err := client.GetCampaign(ctx, result.Items[0].Value.CampaignID, prepared...)
	if err != nil {
		return nil, result, withOperation(err, operation)
	}
	result.Items[0].Value = created
	if created.UserStatus != StatusPaused || created.CampaignGoal != input.Goal {
		return created, result, platformContractError(operation, "created campaign was not returned with the requested goal in PAUSED state")
	}
	return created, result, nil
}

func (client *Client) UpdateCampaigns(ctx context.Context, inputs []CampaignUpdate, options ...socialhub.CallOption) (MutationResult[Campaign], error) {
	const operation = "campaign_update"
	if len(inputs) == 0 || len(inputs) > MaximumCampaignMutationBatch {
		return MutationResult[Campaign]{}, invalidArgument(operation, "1-300 campaign updates are required")
	}
	seen := make(map[int64]struct{}, len(inputs))
	operands := make([]Campaign, 0, len(inputs))
	for _, input := range inputs {
		if !validCampaignUpdate(input) {
			return MutationResult[Campaign]{}, invalidArgument(operation, "campaign update is invalid")
		}
		if _, exists := seen[input.ID]; exists {
			return MutationResult[Campaign]{}, invalidArgument(operation, "campaign IDs must be unique")
		}
		seen[input.ID] = struct{}{}
		operand := Campaign{CampaignID: input.ID}
		if input.Name != nil {
			operand.CampaignName = *input.Name
		}
		if input.BudgetAmount != nil {
			amount := *input.BudgetAmount
			operand.Budget = &CampaignBudget{Amount: &amount}
		}
		if input.CPC != nil {
			cpc := *input.CPC
			operand.BiddingStrategyConfiguration = &CampaignBiddingStrategy{BiddingScheme: &CampaignBiddingScheme{
				BiddingStrategyType: BiddingCPC, CPC: &CampaignCPCScheme{CPC: &cpc},
			}}
		}
		if input.StartDate != nil {
			operand.StartDate = *input.StartDate
		}
		if input.EndDate != nil {
			operand.EndDate = *input.EndDate
		}
		operands = append(operands, operand)
	}
	return postMutation(ctx, client, operation, campaignServicePath+"/set",
		campaignOperation{AccountID: client.advertiserAccountID, Operand: operands}, len(operands),
		campaignEntity, func(value *Campaign) error {
			return client.validateCampaignMutation(operation, value)
		}, options...)
}

func (client *Client) SetCampaignsEnabled(ctx context.Context, ids []int64, enabled bool, options ...socialhub.CallOption) (MutationResult[Campaign], error) {
	const operation = "campaign_set_enabled"
	if !validIDs(ids, MaximumCampaignMutationBatch, false) {
		return MutationResult[Campaign]{}, invalidArgument(operation, "1-300 unique campaign IDs are required")
	}
	status := StatusPaused
	if enabled {
		status = StatusActive
	}
	operands := make([]Campaign, 0, len(ids))
	for _, id := range ids {
		operands = append(operands, Campaign{CampaignID: id, UserStatus: status})
	}
	return postMutation(ctx, client, operation, campaignServicePath+"/set",
		campaignOperation{AccountID: client.advertiserAccountID, Operand: operands}, len(operands),
		campaignEntity, func(value *Campaign) error {
			return client.validateCampaignMutation(operation, value)
		}, options...)
}

func (client *Client) DeleteCampaigns(ctx context.Context, ids []int64, options ...socialhub.CallOption) (MutationResult[Campaign], error) {
	const operation = "campaign_delete"
	if !validIDs(ids, MaximumCampaignMutationBatch, false) {
		return MutationResult[Campaign]{}, invalidArgument(operation, "1-300 unique campaign IDs are required")
	}
	prepared, err := prepareCallOptions(operation, options)
	if err != nil {
		return MutationResult[Campaign]{}, err
	}
	if err := client.requirePausedCampaigns(ctx, operation, ids, prepared...); err != nil {
		return MutationResult[Campaign]{}, err
	}
	operands := make([]Campaign, 0, len(ids))
	for _, id := range ids {
		operands = append(operands, Campaign{CampaignID: id})
	}
	return postMutation(ctx, client, operation, campaignServicePath+"/remove",
		campaignOperation{AccountID: client.advertiserAccountID, Operand: operands}, len(operands),
		campaignEntity, func(value *Campaign) error {
			return client.validateCampaignMutation(operation, value)
		}, prepared...)
}

func (client *Client) requirePausedCampaigns(ctx context.Context, operation string, ids []int64, options ...socialhub.CallOption) error {
	expected := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		expected[id] = struct{}{}
	}
	for start := 0; start < len(ids); start += MaximumCampaignSelectorIDs {
		end := start + MaximumCampaignSelectorIDs
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[start:end]
		page, err := client.ListCampaigns(ctx, CampaignSelector{
			CampaignIDs: chunk, PageRequest: PageRequest{StartIndex: 1, NumberResults: int32(len(chunk))},
		}, options...)
		if err != nil {
			return withOperation(err, operation)
		}
		if len(page.Items) != len(chunk) {
			return notFound(operation, "one or more campaigns were not returned before delete")
		}
		for _, campaign := range page.Items {
			if _, exists := expected[campaign.CampaignID]; !exists {
				return platformContractError(operation, "LINE Yahoo returned a duplicate campaign or one outside the delete selection")
			}
			delete(expected, campaign.CampaignID)
			if campaign.UserStatus != StatusPaused {
				return invalidArgument(operation, "campaigns must be PAUSED before delete")
			}
		}
	}
	if len(expected) != 0 {
		return platformContractError(operation, "LINE Yahoo omitted one or more campaigns from the delete preflight")
	}
	return nil
}

func (client *Client) validateCampaign(operation string, value *Campaign, expectedID int64) error {
	if value == nil || value.AccountID != client.advertiserAccountID || value.CampaignID <= 0 ||
		!validText(value.CampaignName, 50) || !validCampaignGoal(value.CampaignGoal) ||
		!validOptionalDate(value.StartDate) || !validOptionalDate(value.EndDate) ||
		value.StartDate != "" && value.EndDate != "" && value.StartDate > value.EndDate ||
		!validReturnedUserStatus(value.UserStatus) {
		return platformContractError(operation, "LINE Yahoo returned an invalid campaign")
	}
	if expectedID > 0 && value.CampaignID != expectedID {
		return platformContractError(operation, "campaign ID did not match the request")
	}
	if value.Budget == nil || value.Budget.Amount == nil && value.Budget.CampaignBudgetID == nil ||
		value.Budget.Amount != nil && value.Budget.CampaignBudgetID != nil ||
		value.Budget.Amount != nil && *value.Budget.Amount <= 0 ||
		value.Budget.CampaignBudgetID != nil && *value.Budget.CampaignBudgetID <= 0 {
		return platformContractError(operation, "LINE Yahoo returned invalid campaign budget data")
	}
	return nil
}

func (client *Client) validateCampaignMutation(operation string, value *Campaign) error {
	if value == nil || value.CampaignID <= 0 || value.AccountID != 0 && value.AccountID != client.advertiserAccountID {
		return platformContractError(operation, "LINE Yahoo returned an invalid campaign mutation value")
	}
	return nil
}

var _ CampaignWorkflow = (*Client)(nil)

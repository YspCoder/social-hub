package microsoftads

import (
	"context"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListCampaigns(ctx context.Context, options ...socialhub.CallOption) ([]Campaign, error) {
	const operation = "list_campaigns"
	var response struct {
		Campaigns []Campaign `json:"Campaigns"`
	}
	_, err := client.postJSON(ctx, operation, client.campaign, "/Campaigns/QueryByAccountId", struct {
		AccountID    string `json:"AccountId"`
		CampaignType string `json:"CampaignType"`
	}{AccountID: client.customerAccountID, CampaignType: "Search"}, &response, options...)
	if err != nil {
		return nil, err
	}
	return response.Campaigns, nil
}

func (client *Client) GetCampaign(ctx context.Context, campaignID string, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "get_campaign"
	if !validNumericID(campaignID) {
		return nil, invalidArgument(operation, "campaign ID must be a nonzero numeric ID")
	}
	var response struct {
		Campaigns     []Campaign    `json:"Campaigns"`
		PartialErrors []wireFailure `json:"PartialErrors"`
	}
	header, err := client.postJSON(ctx, operation, client.campaign, "/Campaigns/QueryByIds", struct {
		AccountID    string   `json:"AccountId"`
		CampaignIDs  []string `json:"CampaignIds"`
		CampaignType string   `json:"CampaignType"`
	}{AccountID: client.customerAccountID, CampaignIDs: []string{campaignID}, CampaignType: "Search"}, &response, options...)
	if err != nil {
		return nil, err
	}
	if err := checkPartialErrors(operation, header, response.PartialErrors); err != nil {
		return nil, err
	}
	if len(response.Campaigns) != 1 || response.Campaigns[0].ID != campaignID {
		return nil, platformContractError(operation, "response campaign does not match requested account and ID")
	}
	return &response.Campaigns[0], nil
}

func (client *Client) CreateCampaign(ctx context.Context, input CreateCampaignRequest, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "create_campaign"
	if !validRequiredText(input.Name, 128) || input.DailyBudget <= 0 || !validRequiredText(input.TimeZone, 128) {
		return nil, invalidArgument(operation, "name, positive daily budget, and time zone are required")
	}
	if !validLanguages(input.Languages) {
		return nil, invalidArgument(operation, "languages contain an invalid value")
	}
	if err := client.validateAccount(ctx, options...); err != nil {
		return nil, err
	}
	payload := campaignWrite{
		Name: &input.Name, BudgetType: stringPointer("DailyBudgetStandard"), DailyBudget: &input.DailyBudget,
		TimeZone: &input.TimeZone, CampaignType: stringPointer("Search"), Status: statusPointer(StatusPaused),
	}
	if len(input.Languages) > 0 {
		payload.Languages = &input.Languages
	}
	var response struct {
		CampaignIDs   []*string     `json:"CampaignIds"`
		PartialErrors []wireFailure `json:"PartialErrors"`
	}
	header, err := client.postJSON(ctx, operation, client.campaign, "/Campaigns", struct {
		AccountID string          `json:"AccountId"`
		Campaigns []campaignWrite `json:"Campaigns"`
	}{AccountID: client.customerAccountID, Campaigns: []campaignWrite{payload}}, &response, options...)
	if err != nil {
		return nil, err
	}
	if err := checkPartialErrors(operation, header, response.PartialErrors); err != nil {
		return nil, err
	}
	if len(response.CampaignIDs) != 1 || response.CampaignIDs[0] == nil || !validNumericID(*response.CampaignIDs[0]) {
		return nil, platformContractError(operation, "response did not contain one campaign ID")
	}
	return client.GetCampaign(ctx, *response.CampaignIDs[0], options...)
}

func (client *Client) UpdateCampaign(ctx context.Context, campaignID string, input UpdateCampaignRequest, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "update_campaign"
	if !validNumericID(campaignID) || input.empty() ||
		(input.Name != nil && !validRequiredText(*input.Name, 128)) ||
		(input.DailyBudget != nil && *input.DailyBudget <= 0) ||
		(input.TimeZone != nil && !validRequiredText(*input.TimeZone, 128)) ||
		(input.Languages != nil && !validLanguages(*input.Languages)) {
		return nil, invalidArgument(operation, "campaign ID and at least one valid update field are required")
	}
	if err := client.validateAccount(ctx, options...); err != nil {
		return nil, err
	}
	if _, err := client.GetCampaign(ctx, campaignID, options...); err != nil {
		return nil, err
	}
	payload := campaignWrite{ID: campaignID, Name: input.Name, DailyBudget: input.DailyBudget, TimeZone: input.TimeZone, Languages: input.Languages}
	if err := client.updateCampaign(ctx, operation, payload, options...); err != nil {
		return nil, err
	}
	return client.GetCampaign(ctx, campaignID, options...)
}

func (client *Client) SetCampaignStatus(ctx context.Context, campaignID string, status Status, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "set_campaign_status"
	if !validNumericID(campaignID) || !validStatus(status) {
		return nil, invalidArgument(operation, "campaign ID and Active or Paused status are required")
	}
	if err := client.validateAccount(ctx, options...); err != nil {
		return nil, err
	}
	if _, err := client.GetCampaign(ctx, campaignID, options...); err != nil {
		return nil, err
	}
	if err := client.updateCampaign(ctx, operation, campaignWrite{ID: campaignID, Status: &status}, options...); err != nil {
		return nil, err
	}
	return client.GetCampaign(ctx, campaignID, options...)
}

type campaignWrite struct {
	ID           string    `json:"Id,omitempty"`
	Name         *string   `json:"Name,omitempty"`
	Status       *Status   `json:"Status,omitempty"`
	BudgetType   *string   `json:"BudgetType,omitempty"`
	DailyBudget  *float64  `json:"DailyBudget,omitempty"`
	TimeZone     *string   `json:"TimeZone,omitempty"`
	CampaignType *string   `json:"CampaignType,omitempty"`
	Languages    *[]string `json:"Languages,omitempty"`
}

func (client *Client) updateCampaign(ctx context.Context, operation string, payload campaignWrite, options ...socialhub.CallOption) error {
	var response struct {
		PartialErrors []wireFailure `json:"PartialErrors"`
	}
	header, err := client.putJSON(ctx, operation, "/Campaigns", struct {
		AccountID string          `json:"AccountId"`
		Campaigns []campaignWrite `json:"Campaigns"`
	}{AccountID: client.customerAccountID, Campaigns: []campaignWrite{payload}}, &response, options...)
	if err != nil {
		return err
	}
	return checkPartialErrors(operation, header, response.PartialErrors)
}

func (input UpdateCampaignRequest) empty() bool {
	return input.Name == nil && input.DailyBudget == nil && input.TimeZone == nil && input.Languages == nil
}

func validLanguages(values []string) bool {
	if len(values) > 16 {
		return false
	}
	for _, value := range values {
		if !validRequiredText(value, 16) {
			return false
		}
	}
	return true
}

func stringPointer(value string) *string { return &value }
func statusPointer(value Status) *Status { return &value }

package naversearchads

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"social-hub/pkg/socialhub"
)

type campaignWrite struct {
	ID                      string          `json:"nccCampaignId,omitempty"`
	CustomerID              int64           `json:"customerId,omitempty"`
	Type                    CampaignType    `json:"campaignTp,omitempty"`
	Name                    string          `json:"name,omitempty"`
	DailyBudget             *int64          `json:"dailyBudget,omitempty"`
	UseDailyBudget          *bool           `json:"useDailyBudget,omitempty"`
	DeliveryMethod          DeliveryMethod  `json:"deliveryMethod,omitempty"`
	UsePeriod               *bool           `json:"usePeriod,omitempty"`
	PeriodStart             string          `json:"periodStartDt,omitempty"`
	PeriodEnd               string          `json:"periodEndDt,omitempty"`
	SharedBudgetID          string          `json:"sharedBudgetId,omitempty"`
	TrackingMode            TrackingMode    `json:"trackingMode,omitempty"`
	TrackingURL             string          `json:"trackingUrl,omitempty"`
	TrackingURLCustomParams json.RawMessage `json:"trackingUrlCustomParams,omitempty"`
	UserLock                *bool           `json:"userLock,omitempty"`
}

func (client *Client) ListCampaigns(ctx context.Context, input ListCampaignsRequest, options ...socialhub.CallOption) (Page[Campaign], error) {
	const operation = "campaign_list"
	if !validCampaignType(input.Type, true) || !validListOptions(input.ListOptions) {
		return Page[Campaign]{}, invalidArgument(operation, "campaign type or list options are invalid")
	}
	list := normalizeList(input.ListOptions)
	query := listValues(list)
	if input.Type != "" {
		query.Set("campaignType", string(input.Type))
	}
	var campaigns []Campaign
	if err := client.getJSON(ctx, operation, "/ncc/campaigns", query, &campaigns, options...); err != nil {
		return Page[Campaign]{}, err
	}
	for index := range campaigns {
		if err := client.validateCampaign(operation, &campaigns[index], ""); err != nil {
			return Page[Campaign]{}, err
		}
	}
	return listPage(campaigns, list, func(value Campaign) string { return value.ID }), nil
}

func (client *Client) GetCampaign(ctx context.Context, id string, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_get"
	if !validID(id) {
		return nil, invalidArgument(operation, "campaign ID is invalid")
	}
	var campaign Campaign
	if err := client.getJSON(ctx, operation, "/ncc/campaigns/"+id, nil, &campaign, options...); err != nil {
		return nil, err
	}
	if err := client.validateCampaign(operation, &campaign, id); err != nil {
		return nil, err
	}
	return &campaign, nil
}

func (client *Client) CreateCampaign(ctx context.Context, input CreateCampaignRequest, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_create"
	if !validCreateCampaign(input) {
		return nil, invalidArgument(operation, "campaign name, type, budget, period, tracking, or shared budget is invalid")
	}
	paused := true
	payload := campaignWrite{
		Name: input.Name, Type: input.Type, DailyBudget: input.DailyBudget,
		UseDailyBudget: input.UseDailyBudget, DeliveryMethod: input.DeliveryMethod,
		UsePeriod: input.UsePeriod, PeriodStart: input.PeriodStart, PeriodEnd: input.PeriodEnd,
		SharedBudgetID: input.SharedBudgetID, TrackingMode: input.TrackingMode,
		TrackingURL: input.TrackingURL, TrackingURLCustomParams: cloneRaw(input.TrackingURLCustomParams),
		UserLock: &paused,
	}
	var campaign Campaign
	if err := client.writeJSON(ctx, operation, http.MethodPost, "/ncc/campaigns", nil, payload, &campaign, options...); err != nil {
		return nil, err
	}
	if err := client.validateCampaign(operation, &campaign, ""); err != nil {
		return nil, outcomeUnknownError(operation, err)
	}
	if !campaign.UserLock || campaign.Status != StatusPaused {
		return nil, outcomeUnknownError(operation, platformContractError(operation, "created Campaign was not paused"))
	}
	return &campaign, nil
}

func (client *Client) UpdateCampaignBudget(ctx context.Context, id string, input CampaignBudgetUpdate, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_budget"
	if !validID(id) || input.DailyBudget < 0 || input.UseDailyBudget && input.DailyBudget == 0 || !input.UseDailyBudget && input.DailyBudget != 0 {
		return nil, invalidArgument(operation, "campaign ID or budget is invalid")
	}
	prepared, err := prepareCallOptions(operation, options)
	if err != nil {
		return nil, err
	}
	current, err := client.GetCampaign(ctx, id, prepared...)
	if err != nil {
		return nil, withOperation(err, operation)
	}
	payload := campaignPayload(current)
	payload.DailyBudget = &input.DailyBudget
	payload.UseDailyBudget = &input.UseDailyBudget
	return client.updateCampaign(ctx, operation, id, "budget", payload, prepared...)
}

func (client *Client) UpdateCampaignPeriod(ctx context.Context, id string, input CampaignPeriodUpdate, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_period"
	if !validID(id) || !validPeriodUpdate(input) {
		return nil, invalidArgument(operation, "campaign ID or period is invalid")
	}
	prepared, err := prepareCallOptions(operation, options)
	if err != nil {
		return nil, err
	}
	current, err := client.GetCampaign(ctx, id, prepared...)
	if err != nil {
		return nil, withOperation(err, operation)
	}
	payload := campaignPayload(current)
	payload.UsePeriod = &input.UsePeriod
	payload.PeriodStart, payload.PeriodEnd = input.Start, input.End
	return client.updateCampaign(ctx, operation, id, "period", payload, prepared...)
}

func (client *Client) SetCampaignPaused(ctx context.Context, id string, paused bool, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_set_paused"
	if !validID(id) {
		return nil, invalidArgument(operation, "campaign ID is invalid")
	}
	prepared, err := prepareCallOptions(operation, options)
	if err != nil {
		return nil, err
	}
	current, err := client.GetCampaign(ctx, id, prepared...)
	if err != nil {
		return nil, withOperation(err, operation)
	}
	if current.Status == StatusDeleted {
		return nil, invalidArgument(operation, "deleted Campaign cannot be changed")
	}
	payload := campaignPayload(current)
	payload.UserLock = &paused
	campaign, err := client.updateCampaign(ctx, operation, id, "userLock", payload, prepared...)
	if err != nil {
		return nil, err
	}
	if campaign.UserLock != paused {
		return nil, outcomeUnknownError(operation, platformContractError(operation, "Campaign lock did not match the requested state"))
	}
	return campaign, nil
}

func (client *Client) DeleteCampaign(ctx context.Context, id string, options ...socialhub.CallOption) error {
	const operation = "campaign_delete"
	if !validID(id) {
		return invalidArgument(operation, "campaign ID is invalid")
	}
	prepared, err := prepareCallOptions(operation, options)
	if err != nil {
		return err
	}
	current, err := client.GetCampaign(ctx, id, prepared...)
	if err != nil {
		return withOperation(err, operation)
	}
	if !current.UserLock || current.Status != StatusPaused {
		return invalidArgument(operation, "Campaign must be paused before deletion")
	}
	return client.delete(ctx, operation, "/ncc/campaigns/"+id, prepared...)
}

func (client *Client) updateCampaign(ctx context.Context, operation, id, field string, payload campaignWrite, options ...socialhub.CallOption) (*Campaign, error) {
	query := url.Values{"fields": {field}}
	var campaign Campaign
	if err := client.writeJSON(ctx, operation, http.MethodPut, "/ncc/campaigns/"+id, query, payload, &campaign, options...); err != nil {
		return nil, err
	}
	if err := client.validateCampaign(operation, &campaign, id); err != nil {
		return nil, outcomeUnknownError(operation, err)
	}
	return &campaign, nil
}

func (client *Client) validateCampaign(operation string, campaign *Campaign, expectedID string) error {
	if campaign == nil || !validID(campaign.ID) || campaign.CustomerID != client.customerID {
		return platformContractError(operation, "Campaign response has invalid ID or customer ownership")
	}
	if expectedID != "" && campaign.ID != expectedID {
		return platformContractError(operation, "Campaign response ID did not match the request")
	}
	return nil
}

func campaignPayload(value *Campaign) campaignWrite {
	return campaignWrite{
		ID: value.ID, CustomerID: value.CustomerID, Type: value.Type, Name: value.Name,
		DailyBudget: &value.DailyBudget, UseDailyBudget: &value.UseDailyBudget,
		DeliveryMethod: value.DeliveryMethod, UsePeriod: &value.UsePeriod,
		PeriodStart: value.PeriodStart, PeriodEnd: value.PeriodEnd, SharedBudgetID: value.SharedBudgetID,
		TrackingMode: value.TrackingMode, TrackingURL: value.TrackingURL, UserLock: &value.UserLock,
	}
}

func validCreateCampaign(input CreateCampaignRequest) bool {
	if !validText(input.Name, 128) || !validCampaignType(input.Type, false) ||
		!validDeliveryMethod(input.DeliveryMethod, true) || !validTrackingMode(input.TrackingMode, true) ||
		!validOpaqueOptional(input.SharedBudgetID, 128) || !validRawObject(input.TrackingURLCustomParams, true) {
		return false
	}
	if input.DailyBudget != nil && input.UseDailyBudget == nil {
		return false
	}
	if input.UseDailyBudget != nil {
		if *input.UseDailyBudget && (input.DailyBudget == nil || *input.DailyBudget <= 0) ||
			!*input.UseDailyBudget && input.DailyBudget != nil && *input.DailyBudget != 0 {
			return false
		}
	}
	if input.DailyBudget != nil && *input.DailyBudget < 0 {
		return false
	}
	if input.UsePeriod != nil {
		if *input.UsePeriod && !validRFC3339Range(input.PeriodStart, input.PeriodEnd) ||
			!*input.UsePeriod && (input.PeriodStart != "" || input.PeriodEnd != "") {
			return false
		}
	} else if input.PeriodStart != "" || input.PeriodEnd != "" {
		return false
	}
	if input.TrackingMode == TrackingPassThrough {
		return validURL(input.TrackingURL, false)
	}
	return input.TrackingURL == ""
}

func validPeriodUpdate(input CampaignPeriodUpdate) bool {
	if !input.UsePeriod {
		return input.Start == "" && input.End == ""
	}
	return validRFC3339Range(input.Start, input.End)
}

func validRFC3339Range(start, end string) bool {
	startTime, startErr := time.Parse(time.RFC3339, start)
	endTime, endErr := time.Parse(time.RFC3339, end)
	return startErr == nil && endErr == nil && !startTime.After(endTime)
}

func validOpaqueOptional(value string, maximum int) bool {
	return value == "" || validOpaque(value, maximum)
}

func listValues(input ListOptions) url.Values {
	values := url.Values{"recordSize": {strconv.Itoa(input.Limit)}, "selector": {string(input.Direction)}}
	if input.Cursor != "" {
		values.Set("baseSearchId", input.Cursor)
	}
	return values
}

func cloneRaw(value json.RawMessage) json.RawMessage { return append(json.RawMessage(nil), value...) }

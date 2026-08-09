package appleads

import (
	"context"
	"encoding/json"
	"math/big"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListCampaigns(ctx context.Context, pagination Pagination, options ...socialhub.CallOption) (Page[Campaign], error) {
	const operation = "campaigns_list"
	if !validPagination(pagination) {
		return Page[Campaign]{}, invalidArgument(operation, "pagination must use offset >= 0 and limit 1..1000")
	}
	var response responseEnvelope[[]Campaign]
	if err := client.getJSON(ctx, operation, "/campaigns", listQuery(pagination), &response, options...); err != nil {
		return Page[Campaign]{}, err
	}
	if err := checkEnvelopeError(operation, response.Error); err != nil {
		return Page[Campaign]{}, err
	}
	for index := range response.Data {
		if err := client.validateCampaign(operation, &response.Data[index], 0); err != nil {
			return Page[Campaign]{}, err
		}
	}
	return pageResult(response.Data, response.Pagination), nil
}

func (client *Client) FindCampaigns(ctx context.Context, selector Selector, options ...socialhub.CallOption) (Page[Campaign], error) {
	const operation = "campaigns_find"
	if !validSelector(selector) {
		return Page[Campaign]{}, invalidArgument(operation, "selector is invalid")
	}
	var response responseEnvelope[[]Campaign]
	if err := client.postJSON(ctx, operation, "/campaigns/find", selector, &response, options...); err != nil {
		return Page[Campaign]{}, err
	}
	if err := checkEnvelopeError(operation, response.Error); err != nil {
		return Page[Campaign]{}, err
	}
	for index := range response.Data {
		if err := client.validateCampaign(operation, &response.Data[index], 0); err != nil {
			return Page[Campaign]{}, err
		}
	}
	return pageResult(response.Data, response.Pagination), nil
}

func (client *Client) GetCampaign(ctx context.Context, campaignID int64, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_get"
	if !validID(campaignID) {
		return nil, invalidArgument(operation, "campaign ID must be positive")
	}
	var response responseEnvelope[Campaign]
	if err := client.getJSON(ctx, operation, "/campaigns/"+formatID(campaignID), nil, &response, options...); err != nil {
		return nil, err
	}
	if err := checkEnvelopeError(operation, response.Error); err != nil {
		return nil, err
	}
	if err := client.validateCampaign(operation, &response.Data, campaignID); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

type campaignCreate struct {
	OrgID              int64          `json:"orgId"`
	Name               string         `json:"name"`
	AdamID             int64          `json:"adamId"`
	DailyBudgetAmount  Money          `json:"dailyBudgetAmount"`
	BudgetAmount       *Money         `json:"budgetAmount,omitempty"`
	BillingEvent       string         `json:"billingEvent"`
	SupplySources      []string       `json:"supplySources"`
	CountriesOrRegions []string       `json:"countriesOrRegions"`
	AdChannelType      string         `json:"adChannelType"`
	BiddingStrategy    string         `json:"biddingStrategy,omitempty"`
	TargetCPA          *Money         `json:"targetCpa,omitempty"`
	StartTime          string         `json:"startTime,omitempty"`
	EndTime            string         `json:"endTime,omitempty"`
	Status             CampaignStatus `json:"status"`
}

func (client *Client) CreateCampaign(ctx context.Context, input CreateCampaignRequest, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_create"
	if err := validateCreateCampaign(input); err != nil {
		return nil, err
	}
	payload := campaignCreate{
		OrgID: client.orgID, Name: input.Name, AdamID: input.AdamID,
		DailyBudgetAmount: input.DailyBudgetAmount, BudgetAmount: input.BudgetAmount,
		BillingEvent: input.BillingEvent, SupplySources: append([]string(nil), input.SupplySources...),
		CountriesOrRegions: append([]string(nil), input.CountriesOrRegions...), AdChannelType: input.AdChannelType,
		BiddingStrategy: input.BiddingStrategy, TargetCPA: input.TargetCPA,
		StartTime: input.StartTime, EndTime: input.EndTime, Status: CampaignPaused,
	}
	var response responseEnvelope[Campaign]
	if err := client.postJSON(ctx, operation, "/campaigns", payload, &response, options...); err != nil {
		return nil, err
	}
	if err := checkEnvelopeError(operation, response.Error); err != nil {
		return nil, err
	}
	if err := client.validateCampaign(operation, &response.Data, 0); err != nil {
		return nil, err
	}
	if response.Data.Status != CampaignPaused {
		return nil, platformContractError(operation, "created Campaign was not paused")
	}
	return &response.Data, nil
}

type campaignWrite struct {
	Name               *string         `json:"name,omitempty"`
	DailyBudgetAmount  *Money          `json:"dailyBudgetAmount,omitempty"`
	BudgetAmount       *Money          `json:"budgetAmount,omitempty"`
	CountriesOrRegions []string        `json:"countriesOrRegions,omitempty"`
	BiddingStrategy    *string         `json:"biddingStrategy,omitempty"`
	TargetCPA          *Money          `json:"targetCpa,omitempty"`
	StartTime          *string         `json:"startTime,omitempty"`
	EndTime            *string         `json:"endTime,omitempty"`
	Status             *CampaignStatus `json:"status,omitempty"`
}

type updateCampaignPayload struct {
	Campaign                                 campaignWrite `json:"campaign"`
	ClearGeoTargetingOnCountryOrRegionChange bool          `json:"clearGeoTargetingOnCountryOrRegionChange"`
}

func (client *Client) UpdateCampaign(ctx context.Context, campaignID int64, input UpdateCampaignRequest, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_update"
	if !validID(campaignID) || !validUpdateCampaign(input) {
		return nil, invalidArgument(operation, "campaign ID or update fields are invalid")
	}
	if _, err := client.GetCampaign(ctx, campaignID, options...); err != nil {
		return nil, err
	}
	clearGeo := false
	if input.ClearGeoTargetingOnCountryChange != nil {
		clearGeo = *input.ClearGeoTargetingOnCountryChange
	}
	payload := updateCampaignPayload{Campaign: campaignWrite{
		Name: input.Name, DailyBudgetAmount: input.DailyBudgetAmount, BudgetAmount: input.BudgetAmount,
		CountriesOrRegions: append([]string(nil), input.CountriesOrRegions...), BiddingStrategy: input.BiddingStrategy,
		TargetCPA: input.TargetCPA, StartTime: input.StartTime, EndTime: input.EndTime,
	}, ClearGeoTargetingOnCountryOrRegionChange: clearGeo}
	return client.writeCampaign(ctx, operation, campaignID, payload, nil, options...)
}

func (client *Client) SetCampaignEnabled(ctx context.Context, campaignID int64, enabled bool, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_set_enabled"
	if !validID(campaignID) {
		return nil, invalidArgument(operation, "campaign ID must be positive")
	}
	if _, err := client.GetCampaign(ctx, campaignID, options...); err != nil {
		return nil, err
	}
	if enabled {
		groups, err := client.listAdGroups(ctx, operation, campaignID, Pagination{Offset: 0, Limit: 1000}, options...)
		if err != nil {
			return nil, err
		}
		if groups.Pagination.TotalResults == 0 || len(groups.Items) == 0 {
			return nil, invalidArgument(operation, "Campaign must contain at least one paused Ad Group before it can be enabled")
		}
		if groups.Pagination.TotalResults > int64(len(groups.Items)) {
			return nil, invalidArgument(operation, "Campaign has more Ad Groups than can be audited in one request")
		}
		for _, group := range groups.Items {
			if group.Deleted || group.Status != AdGroupPaused {
				return nil, invalidArgument(operation, "every Ad Group must be undeleted and paused before enabling the Campaign")
			}
		}
	}
	status := CampaignPaused
	if enabled {
		status = CampaignEnabled
	}
	payload := updateCampaignPayload{Campaign: campaignWrite{Status: &status}}
	return client.writeCampaign(ctx, operation, campaignID, payload, &status, options...)
}

func (client *Client) DeleteCampaign(ctx context.Context, campaignID int64, options ...socialhub.CallOption) error {
	const operation = "campaign_delete"
	current, err := client.GetCampaign(ctx, campaignID, options...)
	if err != nil {
		return err
	}
	if current.Status != CampaignPaused {
		return invalidArgument(operation, "Campaign must be paused before deletion")
	}
	var response responseEnvelope[json.RawMessage]
	if err := client.deleteJSON(ctx, operation, "/campaigns/"+formatID(campaignID), &response, options...); err != nil {
		return err
	}
	return checkEnvelopeError(operation, response.Error)
}

func (client *Client) writeCampaign(ctx context.Context, operation string, campaignID int64, payload updateCampaignPayload, expected *CampaignStatus, options ...socialhub.CallOption) (*Campaign, error) {
	var response responseEnvelope[Campaign]
	if err := client.putJSON(ctx, operation, "/campaigns/"+formatID(campaignID), payload, &response, options...); err != nil {
		return nil, err
	}
	if err := checkEnvelopeError(operation, response.Error); err != nil {
		return nil, err
	}
	if err := client.validateCampaign(operation, &response.Data, campaignID); err != nil {
		return nil, err
	}
	if expected != nil && response.Data.Status != *expected {
		return nil, platformContractError(operation, "Campaign status did not match the requested state")
	}
	return &response.Data, nil
}

func (client *Client) validateCampaign(operation string, campaign *Campaign, expectedID int64) error {
	if campaign == nil || !validID(campaign.ID) || campaign.OrgID != client.orgID {
		return platformContractError(operation, "Campaign response has invalid ID or organization ownership")
	}
	if expectedID != 0 && campaign.ID != expectedID {
		return platformContractError(operation, "Campaign response ID did not match the requested Campaign")
	}
	return nil
}

func validateCreateCampaign(input CreateCampaignRequest) error {
	const operation = "campaign_create"
	if !validText(input.Name, 200) || !validID(input.AdamID) || !validPositiveMoney(&input.DailyBudgetAmount) ||
		!validPositiveMoney(input.BudgetAmount) || !validCountries(input.CountriesOrRegions) || len(input.SupplySources) == 0 ||
		len(input.SupplySources) > 4 || !validDateTime(input.StartTime) || !validDateTime(input.EndTime) ||
		input.StartTime != "" && input.EndTime != "" && input.EndTime <= input.StartTime {
		return invalidArgument(operation, "Campaign fields are invalid")
	}
	if input.BillingEvent != "TAPS" && input.BillingEvent != "IMPRESSIONS" {
		return invalidArgument(operation, "billing event must be TAPS or IMPRESSIONS")
	}
	if input.AdChannelType != "SEARCH" && input.AdChannelType != "DISPLAY" {
		return invalidArgument(operation, "ad channel type must be SEARCH or DISPLAY")
	}
	allowedSources := map[string]bool{
		"APPSTORE_PRODUCT_PAGES_BROWSE": true, "APPSTORE_SEARCH_RESULTS": true,
		"APPSTORE_SEARCH_TAB": true, "APPSTORE_TODAY_TAB": true,
	}
	for _, source := range input.SupplySources {
		if !allowedSources[source] {
			return invalidArgument(operation, "supply source is invalid")
		}
	}
	switch input.BiddingStrategy {
	case "", "MANUAL_CPT":
		if input.TargetCPA != nil {
			return invalidArgument(operation, "MANUAL_CPT Campaign must not specify target CPA")
		}
	case "MAX_CONVERSIONS":
		if !validPositiveMoney(input.TargetCPA) || input.TargetCPA == nil || len(input.SupplySources) != 1 || input.SupplySources[0] != "APPSTORE_SEARCH_RESULTS" {
			return invalidArgument(operation, "MAX_CONVERSIONS requires a positive target CPA and APPSTORE_SEARCH_RESULTS only")
		}
	default:
		return invalidArgument(operation, "bidding strategy is invalid")
	}
	if input.BudgetAmount != nil {
		if input.BudgetAmount.Currency != input.DailyBudgetAmount.Currency || compareMoney(*input.BudgetAmount, input.DailyBudgetAmount) <= 0 {
			return invalidArgument(operation, "lifetime budget must use the daily budget currency and be greater than the daily budget")
		}
	}
	return nil
}

func validUpdateCampaign(input UpdateCampaignRequest) bool {
	if input.Name == nil && input.DailyBudgetAmount == nil && input.BudgetAmount == nil && len(input.CountriesOrRegions) == 0 &&
		input.BiddingStrategy == nil && input.TargetCPA == nil && input.StartTime == nil && input.EndTime == nil && input.ClearGeoTargetingOnCountryChange == nil {
		return false
	}
	if !validOptionalText(input.Name, 200) || !validPositiveMoney(input.DailyBudgetAmount) || !validPositiveMoney(input.BudgetAmount) ||
		len(input.CountriesOrRegions) > 0 && !validCountries(input.CountriesOrRegions) ||
		input.StartTime != nil && !validDateTime(*input.StartTime) || input.EndTime != nil && !validDateTime(*input.EndTime) {
		return false
	}
	if input.BiddingStrategy != nil && *input.BiddingStrategy != "MANUAL_CPT" && *input.BiddingStrategy != "MAX_CONVERSIONS" {
		return false
	}
	return true
}

func compareMoney(left, right Money) int {
	l, _ := new(big.Rat).SetString(left.Amount)
	r, _ := new(big.Rat).SetString(right.Amount)
	return l.Cmp(r)
}

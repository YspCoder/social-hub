package marketing

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

const (
	adAccountFields = "id,name,account_status,currency,timezone_name,amount_spent,balance,spend_cap,disable_reason,business{id,name}"
	campaignFields  = "id,account_id,name,objective,status,configured_status,effective_status,buying_type,bid_strategy,daily_budget,lifetime_budget,budget_remaining,special_ad_categories,created_time,updated_time"
)

func (client *Client) GetAdAccount(ctx context.Context, options ...socialhub.CallOption) (*AdAccount, error) {
	if err := client.requireRead("ad_account_get"); err != nil {
		return nil, err
	}
	var response AdAccount
	query := url.Values{"fields": {adAccountFields}}
	if err := client.api.JSON(ctx, http.MethodGet, "/"+client.adAccountResource(), query, nil, &response, options...); err != nil {
		return nil, err
	}
	actual := strings.TrimPrefix(response.ID, "act_")
	if err := requireResponseID("ad_account_get", client.adAccountID, actual); err != nil {
		return nil, err
	}
	return &response, nil
}

func (client *Client) GetCampaign(ctx context.Context, campaignID string, options ...socialhub.CallOption) (*Campaign, error) {
	if !validNumericID(campaignID) {
		return nil, invalidArgument("campaign_get", "campaign ID must be numeric")
	}
	if err := client.requireRead("campaign_get"); err != nil {
		return nil, err
	}
	var response Campaign
	query := url.Values{"fields": {campaignFields}}
	if err := client.api.JSON(ctx, http.MethodGet, "/"+url.PathEscape(campaignID), query, nil, &response, options...); err != nil {
		return nil, err
	}
	if err := requireResponseID("campaign_get", campaignID, response.ID); err != nil {
		return nil, err
	}
	return &response, nil
}

func (client *Client) ListCampaigns(ctx context.Context, input ListCampaignsRequest, options ...socialhub.CallOption) (socialhub.Page[Campaign], error) {
	limit, err := validatePage(input.Cursor, input.MaxResults)
	if err != nil {
		return socialhub.Page[Campaign]{}, err
	}
	if !validateStatuses(input.EffectiveStatuses) {
		return socialhub.Page[Campaign]{}, invalidArgument("campaign_list", "effective statuses must be uppercase enum tokens")
	}
	if err := client.requireRead("campaign_list"); err != nil {
		return socialhub.Page[Campaign]{}, err
	}
	query := url.Values{"fields": {campaignFields}}
	addPaging(query, input.Cursor, limit)
	if len(input.EffectiveStatuses) > 0 {
		if err := setJSONForm(query, "effective_status", input.EffectiveStatuses, "campaign_list"); err != nil {
			return socialhub.Page[Campaign]{}, err
		}
	}
	var response graphPage[Campaign]
	path := "/" + client.adAccountResource() + "/campaigns"
	if err := client.api.JSON(ctx, http.MethodGet, path, query, nil, &response, options...); err != nil {
		return socialhub.Page[Campaign]{}, err
	}
	for _, campaign := range response.Data {
		if err := requireResponseID("campaign_list", "", campaign.ID); err != nil {
			return socialhub.Page[Campaign]{}, err
		}
	}
	return toPage(response), nil
}

func (client *Client) CreateCampaign(ctx context.Context, input CreateCampaignRequest, options ...socialhub.CallOption) (*Campaign, error) {
	if !validRequiredText(input.Name, 512) || !validEnumToken(string(input.Objective)) {
		return nil, invalidArgument("campaign_create", "name and an uppercase objective are required")
	}
	if input.BuyingType != "" && !validEnumToken(input.BuyingType) || input.BidStrategy != "" && !validEnumToken(input.BidStrategy) {
		return nil, invalidArgument("campaign_create", "buying type and bid strategy must be uppercase enum tokens")
	}
	if !validateBudget(input.DailyBudget, input.LifetimeBudget) {
		return nil, invalidArgument("campaign_create", "daily and lifetime budgets must be non-negative and mutually exclusive")
	}
	for _, category := range input.SpecialAdCategories {
		if !validEnumToken(string(category)) {
			return nil, invalidArgument("campaign_create", "special ad categories must be uppercase enum tokens")
		}
	}
	if err := client.requireManagement("campaign_create"); err != nil {
		return nil, err
	}
	form := url.Values{
		"name": {input.Name}, "objective": {string(input.Objective)}, "status": {string(StatusPaused)},
	}
	if input.BuyingType != "" {
		form.Set("buying_type", input.BuyingType)
	}
	if input.BidStrategy != "" {
		form.Set("bid_strategy", input.BidStrategy)
	}
	setPositiveInt(form, "daily_budget", input.DailyBudget)
	setPositiveInt(form, "lifetime_budget", input.LifetimeBudget)
	categories := input.SpecialAdCategories
	if categories == nil {
		categories = []SpecialAdCategory{}
	}
	if err := setJSONForm(form, "special_ad_categories", categories, "campaign_create"); err != nil {
		return nil, err
	}
	var response idResponse
	path := "/" + client.adAccountResource() + "/campaigns"
	if err := client.postForm(ctx, path, form, &response, options...); err != nil {
		return nil, err
	}
	if err := requireResponseID("campaign_create", "", response.ID); err != nil {
		return nil, err
	}
	return &Campaign{
		ID: response.ID, AccountID: client.adAccountID, Name: input.Name, Objective: input.Objective,
		Status: StatusPaused, BuyingType: input.BuyingType, BidStrategy: input.BidStrategy,
		DailyBudget: positiveIntString(input.DailyBudget), LifetimeBudget: positiveIntString(input.LifetimeBudget),
		SpecialAdCategories: append([]SpecialAdCategory(nil), categories...),
	}, nil
}

func (client *Client) UpdateCampaign(ctx context.Context, campaignID string, input UpdateCampaignRequest, options ...socialhub.CallOption) error {
	if !validNumericID(campaignID) {
		return invalidArgument("campaign_update", "campaign ID must be numeric")
	}
	if input.Name == nil && input.Status == nil && input.BidStrategy == nil && input.DailyBudget == nil && input.LifetimeBudget == nil && input.SpecialAdCategories == nil {
		return invalidArgument("campaign_update", "at least one patch field is required")
	}
	if input.Name != nil && !validRequiredText(*input.Name, 512) || input.Status != nil && !validMutationStatus(*input.Status) ||
		input.BidStrategy != nil && !validEnumToken(*input.BidStrategy) {
		return invalidArgument("campaign_update", "one or more patch fields are invalid")
	}
	daily, lifetime := int64(0), int64(0)
	if input.DailyBudget != nil {
		daily = *input.DailyBudget
	}
	if input.LifetimeBudget != nil {
		lifetime = *input.LifetimeBudget
	}
	if !validateBudget(daily, lifetime) {
		return invalidArgument("campaign_update", "budgets must be non-negative and mutually exclusive when supplied together")
	}
	if input.SpecialAdCategories != nil {
		for _, category := range *input.SpecialAdCategories {
			if !validEnumToken(string(category)) {
				return invalidArgument("campaign_update", "special ad categories must be uppercase enum tokens")
			}
		}
	}
	if err := client.requireManagement("campaign_update"); err != nil {
		return err
	}
	form := url.Values{}
	if input.Name != nil {
		form.Set("name", *input.Name)
	}
	if input.Status != nil {
		form.Set("status", string(*input.Status))
	}
	if input.BidStrategy != nil {
		form.Set("bid_strategy", *input.BidStrategy)
	}
	if input.DailyBudget != nil {
		form.Set("daily_budget", strconv.FormatInt(*input.DailyBudget, 10))
	}
	if input.LifetimeBudget != nil {
		form.Set("lifetime_budget", strconv.FormatInt(*input.LifetimeBudget, 10))
	}
	if input.SpecialAdCategories != nil {
		if err := setJSONForm(form, "special_ad_categories", *input.SpecialAdCategories, "campaign_update"); err != nil {
			return err
		}
	}
	var response successResponse
	if err := client.postForm(ctx, "/"+url.PathEscape(campaignID), form, &response, options...); err != nil {
		return err
	}
	return requireMutationSuccess("campaign_update", response)
}

func positiveIntString(value int64) string {
	if value <= 0 {
		return ""
	}
	return strconv.FormatInt(value, 10)
}

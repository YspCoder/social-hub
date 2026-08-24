package marketing

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"social-hub/pkg/socialhub"
)

const adSetFields = "id,account_id,campaign_id,name,status,configured_status,effective_status,optimization_goal,billing_event,bid_strategy,bid_amount,daily_budget,lifetime_budget,budget_remaining,start_time,end_time,targeting,promoted_object,created_time,updated_time"

func (client *Client) GetAdSet(ctx context.Context, adSetID string, options ...socialhub.CallOption) (*AdSet, error) {
	if !validNumericID(adSetID) {
		return nil, invalidArgument("adset_get", "ad set ID must be numeric")
	}
	if err := client.requireRead("adset_get"); err != nil {
		return nil, err
	}
	var response AdSet
	query := url.Values{"fields": {adSetFields}}
	if err := client.api.JSON(ctx, http.MethodGet, "/"+url.PathEscape(adSetID), query, nil, &response, options...); err != nil {
		return nil, err
	}
	if err := requireResponseID("adset_get", adSetID, response.ID); err != nil {
		return nil, err
	}
	return &response, nil
}

func (client *Client) ListAdSets(ctx context.Context, input ListAdSetsRequest, options ...socialhub.CallOption) (socialhub.Page[AdSet], error) {
	limit, err := validatePage(input.Cursor, input.MaxResults)
	if err != nil {
		return socialhub.Page[AdSet]{}, err
	}
	if input.CampaignID != "" && !validNumericID(input.CampaignID) || !validateStatuses(input.EffectiveStatuses) {
		return socialhub.Page[AdSet]{}, invalidArgument("adset_list", "campaign ID or effective status is invalid")
	}
	if err := client.requireRead("adset_list"); err != nil {
		return socialhub.Page[AdSet]{}, err
	}
	query := url.Values{"fields": {adSetFields}}
	addPaging(query, input.Cursor, limit)
	if len(input.EffectiveStatuses) > 0 {
		if err := setJSONForm(query, "effective_status", input.EffectiveStatuses, "adset_list"); err != nil {
			return socialhub.Page[AdSet]{}, err
		}
	}
	parent := client.adAccountResource()
	if input.CampaignID != "" {
		parent = url.PathEscape(input.CampaignID)
	}
	var response graphPage[AdSet]
	if err := client.api.JSON(ctx, http.MethodGet, "/"+parent+"/adsets", query, nil, &response, options...); err != nil {
		return socialhub.Page[AdSet]{}, err
	}
	for _, item := range response.Data {
		if err := requireResponseID("adset_list", "", item.ID); err != nil {
			return socialhub.Page[AdSet]{}, err
		}
	}
	return toPage(response), nil
}

func (client *Client) CreateAdSet(ctx context.Context, input CreateAdSetRequest, options ...socialhub.CallOption) (*AdSet, error) {
	if !validRequiredText(input.Name, 512) || !validNumericID(input.CampaignID) || !validEnumToken(input.OptimizationGoal) || !validEnumToken(string(input.BillingEvent)) {
		return nil, invalidArgument("adset_create", "name, campaign ID, optimization goal, and billing event are required")
	}
	if input.BidStrategy != "" && !validEnumToken(input.BidStrategy) || input.BidAmount < 0 || !validateBudget(input.DailyBudget, input.LifetimeBudget) {
		return nil, invalidArgument("adset_create", "bid or budget fields are invalid")
	}
	if !validateTargeting(input.Targeting) || !validatePromotedObject(input.PromotedObject) {
		return nil, invalidArgument("adset_create", "targeting or promoted object is invalid")
	}
	if !validSchedule(input.StartTime, input.EndTime) {
		return nil, invalidArgument("adset_create", "end_time must be after start_time")
	}
	if err := client.requireManagement("adset_create"); err != nil {
		return nil, err
	}
	form := url.Values{
		"name": {input.Name}, "campaign_id": {input.CampaignID},
		"optimization_goal": {input.OptimizationGoal}, "billing_event": {string(input.BillingEvent)},
		"status": {string(StatusPaused)},
	}
	if input.BidStrategy != "" {
		form.Set("bid_strategy", input.BidStrategy)
	}
	setPositiveInt(form, "bid_amount", input.BidAmount)
	setPositiveInt(form, "daily_budget", input.DailyBudget)
	setPositiveInt(form, "lifetime_budget", input.LifetimeBudget)
	setTime(form, "start_time", input.StartTime)
	setTime(form, "end_time", input.EndTime)
	if err := setJSONForm(form, "targeting", input.Targeting, "adset_create"); err != nil {
		return nil, err
	}
	if input.PromotedObject != nil {
		if err := setJSONForm(form, "promoted_object", input.PromotedObject, "adset_create"); err != nil {
			return nil, err
		}
	}
	var response idResponse
	if err := client.postForm(ctx, "/"+client.adAccountResource()+"/adsets", form, &response, options...); err != nil {
		return nil, err
	}
	if err := requireResponseID("adset_create", "", response.ID); err != nil {
		return nil, err
	}
	return &AdSet{
		ID: response.ID, AccountID: client.adAccountID, CampaignID: input.CampaignID, Name: input.Name,
		Status: StatusPaused, OptimizationGoal: input.OptimizationGoal, BillingEvent: input.BillingEvent,
		BidStrategy: input.BidStrategy, BidAmount: positiveIntString(input.BidAmount),
		DailyBudget: positiveIntString(input.DailyBudget), LifetimeBudget: positiveIntString(input.LifetimeBudget),
		Targeting: input.Targeting, PromotedObject: input.PromotedObject,
	}, nil
}

func (client *Client) UpdateAdSet(ctx context.Context, adSetID string, input UpdateAdSetRequest, options ...socialhub.CallOption) error {
	if !validNumericID(adSetID) {
		return invalidArgument("adset_update", "ad set ID must be numeric")
	}
	if input.Name == nil && input.Status == nil && input.BidAmount == nil && input.DailyBudget == nil && input.LifetimeBudget == nil && input.EndTime == nil && input.Targeting == nil && input.PromotedObject == nil {
		return invalidArgument("adset_update", "at least one patch field is required")
	}
	if input.Name != nil && !validRequiredText(*input.Name, 512) || input.Status != nil && !validMutationStatus(*input.Status) {
		return invalidArgument("adset_update", "name or status is invalid")
	}
	daily, lifetime := int64(0), int64(0)
	if input.DailyBudget != nil {
		daily = *input.DailyBudget
	}
	if input.LifetimeBudget != nil {
		lifetime = *input.LifetimeBudget
	}
	if input.BidAmount != nil && *input.BidAmount < 0 || !validateBudget(daily, lifetime) ||
		input.Targeting != nil && !validateTargeting(*input.Targeting) || !validatePromotedObject(input.PromotedObject) {
		return invalidArgument("adset_update", "bid, budget, targeting, or promoted object is invalid")
	}
	if err := client.requireManagement("adset_update"); err != nil {
		return err
	}
	form := url.Values{}
	if input.Name != nil {
		form.Set("name", *input.Name)
	}
	if input.Status != nil {
		form.Set("status", string(*input.Status))
	}
	if input.BidAmount != nil {
		form.Set("bid_amount", strconv.FormatInt(*input.BidAmount, 10))
	}
	if input.DailyBudget != nil {
		form.Set("daily_budget", strconv.FormatInt(*input.DailyBudget, 10))
	}
	if input.LifetimeBudget != nil {
		form.Set("lifetime_budget", strconv.FormatInt(*input.LifetimeBudget, 10))
	}
	setTime(form, "end_time", input.EndTime)
	if input.Targeting != nil {
		if err := setJSONForm(form, "targeting", input.Targeting, "adset_update"); err != nil {
			return err
		}
	}
	if input.PromotedObject != nil {
		if err := setJSONForm(form, "promoted_object", input.PromotedObject, "adset_update"); err != nil {
			return err
		}
	}
	var response successResponse
	if err := client.postForm(ctx, "/"+url.PathEscape(adSetID), form, &response, options...); err != nil {
		return err
	}
	return requireMutationSuccess("adset_update", response)
}

func validSchedule(start, end *time.Time) bool {
	return start == nil || end == nil || end.After(*start)
}

func setTime(form url.Values, key string, value *time.Time) {
	if value != nil {
		form.Set(key, value.Format(time.RFC3339))
	}
}

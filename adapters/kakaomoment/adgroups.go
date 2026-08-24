package kakaomoment

import (
	"context"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

type adGroupListResponse struct {
	Content []AdGroup `json:"content"`
}

type adGroupBudgetWire struct {
	ID                int64 `json:"id"`
	DailyBudgetAmount int64 `json:"dailyBudgetAmount"`
}

type adGroupBidWire struct {
	ID        int64 `json:"id"`
	BidAmount int64 `json:"bidAmount"`
}

func (client *Client) ListAdGroups(ctx context.Context, campaignID int64, configs []ConfigStatus, options ...socialhub.CallOption) ([]AdGroup, error) {
	const operation = "adgroup_list"
	if campaignID <= 0 || !validConfigFilter(configs) {
		return nil, invalidArgument(operation, "campaign ID or Ad Group config filters are invalid")
	}
	query := url.Values{"campaignId": {formatID(campaignID)}}
	if len(configs) > 0 {
		query.Set("config", joinConfigs(configs))
	}
	var response adGroupListResponse
	_, err := client.doJSON(
		ctx, operation, []string{ScopeManagement}, http.MethodGet,
		"adGroups", query, nil, &response, false, options...,
	)
	if err != nil {
		return nil, err
	}
	if response.Content == nil {
		return nil, platformContractError(operation, "Kakao Moment Ad Group response omitted content")
	}
	for index := range response.Content {
		if err := validateAdGroup(operation, &response.Content[index], false); err != nil {
			return nil, err
		}
	}
	return response.Content, nil
}

func (client *Client) GetAdGroup(ctx context.Context, id int64, options ...socialhub.CallOption) (*AdGroup, error) {
	const operation = "adgroup_get"
	if id <= 0 {
		return nil, invalidArgument(operation, "Ad Group ID must be positive")
	}
	var adGroup AdGroup
	_, err := client.doJSON(
		ctx, operation, []string{ScopeManagement}, http.MethodGet,
		"adGroups/"+formatID(id), nil, nil, &adGroup, false, options...,
	)
	if err != nil {
		return nil, err
	}
	if err := validateAdGroup(operation, &adGroup, true); err != nil {
		return nil, err
	}
	if adGroup.ID != id || adGroup.Campaign == nil || adGroup.Campaign.AdAccountID != client.adAccountID {
		return nil, platformContractError(operation, "Kakao Moment returned a different or unbound Ad Group")
	}
	return &adGroup, nil
}

func (client *Client) SetAdGroupDailyBudget(ctx context.Context, id, amount int64, options ...socialhub.CallOption) error {
	const operation = "adgroup_set_daily_budget"
	if id <= 0 || amount < 10_000 || amount > 500_000_000 || amount%10 != 0 {
		return invalidArgument(operation, "Ad Group ID or daily budget is invalid; budget must be 10,000-500,000,000 KRW in multiples of 10")
	}
	_, err := client.doJSON(
		ctx, operation, []string{ScopeManagement}, http.MethodPut,
		"adGroups/dailyBudgetAmount", nil, adGroupBudgetWire{ID: id, DailyBudgetAmount: amount}, nil, true, options...,
	)
	return err
}

func (client *Client) SetAdGroupBidAmount(ctx context.Context, id, amount int64, options ...socialhub.CallOption) error {
	const operation = "adgroup_set_bid_amount"
	if id <= 0 || amount <= 0 || amount > 100_000 {
		return invalidArgument(operation, "Ad Group ID or bid amount is invalid; bid must be positive and no more than 100,000 KRW")
	}
	_, err := client.doJSON(
		ctx, operation, []string{ScopeManagement}, http.MethodPut,
		"adGroups/bidAmount", nil, adGroupBidWire{ID: id, BidAmount: amount}, nil, true, options...,
	)
	return err
}

func (client *Client) SetAdGroupConfig(ctx context.Context, id int64, config ConfigStatus, options ...socialhub.CallOption) error {
	const operation = "adgroup_set_config"
	if id <= 0 || !validConfig(config, false) {
		return invalidArgument(operation, "Ad Group ID must be positive and config must be ON or OFF")
	}
	_, err := client.doJSON(
		ctx, operation, []string{ScopeManagement}, http.MethodPut,
		"adGroups/onOff", nil, configWire{ID: id, Config: config}, nil, true, options...,
	)
	return err
}

func (client *Client) DeleteAdGroup(ctx context.Context, id int64, options ...socialhub.CallOption) error {
	const operation = "adgroup_delete"
	if id <= 0 {
		return invalidArgument(operation, "Ad Group ID must be positive")
	}
	adGroup, err := client.GetAdGroup(ctx, id, options...)
	if err != nil {
		return withOperation(err, operation)
	}
	if adGroup.Config != ConfigOff {
		return conflict(operation, "Ad Group must be OFF before guarded deletion")
	}
	_, err = client.doJSON(
		ctx, operation, []string{ScopeManagement, ScopeDelete}, http.MethodDelete,
		"adGroups/"+formatID(id), nil, nil, nil, true, options...,
	)
	return err
}

func validateAdGroup(operation string, adGroup *AdGroup, detailed bool) error {
	if adGroup == nil || adGroup.ID <= 0 || !validText(adGroup.Name, 1024) || !validConfig(adGroup.Config, true) {
		return platformContractError(operation, "Kakao Moment returned an invalid Ad Group")
	}
	if detailed && (adGroup.Campaign == nil || adGroup.Campaign.ID <= 0 || adGroup.DailyBudgetAmount != nil && *adGroup.DailyBudgetAmount < 0) {
		return platformContractError(operation, "Kakao Moment returned invalid Ad Group detail")
	}
	return nil
}

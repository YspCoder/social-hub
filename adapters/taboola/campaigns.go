package taboola

import (
	"context"
	"net/url"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListCampaigns(ctx context.Context, input ListCampaignsRequest, options ...socialhub.CallOption) (CampaignPage, error) {
	const operation = "campaigns_list"
	if !validCampaignList(input) {
		return CampaignPage{}, invalidArgument(operation, "page and page_size must be provided together and list filters must be valid")
	}
	query := paginationQuery(input.Page, input.PageSize)
	if input.FetchLevel != "" {
		query.Set("fetch_level", input.FetchLevel)
	}
	if input.Sort != "" {
		query.Set("sort", input.Sort)
	}
	var response pageEnvelope[Campaign]
	if err := client.getJSON(ctx, operation, client.accountPath("campaigns/"), query, &response, options...); err != nil {
		return CampaignPage{}, err
	}
	for index := range response.Results {
		if err := client.validateCampaign(operation, &response.Results[index], ""); err != nil {
			return CampaignPage{}, err
		}
	}
	hasMore := input.Page > 0 && input.PageSize > 0 && input.Page*input.PageSize < response.Metadata.Total
	return CampaignPage{
		Items: response.Results, Page: input.Page, PageSize: input.PageSize,
		Total: response.Metadata.Total, Count: response.Metadata.Count, HasMore: hasMore,
	}, nil
}

func (client *Client) GetCampaign(ctx context.Context, campaignID string, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_get"
	if !validPathID(campaignID, true) {
		return nil, invalidArgument(operation, "campaign ID is invalid")
	}
	var response Campaign
	if err := client.getJSON(ctx, operation, client.accountPath("campaigns/"+url.PathEscape(campaignID)+"/"), nil, &response, options...); err != nil {
		return nil, err
	}
	if err := client.validateCampaign(operation, &response, campaignID); err != nil {
		return nil, err
	}
	return &response, nil
}

type campaignWrite struct {
	Name               *string             `json:"name,omitempty"`
	BrandingText       *string             `json:"branding_text,omitempty"`
	BidStrategy        *BidStrategy        `json:"bid_strategy,omitempty"`
	MarketingObjective *string             `json:"marketing_objective,omitempty"`
	CPC                *float64            `json:"cpc,omitempty"`
	DailyCap           *float64            `json:"daily_cap,omitempty"`
	SpendingLimit      *float64            `json:"spending_limit,omitempty"`
	SpendingLimitModel *SpendingLimitModel `json:"spending_limit_model,omitempty"`
	StartDate          *string             `json:"start_date,omitempty"`
	EndDate            *string             `json:"end_date,omitempty"`
	IsActive           *bool               `json:"is_active,omitempty"`
}

func (client *Client) CreateCampaign(ctx context.Context, input CreateCampaignRequest, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_create"
	if err := validateCreateCampaign(input); err != nil {
		return nil, err
	}
	paused := false
	payload := campaignWrite{
		Name: &input.Name, BrandingText: &input.BrandingText, BidStrategy: &input.BidStrategy,
		MarketingObjective: &input.MarketingObjective, CPC: input.CPC, DailyCap: input.DailyCap,
		SpendingLimit: input.SpendingLimit, SpendingLimitModel: &input.SpendingLimitModel,
		IsActive: &paused,
	}
	if input.StartDate != "" {
		payload.StartDate = &input.StartDate
	}
	if input.EndDate != "" {
		payload.EndDate = &input.EndDate
	}
	var response Campaign
	if err := client.postJSON(ctx, operation, client.accountPath("campaigns/"), payload, &response, options...); err != nil {
		return nil, err
	}
	if err := client.validateCampaign(operation, &response, ""); err != nil {
		return nil, err
	}
	if response.IsActive == nil || *response.IsActive || response.Status == CampaignRunning {
		return nil, platformContractError(operation, "created Campaign was not paused")
	}
	return &response, nil
}

func (client *Client) UpdateCampaign(ctx context.Context, campaignID string, input UpdateCampaignRequest, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_update"
	if !validPathID(campaignID, true) || !validUpdateCampaign(input) {
		return nil, invalidArgument(operation, "campaign ID or update fields are invalid")
	}
	if _, err := client.GetCampaign(ctx, campaignID, options...); err != nil {
		return nil, err
	}
	payload := campaignWrite{
		Name: input.Name, BrandingText: input.BrandingText, MarketingObjective: input.MarketingObjective,
		CPC: input.CPC, DailyCap: input.DailyCap, SpendingLimit: input.SpendingLimit,
		SpendingLimitModel: input.SpendingLimitModel, StartDate: input.StartDate, EndDate: input.EndDate,
	}
	return client.writeCampaign(ctx, operation, campaignID, payload, options...)
}

func (client *Client) SetCampaignActive(ctx context.Context, campaignID string, active bool, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_set_active"
	if !validPathID(campaignID, true) {
		return nil, invalidArgument(operation, "campaign ID is invalid")
	}
	if _, err := client.GetCampaign(ctx, campaignID, options...); err != nil {
		return nil, err
	}
	if active {
		items, err := client.listItems(ctx, campaignID, options...)
		if err != nil {
			return nil, err
		}
		if len(items) == 0 {
			return nil, invalidArgument(operation, "Campaign must contain at least one explicitly paused Item before it can be enabled")
		}
		for _, item := range items {
			if item.IsActive == nil {
				return nil, platformContractError(operation, "Item response omitted is_active")
			}
			if *item.IsActive || item.Status == ItemCrawling || item.Status == ItemPendingApproval {
				return nil, invalidArgument(operation, "every Item must be explicitly paused and out of CRAWLING/PENDING_APPROVAL before enabling the Campaign")
			}
		}
	}
	return client.writeCampaign(ctx, operation, campaignID, campaignWrite{IsActive: &active}, options...)
}

func (client *Client) writeCampaign(ctx context.Context, operation, campaignID string, payload campaignWrite, options ...socialhub.CallOption) (*Campaign, error) {
	var response Campaign
	if err := client.postJSON(ctx, operation, client.accountPath("campaigns/"+url.PathEscape(campaignID)), payload, &response, options...); err != nil {
		return nil, err
	}
	if err := client.validateCampaign(operation, &response, campaignID); err != nil {
		return nil, err
	}
	if payload.IsActive != nil && (response.IsActive == nil || *response.IsActive != *payload.IsActive) {
		return nil, platformContractError(operation, "Campaign is_active did not match the requested state")
	}
	return &response, nil
}

func (client *Client) validateCampaign(operation string, campaign *Campaign, expectedID string) error {
	if campaign == nil || !validPathID(campaign.ID, true) || campaign.AdvertiserID != client.advertiserID {
		return platformContractError(operation, "Campaign response has invalid ID or advertiser ownership")
	}
	if expectedID != "" && campaign.ID != expectedID {
		return platformContractError(operation, "Campaign response ID did not match the requested Campaign")
	}
	return nil
}

func validateCreateCampaign(input CreateCampaignRequest) error {
	const operation = "campaign_create"
	if !validText(input.Name, 1024) || !validText(input.BrandingText, 1024) || !validText(input.MarketingObjective, 128) ||
		!validPositive(input.CPC) || !validPositive(input.DailyCap) || !validPositive(input.SpendingLimit) ||
		!validDate(input.StartDate) || !validDate(input.EndDate) {
		return invalidArgument(operation, "Campaign fields are invalid")
	}
	if input.StartDate != "" && input.EndDate != "" && input.EndDate < input.StartDate {
		return invalidArgument(operation, "end_date must not precede start_date")
	}
	switch input.BidStrategy {
	case BidStrategyFixed:
		if input.CPC == nil {
			return invalidArgument(operation, "FIXED bid strategy requires cpc")
		}
	case BidStrategyMaxConversions:
	default:
		return invalidArgument(operation, "bid strategy is invalid")
	}
	switch input.SpendingLimitModel {
	case SpendingNone:
		if input.DailyCap == nil || input.SpendingLimit != nil {
			return invalidArgument(operation, "NONE spending model requires daily_cap and forbids spending_limit")
		}
	case SpendingMonthly, SpendingEntire:
		if input.SpendingLimit == nil {
			return invalidArgument(operation, "spending model requires spending_limit")
		}
	default:
		return invalidArgument(operation, "spending_limit_model is invalid")
	}
	return nil
}

func validUpdateCampaign(input UpdateCampaignRequest) bool {
	if input == (UpdateCampaignRequest{}) || !validOptionalText(input.Name, 1024) || !validOptionalText(input.BrandingText, 1024) ||
		!validOptionalText(input.MarketingObjective, 128) || !validPositive(input.CPC) || !validPositive(input.DailyCap) ||
		!validPositive(input.SpendingLimit) {
		return false
	}
	if input.StartDate != nil && !validDate(*input.StartDate) || input.EndDate != nil && !validDate(*input.EndDate) {
		return false
	}
	if input.SpendingLimitModel != nil {
		switch *input.SpendingLimitModel {
		case SpendingNone, SpendingMonthly, SpendingEntire:
		default:
			return false
		}
	}
	return true
}

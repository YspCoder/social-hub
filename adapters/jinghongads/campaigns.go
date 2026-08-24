package jinghongads

import (
	"context"
	"encoding/json"
	"net/http"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListCampaigns(ctx context.Context, input ListCampaignsRequest, options ...socialhub.CallOption) (CampaignPage, error) {
	const operation = "campaigns_list"
	if err := validateCampaignRequest(input); err != nil {
		return CampaignPage{}, err
	}
	body := struct {
		AdvertiserID Decimal         `json:"advertiser_id,omitempty"`
		Page         int             `json:"page"`
		PageSize     int             `json:"page_size"`
		Filtering    *CampaignFilter `json:"filtering,omitempty"`
	}{Page: input.Page, PageSize: input.PageSize}
	if client.advertiserID != "" {
		body.AdvertiserID = Decimal(client.advertiserID)
	}
	if !emptyCampaignFilter(input.Filter) {
		filter := input.Filter
		filter.IDs = append([]string(nil), filter.IDs...)
		body.Filtering = &filter
	}
	var response struct {
		Total     json.RawMessage `json:"total"`
		Campaigns []Campaign      `json:"data"`
	}
	// Huawei's mainland new-delivery contract uses a JSON body on this GET.
	if err := client.doJSON(ctx, operation, ScopePromotion, http.MethodGet, "/ads/v1/promotion/campaign/query", body, &response, options...); err != nil {
		return CampaignPage{}, err
	}
	total, err := decodeNonnegativeInt(response.Total, 1_000_000_000)
	if err != nil || len(response.Campaigns) > input.PageSize || total < len(response.Campaigns) {
		return CampaignPage{}, platformContractError(operation, "Jinghong returned invalid Campaign pagination")
	}
	for _, campaign := range response.Campaigns {
		if !validCampaign(campaign) {
			return CampaignPage{}, platformContractError(operation, "Jinghong returned an invalid Campaign")
		}
	}
	return CampaignPage{
		Campaigns: response.Campaigns, Page: input.Page, PageSize: input.PageSize, Total: total,
		HasMore: input.Page*input.PageSize < total,
	}, nil
}

func emptyCampaignFilter(filter CampaignFilter) bool {
	return filter.Name == "" && len(filter.IDs) == 0 && filter.UpdatedBeginTime == "" && filter.UpdatedEndTime == "" &&
		filter.CreatedBeginTime == "" && filter.CreatedEndTime == "" && filter.ShowStatus == "" && filter.CampaignType == ""
}

func validCampaign(campaign Campaign) bool {
	return validID(campaign.ID) && validResponseText(campaign.Name, 512) && validResponseText(campaign.Status, 128) &&
		validResponseText(campaign.ShowStatus, 128) && validResponseText(campaign.DailyBudgetStatus, 128) &&
		validResponseText(campaign.UserBalanceStatus, 128) && validResponseText(campaign.ProductType, 128) &&
		validReportValue(campaign.TodayDailyBudget) && validReportValue(campaign.TomorrowBudget) &&
		(campaign.CreatedTime == "" || validTimestamp(campaign.CreatedTime)) && validResponseText(campaign.Type, 128) &&
		validResponseText(campaign.FlowResource, 128) && (campaign.StoreID == "" || validID(campaign.StoreID))
}

func validReportValue(value ReportValue) bool {
	return value.Null || validResponseText(value.Text, 4096)
}

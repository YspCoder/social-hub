package xiaohongshumarketing

import (
	"context"

	"social-hub/pkg/socialhub"
)

const (
	campaignListPath   = "/api/open/jg/campaign/list"
	campaignUpdatePath = "/api/open/jg/cascade/modify"
)

type pageRequestWire struct {
	PageIndex int `json:"page_index"`
	PageSize  int `json:"page_size"`
}

type pageResponseWire struct {
	PageIndex  int   `json:"page_index"`
	TotalCount int64 `json:"total_count"`
}

type listCampaignsWire struct {
	AdvertiserID    uint64          `json:"advertiser_id"`
	CampaignIDs     []uint64        `json:"campaign_ids,omitempty"`
	StartDate       Date            `json:"start_time,omitempty"`
	EndDate         Date            `json:"expire_time,omitempty"`
	Status          int             `json:"status,omitempty"`
	UpdateStartDate Date            `json:"update_start_date,omitempty"`
	UpdateEndDate   Date            `json:"update_end_date,omitempty"`
	Page            pageRequestWire `json:"page"`
}

func (client *Client) ListCampaigns(ctx context.Context, input ListCampaignsRequest, options ...socialhub.CallOption) (NumberPage[Campaign], error) {
	const operation = "campaign_list"
	if err := validateListCampaigns(input); err != nil {
		return NumberPage[Campaign]{}, err
	}
	page, pageSize, err := normalizePage(operation, input.Page, input.PageSize)
	if err != nil {
		return NumberPage[Campaign]{}, err
	}
	wire := listCampaignsWire{
		AdvertiserID: client.advertiserID, CampaignIDs: append([]uint64(nil), input.IDs...),
		StartDate: input.StartDate, EndDate: input.EndDate, Status: input.Status,
		UpdateStartDate: input.UpdateStartDate, UpdateEndDate: input.UpdateEndDate,
		Page: pageRequestWire{PageIndex: page, PageSize: pageSize},
	}
	raw, requestID, err := client.doJSON(ctx, operation, campaignListPath, wire, false, options...)
	if err != nil {
		return NumberPage[Campaign]{}, err
	}
	var data struct {
		Page      *pageResponseWire `json:"page"`
		Campaigns []Campaign        `json:"base_campaign_dtos"`
	}
	if err := decodeRequiredData(operation, raw, &data); err != nil {
		return NumberPage[Campaign]{}, err
	}
	if data.Page == nil || !validResponsePage(data.Page.PageIndex, page, data.Page.TotalCount, len(data.Campaigns), pageSize) {
		return NumberPage[Campaign]{}, platformContractError(operation, "Spotlight returned invalid campaign pagination")
	}
	for index := range data.Campaigns {
		if data.Campaigns[index].ID == 0 || !validOptionalText(data.Campaigns[index].Name, 4_096) {
			return NumberPage[Campaign]{}, platformContractError(operation, "Spotlight returned an invalid campaign")
		}
	}
	return NumberPage[Campaign]{
		Items: append([]Campaign(nil), data.Campaigns...), Page: page, PageSize: pageSize,
		TotalNumber: data.Page.TotalCount, HasMore: int64(page*pageSize) < data.Page.TotalCount,
		RequestID: requestID,
	}, nil
}

type campaignUpdateWire struct {
	CampaignID     uint64               `json:"campaign_id"`
	UpdateFields   []string             `json:"update_fields"`
	Name           *string              `json:"campaign_name,omitempty"`
	TimeType       *int                 `json:"time_type,omitempty"`
	StartDate      *Date                `json:"start_time,omitempty"`
	EndDate        *Date                `json:"expire_time,omitempty"`
	TimePeriodType *int                 `json:"time_period_type,omitempty"`
	TimePeriod     *TimePeriod          `json:"time_period,omitempty"`
	LimitDayBudget *int                 `json:"limit_day_budget,omitempty"`
	DayBudgetCents *int64               `json:"origin_campaign_day_budget,omitempty"`
	SmartSwitch    *int                 `json:"smart_switch,omitempty"`
	ExploreState   *int                 `json:"explore_state,omitempty"`
	ExploreConfig  *UpdateExploreConfig `json:"explore_config,omitempty"`
	SearchFlag     *int                 `json:"search_flag,omitempty"`
}

type cascadeUpdateItemWire struct {
	Campaign campaignUpdateWire `json:"campaign"`
}

type updateCampaignWire struct {
	AdvertiserID uint64                  `json:"advertiser_id"`
	ModifyType   int                     `json:"modify_type"`
	Items        []cascadeUpdateItemWire `json:"mod_cascade_info_list"`
}

func (client *Client) UpdateCampaign(ctx context.Context, campaignID uint64, input UpdateCampaignRequest, options ...socialhub.CallOption) (MutationResult, error) {
	const operation = "campaign_update"
	if campaignID == 0 {
		return MutationResult{}, invalidArgument(operation, "campaign ID must be positive")
	}
	if err := validateUpdateCampaign(input, client.clock.Now()); err != nil {
		return MutationResult{}, err
	}
	wire := updateCampaignWire{
		AdvertiserID: client.advertiserID, ModifyType: 1,
		Items: []cascadeUpdateItemWire{{Campaign: campaignUpdateWire{
			CampaignID: campaignID, UpdateFields: campaignUpdateFields(input),
			Name: input.Name, TimeType: input.TimeType,
			StartDate: input.StartDate, EndDate: input.EndDate,
			TimePeriodType: input.TimePeriodType, TimePeriod: input.TimePeriod,
			LimitDayBudget: input.LimitDayBudget, DayBudgetCents: input.DayBudgetCents,
			SmartSwitch: input.SmartSwitch, ExploreState: input.ExploreState,
			ExploreConfig: input.ExploreConfig, SearchFlag: input.SearchFlag,
		}}},
	}
	_, requestID, err := client.doJSON(ctx, operation, campaignUpdatePath, wire, true, options...)
	if err != nil {
		return MutationResult{RequestedIDs: []uint64{campaignID}}, err
	}
	return MutationResult{
		RequestedIDs: []uint64{campaignID}, AcknowledgedIDs: []uint64{campaignID}, RequestID: requestID,
	}, nil
}

func campaignUpdateFields(input UpdateCampaignRequest) []string {
	fields := make([]string, 0, 12)
	if input.Name != nil {
		fields = append(fields, "campaign_name")
	}
	if input.TimeType != nil {
		fields = append(fields, "time_type")
	}
	if input.StartDate != nil {
		fields = append(fields, "start_time")
	}
	if input.EndDate != nil {
		fields = append(fields, "expire_time")
	}
	if input.TimePeriodType != nil {
		fields = append(fields, "time_period_type")
	}
	if input.TimePeriod != nil {
		fields = append(fields, "time_period")
	}
	if input.LimitDayBudget != nil {
		fields = append(fields, "limit_day_budget")
	}
	if input.DayBudgetCents != nil {
		fields = append(fields, "origin_campaign_day_budget")
	}
	if input.SmartSwitch != nil {
		fields = append(fields, "smart_switch")
	}
	if input.ExploreState != nil {
		fields = append(fields, "explore_state")
	}
	if input.ExploreConfig != nil {
		fields = append(fields, "explore_config")
	}
	if input.SearchFlag != nil {
		fields = append(fields, "search_flag")
	}
	return fields
}

package xiaohongshumarketing

import (
	"context"

	"social-hub/pkg/socialhub"
)

const unitListPath = "/api/open/jg/unit/list"

type listUnitsWire struct {
	AdvertiserID    uint64   `json:"advertiser_id"`
	CampaignID      uint64   `json:"campaign_id,omitempty"`
	UnitIDs         []uint64 `json:"unit_ids,omitempty"`
	Status          int      `json:"status,omitempty"`
	Name            string   `json:"unit_name,omitempty"`
	StartDate       Date     `json:"start_date,omitempty"`
	EndDate         Date     `json:"end_date,omitempty"`
	Page            int      `json:"page"`
	PageSize        int      `json:"page_size"`
	UpdateStartDate Date     `json:"update_start_date,omitempty"`
	UpdateEndDate   Date     `json:"update_end_date,omitempty"`
}

func (client *Client) ListUnits(ctx context.Context, input ListUnitsRequest, options ...socialhub.CallOption) (NumberPage[Unit], error) {
	const operation = "unit_list"
	if err := validateListUnits(input); err != nil {
		return NumberPage[Unit]{}, err
	}
	page, pageSize, err := normalizePage(operation, input.Page, input.PageSize)
	if err != nil {
		return NumberPage[Unit]{}, err
	}
	wire := listUnitsWire{
		AdvertiserID: client.advertiserID, CampaignID: input.CampaignID,
		UnitIDs: append([]uint64(nil), input.IDs...), Status: input.Status, Name: input.Name,
		StartDate: input.StartDate, EndDate: input.EndDate, Page: page, PageSize: pageSize,
		UpdateStartDate: input.UpdateStartDate, UpdateEndDate: input.UpdateEndDate,
	}
	raw, requestID, err := client.doJSON(ctx, operation, unitListPath, wire, false, options...)
	if err != nil {
		return NumberPage[Unit]{}, err
	}
	var data struct {
		TotalCount int64  `json:"total_count"`
		Units      []Unit `json:"unit_infos"`
	}
	if err := decodeRequiredData(operation, raw, &data); err != nil {
		return NumberPage[Unit]{}, err
	}
	if !validResponsePage(page, page, data.TotalCount, len(data.Units), pageSize) {
		return NumberPage[Unit]{}, platformContractError(operation, "Spotlight returned invalid unit pagination")
	}
	for index := range data.Units {
		if data.Units[index].ID == 0 || !validOptionalText(data.Units[index].Name, 4_096) {
			return NumberPage[Unit]{}, platformContractError(operation, "Spotlight returned an invalid unit")
		}
		if input.CampaignID != 0 && data.Units[index].CampaignID != input.CampaignID {
			return NumberPage[Unit]{}, platformContractError(operation, "Spotlight returned a unit from another campaign")
		}
	}
	return NumberPage[Unit]{
		Items: append([]Unit(nil), data.Units...), Page: page, PageSize: pageSize,
		TotalNumber: data.TotalCount, HasMore: int64(page*pageSize) < data.TotalCount,
		RequestID: requestID,
	}, nil
}

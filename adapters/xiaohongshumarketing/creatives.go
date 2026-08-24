package xiaohongshumarketing

import (
	"context"

	"social-hub/pkg/socialhub"
)

const creativeSearchPath = "/api/open/jg/creativity/search"

type searchCreativesWire struct {
	AdvertiserID uint64          `json:"advertiser_id"`
	CampaignID   uint64          `json:"campaign_id,omitempty"`
	UnitID       uint64          `json:"unit_id,omitempty"`
	CreativeIDs  []uint64        `json:"creativity_ids,omitempty"`
	Status       int             `json:"status,omitempty"`
	StartDate    Date            `json:"start_time,omitempty"`
	EndDate      Date            `json:"end_time,omitempty"`
	NoteID       string          `json:"note_id,omitempty"`
	Page         pageRequestWire `json:"page"`
}

func (client *Client) SearchCreatives(ctx context.Context, input SearchCreativesRequest, options ...socialhub.CallOption) (NumberPage[Creative], error) {
	const operation = "creative_search"
	if err := validateSearchCreatives(input); err != nil {
		return NumberPage[Creative]{}, err
	}
	page, pageSize, err := normalizePage(operation, input.Page, input.PageSize)
	if err != nil {
		return NumberPage[Creative]{}, err
	}
	wire := searchCreativesWire{
		AdvertiserID: client.advertiserID, CampaignID: input.CampaignID, UnitID: input.UnitID,
		CreativeIDs: append([]uint64(nil), input.IDs...), Status: input.Status,
		StartDate: input.StartDate, EndDate: input.EndDate, NoteID: input.NoteID,
		Page: pageRequestWire{PageIndex: page, PageSize: pageSize},
	}
	raw, requestID, err := client.doJSON(ctx, operation, creativeSearchPath, wire, false, options...)
	if err != nil {
		return NumberPage[Creative]{}, err
	}
	var data struct {
		Page      *pageResponseWire `json:"page"`
		Creatives []Creative        `json:"creativity_dtos"`
	}
	if err := decodeRequiredData(operation, raw, &data); err != nil {
		return NumberPage[Creative]{}, err
	}
	if data.Page == nil || !validResponsePage(data.Page.PageIndex, page, data.Page.TotalCount, len(data.Creatives), pageSize) {
		return NumberPage[Creative]{}, platformContractError(operation, "Spotlight returned invalid creative pagination")
	}
	for index := range data.Creatives {
		creative := &data.Creatives[index]
		if creative.ID == 0 || !validOptionalText(creative.Name, 4_096) ||
			creative.AdvertiserID != 0 && creative.AdvertiserID != client.advertiserID {
			return NumberPage[Creative]{}, platformContractError(operation, "Spotlight returned an invalid creative")
		}
		creative.AdvertiserID = client.advertiserID
		if input.CampaignID != 0 && creative.CampaignID != input.CampaignID || input.UnitID != 0 && creative.UnitID != input.UnitID {
			return NumberPage[Creative]{}, platformContractError(operation, "Spotlight returned a creative outside the requested parent")
		}
	}
	return NumberPage[Creative]{
		Items: append([]Creative(nil), data.Creatives...), Page: page, PageSize: pageSize,
		TotalNumber: data.Page.TotalCount, HasMore: int64(page*pageSize) < data.Page.TotalCount,
		RequestID: requestID,
	}, nil
}

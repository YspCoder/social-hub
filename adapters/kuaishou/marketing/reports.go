package marketing

import (
	"context"
	"time"

	"social-hub/pkg/socialhub"
)

func (client *Client) GetReport(ctx context.Context, input ReportRequest, options ...socialhub.CallOption) (NumberPage[ReportRow], error) {
	const operation = "report_get"
	if !validReportLevel(input.Level) || !validGranularity(input.TemporalGranularity) ||
		!validDate(input.StartDate) || !validDate(input.EndDate) || input.StartDate > input.EndDate ||
		!validReportDimensions(input.ReportDimensions) || !validateIDs(input.CampaignIDs, 5000) ||
		!validateIDs(input.UnitIDs, 5000) || !validateIDs(input.CreativeIDs, 5000) {
		return NumberPage[ReportRow]{}, invalidArgument(operation, "report level, date range, granularity, dimensions, or IDs are invalid")
	}
	start, _ := time.Parse("2006-01-02", input.StartDate)
	end, _ := time.Parse("2006-01-02", input.EndDate)
	if end.Sub(start) >= 7*24*time.Hour {
		return NumberPage[ReportRow]{}, invalidArgument(operation, "real-time report range must not exceed seven days")
	}
	page, pageSize, err := validatePage(input.Page, input.PageSize, 2000)
	if err != nil {
		return NumberPage[ReportRow]{}, err
	}
	granularity := input.TemporalGranularity
	if granularity == "" {
		granularity = GranularityDaily
	}
	body := map[string]any{
		"advertiser_id": client.advertiserID, "start_date": input.StartDate, "end_date": input.EndDate,
		"temporal_granularity": granularity, "page": page, "page_size": pageSize,
	}
	if len(input.ReportDimensions) > 0 {
		body["report_dims"] = input.ReportDimensions
	}
	if input.CampaignType > 0 {
		body["campaign_type"] = input.CampaignType
	}
	if len(input.CampaignIDs) > 0 {
		body["campaign_ids"] = input.CampaignIDs
	}
	if len(input.UnitIDs) > 0 {
		body["unit_ids"] = input.UnitIDs
	}
	if len(input.CreativeIDs) > 0 {
		body["creative_ids"] = input.CreativeIDs
	}
	if len(input.ExtendInfo) > 0 {
		if input.Level != ReportLevelCreative {
			return NumberPage[ReportRow]{}, invalidArgument(operation, "extend_info is supported only for Creative reports")
		}
		for _, value := range input.ExtendInfo {
			if !validOpaque(value, 128) {
				return NumberPage[ReportRow]{}, invalidArgument(operation, "extend_info values are invalid")
			}
		}
		body["extend_info"] = input.ExtendInfo
	}
	paths := map[ReportLevel]string{
		ReportLevelAccount: "/v1/report/account_report", ReportLevelCampaign: "/v1/report/campaign_report",
		ReportLevelUnit: "/v1/report/unit_report", ReportLevelCreative: "/v1/report/creative_report",
	}
	var response apiEnvelope[struct {
		Details    []ReportRow `json:"details"`
		TotalCount int64       `json:"total_count"`
	}]
	header, err := client.postJSON(ctx, operation, paths[input.Level], body, &response, options...)
	if err != nil {
		return NumberPage[ReportRow]{}, err
	}
	data, err := requireEnvelope(operation, response, header)
	if err != nil {
		return NumberPage[ReportRow]{}, err
	}
	for index := range data.Details {
		actual := data.Details[index].AdvertiserID
		if actual == 0 {
			actual = data.Details[index].AccountID
		}
		if err := requireAdvertiser(operation, client.advertiserID, actual); err != nil {
			return NumberPage[ReportRow]{}, err
		}
		data.Details[index].AdvertiserID = client.advertiserID
	}
	return numberPage(data.Details, page, pageSize, data.TotalCount)
}

package appleads

import (
	"context"

	"social-hub/pkg/socialhub"
)

type reportingData struct {
	ReportingDataResponse reportingDataResponse `json:"reportingDataResponse"`
}

type reportingDataResponse struct {
	Rows        []ReportRow     `json:"row"`
	GrandTotals *grandTotalsRow `json:"grandTotals"`
}

type grandTotalsRow struct {
	Other bool          `json:"other"`
	Total *SpendMetrics `json:"total"`
}

func (client *Client) CampaignReport(ctx context.Context, input ReportingRequest, options ...socialhub.CallOption) (*Report, error) {
	return client.report(ctx, "report_campaigns", "/reports/campaigns", 0, input, false, options...)
}

func (client *Client) AdGroupReport(ctx context.Context, campaignID int64, input ReportingRequest, options ...socialhub.CallOption) (*Report, error) {
	const operation = "report_adgroups"
	if !validID(campaignID) {
		return nil, invalidArgument(operation, "campaign ID must be positive")
	}
	if _, err := client.GetCampaign(ctx, campaignID, options...); err != nil {
		return nil, err
	}
	return client.report(ctx, operation, "/reports/campaigns/"+formatID(campaignID)+"/adgroups", campaignID, input, false, options...)
}

func (client *Client) KeywordReport(ctx context.Context, campaignID int64, input ReportingRequest, options ...socialhub.CallOption) (*Report, error) {
	const operation = "report_keywords"
	if !validID(campaignID) {
		return nil, invalidArgument(operation, "campaign ID must be positive")
	}
	if _, err := client.GetCampaign(ctx, campaignID, options...); err != nil {
		return nil, err
	}
	return client.report(ctx, operation, "/reports/campaigns/"+formatID(campaignID)+"/keywords", campaignID, input, false, options...)
}

func (client *Client) AdReport(ctx context.Context, campaignID int64, input ReportingRequest, options ...socialhub.CallOption) (*Report, error) {
	const operation = "report_ads"
	if !validID(campaignID) {
		return nil, invalidArgument(operation, "campaign ID must be positive")
	}
	if _, err := client.GetCampaign(ctx, campaignID, options...); err != nil {
		return nil, err
	}
	return client.report(ctx, operation, "/reports/campaigns/"+formatID(campaignID)+"/ads", campaignID, input, true, options...)
}

func (client *Client) report(ctx context.Context, operation, path string, campaignID int64, input ReportingRequest, adLevel bool, options ...socialhub.CallOption) (*Report, error) {
	if !validReportRequest(input) || adLevel && (input.Granularity == GranularityHourly || len(input.Selector.OrderBy) != 1) {
		return nil, invalidArgument(operation, "report request is invalid for the selected report level")
	}
	if adLevel {
		for _, group := range input.GroupBy {
			if group != "countryOrRegion" {
				return nil, invalidArgument(operation, "Ad reports only support countryOrRegion grouping")
			}
		}
	}
	var response responseEnvelope[reportingData]
	if err := client.postJSON(ctx, operation, path, input, &response, options...); err != nil {
		return nil, err
	}
	if err := checkEnvelopeError(operation, response.Error); err != nil {
		return nil, err
	}
	rows := response.Data.ReportingDataResponse.Rows
	for index := range rows {
		metadata := rows[index].Metadata
		if metadata.OrgID != client.orgID || !validID(metadata.CampaignID) || campaignID != 0 && metadata.CampaignID != campaignID {
			return nil, platformContractError(operation, "report row has invalid organization or Campaign ownership")
		}
	}
	result := &Report{Rows: rows, Pagination: response.Pagination}
	if response.Data.ReportingDataResponse.GrandTotals != nil {
		result.GrandTotals = response.Data.ReportingDataResponse.GrandTotals.Total
	}
	return result, nil
}

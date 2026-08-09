package taboola

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

type reportEnvelope struct {
	Results     []ReportRow `json:"results"`
	RecordCount int         `json:"recordCount"`
	Timezone    string      `json:"timezone"`
	Metadata    struct {
		Total int `json:"total"`
		Count int `json:"count"`
	} `json:"metadata"`
}

func (client *Client) CampaignSummaryReport(ctx context.Context, input ReportRequest, options ...socialhub.CallOption) (ReportResult, error) {
	const operation = "report_campaign_summary"
	if !validDimension(input.Dimension) || !validReportWindow(input.StartDate, input.EndDate, false) ||
		!validCampaignIDs(input.CampaignIDs) || exclusiveFilterCount(input.Platform, input.Country, input.Site, input.PartnerName) > 1 {
		return ReportResult{}, invalidArgument(operation, "dimension, date range, campaign IDs, or mutually exclusive filters are invalid")
	}
	query := url.Values{"start_date": {input.StartDate}, "end_date": {input.EndDate}}
	setReportCommon(query, input.CampaignIDs, input.Platform, input.Country)
	if input.Site != "" {
		query.Set("site", input.Site)
	}
	if input.PartnerName != "" {
		query.Set("partner_name", input.PartnerName)
	}
	if input.IncludeMultiConversions {
		query.Set("include_multi_conversions", strconv.FormatBool(true))
	}
	path := client.accountPath("reports/campaign-summary/dimensions/" + url.PathEscape(input.Dimension))
	return client.report(ctx, operation, path, query, options...)
}

func (client *Client) RealtimeCampaignReport(ctx context.Context, input RealtimeReportRequest, options ...socialhub.CallOption) (ReportResult, error) {
	const operation = "report_realtime_campaign"
	if !validDimension(input.Dimension) || !validReportWindow(input.StartDate, input.EndDate, true) ||
		!validCampaignIDs(input.CampaignIDs) || input.SiteID != "" && !validPathID(input.SiteID, true) {
		return ReportResult{}, invalidArgument(operation, "dimension, date range, campaign IDs, or site ID are invalid")
	}
	query := url.Values{"start_date": {input.StartDate}, "end_date": {input.EndDate}}
	setReportCommon(query, input.CampaignIDs, input.Platform, input.Country)
	if input.SiteID != "" {
		query.Set("site_id", input.SiteID)
	}
	if input.FetchConfig {
		query.Set("fetch_config", strconv.FormatBool(true))
	}
	path := client.accountPath("reports/realtime-campaign-summary/dimensions/" + url.PathEscape(input.Dimension))
	return client.report(ctx, operation, path, query, options...)
}

func (client *Client) report(ctx context.Context, operation, path string, query url.Values, options ...socialhub.CallOption) (ReportResult, error) {
	var response reportEnvelope
	if err := client.getJSON(ctx, operation, path, query, &response, options...); err != nil {
		return ReportResult{}, err
	}
	if response.RecordCount < 0 || response.Metadata.Total < 0 || response.Metadata.Count < 0 ||
		response.Metadata.Count != len(response.Results) {
		return ReportResult{}, platformContractError(operation, "report metadata is inconsistent")
	}
	return ReportResult{
		Rows: response.Results, RecordCount: response.RecordCount,
		Total: response.Metadata.Total, Count: response.Metadata.Count, Timezone: response.Timezone,
	}, nil
}

func setReportCommon(query url.Values, campaignIDs []string, platform, country string) {
	if len(campaignIDs) > 0 {
		query.Set("campaign", strings.Join(campaignIDs, ","))
	}
	if platform != "" {
		query.Set("platform", platform)
	}
	if country != "" {
		query.Set("country", country)
	}
}

func validCampaignIDs(values []string) bool {
	for _, value := range values {
		if !validPathID(value, true) {
			return false
		}
	}
	return true
}

func exclusiveFilterCount(values ...string) int {
	count := 0
	for _, value := range values {
		if value != "" {
			count++
		}
	}
	return count
}

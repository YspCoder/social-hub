package kakaomoment

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) AdAccountReport(ctx context.Context, input ReportRequest, options ...socialhub.CallOption) (ReportResult, error) {
	const operation = "report_ad_account"
	if !validReportRequest(input) || input.Level != "" && input.Level != "AD_ACCOUNT" && input.Level != "CAMPAIGN" {
		return ReportResult{}, invalidArgument(operation, "report date, level, dimension, or metrics groups are invalid")
	}
	query := reportQuery(input)
	query.Set("adAccountId", formatID(client.adAccountID))
	if input.Level == "" {
		query.Set("level", "AD_ACCOUNT")
	}
	return client.report(ctx, operation, "adAccounts/report", query, options...)
}

func (client *Client) CampaignReport(ctx context.Context, input EntityReportRequest, options ...socialhub.CallOption) (ReportResult, error) {
	const operation = "report_campaign"
	if !validIDs(input.IDs, 5) || !validReportRequest(input.ReportRequest) ||
		input.Level != "" && input.Level != "CAMPAIGN" && input.Level != "AD_GROUP" {
		return ReportResult{}, invalidArgument(operation, "1-5 Campaign IDs and valid report criteria are required")
	}
	query := reportQuery(input.ReportRequest)
	query.Set("campaignId", joinIDs(input.IDs))
	if input.Level == "" {
		query.Set("level", "CAMPAIGN")
	}
	return client.report(ctx, operation, "campaigns/report", query, options...)
}

func (client *Client) AdGroupReport(ctx context.Context, input EntityReportRequest, options ...socialhub.CallOption) (ReportResult, error) {
	const operation = "report_adgroup"
	if !validIDs(input.IDs, 40) || !validReportRequest(input.ReportRequest) ||
		input.Level != "" && input.Level != "AD_GROUP" && input.Level != "CREATIVE" {
		return ReportResult{}, invalidArgument(operation, "1-40 Ad Group IDs and valid report criteria are required")
	}
	query := reportQuery(input.ReportRequest)
	query.Set("adGroupId", joinIDs(input.IDs))
	if input.Level == "" {
		query.Set("level", "AD_GROUP")
	}
	return client.report(ctx, operation, "adGroups/report", query, options...)
}

func (client *Client) CreativeReport(ctx context.Context, input EntityReportRequest, options ...socialhub.CallOption) (ReportResult, error) {
	const operation = "report_creative"
	if !validIDs(input.IDs, 100) || !validReportRequest(input.ReportRequest) ||
		input.Level != "" && input.Level != "CREATIVE" {
		return ReportResult{}, invalidArgument(operation, "1-100 Creative IDs and valid report criteria are required")
	}
	query := reportQuery(input.ReportRequest)
	query.Set("creativeId", joinIDs(input.IDs))
	return client.report(ctx, operation, "creatives/report", query, options...)
}

func reportQuery(input ReportRequest) url.Values {
	query := make(url.Values)
	if input.DatePreset != "" {
		query.Set("datePreset", string(input.DatePreset))
	} else if input.Start != "" {
		query.Set("start", input.Start)
		query.Set("end", input.End)
	}
	if input.TimeUnit != "" {
		query.Set("timeUnit", input.TimeUnit)
	}
	if input.Level != "" {
		query.Set("level", input.Level)
	}
	if input.Dimension != "" {
		query.Set("dimension", input.Dimension)
	}
	query.Set("metricsGroup", strings.Join(input.MetricsGroups, ","))
	return query
}

func (client *Client) report(ctx context.Context, operation, path string, query url.Values, options ...socialhub.CallOption) (ReportResult, error) {
	var result ReportResult
	requestID, err := client.doJSON(
		ctx, operation, []string{ScopeManagement}, http.MethodGet,
		path, query, nil, &result, false, options...,
	)
	if err != nil {
		return ReportResult{}, err
	}
	if result.Code != http.StatusOK || result.Data == nil || !validOptionalText(result.Message, 1024) {
		return ReportResult{}, platformContractError(operation, "Kakao Moment returned an invalid report envelope")
	}
	for _, row := range result.Data {
		empty := row.Start == "" && row.End == "" && row.Dimensions == nil && row.Metrics == nil
		if !empty && (len(row.Start) > 64 || len(row.End) > 64 || row.Dimensions == nil || row.Metrics == nil) {
			return ReportResult{}, platformContractError(operation, "Kakao Moment returned an invalid report row")
		}
	}
	result.RequestID = requestID
	return result, nil
}

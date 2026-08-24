package adsense

import (
	"context"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

type ReportingWorkflow interface {
	GenerateReport(context.Context, GenerateReportRequest, ...socialhub.CallOption) (*ReportResult, error)
	GetSavedReport(context.Context, string, ...socialhub.CallOption) (*SavedReport, error)
	ListSavedReports(context.Context, ListRequest, ...socialhub.CallOption) (Page[SavedReport], error)
	GenerateSavedReport(context.Context, string, GenerateSavedReportRequest, ...socialhub.CallOption) (*ReportResult, error)
}

func (client *Client) GenerateReport(ctx context.Context, input GenerateReportRequest, options ...socialhub.CallOption) (*ReportResult, error) {
	const operation = "report_generate"
	if !validGenerateReport(input) {
		return nil, invalidArgument(operation, "report fields, date range, locale, ordering, filters, or limit are invalid")
	}
	query := reportQuery(input.DateRange, input.StartDate, input.EndDate, input.ReportingTimeZone, input.CurrencyCode, input.LanguageCode)
	for _, dimension := range input.Dimensions {
		query.Add("dimensions", string(dimension))
	}
	for _, metric := range input.Metrics {
		query.Add("metrics", string(metric))
	}
	for _, filter := range input.Filters {
		query.Add("filters", filter)
	}
	for _, order := range input.OrderBy {
		query.Add("orderBy", order)
	}
	limit := input.Limit
	if limit == 0 {
		limit = 10_000
	}
	query.Set("limit", strconv.FormatInt(int64(limit), 10))
	var output ReportResult
	if err := client.getJSON(ctx, operation, "/v2/"+client.accountName()+"/reports:generate", query, &output, options...); err != nil {
		return nil, err
	}
	if !validReportShape(&output, expectedHeaders(input), len(input.Dimensions)) || len(output.Rows) > int(limit) {
		return nil, platformContractError(operation, "AdSense returned a malformed or reordered report")
	}
	return &output, nil
}

func (client *Client) GetSavedReport(ctx context.Context, reportID string, options ...socialhub.CallOption) (*SavedReport, error) {
	const operation = "saved_report_get"
	name, err := client.resourceName(operation, client.accountName(), "reports", reportID)
	if err != nil {
		return nil, err
	}
	var output SavedReport
	if err := client.getJSON(ctx, operation, "/v2/"+name+"/saved", nil, &output, options...); err != nil {
		return nil, err
	}
	if output.Name != name {
		return nil, ownershipError(operation, "saved report")
	}
	return &output, nil
}

func (client *Client) ListSavedReports(ctx context.Context, input ListRequest, options ...socialhub.CallOption) (Page[SavedReport], error) {
	const operation = "saved_reports_list"
	query, err := listQuery(operation, input)
	if err != nil {
		return Page[SavedReport]{}, err
	}
	var output struct {
		SavedReports  []SavedReport `json:"savedReports"`
		NextPageToken string        `json:"nextPageToken"`
	}
	if err := client.getJSON(ctx, operation, "/v2/"+client.accountName()+"/reports/saved", query, &output, options...); err != nil {
		return Page[SavedReport]{}, err
	}
	for _, item := range output.SavedReports {
		if !client.ownsResource(item.Name, client.accountName(), "reports") {
			return Page[SavedReport]{}, ownershipError(operation, "saved report")
		}
	}
	return Page[SavedReport]{Items: output.SavedReports, NextPageToken: output.NextPageToken}, nil
}

func (client *Client) GenerateSavedReport(ctx context.Context, reportID string, input GenerateSavedReportRequest, options ...socialhub.CallOption) (*ReportResult, error) {
	const operation = "saved_report_generate"
	name, err := client.resourceName(operation, client.accountName(), "reports", reportID)
	if err != nil {
		return nil, err
	}
	if !validGenerateSavedReport(input) {
		return nil, invalidArgument(operation, "saved report date range, locale, currency, or reporting time zone is invalid")
	}
	query := reportQuery(input.DateRange, input.StartDate, input.EndDate, input.ReportingTimeZone, input.CurrencyCode, input.LanguageCode)
	var output ReportResult
	if err := client.getJSON(ctx, operation, "/v2/"+name+"/saved:generate", query, &output, options...); err != nil {
		return nil, err
	}
	if !validReportShape(&output, nil, -1) || len(output.Rows) > int(DefaultQuotaPolicy().MaximumJSONReportRows) {
		return nil, platformContractError(operation, "AdSense returned a malformed saved report")
	}
	return &output, nil
}

func reportQuery(dateRange ReportDateRange, start, end Date, zone ReportingTimeZone, currency, language string) url.Values {
	query := make(url.Values)
	if dateRange != "" {
		query.Set("dateRange", string(dateRange))
	}
	if !zeroDate(start) {
		query.Set("startDate.year", strconv.FormatInt(int64(start.Year), 10))
		query.Set("startDate.month", strconv.FormatInt(int64(start.Month), 10))
		query.Set("startDate.day", strconv.FormatInt(int64(start.Day), 10))
		query.Set("endDate.year", strconv.FormatInt(int64(end.Year), 10))
		query.Set("endDate.month", strconv.FormatInt(int64(end.Month), 10))
		query.Set("endDate.day", strconv.FormatInt(int64(end.Day), 10))
	}
	if zone != "" {
		query.Set("reportingTimeZone", string(zone))
	}
	if currency != "" {
		query.Set("currencyCode", currency)
	}
	if language != "" {
		query.Set("languageCode", language)
	}
	return query
}

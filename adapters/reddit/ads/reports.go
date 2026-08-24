package ads

import (
	"context"
	"net/url"
	"time"

	"social-hub/pkg/socialhub"
)

type reportData struct {
	StartsAt   string            `json:"starts_at"`
	EndsAt     string            `json:"ends_at"`
	Fields     []ReportField     `json:"fields"`
	Breakdowns []ReportBreakdown `json:"breakdowns,omitempty"`
	TimeZoneID string            `json:"time_zone_id,omitempty"`
	Filter     string            `json:"filter,omitempty"`
}

type reportResponse struct {
	Data struct {
		Metrics          []ReportMetric `json:"metrics"`
		MetricsUpdatedAt string         `json:"metrics_updated_at"`
	} `json:"data"`
	Pagination pagination `json:"pagination"`
}

func (client *Client) GetReport(ctx context.Context, input ReportRequest, options ...socialhub.CallOption) (ReportResult, error) {
	const operation = "report_get"
	if !client.validReportRequest(input) {
		return ReportResult{}, invalidArgument(operation, "time range, fields, breakdowns, filter, or pagination is invalid")
	}
	path := client.accountPath("reports")
	query := make(url.Values)
	if input.Cursor != "" {
		query.Set("page.token", input.Cursor)
	}
	if input.PageSize > 0 {
		query.Set("page.size", formatInt(input.PageSize))
	}
	payload := struct {
		Data reportData `json:"data"`
	}{Data: reportData{
		StartsAt: input.StartsAt.UTC().Format("2006-01-02T15:00:00Z"),
		EndsAt:   input.EndsAt.UTC().Format("2006-01-02T15:00:00Z"),
		Fields:   append([]ReportField(nil), input.Fields...), Breakdowns: append([]ReportBreakdown(nil), input.Breakdowns...),
		TimeZoneID: input.TimeZoneID, Filter: input.Filter,
	}}
	var response reportResponse
	if _, err := client.reportJSON(ctx, operation, path, query, payload, &response, options...); err != nil {
		return ReportResult{}, err
	}
	for _, metric := range response.Data.Metrics {
		if metric == nil {
			return ReportResult{}, platformContractError(operation, "Reddit returned a null report metric")
		}
	}
	cursor, err := client.pageCursor(operation, path, response.Pagination.NextURL)
	if err != nil {
		return ReportResult{}, err
	}
	return ReportResult{
		Metrics: response.Data.Metrics, MetricsUpdatedAt: response.Data.MetricsUpdatedAt,
		NextCursor: cursor, HasMore: cursor != nil,
		PageIndex: response.Pagination.PageIndex, TotalCount: response.Pagination.TotalCount,
	}, nil
}

func (client *Client) validReportRequest(input ReportRequest) bool {
	if !hourAligned(input.StartsAt) || !hourAligned(input.EndsAt) || !input.EndsAt.After(input.StartsAt) ||
		len(input.Fields) == 0 || len(input.Fields) > 256 || len(input.Breakdowns) > 4 ||
		input.TimeZoneID != "" && !validOpaque(input.TimeZoneID, 128) || input.Filter != "" && !validOpaque(input.Filter, 8192) ||
		!validList(ListRequest{Cursor: input.Cursor, PageSize: input.PageSize}) {
		return false
	}
	fields := make(map[ReportField]struct{}, len(input.Fields))
	for _, field := range input.Fields {
		if !validReportField(field) {
			return false
		}
		if _, exists := fields[field]; exists {
			return false
		}
		fields[field] = struct{}{}
	}
	breakdowns := make(map[ReportBreakdown]struct{}, len(input.Breakdowns))
	for _, breakdown := range input.Breakdowns {
		if !validReportBreakdown(breakdown) {
			return false
		}
		if _, exists := breakdowns[breakdown]; exists {
			return false
		}
		breakdowns[breakdown] = struct{}{}
	}
	if len(input.Breakdowns) == 4 && !(containsBreakdown(input.Breakdowns, BreakdownCountry) && containsBreakdown(input.Breakdowns, BreakdownRegion)) {
		return false
	}
	// Reddit announced this enforcement for 2026-10-30; avoid applying the
	// future rule early while keeping the adapter correct after the cutoff.
	if !client.clock.Now().Before(time.Date(2026, time.October, 30, 0, 0, 0, 0, time.UTC)) &&
		containsBreakdown(input.Breakdowns, BreakdownHour) && input.EndsAt.Sub(input.StartsAt) > 7*24*time.Hour {
		return false
	}
	return true
}

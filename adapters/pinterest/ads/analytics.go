package ads

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) GetAccountAnalytics(ctx context.Context, input AnalyticsRequest, options ...socialhub.CallOption) ([]AnalyticsRow, error) {
	const operation = "analytics_get"
	span, valid := dateSpan(input.StartDate, input.EndDate)
	if !valid || span > 90 || input.Granularity == GranularityHour && span > 8 || !validGranularity(input.Granularity) || !validColumns(input.Columns) ||
		!validOptionalWindow(input.ClickWindowDays) || !validOptionalWindow(input.EngagementWindowDays) || !validOptionalWindow(input.ViewWindowDays) ||
		input.ConversionReportTime != "" && input.ConversionReportTime != ReportTimeAdAction && input.ConversionReportTime != ReportTimeConversion ||
		input.ReportingTimezone != "" && input.ReportingTimezone != TimezonePinterest && input.ReportingTimezone != TimezoneAdAccount {
		return nil, invalidArgument(operation, "Analytics dates, granularity, columns, attribution, or timezone are invalid")
	}
	query := url.Values{
		"start_date": {input.StartDate}, "end_date": {input.EndDate},
		"columns": {strings.Join(input.Columns, ",")}, "granularity": {string(input.Granularity)},
	}
	setOptionalInt(query, "click_window_days", input.ClickWindowDays)
	setOptionalInt(query, "engagement_window_days", input.EngagementWindowDays)
	setOptionalInt(query, "view_window_days", input.ViewWindowDays)
	if input.ConversionReportTime != "" {
		query.Set("conversion_report_time", string(input.ConversionReportTime))
	}
	if input.ReportingTimezone != "" {
		query.Set("reporting_timezone", string(input.ReportingTimezone))
	}
	var response []AnalyticsRow
	if _, err := client.getJSON(ctx, operation, client.resourcePath("analytics"), query, &response, options...); err != nil {
		return nil, err
	}
	for _, row := range response {
		if row.AdAccountID != client.adAccountID || row.Date != "" && !validDate(row.Date) {
			return nil, platformContractError(operation, "Pinterest returned Analytics for another Ad Account or an invalid date")
		}
	}
	return response, nil
}

func validGranularity(value Granularity) bool {
	return value == GranularityTotal || value == GranularityDay || value == GranularityHour || value == GranularityWeek || value == GranularityMonth
}

func validOptionalWindow(value *int) bool { return value == nil || validAttributionWindow(*value) }

func setOptionalInt(query url.Values, key string, value *int) {
	if value != nil {
		query.Set(key, strconv.Itoa(*value))
	}
}

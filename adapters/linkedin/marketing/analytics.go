package marketing

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

type analyticsResponse struct {
	Elements []AnalyticsRow `json:"elements"`
}

func (client *Client) GetAdAnalytics(ctx context.Context, input AnalyticsRequest, options ...socialhub.CallOption) ([]AnalyticsRow, error) {
	const operation = "analytics_get"
	span, valid := dateSpan(input.StartDate, input.EndDate)
	if !valid || span > 366 || !validPivot(input.Pivot) || !validGranularity(input.Granularity) || !validFields(input.Fields) {
		return nil, invalidArgument(operation, "date range, pivot, granularity, or metric fields are invalid")
	}
	start, _ := time.Parse("2006-01-02", input.StartDate)
	end, _ := time.Parse("2006-01-02", input.EndDate)
	rawQuery := strings.Join([]string{
		"q=analytics",
		"accounts=List(" + url.QueryEscape(client.accountURN()) + ")",
		"dateRange=" + analyticsDateRange(start, end),
		"pivot=" + string(input.Pivot),
		"timeGranularity=" + string(input.Granularity),
		"fields=" + strings.Join(input.Fields, ","),
	}, "&")
	var response analyticsResponse
	if _, err := client.reportJSON(ctx, operation, "/adAnalytics", rawQuery, &response, options...); err != nil {
		return nil, err
	}
	if len(response.Elements) > 15000 {
		return nil, platformContractError(operation, "LinkedIn returned more than 15,000 Analytics rows")
	}
	for index := range response.Elements {
		if err := client.validateAnalyticsRow(operation, input.Pivot, &response.Elements[index]); err != nil {
			return nil, err
		}
	}
	return response.Elements, nil
}

func analyticsDateRange(start, end time.Time) string {
	return fmt.Sprintf("(start:(year:%d,month:%d,day:%d),end:(year:%d,month:%d,day:%d))",
		start.Year(), start.Month(), start.Day(), end.Year(), end.Month(), end.Day())
}

func (client *Client) validateAnalyticsRow(operation string, pivot AnalyticsPivot, row *AnalyticsRow) error {
	if row.DateRange.Start.Year != 0 && !validAnalyticsDate(row.DateRange.Start) ||
		row.DateRange.End.Year != 0 && !validAnalyticsDate(row.DateRange.End) {
		return platformContractError(operation, "LinkedIn returned an invalid Analytics date range")
	}
	for _, value := range row.PivotValues {
		valid := false
		switch pivot {
		case PivotAccount:
			valid = value == client.accountURN()
		case PivotCampaignGroup:
			valid = validNumericURN(value, campaignGroupURNPrefix)
		case PivotCampaign:
			valid = validNumericURN(value, campaignURNPrefix)
		case PivotCreative:
			valid = validNumericURN(value, creativeURNPrefix)
		}
		if !valid {
			return platformContractError(operation, "LinkedIn returned an invalid Analytics pivot value")
		}
	}
	return nil
}

func validAnalyticsDate(value AnalyticsDate) bool {
	if value.Year < 2000 || value.Year > 9999 || value.Month < 1 || value.Month > 12 || value.Day < 1 || value.Day > 31 {
		return false
	}
	parsed := time.Date(value.Year, time.Month(value.Month), value.Day, 0, 0, 0, 0, time.UTC)
	return parsed.Year() == value.Year && int(parsed.Month()) == value.Month && parsed.Day() == value.Day
}

package criteo

import (
	"context"
	"strings"

	"social-hub/pkg/socialhub"
)

const statisticsReportPath = "/2026-01/statistics/report"

func (client *Client) Report(ctx context.Context, input StatisticsReportRequest, options ...socialhub.CallOption) (StatisticsReport, error) {
	const operation = "statistics_report"
	if !validStatisticsReport(input) {
		return StatisticsReport{}, invalidArgument(operation, "statistics report fields are invalid")
	}
	timezone := input.Timezone
	if timezone == "" {
		timezone = "UTC"
	}
	request := statisticsReportWire{
		AdvertiserIDs: client.advertiserID, Currency: input.Currency,
		Dimensions: append([]Dimension(nil), input.Dimensions...), Metrics: append([]Metric(nil), input.Metrics...),
		StartDate: input.StartDate, EndDate: input.EndDate, Format: "json", Timezone: timezone,
		AdSetIDs: append([]string(nil), input.AdSetIDs...), AdSetNames: append([]string(nil), input.AdSetNames...),
		AdSetStatus: append([]AdSetReportStatus(nil), input.AdSetStatus...),
	}
	data, contentType, err := client.postRawJSON(ctx, operation, statisticsReportPath, request, options...)
	if err != nil {
		return StatisticsReport{}, err
	}
	return StatisticsReport{ContentType: contentType, Data: data}, nil
}

func validStatisticsReport(input StatisticsReportRequest) bool {
	start, startOK := parseReportDate(input.StartDate)
	end, endOK := parseReportDate(input.EndDate)
	if !currencyPattern.MatchString(input.Currency) || !startOK || !endOK || end.Before(start) ||
		len(input.Dimensions) == 0 || !validStrings(input.Dimensions, 64) || len(input.Metrics) == 0 ||
		!validStrings(input.Metrics, 512) || !validIDs(input.AdSetIDs, 1000) || !validStrings(input.AdSetNames, 1000) ||
		!validStrings(input.AdSetStatus, 3) || !validOptionalText(input.Timezone, 128) {
		return false
	}
	for _, status := range input.AdSetStatus {
		if status != ReportAdSetActive && status != ReportAdSetNotRunning && status != ReportAdSetDead {
			return false
		}
	}
	return !strings.ContainsAny(input.Timezone, "\x00\r\n")
}

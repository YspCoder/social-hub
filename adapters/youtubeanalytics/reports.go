package youtubeanalytics

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

type ReportingWorkflow interface {
	QueryReport(context.Context, ReportQuery, ...socialhub.CallOption) (*Report, error)
}

func (client *Client) QueryReport(ctx context.Context, input ReportQuery, options ...socialhub.CallOption) (*Report, error) {
	const operation = "report_query"
	if !validReportQuery(input, client.binding.ContentOwnerID != "") {
		return nil, invalidArgument(operation, "report dates, metrics, dimensions, filters, sorting, paging, currency, historical-data setting, or traffic-source cost are invalid")
	}
	monetary := requiresMonetaryScope(input)
	if err := client.requireReportScope(operation, monetary); err != nil {
		return nil, err
	}
	query := url.Values{
		"ids":       {client.reportIDs()},
		"startDate": {input.StartDate},
		"endDate":   {input.EndDate},
		"metrics":   {joinMetrics(input.Metrics)},
	}
	if len(input.Dimensions) > 0 {
		query.Set("dimensions", joinDimensions(input.Dimensions))
	}
	if len(input.Filters) > 0 {
		encoded := make([]string, len(input.Filters))
		for index, filter := range input.Filters {
			encoded[index] = string(filter.Dimension) + "==" + strings.Join(filter.Values, ",")
		}
		query.Set("filters", strings.Join(encoded, ";"))
	}
	if len(input.Sort) > 0 {
		encoded := make([]string, len(input.Sort))
		for index, sort := range input.Sort {
			prefix := ""
			if sort.Descending {
				prefix = "-"
			}
			encoded[index] = prefix + sort.Name
		}
		query.Set("sort", strings.Join(encoded, ","))
	}
	if input.Currency != "" {
		query.Set("currency", input.Currency)
	}
	if input.MaxResults > 0 {
		query.Set("maxResults", strconv.FormatInt(int64(input.MaxResults), 10))
	}
	if input.StartIndex > 0 {
		query.Set("startIndex", strconv.FormatInt(int64(input.StartIndex), 10))
	}
	if input.IncludeHistoricalChannelData {
		query.Set("includeHistoricalChannelData", "true")
	}
	var output Report
	if err := client.getJSON(ctx, operation, "/v2/reports", query, &output, options...); err != nil {
		return nil, err
	}
	if !validReportResponse(&output, input) {
		return nil, platformContractError(operation, "YouTube Analytics returned malformed, reordered, or type-incompatible report data")
	}
	return &output, nil
}

func joinMetrics(values []Metric) string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = string(value)
	}
	return strings.Join(result, ",")
}

func joinDimensions(values []Dimension) string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = string(value)
	}
	return strings.Join(result, ",")
}

var _ ReportingWorkflow = (*Client)(nil)

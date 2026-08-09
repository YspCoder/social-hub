package analyticsdata

import (
	"context"

	"social-hub/pkg/socialhub"
)

type RealtimeWorkflow interface {
	RunRealtimeReport(context.Context, RunRealtimeReportRequest, ...socialhub.CallOption) (*ReportResponse, error)
}

func (client *Client) RunRealtimeReport(ctx context.Context, input RunRealtimeReportRequest, options ...socialhub.CallOption) (*ReportResponse, error) {
	const operation = "realtime_report_run"
	if !validRealtimeReport(input) {
		return nil, invalidArgument(operation, "realtime dimensions, metrics, minute ranges, filters, ordering, or limit are invalid")
	}
	var output ReportResponse
	if err := client.postJSON(ctx, operation, "/v1beta/"+client.propertyName()+":runRealtimeReport", input, &output, options...); err != nil {
		return nil, err
	}
	if !validReportResponse(
		&output,
		requestedDimensionHeaders(input.Dimensions, len(input.MinuteRanges), 0),
		visibleMetricHeaders(input.Metrics),
		effectiveLimit(input.Limit),
		"analyticsData#runRealtimeReport",
	) {
		return nil, platformContractError(operation, "Google Analytics returned a malformed or reordered realtime report")
	}
	return &output, nil
}

var _ RealtimeWorkflow = (*Client)(nil)

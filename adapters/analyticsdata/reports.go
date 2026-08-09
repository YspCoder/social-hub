package analyticsdata

import (
	"context"

	"social-hub/pkg/socialhub"
)

type ReportingWorkflow interface {
	RunReport(context.Context, RunReportRequest, ...socialhub.CallOption) (*ReportResponse, error)
	BatchRunReports(context.Context, BatchRunReportsRequest, ...socialhub.CallOption) (*BatchReportResponse, error)
}

func (client *Client) RunReport(ctx context.Context, input RunReportRequest, options ...socialhub.CallOption) (*ReportResponse, error) {
	const operation = "report_run"
	if !validRunReport(input) {
		return nil, invalidArgument(operation, "report dimensions, metrics, date ranges, filters, ordering, comparisons, cohort, offset, or limit are invalid")
	}
	var output ReportResponse
	if err := client.postJSON(ctx, operation, "/v1beta/"+client.propertyName()+":runReport", input, &output, options...); err != nil {
		return nil, err
	}
	if !validCoreReportResponse(&output, input) {
		return nil, platformContractError(operation, "Google Analytics returned a malformed or reordered report")
	}
	return &output, nil
}

func (client *Client) BatchRunReports(ctx context.Context, input BatchRunReportsRequest, options ...socialhub.CallOption) (*BatchReportResponse, error) {
	const operation = "reports_batch_run"
	if !validBatchReports(input) {
		return nil, invalidArgument(operation, "batch must contain one to five valid report requests")
	}
	var output BatchReportResponse
	if err := client.postJSON(ctx, operation, "/v1beta/"+client.propertyName()+":batchRunReports", input, &output, options...); err != nil {
		return nil, err
	}
	if output.Kind != "analyticsData#batchRunReports" || len(output.Reports) != len(input.Requests) {
		return nil, platformContractError(operation, "Google Analytics returned a malformed report batch")
	}
	for index := range output.Reports {
		if !validCoreReportResponse(&output.Reports[index], input.Requests[index]) {
			return nil, platformContractError(operation, "Google Analytics returned a malformed or reordered report batch item")
		}
	}
	return &output, nil
}

func validCoreReportResponse(output *ReportResponse, input RunReportRequest) bool {
	return validReportResponse(
		output,
		requestedDimensionHeaders(input.Dimensions, len(input.DateRanges), len(input.Comparisons)),
		visibleMetricHeaders(input.Metrics),
		effectiveLimit(input.Limit),
		"analyticsData#runReport",
	)
}

var _ ReportingWorkflow = (*Client)(nil)

package analyticsdata

import (
	"context"

	"social-hub/pkg/socialhub"
)

type PivotWorkflow interface {
	RunPivotReport(context.Context, RunPivotReportRequest, ...socialhub.CallOption) (*PivotReportResponse, error)
	BatchRunPivotReports(context.Context, BatchRunPivotReportsRequest, ...socialhub.CallOption) (*BatchPivotReportResponse, error)
}

func (client *Client) RunPivotReport(ctx context.Context, input RunPivotReportRequest, options ...socialhub.CallOption) (*PivotReportResponse, error) {
	const operation = "pivot_report_run"
	if !validPivotReport(input) {
		return nil, invalidArgument(operation, "pivot dimensions, metrics, date ranges, filters, ordering, comparisons, cohort, or limits are invalid")
	}
	var output PivotReportResponse
	if err := client.postJSON(ctx, operation, "/v1beta/"+client.propertyName()+":runPivotReport", input, &output, options...); err != nil {
		return nil, err
	}
	if !validPivotResponse(&output, input) {
		return nil, platformContractError(operation, "Google Analytics returned a malformed or reordered pivot report")
	}
	return &output, nil
}

func (client *Client) BatchRunPivotReports(ctx context.Context, input BatchRunPivotReportsRequest, options ...socialhub.CallOption) (*BatchPivotReportResponse, error) {
	const operation = "pivot_reports_batch_run"
	if !validBatchPivotReports(input) {
		return nil, invalidArgument(operation, "batch must contain one to five valid pivot report requests")
	}
	var output BatchPivotReportResponse
	if err := client.postJSON(ctx, operation, "/v1beta/"+client.propertyName()+":batchRunPivotReports", input, &output, options...); err != nil {
		return nil, err
	}
	if output.Kind != "analyticsData#batchRunPivotReports" || len(output.PivotReports) != len(input.Requests) {
		return nil, platformContractError(operation, "Google Analytics returned a malformed pivot report batch")
	}
	for index := range output.PivotReports {
		if !validPivotResponse(&output.PivotReports[index], input.Requests[index]) {
			return nil, platformContractError(operation, "Google Analytics returned a malformed or reordered pivot report batch item")
		}
	}
	return &output, nil
}

var _ PivotWorkflow = (*Client)(nil)

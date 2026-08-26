package moloco

import (
	"context"
	"net/url"

	"social-hub/pkg/socialhub"
)

const dateLayout = "2006-01-02"

func (client *Client) ListReports(
	ctx context.Context,
	options ...socialhub.CallOption,
) (ListReportsResponse, error) {
	const operation = "list_reports"
	query := url.Values{"ad_account_id": {client.adAccountID}}
	var output ListReportsResponse
	metadata, err := client.getJSON(ctx, operation, "/cm/v1/reports", query, &output, options...)
	if err != nil {
		return ListReportsResponse{}, err
	}
	for _, report := range output.Reports {
		if !validReport(report, client.adAccountID) {
			return ListReportsResponse{}, platformContractError(operation, "Moloco returned an invalid or cross-account report")
		}
	}
	output.Meta = metadata
	return output, nil
}

func (client *Client) GetReport(
	ctx context.Context,
	reportID string,
	options ...socialhub.CallOption,
) (Report, error) {
	const operation = "get_report"
	if !validIdentifier(reportID) {
		return Report{}, invalidArgument(operation, "report ID is invalid")
	}
	var output struct {
		Report Report `json:"report"`
	}
	metadata, err := client.getJSON(ctx, operation, "/cm/v1/reports/"+reportID, nil, &output, options...)
	if err != nil {
		return Report{}, err
	}
	if output.Report.ID != reportID || !validReport(output.Report, client.adAccountID) {
		return Report{}, platformContractError(operation, "Moloco returned an invalid, mismatched, or cross-account report")
	}
	output.Report.Meta = metadata
	return output.Report, nil
}

func (client *Client) GetReportStatus(
	ctx context.Context,
	reportID string,
	options ...socialhub.CallOption,
) (ReportStatusResponse, error) {
	const operation = "get_report_status"
	if !validIdentifier(reportID) {
		return ReportStatusResponse{}, invalidArgument(operation, "report ID is invalid")
	}
	var output ReportStatusResponse
	metadata, err := client.getJSON(
		ctx, operation, "/cm/v1/reports/"+reportID+"/status", nil, &output, options...,
	)
	if err != nil {
		return ReportStatusResponse{}, err
	}
	if !validIdentifier(string(output.Status)) || output.ID != "" && output.ID != reportID {
		return ReportStatusResponse{}, platformContractError(operation, "Moloco returned an invalid or mismatched report status")
	}
	if output.LocationJSON != "" && !validDownloadURL(output.LocationJSON) ||
		output.LocationCSV != "" && !validDownloadURL(output.LocationCSV) {
		return ReportStatusResponse{}, platformContractError(operation, "Moloco returned an invalid report download URL")
	}
	if output.Status == ReportStatusReady && output.LocationJSON == "" && output.LocationCSV == "" {
		return ReportStatusResponse{}, platformContractError(operation, "Moloco marked the report ready without a download URL")
	}
	output.Meta = metadata
	return output, nil
}

var _ ReportWorkflow = (*Client)(nil)

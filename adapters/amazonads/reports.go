package amazonads

import (
	"context"
	"net/http"

	"social-hub/pkg/socialhub"
)

func (client *Client) CreateReport(ctx context.Context, input CreateReportRequest, options ...socialhub.CallOption) (*Report, error) {
	const operation = "report_create"
	if !validText(input.Name, 256) || !validDate(input.StartDate) || !validDate(input.EndDate) || input.EndDate < input.StartDate ||
		!validIdentifiers(input.GroupBy, 1, 10) || !validIdentifiers(input.Columns, 1, 200) || !validIdentifier(input.ReportTypeID) ||
		input.TimeUnit != ReportTimeDaily && input.TimeUnit != ReportTimeSummary || input.Format != ReportFormatGZIPJSON {
		return nil, invalidArgument(operation, "report name, dates, grouping, columns, type, time unit, or format is invalid")
	}
	body := struct {
		Name          string              `json:"name"`
		StartDate     string              `json:"startDate"`
		EndDate       string              `json:"endDate"`
		Configuration ReportConfiguration `json:"configuration"`
	}{
		Name: input.Name, StartDate: input.StartDate, EndDate: input.EndDate,
		Configuration: ReportConfiguration{
			AdProduct: "SPONSORED_PRODUCTS", GroupBy: input.GroupBy, Columns: input.Columns,
			ReportTypeID: input.ReportTypeID, TimeUnit: input.TimeUnit, Format: input.Format,
		},
	}
	var response Report
	if _, err := client.vendorJSON(ctx, operation, http.MethodPost, "/reporting/reports", reportCreateMediaType, body, &response, false, options...); err != nil {
		return nil, err
	}
	if !validPathID(response.ID) {
		return nil, platformContractError(operation, "Amazon Ads returned an invalid report ID")
	}
	return &response, nil
}

func (client *Client) GetReport(ctx context.Context, id string, options ...socialhub.CallOption) (*Report, error) {
	const operation = "report_get"
	if !validPathID(id) {
		return nil, invalidArgument(operation, "report ID is invalid")
	}
	var response Report
	if _, err := client.getJSON(ctx, operation, "/reporting/reports/"+id, "application/json", &response, options...); err != nil {
		return nil, err
	}
	if response.ID != id {
		return nil, platformContractError(operation, "Amazon Ads returned a missing or mismatched report ID")
	}
	return &response, nil
}

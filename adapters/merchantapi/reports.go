package merchantapi

import (
	"context"
	"net/http"

	"social-hub/pkg/socialhub"
)

func (client *Client) SearchReports(ctx context.Context, input ReportRequest, options ...socialhub.CallOption) (ReportPage, error) {
	const operation = "reports.search"
	if !validReportRequest(input) {
		return ReportPage{}, invalidArgument(operation, "query, page size, or page token is invalid")
	}
	body := struct {
		Query     string `json:"query"`
		PageSize  int    `json:"pageSize,omitempty"`
		PageToken string `json:"pageToken,omitempty"`
	}{Query: input.Query, PageSize: input.PageSize, PageToken: input.PageToken}
	var response struct {
		Results       []ReportRow `json:"results"`
		NextPageToken string      `json:"nextPageToken"`
	}
	path := "/reports/v1/" + client.accountName() + "/reports:search"
	if _, err := client.sendJSON(ctx, operation, http.MethodPost, path, nil, body, &response, false, options...); err != nil {
		return ReportPage{}, err
	}
	if len(response.Results) > effectivePageSize(input.PageSize, 1000) || !validPageToken(response.NextPageToken) {
		return ReportPage{}, platformContractError(operation, "Merchant API returned invalid report pagination")
	}
	for _, row := range response.Results {
		if len(row) != 1 {
			return ReportPage{}, platformContractError(operation, "Merchant API report row must contain exactly one queried view")
		}
	}
	return ReportPage{Rows: append([]ReportRow(nil), response.Results...), NextPageToken: response.NextPageToken}, nil
}

var _ ReportWorkflow = (*Client)(nil)

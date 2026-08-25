package searchads360

import (
	"context"

	"social-hub/pkg/socialhub"
)

func (client *Client) Search(ctx context.Context, input SearchRequest, options ...socialhub.CallOption) (SearchPage, error) {
	const operation = "reports.search"
	if !validReportRequest(input) {
		return SearchPage{}, invalidArgument(operation, "query, page size, page token, or summary-row setting is invalid")
	}
	body := struct {
		Query                   string            `json:"query"`
		PageSize                int               `json:"pageSize,omitempty"`
		PageToken               string            `json:"pageToken,omitempty"`
		ValidateOnly            bool              `json:"validateOnly,omitempty"`
		ReturnTotalResultsCount bool              `json:"returnTotalResultsCount,omitempty"`
		SummaryRowSetting       SummaryRowSetting `json:"summaryRowSetting,omitempty"`
	}{
		Query: input.Query, PageSize: input.PageSize, PageToken: input.PageToken,
		ValidateOnly: input.ValidateOnly, ReturnTotalResultsCount: input.ReturnTotalResultsCount,
		SummaryRowSetting: input.SummaryRowSetting,
	}
	var response searchResponse
	path := "/v0/customers/" + client.customerID + "/searchAds360:search"
	if _, err := client.postJSON(ctx, operation, path, body, &response, options...); err != nil {
		return SearchPage{}, err
	}
	if !validSearchResponse(response, input.PageSize) {
		return SearchPage{}, platformContractError(operation, "Search Ads 360 returned invalid report rows or pagination metadata")
	}
	return SearchPage{
		Rows: response.Results, SummaryRow: response.SummaryRow,
		NextPageToken: response.NextPageToken, FieldMask: response.FieldMask,
		TotalResultsCount:                  response.TotalResultsCount,
		ConversionCustomMetricHeaders:      append([]ResultHeader(nil), response.ConversionCustomMetricHeaders...),
		ConversionCustomDimensionHeaders:   append([]ResultHeader(nil), response.ConversionCustomDimensionHeaders...),
		RawEventConversionMetricHeaders:    append([]ResultHeader(nil), response.RawEventConversionMetricHeaders...),
		RawEventConversionDimensionHeaders: append([]ResultHeader(nil), response.RawEventConversionDimensionHeaders...),
		CustomColumnHeaders:                append([]CustomColumnHeader(nil), response.CustomColumnHeaders...),
	}, nil
}

var _ ReportWorkflow = (*Client)(nil)

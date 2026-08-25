package searchads360

import (
	"context"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListCustomColumns(ctx context.Context, options ...socialhub.CallOption) ([]CustomColumn, error) {
	const operation = "custom_columns.list"
	var response struct {
		CustomColumns []CustomColumn `json:"customColumns"`
	}
	path := "/v0/customers/" + client.customerID + "/customColumns"
	if _, err := client.getJSON(ctx, operation, path, &response, options...); err != nil {
		return nil, err
	}
	for index := range response.CustomColumns {
		if !validCustomColumn(client.customerID, response.CustomColumns[index]) {
			return nil, platformContractError(operation, "Search Ads 360 returned an invalid or cross-customer Custom Column")
		}
	}
	return append([]CustomColumn(nil), response.CustomColumns...), nil
}

func (client *Client) GetCustomColumn(ctx context.Context, resourceName string, options ...socialhub.CallOption) (*CustomColumn, error) {
	const operation = "custom_columns.get"
	if !validCustomColumnResourceName(client.customerID, resourceName) {
		return nil, invalidArgument(operation, "resource name must identify a Custom Column owned by the configured customer")
	}
	var response CustomColumn
	if _, err := client.getJSON(ctx, operation, "/v0/"+resourceName, &response, options...); err != nil {
		return nil, err
	}
	if response.ResourceName != resourceName || !validCustomColumn(client.customerID, response) {
		return nil, platformContractError(operation, "Search Ads 360 returned a mismatched Custom Column")
	}
	return &response, nil
}

func (client *Client) SearchFields(ctx context.Context, input FieldSearchRequest, options ...socialhub.CallOption) (FieldPage, error) {
	const operation = "fields.search"
	if !validFieldSearchRequest(input) {
		return FieldPage{}, invalidArgument(operation, "field query, page size, or page token is invalid")
	}
	body := struct {
		Query     string `json:"query"`
		PageSize  int    `json:"pageSize,omitempty"`
		PageToken string `json:"pageToken,omitempty"`
	}{Query: input.Query, PageSize: input.PageSize, PageToken: input.PageToken}
	var response fieldSearchResponse
	if _, err := client.postJSON(ctx, operation, "/v0/searchAds360Fields:search", body, &response, options...); err != nil {
		return FieldPage{}, err
	}
	if !validPageToken(response.NextPageToken) || !validOptionalUint(response.TotalResultsCount) ||
		!validFieldPage(response.Results, input.PageSize) {
		return FieldPage{}, platformContractError(operation, "Search Ads 360 returned invalid field metadata or pagination")
	}
	return FieldPage{
		Items: append([]Field(nil), response.Results...), NextPageToken: response.NextPageToken,
		TotalResultsCount: response.TotalResultsCount,
	}, nil
}

func (client *Client) GetField(ctx context.Context, resourceName string, options ...socialhub.CallOption) (*Field, error) {
	const operation = "fields.get"
	if !validFieldResourceName(resourceName) {
		return nil, invalidArgument(operation, "resource name must have the form searchAds360Fields/{field_name}")
	}
	var response Field
	if _, err := client.getJSON(ctx, operation, "/v0/"+resourceName, &response, options...); err != nil {
		return nil, err
	}
	if response.ResourceName != resourceName || !validField(response) {
		return nil, platformContractError(operation, "Search Ads 360 returned mismatched field metadata")
	}
	return &response, nil
}

var _ CustomColumnWorkflow = (*Client)(nil)
var _ FieldWorkflow = (*Client)(nil)

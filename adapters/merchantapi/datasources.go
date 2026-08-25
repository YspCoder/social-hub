package merchantapi

import (
	"context"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListDataSources(ctx context.Context, input ListRequest, options ...socialhub.CallOption) (TokenPage[DataSource], error) {
	const operation = "data_sources.list"
	if !validListRequest(input, 1000) {
		return TokenPage[DataSource]{}, invalidArgument(operation, "page size or page token is invalid")
	}
	query := listQuery(input)
	var response struct {
		DataSources   []DataSource `json:"dataSources"`
		NextPageToken string       `json:"nextPageToken"`
	}
	path := "/datasources/v1/" + client.accountName() + "/dataSources"
	if _, err := client.getJSON(ctx, operation, path, query, &response, options...); err != nil {
		return TokenPage[DataSource]{}, err
	}
	if len(response.DataSources) > effectivePageSize(input.PageSize, 1000) || !validPageToken(response.NextPageToken) {
		return TokenPage[DataSource]{}, platformContractError(operation, "Merchant API returned invalid Data Source pagination")
	}
	seen := make(map[string]struct{}, len(response.DataSources))
	for _, dataSource := range response.DataSources {
		if !validDataSource(client.merchantAccountID, dataSource) {
			return TokenPage[DataSource]{}, platformContractError(operation, "Merchant API returned a malformed or cross-account Data Source")
		}
		if _, found := seen[dataSource.Name]; found {
			return TokenPage[DataSource]{}, platformContractError(operation, "Merchant API returned a duplicate Data Source")
		}
		seen[dataSource.Name] = struct{}{}
	}
	return TokenPage[DataSource]{
		Items: append([]DataSource(nil), response.DataSources...), NextPageToken: response.NextPageToken,
	}, nil
}

func (client *Client) GetDataSource(ctx context.Context, resourceName string, options ...socialhub.CallOption) (*DataSource, error) {
	const operation = "data_sources.get"
	if !validDataSourceName(client.merchantAccountID, resourceName) {
		return nil, invalidArgument(operation, "resource name must identify a Data Source owned by the configured account")
	}
	var response DataSource
	if _, err := client.getJSON(ctx, operation, "/datasources/v1/"+resourceName, nil, &response, options...); err != nil {
		return nil, err
	}
	if response.Name != resourceName || !validDataSource(client.merchantAccountID, response) {
		return nil, platformContractError(operation, "Merchant API returned a mismatched Data Source")
	}
	return &response, nil
}

func listQuery(input ListRequest) url.Values {
	query := make(url.Values)
	if input.PageSize > 0 {
		query.Set("pageSize", strconv.Itoa(input.PageSize))
	}
	if input.PageToken != "" {
		query.Set("pageToken", input.PageToken)
	}
	return query
}

var _ DataSourceWorkflow = (*Client)(nil)

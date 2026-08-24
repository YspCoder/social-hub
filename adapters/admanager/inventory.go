package admanager

import (
	"context"
	"net/url"

	"social-hub/pkg/socialhub"
)

type InventoryWorkflow interface {
	GetNetwork(context.Context, ...socialhub.CallOption) (*Network, error)
	GetCompany(context.Context, string, ...socialhub.CallOption) (*Company, error)
	ListCompanies(context.Context, ListRequest, ...socialhub.CallOption) (Page[Company], error)
	GetAdUnit(context.Context, string, ...socialhub.CallOption) (*AdUnit, error)
	ListAdUnits(context.Context, ListRequest, ...socialhub.CallOption) (Page[AdUnit], error)
}

func (client *Client) GetNetwork(ctx context.Context, options ...socialhub.CallOption) (*Network, error) {
	const operation = "network_get"
	var output Network
	if err := client.getJSON(ctx, operation, "/v1/"+client.networkName(), nil, &output, options...); err != nil {
		return nil, err
	}
	if output.Name != client.networkName() || output.NetworkCode != "" && output.NetworkCode != client.networkCode ||
		output.EffectiveRootAdUnit != "" && !client.ownsResource(output.EffectiveRootAdUnit, "adUnits") {
		return nil, ownershipError(operation, "network")
	}
	return &output, nil
}

func (client *Client) GetCompany(ctx context.Context, companyID string, options ...socialhub.CallOption) (*Company, error) {
	const operation = "company_get"
	name, err := client.resourceName(operation, "companies", companyID)
	if err != nil {
		return nil, err
	}
	var output Company
	if err := client.getJSON(ctx, operation, "/v1/"+name, nil, &output, options...); err != nil {
		return nil, err
	}
	if output.Name != name || !client.validCompanyReferences(output) {
		return nil, ownershipError(operation, "company")
	}
	return &output, nil
}

func (client *Client) ListCompanies(ctx context.Context, input ListRequest, options ...socialhub.CallOption) (Page[Company], error) {
	const operation = "companies_list"
	query, err := listQuery(operation, input, 1000)
	if err != nil {
		return Page[Company]{}, err
	}
	query.Set("fields", "companies,nextPageToken,totalSize")
	var output struct {
		Companies     []Company `json:"companies"`
		NextPageToken string    `json:"nextPageToken"`
		TotalSize     int32     `json:"totalSize"`
	}
	if err := client.getJSON(ctx, operation, "/v1/"+client.networkName()+"/companies", query, &output, options...); err != nil {
		return Page[Company]{}, err
	}
	for _, item := range output.Companies {
		if !client.ownsResource(item.Name, "companies") || !client.validCompanyReferences(item) {
			return Page[Company]{}, ownershipError(operation, "company")
		}
	}
	return Page[Company]{Items: output.Companies, NextPageToken: output.NextPageToken, TotalSize: output.TotalSize}, nil
}

func (client *Client) GetAdUnit(ctx context.Context, adUnitID string, options ...socialhub.CallOption) (*AdUnit, error) {
	const operation = "ad_unit_get"
	name, err := client.resourceName(operation, "adUnits", adUnitID)
	if err != nil {
		return nil, err
	}
	var output AdUnit
	if err := client.getJSON(ctx, operation, "/v1/"+name, nil, &output, options...); err != nil {
		return nil, err
	}
	if output.Name != name || !client.validAdUnitReferences(output) {
		return nil, ownershipError(operation, "ad unit")
	}
	return &output, nil
}

func (client *Client) ListAdUnits(ctx context.Context, input ListRequest, options ...socialhub.CallOption) (Page[AdUnit], error) {
	const operation = "ad_units_list"
	query, err := listQuery(operation, input, 1000)
	if err != nil {
		return Page[AdUnit]{}, err
	}
	query.Set("fields", "adUnits,nextPageToken,totalSize")
	var output struct {
		AdUnits       []AdUnit `json:"adUnits"`
		NextPageToken string   `json:"nextPageToken"`
		TotalSize     int32    `json:"totalSize"`
	}
	if err := client.getJSON(ctx, operation, "/v1/"+client.networkName()+"/adUnits", query, &output, options...); err != nil {
		return Page[AdUnit]{}, err
	}
	for _, item := range output.AdUnits {
		if !client.ownsResource(item.Name, "adUnits") || !client.validAdUnitReferences(item) {
			return Page[AdUnit]{}, ownershipError(operation, "ad unit")
		}
	}
	return Page[AdUnit]{Items: output.AdUnits, NextPageToken: output.NextPageToken, TotalSize: output.TotalSize}, nil
}

func (client *Client) validCompanyReferences(value Company) bool {
	return value.PrimaryContact == "" || client.ownsResource(value.PrimaryContact, "contacts")
}

func (client *Client) validAdUnitReferences(value AdUnit) bool {
	if value.ParentAdUnit != "" && !client.ownsResource(value.ParentAdUnit, "adUnits") {
		return false
	}
	for _, parent := range value.ParentPath {
		if !client.ownsResource(parent.ParentAdUnit, "adUnits") {
			return false
		}
	}
	return true
}

func listQuery(operation string, input ListRequest, maximum int32) (url.Values, error) {
	if !validListRequest(input, maximum) {
		return nil, invalidArgument(operation, "pagination, filter, or ordering is invalid")
	}
	query := make(url.Values)
	if input.PageSize > 0 {
		query.Set("pageSize", int32String(input.PageSize))
	}
	if input.PageToken != "" {
		query.Set("pageToken", input.PageToken)
	}
	if input.Filter != "" {
		query.Set("filter", input.Filter)
	}
	if input.OrderBy != "" {
		query.Set("orderBy", input.OrderBy)
	}
	if input.Skip > 0 {
		query.Set("skip", int32String(input.Skip))
	}
	return query, nil
}

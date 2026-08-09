package admob

import (
	"context"

	"social-hub/pkg/socialhub"
)

type InventoryWorkflow interface {
	ListApps(context.Context, ListRequest, ...socialhub.CallOption) (Page[App], error)
	ListAdUnits(context.Context, ListRequest, ...socialhub.CallOption) (Page[AdUnit], error)
}

func (client *Client) ListApps(ctx context.Context, input ListRequest, options ...socialhub.CallOption) (Page[App], error) {
	const operation = "apps_list"
	if err := client.requireScope(operation, readOnlyScope); err != nil {
		return Page[App]{}, err
	}
	query, err := listQuery(operation, input, true)
	if err != nil {
		return Page[App]{}, err
	}
	var output struct {
		Apps          []App  `json:"apps"`
		NextPageToken string `json:"nextPageToken"`
	}
	if err := client.getJSON(ctx, operation, "/v1/"+client.accountName()+"/apps", query, &output, options...); err != nil {
		return Page[App]{}, err
	}
	if !validInventoryPage(input, len(output.Apps), output.NextPageToken) {
		return Page[App]{}, platformContractError(operation, "AdMob returned invalid app pagination")
	}
	seen := make(map[string]struct{}, len(output.Apps))
	for _, app := range output.Apps {
		if !validApp(app, client.publisherID) {
			return Page[App]{}, ownershipError(operation, "app")
		}
		if _, exists := seen[app.Name]; exists {
			return Page[App]{}, platformContractError(operation, "AdMob returned duplicate apps")
		}
		seen[app.Name] = struct{}{}
	}
	return Page[App]{Items: output.Apps, NextPageToken: output.NextPageToken}, nil
}

func (client *Client) ListAdUnits(ctx context.Context, input ListRequest, options ...socialhub.CallOption) (Page[AdUnit], error) {
	const operation = "ad_units_list"
	if err := client.requireScope(operation, readOnlyScope); err != nil {
		return Page[AdUnit]{}, err
	}
	query, err := listQuery(operation, input, true)
	if err != nil {
		return Page[AdUnit]{}, err
	}
	var output struct {
		AdUnits       []AdUnit `json:"adUnits"`
		NextPageToken string   `json:"nextPageToken"`
	}
	if err := client.getJSON(ctx, operation, "/v1/"+client.accountName()+"/adUnits", query, &output, options...); err != nil {
		return Page[AdUnit]{}, err
	}
	if !validInventoryPage(input, len(output.AdUnits), output.NextPageToken) {
		return Page[AdUnit]{}, platformContractError(operation, "AdMob returned invalid ad-unit pagination")
	}
	seen := make(map[string]struct{}, len(output.AdUnits))
	for _, adUnit := range output.AdUnits {
		if !validAdUnit(adUnit, client.publisherID) {
			return Page[AdUnit]{}, ownershipError(operation, "ad unit")
		}
		if _, exists := seen[adUnit.Name]; exists {
			return Page[AdUnit]{}, platformContractError(operation, "AdMob returned duplicate ad units")
		}
		seen[adUnit.Name] = struct{}{}
	}
	return Page[AdUnit]{Items: output.AdUnits, NextPageToken: output.NextPageToken}, nil
}

func validInventoryPage(input ListRequest, count int, nextPageToken string) bool {
	limit := input.PageSize
	if limit == 0 {
		limit = DefaultQuotaPolicy().DefaultInventoryPageSize
	}
	return count <= int(limit) && validPageToken(nextPageToken)
}

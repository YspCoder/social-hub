package searchads360

import (
	"context"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListAccessibleCustomers(ctx context.Context, options ...socialhub.CallOption) ([]string, error) {
	const operation = "customers.list_accessible"
	var response struct {
		ResourceNames []string `json:"resourceNames"`
	}
	if _, err := client.getJSON(ctx, operation, "/v0/customers:listAccessibleCustomers", &response, options...); err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(response.ResourceNames))
	for _, name := range response.ResourceNames {
		if !validCustomerResourceName(name) {
			return nil, platformContractError(operation, "Search Ads 360 returned an invalid customer resource name")
		}
		if _, found := seen[name]; found {
			return nil, platformContractError(operation, "Search Ads 360 returned a duplicate customer resource name")
		}
		seen[name] = struct{}{}
	}
	return append([]string(nil), response.ResourceNames...), nil
}

var _ CustomerWorkflow = (*Client)(nil)

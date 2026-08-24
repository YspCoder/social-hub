package googleads

import (
	"context"

	"social-hub/pkg/socialhub"
)

const customerQuery = "SELECT customer.resource_name, customer.id, customer.descriptive_name, customer.currency_code, customer.time_zone, customer.manager, customer.test_account FROM customer LIMIT 1"

func (client *Client) GetCustomer(ctx context.Context, options ...socialhub.CallOption) (*Customer, error) {
	const operation = "customer_get"
	type row struct {
		Customer Customer `json:"customer"`
	}
	response, err := searchRows[row](ctx, client, operation, customerQuery, "", false, options...)
	if err != nil {
		return nil, err
	}
	if len(response.Results) != 1 {
		return nil, platformContractError(operation, "Google Ads customer query did not return exactly one Customer")
	}
	customer := response.Results[0].Customer
	expected := "customers/" + client.customerID
	if customer.ResourceName != expected || customer.ID != "" && customer.ID != client.customerID {
		return nil, platformContractError(operation, "Google Ads returned a different Customer")
	}
	customer.ID = client.customerID
	return &customer, nil
}

func (client *Client) ListAccessibleCustomers(ctx context.Context, options ...socialhub.CallOption) ([]string, error) {
	const operation = "customer_list_accessible"
	var response struct {
		ResourceNames []string `json:"resourceNames"`
	}
	if _, err := client.getJSON(ctx, operation, "/v25/customers:listAccessibleCustomers", &response, options...); err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(response.ResourceNames))
	for _, name := range response.ResourceNames {
		if !validCustomerResourceName(name) {
			return nil, platformContractError(operation, "Google Ads returned an invalid Customer resource name")
		}
		if _, found := seen[name]; found {
			return nil, platformContractError(operation, "Google Ads returned duplicate Customer resource names")
		}
		seen[name] = struct{}{}
	}
	return append([]string(nil), response.ResourceNames...), nil
}

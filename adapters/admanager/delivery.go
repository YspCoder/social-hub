package admanager

import (
	"context"

	"social-hub/pkg/socialhub"
)

type DeliveryWorkflow interface {
	GetOrder(context.Context, string, ...socialhub.CallOption) (*Order, error)
	ListOrders(context.Context, ListRequest, ...socialhub.CallOption) (Page[Order], error)
	GetLineItem(context.Context, string, ...socialhub.CallOption) (*LineItem, error)
	ListLineItems(context.Context, ListRequest, ...socialhub.CallOption) (Page[LineItem], error)
}

func (client *Client) GetOrder(ctx context.Context, orderID string, options ...socialhub.CallOption) (*Order, error) {
	const operation = "order_get"
	name, err := client.resourceName(operation, "orders", orderID)
	if err != nil {
		return nil, err
	}
	var output Order
	if err := client.getJSON(ctx, operation, "/v1/"+name, nil, &output, options...); err != nil {
		return nil, err
	}
	if output.Name != name || !client.validOrderReferences(output) {
		return nil, ownershipError(operation, "order")
	}
	return &output, nil
}

func (client *Client) ListOrders(ctx context.Context, input ListRequest, options ...socialhub.CallOption) (Page[Order], error) {
	const operation = "orders_list"
	query, err := listQuery(operation, input, 1000)
	if err != nil {
		return Page[Order]{}, err
	}
	query.Set("fields", "orders,nextPageToken,totalSize")
	var output struct {
		Orders        []Order `json:"orders"`
		NextPageToken string  `json:"nextPageToken"`
		TotalSize     int32   `json:"totalSize"`
	}
	if err := client.getJSON(ctx, operation, "/v1/"+client.networkName()+"/orders", query, &output, options...); err != nil {
		return Page[Order]{}, err
	}
	for _, item := range output.Orders {
		if !client.ownsResource(item.Name, "orders") || !client.validOrderReferences(item) {
			return Page[Order]{}, ownershipError(operation, "order")
		}
	}
	return Page[Order]{Items: output.Orders, NextPageToken: output.NextPageToken, TotalSize: output.TotalSize}, nil
}

func (client *Client) GetLineItem(ctx context.Context, lineItemID string, options ...socialhub.CallOption) (*LineItem, error) {
	const operation = "line_item_get"
	name, err := client.resourceName(operation, "lineItems", lineItemID)
	if err != nil {
		return nil, err
	}
	var output LineItem
	if err := client.getJSON(ctx, operation, "/v1/"+name, nil, &output, options...); err != nil {
		return nil, err
	}
	if output.Name != name ||
		output.Order != "" && !client.ownsResource(output.Order, "orders") {
		return nil, ownershipError(operation, "line item")
	}
	return &output, nil
}

func (client *Client) validOrderReferences(value Order) bool {
	for _, reference := range []struct {
		name     string
		resource string
	}{
		{value.Advertiser, "companies"}, {value.Agency, "companies"},
		{value.Trafficker, "users"}, {value.Creator, "users"}, {value.Salesperson, "users"},
	} {
		if reference.name != "" && !client.ownsResource(reference.name, reference.resource) {
			return false
		}
	}
	return true
}

func (client *Client) ListLineItems(ctx context.Context, input ListRequest, options ...socialhub.CallOption) (Page[LineItem], error) {
	const operation = "line_items_list"
	query, err := listQuery(operation, input, 1000)
	if err != nil {
		return Page[LineItem]{}, err
	}
	query.Set("fields", "lineItems,nextPageToken,totalSize")
	var output struct {
		LineItems     []LineItem `json:"lineItems"`
		NextPageToken string     `json:"nextPageToken"`
		TotalSize     int32      `json:"totalSize"`
	}
	if err := client.getJSON(ctx, operation, "/v1/"+client.networkName()+"/lineItems", query, &output, options...); err != nil {
		return Page[LineItem]{}, err
	}
	for _, item := range output.LineItems {
		if !client.ownsResource(item.Name, "lineItems") ||
			item.Order != "" && !client.ownsResource(item.Order, "orders") {
			return Page[LineItem]{}, ownershipError(operation, "line item")
		}
	}
	return Page[LineItem]{Items: output.LineItems, NextPageToken: output.NextPageToken, TotalSize: output.TotalSize}, nil
}

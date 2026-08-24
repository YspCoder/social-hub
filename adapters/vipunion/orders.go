package vipunion

import (
	"context"

	"social-hub/pkg/socialhub"
)

const (
	orderService    = "com.vip.adp.api.open.service.UnionOrderV2Service"
	orderListMethod = "orderListWithOauth"
)

type orderQueryWire struct {
	Status                  *OrderStatus `json:"status,omitempty"`
	OrderTimeStart          int64        `json:"orderTimeStart,omitempty"`
	OrderTimeEnd            int64        `json:"orderTimeEnd,omitempty"`
	Page                    int          `json:"page"`
	PageSize                int          `json:"pageSize"`
	RequestID               string       `json:"requestId"`
	UpdateTimeStart         int64        `json:"updateTimeStart,omitempty"`
	UpdateTimeEnd           int64        `json:"updateTimeEnd,omitempty"`
	OrderSNs                []string     `json:"orderSnList,omitempty"`
	ChanTag                 string       `json:"chanTag,omitempty"`
	QuerySubsidyActivity    *bool        `json:"querySubsidyActFlag,omitempty"`
	FilterSplitParentOrders *bool        `json:"filterSplitOrder,omitempty"`
}

func (client *Client) ListOrders(
	ctx context.Context,
	input OrderListRequest,
	options ...socialhub.CallOption,
) (OrderPage, error) {
	const operation = "list_orders"
	if !validOrderList(input) {
		return OrderPage{}, invalidArgument(operation, "order/update range, order numbers, status, channel tag, or paging is invalid")
	}
	page, pageSize := input.Page, input.PageSize
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 20
	}
	requestID, forwarded, err := prepareCallOptions(operation, options)
	if err != nil {
		return OrderPage{}, err
	}
	body := struct {
		Query orderQueryWire `json:"queryModel"`
	}{Query: orderQueryWire{
		Status: input.Status, OrderTimeStart: milliseconds(input.OrderTimeStart),
		OrderTimeEnd: milliseconds(input.OrderTimeEnd), Page: page, PageSize: pageSize,
		RequestID: requestID, UpdateTimeStart: milliseconds(input.UpdateTimeStart),
		UpdateTimeEnd: milliseconds(input.UpdateTimeEnd), OrderSNs: input.OrderSNs,
		ChanTag: input.ChanTag, QuerySubsidyActivity: input.QuerySubsidyActivity,
		FilterSplitParentOrders: input.FilterSplitParentOrders,
	}}
	var response struct {
		Orders   []Order `json:"orderInfoList"`
		Total    int     `json:"total"`
		Page     int     `json:"page"`
		PageSize int     `json:"pageSize"`
	}
	meta, err := client.doJSON(
		ctx, operation, orderService, orderListMethod, requestID, body, &response, forwarded...,
	)
	if err != nil {
		return OrderPage{}, err
	}
	return OrderPage{
		Orders: response.Orders, Total: response.Total, Page: response.Page,
		PageSize: response.PageSize, Meta: meta,
	}, nil
}

func validOrderList(input OrderListRequest) bool {
	if !validTimeRange(input.OrderTimeStart, input.OrderTimeEnd) ||
		!validTimeRange(input.UpdateTimeStart, input.UpdateTimeEnd) || !validOrderSNs(input.OrderSNs) ||
		input.Page < 0 || input.PageSize < 0 || input.PageSize > 100 ||
		input.ChanTag != "" && !validChanTag(input.ChanTag) {
		return false
	}
	if input.Status != nil && (*input.Status < OrderStatusInvalid || *input.Status > OrderStatusCompleted) {
		return false
	}
	hasOrderRange := !input.OrderTimeStart.IsZero()
	hasUpdateRange := !input.UpdateTimeStart.IsZero()
	return hasOrderRange || hasUpdateRange || len(input.OrderSNs) > 0
}

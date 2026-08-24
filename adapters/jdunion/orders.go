package jdunion

import (
	"context"
	"strings"

	"social-hub/pkg/socialhub"
)

const orderRowQueryMethod = "jd.union.open.order.row.query"

func (client *Client) QueryOrderRows(
	ctx context.Context,
	input OrderQueryRequest,
	options ...socialhub.CallOption,
) (OrderPage, error) {
	const operation = "query_order_rows"
	if !validOrderQuery(input) {
		return OrderPage{}, invalidArgument(operation, "pagination, one-hour time range, authorization selector, fields, or order ID are invalid")
	}
	fields := make([]string, len(input.Fields))
	for index, field := range input.Fields {
		fields[index] = string(field)
	}
	request := struct {
		PageIndex    uint64 `json:"pageIndex"`
		PageSize     uint64 `json:"pageSize"`
		QueryType    uint64 `json:"type"`
		StartTime    string `json:"startTime"`
		EndTime      string `json:"endTime"`
		ChildUnionID uint64 `json:"childUnionId,omitempty"`
		Key          string `json:"key,omitempty"`
		Fields       string `json:"fields,omitempty"`
		OrderID      uint64 `json:"orderId,omitempty"`
	}{
		PageIndex: input.PageIndex, PageSize: input.PageSize, QueryType: uint64(input.QueryType),
		StartTime:    input.StartTime.In(jdLocation).Format("2006-01-02 15:04:05"),
		EndTime:      input.EndTime.In(jdLocation).Format("2006-01-02 15:04:05"),
		ChildUnionID: input.ChildUnionID, Key: input.Key, Fields: strings.Join(fields, ","), OrderID: input.OrderID,
	}
	result, meta, err := client.doMethod(ctx, operation, orderRowQueryMethod, "orderReq", request, "queryResult", options...)
	if err != nil {
		return OrderPage{}, err
	}
	orders, err := decodeProviderList[Order](result.Data, "orderRowResp")
	if err != nil {
		return OrderPage{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return OrderPage{Orders: orders, HasMore: result.HasMore.value, Meta: meta}, nil
}

func validOrderQuery(input OrderQueryRequest) bool {
	if input.PageIndex == 0 || input.PageIndex > 1_000_000 || input.PageSize == 0 || input.PageSize > 200 ||
		input.QueryType < OrderQueryCreated || input.QueryType > OrderQueryUpdated ||
		!validOrderRange(input.StartTime, input.EndTime) || input.ChildUnionID != 0 && input.Key != "" ||
		input.Key != "" && !validIdentifier(input.Key, 4096) || !uniqueOrderFields(input.Fields) {
		return false
	}
	return true
}

package taobaounion

import (
	"context"
	"net/url"

	"social-hub/pkg/socialhub"
)

const orderDetailsMethod = "taobao.tbk.order.details.get"

func (client *Client) ListOrders(
	ctx context.Context,
	input OrderDetailsRequest,
	options ...socialhub.CallOption,
) (OrderPage, error) {
	const operation = "list_orders"
	if !validOrderDetails(input) {
		return OrderPage{}, invalidArgument(operation, "order time range, filters, or pagination are invalid; the normal query window is at most 3 hours")
	}
	queryType := input.QueryType
	if queryType == 0 {
		queryType = OrderQueryCreated
	}
	values := make(url.Values)
	values.Set("start_time", input.StartTime.In(topLocation).Format("2006-01-02 15:04:05"))
	values.Set("end_time", input.EndTime.In(topLocation).Format("2006-01-02 15:04:05"))
	setInt(values, "query_type", int64(queryType))
	setInt(values, "page_size", input.PageSize)
	setInt(values, "page_no", input.PageNo)
	setString(values, "position_index", input.PositionIndex)
	setInt(values, "jump_type", int64(input.Direction))
	setInt(values, "member_type", int64(input.MemberType))
	setInt(values, "tk_status", int64(input.Status))
	setInt(values, "order_scene", int64(input.Scene))
	var response struct {
		Data *struct {
			Orders        []Order    `json:"results"`
			PositionIndex string     `json:"position_index"`
			PageNo        ExactValue `json:"page_no"`
			PageSize      ExactValue `json:"page_size"`
			PreviousPage  ExactValue `json:"pre_page"`
			NextPage      ExactValue `json:"next_page"`
			HasPrevious   bool       `json:"has_pre"`
			HasNext       bool       `json:"has_next"`
		} `json:"data"`
	}
	meta, err := client.doForm(ctx, operation, orderDetailsMethod, values, &response, options...)
	if err != nil {
		return OrderPage{}, err
	}
	if response.Data == nil {
		return OrderPage{}, platformContractError(operation, "TOP order response omitted data")
	}
	return OrderPage{
		Orders: response.Data.Orders, PositionIndex: response.Data.PositionIndex,
		PageNo: response.Data.PageNo, PageSize: response.Data.PageSize,
		PreviousPage: response.Data.PreviousPage, NextPage: response.Data.NextPage,
		HasPrevious: response.Data.HasPrevious, HasNext: response.Data.HasNext, Meta: meta,
	}, nil
}

func validOrderDetails(input OrderDetailsRequest) bool {
	if !validOrderRange(input.StartTime, input.EndTime) || input.QueryType < 0 || input.QueryType > OrderQueryUpdated ||
		input.PageSize < 0 || input.PageSize > 100 || input.PageNo < 0 || input.PageNo > 100 ||
		input.PositionIndex != "" && !validOpaque(input.PositionIndex, 16_384) ||
		input.Direction != 0 && input.Direction != PagePrevious && input.Direction != PageNext ||
		input.MemberType != 0 && input.MemberType != OrderMemberSecondParty && input.MemberType != OrderMemberThirdParty ||
		!validOrderStatus(input.Status) || input.Scene < 0 || input.Scene > OrderSceneMember {
		return false
	}
	effectivePage := input.PageNo
	if effectivePage == 0 {
		effectivePage = 1
	}
	if effectivePage > 1 && input.PositionIndex == "" {
		return false
	}
	return (input.PositionIndex == "") == (input.Direction == 0)
}

func validOrderStatus(value OrderStatus) bool {
	switch value {
	case 0, OrderStatusSettled, OrderStatusCreated, OrderStatusPaid, OrderStatusClosed, OrderStatusReceived:
		return true
	default:
		return false
	}
}

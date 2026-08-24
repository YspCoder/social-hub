package pddunion

import (
	"context"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

const orderIncrementMethod = "pdd.ddk.order.list.increment.get"

func (client *Client) ListIncrementalOrders(
	ctx context.Context,
	input OrderIncrementRequest,
	options ...socialhub.CallOption,
) (OrderPage, error) {
	const operation = "list_incremental_orders"
	if !validOrderIncrement(input) {
		return OrderPage{}, invalidArgument(operation, "update range, page size, or page is invalid")
	}
	pageSize, page := input.PageSize, input.Page
	if pageSize == 0 {
		pageSize = 50
	}
	if page == 0 {
		page = 1
	}
	values := make(url.Values)
	values.Set("start_update_time", strconv.FormatInt(input.StartUpdateTime.Unix(), 10))
	values.Set("end_update_time", strconv.FormatInt(input.EndUpdateTime.Unix(), 10))
	values.Set("page_size", strconv.FormatInt(pageSize, 10))
	values.Set("page", strconv.FormatInt(page, 10))
	var response struct {
		Orders     []Order    `json:"order_list"`
		TotalCount ExactValue `json:"total_count"`
	}
	meta, err := client.doForm(ctx, operation, orderIncrementMethod, "order_list_get_response", values, &response, options...)
	if err != nil {
		return OrderPage{}, err
	}
	return OrderPage{Orders: response.Orders, TotalCount: response.TotalCount, Meta: meta}, nil
}

func validOrderIncrement(input OrderIncrementRequest) bool {
	if !validOrderRange(input.StartUpdateTime, input.EndUpdateTime) || input.PageSize < 0 ||
		input.PageSize > 0 && (input.PageSize < 10 || input.PageSize > 100) ||
		input.Page < 0 || input.Page > 10_000 {
		return false
	}
	return true
}

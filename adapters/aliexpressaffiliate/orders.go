package aliexpressaffiliate

import (
	"context"
	"net/url"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	orderListMethod = "aliexpress.affiliate.order.list"
	orderGetMethod  = "aliexpress.affiliate.order.get"
)

var affiliatePST = time.FixedZone("PST", -8*60*60)

func (client *Client) ListOrders(
	ctx context.Context,
	input OrderListRequest,
	options ...socialhub.CallOption,
) (OrderPage, error) {
	const operation = "list_orders"
	if !validOrderList(input) {
		return OrderPage{}, invalidArgument(operation, "order time range, status, fields, locale, or pagination are invalid")
	}
	values := make(url.Values)
	setString(values, "time_type", string(input.TimeType))
	setString(values, "app_signature", client.appSignature(input.AppSignature))
	values.Set("start_time", input.StartTime.In(affiliatePST).Format("2006-01-02 15:04:05"))
	values.Set("end_time", input.EndTime.In(affiliatePST).Format("2006-01-02 15:04:05"))
	if len(input.Fields) > 0 {
		values.Set("fields", strings.Join(input.Fields, ","))
	}
	setString(values, "locale_site", input.LocaleSite)
	setUint(values, "page_no", input.PageNo)
	setUint(values, "page_size", input.PageSize)
	values.Set("status", string(input.Status))
	var response struct {
		CurrentPageNo      ExactValue `json:"current_page_no"`
		CurrentRecordCount ExactValue `json:"current_record_count"`
		Orders             []Order    `json:"orders"`
		TotalPageNo        ExactValue `json:"total_page_no"`
		TotalRecordCount   ExactValue `json:"total_record_count"`
	}
	meta, err := client.doForm(ctx, operation, orderListMethod, values, &response, options...)
	if err != nil {
		return OrderPage{}, err
	}
	return OrderPage{
		Orders: response.Orders, CurrentPageNo: response.CurrentPageNo,
		CurrentRecordCount: response.CurrentRecordCount, TotalPageNo: response.TotalPageNo,
		TotalRecordCount: response.TotalRecordCount, Meta: meta,
	}, nil
}

func (client *Client) GetOrders(
	ctx context.Context,
	input OrderGetRequest,
	options ...socialhub.CallOption,
) (OrderGetResult, error) {
	const operation = "get_orders"
	if !validStringIDs(input.OrderIDs) || !validFields(input.Fields) ||
		input.AppSignature != "" && !validOpaque(input.AppSignature, 4096) {
		return OrderGetResult{}, invalidArgument(operation, "order IDs, fields, or app signature are invalid")
	}
	values := make(url.Values)
	setString(values, "app_signature", client.appSignature(input.AppSignature))
	if len(input.Fields) > 0 {
		values.Set("fields", strings.Join(input.Fields, ","))
	}
	values.Set("order_ids", strings.Join(input.OrderIDs, ","))
	var response struct {
		CurrentRecordCount ExactValue `json:"current_record_count"`
		Orders             []Order    `json:"orders"`
	}
	meta, err := client.doForm(ctx, operation, orderGetMethod, values, &response, options...)
	if err != nil {
		return OrderGetResult{}, err
	}
	return OrderGetResult{Orders: response.Orders, CurrentRecordCount: response.CurrentRecordCount, Meta: meta}, nil
}

func validOrderList(input OrderListRequest) bool {
	return validOrderRange(input.StartTime, input.EndTime) && validOrderTimeType(input.TimeType) &&
		validOrderStatus(input.Status) && validFields(input.Fields) && validProviderLong(input.PageNo) && input.PageSize <= 50 &&
		(input.LocaleSite == "" || validCSVValue(input.LocaleSite, 128)) &&
		(input.AppSignature == "" || validOpaque(input.AppSignature, 4096))
}

func validOrderTimeType(value OrderTimeType) bool {
	switch value {
	case "", OrderTimePaymentCompleted, OrderTimeBuyerConfirmedReceipt, OrderTimeCompletedSettlement:
		return true
	default:
		return false
	}
}

func validOrderStatus(value OrderStatus) bool {
	switch value {
	case OrderStatusPaymentCompleted, OrderStatusBuyerConfirmedReceipt, OrderStatusCompletedSettlement, OrderStatusInvalid:
		return true
	default:
		return false
	}
}

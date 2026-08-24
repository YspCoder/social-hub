package jdunion

import (
	"context"
	"strings"

	"social-hub/pkg/socialhub"
)

const goodsJingfenQueryMethod = "jd.union.open.goods.jingfen.query"

func (client *Client) QueryJingfen(
	ctx context.Context,
	input GoodsQueryRequest,
	options ...socialhub.CallOption,
) (GoodsPage, error) {
	const operation = "query_jingfen"
	if !validGoodsQuery(input) {
		return GoodsPage{}, invalidArgument(operation, "elite ID, pagination, sort, PID, or fields are invalid")
	}
	pageIndex, pageSize := input.PageIndex, input.PageSize
	if pageIndex == 0 {
		pageIndex = 1
	}
	if pageSize == 0 {
		pageSize = 20
	}
	fields := make([]string, len(input.Fields))
	for index, field := range input.Fields {
		fields[index] = string(field)
	}
	request := struct {
		EliteID   uint64 `json:"eliteId"`
		PageIndex uint64 `json:"pageIndex,omitempty"`
		PageSize  uint64 `json:"pageSize,omitempty"`
		SortName  string `json:"sortName,omitempty"`
		Sort      string `json:"sort,omitempty"`
		PID       string `json:"pid,omitempty"`
		Fields    string `json:"fields,omitempty"`
	}{
		EliteID: input.EliteID, PageIndex: pageIndex, PageSize: pageSize,
		SortName: input.SortName, Sort: input.Sort, PID: input.PID, Fields: strings.Join(fields, ","),
	}
	result, meta, err := client.doMethod(ctx, operation, goodsJingfenQueryMethod, "goodsReq", request, "queryResult", options...)
	if err != nil {
		return GoodsPage{}, err
	}
	goods, err := decodeProviderList[Goods](result.Data, "jfGoodsResp")
	if err != nil {
		return GoodsPage{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return GoodsPage{Goods: goods, TotalCount: result.TotalCount, Meta: meta}, nil
}

func validGoodsQuery(input GoodsQueryRequest) bool {
	if input.EliteID == 0 || input.PageIndex > 1_000_000 || input.PageSize > 50 || !validPID(input.PID) ||
		!uniqueGoodsFields(input.Fields) {
		return false
	}
	switch input.SortName {
	case "", "price", "commissionShare", "commission", "inOrderCount30DaysSku", "comments", "goodComments":
	default:
		return false
	}
	return input.Sort == "" || input.Sort == "asc" || input.Sort == "desc"
}

package pddunion

import (
	"context"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

const (
	goodsRecommendMethod = "pdd.ddk.goods.recommend.get"
	goodsDetailMethod    = "pdd.ddk.goods.detail"
)

func (client *Client) RecommendGoods(
	ctx context.Context,
	input GoodsRecommendRequest,
	options ...socialhub.CallOption,
) (GoodsPage, error) {
	const operation = "recommend_goods"
	if !validGoodsRecommendation(input) {
		return GoodsPage{}, invalidArgument(operation, "offset, limit, or recommendation channel is invalid")
	}
	limit := input.Limit
	if limit == 0 {
		limit = 20
	}
	values := make(url.Values)
	setInt(values, "offset", input.Offset)
	setInt(values, "limit", limit)
	if input.Channel != nil {
		values.Set("channel_type", strconv.FormatInt(int64(*input.Channel), 10))
	}
	var response struct {
		Goods    []Goods    `json:"list"`
		ListID   string     `json:"list_id"`
		SearchID string     `json:"search_id"`
		Total    ExactValue `json:"total"`
	}
	meta, err := client.doForm(ctx, operation, goodsRecommendMethod, "goods_basic_detail_response", values, &response, options...)
	if err != nil {
		return GoodsPage{}, err
	}
	return GoodsPage{
		Goods: response.Goods, ListID: response.ListID, SearchID: response.SearchID,
		Total: response.Total, Meta: meta,
	}, nil
}

func (client *Client) GetGoods(
	ctx context.Context,
	input GoodsDetailRequest,
	options ...socialhub.CallOption,
) (GoodsDetailResult, error) {
	const operation = "get_goods"
	pid := client.optionalPID(input.PID)
	if !validGoodsDetail(input, pid) {
		return GoodsDetailResult{}, invalidArgument(operation, "goods sign, PID, custom parameters, or search ID is invalid")
	}
	values := make(url.Values)
	values.Set("goods_sign", input.GoodsSign)
	setString(values, "pid", pid)
	setString(values, "custom_parameters", input.CustomParameters)
	setString(values, "search_id", input.SearchID)
	var response struct {
		Goods []Goods `json:"goods_details"`
	}
	meta, err := client.doForm(ctx, operation, goodsDetailMethod, "goods_detail_response", values, &response, options...)
	if err != nil {
		return GoodsDetailResult{}, err
	}
	return GoodsDetailResult{Goods: response.Goods, Meta: meta}, nil
}

func validGoodsRecommendation(input GoodsRecommendRequest) bool {
	if input.Offset < 0 || input.Limit < 0 || input.Limit > 400 {
		return false
	}
	if input.Channel == nil {
		return true
	}
	return *input.Channel >= RecommendationChannelBudget && *input.Channel <= RecommendationChannelMall
}

func validGoodsDetail(input GoodsDetailRequest, pid string) bool {
	return validOpaque(input.GoodsSign, 1024) && (pid == "" || validPID(pid)) &&
		validCustomParameters(input.CustomParameters) && validOptionalOpaque(input.SearchID, 1024)
}

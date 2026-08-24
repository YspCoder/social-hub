package vipunion

import (
	"context"

	"social-hub/pkg/socialhub"
)

const (
	goodsService               = "com.vip.adp.api.open.service.UnionGoodsV2Service"
	goodsSearchMethod          = "queryWithOauth"
	goodsLookupMethod          = "getByGoodsIdsV2WithOauth"
	marketingGoodsDetailMethod = "getGoodsDetailMarketingWithOauth"
)

type goodsSearchWire struct {
	Keyword                     string           `json:"keyword"`
	FieldName                   GoodsSortField   `json:"fieldName,omitempty"`
	Order                       *SortOrder       `json:"order,omitempty"`
	Page                        int              `json:"page"`
	PageSize                    int              `json:"pageSize"`
	RequestID                   string           `json:"requestId"`
	PriceStart                  string           `json:"priceStart,omitempty"`
	PriceEnd                    string           `json:"priceEnd,omitempty"`
	QueryReputation             *bool            `json:"queryReputation,omitempty"`
	QueryStoreServiceCapability *bool            `json:"queryStoreServiceCapability,omitempty"`
	QueryStock                  *bool            `json:"queryStock,omitempty"`
	QueryActivity               *bool            `json:"queryActivity,omitempty"`
	QueryPrepay                 *bool            `json:"queryPrepay,omitempty"`
	ChanTag                     string           `json:"chanTag"`
	QueryExclusiveCoupon        *bool            `json:"queryExclusiveCoupon,omitempty"`
	QueryCPSInfo                *CPSInfoFlags    `json:"queryCpsInfo,omitempty"`
	Research                    *bool            `json:"research,omitempty"`
	QueryFuturePrice            *bool            `json:"queryFuturePrice,omitempty"`
	OpenID                      string           `json:"openId"`
	RealCall                    bool             `json:"realCall"`
	ExtendSKU                   *bool            `json:"extendSku,omitempty"`
	PriceScene                  *GoodsPriceScene `json:"goodsPriceScene,omitempty"`
	QuerySuperPriceDown         *bool            `json:"querySuperPriceDown,omitempty"`
}

func (client *Client) SearchGoods(
	ctx context.Context,
	input GoodsSearchRequest,
	options ...socialhub.CallOption,
) (GoodsPage, error) {
	const operation = "search_goods"
	chanTag, openID := client.chanTag(input.ChanTag), client.openID(input.OpenID)
	if !validGoodsSearch(input, chanTag, openID) {
		return GoodsPage{}, invalidArgument(operation, "keyword, paging, channel identity, sort, price, or query options are invalid")
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
		return GoodsPage{}, err
	}
	body := struct {
		Request goodsSearchWire `json:"request"`
	}{Request: goodsSearchWire{
		Keyword: input.Keyword, FieldName: input.SortField, Order: input.SortOrder,
		Page: page, PageSize: pageSize, RequestID: requestID,
		PriceStart: input.PriceStart, PriceEnd: input.PriceEnd,
		QueryReputation:             input.QueryReputation,
		QueryStoreServiceCapability: input.QueryStoreServiceCapability,
		QueryStock:                  input.QueryStock, QueryActivity: input.QueryActivity,
		QueryPrepay: input.QueryPrepay, ChanTag: chanTag,
		QueryExclusiveCoupon: input.QueryExclusiveCoupon, QueryCPSInfo: input.QueryCPSInfo,
		Research: input.Research, QueryFuturePrice: input.QueryFuturePrice,
		OpenID: openID, RealCall: input.RealCall, ExtendSKU: input.ExtendSKU,
		PriceScene: input.PriceScene, QuerySuperPriceDown: input.QuerySuperPriceDown,
	}}
	var response struct {
		Goods            []Goods     `json:"goodsInfoList"`
		Total            int         `json:"total"`
		SortFields       []SortField `json:"sortFields"`
		Page             int         `json:"page"`
		PageSize         int         `json:"pageSize"`
		RecommendKeyword string      `json:"recommendKeyword"`
		BatchNumber      string      `json:"batchNo"`
	}
	meta, err := client.doJSON(
		ctx, operation, goodsService, goodsSearchMethod, requestID, body, &response, forwarded...,
	)
	if err != nil {
		return GoodsPage{}, err
	}
	return GoodsPage{
		Goods: response.Goods, Total: response.Total, Page: response.Page,
		PageSize: response.PageSize, SortFields: response.SortFields,
		RecommendKeyword: response.RecommendKeyword, BatchNumber: response.BatchNumber, Meta: meta,
	}, nil
}

type goodsLookupWire struct {
	GoodsIDs                    []string          `json:"goodsIds"`
	QueryDetail                 *bool             `json:"queryDetail,omitempty"`
	QueryStock                  *bool             `json:"queryStock,omitempty"`
	QueryReputation             *bool             `json:"queryReputation,omitempty"`
	QueryStoreServiceCapability *bool             `json:"queryStoreServiceCapability,omitempty"`
	QueryActivity               *bool             `json:"queryPMSAct,omitempty"`
	QueryPrepay                 *bool             `json:"queryPrepay,omitempty"`
	ChanTag                     string            `json:"chanTag"`
	ExtendBySPU                 *bool             `json:"extendBySpu,omitempty"`
	RequestID                   string            `json:"requestId"`
	SizeIDs                     map[string]string `json:"sizeIdMap,omitempty"`
	QueryExclusiveCoupon        *bool             `json:"queryExclusiveCoupon,omitempty"`
	ExtendSKU                   *bool             `json:"extendSku,omitempty"`
	QueryCPSInfo                *CPSInfoFlags     `json:"queryCpsInfo,omitempty"`
	QueryFuturePrice            *bool             `json:"queryFuturePrice,omitempty"`
	OpenID                      string            `json:"openId"`
	RealCall                    bool              `json:"realCall"`
	PriceScene                  *GoodsPriceScene  `json:"goodsPriceScene,omitempty"`
	QuerySuperPriceDown         *bool             `json:"querySuperPriceDown,omitempty"`
}

func (client *Client) GetGoods(
	ctx context.Context,
	input GoodsLookupRequest,
	options ...socialhub.CallOption,
) (GoodsResult, error) {
	const operation = "get_goods"
	chanTag, openID := client.chanTag(input.ChanTag), client.openID(input.OpenID)
	if !validGoodsLookup(input, chanTag, openID) {
		return GoodsResult{}, invalidArgument(operation, "goods IDs, channel identity, size IDs, or query options are invalid")
	}
	requestID, forwarded, err := prepareCallOptions(operation, options)
	if err != nil {
		return GoodsResult{}, err
	}
	body := struct {
		Request goodsLookupWire `json:"request"`
	}{Request: goodsLookupWire{
		GoodsIDs: input.GoodsIDs, QueryDetail: input.QueryDetail, QueryStock: input.QueryStock,
		QueryReputation:             input.QueryReputation,
		QueryStoreServiceCapability: input.QueryStoreServiceCapability,
		QueryActivity:               input.QueryActivity, QueryPrepay: input.QueryPrepay, ChanTag: chanTag,
		ExtendBySPU: input.ExtendBySPU, RequestID: requestID, SizeIDs: input.SizeIDs,
		QueryExclusiveCoupon: input.QueryExclusiveCoupon, ExtendSKU: input.ExtendSKU,
		QueryCPSInfo: input.QueryCPSInfo, QueryFuturePrice: input.QueryFuturePrice,
		OpenID: openID, RealCall: input.RealCall, PriceScene: input.PriceScene,
		QuerySuperPriceDown: input.QuerySuperPriceDown,
	}}
	var response []Goods
	meta, err := client.doJSON(
		ctx, operation, goodsService, goodsLookupMethod, requestID, body, &response, forwarded...,
	)
	if err != nil {
		return GoodsResult{}, err
	}
	return GoodsResult{Goods: response, Meta: meta}, nil
}

type marketingGoodsWire struct {
	GoodsID                     string            `json:"goodsId"`
	QueryDetail                 *bool             `json:"queryDetail,omitempty"`
	QueryReputation             *bool             `json:"queryReputation,omitempty"`
	QueryStoreServiceCapability *bool             `json:"queryStoreServiceCapability,omitempty"`
	QueryStock                  *bool             `json:"queryStock,omitempty"`
	QueryPrepay                 *bool             `json:"queryPrepay,omitempty"`
	ChanTag                     string            `json:"chanTag"`
	ExtendBySPU                 *bool             `json:"extendBySpu,omitempty"`
	RequestID                   string            `json:"requestId"`
	SizeIDs                     map[string]string `json:"sizeIdMap,omitempty"`
	ExtendSKU                   *bool             `json:"extendSku,omitempty"`
	QueryCPSInfo                *CPSInfoFlags     `json:"queryCpsInfo,omitempty"`
	OpenID                      string            `json:"openId"`
	RealCall                    bool              `json:"realCall"`
	PriceScene                  *GoodsPriceScene  `json:"goodsPriceScene,omitempty"`
}

func (client *Client) GetMarketingGoods(
	ctx context.Context,
	input MarketingGoodsRequest,
	options ...socialhub.CallOption,
) (MarketingGoodsResult, error) {
	const operation = "get_marketing_goods"
	chanTag, openID := client.chanTag(input.ChanTag), client.openID(input.OpenID)
	if !validMarketingGoods(input, chanTag, openID) {
		return MarketingGoodsResult{}, invalidArgument(operation, "goods ID, channel identity, size ID, or query options are invalid")
	}
	requestID, forwarded, err := prepareCallOptions(operation, options)
	if err != nil {
		return MarketingGoodsResult{}, err
	}
	var sizeIDs map[string]string
	if input.SizeID != "" {
		sizeIDs = map[string]string{input.GoodsID: input.SizeID}
	}
	body := struct {
		Request marketingGoodsWire `json:"request"`
	}{Request: marketingGoodsWire{
		GoodsID: input.GoodsID, QueryDetail: input.QueryDetail,
		QueryReputation:             input.QueryReputation,
		QueryStoreServiceCapability: input.QueryStoreServiceCapability,
		QueryStock:                  input.QueryStock, QueryPrepay: input.QueryPrepay, ChanTag: chanTag,
		ExtendBySPU: input.ExtendBySPU, RequestID: requestID, SizeIDs: sizeIDs,
		ExtendSKU: input.ExtendSKU, QueryCPSInfo: input.QueryCPSInfo,
		OpenID: openID, RealCall: input.RealCall, PriceScene: input.PriceScene,
	}}
	var response Goods
	meta, err := client.doJSON(
		ctx, operation, goodsService, marketingGoodsDetailMethod, requestID, body, &response, forwarded...,
	)
	if err != nil {
		return MarketingGoodsResult{}, err
	}
	return MarketingGoodsResult{Goods: response, Meta: meta}, nil
}

func validGoodsSearch(input GoodsSearchRequest, chanTag, openID string) bool {
	if !validOpaque(input.Keyword, 512) || input.Page < 0 || input.PageSize < 0 || input.PageSize > 50 ||
		!validChanTag(chanTag) || !validOpenID(openID) || !validPriceRange(input.PriceStart, input.PriceEnd) ||
		!validSort(input.SortField, input.SortOrder) || !validQueryOptions(input.QueryCPSInfo, input.PriceScene) {
		return false
	}
	return true
}

func validGoodsLookup(input GoodsLookupRequest, chanTag, openID string) bool {
	return validGoodsIDs(input.GoodsIDs, 10) && validChanTag(chanTag) && validOpenID(openID) &&
		validSizeIDs(input.SizeIDs, input.GoodsIDs) && validQueryOptions(input.QueryCPSInfo, input.PriceScene)
}

func validMarketingGoods(input MarketingGoodsRequest, chanTag, openID string) bool {
	return validGoodsID(input.GoodsID) && validChanTag(chanTag) && validOpenID(openID) &&
		(input.SizeID == "" || validGoodsID(input.SizeID)) && validQueryOptions(input.QueryCPSInfo, input.PriceScene)
}

func validSort(field GoodsSortField, order *SortOrder) bool {
	if field == "" {
		return order == nil
	}
	switch field {
	case GoodsSortPrice, GoodsSortDiscount, GoodsSortSales, GoodsSortCommissionRate, GoodsSortCommission:
	default:
		return false
	}
	return order == nil || *order == SortAscending || *order == SortDescending
}

func validQueryOptions(cps *CPSInfoFlags, scene *GoodsPriceScene) bool {
	if cps != nil && (*cps < CPSInfoNone || *cps > CPSInfoAll) {
		return false
	}
	return scene == nil || *scene >= GoodsPricePublicBest && *scene <= GoodsPricePublic
}

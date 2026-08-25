package shopeeads

import (
	"context"
	"math"
	"net/url"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

const (
	pathTotalBalance                     = "/api/v2/ads/get_total_balance"
	pathShopToggleInfo                   = "/api/v2/ads/get_shop_toggle_info"
	pathRecommendedKeywords              = "/api/v2/ads/get_recommended_keyword_list"
	pathRecommendedItems                 = "/api/v2/ads/get_recommended_item_list"
	pathAllCPCHourlyPerformance          = "/api/v2/ads/get_all_cpc_ads_hourly_performance"
	pathAllCPCDailyPerformance           = "/api/v2/ads/get_all_cpc_ads_daily_performance"
	pathProductCampaignDailyPerformance  = "/api/v2/ads/get_product_campaign_daily_performance"
	pathProductCampaignHourlyPerformance = "/api/v2/ads/get_product_campaign_hourly_performance"
	pathProductCampaignIDs               = "/api/v2/ads/get_product_level_campaign_id_list"
	pathProductCampaignSettings          = "/api/v2/ads/get_product_level_campaign_setting_info"
)

func (client *Client) GetTotalBalance(ctx context.Context, options ...socialhub.CallOption) (*Balance, error) {
	const operation = "balance_get"
	var result Balance
	meta, err := client.doJSON(ctx, operation, pathTotalBalance, nil, &result, options...)
	if err != nil {
		return nil, err
	}
	if result.DataTimestamp <= 0 || !result.TotalBalance.IsNumber() {
		return nil, platformContractError(operation, "Shopee returned an invalid Ads balance")
	}
	result.Meta = meta
	return &result, nil
}

func (client *Client) GetShopToggleInfo(ctx context.Context, options ...socialhub.CallOption) (*ShopToggleInfo, error) {
	const operation = "shop_toggle_get"
	var result ShopToggleInfo
	meta, err := client.doJSON(ctx, operation, pathShopToggleInfo, nil, &result, options...)
	if err != nil {
		return nil, err
	}
	if result.DataTimestamp <= 0 {
		return nil, platformContractError(operation, "Shopee returned invalid shop toggle data")
	}
	result.Meta = meta
	return &result, nil
}

func (client *Client) GetRecommendedKeywords(ctx context.Context, itemID int64, inputKeyword string, options ...socialhub.CallOption) (*KeywordRecommendations, error) {
	const operation = "recommended_keywords_get"
	if !validItemID(itemID) || !validOptionalText(inputKeyword, 1024) {
		return nil, invalidArgument(operation, "a positive item_id and a bounded optional input keyword are required")
	}
	query := url.Values{"item_id": {formatID(itemID)}}
	if inputKeyword != "" {
		query.Set("input_keyword", inputKeyword)
	}
	var result KeywordRecommendations
	meta, err := client.doJSON(ctx, operation, pathRecommendedKeywords, query, &result, options...)
	if err != nil {
		return nil, err
	}
	if result.ItemID != itemID || result.InputKeyword != inputKeyword || result.SuggestedKeywords == nil {
		return nil, platformContractError(operation, "Shopee returned invalid keyword recommendations")
	}
	seen := make(map[string]struct{}, len(result.SuggestedKeywords))
	for _, keyword := range result.SuggestedKeywords {
		if !validOpaque(keyword.Keyword, 4096) || utf8.RuneCountInString(keyword.Keyword) > 1024 ||
			keyword.QualityScore < 0 || keyword.QualityScore > math.MaxInt32 ||
			keyword.SearchVolume < 0 || keyword.SearchVolume > math.MaxInt32 || !keyword.SuggestedBid.IsNumber() {
			return nil, platformContractError(operation, "Shopee returned an invalid suggested keyword")
		}
		if _, duplicate := seen[keyword.Keyword]; duplicate {
			return nil, platformContractError(operation, "Shopee returned duplicate suggested keywords")
		}
		seen[keyword.Keyword] = struct{}{}
	}
	result.Meta = meta
	return &result, nil
}

func (client *Client) GetRecommendedItems(ctx context.Context, options ...socialhub.CallOption) (RecommendedItems, error) {
	const operation = "recommended_items_get"
	var items []RecommendedItem
	meta, err := client.doJSON(ctx, operation, pathRecommendedItems, nil, &items, options...)
	if err != nil {
		return RecommendedItems{}, err
	}
	if items == nil {
		return RecommendedItems{}, platformContractError(operation, "Shopee omitted the recommended item list")
	}
	seen := make(map[int64]struct{}, len(items))
	for _, item := range items {
		if !validItemID(item.ItemID) || !validTextList(item.ItemStatusList, 128, 256) ||
			!validTextList(item.SKUTagList, 128, 256) || !validTextList(item.OngoingAdTypeList, 128, 256) {
			return RecommendedItems{}, platformContractError(operation, "Shopee returned an invalid recommended item")
		}
		if _, duplicate := seen[item.ItemID]; duplicate {
			return RecommendedItems{}, platformContractError(operation, "Shopee returned duplicate recommended items")
		}
		seen[item.ItemID] = struct{}{}
	}
	return RecommendedItems{Items: items, Meta: meta}, nil
}

func (client *Client) GetAllCPCHourlyPerformance(ctx context.Context, performanceDate string, options ...socialhub.CallOption) (CPCPerformanceResult, error) {
	const operation = "cpc_hourly_performance_get"
	if !validDate(performanceDate) {
		return CPCPerformanceResult{}, invalidArgument(operation, "performance_date must use DD-MM-YYYY")
	}
	return client.cpcPerformance(ctx, operation, pathAllCPCHourlyPerformance, url.Values{
		"performance_date": {performanceDate},
	}, performanceDate, performanceDate, true, options...)
}

func (client *Client) GetAllCPCDailyPerformance(ctx context.Context, startDate, endDate string, options ...socialhub.CallOption) (CPCPerformanceResult, error) {
	const operation = "cpc_daily_performance_get"
	if !validDateRange(startDate, endDate) {
		return CPCPerformanceResult{}, invalidArgument(operation, "start_date and end_date must be an ordered DD-MM-YYYY range")
	}
	return client.cpcPerformance(ctx, operation, pathAllCPCDailyPerformance, url.Values{
		"start_date": {startDate}, "end_date": {endDate},
	}, startDate, endDate, false, options...)
}

func (client *Client) cpcPerformance(ctx context.Context, operation, path string, query url.Values, startDate, endDate string, hourly bool, options ...socialhub.CallOption) (CPCPerformanceResult, error) {
	var rows []CPCPerformance
	meta, err := client.doJSON(ctx, operation, path, query, &rows, options...)
	if err != nil {
		return CPCPerformanceResult{}, err
	}
	if rows == nil {
		return CPCPerformanceResult{}, platformContractError(operation, "Shopee omitted CPC performance rows")
	}
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		if !dateWithin(row.Date, startDate, endDate) ||
			hourly && (row.Hour == nil || *row.Hour < 0 || *row.Hour > 23) || !hourly && row.Hour != nil ||
			!validCPCNumbers(row) {
			return CPCPerformanceResult{}, platformContractError(operation, "Shopee returned an invalid CPC performance row")
		}
		key := row.Date
		if row.Hour != nil {
			key += "/" + formatID(int64(*row.Hour))
		}
		if _, duplicate := seen[key]; duplicate {
			return CPCPerformanceResult{}, platformContractError(operation, "Shopee returned duplicate CPC performance rows")
		}
		seen[key] = struct{}{}
	}
	return CPCPerformanceResult{Rows: rows, Meta: meta}, nil
}

func (client *Client) GetProductCampaignDailyPerformance(ctx context.Context, input CampaignPerformanceRequest, options ...socialhub.CallOption) (ProductCampaignPerformanceResult, error) {
	const operation = "product_campaign_daily_performance_get"
	if !validIDs(input.CampaignIDs, 100) || !validDateRange(input.StartDate, input.EndDate) || input.PerformanceDate != "" {
		return ProductCampaignPerformanceResult{}, invalidArgument(operation, "1-100 Campaign IDs and an ordered DD-MM-YYYY date range are required")
	}
	return client.productCampaignPerformance(ctx, operation, pathProductCampaignDailyPerformance, url.Values{
		"campaign_id_list": {joinIDs(input.CampaignIDs)}, "start_date": {input.StartDate}, "end_date": {input.EndDate},
	}, input.CampaignIDs, input.StartDate, input.EndDate, false, options...)
}

func (client *Client) GetProductCampaignHourlyPerformance(ctx context.Context, input CampaignPerformanceRequest, options ...socialhub.CallOption) (ProductCampaignPerformanceResult, error) {
	const operation = "product_campaign_hourly_performance_get"
	if !validIDs(input.CampaignIDs, 100) || !validDate(input.PerformanceDate) || input.StartDate != "" || input.EndDate != "" {
		return ProductCampaignPerformanceResult{}, invalidArgument(operation, "1-100 Campaign IDs and performance_date in DD-MM-YYYY are required")
	}
	return client.productCampaignPerformance(ctx, operation, pathProductCampaignHourlyPerformance, url.Values{
		"campaign_id_list": {joinIDs(input.CampaignIDs)}, "performance_date": {input.PerformanceDate},
	}, input.CampaignIDs, input.PerformanceDate, input.PerformanceDate, true, options...)
}

func (client *Client) productCampaignPerformance(ctx context.Context, operation, path string, query url.Values, requestedIDs []int64, startDate, endDate string, hourly bool, options ...socialhub.CallOption) (ProductCampaignPerformanceResult, error) {
	var shops []ProductCampaignPerformanceShop
	meta, err := client.doJSON(ctx, operation, path, query, &shops, options...)
	if err != nil {
		return ProductCampaignPerformanceResult{}, err
	}
	if shops == nil {
		return ProductCampaignPerformanceResult{}, platformContractError(operation, "Shopee omitted product Campaign performance data")
	}
	seenShops := make(map[int64]struct{}, len(shops))
	for _, shop := range shops {
		if shop.ShopID != client.shopID || !validOpaque(shop.Region, 32) || shop.Campaigns == nil {
			return ProductCampaignPerformanceResult{}, platformContractError(operation, "Shopee returned unbound product Campaign performance data")
		}
		if _, duplicate := seenShops[shop.ShopID]; duplicate {
			return ProductCampaignPerformanceResult{}, platformContractError(operation, "Shopee returned duplicate shop performance data")
		}
		seenShops[shop.ShopID] = struct{}{}
		seenCampaigns := make(map[int64]struct{}, len(shop.Campaigns))
		for _, campaign := range shop.Campaigns {
			if campaign.CampaignID <= 0 || !containsID(requestedIDs, campaign.CampaignID) || campaign.Metrics == nil ||
				!validCampaignAdType(campaign.AdType) || !validCampaignPlacement(campaign.CampaignPlacement) ||
				!validOptionalText(campaign.AdName, 1024) {
				return ProductCampaignPerformanceResult{}, platformContractError(operation, "Shopee returned an invalid product Campaign")
			}
			if _, duplicate := seenCampaigns[campaign.CampaignID]; duplicate {
				return ProductCampaignPerformanceResult{}, platformContractError(operation, "Shopee returned duplicate product Campaigns")
			}
			seenCampaigns[campaign.CampaignID] = struct{}{}
			seenMetrics := make(map[string]struct{}, len(campaign.Metrics))
			for _, metric := range campaign.Metrics {
				if !dateWithin(metric.Date, startDate, endDate) ||
					hourly && (metric.Hour == nil || *metric.Hour < 0 || *metric.Hour > 23) || !hourly && metric.Hour != nil ||
					!validProductCampaignNumbers(metric) {
					return ProductCampaignPerformanceResult{}, platformContractError(operation, "Shopee returned an invalid product Campaign metric")
				}
				key := metric.Date
				if metric.Hour != nil {
					key += "/" + formatID(int64(*metric.Hour))
				}
				if _, duplicate := seenMetrics[key]; duplicate {
					return ProductCampaignPerformanceResult{}, platformContractError(operation, "Shopee returned duplicate product Campaign metrics")
				}
				seenMetrics[key] = struct{}{}
			}
		}
	}
	return ProductCampaignPerformanceResult{Shops: shops, Meta: meta}, nil
}

func (client *Client) ListProductCampaignIDs(ctx context.Context, input CampaignIDListRequest, options ...socialhub.CallOption) (*CampaignIDList, error) {
	const operation = "product_campaign_ids_list"
	if !validAdType(input.AdType) || input.Offset < 0 || input.Limit < 0 {
		return nil, invalidArgument(operation, "ad_type, offset, or limit is invalid")
	}
	query := make(url.Values)
	if input.AdType != "" {
		query.Set("ad_type", input.AdType)
	}
	if input.Offset > 0 {
		query.Set("offset", formatID(input.Offset))
	}
	if input.Limit > 0 {
		query.Set("limit", formatID(input.Limit))
	}
	var result CampaignIDList
	meta, err := client.doJSON(ctx, operation, pathProductCampaignIDs, query, &result, options...)
	if err != nil {
		return nil, err
	}
	if result.ShopID != client.shopID || !validOpaque(result.Region, 32) || result.Campaigns == nil {
		return nil, platformContractError(operation, "Shopee returned invalid or unbound Campaign IDs")
	}
	if input.Limit > 0 && int64(len(result.Campaigns)) > input.Limit {
		return nil, platformContractError(operation, "Shopee returned more Campaign IDs than requested")
	}
	seen := make(map[int64]struct{}, len(result.Campaigns))
	for _, campaign := range result.Campaigns {
		if campaign.CampaignID <= 0 || !validAdType(campaign.AdType) || campaign.AdType == "" || campaign.AdType == "all" {
			return nil, platformContractError(operation, "Shopee returned an invalid Campaign ID record")
		}
		if input.AdType == "auto" || input.AdType == "manual" {
			if campaign.AdType != input.AdType {
				return nil, platformContractError(operation, "Shopee returned a Campaign outside the requested ad_type")
			}
		}
		if _, exists := seen[campaign.CampaignID]; exists {
			return nil, platformContractError(operation, "Shopee returned duplicate Campaign IDs")
		}
		seen[campaign.CampaignID] = struct{}{}
	}
	result.Meta = meta
	return &result, nil
}

func (client *Client) GetProductCampaignSettings(ctx context.Context, input CampaignSettingsRequest, options ...socialhub.CallOption) (*CampaignSettings, error) {
	const operation = "product_campaign_settings_get"
	if !validIDs(input.CampaignIDs, 100) || !validInfoTypes(input.InfoTypes) {
		return nil, invalidArgument(operation, "1-100 Campaign IDs and unique info types 1-4 are required")
	}
	query := url.Values{
		"campaign_id_list": {joinIDs(input.CampaignIDs)}, "info_type_list": {joinInfoTypes(input.InfoTypes)},
	}
	var result CampaignSettings
	meta, err := client.doJSON(ctx, operation, pathProductCampaignSettings, query, &result, options...)
	if err != nil {
		return nil, err
	}
	if result.ShopID != client.shopID || !validOpaque(result.Region, 32) || result.Campaigns == nil {
		return nil, platformContractError(operation, "Shopee returned invalid or unbound Campaign settings")
	}
	seen := make(map[int64]struct{}, len(result.Campaigns))
	for _, campaign := range result.Campaigns {
		if campaign.CampaignID <= 0 || !containsID(input.CampaignIDs, campaign.CampaignID) {
			return nil, platformContractError(operation, "Shopee returned an invalid Campaign setting")
		}
		if _, exists := seen[campaign.CampaignID]; exists {
			return nil, platformContractError(operation, "Shopee returned duplicate Campaign settings")
		}
		seen[campaign.CampaignID] = struct{}{}
		if !validCampaignSetting(campaign, input.InfoTypes) {
			return nil, platformContractError(operation, "Shopee returned invalid or incomplete Campaign setting details")
		}
	}
	result.Meta = meta
	return &result, nil
}

func allNumbers(values ...ExactValue) bool {
	for _, value := range values {
		if !value.IsNumber() {
			return false
		}
	}
	return true
}

func validCPCNumbers(row CPCPerformance) bool {
	return allNumbers(
		row.Impression, row.Clicks, row.CTR, row.DirectOrder, row.BroadOrder,
		row.DirectConversions, row.BroadConversions, row.DirectItemSold, row.BroadItemSold,
		row.DirectGMV, row.BroadGMV, row.Expense, row.CostPerConversion,
		row.DirectROAS, row.BroadROAS,
	)
}

func validProductCampaignNumbers(metric ProductCampaignMetric) bool {
	return allNumbers(
		metric.Impression, metric.Clicks, metric.CTR, metric.Expense, metric.BroadGMV,
		metric.BroadOrder, metric.BroadOrderAmount, metric.BroadROI, metric.BroadCIR,
		metric.CR, metric.CPC, metric.DirectOrder, metric.DirectOrderAmount, metric.DirectGMV,
		metric.DirectROI, metric.DirectCIR, metric.DirectCR, metric.CPDC,
	)
}

func validCampaignSetting(campaign ProductCampaignSettings, requested []CampaignInfoType) bool {
	if campaignInfoRequested(requested, CampaignInfoCommon) && campaign.CommonInfo == nil ||
		campaignInfoRequested(requested, CampaignInfoManualBid) && campaign.ManualBiddingInfo == nil ||
		campaignInfoRequested(requested, CampaignInfoAutoBid) && campaign.AutoBiddingInfo == nil ||
		campaignInfoRequested(requested, CampaignInfoAutoProduct) && campaign.AutoProductAds == nil {
		return false
	}
	if campaign.CommonInfo != nil && !validCommonCampaignInfo(*campaign.CommonInfo) {
		return false
	}
	if campaign.ManualBiddingInfo != nil && !validManualBiddingInfo(*campaign.ManualBiddingInfo) {
		return false
	}
	if campaign.AutoBiddingInfo != nil && !campaign.AutoBiddingInfo.ROASTarget.IsNumber() {
		return false
	}
	if campaign.AutoProductAds != nil && !validAutoProductAds(campaign.AutoProductAds) {
		return false
	}
	return true
}

func campaignInfoRequested(values []CampaignInfoType, target CampaignInfoType) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func validCommonCampaignInfo(info CampaignCommonInfo) bool {
	if !validCampaignAdType(info.AdType) || !validOptionalText(info.AdName, 1024) ||
		!oneOf(info.CampaignStatus, "ongoing", "scheduled", "ended", "paused", "deleted", "closed") ||
		!oneOf(info.BiddingMethod, "auto", "manual") || !validCampaignPlacement(info.CampaignPlacement) ||
		!info.CampaignBudget.IsNumber() || info.CampaignDuration.StartTime <= 0 ||
		info.CampaignDuration.EndTime < 0 || info.CampaignDuration.EndTime > 0 && info.CampaignDuration.EndTime < info.CampaignDuration.StartTime {
		return false
	}
	return validOptionalIDs(info.ItemIDs, 10_000)
}

func validManualBiddingInfo(info ManualBiddingInfo) bool {
	if info.SelectedKeywords == nil || info.DiscoveryAdsLocations == nil ||
		len(info.SelectedKeywords) > 10_000 || len(info.DiscoveryAdsLocations) > 128 {
		return false
	}
	seenKeywords := make(map[string]struct{}, len(info.SelectedKeywords))
	for _, keyword := range info.SelectedKeywords {
		if !validOpaque(keyword.Keyword, 4096) || utf8.RuneCountInString(keyword.Keyword) > 1024 ||
			!oneOf(keyword.Status, "deleted", "normal", "reserved", "blacklist") ||
			!oneOf(keyword.MatchType, "exact", "broad") || !keyword.BidPricePerClick.IsNumber() {
			return false
		}
		key := keyword.Keyword + "\x00" + keyword.MatchType
		if _, duplicate := seenKeywords[key]; duplicate {
			return false
		}
		seenKeywords[key] = struct{}{}
	}
	seenLocations := make(map[string]struct{}, len(info.DiscoveryAdsLocations))
	for _, location := range info.DiscoveryAdsLocations {
		if !validOpaque(location.Location, 256) || !validOpaque(location.Status, 256) || !location.BidPrice.IsNumber() {
			return false
		}
		if _, duplicate := seenLocations[location.Location]; duplicate {
			return false
		}
		seenLocations[location.Location] = struct{}{}
	}
	return true
}

func validAutoProductAds(values []AutoProductAdsInfo) bool {
	if len(values) > 10_000 {
		return false
	}
	seen := make(map[int64]struct{}, len(values))
	for _, value := range values {
		if !validOptionalText(value.ProductName, 1024) ||
			!oneOf(value.Status, "learning", "ongoing", "paused", "ended", "unavailable") || value.ItemID <= 0 {
			return false
		}
		if _, duplicate := seen[value.ItemID]; duplicate {
			return false
		}
		seen[value.ItemID] = struct{}{}
	}
	return true
}

func validOptionalIDs(values []int64, maximum int) bool {
	if values == nil || len(values) > maximum {
		return false
	}
	seen := make(map[int64]struct{}, len(values))
	for _, value := range values {
		if value <= 0 {
			return false
		}
		if _, duplicate := seen[value]; duplicate {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

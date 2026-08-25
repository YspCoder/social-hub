package mercadoads

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	advertisersPath = "/advertising/advertisers"
	campaignsPath   = "/advertising/product_ads/campaigns/"
	itemsPath       = "/advertising/product_ads/items/"
)

func (client *Client) ListAdvertisers(ctx context.Context, options ...socialhub.CallOption) (AdvertiserList, error) {
	const operation = "advertisers_list"
	var result AdvertiserList
	meta, err := client.doJSON(ctx, operation, advertisersPath, url.Values{"product_id": {"PADS"}}, "1", &result, options...)
	if err != nil {
		return AdvertiserList{}, err
	}
	if result.Advertisers == nil {
		return AdvertiserList{}, platformContractError(operation, "Mercado Libre omitted the advertiser list")
	}
	seen := make(map[int64]struct{}, len(result.Advertisers))
	for _, advertiser := range result.Advertisers {
		if advertiser.AdvertiserID <= 0 || !validOpaque(advertiser.SiteID, 32) ||
			!validOpaque(advertiser.AdvertiserName, 1024) || !validOpaque(advertiser.AccountName, 1024) {
			return AdvertiserList{}, platformContractError(operation, "Mercado Libre returned an invalid advertiser")
		}
		if _, duplicate := seen[advertiser.AdvertiserID]; duplicate {
			return AdvertiserList{}, platformContractError(operation, "Mercado Libre returned duplicate advertiser IDs")
		}
		seen[advertiser.AdvertiserID] = struct{}{}
	}
	result.Meta = meta
	return result, nil
}

func (client *Client) ListCampaigns(ctx context.Context, input CampaignListRequest, options ...socialhub.CallOption) (CampaignPage, error) {
	const operation = "campaigns_list"
	if client.advertiserID <= 0 || !validCampaignListRequest(input, false) {
		return CampaignPage{}, invalidArgument(operation, "configured advertiser_id, pagination, filters, or aggregate metrics are invalid")
	}
	var result CampaignPage
	meta, err := client.doJSON(ctx, operation, advertiserCampaignsPath(client.advertiserID), campaignListQuery(input), "2", &result, options...)
	if err != nil {
		return CampaignPage{}, err
	}
	if !validPaging(result.Paging, len(result.Results), input.Limit, input.Offset) || result.Results == nil ||
		!validCampaigns(result.Results, input.Metrics) ||
		input.MetricsSummary && !validMetrics(result.MetricsSummary, input.Metrics) {
		return CampaignPage{}, platformContractError(operation, "Mercado Libre returned invalid Campaign search data")
	}
	result.Meta = meta
	return result, nil
}

func (client *Client) ListCampaignDailyMetrics(ctx context.Context, input CampaignListRequest, options ...socialhub.CallOption) (DailyMetricsPage, error) {
	const operation = "campaign_daily_metrics_list"
	if client.advertiserID <= 0 || !validCampaignListRequest(input, true) {
		return DailyMetricsPage{}, invalidArgument(operation, "configured advertiser_id, pagination, filters, and required daily metrics are invalid")
	}
	query := campaignListQuery(input)
	query.Set("aggregation_type", "DAILY")
	var result DailyMetricsPage
	meta, err := client.doJSON(ctx, operation, advertiserCampaignsPath(client.advertiserID), query, "2", &result, options...)
	if err != nil {
		return DailyMetricsPage{}, err
	}
	if !validPaging(result.Paging, len(result.Results), input.Limit, input.Offset) ||
		!validDailyMetrics(result.Results, input.DateFrom, input.DateTo, input.Metrics) {
		return DailyMetricsPage{}, platformContractError(operation, "Mercado Libre returned invalid daily Campaign metrics")
	}
	result.Meta = meta
	return result, nil
}

func (client *Client) GetCampaign(ctx context.Context, campaignID int64, input MetricQuery, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_get"
	if campaignID <= 0 {
		return nil, invalidArgument(operation, "campaign_id must be positive")
	}
	if err := validateMetricQuery(operation, input, campaignDetailMetrics, false); err != nil {
		return nil, err
	}
	var result Campaign
	meta, err := client.doJSON(ctx, operation, campaignsPath+formatInt64(campaignID), metricQuery(input), "2", &result, options...)
	if err != nil {
		return nil, err
	}
	if result.ID != campaignID || !validCampaign(result, input.Metrics) {
		return nil, platformContractError(operation, "Mercado Libre returned an invalid or unbound Campaign")
	}
	result.Meta = meta
	return &result, nil
}

func (client *Client) GetCampaignDailyMetrics(ctx context.Context, campaignID int64, input MetricQuery, options ...socialhub.CallOption) ([]DailyMetrics, ResponseMeta, error) {
	const operation = "campaign_daily_metrics_get"
	if campaignID <= 0 {
		return nil, ResponseMeta{}, invalidArgument(operation, "campaign_id must be positive")
	}
	if err := validateMetricQuery(operation, input, campaignDetailMetrics, true); err != nil {
		return nil, ResponseMeta{}, err
	}
	query := metricQuery(input)
	query.Set("aggregation_type", "DAILY")
	var result dailyEnvelope
	meta, err := client.doJSON(ctx, operation, campaignsPath+formatInt64(campaignID), query, "2", &result, options...)
	if err != nil {
		return nil, ResponseMeta{}, err
	}
	if !validDailyMetrics(result.Results, input.DateFrom, input.DateTo, input.Metrics) {
		return nil, ResponseMeta{}, platformContractError(operation, "Mercado Libre returned invalid daily Campaign metrics")
	}
	return result.Results, meta, nil
}

func (client *Client) ListItems(ctx context.Context, input ItemListRequest, options ...socialhub.CallOption) (ItemPage, error) {
	const operation = "items_list"
	if client.advertiserID <= 0 || !validItemListRequest(input, false) {
		return ItemPage{}, invalidArgument(operation, "configured advertiser_id, pagination, filters, or aggregate metrics are invalid")
	}
	var result ItemPage
	meta, err := client.doJSON(ctx, operation, advertiserItemsPath(client.advertiserID), itemListQuery(input), "2", &result, options...)
	if err != nil {
		return ItemPage{}, err
	}
	if !validPaging(result.Paging, len(result.Results), input.Limit, input.Offset) || result.Results == nil ||
		!validItems(result.Results, input.Metrics) ||
		input.MetricsSummary && !validMetrics(result.MetricsSummary, input.Metrics) {
		return ItemPage{}, platformContractError(operation, "Mercado Libre returned invalid Product Ads item search data")
	}
	result.Meta = meta
	return result, nil
}

func (client *Client) ListItemDailyMetrics(ctx context.Context, input ItemListRequest, options ...socialhub.CallOption) (DailyMetricsPage, error) {
	const operation = "item_daily_metrics_list"
	if client.advertiserID <= 0 || !validItemListRequest(input, true) {
		return DailyMetricsPage{}, invalidArgument(operation, "configured advertiser_id, pagination, filters, and required daily metrics are invalid")
	}
	query := itemListQuery(input)
	query.Set("aggregation_type", "DAILY")
	var result DailyMetricsPage
	meta, err := client.doJSON(ctx, operation, advertiserItemsPath(client.advertiserID), query, "2", &result, options...)
	if err != nil {
		return DailyMetricsPage{}, err
	}
	if !validPaging(result.Paging, len(result.Results), input.Limit, input.Offset) ||
		!validDailyMetrics(result.Results, input.DateFrom, input.DateTo, input.Metrics) {
		return DailyMetricsPage{}, platformContractError(operation, "Mercado Libre returned invalid daily item metrics")
	}
	result.Meta = meta
	return result, nil
}

func (client *Client) GetItem(ctx context.Context, itemID string, options ...socialhub.CallOption) (*Item, error) {
	return client.getItem(ctx, "item_get", itemID, MetricQuery{}, false, options...)
}

func (client *Client) GetItemMetrics(ctx context.Context, itemID string, input MetricQuery, options ...socialhub.CallOption) (*Item, error) {
	return client.getItem(ctx, "item_metrics_get", itemID, input, true, options...)
}

func (client *Client) getItem(ctx context.Context, operation, itemID string, input MetricQuery, requireMetrics bool, options ...socialhub.CallOption) (*Item, error) {
	if !validOpaque(itemID, 256) || strings.ContainsAny(itemID, "/?#") {
		return nil, invalidArgument(operation, "item_id is invalid")
	}
	if err := validateMetricQuery(operation, input, itemDetailMetrics, requireMetrics); err != nil {
		return nil, err
	}
	var result Item
	meta, err := client.doJSON(ctx, operation, itemsPath+url.PathEscape(itemID), metricQuery(input), "2", &result, options...)
	if err != nil {
		return nil, err
	}
	if result.ItemID != itemID || !validItem(result, nil) {
		return nil, platformContractError(operation, "Mercado Libre returned an invalid or unbound Product Ads item")
	}
	if requireMetrics && !validMetrics(result.Metrics, input.Metrics) && !validMetrics(result.MetricsSummary, input.Metrics) {
		return nil, platformContractError(operation, "Mercado Libre omitted requested item metrics")
	}
	result.Meta = meta
	return &result, nil
}

func (client *Client) GetItemDailyMetrics(ctx context.Context, itemID string, input MetricQuery, options ...socialhub.CallOption) ([]DailyMetrics, ResponseMeta, error) {
	const operation = "item_daily_metrics_get"
	if !validOpaque(itemID, 256) || strings.ContainsAny(itemID, "/?#") {
		return nil, ResponseMeta{}, invalidArgument(operation, "item_id is invalid")
	}
	if err := validateMetricQuery(operation, input, itemDetailMetrics, true); err != nil {
		return nil, ResponseMeta{}, err
	}
	query := metricQuery(input)
	query.Set("aggregation_type", "DAILY")
	var result dailyEnvelope
	meta, err := client.doJSON(ctx, operation, itemsPath+url.PathEscape(itemID), query, "2", &result, options...)
	if err != nil {
		return nil, ResponseMeta{}, err
	}
	if !validDailyMetrics(result.Results, input.DateFrom, input.DateTo, input.Metrics) {
		return nil, ResponseMeta{}, platformContractError(operation, "Mercado Libre returned invalid daily item metrics")
	}
	return result.Results, meta, nil
}

func advertiserCampaignsPath(advertiserID int64) string {
	return "/advertising/advertisers/" + formatInt64(advertiserID) + "/product_ads/campaigns"
}

func advertiserItemsPath(advertiserID int64) string {
	return "/advertising/advertisers/" + formatInt64(advertiserID) + "/product_ads/items"
}

func campaignListQuery(input CampaignListRequest) url.Values {
	query := metricQuery(MetricQuery{DateFrom: input.DateFrom, DateTo: input.DateTo, Metrics: input.Metrics})
	setPagination(query, input.Limit, input.Offset)
	if input.MetricsSummary {
		query.Set("metrics_summary", "true")
	}
	setInt64Filter(query, "campaign_ids", input.CampaignIDs)
	if len(input.Statuses) > 0 {
		values := make([]string, len(input.Statuses))
		for index, status := range input.Statuses {
			values[index] = string(status)
		}
		query.Set("filters[status]", strings.Join(values, ","))
	}
	if input.Channel != "" {
		query.Set("filters[channel]", input.Channel)
	}
	return query
}

func itemListQuery(input ItemListRequest) url.Values {
	query := metricQuery(MetricQuery{DateFrom: input.DateFrom, DateTo: input.DateTo, Metrics: input.Metrics})
	setPagination(query, input.Limit, input.Offset)
	if input.MetricsSummary {
		query.Set("metrics_summary", "true")
	}
	filter := input.Filters
	setStringFilter(query, "item_id", filter.ItemIDs)
	if len(filter.Statuses) > 0 {
		values := make([]string, len(filter.Statuses))
		for index, status := range filter.Statuses {
			values[index] = string(status)
		}
		query.Set("filters[statuses]", strings.Join(values, ","))
	}
	setOptionalFilter(query, "channel", filter.Channel)
	setBoolFilter(query, "buy_box_winner", filter.BuyBoxWinner)
	setOptionalFilter(query, "condition", filter.Condition)
	setOptionalFilter(query, "current_level", filter.CurrentLevel)
	setBoolFilter(query, "deferred_stock", filter.DeferredStock)
	setStringFilter(query, "domains", filter.Domains)
	setStringFilter(query, "logistic_types", filter.LogisticTypes)
	setStringFilter(query, "listing_types", filter.ListingTypes)
	setInt64Filter(query, "official_stores", filter.OfficialStores)
	setBoolFilter(query, "recommended", filter.Recommended)
	if filter.CampaignID > 0 {
		query.Set("filters[campaign_id]", formatInt64(filter.CampaignID))
	}
	setInt64Filter(query, "campaigns", filter.Campaigns)
	setOptionalFilter(query, "brand_value_id", filter.BrandValueID)
	setOptionalFilter(query, "brand_value_name", filter.BrandValueName)
	return query
}

func metricQuery(input MetricQuery) url.Values {
	query := make(url.Values)
	if len(input.Metrics) == 0 {
		return query
	}
	query.Set("date_from", string(input.DateFrom))
	query.Set("date_to", string(input.DateTo))
	values := make([]string, len(input.Metrics))
	for index, metric := range input.Metrics {
		values[index] = string(metric)
	}
	query.Set("metrics", strings.Join(values, ","))
	return query
}

func setPagination(query url.Values, limit, offset int64) {
	if limit > 0 {
		query.Set("limit", formatInt64(limit))
	}
	if offset > 0 {
		query.Set("offset", formatInt64(offset))
	}
}

func setOptionalFilter(query url.Values, name, value string) {
	if value != "" {
		query.Set("filters["+name+"]", value)
	}
}

func setStringFilter(query url.Values, name string, values []string) {
	if len(values) > 0 {
		query.Set("filters["+name+"]", strings.Join(values, ","))
	}
}

func setInt64Filter(query url.Values, name string, values []int64) {
	if len(values) == 0 {
		return
	}
	formatted := make([]string, len(values))
	for index, value := range values {
		formatted[index] = formatInt64(value)
	}
	query.Set("filters["+name+"]", strings.Join(formatted, ","))
}

func setBoolFilter(query url.Values, name string, value *bool) {
	if value != nil {
		query.Set("filters["+name+"]", strconv.FormatBool(*value))
	}
}

func validPaging(paging Paging, resultCount int, requestedLimit, requestedOffset int64) bool {
	return paging.Total >= 0 && paging.Offset == requestedOffset && paging.Offset <= paging.Total && paging.Limit > 0 &&
		(requestedLimit == 0 || paging.Limit == requestedLimit) && int64(resultCount) <= paging.Limit &&
		int64(resultCount) <= paging.Total-paging.Offset
}

func validCampaigns(campaigns []Campaign, metrics []Metric) bool {
	seen := make(map[int64]struct{}, len(campaigns))
	for _, campaign := range campaigns {
		if !validCampaign(campaign, metrics) {
			return false
		}
		if _, duplicate := seen[campaign.ID]; duplicate {
			return false
		}
		seen[campaign.ID] = struct{}{}
	}
	return true
}

func validCampaign(campaign Campaign, metrics []Metric) bool {
	if campaign.ID <= 0 || !validOpaque(campaign.Name, 2048) || !validOptionalOpaque(string(campaign.Status), 64) ||
		!validOptionalOpaque(campaign.CurrencyID, 32) || !validOptionalOpaque(campaign.Strategy, 128) ||
		!validOptionalOpaque(campaign.Channel, 128) {
		return false
	}
	return validMetrics(campaign.Metrics, metrics)
}

func validItems(items []Item, metrics []Metric) bool {
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		if !validItem(item, metrics) {
			return false
		}
		if _, duplicate := seen[item.ItemID]; duplicate {
			return false
		}
		seen[item.ItemID] = struct{}{}
	}
	return true
}

func validItem(item Item, metrics []Metric) bool {
	if !validOpaque(item.ItemID, 256) || !validOptionalOpaque(item.Title, 4096) ||
		!validOptionalOpaque(string(item.Status), 64) || !validOptionalOpaque(item.Channel, 128) ||
		item.CampaignID != nil && *item.CampaignID < 0 || item.OfficialStoreID != nil && *item.OfficialStoreID < 0 {
		return false
	}
	return validMetrics(item.Metrics, metrics)
}

func validDailyMetrics(rows []DailyMetrics, from, to Date, metrics []Metric) bool {
	if rows == nil {
		return false
	}
	start, startErr := time.Parse("2006-01-02", string(from))
	end, endErr := time.Parse("2006-01-02", string(to))
	if startErr != nil || endErr != nil {
		return false
	}
	seen := make(map[Date]struct{}, len(rows))
	for _, row := range rows {
		date, err := time.Parse("2006-01-02", string(row.Date))
		if err != nil || date.Before(start) || date.After(end) || !validMetrics(&row.Metrics, metrics) {
			return false
		}
		if _, duplicate := seen[row.Date]; duplicate {
			return false
		}
		seen[row.Date] = struct{}{}
	}
	return true
}

func validMetrics(values *Metrics, requested []Metric) bool {
	if len(requested) == 0 {
		return true
	}
	if values == nil {
		return false
	}
	for _, metric := range requested {
		if !metricValue(*values, metric).IsSet() {
			return false
		}
	}
	return true
}

func metricValue(values Metrics, metric Metric) ExactValue {
	switch metric {
	case MetricClicks:
		return values.Clicks
	case MetricPrints:
		return values.Prints
	case MetricCTR:
		return values.CTR
	case MetricCost:
		return values.Cost
	case MetricCostUSD:
		return values.CostUSD
	case MetricCPC:
		return values.CPC
	case MetricACOS:
		return values.ACOS
	case MetricOrganicUnitsQuantity:
		return values.OrganicUnitsQuantity
	case MetricOrganicUnitsAmount:
		return values.OrganicUnitsAmount
	case MetricOrganicItemsQuantity:
		return values.OrganicItemsQuantity
	case MetricDirectItemsQuantity:
		return values.DirectItemsQuantity
	case MetricIndirectItemsQuantity:
		return values.IndirectItemsQuantity
	case MetricAdvertisingItemsQuantity:
		return values.AdvertisingItemsQuantity
	case MetricCVR:
		return values.CVR
	case MetricROAS:
		return values.ROAS
	case MetricSOV:
		return values.SOV
	case MetricDirectUnitsQuantity:
		return values.DirectUnitsQuantity
	case MetricIndirectUnitsQuantity:
		return values.IndirectUnitsQuantity
	case MetricUnitsQuantity:
		return values.UnitsQuantity
	case MetricDirectAmount:
		return values.DirectAmount
	case MetricIndirectAmount:
		return values.IndirectAmount
	case MetricTotalAmount:
		return values.TotalAmount
	case MetricImpressionShare:
		return values.ImpressionShare
	case MetricTopImpressionShare:
		return values.TopImpressionShare
	case MetricLostImpressionShareByBudget:
		return values.LostImpressionShareByBudget
	case MetricLostImpressionShareByAdRank:
		return values.LostImpressionShareByAdRank
	case MetricACOSBenchmark:
		return values.ACOSBenchmark
	default:
		return ExactValue{}
	}
}

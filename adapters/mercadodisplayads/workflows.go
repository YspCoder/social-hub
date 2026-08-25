package mercadodisplayads

import (
	"context"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

const advertisersPath = "/advertising/advertisers"

type resultEnvelope[T any] struct {
	Results []T `json:"results"`
}

func (client *Client) ListAdvertisers(ctx context.Context, input AdvertiserListRequest, options ...socialhub.CallOption) (AdvertiserList, error) {
	const operation = "advertisers_list"
	if !validAdvertiserListRequest(input) {
		return AdvertiserList{}, invalidArgument(operation, "sort_by or sort_order is invalid")
	}
	query := sortQuery(string(input.SortBy), input.SortOrder)
	query.Set("product_id", "DISPLAY")
	var result AdvertiserList
	meta, err := client.doJSON(ctx, operation, advertisersPath, query, &result, options...)
	if err != nil {
		return AdvertiserList{}, err
	}
	if result.Advertisers == nil || !validAdvertisers(result.Advertisers, client.advertiserID) {
		return AdvertiserList{}, platformContractError(operation, "Mercado Libre returned invalid or unbound Display Ads advertisers")
	}
	result.Meta = meta
	return result, nil
}

func (client *Client) ListCampaigns(ctx context.Context, input CampaignListRequest, options ...socialhub.CallOption) ([]Campaign, ResponseMeta, error) {
	const operation = "campaigns_list"
	if client.advertiserID <= 0 || !validCampaignListRequest(input) {
		return nil, ResponseMeta{}, invalidArgument(operation, "configured advertiser_id, sort_by, or sort_order is invalid")
	}
	var result resultEnvelope[Campaign]
	meta, err := client.doJSON(ctx, operation, advertiserDisplayPath(client.advertiserID)+"/campaigns", sortQuery(string(input.SortBy), input.SortOrder), &result, options...)
	if err != nil {
		return nil, ResponseMeta{}, err
	}
	if result.Results == nil || !validCampaigns(result.Results, client.advertiserID) {
		return nil, meta, platformContractError(operation, "Mercado Libre returned invalid or unbound Display Ads Campaigns")
	}
	return result.Results, meta, nil
}

func (client *Client) CampaignMetrics(ctx context.Context, input CampaignMetricsRequest, options ...socialhub.CallOption) (*CampaignMetrics, error) {
	const operation = "campaign_metrics"
	if client.advertiserID <= 0 || !validCampaignMetricsRequest(input) {
		return nil, invalidArgument(operation, "configured advertiser_id, campaign_id, or metric date range is invalid")
	}
	var result CampaignMetrics
	path := advertiserDisplayPath(client.advertiserID) + "/campaigns/" + formatInt64(input.CampaignID) + "/metrics"
	meta, err := client.doJSON(ctx, operation, path, metricDateQuery(input.DateFrom, input.DateTo), &result, options...)
	if err != nil {
		return nil, err
	}
	if !validMetricRows(result.Metrics, input.DateFrom, input.DateTo) || !validMetricSummary(result.Summary) {
		return nil, platformContractError(operation, "Mercado Libre returned invalid Campaign metrics")
	}
	result.Meta = meta
	return &result, nil
}

func (client *Client) ListLineItems(ctx context.Context, input LineItemListRequest, options ...socialhub.CallOption) ([]LineItem, ResponseMeta, error) {
	const operation = "line_items_list"
	if client.advertiserID <= 0 || !validLineItemListRequest(input) {
		return nil, ResponseMeta{}, invalidArgument(operation, "configured advertiser_id, campaign_id, sort_by, or sort_order is invalid")
	}
	path := advertiserDisplayPath(client.advertiserID) + "/campaigns/" + formatInt64(input.CampaignID) + "/line_items"
	var result resultEnvelope[LineItem]
	meta, err := client.doJSON(ctx, operation, path, sortQuery(string(input.SortBy), input.SortOrder), &result, options...)
	if err != nil {
		return nil, ResponseMeta{}, err
	}
	if result.Results == nil || !validLineItems(result.Results, input.CampaignID) {
		return nil, meta, platformContractError(operation, "Mercado Libre returned invalid or unbound Line Items")
	}
	return result.Results, meta, nil
}

func (client *Client) LineItemMetrics(ctx context.Context, input LineItemMetricsRequest, options ...socialhub.CallOption) ([]LineItemMetricGroup, ResponseMeta, error) {
	const operation = "line_item_metrics"
	if client.advertiserID <= 0 || !validLineItemMetricsRequest(input) {
		return nil, ResponseMeta{}, invalidArgument(operation, "configured advertiser_id, filters, IDs, or metric date range is invalid")
	}
	query := metricDateQuery(input.DateFrom, input.DateTo)
	query.Set("dimension", "line_items")
	if input.CampaignID > 0 {
		query.Set("campaign_id", formatInt64(input.CampaignID))
	}
	setIDs(query, input.IDs)
	var result []LineItemMetricGroup
	meta, err := client.doJSON(ctx, operation, advertiserDisplayPath(client.advertiserID)+"/metrics", query, &result, options...)
	if err != nil {
		return nil, ResponseMeta{}, err
	}
	if result == nil || !validLineItemMetricGroups(result, input) {
		return nil, meta, platformContractError(operation, "Mercado Libre returned invalid or unbound Line Item metrics")
	}
	return result, meta, nil
}

func (client *Client) ListCreatives(ctx context.Context, input CreativeListRequest, options ...socialhub.CallOption) ([]Creative, ResponseMeta, error) {
	const operation = "creatives_list"
	if client.advertiserID <= 0 || !validCreativeListRequest(input) {
		return nil, ResponseMeta{}, invalidArgument(operation, "configured advertiser_id, hierarchy IDs, sort_by, or sort_order is invalid")
	}
	path := advertiserDisplayPath(client.advertiserID) + "/campaigns/" + formatInt64(input.CampaignID) +
		"/line_items/" + formatInt64(input.LineItemID) + "/creatives"
	var result resultEnvelope[Creative]
	meta, err := client.doJSON(ctx, operation, path, sortQuery(string(input.SortBy), input.SortOrder), &result, options...)
	if err != nil {
		return nil, ResponseMeta{}, err
	}
	if result.Results == nil || !validCreatives(result.Results, input.CampaignID, input.LineItemID) {
		return nil, meta, platformContractError(operation, "Mercado Libre returned invalid or unbound Creatives")
	}
	return result.Results, meta, nil
}

func (client *Client) CreativeMetrics(ctx context.Context, input CreativeMetricsRequest, options ...socialhub.CallOption) ([]CreativeMetricGroup, ResponseMeta, error) {
	const operation = "creative_metrics"
	if client.advertiserID <= 0 || !validCreativeMetricsRequest(input) {
		return nil, ResponseMeta{}, invalidArgument(operation, "configured advertiser_id, filters, IDs, or metric date range is invalid")
	}
	query := metricDateQuery(input.DateFrom, input.DateTo)
	query.Set("dimension", "creatives")
	if input.LineItemID > 0 {
		query.Set("line_item_id", formatInt64(input.LineItemID))
	}
	setIDs(query, input.IDs)
	var result []CreativeMetricGroup
	meta, err := client.doJSON(ctx, operation, advertiserDisplayPath(client.advertiserID)+"/metrics", query, &result, options...)
	if err != nil {
		return nil, ResponseMeta{}, err
	}
	if result == nil || !validCreativeMetricGroups(result, input) {
		return nil, meta, platformContractError(operation, "Mercado Libre returned invalid or unbound Creative metrics")
	}
	return result, meta, nil
}

func advertiserDisplayPath(advertiserID int64) string {
	return "/advertising/advertisers/" + formatInt64(advertiserID) + "/display"
}

func sortQuery(field string, order SortOrder) url.Values {
	query := make(url.Values)
	if field != "" {
		query.Set("sort_by", field)
	}
	if order != "" {
		query.Set("sort_order", string(order))
	}
	return query
}

func metricDateQuery(from, to Date) url.Values {
	return url.Values{"date_from": {string(from)}, "date_to": {string(to)}}
}

func setIDs(query url.Values, ids []int64) {
	if len(ids) == 0 {
		return
	}
	values := make([]string, len(ids))
	for index, id := range ids {
		values[index] = formatInt64(id)
	}
	query.Set("ids", strings.Join(values, ","))
}

func validAdvertisers(advertisers []Advertiser, configuredID int64) bool {
	seen := make(map[int64]struct{}, len(advertisers))
	foundConfigured := configuredID == 0
	for _, advertiser := range advertisers {
		if advertiser.AdvertiserID <= 0 || !validOpaque(advertiser.SiteID, 32) ||
			!validOpaque(advertiser.AdvertiserName, 1024) || !validOpaque(advertiser.AccountName, 1024) {
			return false
		}
		if _, duplicate := seen[advertiser.AdvertiserID]; duplicate {
			return false
		}
		seen[advertiser.AdvertiserID] = struct{}{}
		foundConfigured = foundConfigured || advertiser.AdvertiserID == configuredID
	}
	return foundConfigured
}

func validCampaigns(campaigns []Campaign, advertiserID int64) bool {
	seen := make(map[int64]struct{}, len(campaigns))
	for _, campaign := range campaigns {
		if campaign.ID <= 0 || campaign.AdvertiserID != advertiserID || !validOpaque(campaign.Name, 2048) ||
			!validOpaque(campaign.StartDate, 128) || !validOpaque(campaign.EndDate, 128) ||
			(campaign.Type != CampaignTypeProgrammatic && campaign.Type != CampaignTypeGuaranteed) ||
			!validOpaque(campaign.Status, 128) || !validOpaque(campaign.SiteID, 32) || !validOpaque(campaign.Goal, 128) {
			return false
		}
		if _, duplicate := seen[campaign.ID]; duplicate {
			return false
		}
		seen[campaign.ID] = struct{}{}
	}
	return true
}

func validLineItems(items []LineItem, campaignID int64) bool {
	seen := make(map[int64]struct{}, len(items))
	for _, item := range items {
		if item.LineItemID <= 0 || item.CampaignID != campaignID || !validOpaque(item.Name, 2048) ||
			!validOpaque(item.StartDate, 128) || !validOpaque(item.EndDate, 128) ||
			(item.Type != LineItemTypeDisplay && item.Type != LineItemTypeSocial && item.Type != LineItemTypeVideo) ||
			!validOpaque(item.Status, 128) {
			return false
		}
		if _, duplicate := seen[item.LineItemID]; duplicate {
			return false
		}
		seen[item.LineItemID] = struct{}{}
	}
	return true
}

func validCreatives(creatives []Creative, campaignID, lineItemID int64) bool {
	seen := make(map[int64]struct{}, len(creatives))
	for _, creative := range creatives {
		if creative.CreativeID <= 0 || creative.CampaignID != campaignID || creative.LineItemID != lineItemID ||
			!validOpaque(creative.Name, 2048) || !validOpaque(creative.Status, 128) {
			return false
		}
		if _, duplicate := seen[creative.CreativeID]; duplicate {
			return false
		}
		seen[creative.CreativeID] = struct{}{}
	}
	return true
}

func validLineItemMetricGroups(groups []LineItemMetricGroup, input LineItemMetricsRequest) bool {
	seen := make(map[int64]struct{}, len(groups))
	for _, group := range groups {
		if group.CampaignID <= 0 || group.LineItemID <= 0 ||
			input.CampaignID > 0 && group.CampaignID != input.CampaignID || !containsID(input.IDs, group.LineItemID) ||
			!validMetricRows(group.Metrics, input.DateFrom, input.DateTo) || !validMetricSummary(group.Summary) {
			return false
		}
		if _, duplicate := seen[group.LineItemID]; duplicate {
			return false
		}
		seen[group.LineItemID] = struct{}{}
	}
	return true
}

func validCreativeMetricGroups(groups []CreativeMetricGroup, input CreativeMetricsRequest) bool {
	seen := make(map[int64]struct{}, len(groups))
	for _, group := range groups {
		if group.CampaignID <= 0 || group.LineItemID <= 0 || group.CreativeID <= 0 ||
			input.LineItemID > 0 && group.LineItemID != input.LineItemID || !containsID(input.IDs, group.CreativeID) ||
			!validMetricRows(group.Metrics, input.DateFrom, input.DateTo) || !validMetricSummary(group.Summary) {
			return false
		}
		if _, duplicate := seen[group.CreativeID]; duplicate {
			return false
		}
		seen[group.CreativeID] = struct{}{}
	}
	return true
}

func validMetricRows(rows []DeliveryMetric, from, to Date) bool {
	if rows == nil {
		return false
	}
	seen := make(map[Date]struct{}, len(rows))
	for _, row := range rows {
		if !dateWithin(row.Date, from, to) || !validMetric(row) {
			return false
		}
		if _, duplicate := seen[row.Date]; duplicate {
			return false
		}
		seen[row.Date] = struct{}{}
	}
	return true
}

func validMetricSummary(summary *DeliveryMetric) bool {
	return summary != nil && (summary.Date == "" || validDate(summary.Date)) && validMetric(*summary)
}

func validMetric(metric DeliveryMetric) bool {
	if !validOptionalOpaque(metric.SiteID, 32) || !validOptionalOpaque(metric.Currency, 32) {
		return false
	}
	return metric.Prints.IsSet() || metric.Clicks.IsSet() || metric.ActiveViews.IsSet() ||
		metric.CompletedViews.IsSet() || metric.Reach.IsSet() || metric.CTR.IsSet() ||
		metric.ConsumedBudget.IsSet() || metric.CPM.IsSet() || metric.CPC.IsSet() ||
		metric.AverageFrequency.IsSet() || attributionMetricSet(metric.EventTime) || attributionMetricSet(metric.TouchPoint)
}

func attributionMetricSet(metric AttributionMetric) bool {
	return metric.CPAOrder.IsSet() || metric.CPAPPV.IsSet() || metric.ROAS.IsSet() ||
		metric.UnitsQuantity.IsSet() || metric.DirectAmount.IsSet() || metric.DirectItemQuantity.IsSet() ||
		metric.AttributionPPV.IsSet() || metric.AttributionAddToCart.IsSet() || metric.AttributionBookmark.IsSet() ||
		metric.AttributionCheckout.IsSet() || metric.AttributionLeads.IsSet() || metric.CostPerAttributedLead.IsSet()
}

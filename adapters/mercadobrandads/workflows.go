package mercadobrandads

import (
	"context"
	"net/url"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

const advertisersPath = "/advertising/advertisers"

func (client *Client) ListAdvertisers(ctx context.Context, options ...socialhub.CallOption) (AdvertiserList, error) {
	const operation = "advertisers_list"
	var result AdvertiserList
	meta, err := client.doJSON(ctx, operation, advertisersPath, url.Values{"product_id": {"BADS"}}, "1", &result, options...)
	if err != nil {
		return AdvertiserList{}, err
	}
	if result.Advertisers == nil || !validAdvertisers(result.Advertisers, client.advertiserID) {
		return AdvertiserList{}, platformContractError(operation, "Mercado Libre returned invalid or unbound Brand Ads advertisers")
	}
	result.Meta = meta
	return result, nil
}

func (client *Client) ListCampaigns(ctx context.Context, options ...socialhub.CallOption) (CampaignPage, error) {
	const operation = "campaigns_list"
	if client.advertiserID <= 0 {
		return CampaignPage{}, invalidArgument(operation, "configured advertiser_id must be positive")
	}
	var result CampaignPage
	meta, err := client.doJSON(ctx, operation, advertiserCampaignsPath(client.advertiserID), nil, "", &result, options...)
	if err != nil {
		return CampaignPage{}, err
	}
	if !validPaging(result.Paging, len(result.Campaigns)) || result.Campaigns == nil ||
		!validCampaigns(result.Campaigns, client.advertiserID) {
		return CampaignPage{}, platformContractError(operation, "Mercado Libre returned invalid or unbound Brand Ads Campaigns")
	}
	result.Meta = meta
	return result, nil
}

func (client *Client) GetCampaign(ctx context.Context, campaignID int64, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_get"
	if client.advertiserID <= 0 || campaignID <= 0 {
		return nil, invalidArgument(operation, "configured advertiser_id and campaign_id must be positive")
	}
	var result Campaign
	meta, err := client.doJSON(ctx, operation, campaignPath(client.advertiserID, campaignID), nil, "", &result, options...)
	if err != nil {
		return nil, err
	}
	if result.CampaignID != campaignID || !validCampaign(result, client.advertiserID) {
		return nil, platformContractError(operation, "Mercado Libre returned an invalid or unbound Brand Ads Campaign")
	}
	result.Meta = meta
	return &result, nil
}

func (client *Client) ListItems(ctx context.Context, campaignID int64, options ...socialhub.CallOption) ([]Item, ResponseMeta, error) {
	const operation = "items_list"
	if client.advertiserID <= 0 || campaignID <= 0 {
		return nil, ResponseMeta{}, invalidArgument(operation, "configured advertiser_id and campaign_id must be positive")
	}
	var result []Item
	meta, err := client.doJSON(ctx, operation, campaignPath(client.advertiserID, campaignID)+"/items", nil, "", &result, options...)
	if err != nil {
		return nil, ResponseMeta{}, err
	}
	if result == nil || !validItems(result, campaignID) {
		return nil, meta, platformContractError(operation, "Mercado Libre returned invalid or unbound Brand Ads Items")
	}
	return result, meta, nil
}

func (client *Client) ListKeywords(ctx context.Context, campaignID int64, options ...socialhub.CallOption) ([]Keyword, ResponseMeta, error) {
	const operation = "keywords_list"
	if client.advertiserID <= 0 || campaignID <= 0 {
		return nil, ResponseMeta{}, invalidArgument(operation, "configured advertiser_id and campaign_id must be positive")
	}
	var result []Keyword
	meta, err := client.doJSON(ctx, operation, campaignPath(client.advertiserID, campaignID)+"/keywords", nil, "", &result, options...)
	if err != nil {
		return nil, ResponseMeta{}, err
	}
	if result == nil || !validKeywords(result, campaignID) {
		return nil, meta, platformContractError(operation, "Mercado Libre returned invalid or unbound Brand Ads Keywords")
	}
	return result, meta, nil
}

func (client *Client) ListAdvertiserCampaignMetrics(ctx context.Context, input AdvertiserMetricRequest, options ...socialhub.CallOption) (CampaignMetricPage, error) {
	const operation = "advertiser_campaign_metrics"
	if client.advertiserID <= 0 || !validAdvertiserMetricRequest(input) {
		return CampaignMetricPage{}, invalidArgument(operation, "configured advertiser_id, metrics query, status, or destination_id is invalid")
	}
	query := metricQuery(input.MetricQuery)
	if input.Status != "" {
		query.Set("status", string(input.Status))
	}
	if input.DestinationID > 0 {
		query.Set("destination_id", formatInt64(input.DestinationID))
	}
	var result CampaignMetricPage
	meta, err := client.doJSON(ctx, operation, advertiserCampaignsPath(client.advertiserID)+"/metrics", query, "", &result, options...)
	if err != nil {
		return CampaignMetricPage{}, err
	}
	if !validCampaignMetricPage(result, input.MetricQuery, false) {
		return CampaignMetricPage{}, platformContractError(operation, "Mercado Libre returned invalid advertiser Campaign metrics")
	}
	result.Meta = meta
	return result, nil
}

func (client *Client) GetCampaignMetrics(ctx context.Context, input CampaignMetricRequest, options ...socialhub.CallOption) (CampaignMetricPage, error) {
	const operation = "campaign_metrics"
	if client.advertiserID <= 0 || !validCampaignMetricRequest(input) {
		return CampaignMetricPage{}, invalidArgument(operation, "configured advertiser_id, campaign_id, or metrics query is invalid")
	}
	var result CampaignMetricPage
	meta, err := client.doJSON(ctx, operation, campaignPath(client.advertiserID, input.CampaignID)+"/metrics", metricQuery(input.MetricQuery), "", &result, options...)
	if err != nil {
		return CampaignMetricPage{}, err
	}
	if !validCampaignMetricPage(result, input.MetricQuery, true) {
		return CampaignMetricPage{}, platformContractError(operation, "Mercado Libre returned invalid Campaign metrics")
	}
	result.Meta = meta
	return result, nil
}

func (client *Client) GetKeywordMetrics(ctx context.Context, input KeywordMetricRequest, options ...socialhub.CallOption) (KeywordMetricPage, error) {
	const operation = "keyword_metrics"
	if client.advertiserID <= 0 || !validKeywordMetricRequest(input) {
		return KeywordMetricPage{}, invalidArgument(operation, "configured advertiser_id, campaign_id, or metrics query is invalid")
	}
	var result KeywordMetricPage
	path := campaignPath(client.advertiserID, input.CampaignID) + "/keywords/metrics"
	meta, err := client.doJSON(ctx, operation, path, metricQuery(input.MetricQuery), "", &result, options...)
	if err != nil {
		return KeywordMetricPage{}, err
	}
	if !validKeywordMetricPage(result, input.MetricQuery) {
		return KeywordMetricPage{}, platformContractError(operation, "Mercado Libre returned invalid Keyword metrics")
	}
	result.Meta = meta
	return result, nil
}

func advertiserCampaignsPath(advertiserID int64) string {
	return "/advertising/advertisers/" + formatInt64(advertiserID) + "/brand_ads/campaigns"
}

func campaignPath(advertiserID, campaignID int64) string {
	return advertiserCampaignsPath(advertiserID) + "/" + formatInt64(campaignID)
}

func metricQuery(input MetricQuery) url.Values {
	query := url.Values{"date_from": {string(input.DateFrom)}, "date_to": {string(input.DateTo)}}
	if input.Limit > 0 {
		query.Set("limit", formatInt64(input.Limit))
	}
	if input.Offset > 0 {
		query.Set("offset", formatInt64(input.Offset))
	}
	if input.AggregationType != "" {
		query.Set("aggregation_type", string(input.AggregationType))
	}
	if len(input.Fields) > 0 {
		fields := make([]string, len(input.Fields))
		for index, field := range input.Fields {
			fields[index] = string(field)
		}
		query.Set("fields", strings.Join(fields, ","))
	}
	return query
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

func validPaging(paging Paging, resultCount int) bool {
	return paging.Total >= 0 && paging.Offset >= 0 && paging.Limit >= 0 &&
		(paging.Limit == 0 || int64(resultCount) <= paging.Limit) && int64(resultCount) <= paging.Total
}

func validCampaigns(campaigns []Campaign, advertiserID int64) bool {
	seen := make(map[int64]struct{}, len(campaigns))
	for _, campaign := range campaigns {
		if !validCampaign(campaign, advertiserID) {
			return false
		}
		if _, duplicate := seen[campaign.CampaignID]; duplicate {
			return false
		}
		seen[campaign.CampaignID] = struct{}{}
	}
	return true
}

func validCampaign(campaign Campaign, advertiserID int64) bool {
	if campaign.CampaignID <= 0 || campaign.AdvertiserID != advertiserID || !validOpaque(campaign.Name, 2048) ||
		!validTimestamp(campaign.StartDate) || !validTimestamp(campaign.EndDate) ||
		(campaign.CampaignType != CampaignTypeAutomatic && campaign.CampaignType != CampaignTypeCustom) ||
		!validOpaque(campaign.Status, 128) || !validOpaque(campaign.SiteID, 32) ||
		campaign.OfficialStoreID <= 0 || campaign.DestinationID <= 0 || !validOpaque(campaign.Headline, 4096) ||
		!campaign.Budget.Amount.IsNumber() || !validOpaque(campaign.Budget.Currency, 32) || !campaign.CPC.IsNumber() ||
		campaign.Items == nil || campaign.Keywords == nil {
		return false
	}
	start, _ := time.Parse(time.RFC3339Nano, campaign.StartDate)
	end, _ := time.Parse(time.RFC3339Nano, campaign.EndDate)
	return !end.Before(start) && validItems(campaign.Items, campaign.CampaignID) && validKeywords(campaign.Keywords, campaign.CampaignID)
}

func validItems(items []Item, campaignID int64) bool {
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item.CampaignID != campaignID || !validOpaque(item.Status, 128) || !validOpaque(item.ItemID, 256) ||
			strings.ContainsAny(item.ItemID, "/?#") {
			return false
		}
		if _, duplicate := seen[item.ItemID]; duplicate {
			return false
		}
		seen[item.ItemID] = struct{}{}
	}
	return true
}

func validKeywords(keywords []Keyword, campaignID int64) bool {
	seen := make(map[string]struct{}, len(keywords))
	for _, keyword := range keywords {
		if keyword.CampaignID != campaignID || keyword.KeywordID < 0 || !validOpaque(keyword.Type, 128) ||
			!validOpaque(keyword.Term, 2048) || !validOpaque(keyword.MatchType, 128) || !keyword.CPC.IsNumber() {
			return false
		}
		key := keyword.Term + "\x00" + keyword.MatchType + "\x00" + formatInt64(keyword.KeywordID)
		if keyword.IsNegative {
			key += "\x00negative"
		}
		if _, duplicate := seen[key]; duplicate {
			return false
		}
		seen[key] = struct{}{}
	}
	return true
}

func validCampaignMetricPage(page CampaignMetricPage, input MetricQuery, allowCompetitive bool) bool {
	wantDaily, wantTotal := metricResponseParts(input.AggregationType)
	if !validPaging(page.Paging, len(page.Metrics)) || wantDaily && page.Metrics == nil || wantTotal && page.Summary == nil {
		return false
	}
	if page.Summary != nil && (!validMetric(*page.Summary, false, allowCompetitive) ||
		!hasRequestedMetricFields(*page.Summary, input.Fields)) {
		return false
	}
	seen := make(map[Date]struct{}, len(page.Metrics))
	for _, metric := range page.Metrics {
		if !dateWithin(metric.Date, input.DateFrom, input.DateTo) || !validMetric(metric, false, false) ||
			!hasRequestedMetricFields(metric, input.Fields) {
			return false
		}
		if _, duplicate := seen[metric.Date]; duplicate {
			return false
		}
		seen[metric.Date] = struct{}{}
	}
	return true
}

func validKeywordMetricPage(page KeywordMetricPage, input MetricQuery) bool {
	wantDaily, wantTotal := metricResponseParts(input.AggregationType)
	if !validPaging(page.Paging, len(page.Metrics)) || wantDaily && page.Metrics == nil || wantTotal && page.Summary == nil {
		return false
	}
	seenDates := make(map[Date]struct{}, len(page.Metrics))
	for _, row := range page.Metrics {
		if !dateWithin(row.Date, input.DateFrom, input.DateTo) || row.Keywords == nil ||
			!validKeywordMetricSet(row.Keywords, input.Fields) {
			return false
		}
		if _, duplicate := seenDates[row.Date]; duplicate {
			return false
		}
		seenDates[row.Date] = struct{}{}
	}
	return page.Summary == nil || validKeywordMetricSet(page.Summary, input.Fields)
}

func validKeywordMetricSet(metrics []Metric, fields []MetricField) bool {
	seen := make(map[string]struct{}, len(metrics))
	for _, metric := range metrics {
		if !validOpaque(metric.Keyword, 2048) || !validMetric(metric, true, false) ||
			!hasRequestedMetricFields(metric, fields) {
			return false
		}
		if _, duplicate := seen[metric.Keyword]; duplicate {
			return false
		}
		seen[metric.Keyword] = struct{}{}
	}
	return true
}

func validMetric(metric Metric, keyword, allowCompetitive bool) bool {
	if !validOptionalOpaque(metric.SiteID, 32) || !validOptionalOpaque(metric.Currency, 32) ||
		keyword != (metric.Keyword != "") || metric.Date != "" && !validDate(metric.Date) ||
		metric.Competitive != nil && (!allowCompetitive || !competitiveMetricSet(metric.Competitive)) ||
		!validExactNumbers(metric.Prints, metric.Clicks, metric.CTR, metric.CVR, metric.ConsumedBudget, metric.CPC, metric.ACOS) ||
		!validAttributionMetric(metric.EventTime) || !validAttributionMetric(metric.TouchPoint) {
		return false
	}
	return metric.Prints.IsNumber() || metric.Clicks.IsNumber() || metric.CTR.IsNumber() || metric.CVR.IsNumber() ||
		metric.ConsumedBudget.IsNumber() || metric.CPC.IsNumber() || metric.ACOS.IsNumber() ||
		attributionMetricSet(metric.EventTime) || attributionMetricSet(metric.TouchPoint) ||
		competitiveMetricSet(metric.Competitive)
}

func metricResponseParts(aggregation AggregationType) (daily, total bool) {
	return aggregation != AggregationTotal, aggregation != AggregationDaily
}

func hasRequestedMetricFields(metric Metric, fields []MetricField) bool {
	for _, field := range fields {
		var present bool
		switch field {
		case MetricFieldPrints:
			present = metric.Prints.IsNumber()
		case MetricFieldClicks:
			present = metric.Clicks.IsNumber()
		case MetricFieldCTR:
			present = metric.CTR.IsNumber()
		case MetricFieldCVR:
			present = metric.CVR.IsNumber()
		case MetricFieldConsumedBudget:
			present = metric.ConsumedBudget.IsNumber()
		case MetricFieldCPC:
			present = metric.CPC.IsNumber()
		case MetricFieldACOS:
			present = metric.ACOS.IsNumber()
		case MetricFieldEventTime:
			present = attributionMetricSet(metric.EventTime)
		case MetricFieldTouchPoint:
			present = attributionMetricSet(metric.TouchPoint)
		case MetricFieldCompetitive:
			present = competitiveMetricSet(metric.Competitive)
		}
		if !present {
			return false
		}
	}
	return true
}

func validExactNumbers(values ...ExactValue) bool {
	for _, value := range values {
		if value.IsSet() && !value.IsNumber() {
			return false
		}
	}
	return true
}

func validAttributionMetric(metric AttributionMetric) bool {
	return validExactNumbers(metric.UnitsQuantity, metric.UnitsAmount, metric.ItemsQuantity,
		metric.PPVConversions, metric.BookmarkConversions, metric.CartConversions,
		metric.CheckoutConversions, metric.LeadsQuestionConversions, metric.LeadsIMConversions,
		metric.EshopConversions)
}

func attributionMetricSet(metric AttributionMetric) bool {
	return metric.UnitsQuantity.IsNumber() || metric.UnitsAmount.IsNumber() || metric.ItemsQuantity.IsNumber() ||
		metric.PPVConversions.IsNumber() || metric.BookmarkConversions.IsNumber() || metric.CartConversions.IsNumber() ||
		metric.CheckoutConversions.IsNumber() || metric.LeadsQuestionConversions.IsNumber() ||
		metric.LeadsIMConversions.IsNumber() || metric.EshopConversions.IsNumber()
}

func competitiveMetricSet(metric *CompetitiveMetric) bool {
	return metric != nil && validExactNumbers(metric.LostImpressionShareByBudget,
		metric.LostImpressionShareByAdRank, metric.ImpressionShare, metric.CompetitiveCPC) &&
		(metric.LostImpressionShareByBudget.IsNumber() || metric.LostImpressionShareByAdRank.IsNumber() ||
			metric.ImpressionShare.IsNumber() || metric.CompetitiveCPC.IsNumber())
}

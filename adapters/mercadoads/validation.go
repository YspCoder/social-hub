package mercadoads

import (
	"mime"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

var (
	campaignListMetrics = metricSet(
		MetricClicks, MetricPrints, MetricCTR, MetricCost, MetricCostUSD, MetricCPC, MetricACOS,
		MetricOrganicUnitsQuantity, MetricOrganicUnitsAmount, MetricOrganicItemsQuantity,
		MetricDirectItemsQuantity, MetricIndirectItemsQuantity, MetricAdvertisingItemsQuantity,
		MetricCVR, MetricROAS, MetricSOV, MetricDirectUnitsQuantity, MetricIndirectUnitsQuantity,
		MetricUnitsQuantity, MetricDirectAmount, MetricIndirectAmount, MetricTotalAmount,
	)
	campaignDetailMetrics = metricSet(
		MetricClicks, MetricPrints, MetricCTR, MetricCost, MetricCPC, MetricACOS,
		MetricOrganicUnitsQuantity, MetricOrganicUnitsAmount, MetricOrganicItemsQuantity,
		MetricDirectItemsQuantity, MetricIndirectItemsQuantity, MetricAdvertisingItemsQuantity,
		MetricCVR, MetricROAS, MetricSOV, MetricDirectUnitsQuantity, MetricIndirectUnitsQuantity,
		MetricUnitsQuantity, MetricDirectAmount, MetricIndirectAmount, MetricTotalAmount,
		MetricImpressionShare, MetricTopImpressionShare, MetricLostImpressionShareByBudget,
		MetricLostImpressionShareByAdRank, MetricACOSBenchmark,
	)
	itemListMetrics = metricSet(
		MetricClicks, MetricPrints, MetricCost, MetricCPC, MetricACOS, MetricOrganicUnitsQuantity,
		MetricOrganicUnitsAmount, MetricOrganicItemsQuantity, MetricDirectItemsQuantity,
		MetricIndirectItemsQuantity, MetricAdvertisingItemsQuantity, MetricDirectUnitsQuantity,
		MetricIndirectUnitsQuantity, MetricUnitsQuantity, MetricDirectAmount, MetricIndirectAmount,
		MetricTotalAmount,
	)
	itemDetailMetrics = extendMetricSet(itemListMetrics, MetricCTR, MetricCVR, MetricROAS, MetricSOV)
)

func resolveCallOptions(operation string, options []socialhub.CallOption) (socialhub.CallOptions, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return socialhub.CallOptions{}, invalidArgument(operation, "call options are invalid")
	}
	if resolved.RequestID != "" {
		return socialhub.CallOptions{}, invalidArgument(operation, "Mercado Libre assigns request IDs; caller request IDs are not supported")
	}
	if resolved.IdempotencyKey != "" {
		return socialhub.CallOptions{}, invalidArgument(operation, "read-only Product Ads operations do not accept idempotency keys")
	}
	if len(resolved.Fields) > 0 {
		return socialhub.CallOptions{}, invalidArgument(operation, "field selection is expressed by the typed metrics request")
	}
	if resolved.Timeout < 0 {
		return socialhub.CallOptions{}, invalidArgument(operation, "timeout must not be negative")
	}
	return resolved, nil
}

func resolvedCallOption(resolved socialhub.CallOptions) socialhub.CallOption {
	return func(target *socialhub.CallOptions) error {
		*target = resolved
		target.Fields = append([]string(nil), resolved.Fields...)
		return nil
	}
}

func validAuthorizationEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil || parsed == nil || parsed.Scheme != "https" || parsed.Port() != "" ||
		parsed.User != nil || parsed.Path != "/authorization" || parsed.RawPath != "" ||
		parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return false
	}
	switch strings.ToLower(parsed.Hostname()) {
	case "auth.mercadolibre.com.ar", "auth.mercadolibre.com.bo", "auth.mercadolivre.com.br",
		"auth.mercadolibre.cl", "auth.mercadolibre.com.co", "auth.mercadolibre.co.cr",
		"auth.mercadolibre.com.do", "auth.mercadolibre.com.ec", "auth.mercadolibre.com.gt",
		"auth.mercadolibre.com.hn", "auth.mercadolibre.com.mx", "auth.mercadolibre.com.ni",
		"auth.mercadolibre.com.pa", "auth.mercadolibre.com.pe", "auth.mercadolibre.com.py",
		"auth.mercadolibre.com.sv", "auth.mercadolibre.com.uy", "auth.mercadolibre.com.ve":
		return true
	default:
		return false
	}
}

func validTokenEndpoint(value string) bool {
	return value == defaultTokenURL
}

func validCallbackURL(value string) bool {
	if !validOpaque(value, 4096) {
		return false
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return false
	}
	if parsed.Scheme == "https" {
		return true
	}
	if parsed.Scheme != "http" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "localhost" {
		return true
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}

func validJSONMediaType(value string) bool {
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && mediaType == "application/json"
}

func validPositiveDecimal(value string) bool {
	if !validOpaque(value, 128) || strings.HasPrefix(value, "+") {
		return false
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	return err == nil && parsed > 0
}

func validOpaque(value string, maximum int) bool {
	if value == "" || value != strings.TrimSpace(value) || !utf8.ValidString(value) || len(value) > maximum {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validPKCEChallenge(challenge, method string) bool {
	if challenge == "" || method == "" {
		return challenge == "" && method == ""
	}
	return (method == "S256" || method == "plain") && validPKCEValue(challenge)
}

func validPKCEVerifier(value string) bool { return validPKCEValue(value) }

func validPKCEValue(value string) bool {
	if len(value) < 43 || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if !unicode.IsLetter(character) && !unicode.IsDigit(character) && !strings.ContainsRune("-._~", character) {
			return false
		}
	}
	return true
}

func validDate(value Date) bool {
	if value == "" {
		return false
	}
	parsed, err := time.Parse("2006-01-02", string(value))
	return err == nil && DateFromTime(parsed) == value
}

func validateMetricQuery(operation string, query MetricQuery, allowed map[Metric]struct{}, required bool) error {
	if len(query.Metrics) == 0 {
		if required || query.DateFrom != "" || query.DateTo != "" {
			return invalidArgument(operation, "metrics and an ordered date_from/date_to range are required together")
		}
		return nil
	}
	if !validMetricList(query.Metrics, allowed) || !validMetricDateRange(query.DateFrom, query.DateTo) {
		return invalidArgument(operation, "metrics or the date range is invalid; Product Ads metrics are limited to a 90-day range")
	}
	return nil
}

func validMetricDateRange(from, to Date) bool {
	if !validDate(from) || !validDate(to) {
		return false
	}
	start, _ := time.Parse("2006-01-02", string(from))
	end, _ := time.Parse("2006-01-02", string(to))
	return !end.Before(start) && end.Sub(start) < 90*24*time.Hour
}

func validMetricList(metrics []Metric, allowed map[Metric]struct{}) bool {
	if len(metrics) == 0 || len(metrics) > len(allowed) {
		return false
	}
	seen := make(map[Metric]struct{}, len(metrics))
	for _, metric := range metrics {
		if _, ok := allowed[metric]; !ok {
			return false
		}
		if _, duplicate := seen[metric]; duplicate {
			return false
		}
		seen[metric] = struct{}{}
	}
	return true
}

func metricSet(metrics ...Metric) map[Metric]struct{} {
	result := make(map[Metric]struct{}, len(metrics))
	for _, metric := range metrics {
		result[metric] = struct{}{}
	}
	return result
}

func extendMetricSet(base map[Metric]struct{}, metrics ...Metric) map[Metric]struct{} {
	result := make(map[Metric]struct{}, len(base)+len(metrics))
	for metric := range base {
		result[metric] = struct{}{}
	}
	for _, metric := range metrics {
		result[metric] = struct{}{}
	}
	return result
}

func validCampaignListRequest(input CampaignListRequest, daily bool) bool {
	if input.Limit < 0 || input.Offset < 0 || daily && input.MetricsSummary ||
		!validIDs(input.CampaignIDs) || !validCampaignStatuses(input.Statuses) ||
		input.Channel != "" && input.Channel != "marketplace" {
		return false
	}
	query := MetricQuery{DateFrom: input.DateFrom, DateTo: input.DateTo, Metrics: input.Metrics}
	return validateMetricQuery("campaign_list", query, campaignListMetrics, daily) == nil &&
		(!input.MetricsSummary || len(input.Metrics) > 0)
}

func validItemListRequest(input ItemListRequest, daily bool) bool {
	if input.Limit < 0 || input.Offset < 0 || daily && input.MetricsSummary ||
		!validItemFilters(input.Filters) {
		return false
	}
	query := MetricQuery{DateFrom: input.DateFrom, DateTo: input.DateTo, Metrics: input.Metrics}
	if validateMetricQuery("item_list", query, itemListMetrics, daily) != nil || input.MetricsSummary && len(input.Metrics) == 0 {
		return false
	}
	return input.Filters.CampaignID == 0 || len(input.Metrics) > 0
}

func validIDs(values []int64) bool {
	if len(values) > 100 {
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

func validCampaignStatuses(values []CampaignStatus) bool {
	seen := make(map[CampaignStatus]struct{}, len(values))
	for _, value := range values {
		if value != CampaignStatusActive && value != CampaignStatusPaused {
			return false
		}
		if _, duplicate := seen[value]; duplicate {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validItemStatuses(values []ItemStatus) bool {
	seen := make(map[ItemStatus]struct{}, len(values))
	for _, value := range values {
		switch value {
		case ItemStatusActive, ItemStatusPaused, ItemStatusHold, ItemStatusIdle, ItemStatusDelegated, ItemStatusRevoked:
		default:
			return false
		}
		if _, duplicate := seen[value]; duplicate {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validItemFilters(filter ItemFilters) bool {
	return validOpaqueList(filter.ItemIDs, 100, 256) && validItemStatuses(filter.Statuses) &&
		(filter.Channel == "" || filter.Channel == "marketplace") &&
		validOptionalOpaque(filter.Condition, 128) && validOptionalOpaque(filter.CurrentLevel, 128) &&
		validOpaqueList(filter.Domains, 100, 256) && validOpaqueList(filter.LogisticTypes, 100, 128) &&
		validOpaqueList(filter.ListingTypes, 100, 128) && validIDs(filter.OfficialStores) &&
		filter.CampaignID >= 0 && validIDs(filter.Campaigns) &&
		validOptionalOpaque(filter.BrandValueID, 256) && validOptionalOpaque(filter.BrandValueName, 256)
}

func validOptionalOpaque(value string, maximum int) bool {
	return value == "" || validOpaque(value, maximum)
}

func validOpaqueList(values []string, maximumCount, maximumLength int) bool {
	if len(values) > maximumCount {
		return false
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validOpaque(value, maximumLength) {
			return false
		}
		if _, duplicate := seen[value]; duplicate {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func formatInt64(value int64) string { return strconv.FormatInt(value, 10) }

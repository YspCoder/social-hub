package petalads

import (
	"net"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

func validCallbackURL(value string) bool {
	if !validOpaque(value, 4096) || strings.Contains(value, "#") {
		return false
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Opaque != "" || parsed.Host == "" || parsed.Hostname() == "" ||
		parsed.User != nil || parsed.Fragment != "" {
		return false
	}
	if strings.EqualFold(parsed.Scheme, "https") {
		return true
	}
	if !strings.EqualFold(parsed.Scheme, "http") {
		return false
	}
	host := parsed.Hostname()
	return strings.EqualFold(host, "localhost") || net.ParseIP(host) != nil && net.ParseIP(host).IsLoopback()
}

func validOpaque(value string, maximum int) bool {
	if value == "" || len(value) > maximum || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validOptionalOpaque(value string, maximum int) bool {
	return value == "" || validOpaque(value, maximum)
}

func validID(value string) bool {
	if value == "" || len(value) > 32 {
		return false
	}
	nonZero := false
	for index := range value {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
		nonZero = nonZero || value[index] != '0'
	}
	return nonZero
}

func validClientID(value string) bool { return validID(value) }

func validIDs(values []string, maximum int) bool {
	if len(values) > maximum {
		return false
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validID(value) {
			return false
		}
		if _, found := seen[value]; found {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validIdentifier(value string, maximum int) bool {
	if value == "" || len(value) > maximum {
		return false
	}
	for index := range value {
		character := value[index]
		if character != '_' && character != '-' && character != '.' &&
			(character < '0' || character > '9') &&
			(character < 'A' || character > 'Z') &&
			(character < 'a' || character > 'z') {
			return false
		}
	}
	return true
}

func validIdentifiers(values []string, maximum int) bool {
	if len(values) > maximum {
		return false
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validIdentifier(value, 128) {
			return false
		}
		if _, found := seen[value]; found {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validDate(value Date) bool {
	if len(value) != len("2006-01-02") {
		return false
	}
	_, err := time.Parse("2006-01-02", string(value))
	return err == nil
}

func validTimestamp(value string) bool {
	if len(value) != len("2006-01-02 15:04:05") {
		return false
	}
	_, err := time.Parse("2006-01-02 15:04:05", value)
	return err == nil
}

func validDecimal(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	dot := false
	for index := range value {
		character := value[index]
		if character == '.' {
			if dot || index == 0 || index == len(value)-1 {
				return false
			}
			dot = true
			continue
		}
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func prepareCallOptions(operation string, options []socialhub.CallOption) ([]socialhub.CallOption, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return nil, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" {
		return nil, invalidArgument(operation, "Petal Ads does not document caller-supplied request IDs")
	}
	if resolved.IdempotencyKey != "" || len(resolved.Fields) > 0 {
		return nil, invalidArgument(operation, "idempotency keys and field selection are not supported")
	}
	if resolved.Timeout > 0 {
		return []socialhub.CallOption{socialhub.WithCallTimeout(resolved.Timeout)}, nil
	}
	return nil, nil
}

func validApprovalScopes(scopes []string) bool {
	allowed := make(map[string]struct{}, len(requiredOAuthScopes))
	for _, scope := range requiredOAuthScopes {
		allowed[scope] = struct{}{}
	}
	seen := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		if _, ok := allowed[scope]; !ok || !validOpaque(scope, 256) {
			return false
		}
		if _, duplicate := seen[scope]; duplicate {
			return false
		}
		seen[scope] = struct{}{}
	}
	return true
}

func completeOAuthScopes(scopes []string) bool {
	return len(scopes) == len(requiredOAuthScopes) && validApprovalScopes(scopes)
}

func validateCampaignRequest(input ListCampaignsRequest) error {
	if input.Page < 1 || input.Page > 10_000 || input.PageSize < 10 || input.PageSize > 50 {
		return invalidArgument("campaigns_list", "page must be 1..10000 and page size must be 10..50")
	}
	filter := input.Filter
	if filter.Name != "" && (!validOpaque(filter.Name, 100) || strings.ContainsAny(filter.Name, "^|")) ||
		!validIDs(filter.IDs, 100) ||
		!validTimeRange(filter.UpdatedBeginTime, filter.UpdatedEndTime) ||
		!validTimeRange(filter.CreatedBeginTime, filter.CreatedEndTime) ||
		!validOptionalIdentifier(filter.ShowStatus) || !validOptionalIdentifier(filter.CampaignType) {
		return invalidArgument("campaigns_list", "Campaign filters are invalid")
	}
	return nil
}

func validTimeRange(begin, end string) bool {
	if begin == "" && end == "" {
		return true
	}
	if begin == "" || end == "" || !validTimestamp(begin) || !validTimestamp(end) {
		return false
	}
	return begin <= end
}

func validOptionalIdentifier(value string) bool {
	return value == "" || validIdentifier(value, 128)
}

type reportKind uint8

const (
	reportAdvertiser reportKind = iota
	reportCampaign
	reportAdGroup
	reportCreative
	reportCountry
)

func validateReportBase(operation string, input ReportBase, kind reportKind) error {
	if !validDate(input.StartDate) || !validDate(input.EndDate) || input.EndDate < input.StartDate {
		return invalidArgument(operation, "start and end dates must be valid yyyy-MM-dd values in ascending order")
	}
	if input.TimeGranularity != "" && input.TimeGranularity != TimeGranularityDaily && input.TimeGranularity != TimeGranularitySummary ||
		kind != reportAdvertiser && input.TimeGranularity == "" {
		return invalidArgument(operation, "time granularity must be DAILY or SUMMARY and is required for this report")
	}
	if kind == reportCountry {
		if input.Page != 0 || input.PageSize != 0 || len(input.GroupBy) > 0 {
			return invalidArgument(operation, "Country reports do not accept page, page size, or group-by fields")
		}
	} else if input.Page < 0 || input.Page > 10_000 || input.PageSize < 0 || input.PageSize > 10_000 {
		return invalidArgument(operation, "page and page size must be 0 or within 1..10000")
	}
	if kind == reportAdvertiser && (input.OrderField != "" || input.OrderType != "") {
		return invalidArgument(operation, "advertiser reports do not accept order fields")
	}
	if (input.OrderField == "") != (input.OrderType == "") ||
		input.OrderField != "" && !validIdentifier(input.OrderField, 128) ||
		input.OrderType != "" && input.OrderType != OrderAscending && input.OrderType != OrderDescending {
		return invalidArgument(operation, "order field and ASC or DESC order type must be supplied together")
	}
	if input.TopN < 0 || input.TopN > 10_000 || input.FlowResource < 0 || input.FlowResource > 100 ||
		input.CampaignType < 0 || input.CampaignType > CampaignTypeShopping {
		return invalidArgument(operation, "top-N, flow resource, or Campaign type is outside its supported range")
	}
	if input.TimeLine != "" && !validIdentifier(input.TimeLine, 128) || !validIdentifiers(input.GroupBy, 32) ||
		!validCountries(input.TargetCountries) {
		return invalidArgument(operation, "time line, group-by, or target-country values are invalid")
	}
	if len(input.TargetCountries) > 0 && input.TimeGranularity != TimeGranularitySummary {
		return invalidArgument(operation, "target countries require SUMMARY time granularity")
	}
	if !validMetricFilters(input.MetricFilters) || !validDimension(input.Dimension, kind) {
		return invalidArgument(operation, "metric or dimension filters are invalid")
	}
	return nil
}

func validMetricFilters(filters []MetricFilter) bool {
	if len(filters) > 5 {
		return false
	}
	seen := make(map[Metric]struct{}, len(filters))
	for _, filter := range filters {
		switch filter.Metric {
		case MetricCost, MetricImpressions, MetricClicks, MetricCPC, MetricDownloads:
		default:
			return false
		}
		if _, found := seen[filter.Metric]; found {
			return false
		}
		seen[filter.Metric] = struct{}{}
		lowOK := filter.LowValue != nil && validDecimal(string(*filter.LowValue))
		highOK := filter.HighValue != nil && validDecimal(string(*filter.HighValue))
		switch filter.Type {
		case MetricBetween:
			if !lowOK || !highOK || compareDecimal(*filter.LowValue, *filter.HighValue) > 0 {
				return false
			}
		case MetricGreaterOrEqual:
			if !lowOK || filter.HighValue != nil {
				return false
			}
		case MetricLessOrEqual:
			if filter.LowValue != nil || !highOK {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func compareDecimal(left, right Decimal) int {
	leftText, rightText := strings.TrimLeft(string(left), "0"), strings.TrimLeft(string(right), "0")
	leftParts, rightParts := strings.SplitN(leftText, ".", 2), strings.SplitN(rightText, ".", 2)
	if leftParts[0] == "" {
		leftParts[0] = "0"
	}
	if rightParts[0] == "" {
		rightParts[0] = "0"
	}
	if len(leftParts[0]) < len(rightParts[0]) {
		return -1
	}
	if len(leftParts[0]) > len(rightParts[0]) {
		return 1
	}
	if leftParts[0] < rightParts[0] {
		return -1
	}
	if leftParts[0] > rightParts[0] {
		return 1
	}
	leftFraction, rightFraction := "", ""
	if len(leftParts) == 2 {
		leftFraction = strings.TrimRight(leftParts[1], "0")
	}
	if len(rightParts) == 2 {
		rightFraction = strings.TrimRight(rightParts[1], "0")
	}
	maximum := len(leftFraction)
	if len(rightFraction) > maximum {
		maximum = len(rightFraction)
	}
	leftFraction += strings.Repeat("0", maximum-len(leftFraction))
	rightFraction += strings.Repeat("0", maximum-len(rightFraction))
	return strings.Compare(leftFraction, rightFraction)
}

func validDimension(filter *DimensionFilter, kind reportKind) bool {
	if filter == nil {
		return true
	}
	if filter.Dimension != "adposition_id" && (filter.Dimension != "country" || kind == reportCountry) ||
		len(filter.Data) > 1_000 {
		return false
	}
	if filter.Dimension == "country" {
		return validCountries(filter.Data)
	}
	for _, value := range filter.Data {
		if !validOpaque(value, 128) {
			return false
		}
	}
	return true
}

func validCountries(values []string) bool {
	if len(values) > 256 {
		return false
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if len(value) != 2 || value[0] < 'A' || value[0] > 'Z' || value[1] < 'A' || value[1] > 'Z' {
			return false
		}
		if _, found := seen[value]; found {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validateCampaignReportFilter(filter CampaignReportFilter) bool {
	return validIDs(filter.CampaignIDs, 100) && validOptionalName(filter.CampaignName) &&
		validIdentifiers(filter.ProductTypes, 100)
}

func validateAdGroupReportFilter(filter AdGroupReportFilter) bool {
	return validIDs(filter.CampaignIDs, 100) && validOptionalName(filter.CampaignName) &&
		validIDs(filter.AdGroupIDs, 100) && validOptionalName(filter.AdGroupName) &&
		validIdentifiers(filter.ProductTypes, 100) && validIDs(filter.AppIDs, 100) &&
		validOpaqueList(filter.AppChannelPackageIDs, 100, 128) && validOptionalName(filter.PlacementName) &&
		validIdentifiers(filter.Pricings, 100)
}

func validateCreativeReportFilter(filter CreativeReportFilter) bool {
	return validIDs(filter.CampaignIDs, 100) && validOptionalName(filter.CampaignName) &&
		validIDs(filter.AdGroupIDs, 100) && validOptionalName(filter.AdGroupName) &&
		validIDs(filter.CreativeIDs, 100) && validOptionalName(filter.PlacementName) &&
		validIdentifiers(filter.Pricings, 100)
}

func validateCountryReportFilter(filter CountryReportFilter) bool {
	if filter.Type == "" {
		return len(filter.CampaignIDs) == 0 && len(filter.AdGroupIDs) == 0 && len(filter.CreativeIDs) == 0
	}
	if !validIDs(filter.CampaignIDs, 100) || !validIDs(filter.AdGroupIDs, 100) || !validIDs(filter.CreativeIDs, 100) {
		return false
	}
	switch filter.Type {
	case CountryFilterCampaign:
		return len(filter.AdGroupIDs) == 0 && len(filter.CreativeIDs) == 0
	case CountryFilterAdGroup:
		return len(filter.CampaignIDs) == 0 && len(filter.CreativeIDs) == 0
	case CountryFilterCreative:
		return len(filter.CampaignIDs) == 0 && len(filter.AdGroupIDs) == 0
	default:
		return false
	}
}

func validOptionalName(value string) bool {
	return value == "" || validOpaque(value, 100)
}

func validOpaqueList(values []string, maximum, itemMaximum int) bool {
	if len(values) > maximum {
		return false
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validOpaque(value, itemMaximum) {
			return false
		}
		if _, found := seen[value]; found {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validReportRow(row ReportRow) bool {
	if len(row) > 512 {
		return false
	}
	for key, value := range row {
		if !validIdentifier(key, 128) || !value.Null && !validResponseText(value.Text, 4096) {
			return false
		}
	}
	return true
}

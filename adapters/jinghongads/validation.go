package jinghongads

import (
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

func validCallbackURL(value string) bool {
	if len(value) > 255 {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil &&
		parsed.RawQuery == "" && parsed.Fragment == ""
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

func prepareCallOptions(operation string, options []socialhub.CallOption) (socialhub.CallOptions, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return socialhub.CallOptions{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" {
		return socialhub.CallOptions{}, invalidArgument(operation, "Jinghong does not document a caller request-ID header")
	}
	if resolved.IdempotencyKey != "" || len(resolved.Fields) > 0 {
		return socialhub.CallOptions{}, invalidArgument(operation, "idempotency keys and field selection are not supported")
	}
	return resolved, nil
}

func forwardCallOptions(options socialhub.CallOptions) []socialhub.CallOption {
	if options.Timeout == 0 {
		return nil
	}
	return []socialhub.CallOption{socialhub.WithCallTimeout(options.Timeout)}
}

func validateCampaignRequest(input ListCampaignsRequest) error {
	if input.Page < 1 || input.Page > 10_000 || input.PageSize < 10 || input.PageSize > 50 {
		return invalidArgument("campaigns_list", "page must be 1..10000 and page size must be 10..50")
	}
	filter := input.Filter
	if filter.Name != "" && (!validOpaque(filter.Name, 100) || strings.ContainsAny(filter.Name, "^|")) ||
		!validIDs(filter.IDs, 100) || !validTimeRange(filter.UpdatedBeginTime, filter.UpdatedEndTime) ||
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

func validOptionalIdentifier(value string) bool { return value == "" || validIdentifier(value, 128) }

type reportKind uint8

const (
	reportAdvertiser reportKind = iota
	reportCampaign
	reportAdGroup
	reportCreative
)

func validateReportBase(operation string, input ReportBase, kind reportKind) error {
	if !validDate(input.StartDate) || !validDate(input.EndDate) || input.EndDate < input.StartDate {
		return invalidArgument(operation, "start and end dates must be valid yyyy-MM-dd values in ascending order")
	}
	switch input.TimeGranularity {
	case "", TimeGranularityHourly, TimeGranularityDaily, TimeGranularityMonthly, TimeGranularitySummary:
	default:
		return invalidArgument(operation, "time granularity is invalid")
	}
	if input.Page < 0 || input.Page > 10_000 || input.PageSize < 0 || input.PageSize > 10_000 {
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
	if input.TimeLine != "" && !validIdentifier(input.TimeLine, 128) || !validGroupBy(input.GroupBy) ||
		!validMetricFilters(input.MetricFilters) || !validDimension(input.Dimension) {
		return invalidArgument(operation, "time line, group-by, metric, or dimension filters are invalid")
	}
	return nil
}

func validGroupBy(values []GroupBy) bool {
	if len(values) > 10 {
		return false
	}
	seen := make(map[GroupBy]struct{}, len(values))
	for _, value := range values {
		switch value {
		case GroupByDate, GroupByHour, GroupByAdGroupID, GroupByCountry, GroupBySearchWord,
			GroupByDealID, GroupByCampaignID, GroupByAdvertiserID, GroupByCreativeID, GroupByPositionID:
		default:
			return false
		}
		if _, found := seen[value]; found {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validMetricFilters(filters []MetricFilter) bool {
	if len(filters) > 7 {
		return false
	}
	seen := make(map[Metric]struct{}, len(filters))
	for _, filter := range filters {
		switch filter.Metric {
		case MetricCost, MetricImpressions, MetricClicks, MetricCPC, MetricDownloads,
			MetricClickRate, MetricClickDownloadRate:
		default:
			return false
		}
		if _, found := seen[filter.Metric]; found {
			return false
		}
		seen[filter.Metric] = struct{}{}
		lowOK := filter.LowValue != nil && validDecimal(string(*filter.LowValue))
		highOK := filter.HighValue != nil && validDecimal(string(*filter.HighValue))
		switch filter.Mode {
		case MetricGreaterOrEqual:
			if !lowOK || filter.HighValue != nil {
				return false
			}
		case MetricLessOrEqual:
			if filter.LowValue != nil || !highOK {
				return false
			}
		case MetricBetween:
			if !lowOK || !highOK || compareDecimal(*filter.LowValue, *filter.HighValue) > 0 {
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

func validDimension(filter *DimensionFilter) bool {
	if filter == nil {
		return true
	}
	if !validIdentifier(filter.Dimension, 128) || len(filter.Data) > 1_000 {
		return false
	}
	for _, value := range filter.Data {
		if !validOpaque(value, 128) {
			return false
		}
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

func validOptionalName(value string) bool { return value == "" || validOpaque(value, 100) }

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

func validResponseText(value string, maximum int) bool {
	return utf8.ValidString(value) && len(value) <= maximum && !strings.ContainsRune(value, '\x00')
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

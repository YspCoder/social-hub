package admob

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

func validEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == "" && !strings.HasSuffix(value, "/")
}

func validCallbackURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Host != "" && parsed.User == nil && parsed.Fragment == "" &&
		(parsed.Scheme == "https" || parsed.Scheme == "http")
}

func validOpaque(value string, maximum int) bool {
	return strings.TrimSpace(value) != "" && value == strings.TrimSpace(value) && len(value) <= maximum &&
		!strings.ContainsAny(value, "\x00\r\n")
}

func validPublisherID(value string) bool {
	if !strings.HasPrefix(value, "pub-") || len(value) <= len("pub-") || len(value) > 64 {
		return false
	}
	return digits(strings.TrimPrefix(value, "pub-"), 60)
}

func digits(value string, maximum int) bool {
	if value == "" || len(value) > maximum {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func validText(value string, maximumBytes int, required bool) bool {
	if required && strings.TrimSpace(value) == "" || value != strings.TrimSpace(value) ||
		!utf8.ValidString(value) || len([]byte(value)) > maximumBytes || strings.ContainsAny(value, "\x00\r\n") {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validPageToken(value string) bool { return value == "" || validOpaque(value, 4096) }

func listQuery(operation string, input ListRequest, inventory bool) (url.Values, error) {
	maximum := int32(0)
	if inventory {
		maximum = DefaultQuotaPolicy().MaximumInventoryPageSize
	}
	if input.PageSize < 0 || maximum > 0 && input.PageSize > maximum || !validPageToken(input.PageToken) {
		return nil, invalidArgument(operation, "pagination is invalid")
	}
	query := make(url.Values)
	if input.PageSize > 0 {
		query.Set("pageSize", strconv.FormatInt(int64(input.PageSize), 10))
	}
	if input.PageToken != "" {
		query.Set("pageToken", input.PageToken)
	}
	return query, nil
}

func validOAuthScopes(scopes []string) bool {
	if len(scopes) == 0 || len(scopes) > 2 {
		return false
	}
	seen := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		if scope != readOnlyScope && scope != reportScope {
			return false
		}
		if _, exists := seen[scope]; exists {
			return false
		}
		seen[scope] = struct{}{}
	}
	return true
}

func validCurrency(value string, required bool) bool {
	if value == "" {
		return !required
	}
	if len(value) != 3 {
		return false
	}
	for _, character := range value {
		if character < 'A' || character > 'Z' {
			return false
		}
	}
	return true
}

func validLanguageCode(value string, required bool) bool {
	if value == "" {
		return !required
	}
	if len(value) > 64 || value != strings.TrimSpace(value) || strings.HasPrefix(value, "-") ||
		strings.HasSuffix(value, "-") || strings.Contains(value, "--") {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func validOutputTimeZone(value string) bool {
	if !validText(value, 128, true) || strings.HasPrefix(value, "/") || strings.HasSuffix(value, "/") || strings.Contains(value, "//") {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || strings.ContainsRune("_+-/", character) {
			continue
		}
		return false
	}
	return true
}

func validDate(value Date) bool {
	if value.Year < 1 || value.Year > 9999 || value.Month < 1 || value.Month > 12 || value.Day < 1 || value.Day > 31 {
		return false
	}
	parsed := dateTime(value)
	year, month, day := parsed.Date()
	return year == int(value.Year) && month == time.Month(value.Month) && day == int(value.Day)
}

func dateTime(value Date) time.Time {
	return time.Date(int(value.Year), time.Month(value.Month), int(value.Day), 0, 0, 0, 0, time.UTC)
}

func validDateRange(value DateRange) bool {
	return validDate(value.StartDate) && validDate(value.EndDate) && !dateTime(value.EndDate).Before(dateTime(value.StartDate))
}

func accountNameFor(publisherID string) string { return "accounts/" + publisherID }

func validPublisherAccount(value PublisherAccount, expectedPublisherID string) bool {
	if !validPublisherID(value.PublisherID) || value.Name != accountNameFor(value.PublisherID) ||
		!validCurrency(value.CurrencyCode, true) || !validOutputTimeZone(value.ReportingTimeZone) || len(value.Raw) == 0 {
		return false
	}
	return expectedPublisherID == "" || value.PublisherID == expectedPublisherID
}

func resourceFragment(name, publisherID, collection string) (string, bool) {
	prefix := accountNameFor(publisherID) + "/" + collection + "/"
	if !strings.HasPrefix(name, prefix) {
		return "", false
	}
	fragment := strings.TrimPrefix(name, prefix)
	return fragment, digits(fragment, 64)
}

func externalIDFragment(value, publisherID, separator string) (string, bool) {
	prefix := "ca-app-" + publisherID + separator
	if !strings.HasPrefix(value, prefix) {
		return "", false
	}
	fragment := strings.TrimPrefix(value, prefix)
	return fragment, digits(fragment, 64)
}

func validApp(value App, publisherID string) bool {
	nameFragment, validName := resourceFragment(value.Name, publisherID, "apps")
	idFragment, validID := externalIDFragment(value.AppID, publisherID, "~")
	if !validName || !validID || nameFragment != idFragment || len(value.Raw) == 0 ||
		value.Platform != AppPlatformIOS && value.Platform != AppPlatformAndroid {
		return false
	}
	switch value.AppApprovalState {
	case AppApprovalActionRequired, AppApprovalInReview, AppApprovalApproved:
	default:
		return false
	}
	if value.ManualAppInfo != nil && !validText(value.ManualAppInfo.DisplayName, 80, false) {
		return false
	}
	if value.LinkedAppInfo != nil && (!validText(value.LinkedAppInfo.AppStoreID, 256, true) ||
		!validText(value.LinkedAppInfo.DisplayName, 512, false)) {
		return false
	}
	return true
}

func validAdFormat(value AdFormat) bool {
	switch value {
	case AdFormatAppOpen, AdFormatBanner, AdFormatBannerInterstitial, AdFormatInterstitial,
		AdFormatNative, AdFormatRewarded, AdFormatRewardedInterstitial:
		return true
	default:
		return false
	}
}

func validAdUnit(value AdUnit, publisherID string) bool {
	nameFragment, validName := resourceFragment(value.Name, publisherID, "adUnits")
	idFragment, validID := externalIDFragment(value.AdUnitID, publisherID, "/")
	_, validAppID := externalIDFragment(value.AppID, publisherID, "~")
	if !validName || !validID || !validAppID || nameFragment != idFragment || len(value.Raw) == 0 ||
		!validText(value.DisplayName, 80, true) || !validAdFormat(value.AdFormat) || len(value.AdTypes) == 0 || len(value.AdTypes) > 2 {
		return false
	}
	seen := make(map[AdType]struct{}, len(value.AdTypes))
	for _, adType := range value.AdTypes {
		if adType != AdTypeRichMedia && adType != AdTypeVideo {
			return false
		}
		if _, exists := seen[adType]; exists {
			return false
		}
		seen[adType] = struct{}{}
	}
	return true
}

var networkDimensions = map[Dimension]struct{}{
	DimensionDate: {}, DimensionMonth: {}, DimensionWeek: {}, DimensionAdUnit: {}, DimensionApp: {},
	DimensionAdType: {}, DimensionCountry: {}, DimensionFormat: {}, DimensionPlatform: {},
	DimensionMobileOSVersion: {}, DimensionGMASDKVersion: {}, DimensionAppVersionName: {}, DimensionServingRestriction: {},
}

var mediationDimensions = map[Dimension]struct{}{
	DimensionDate: {}, DimensionMonth: {}, DimensionWeek: {}, DimensionAdSource: {}, DimensionAdSourceInstance: {},
	DimensionAdUnit: {}, DimensionApp: {}, DimensionMediationGroup: {}, DimensionCountry: {}, DimensionFormat: {},
	DimensionPlatform: {}, DimensionMobileOSVersion: {}, DimensionGMASDKVersion: {}, DimensionAppVersionName: {},
	DimensionServingRestriction: {},
}

var networkMetrics = map[Metric]struct{}{
	MetricAdRequests: {}, MetricClicks: {}, MetricEstimatedEarnings: {}, MetricImpressions: {},
	MetricImpressionCTR: {}, MetricImpressionRPM: {}, MetricMatchedRequests: {}, MetricMatchRate: {}, MetricShowRate: {},
}

var mediationMetrics = map[Metric]struct{}{
	MetricAdRequests: {}, MetricClicks: {}, MetricEstimatedEarnings: {}, MetricImpressions: {},
	MetricImpressionCTR: {}, MetricMatchedRequests: {}, MetricMatchRate: {}, MetricObservedECPM: {},
}

func validUniqueDimensions(values []Dimension, allowed map[Dimension]struct{}) bool {
	if len(values) > len(allowed) {
		return false
	}
	seen := make(map[Dimension]struct{}, len(values))
	timeDimensions := 0
	for _, value := range values {
		if _, exists := allowed[value]; !exists {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
		if value == DimensionDate || value == DimensionMonth || value == DimensionWeek {
			timeDimensions++
		}
	}
	return timeDimensions <= 1
}

func validUniqueMetrics(values []Metric, allowed map[Metric]struct{}) bool {
	if len(values) == 0 || len(values) > len(allowed) {
		return false
	}
	seen := make(map[Metric]struct{}, len(values))
	for _, value := range values {
		if _, exists := allowed[value]; !exists {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validDimensionFilters(values []DimensionFilter, allowed map[Dimension]struct{}) bool {
	if len(values) > len(allowed) {
		return false
	}
	for _, filter := range values {
		if _, exists := allowed[filter.Dimension]; !exists || len(filter.MatchesAny.Values) == 0 || len(filter.MatchesAny.Values) > 1_000 {
			return false
		}
		seen := make(map[string]struct{}, len(filter.MatchesAny.Values))
		for _, value := range filter.MatchesAny.Values {
			if !validText(value, 4096, true) {
				return false
			}
			if _, exists := seen[value]; exists {
				return false
			}
			seen[value] = struct{}{}
		}
	}
	return true
}

func validSortConditions(values []SortCondition, dimensions map[Dimension]struct{}, metrics map[Metric]struct{}) bool {
	if len(values) > len(dimensions)+len(metrics) {
		return false
	}
	for _, condition := range values {
		if condition.Order != SortAscending && condition.Order != SortDescending {
			return false
		}
		dimensionSet := condition.Dimension != ""
		metricSet := condition.Metric != ""
		if dimensionSet == metricSet {
			return false
		}
		if dimensionSet {
			if _, exists := dimensions[condition.Dimension]; !exists {
				return false
			}
		} else if _, exists := metrics[condition.Metric]; !exists {
			return false
		}
	}
	return true
}

func validLocalization(value *LocalizationSettings) bool {
	return value == nil || validCurrency(value.CurrencyCode, false) && validLanguageCode(value.LanguageCode, false)
}

func validReportTimeZone(value string) bool { return value == "" || value == "America/Los_Angeles" }

func validReportCommon(dateRange DateRange, localization *LocalizationSettings, timeZone string, maxRows int32) bool {
	return validDateRange(dateRange) && validLocalization(localization) && validReportTimeZone(timeZone) &&
		maxRows >= 0 && maxRows <= DefaultQuotaPolicy().MaximumReportRows
}

func containsDimension(values []Dimension, target Dimension) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func containsMetric(values []Metric, targets ...Metric) bool {
	for _, value := range values {
		for _, target := range targets {
			if value == target {
				return true
			}
		}
	}
	return false
}

func validNetworkReportSpec(value NetworkReportSpec) bool {
	if !validReportCommon(value.DateRange, value.LocalizationSettings, value.TimeZone, value.MaxReportRows) ||
		!validUniqueDimensions(value.Dimensions, networkDimensions) || !validUniqueMetrics(value.Metrics, networkMetrics) ||
		!validDimensionFilters(value.DimensionFilters, networkDimensions) ||
		!validSortConditions(value.SortConditions, networkDimensions, networkMetrics) {
		return false
	}
	return !containsDimension(value.Dimensions, DimensionAdType) ||
		!containsMetric(value.Metrics, MetricAdRequests, MetricMatchRate, MetricImpressionRPM)
}

func validMediationReportSpec(value MediationReportSpec) bool {
	if !validReportCommon(value.DateRange, value.LocalizationSettings, value.TimeZone, value.MaxReportRows) ||
		!validUniqueDimensions(value.Dimensions, mediationDimensions) || !validUniqueMetrics(value.Metrics, mediationMetrics) ||
		!validDimensionFilters(value.DimensionFilters, mediationDimensions) ||
		!validSortConditions(value.SortConditions, mediationDimensions, mediationMetrics) {
		return false
	}
	incompatibleDimension := containsDimension(value.Dimensions, DimensionMobileOSVersion) ||
		containsDimension(value.Dimensions, DimensionGMASDKVersion) || containsDimension(value.Dimensions, DimensionAppVersionName)
	return !incompatibleDimension || !containsMetric(value.Metrics, MetricEstimatedEarnings, MetricObservedECPM)
}

type reportExpectation struct {
	DateRange    DateRange
	Dimensions   []Dimension
	Metrics      []Metric
	Localization *LocalizationSettings
	TimeZone     string
	MaximumRows  int32
}

func expectedDimensions(values []Dimension) []Dimension {
	result := append([]Dimension(nil), values...)
	if containsDimension(values, DimensionAdUnit) && !containsDimension(values, DimensionApp) {
		result = append(result, DimensionApp)
	}
	return result
}

func validReportHeader(value ReportHeader, expected reportExpectation) bool {
	if !validDateRange(value.DateRange) || value.DateRange != expected.DateRange ||
		!validCurrency(value.LocalizationSettings.CurrencyCode, true) ||
		!validLanguageCode(value.LocalizationSettings.LanguageCode, true) || !validOutputTimeZone(value.ReportingTimeZone) {
		return false
	}
	if expected.TimeZone != "" && value.ReportingTimeZone != expected.TimeZone {
		return false
	}
	if expected.Localization != nil {
		if expected.Localization.CurrencyCode != "" && value.LocalizationSettings.CurrencyCode != expected.Localization.CurrencyCode ||
			expected.Localization.LanguageCode != "" && value.LocalizationSettings.LanguageCode != expected.Localization.LanguageCode {
			return false
		}
	}
	return true
}

func metricKind(value Metric) string {
	switch value {
	case MetricAdRequests, MetricClicks, MetricImpressions, MetricMatchedRequests:
		return "integer"
	case MetricEstimatedEarnings, MetricImpressionRPM, MetricObservedECPM:
		return "micros"
	case MetricImpressionCTR, MetricMatchRate, MetricShowRate:
		return "double"
	default:
		return ""
	}
}

func validMetricValue(metric Metric, value MetricValue) bool {
	fields := 0
	if value.IntegerValue != nil {
		fields++
	}
	if value.MicrosValue != nil {
		fields++
	}
	if value.DoubleValue != nil {
		fields++
	}
	if fields != 1 || len(value.Raw) == 0 {
		return false
	}
	switch metricKind(metric) {
	case "integer":
		return value.IntegerValue != nil && *value.IntegerValue >= 0
	case "micros":
		return value.MicrosValue != nil
	case "double":
		return value.DoubleValue != nil && *value.DoubleValue >= 0 && *value.DoubleValue <= 1
	default:
		return false
	}
}

func validReportRow(value ReportRow, expected reportExpectation) bool {
	dimensions := expectedDimensions(expected.Dimensions)
	if len(value.DimensionValues) != len(dimensions) || len(value.MetricValues) != len(expected.Metrics) {
		return false
	}
	for _, dimension := range dimensions {
		entry, exists := value.DimensionValues[dimension]
		if !exists || !validText(entry.Value, 4096, true) || !validText(entry.DisplayLabel, 4096, false) {
			return false
		}
	}
	for _, metric := range expected.Metrics {
		entry, exists := value.MetricValues[metric]
		if !exists || !validMetricValue(metric, entry) {
			return false
		}
	}
	return true
}

func validReportFooter(value ReportFooter) bool {
	if !value.matchingPresent || value.MatchingRowCount < 0 || len(value.Raw) == 0 {
		return false
	}
	for _, warning := range value.Warnings {
		switch warning.Type {
		case ReportWarningBeforeTimeZoneChange, ReportWarningDataDelayed, ReportWarningOther, ReportWarningCurrencyDiffers:
		default:
			return false
		}
		if !validText(warning.Description, 4096, true) {
			return false
		}
	}
	return true
}

func decodeReport(reader io.Reader, expected reportExpectation) (*Report, error) {
	decoder := json.NewDecoder(reader)
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('[') {
		return nil, fmt.Errorf("response must be a JSON array")
	}
	maximumRows := expected.MaximumRows
	if maximumRows == 0 {
		maximumRows = DefaultQuotaPolicy().MaximumReportRows
	}
	result := &Report{Rows: make([]ReportRow, 0)}
	headerSeen, footerSeen := false, false
	for decoder.More() {
		var line map[string]json.RawMessage
		if err := decoder.Decode(&line); err != nil {
			return nil, err
		}
		if len(line) != 1 {
			return nil, fmt.Errorf("report line must contain exactly one message")
		}
		for kind, raw := range line {
			switch kind {
			case "header":
				if headerSeen || footerSeen || len(result.Rows) != 0 {
					return nil, fmt.Errorf("header is out of order")
				}
				if err := json.Unmarshal(raw, &result.Header); err != nil || !validReportHeader(result.Header, expected) {
					return nil, fmt.Errorf("header is invalid")
				}
				headerSeen = true
			case "row":
				if !headerSeen || footerSeen || int32(len(result.Rows)) >= maximumRows {
					return nil, fmt.Errorf("row is out of order or exceeds the requested limit")
				}
				var row ReportRow
				if err := json.Unmarshal(raw, &row); err != nil || !validReportRow(row, expected) {
					return nil, fmt.Errorf("row is invalid")
				}
				result.Rows = append(result.Rows, row)
			case "footer":
				if !headerSeen || footerSeen {
					return nil, fmt.Errorf("footer is out of order")
				}
				if err := json.Unmarshal(raw, &result.Footer); err != nil || !validReportFooter(result.Footer) {
					return nil, fmt.Errorf("footer is invalid")
				}
				footerSeen = true
			default:
				return nil, fmt.Errorf("unknown report message %q", kind)
			}
		}
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim(']') || !headerSeen || !footerSeen {
		return nil, fmt.Errorf("report is incomplete")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("report has trailing data")
	}
	return result, nil
}

func sanitizeCause(err error) error {
	if err == nil {
		return nil
	}
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}

package adsense

import (
	"errors"
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
	for _, character := range strings.TrimPrefix(value, "pub-") {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func validResourceID(value string) bool {
	if value == "" || value == "." || value == ".." || len(value) > 256 {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || strings.ContainsRune("-_.~:", character) {
			continue
		}
		return false
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

func validListRequest(value ListRequest) bool {
	return value.PageSize >= 0 && value.PageSize <= DefaultQuotaPolicy().MaximumListPageSize && validPageToken(value.PageToken)
}

func listQuery(operation string, input ListRequest) (url.Values, error) {
	if !validListRequest(input) {
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

func (client *Client) resourceName(operation, parent, collection, id string) (string, error) {
	if !validResourceID(id) {
		return "", invalidArgument(operation, collection+" ID contains unsupported characters")
	}
	return parent + "/" + collection + "/" + id, nil
}

func (client *Client) adClientName(operation, adClientID string) (string, error) {
	return client.resourceName(operation, client.accountName(), "adclients", adClientID)
}

func (client *Client) nestedName(operation, adClientID, collection, id string) (string, error) {
	parent, err := client.adClientName(operation, adClientID)
	if err != nil {
		return "", err
	}
	return client.resourceName(operation, parent, collection, id)
}

func (client *Client) ownsResource(name, parent, collection string) bool {
	prefix := parent + "/" + collection + "/"
	return strings.HasPrefix(name, prefix) && validResourceID(strings.TrimPrefix(name, prefix))
}

func (client *Client) ownsAdClient(name string) bool {
	return client.ownsResource(name, client.accountName(), "adclients")
}

func (client *Client) ownsNested(name, collection string) bool {
	prefix := client.accountName() + "/adclients/"
	if !strings.HasPrefix(name, prefix) {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(name, prefix), "/")
	return len(parts) == 3 && validResourceID(parts[0]) && parts[1] == collection && validResourceID(parts[2])
}

func validAccountName(name string) bool {
	return strings.HasPrefix(name, "accounts/") && validPublisherID(strings.TrimPrefix(name, "accounts/"))
}

func validAdClientName(name string) bool {
	if !strings.HasPrefix(name, "accounts/") {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(name, "accounts/"), "/")
	return len(parts) == 3 && validPublisherID(parts[0]) && parts[1] == "adclients" && validResourceID(parts[2])
}

func validOAuthScopes(scopes []string) bool {
	if len(scopes) == 0 || len(scopes) > 2 {
		return false
	}
	seen := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		if scope != fullScope && scope != readOnlyScope {
			return false
		}
		if _, exists := seen[scope]; exists {
			return false
		}
		seen[scope] = struct{}{}
	}
	return true
}

func validLanguageCode(value string) bool {
	if value == "" {
		return true
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

func validCurrency(value string) bool {
	if value == "" {
		return true
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

func validDate(value Date) bool {
	if value.Year < 1 || value.Year > 9999 || value.Month < 1 || value.Month > 12 || value.Day < 1 || value.Day > 31 {
		return false
	}
	parsed := dateTime(value)
	year, month, day := parsed.Date()
	return year == int(value.Year) && month == time.Month(value.Month) && day == int(value.Day)
}

func zeroDate(value Date) bool { return value == (Date{}) }

func dateTime(value Date) time.Time {
	return time.Date(int(value.Year), time.Month(value.Month), int(value.Day), 0, 0, 0, 0, time.UTC)
}

func validDateRange(kind ReportDateRange, start, end Date) bool {
	switch kind {
	case "", ReportDateCustom:
		return validDate(start) && validDate(end) && !dateTime(end).Before(dateTime(start))
	case ReportDateToday, ReportDateYesterday, ReportDateMonthToDate, ReportDateYearToDate, ReportDateLast7Days, ReportDateLast30Days:
		return zeroDate(start) && zeroDate(end)
	default:
		return false
	}
}

func validReportingTimeZone(value ReportingTimeZone) bool {
	return value == "" || value == ReportingTimeZoneAccount || value == ReportingTimeZoneGoogle
}

func validEnumName(value string) bool {
	if value == "" || len(value) > 128 || value[0] < 'A' || value[0] > 'Z' {
		return false
	}
	for _, character := range value[1:] {
		if character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '_' {
			continue
		}
		return false
	}
	return true
}

func uniqueReportFields[T ~string](values []T, maximum int, required bool) bool {
	if len(values) > maximum || required && len(values) == 0 {
		return false
	}
	seen := make(map[string]struct{}, len(values))
	for _, typed := range values {
		value := string(typed)
		if !validEnumName(value) {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validQueryExpressions(values []string) bool {
	if len(values) > 100 {
		return false
	}
	for _, value := range values {
		if !validText(value, 2048, true) {
			return false
		}
	}
	return true
}

func validGenerateReport(value GenerateReportRequest) bool {
	return uniqueReportFields(value.Dimensions, 100, false) && uniqueReportFields(value.Metrics, 100, true) &&
		validDateRange(value.DateRange, value.StartDate, value.EndDate) && validReportingTimeZone(value.ReportingTimeZone) &&
		validCurrency(value.CurrencyCode) && validLanguageCode(value.LanguageCode) && validQueryExpressions(value.Filters) &&
		validQueryExpressions(value.OrderBy) && value.Limit >= 0 && value.Limit <= DefaultQuotaPolicy().MaximumJSONReportRows
}

func validGenerateSavedReport(value GenerateSavedReportRequest) bool {
	return validDateRange(value.DateRange, value.StartDate, value.EndDate) && validReportingTimeZone(value.ReportingTimeZone) &&
		validCurrency(value.CurrencyCode) && validLanguageCode(value.LanguageCode)
}

func validHeaderType(value HeaderType, dimension bool) bool {
	if dimension {
		return value == HeaderDimension
	}
	switch value {
	case HeaderMetricTally, HeaderMetricRatio, HeaderMetricCurrency, HeaderMetricMilliseconds, HeaderMetricDecimal:
		return true
	default:
		return false
	}
}

func validReportShape(result *ReportResult, expected []string, expectedDimensions int) bool {
	if result == nil || !result.totalRowsPresent || result.TotalMatchedRows < 0 || len(result.Headers) == 0 ||
		result.TotalMatchedRows < int64(len(result.Rows)) || !validDate(result.StartDate) || !validDate(result.EndDate) ||
		dateTime(result.EndDate).Before(dateTime(result.StartDate)) {
		return false
	}
	if expected != nil && len(result.Headers) != len(expected) {
		return false
	}
	seen := make(map[string]struct{}, len(result.Headers))
	seenMetric := false
	for index, header := range result.Headers {
		if !validEnumName(header.Name) {
			return false
		}
		if _, exists := seen[header.Name]; exists {
			return false
		}
		seen[header.Name] = struct{}{}
		if expected != nil && header.Name != expected[index] {
			return false
		}
		dimension := header.Type == HeaderDimension
		if expectedDimensions >= 0 && dimension != (index < expectedDimensions) {
			return false
		}
		if dimension && seenMetric || !validHeaderType(header.Type, dimension) || !validCurrency(header.CurrencyCode) {
			return false
		}
		seenMetric = seenMetric || !dimension
	}
	if !seenMetric {
		return false
	}
	width := len(result.Headers)
	for _, row := range result.Rows {
		if len(row.Cells) != width {
			return false
		}
	}
	if result.Totals != nil && len(result.Totals.Cells) != width || result.Averages != nil && len(result.Averages.Cells) != width {
		return false
	}
	return true
}

func expectedHeaders(value GenerateReportRequest) []string {
	result := make([]string, 0, len(value.Dimensions)+len(value.Metrics))
	for _, field := range value.Dimensions {
		result = append(result, string(field))
	}
	for _, field := range value.Metrics {
		result = append(result, string(field))
	}
	return result
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

package admanager

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

func validID(value string) bool {
	if value == "" || len(value) > 20 {
		return false
	}
	nonzero := false
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
		nonzero = nonzero || character != '0'
	}
	return nonzero
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

func validName(value string, maximumRunes int) bool {
	return validText(value, maximumRunes*4, true) && utf8.RuneCountInString(value) <= maximumRunes
}

func validPageToken(value string) bool { return value == "" || validOpaque(value, 4096) }

func validListRequest(value ListRequest, maximum int32) bool {
	return value.PageSize >= 0 && value.PageSize <= maximum && value.Skip >= 0 && value.Skip <= 10_000_000 &&
		validPageToken(value.PageToken) && validText(value.Filter, 4096, false) && validText(value.OrderBy, 1024, false)
}

func int32String(value int32) string { return strconv.FormatInt(int64(value), 10) }

func (client *Client) resourceName(operation, resource, id string) (string, error) {
	if !validID(id) {
		return "", invalidArgument(operation, resource+" ID must be a positive string-encoded integer")
	}
	return client.networkName() + "/" + resource + "/" + id, nil
}

func (client *Client) ownsResource(name, resource string) bool {
	prefix := client.networkName() + "/" + resource + "/"
	return strings.HasPrefix(name, prefix) && validID(strings.TrimPrefix(name, prefix))
}

func (client *Client) ownsReportOperation(name string) bool {
	prefix := client.networkName() + "/operations/reports/runs/"
	return strings.HasPrefix(name, prefix) && validID(strings.TrimPrefix(name, prefix))
}

func (client *Client) ownsReportResult(name string) bool {
	prefix := client.networkName() + "/reports/"
	if !strings.HasPrefix(name, prefix) {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(name, prefix), "/")
	return len(parts) == 3 && validID(parts[0]) && parts[1] == "results" && validID(parts[2])
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

func validReportDefinition(value ReportDefinition) bool {
	if !uniqueReportFields(value.Dimensions, 100) || !uniqueReportFields(value.Metrics, 100) || len(value.Metrics) == 0 ||
		!validDateRange(value.DateRange) || value.ComparisonDateRange != nil && !validDateRange(*value.ComparisonDateRange) ||
		!validReportType(value.ReportType) || !validTimeZone(value.TimeZoneSource, value.TimeZone) || !validCurrency(value.CurrencyCode) {
		return false
	}
	return true
}

func uniqueReportFields[T ~string](values []T, maximum int) bool {
	if len(values) > maximum {
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

func validDateRange(value DateRange) bool {
	if value.Fixed != nil == (value.Relative != "") {
		return false
	}
	if value.Fixed != nil {
		return validDate(value.Fixed.StartDate) && validDate(value.Fixed.EndDate) &&
			!dateTime(value.Fixed.EndDate).Before(dateTime(value.Fixed.StartDate))
	}
	return validRelativeDateRange(value.Relative)
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

func validRelativeDateRange(value RelativeDateRange) bool {
	switch value {
	case "TODAY", "YESTERDAY", "THIS_WEEK", "THIS_WEEK_TO_DATE", "THIS_WEEK_TO_YESTERDAY",
		"THIS_MONTH", "THIS_MONTH_TO_DATE", "THIS_MONTH_TO_YESTERDAY", "THIS_QUARTER", "THIS_QUARTER_TO_DATE",
		"THIS_QUARTER_TO_YESTERDAY", "THIS_YEAR", "THIS_YEAR_TO_DATE", "THIS_YEAR_TO_YESTERDAY", "LAST_WEEK",
		"LAST_WEEK_STARTING_SUNDAY", "LAST_MONTH", "LAST_QUARTER", "LAST_YEAR", "LAST_7_DAYS", "LAST_30_DAYS",
		"LAST_60_DAYS", "LAST_90_DAYS", "LAST_93_DAYS", "LAST_180_DAYS", "LAST_360_DAYS", "LAST_365_DAYS",
		"LAST_3_MONTHS", "LAST_6_MONTHS", "LAST_12_MONTHS", "ALL_AVAILABLE", "TOMORROW", "NEXT_90_DAYS",
		"NEXT_MONTH", "NEXT_3_MONTHS", "NEXT_12_MONTHS", "NEXT_WEEK", "NEXT_QUARTER", "TO_END_OF_NEXT_MONTH",
		"PREVIOUS_PERIOD", "SAME_PERIOD_PREVIOUS_YEAR":
		return true
	default:
		return false
	}
}

func validReportType(value ReportType) bool {
	switch value {
	case "HISTORICAL", "FUTURE_SELL_THROUGH", "REACH", "PRIVACY_AND_MESSAGING", "REVENUE_VERIFICATION",
		"PARTNER_FINANCE", "AD_SPEED", "REAL_TIME_VIDEO", "YOUTUBE_CONSOLIDATED", "ADS_TRAFFIC_NAVIGATOR",
		"OFF_PROPERTY_CAMPAIGNS", "ON_PLATFORM_MULTICALL":
		return true
	default:
		return false
	}
}

func validTimeZone(source TimeZoneSource, zone string) bool {
	switch source {
	case "", TimeZonePublisher, TimeZoneAdExchange, TimeZoneUTC:
		return zone == ""
	case TimeZoneProvided:
		if !validText(zone, 128, true) {
			return false
		}
		_, err := time.LoadLocation(zone)
		return err == nil
	default:
		return false
	}
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

package marketing

import (
	"encoding/json"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

func validID(value string) bool {
	if value == "" || len(value) > 64 {
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

func validRequiredText(value string, maximum int) bool {
	return strings.TrimSpace(value) != "" && strings.TrimSpace(value) == value && len(value) <= maximum &&
		utf8.ValidString(value) && !strings.ContainsRune(value, '\x00')
}

func validFieldName(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if character != '_' && !unicode.IsLower(character) && !unicode.IsDigit(character) {
			return false
		}
	}
	return true
}

func validEnumToken(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if character != '_' && !unicode.IsUpper(character) && !unicode.IsDigit(character) {
			return false
		}
	}
	return true
}

func validatePage(page, pageSize int) (int, int, error) {
	if page < 0 || pageSize < 0 || pageSize > 1000 {
		return 0, 0, invalidArgument("pagination", "page must be non-negative and page_size must not exceed 1000")
	}
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 10
	}
	return page, pageSize, nil
}

func validateIDs(values []string, maximum int) bool {
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

func validateFields(values []string, maximum int) bool {
	if len(values) > maximum {
		return false
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validFieldName(value) {
			return false
		}
		if _, found := seen[value]; found {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validOperationStatus(value OperationStatus) bool {
	return value == StatusEnable || value == StatusDisable || value == StatusDelete
}

func validDate(value string) bool {
	parsed, err := time.Parse("2006-01-02", value)
	return err == nil && parsed.Format("2006-01-02") == value
}

func validDateTime(value string) bool {
	if value == "" {
		return true
	}
	parsed, err := time.Parse("2006-01-02 15:04:05", value)
	return err == nil && parsed.Format("2006-01-02 15:04:05") == value
}

func validHTTPURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") &&
		parsed.Host != "" && parsed.User == nil
}

func mergeFields(operation string, fixed, fields map[string]any, protected ...string) (map[string]any, error) {
	reserved := map[string]struct{}{
		"access_token": {}, "refresh_token": {}, "secret": {}, "app_id": {}, "advertiser_id": {},
	}
	for _, key := range protected {
		reserved[key] = struct{}{}
	}
	result := make(map[string]any, len(fixed)+len(fields))
	for key, value := range fixed {
		result[key] = value
	}
	for key, value := range fields {
		if !validFieldName(key) {
			return nil, invalidArgument(operation, "extension field names must be lowercase API identifiers")
		}
		_, fixedKey := fixed[key]
		_, protectedKey := reserved[key]
		if fixedKey || protectedKey {
			return nil, invalidArgument(operation, "extension fields cannot override adapter-controlled values")
		}
		if _, err := json.Marshal(value); err != nil {
			return nil, invalidArgument(operation, "extension fields must be JSON encodable")
		}
		result[key] = value
	}
	return result, nil
}

func validReportLevel(value ReportDataLevel) bool {
	return value == ReportLevelAdvertiser || value == ReportLevelCampaign ||
		value == ReportLevelAdGroup || value == ReportLevelAd
}

func reportMaximumDates(dimensions []string) int {
	for _, dimension := range dimensions {
		if dimension == "stat_time_hour" {
			return 1
		}
	}
	for _, dimension := range dimensions {
		if dimension == "stat_time_day" {
			return 30
		}
	}
	return 365
}

func inclusiveDates(start, end string) (int, bool) {
	if !validDate(start) || !validDate(end) || start > end {
		return 0, false
	}
	startDate, _ := time.Parse("2006-01-02", start)
	endDate, _ := time.Parse("2006-01-02", end)
	return int(endDate.Sub(startDate)/(24*time.Hour)) + 1, true
}

func validReportFilters(filters []ReportFilter) bool {
	if len(filters) > 100 {
		return false
	}
	for _, filter := range filters {
		if !validFieldName(filter.FieldName) || !validEnumToken(filter.FilterType) ||
			!validOpaque(filter.FilterValue, 8192) {
			return false
		}
		if filter.FilterType == "IN" || filter.FilterType == "BETWEEN" {
			var values []any
			if json.Unmarshal([]byte(filter.FilterValue), &values) != nil || len(values) == 0 ||
				filter.FilterType == "BETWEEN" && len(values) != 2 {
				return false
			}
		}
	}
	return true
}

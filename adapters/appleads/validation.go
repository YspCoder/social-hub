package appleads

import (
	"encoding/json"
	"math/big"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)

func validID(value int64) bool { return value > 0 }

func validText(value string, maximum int) bool {
	return strings.TrimSpace(value) != "" && !strings.ContainsAny(value, "\r\n\x00") && utf8.RuneCountInString(value) <= maximum
}

func validOptionalText(value *string, maximum int) bool {
	return value == nil || validText(*value, maximum)
}

func validOpaque(value string, maximum int) bool {
	return strings.TrimSpace(value) != "" && !strings.ContainsAny(value, "\r\n\x00") && len(value) <= maximum
}

func validMoney(value Money) bool {
	if !currencyPattern.MatchString(value.Currency) || strings.TrimSpace(value.Amount) != value.Amount || len(value.Amount) > 32 {
		return false
	}
	decimal := new(big.Rat)
	if _, ok := decimal.SetString(value.Amount); !ok || decimal.Sign() < 0 {
		return false
	}
	return true
}

func validPositiveMoney(value *Money) bool {
	if value == nil {
		return true
	}
	if !validMoney(*value) {
		return false
	}
	decimal, _ := new(big.Rat).SetString(value.Amount)
	return decimal.Sign() > 0
}

func validPagination(value Pagination) bool {
	return value.Offset >= 0 && value.Limit >= 1 && value.Limit <= 1000
}

func validSelector(value Selector) bool {
	if !validPagination(value.Pagination) || len(value.Conditions) > 50 || len(value.Fields) > 100 || len(value.OrderBy) > 1 {
		return false
	}
	for _, condition := range value.Conditions {
		if !validText(condition.Field, 128) || !validText(condition.Operator, 64) || len(condition.Values) == 0 || len(condition.Values) > 1000 {
			return false
		}
	}
	for _, field := range value.Fields {
		if !validText(field, 128) {
			return false
		}
	}
	for _, sorting := range value.OrderBy {
		if !validText(sorting.Field, 128) || sorting.SortOrder != SortAscending && sorting.SortOrder != SortDescending {
			return false
		}
	}
	return true
}

func validDate(value string) bool {
	if value == "" {
		return true
	}
	_, err := time.Parse("2006-01-02", value)
	return err == nil
}

func validDateTime(value string) bool {
	if value == "" {
		return true
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.000"} {
		if _, err := time.Parse(layout, value); err == nil {
			return true
		}
	}
	return false
}

func validRawObject(value json.RawMessage) bool {
	if len(value) == 0 {
		return true
	}
	var object map[string]json.RawMessage
	return json.Unmarshal(value, &object) == nil && object != nil
}

func validEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == "" && !strings.HasSuffix(value, "/")
}

func validCountry(value string) bool {
	return len(value) == 2 && value[0] >= 'A' && value[0] <= 'Z' && value[1] >= 'A' && value[1] <= 'Z'
}

func validCountries(values []string) bool {
	if len(values) == 0 || len(values) > 250 {
		return false
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validCountry(value) {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validReportRequest(value ReportingRequest) bool {
	if !validDate(value.StartTime) || value.StartTime == "" || !validDate(value.EndTime) || value.EndTime == "" || value.EndTime < value.StartTime || !validSelector(value.Selector) {
		return false
	}
	switch value.Granularity {
	case "", GranularityHourly, GranularityDaily, GranularityWeekly, GranularityMonthly:
	default:
		return false
	}
	switch value.TimeZone {
	case "", TimeZoneUTC, TimeZoneOrganization:
	default:
		return false
	}
	if value.Granularity != "" && (value.ReturnRowTotals || value.ReturnGrandTotals) {
		return false
	}
	if value.Granularity == "" && !value.ReturnRowTotals {
		return false
	}
	if len(value.GroupBy) > 7 {
		return false
	}
	allowedGroups := map[string]bool{
		"adminArea": true, "ageRange": true, "countryCode": true, "countryOrRegion": true,
		"deviceClass": true, "gender": true, "locality": true,
	}
	seen := make(map[string]struct{}, len(value.GroupBy))
	for _, group := range value.GroupBy {
		if !allowedGroups[group] {
			return false
		}
		if _, exists := seen[group]; exists {
			return false
		}
		seen[group] = struct{}{}
	}
	return true
}

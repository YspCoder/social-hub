package cm360

import (
	"errors"
	"net/url"
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

func validOptionalText(value string, maximumRunes int) bool {
	return validText(value, maximumRunes*4, false) && utf8.RuneCountInString(value) <= maximumRunes
}

func validPageToken(value string) bool { return value == "" || validOpaque(value, 4096) }

func validSearch(value string) bool { return validText(value, 512, false) }

func validDate(value string) bool {
	parsed, err := time.Parse(time.DateOnly, value)
	return err == nil && parsed.Format(time.DateOnly) == value
}

func validAbsoluteDateRange(start, end string) bool {
	if !validDate(start) || !validDate(end) {
		return false
	}
	startTime, _ := time.Parse(time.DateOnly, start)
	endTime, _ := time.Parse(time.DateOnly, end)
	return !endTime.Before(startTime)
}

func validSortOrder(value SortOrder) bool {
	return value == "" || value == SortAscending || value == SortDescending
}

func validIDs(values []string, maximum int) bool {
	if len(values) > maximum {
		return false
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validID(value) {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validListBase(maxResults int, pageToken, search string, sortOrder SortOrder) bool {
	return maxResults >= 0 && maxResults <= 1000 && validPageToken(pageToken) &&
		validSearch(search) && validSortOrder(sortOrder)
}

func validPlacementStatus(value PlacementActiveStatus) bool {
	switch value {
	case "", PlacementUnknown, PlacementActive, PlacementInactive, PlacementArchived, PlacementPermanentlyArchived:
		return true
	default:
		return false
	}
}

func validAdType(value AdType) bool {
	switch value {
	case "", AdStandard, AdDefault, AdClickTracker, AdTracking, AdBrandSafe:
		return true
	default:
		return false
	}
}

func validReportScope(value ReportScope) bool {
	return value == "" || value == ReportScopeAll || value == ReportScopeMine || value == ReportScopeSharedWithMe
}

func validReportFileStatus(value ReportFileStatus) bool {
	switch value {
	case ReportFileProcessing, ReportFileAvailable, ReportFileFailed, ReportFileCancelled, ReportFileQueued:
		return true
	default:
		return false
	}
}

func validCMField(value string) bool {
	if value == "" || len(value) > 128 || value[0] < 'A' || value[0] > 'Z' && value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for _, character := range value[1:] {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || character == '_' {
			continue
		}
		return false
	}
	return true
}

func validRelativeDateRange(value string) bool {
	switch value {
	case "TODAY", "YESTERDAY", "WEEK_TO_DATE", "MONTH_TO_DATE", "QUARTER_TO_DATE", "YEAR_TO_DATE",
		"PREVIOUS_WEEK", "PREVIOUS_MONTH", "PREVIOUS_QUARTER", "PREVIOUS_YEAR", "LAST_7_DAYS",
		"LAST_14_DAYS", "LAST_30_DAYS", "LAST_60_DAYS", "LAST_90_DAYS", "LAST_365_DAYS", "LAST_24_MONTHS":
		return true
	default:
		return false
	}
}

func validOAuthScopes(scopes []string) bool {
	if len(scopes) == 0 || len(scopes) > 3 {
		return false
	}
	seen := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		if scope != traffickingScope && scope != reportingScope && scope != conversionsScope {
			return false
		}
		if _, exists := seen[scope]; exists {
			return false
		}
		seen[scope] = struct{}{}
	}
	return true
}

func validQueryDateRange(value DateRange) bool {
	if value.RelativeDateRange != "" {
		return value.StartDate == "" && value.EndDate == "" && validRelativeDateRange(value.RelativeDateRange)
	}
	return validAbsoluteDateRange(value.StartDate, value.EndDate)
}

func uniqueFields(values []string, maximum int) bool {
	if len(values) > maximum {
		return false
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validCMField(value) {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func prepareReportDataQuery(input ReportDataQueryRequest, advertiserID string) (ReportDataQueryRequest, error) {
	if !validQueryDateRange(input.DateRange) || len(input.MetricNames) == 0 ||
		!uniqueFields(input.DimensionNames, 100) || !uniqueFields(input.MetricNames, 100) ||
		input.MaxResults < 0 || input.MaxResults > 1000 || !validPageToken(input.PageToken) ||
		len(input.DimensionFilters) > 100 || len(input.SortBys) > 100 {
		return ReportDataQueryRequest{}, invalidArgument("report_data_query", "report query fields, date range, or pagination are invalid")
	}
	requested := make(map[string]struct{}, len(input.DimensionNames)+len(input.MetricNames))
	for _, value := range input.DimensionNames {
		requested[value] = struct{}{}
	}
	for _, value := range input.MetricNames {
		requested[value] = struct{}{}
	}
	for _, sortBy := range input.SortBys {
		if _, exists := requested[sortBy.Name]; !exists || !validSortOrder(sortBy.SortOrder) || sortBy.SortOrder == "" {
			return ReportDataQueryRequest{}, invalidArgument("report_data_query", "sort fields must be requested dimensions or metrics with an explicit order")
		}
	}
	filters := append([]DimensionValue(nil), input.DimensionFilters...)
	advertiserFilter := false
	for index, filter := range filters {
		if !validCMField(filter.DimensionName) || !validText(filter.Value, 512, false) ||
			(filter.ID != "" && !validOpaque(filter.ID, 256)) ||
			(filter.MatchType != "" && filter.MatchType != "EXACT" && filter.MatchType != "BEGINS_WITH" &&
				filter.MatchType != "CONTAINS" && filter.MatchType != "WILDCARD_EXPRESSION") {
			return ReportDataQueryRequest{}, invalidArgument("report_data_query", "dimension filter is invalid")
		}
		if filter.DimensionName == "advertiser" {
			if advertiserFilter || filter.ID != advertiserID || filter.Value != "" ||
				(filter.MatchType != "" && filter.MatchType != "EXACT") {
				return ReportDataQueryRequest{}, ownershipError("report_data_query", "report query")
			}
			filters[index].MatchType = "EXACT"
			advertiserFilter = true
		}
	}
	if !advertiserFilter {
		filters = append(filters, DimensionValue{DimensionName: "advertiser", ID: advertiserID, MatchType: "EXACT"})
	}
	input.DimensionNames = append([]string(nil), input.DimensionNames...)
	input.MetricNames = append([]string(nil), input.MetricNames...)
	input.SortBys = append([]SortBy(nil), input.SortBys...)
	input.DimensionFilters = filters
	return input, nil
}

func validByteRange(value ByteRange) bool {
	return value.Start >= 0 && value.End >= value.Start && value.End-value.Start+1 <= maxDownloadChunkBytes
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

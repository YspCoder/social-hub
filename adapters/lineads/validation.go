package lineads

import (
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

func validOpaque(value string, maximum int) bool {
	if value == "" || value != strings.TrimSpace(value) || len(value) > maximum || !utf8.ValidString(value) {
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

func validPathSegment(value string, maximum int) bool {
	return validOpaque(value, maximum) && value != "." && value != ".." && !strings.ContainsAny(value, "/\\?#%")
}

func validPartnerType(value PartnerType) bool {
	switch value {
	case PartnerDataGeneral, PartnerCertifiedAdTechGeneral, PartnerReportingGeneral:
		return true
	default:
		return false
	}
}

func validPagination(page, size int, maximumSize int) bool {
	return page >= 0 && size >= 0 && (maximumSize == 0 || size <= maximumSize)
}

func validSort(value Sort, allowed map[string]struct{}) bool {
	if value == (Sort{}) {
		return true
	}
	if _, ok := allowed[value.Field]; !ok {
		return false
	}
	return value.Direction == "" || value.Direction == SortAscending || value.Direction == SortDescending
}

func validListAdAccounts(input ListAdAccountsRequest) bool {
	return validOptionalOpaque(input.Name, 1024) && validPagination(input.Page, input.Size, 0) &&
		validSort(input.Sort, adAccountSortFields)
}

func validListCampaigns(input ListCampaignsRequest) bool {
	return validPathSegment(input.AdAccountID, 256) && validIDs(input.IDs) &&
		validPagination(input.Page, input.Size, 0) && validSort(input.Sort, campaignSortFields)
}

func validListPerformanceReports(input ListPerformanceReportsRequest) bool {
	return validPathSegment(input.AdAccountID, 256) && validIDs(input.IDs) &&
		validPagination(input.Page, input.Size, 0) && validSort(input.Sort, performanceReportSortFields)
}

func validGetOnlineReport(input GetOnlineReportRequest) bool {
	if !validPathSegment(input.AdAccountID, 256) || !validPagination(input.Page, input.Size, 100) ||
		!validOptionalOpaque(input.SearchKey, 1024) || input.CampaignID < 0 || input.AdGroupID < 0 || input.LandingPageID < 0 {
		return false
	}
	switch input.Level {
	case ReportLevelCampaign, ReportLevelAdGroup, ReportLevelAd:
	default:
		return false
	}
	start, startOK := parseDate(input.Since)
	end, endOK := parseDate(input.Until)
	if !startOK || !endOK {
		return false
	}
	return input.Since == "" || input.Until == "" || !end.Before(start)
}

func validIDs(values []int64) bool {
	seen := make(map[int64]struct{}, len(values))
	for _, value := range values {
		if value <= 0 {
			return false
		}
		if _, found := seen[value]; found {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func parseDate(value Date) (time.Time, bool) {
	if value == "" {
		return time.Time{}, true
	}
	if len(value) != len("2006-01-02") {
		return time.Time{}, false
	}
	parsed, err := time.Parse("2006-01-02", string(value))
	return parsed, err == nil && Date(parsed.Format("2006-01-02")) == value
}

func prepareCallOptions(operation string, options []socialhub.CallOption) (socialhub.CallOptions, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return socialhub.CallOptions{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" {
		return socialhub.CallOptions{}, invalidArgument(operation, "LINE Ads API does not document a caller request-ID header")
	}
	if resolved.IdempotencyKey != "" {
		return socialhub.CallOptions{}, invalidArgument(operation, "bodyless LINE Ads read workflows do not use idempotency keys")
	}
	if len(resolved.Fields) > 0 {
		return socialhub.CallOptions{}, invalidArgument(operation, "these LINE Ads endpoints do not define field selection")
	}
	if resolved.Timeout < 0 {
		return socialhub.CallOptions{}, invalidArgument(operation, "call timeout must not be negative")
	}
	return resolved, nil
}

func forwardCallOptions(options socialhub.CallOptions) []socialhub.CallOption {
	if options.Timeout == 0 {
		return nil
	}
	return []socialhub.CallOption{socialhub.WithCallTimeout(options.Timeout)}
}

var (
	adAccountSortFields = map[string]struct{}{
		"id": {}, "name": {}, "configuredStatus": {}, "currency": {}, "timezone": {}, "createdDate": {},
	}
	campaignSortFields = map[string]struct{}{
		"id": {}, "name": {}, "campaignObjective": {}, "configuredStatus": {}, "spendingLimitMicro": {},
		"startDate": {}, "endDate": {}, "createdDate": {},
	}
	performanceReportSortFields = map[string]struct{}{
		"id": {}, "name": {}, "status": {}, "createdDate": {},
	}
)

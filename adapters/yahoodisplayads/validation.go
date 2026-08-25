package yahoodisplayads

import (
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

const defaultPageSize int32 = 500

func resolveCallOptions(operation string, options []socialhub.CallOption) (socialhub.CallOptions, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return socialhub.CallOptions{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" {
		return socialhub.CallOptions{}, invalidArgument(operation, "LINE Yahoo assigns rid; caller request IDs are not supported")
	}
	if resolved.IdempotencyKey != "" {
		return socialhub.CallOptions{}, invalidArgument(operation, "Display Ads API does not document an idempotency-key contract")
	}
	if len(resolved.Fields) > 0 {
		return socialhub.CallOptions{}, invalidArgument(operation, "field selection is fixed by the typed operation")
	}
	return resolved, nil
}

func preparedCallOptions(resolved socialhub.CallOptions) []socialhub.CallOption {
	if resolved.Timeout == 0 {
		return nil
	}
	return []socialhub.CallOption{socialhub.WithCallTimeout(resolved.Timeout)}
}

func prepareCallOptions(operation string, options []socialhub.CallOption) ([]socialhub.CallOption, error) {
	resolved, err := resolveCallOptions(operation, options)
	if err != nil {
		return nil, err
	}
	return preparedCallOptions(resolved), nil
}

func validCallbackURL(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" || parsed.Opaque != "" {
		return false
	}
	if strings.EqualFold(parsed.Scheme, "https") {
		return true
	}
	if !strings.EqualFold(parsed.Scheme, "http") {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func validHTTPURL(value string) bool {
	if !validOpaque(value, 4096) {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed.Host != "" && parsed.User == nil &&
		(parsed.Scheme == "https" || parsed.Scheme == "http")
}

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

func validText(value string, maximumRunes int) bool {
	return validOpaque(value, maximumRunes*4) && utf8.RuneCountInString(value) <= maximumRunes
}

func validDate(value string) bool {
	if len(value) != len("20060102") {
		return false
	}
	parsed, err := time.Parse("20060102", value)
	return err == nil && parsed.Format("20060102") == value
}

func validOptionalDate(value string) bool { return value == "" || validDate(value) }

func validPage(value PageRequest, maximum int32) bool {
	return value.StartIndex >= 0 && (value.StartIndex == 0 || value.StartIndex >= 1) &&
		value.NumberResults >= 0 && value.NumberResults <= maximum
}

func normalizedPage(value PageRequest) PageRequest {
	if value.StartIndex == 0 {
		value.StartIndex = 1
	}
	if value.NumberResults == 0 {
		value.NumberResults = defaultPageSize
	}
	return value
}

func validIDs(ids []int64, maximum int, allowEmpty bool) bool {
	if (!allowEmpty && len(ids) == 0) || len(ids) > maximum {
		return false
	}
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return false
		}
		if _, exists := seen[id]; exists {
			return false
		}
		seen[id] = struct{}{}
	}
	return true
}

func validUserStatuses(statuses []UserStatus) bool {
	seen := make(map[UserStatus]struct{}, len(statuses))
	for _, status := range statuses {
		if status != StatusActive && status != StatusPaused {
			return false
		}
		if _, exists := seen[status]; exists {
			return false
		}
		seen[status] = struct{}{}
	}
	return true
}

func validCampaignSelector(value CampaignSelector) bool {
	return validIDs(value.CampaignIDs, MaximumCampaignSelectorIDs, true) &&
		validUserStatuses(value.UserStatuses) && validPage(value.PageRequest, MaximumCampaignPageSize)
}

func validCampaignGoal(value string) bool {
	if !validOpaque(value, 64) {
		return false
	}
	for _, character := range value {
		if character != '_' && (character < 'A' || character > 'Z') {
			return false
		}
	}
	return true
}

func validCampaignAdd(value CampaignAdd) bool {
	return validText(value.Name, 50) && validCampaignGoal(value.Goal) &&
		value.BudgetAmount > 0 && value.CPC > 0 &&
		validOptionalDate(value.StartDate) && validOptionalDate(value.EndDate) &&
		(value.StartDate == "" || value.EndDate == "" || value.StartDate <= value.EndDate)
}

func validCampaignUpdate(value CampaignUpdate) bool {
	if value.ID <= 0 || value.Name != nil && !validText(*value.Name, 50) ||
		value.BudgetAmount != nil && *value.BudgetAmount <= 0 || value.CPC != nil && *value.CPC <= 0 ||
		value.StartDate != nil && !validDate(*value.StartDate) || value.EndDate != nil && !validDate(*value.EndDate) {
		return false
	}
	if value.StartDate != nil && value.EndDate != nil && *value.StartDate > *value.EndDate {
		return false
	}
	return value.Name != nil || value.BudgetAmount != nil || value.CPC != nil || value.StartDate != nil || value.EndDate != nil
}

func validAdGroupSelector(value AdGroupSelector) bool {
	return validIDs(value.CampaignIDs, MaximumAdGroupSelectorIDs, true) &&
		validIDs(value.AdGroupIDs, MaximumAdGroupSelectorIDs, true) &&
		validUserStatuses(value.UserStatuses) && validPage(value.PageRequest, MaximumAdGroupPageSize)
}

func validAdGroupAdd(value AdGroupAdd) bool {
	return validText(value.Name, 50) && value.CPC > 0
}

func validAdGroupUpdate(value AdGroupUpdate) bool {
	if value.ID <= 0 || value.Name != nil && !validText(*value.Name, 50) || value.CPC != nil && *value.CPC <= 0 {
		return false
	}
	return value.Name != nil || value.CPC != nil
}

func validAdSelector(value AdSelector) bool {
	return validIDs(value.CampaignIDs, MaximumAdSelectorIDs, true) &&
		validIDs(value.AdGroupIDs, MaximumAdSelectorIDs, true) &&
		validIDs(value.AdIDs, MaximumAdSelectorIDs, true) &&
		validUserStatuses(value.UserStatuses) && validPage(value.PageRequest, MaximumAdPageSize)
}

func validReturnedUserStatus(status UserStatus) bool {
	return status == StatusActive || status == StatusPaused || status == StatusUnknown
}

func validBannerAdAdd(value BannerAdAdd) bool {
	return validText(value.Name, 50) && value.MediaID > 0 && validHTTPURL(value.FinalURL)
}

func validAdUpdate(value AdUpdate) bool {
	if value.ID <= 0 || value.Name != nil && !validText(*value.Name, 50) ||
		value.FinalURL != nil && !validHTTPURL(*value.FinalURL) {
		return false
	}
	return value.Name != nil || value.FinalURL != nil
}

func validReportSelector(value ReportSelector) bool {
	if !validIDs(value.ReportJobIDs, int(MaximumReportPageSize), true) ||
		!validPage(value.PageRequest, MaximumReportPageSize) {
		return false
	}
	seen := make(map[ReportJobStatus]struct{}, len(value.ReportJobStatuses))
	for _, status := range value.ReportJobStatuses {
		if status != ReportWaiting && status != ReportInProgress && status != ReportCompleted &&
			status != ReportCanceled && status != ReportFailed {
			return false
		}
		if _, exists := seen[status]; exists {
			return false
		}
		seen[status] = struct{}{}
	}
	return true
}

func validReportDateRangeType(value ReportDateRangeType) bool {
	switch value {
	case ReportCustomDate, ReportToday, ReportYesterday, ReportLast7Days, ReportLastWeek,
		ReportLastBusinessWeek, ReportLast14Days, ReportLast30Days, ReportThisMonth,
		ReportThisMonthExceptToday, ReportLastMonth:
		return true
	default:
		return false
	}
}

func validReportFormat(value ReportFormat) bool {
	return value == "" || value == ReportCSV || value == ReportTSV || value == ReportXML
}

func validReportDefinitionAdd(value ReportDefinitionAdd) bool {
	if (value.Name != "" && !validText(value.Name, 255)) || !validReportDateRangeType(value.DateRangeType) ||
		!validReportFormat(value.Format) || len(value.Fields) == 0 || len(value.Fields) > 256 {
		return false
	}
	seen := make(map[string]struct{}, len(value.Fields))
	for _, field := range value.Fields {
		if !validOpaque(field, 128) {
			return false
		}
		if _, exists := seen[field]; exists {
			return false
		}
		seen[field] = struct{}{}
	}
	if value.DateRangeType == ReportCustomDate {
		return value.DateRange != nil && validDate(value.DateRange.StartDate) &&
			validDate(value.DateRange.EndDate) && value.DateRange.StartDate <= value.DateRange.EndDate
	}
	return value.DateRange == nil
}

func validReturnedScopes(scopes []string) bool {
	if len(scopes) == 0 || len(scopes) > 32 {
		return false
	}
	seen := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		if !validOpaque(scope, 256) {
			return false
		}
		if _, exists := seen[scope]; exists {
			return false
		}
		seen[scope] = struct{}{}
	}
	return scopeGranted(scopes, oauthScope)
}

func formatID(value int64) string { return strconv.FormatInt(value, 10) }

package kakaomoment

import (
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

func prepareCallOptions(operation string, options []socialhub.CallOption) ([]socialhub.CallOption, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return nil, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" {
		return nil, invalidArgument(operation, "Kakao assigns request IDs; caller request IDs are not supported")
	}
	if resolved.IdempotencyKey != "" {
		return nil, invalidArgument(operation, "Kakao Moment does not document a generic idempotency-key contract")
	}
	if len(resolved.Fields) > 0 {
		return nil, invalidArgument(operation, "field selection is fixed by the typed operation")
	}
	if resolved.Timeout > 0 {
		return []socialhub.CallOption{socialhub.WithCallTimeout(resolved.Timeout)}, nil
	}
	return nil, nil
}

func validateCallOptions(operation string, options []socialhub.CallOption) error {
	_, err := prepareCallOptions(operation, options)
	return err
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

func validOptionalText(value string, maximumRunes int) bool {
	return value == "" || validText(value, maximumRunes)
}

func validText(value string, maximumRunes int) bool {
	return validOpaque(value, maximumRunes*4) && utf8.RuneCountInString(value) <= maximumRunes
}

func validCallbackURL(value string) bool {
	if !validOpaque(value, 4096) {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed.Host != "" && parsed.User == nil && parsed.Fragment == "" &&
		(parsed.Scheme == "https" || parsed.Scheme == "http")
}

func validScopes(scopes []string) bool {
	seen := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		if (scope != ScopeManagement && scope != ScopeDelete) || !validOpaque(scope, 256) || strings.ContainsAny(scope, ", ") {
			return false
		}
		if _, exists := seen[scope]; exists {
			return false
		}
		seen[scope] = struct{}{}
	}
	_, deletes := seen[ScopeDelete]
	_, manages := seen[ScopeManagement]
	return !deletes || manages
}

func validConfig(value ConfigStatus, allowDeleted bool) bool {
	return value == ConfigOn || value == ConfigOff || allowDeleted && value == ConfigDeleted
}

func validConfigFilter(values []ConfigStatus) bool {
	seen := make(map[ConfigStatus]struct{}, len(values))
	for _, value := range values {
		if !validConfig(value, true) {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validEnumToken(value string) bool {
	if !validOpaque(value, 128) {
		return false
	}
	for _, character := range value {
		if character != '_' && (character < 'A' || character > 'Z') {
			return false
		}
	}
	return true
}

func validCampaignCreate(value CampaignCreate) bool {
	if !validOptionalText(value.Name, 100) || !validEnumToken(value.CampaignTypeGoal.CampaignType) ||
		!validEnumToken(value.CampaignTypeGoal.Goal) || !validOptionalText(value.TrackID, 512) {
		return false
	}
	if value.Objective != nil && (!validEnumToken(value.Objective.Type) || !validText(value.Objective.Value, 1024)) {
		return false
	}
	return value.DailyBudgetAmount == nil || *value.DailyBudgetAmount > 0
}

func validCampaignUpdate(value CampaignUpdate) bool {
	if value.ID <= 0 || value.Name != nil && !validText(*value.Name, 100) ||
		value.TrackID != nil && !validOptionalText(*value.TrackID, 512) {
		return false
	}
	return value.Name != nil || value.TrackID != nil || value.KCLID != nil
}

func validDate(value string) bool {
	if len(value) != len("20060102") {
		return false
	}
	parsed, err := time.Parse("20060102", value)
	return err == nil && parsed.Format("20060102") == value
}

func validDatePreset(value DatePreset) bool {
	switch value {
	case DateToday, DateYesterday, DateLast7Days, DateLast14Days, DateLast30Days, DateThisMonth, DateLastMonth:
		return true
	default:
		return false
	}
}

func validReportRequest(value ReportRequest) bool {
	if value.DatePreset != "" {
		if !validDatePreset(value.DatePreset) || value.Start != "" || value.End != "" {
			return false
		}
	} else if value.Start != "" || value.End != "" {
		if !validDate(value.Start) || !validDate(value.End) || value.Start > value.End {
			return false
		}
		start, _ := time.Parse("20060102", value.Start)
		end, _ := time.Parse("20060102", value.End)
		if end.Sub(start) > 30*24*time.Hour {
			return false
		}
	}
	if value.TimeUnit != "" && value.TimeUnit != "DAY" || value.Level != "" && !validEnumToken(value.Level) ||
		value.Dimension != "" && !validEnumToken(value.Dimension) || len(value.MetricsGroups) == 0 || len(value.MetricsGroups) > 16 {
		return false
	}
	seen := make(map[string]struct{}, len(value.MetricsGroups))
	for _, group := range value.MetricsGroups {
		if !validEnumToken(group) {
			return false
		}
		if _, exists := seen[group]; exists {
			return false
		}
		seen[group] = struct{}{}
	}
	return true
}

func validIDs(ids []int64, maximum int) bool {
	if len(ids) == 0 || len(ids) > maximum {
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

func formatID(value int64) string { return strconv.FormatInt(value, 10) }

func joinIDs(values []int64) string {
	items := make([]string, len(values))
	for index, value := range values {
		items[index] = formatID(value)
	}
	return strings.Join(items, ",")
}

func joinConfigs(values []ConfigStatus) string {
	items := make([]string, len(values))
	for index, value := range values {
		items[index] = string(value)
	}
	return strings.Join(items, ",")
}

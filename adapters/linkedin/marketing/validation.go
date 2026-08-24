package marketing

import (
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	accountURNPrefix       = "urn:li:sponsoredAccount:"
	campaignGroupURNPrefix = "urn:li:sponsoredCampaignGroup:"
	campaignURNPrefix      = "urn:li:sponsoredCampaign:"
	creativeURNPrefix      = "urn:li:sponsoredCreative:"
	facetURNPrefix         = "urn:li:adTargetingFacet:"
)

func validNumericID(value string) bool {
	if value == "" || len(value) > 20 {
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

func validText(value string, maximum int) bool {
	return strings.TrimSpace(value) != "" && strings.TrimSpace(value) == value && len(value) <= maximum &&
		utf8.ValidString(value) && !strings.ContainsRune(value, '\x00')
}

func validOptionalText(value string, maximum int) bool {
	return value == "" || validText(value, maximum)
}

func validCallbackURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && parsed.Fragment == ""
}

func validNumericURN(value, prefix string) bool {
	return strings.HasPrefix(value, prefix) && validNumericID(strings.TrimPrefix(value, prefix))
}

func validURN(value string) bool {
	return strings.HasPrefix(value, "urn:li:") && validOpaque(value, 2048) && strings.Count(value, ":") >= 3
}

func validContentURN(value string) bool {
	return validNumericURN(value, "urn:li:share:") || validNumericURN(value, "urn:li:ugcPost:")
}

func validAssociatedEntityURN(value string) bool {
	if validNumericURN(value, "urn:li:organization:") {
		return true
	}
	const personPrefix = "urn:li:person:"
	return strings.HasPrefix(value, personPrefix) && validOpaque(strings.TrimPrefix(value, personPrefix), 1024)
}

func validPage(cursor string, maxResults, maximum int) bool {
	return (cursor == "" || validOpaque(cursor, 16384)) && maxResults >= 0 && maxResults <= maximum
}

func validStatus(value Status) bool {
	switch value {
	case StatusActive, StatusPaused, StatusDraft, StatusArchived, StatusCompleted,
		StatusCanceled, StatusPendingDeletion, StatusRemoved:
		return true
	default:
		return false
	}
}

func validGroupStatus(value Status) bool {
	return validStatus(value) && value != StatusCompleted
}

func validStatuses(values []Status, validate func(Status) bool) bool {
	for _, value := range values {
		if !validate(value) {
			return false
		}
	}
	return true
}

func validMutationStatus(value Status) bool { return value == StatusActive || value == StatusPaused }

func validMoney(value Money) bool {
	return positiveDecimal(value.Amount) && validCurrency(value.CurrencyCode)
}

func positiveDecimal(value string) bool {
	if value == "" || len(value) > 64 || strings.TrimSpace(value) != value {
		return false
	}
	dot, nonZero, digits := false, false, 0
	for index := range value {
		switch character := value[index]; {
		case character == '.' && !dot:
			dot = true
		case character >= '0' && character <= '9':
			digits++
			nonZero = nonZero || character != '0'
		default:
			return false
		}
	}
	return digits > 0 && nonZero && value[0] != '.' && value[len(value)-1] != '.'
}

func validCurrency(value string) bool {
	if len(value) != 3 {
		return false
	}
	for index := range value {
		if value[index] < 'A' || value[index] > 'Z' {
			return false
		}
	}
	return true
}

func validLocale(value Locale) bool {
	if len(value.Language) != 2 || len(value.Country) != 2 {
		return false
	}
	return value.Language[0] >= 'a' && value.Language[0] <= 'z' && value.Language[1] >= 'a' && value.Language[1] <= 'z' &&
		value.Country[0] >= 'A' && value.Country[0] <= 'Z' && value.Country[1] >= 'A' && value.Country[1] <= 'Z'
}

func validSchedule(value RunSchedule) bool {
	return value.Start > 0 && value.End >= 0 && (value.End == 0 || value.End > value.Start)
}

func validTargeting(value TargetingCriteria) bool {
	if len(value.Include.And) == 0 || len(value.Include.And) > 64 {
		return false
	}
	hasLocale, hasLocations, hasProfileLocations := false, false, false
	for _, clause := range value.Include.And {
		if !validTargetingClause(clause) {
			return false
		}
		for facet := range clause.Or {
			switch facet {
			case facetURNPrefix + "interfaceLocales":
				hasLocale = true
			case facetURNPrefix + "locations":
				hasLocations = true
			case facetURNPrefix + "profileLocations":
				hasProfileLocations = true
			}
		}
	}
	if value.Exclude != nil && !validTargetingClause(*value.Exclude) {
		return false
	}
	return hasLocale && hasLocations != hasProfileLocations
}

func validTargetingClause(value TargetingClause) bool {
	if len(value.Or) == 0 || len(value.Or) > 32 {
		return false
	}
	for facet, values := range value.Or {
		if !strings.HasPrefix(facet, facetURNPrefix) || !validOpaque(facet, 256) || len(values) == 0 || len(values) > 500 {
			return false
		}
		for _, urn := range values {
			if !validURN(urn) {
				return false
			}
		}
	}
	return true
}

func validCostType(value CostType) bool {
	return value == CostCPC || value == CostCPM || value == CostCPV
}

func validObjective(value ObjectiveType) bool {
	switch value {
	case ObjectiveWebsiteVisit, ObjectiveEngagement, ObjectiveLeadGeneration, ObjectiveWebsiteConversion,
		ObjectiveBrandAwareness, ObjectiveVideoView, ObjectiveJobApplicant:
		return true
	default:
		return false
	}
}

func validDate(value string) bool {
	if len(value) != len("2006-01-02") {
		return false
	}
	_, err := time.Parse("2006-01-02", value)
	return err == nil
}

func dateSpan(start, end string) (int, bool) {
	if !validDate(start) || !validDate(end) {
		return 0, false
	}
	from, _ := time.Parse("2006-01-02", start)
	to, _ := time.Parse("2006-01-02", end)
	if to.Before(from) {
		return 0, false
	}
	return int(to.Sub(from)/(24*time.Hour)) + 1, true
}

func validGranularity(value TimeGranularity) bool {
	return value == GranularityAll || value == GranularityDaily || value == GranularityMonthly
}

func validPivot(value AnalyticsPivot) bool {
	return value == PivotAccount || value == PivotCampaignGroup || value == PivotCampaign || value == PivotCreative
}

func validFields(values []string) bool {
	if len(values) == 0 || len(values) > 20 {
		return false
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validIdentifier(value) {
			return false
		}
		if _, found := seen[value]; found {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validIdentifier(value string) bool {
	if value == "" || len(value) > 128 || !asciiLetter(value[0]) {
		return false
	}
	for index := 1; index < len(value); index++ {
		character := value[index]
		if character != '_' && (character < 'A' || character > 'Z') && (character < 'a' || character > 'z') && (character < '0' || character > '9') {
			return false
		}
	}
	return true
}

func asciiLetter(value byte) bool {
	return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}

func validOAuthScopes(values []string) bool {
	if len(values) == 0 || len(values) > 3 {
		return false
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value != readAdsScope && value != writeAdsScope && value != reportingAdsScope {
			return false
		}
		if _, found := seen[value]; found {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

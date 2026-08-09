package marketing

import (
	"encoding/json"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

func validID(value int64) bool { return value > 0 }

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

func validatePage(page, pageSize, maximum int) (int, int, error) {
	if page < 0 || pageSize < 0 || pageSize > maximum {
		return 0, 0, invalidArgument("pagination", "page must be non-negative and page_size exceeds the endpoint limit")
	}
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 20
	}
	return page, pageSize, nil
}

func validPutStatus(value PutStatus, allowDelete bool) bool {
	return value == PutStatusDelivering || value == PutStatusPaused || allowDelete && value == PutStatusDeleted
}

func validDate(value string) bool {
	parsed, err := time.Parse("2006-01-02", value)
	return err == nil && parsed.Format("2006-01-02") == value
}

func validateDatePair(start, end string) bool {
	return start == "" && end == "" || validDate(start) && validDate(end) && start <= end
}

func validateIDs(values []int64, maximum int) bool {
	if len(values) > maximum {
		return false
	}
	seen := make(map[int64]struct{}, len(values))
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

func validateStatuses(values []PutStatus) bool {
	if len(values) > 3 {
		return false
	}
	for _, value := range values {
		if !validPutStatus(value, true) {
			return false
		}
	}
	return true
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

func validSceneIDs(values []string) bool {
	if len(values) == 0 || len(values) > 16 {
		return false
	}
	for _, value := range values {
		if !validOpaque(value, 16) {
			return false
		}
		for _, character := range value {
			if !unicode.IsDigit(character) {
				return false
			}
		}
	}
	return true
}

func validReportLevel(level ReportLevel) bool {
	return level == ReportLevelAccount || level == ReportLevelCampaign || level == ReportLevelUnit || level == ReportLevelCreative
}

func validGranularity(value TemporalGranularity) bool {
	return value == "" || value == GranularityDaily || value == GranularityHourly
}

func validReportDimensions(values []string) bool {
	if len(values) > 2 {
		return false
	}
	for _, value := range values {
		if value != "adScene" && value != "placementType" {
			return false
		}
	}
	return true
}

func validOAuthType(value string) bool {
	return value == "" || value == "advertiser" || value == "agent" || value == "ad_social" || value == "series"
}

func validLifetime(seconds int64) bool {
	return seconds > 0 && seconds <= int64((10*365*24*time.Hour)/time.Second)
}

package oceanengine

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

func validEnum(value string) bool {
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

func validOperation(value Operation) bool {
	return value == OperationDisable || value == OperationEnable
}

func validatePage(page, pageSize, maximum int) (int, int, error) {
	if page < 0 || pageSize < 0 || pageSize > maximum {
		return 0, 0, invalidArgument("pagination", "page must be non-negative and page_size must be within the endpoint limit")
	}
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 20
	}
	return page, pageSize, nil
}

func validateFields(values []string) bool {
	if len(values) > 128 {
		return false
	}
	for _, value := range values {
		if !validFieldName(value) {
			return false
		}
	}
	return true
}

func validateIDs(values []int64) bool {
	if len(values) > 100 {
		return false
	}
	for _, value := range values {
		if !validID(value) {
			return false
		}
	}
	return true
}

func mergeFields(operation string, fixed map[string]any, fields map[string]any) (map[string]any, error) {
	protected := map[string]struct{}{
		"access_token": {}, "secret": {}, "advertiser_id": {},
		"project_id": {}, "promotion_id": {}, "operation": {}, "name": {},
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
		_, protectedKey := protected[key]
		if fixedKey || protectedKey {
			return nil, invalidArgument(operation, "extension fields cannot override adapter-controlled values")
		}
		if _, err := json.Marshal(value); err != nil {
			return nil, platformError(operation, "invalid_argument", "permanent", err)
		}
		result[key] = value
	}
	return result, nil
}

func appendRequiredFields(fields, required []string) []string {
	result := append([]string(nil), fields...)
	seen := make(map[string]struct{}, len(result))
	for _, field := range result {
		seen[field] = struct{}{}
	}
	for _, field := range required {
		if _, found := seen[field]; !found {
			result = append(result, field)
		}
	}
	return result
}

func parseDateTime(value string) (time.Time, bool) {
	for _, layout := range []string{"2006-01-02", "2006-01-02 15:04:05", time.RFC3339} {
		if parsed, err := time.Parse(layout, value); err == nil && parsed.Format(layout) == value {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func validTimeRange(start, end string) bool {
	startTime, startOK := parseDateTime(start)
	endTime, endOK := parseDateTime(end)
	return startOK && endOK && !startTime.After(endTime)
}

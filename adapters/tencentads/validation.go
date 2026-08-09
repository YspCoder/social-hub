package tencentads

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

func validatePage(page, pageSize int) (int, int, error) {
	if page < 0 || pageSize < 0 || pageSize > 500 {
		return 0, 0, invalidArgument("pagination", "page must be non-negative and page_size must not exceed 500")
	}
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 20
	}
	return page, pageSize, nil
}

func validateList(fields []string, filters []Filtering, page, pageSize int) (int, int, error) {
	if len(fields) > 128 || len(filters) > 100 {
		return 0, 0, invalidArgument("list", "too many fields or filters")
	}
	for _, field := range fields {
		if !validFieldName(field) {
			return 0, 0, invalidArgument("list", "fields must be lowercase API identifiers")
		}
	}
	for _, filter := range filters {
		if !validFieldName(filter.Field) || !validEnum(filter.Operator) || len(filter.Values) == 0 || len(filter.Values) > 100 {
			return 0, 0, invalidArgument("list", "filters require a field, uppercase operator, and bounded values")
		}
		for _, value := range filter.Values {
			if !validOpaque(value, 4096) {
				return 0, 0, invalidArgument("list", "filter values must be non-empty text")
			}
		}
	}
	return validatePage(page, pageSize)
}

func appendRequiredFields(fields []string, required ...string) []string {
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

func mergeFields(operation string, fixed, fields map[string]any, protected ...string) (map[string]any, error) {
	reserved := map[string]struct{}{
		"access_token": {}, "client_secret": {}, "timestamp": {}, "nonce": {}, "account_id": {},
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

func validDate(value string) bool {
	parsed, err := time.Parse("2006-01-02", value)
	return err == nil && parsed.Format("2006-01-02") == value
}

func validStatus(value ConfiguredStatus) bool {
	return value == ConfiguredStatusNormal || value == ConfiguredStatusSuspend
}

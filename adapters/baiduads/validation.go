package baiduads

import (
	"encoding/json"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

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

func validUserName(value string) bool { return validOpaque(value, 256) }

func validText(value string, minimum, maximum int) bool {
	length := baiduTextLength(value)
	return strings.TrimSpace(value) == value && length >= minimum && length <= maximum && utf8.ValidString(value) &&
		!strings.ContainsRune(value, '\x00')
}

func baiduTextLength(value string) int {
	length := 0
	for _, character := range value {
		if character <= unicode.MaxASCII {
			length++
		} else {
			length += 2
		}
	}
	return length
}

func validFieldName(value string) bool {
	if value == "" || len(value) > 128 || !unicode.IsLower(rune(value[0])) {
		return false
	}
	for _, character := range value {
		if !unicode.IsLetter(character) && !unicode.IsDigit(character) {
			return false
		}
	}
	return true
}

func validateIDs(operation string, values []int64, maximum int, allowEmpty bool) error {
	if len(values) > maximum || !allowEmpty && len(values) == 0 {
		return invalidArgument(operation, "resource IDs have an invalid count")
	}
	for _, value := range values {
		if value <= 0 {
			return invalidArgument(operation, "resource IDs must be positive")
		}
	}
	return nil
}

func validateFields(operation string, fields []string, maximum int) error {
	if len(fields) > maximum {
		return invalidArgument(operation, "too many response fields")
	}
	for _, field := range fields {
		if !validFieldName(field) {
			return invalidArgument(operation, "response fields must be lower-camel API identifiers")
		}
	}
	return nil
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
		"header": {}, "body": {}, "userName": {}, "accessToken": {}, "password": {}, "token": {}, "target": {},
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
			return nil, invalidArgument(operation, "extension field names must be lower-camel API identifiers")
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

func validDestinationURL(value string, maximum int) bool {
	if !validText(value, 1, maximum) {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil
}

func validTabs(values []int) bool {
	if len(values) > 30 {
		return false
	}
	for _, value := range values {
		if value < 0 || value > 30 {
			return false
		}
	}
	return true
}

func boolAsInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func validateCallback(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" ||
		parsed.User != nil || parsed.Fragment != "" || parsed.RawQuery != "" {
		return invalidArgument("oauth_authorize", "callback must be an absolute HTTP(S) URL without credentials, query, or fragment")
	}
	return nil
}

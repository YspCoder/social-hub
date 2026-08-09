package googleads

import (
	"encoding/json"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"
)

func validCustomerID(value string) bool {
	if len(value) != 10 {
		return false
	}
	for index := range value {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	return true
}

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

func validDeveloperToken(value string) bool {
	return len(value) == 22 && validOpaque(value, 22)
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

func validRequiredText(value string, maximum int) bool {
	return strings.TrimSpace(value) != "" && strings.TrimSpace(value) == value && len(value) <= maximum &&
		utf8.ValidString(value) && !strings.ContainsRune(value, '\x00')
}

func validStatus(value Status) bool {
	return value == StatusEnabled || value == StatusPaused
}

func validEUDeclaration(value EUPoliticalAdvertising) bool {
	return value == ContainsEUPoliticalAdvertising || value == DoesNotContainEUPoliticalAdvertising
}

func validPageToken(value string) bool {
	return value == "" || validOpaque(value, 16384)
}

func validHTTPURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil
}

func validResourceName(customerID, collection, value string) bool {
	prefix := "customers/" + customerID + "/" + collection + "/"
	if !strings.HasPrefix(value, prefix) {
		return false
	}
	id := strings.TrimPrefix(value, prefix)
	if collection == "adGroupAds" {
		parts := strings.Split(id, "~")
		return len(parts) == 2 && validNumericID(parts[0]) && validNumericID(parts[1])
	}
	return validNumericID(id)
}

func validCustomerResourceName(value string) bool {
	return strings.HasPrefix(value, "customers/") && validCustomerID(strings.TrimPrefix(value, "customers/"))
}

func requireResourceName(operation, customerID, collection, value string) error {
	if !validResourceName(customerID, collection, value) {
		return platformContractError(operation, "Google Ads returned an invalid or cross-customer resource name")
	}
	return nil
}

func validateFinalURLs(values []string) bool {
	if len(values) == 0 || len(values) > 20 {
		return false
	}
	for _, value := range values {
		if len(value) > 2048 || !validHTTPURL(value) {
			return false
		}
	}
	return true
}

func validateTextAssets(values []AdTextAsset, minimum, maximum, maximumRunes int) bool {
	if len(values) < minimum || len(values) > maximum {
		return false
	}
	for _, value := range values {
		if !validRequiredText(value.Text, maximumRunes*4) || utf8.RuneCountInString(value.Text) > maximumRunes ||
			value.AssetPerformanceLabel != "" || value.PinnedField != "" && !validPinnedField(value.PinnedField) {
			return false
		}
	}
	return true
}

func validPinnedField(value string) bool {
	switch value {
	case "HEADLINE_1", "HEADLINE_2", "HEADLINE_3", "DESCRIPTION_1", "DESCRIPTION_2":
		return true
	default:
		return false
	}
}

func validOptionalPath(value string) bool {
	return value == "" || validRequiredText(value, 15*4) && utf8.RuneCountInString(value) <= 15 &&
		!strings.ContainsAny(value, "/?#")
}

func validGAQL(value string) bool {
	if !validRequiredText(value, 65536) || strings.Contains(value, ";") || strings.Contains(value, "--") ||
		strings.Contains(value, "/*") || strings.Contains(value, "*/") {
		return false
	}
	upper := strings.ToUpper(strings.Join(strings.Fields(value), " "))
	return strings.HasPrefix(upper, "SELECT ") && strings.Contains(upper, " FROM ")
}

func validJSONField(value string) bool {
	if value == "" || len(value) > 128 || value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for index := 1; index < len(value); index++ {
		character := value[index]
		if character < 'a' || character > 'z' {
			if character < 'A' || character > 'Z' {
				if character < '0' || character > '9' {
					return false
				}
			}
		}
	}
	return true
}

func mergeFields(operation string, fixed, fields map[string]any, protected ...string) (map[string]any, error) {
	reserved := map[string]struct{}{
		"accessToken": {}, "refreshToken": {}, "developerToken": {}, "loginCustomerId": {},
		"customerId": {}, "resourceName": {}, "status": {},
	}
	for _, key := range protected {
		reserved[key] = struct{}{}
	}
	result := make(map[string]any, len(fixed)+len(fields))
	for key, value := range fixed {
		result[key] = value
		reserved[key] = struct{}{}
	}
	for key, value := range fields {
		if !validJSONField(key) {
			return nil, invalidArgument(operation, "extension field names must be lowerCamelCase JSON identifiers")
		}
		if _, found := reserved[key]; found {
			return nil, invalidArgument(operation, "extension fields cannot override adapter-controlled values")
		}
		if _, err := json.Marshal(value); err != nil {
			return nil, invalidArgument(operation, "extension fields must be JSON encodable")
		}
		result[key] = value
	}
	return result, nil
}

func updateMask(fields []string) string { return strings.Join(fields, ",") }

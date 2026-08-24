package aliexpressaffiliate

import (
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

const maxProviderLong = uint64(1<<63 - 1)

func validateCallOptions(operation string, options []socialhub.CallOption) error {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" {
		return invalidArgument(operation, "AliExpress assigns request IDs; caller request IDs are not supported")
	}
	if resolved.IdempotencyKey != "" {
		return invalidArgument(operation, "the read and link-generation methods do not define idempotency keys")
	}
	if len(resolved.Fields) > 0 {
		return invalidArgument(operation, "use the typed request Fields value for Affiliate field selection")
	}
	return nil
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
	return value == "" || validOpaque(value, maximumRunes*4) && utf8.RuneCountInString(value) <= maximumRunes
}

func validNumericID(value string, maximum int) bool {
	if value == "" || len(value) > maximum || value[0] == '0' {
		return false
	}
	for index := range value {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	return true
}

func validCSVValue(value string, maximum int) bool {
	return validOpaque(value, maximum) && !strings.ContainsRune(value, ',')
}

func validFields(fields []string) bool {
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		if field == "" || len(field) > 128 {
			return false
		}
		for index := range field {
			character := field[index]
			if character != '_' && (character < 'a' || character > 'z') && (character < '0' || character > '9') {
				return false
			}
		}
		if _, duplicate := seen[field]; duplicate {
			return false
		}
		seen[field] = struct{}{}
	}
	return true
}

func validCountry(value string) bool {
	if value == "" {
		return true
	}
	return len(value) == 2 && value[0] >= 'A' && value[0] <= 'Z' && value[1] >= 'A' && value[1] <= 'Z'
}

func validCurrency(value string, details bool) bool {
	if value == "" {
		return true
	}
	allowed := map[string]bool{
		"USD": true, "GBP": true, "CAD": true, "EUR": true, "UAH": true, "MXN": true,
		"TRY": true, "RUB": true, "BRL": true, "AUD": true, "INR": true, "JPY": true,
		"IDR": true, "SEK": true, "KRW": true,
	}
	if !details {
		allowed["ILS"], allowed["THB"], allowed["CLP"], allowed["VND"] = true, true, true, true
	}
	return allowed[value]
}

func validLanguage(value string) bool {
	if value == "" {
		return true
	}
	switch value {
	case "EN", "RU", "PT", "ES", "FR", "ID", "IT", "TH", "JA", "AR", "VI", "TR", "DE", "HE", "KO", "NL", "PL", "MX", "CL", "IN":
		return true
	default:
		return false
	}
}

func validStringIDs(values []string) bool {
	if len(values) == 0 {
		return false
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validNumericID(value, 32) {
			return false
		}
		if _, duplicate := seen[value]; duplicate {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func setString(values url.Values, key, value string) {
	if value != "" {
		values.Set(key, value)
	}
}

func setUint(values url.Values, key string, value uint64) {
	if value != 0 {
		values.Set(key, strconv.FormatUint(value, 10))
	}
}

func validProviderLong(value uint64) bool {
	return value <= maxProviderLong
}

func validOrderRange(start, end time.Time) bool {
	return !start.IsZero() && !end.IsZero() && !end.Before(start)
}

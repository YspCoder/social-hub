package conversions

import (
	"net"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	conversionURNPrefix = "urn:lla:llaPartnerConversion:"
	leadURNPrefix       = "urn:li:leadGenFormResponse:"
)

var (
	decimalPattern = regexp.MustCompile(`^(?:0|[1-9][0-9]*)(?:\.[0-9]+)?$`)
	lowerSHA256    = regexp.MustCompile(`^[a-f0-9]{64}$`)
	anySHA256      = regexp.MustCompile(`(?i)^[a-f0-9]{64}$`)
	legacyMD5      = regexp.MustCompile(`(?i)^[a-f0-9]{32}$`)
	emailPattern   = regexp.MustCompile("^[a-z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$")
	uuidPattern    = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

func validNumericID(value string) bool {
	if value == "" || len(value) > 20 {
		return false
	}
	nonzero := false
	for index := range value {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
		nonzero = nonzero || value[index] != '0'
	}
	return nonzero
}

func validOpaque(value string, maximum int) bool {
	if value == "" || len(value) > maximum || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	return !strings.ContainsFunc(value, unicode.IsControl)
}

func validOptionalOpaque(value string, maximum int) bool {
	return value == "" || validOpaque(value, maximum)
}

func validText(value string, maximum int) bool {
	return validOpaque(value, maximum*4) && utf8.RuneCountInString(value) <= maximum
}

func validOptionalText(value string, maximum int) bool {
	return value == "" || validText(value, maximum)
}

func validDecimal(value Decimal) bool {
	return len(value) > 0 && len(value) <= 128 && decimalPattern.MatchString(string(value))
}

func validCurrency(value string) bool {
	if len(value) != 3 {
		return false
	}
	for _, character := range value {
		if character < 'A' || character > 'Z' {
			return false
		}
	}
	return true
}

func validCountryCode(value string) bool {
	if value == "" {
		return true
	}
	return len(value) == 2 && value[0] >= 'A' && value[0] <= 'Z' && value[1] >= 'A' && value[1] <= 'Z'
}

func normalizeIPv4(value string) (string, bool) {
	address := net.ParseIP(strings.TrimSpace(value))
	if address == nil || address.To4() == nil || address.IsUnspecified() || address.IsMulticast() {
		return "", false
	}
	return address.To4().String(), true
}

func validUUID(value string) bool {
	return uuidPattern.MatchString(value) && !strings.EqualFold(value, "00000000-0000-0000-0000-000000000000")
}

func validApprovalScopes(scopes []string) bool {
	if len(scopes) > 2 {
		return false
	}
	seen := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		if scope != writeScope && scope != readAdsScope {
			return false
		}
		if _, found := seen[scope]; found {
			return false
		}
		seen[scope] = struct{}{}
	}
	return true
}

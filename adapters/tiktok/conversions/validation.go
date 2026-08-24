package conversions

import (
	"math/big"
	"net"
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	decimalPattern = regexp.MustCompile(`^(?:0|[1-9][0-9]*)(?:\.[0-9]+)?$`)
	lowerSHA256    = regexp.MustCompile(`^[a-f0-9]{64}$`)
	anySHA256      = regexp.MustCompile(`(?i)^[a-f0-9]{64}$`)
	legacyMD5      = regexp.MustCompile(`(?i)^[a-f0-9]{32}$`)
	emailPattern   = regexp.MustCompile("^[a-z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$")
	uuidPattern    = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	localePattern  = regexp.MustCompile(`^[A-Za-z]{2,8}(?:-[A-Za-z0-9]{1,8})*$`)
)

var supportedCurrencies = map[string]struct{}{
	"AED": {}, "ARS": {}, "AUD": {}, "BDT": {}, "BHD": {}, "BIF": {}, "BOB": {}, "BRL": {},
	"CAD": {}, "CHF": {}, "CLP": {}, "CNY": {}, "COP": {}, "CRC": {}, "CZK": {}, "DKK": {},
	"DZD": {}, "EGP": {}, "EUR": {}, "GBP": {}, "GTQ": {}, "HKD": {}, "HNL": {}, "HUF": {},
	"IDR": {}, "ILS": {}, "INR": {}, "ISK": {}, "JPY": {}, "KES": {}, "KHR": {}, "KRW": {},
	"KWD": {}, "KZT": {}, "MAD": {}, "MOP": {}, "MXN": {}, "MYR": {}, "NGN": {}, "NIO": {},
	"NOK": {}, "NZD": {}, "OMR": {}, "PEN": {}, "PHP": {}, "PKR": {}, "PLN": {}, "PYG": {},
	"QAR": {}, "RON": {}, "RUB": {}, "SAR": {}, "SEK": {}, "SGD": {}, "THB": {}, "TRY": {},
	"TWD": {}, "UAH": {}, "USD": {}, "VES": {}, "VND": {}, "ZAR": {},
}

func validEventSource(value EventSource) bool {
	switch value {
	case EventSourceWeb, EventSourceApp, EventSourceOffline, EventSourceCRM:
		return true
	default:
		return false
	}
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

func validHTTPURL(value string) bool {
	if len(value) > 8192 || !utf8.ValidString(value) || strings.ContainsFunc(value, unicode.IsControl) {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.Hostname() != "" && parsed.User == nil && parsed.Fragment == ""
}

func validPublicIP(value string) bool {
	address := net.ParseIP(value)
	return address != nil && !address.IsUnspecified() && !address.IsLoopback() && !address.IsPrivate() &&
		!address.IsMulticast() && !address.IsLinkLocalUnicast() && !address.IsLinkLocalMulticast()
}

func validDecimal(value Decimal) bool {
	return len(value) > 0 && len(value) <= 128 && decimalPattern.MatchString(string(value))
}

func decimalAtMostOne(value Decimal) bool {
	if !validDecimal(value) {
		return false
	}
	number, ok := new(big.Rat).SetString(string(value))
	return ok && number.Cmp(big.NewRat(1, 1)) <= 0
}

func validCurrency(value string) bool {
	_, found := supportedCurrencies[value]
	return found
}

func validContentType(value ContentType) bool {
	return value == "" || value == ContentTypeProduct || value == ContentTypeProductGroup
}

func validCustomerType(value CustomerType) bool {
	return value == "" || value == CustomerNew || value == CustomerReturning
}

func validATTStatus(value ATTStatus) bool {
	switch value {
	case "", ATTAuthorized, ATTDenied, ATTNotDetermined, ATTRestricted, ATTNotApplicable:
		return true
	default:
		return false
	}
}

func validLocale(value string) bool {
	return value == "" || len(value) <= 35 && localePattern.MatchString(value)
}

func validUUID(value string) bool {
	return uuidPattern.MatchString(value) && !strings.EqualFold(value, "00000000-0000-0000-0000-000000000000")
}

func validApprovalScopes(scopes []string) bool {
	return len(scopes) == 0 || len(scopes) == 1 && scopes[0] == conversionPermission
}

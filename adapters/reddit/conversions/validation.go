package conversions

import (
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	nonnegativeDecimalPattern = regexp.MustCompile(`^(?:0|[1-9][0-9]*)(?:\.[0-9]+)?$`)
	emailPattern              = regexp.MustCompile("^[a-z0-9!#$%&'*+/=?^_`{|}~-]+@[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$")
	uuidPattern               = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	pixelCookiePattern        = regexp.MustCompile(`^(?:[0-9]{1,20}\.)?[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

func validPublicURL(value string) bool {
	if len(value) > 8192 || !utf8.ValidString(value) || strings.ContainsFunc(value, unicode.IsControl) {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.Hostname() != "" && parsed.User == nil && parsed.Fragment == ""
}

func validUserAgent(value string) bool {
	if len(value) < 12 || len(value) > 256 || value != strings.TrimSpace(value) ||
		!strings.Contains(value, ":") || !strings.Contains(value, "(by /u/") || !strings.HasSuffix(value, ")") {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character > 0x7e {
			return false
		}
	}
	return true
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

func validOptionalOpaque(value string, maximum int) bool {
	return value == "" || validOpaque(value, maximum)
}

func validText(value string, maximum int) bool {
	return validOpaque(value, 4096) && utf8.RuneCountInString(value) <= maximum
}

func validPixelID(value string) bool {
	if len(value) < 4 || len(value) > 128 ||
		(!strings.HasPrefix(value, "t2_") && !strings.HasPrefix(value, "a2_") && !strings.HasPrefix(value, "p2_")) {
		return false
	}
	for index := range value {
		character := value[index]
		if index == 2 && character == '_' {
			continue
		}
		if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') && (character < '0' || character > '9') {
			return false
		}
	}
	return true
}

func validApproval(accountType string, scopes []string) bool {
	return accountType == "" && (len(scopes) == 0 || len(scopes) == 1 && scopes[0] == conversionScope)
}

func validTrackingType(value TrackingType) bool {
	switch value {
	case TrackingPageVisit, TrackingViewContent, TrackingSearch, TrackingAddToCart,
		TrackingAddToWishlist, TrackingPurchase, TrackingLead, TrackingSignUp, TrackingCustom:
		return true
	default:
		return false
	}
}

func validActionSource(value ActionSource) bool {
	return value == ActionSourceWebsite || value == ActionSourceApp || value == ActionSourceOther || value == ActionSourcePhysicalStore
}

func validNonnegativeDecimal(value Decimal) bool {
	return len(value) > 0 && len(value) <= 128 && nonnegativeDecimalPattern.MatchString(string(value))
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

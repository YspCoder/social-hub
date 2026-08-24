package conversions

import (
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

var (
	decimalPattern   = regexp.MustCompile(`^(?:0|[1-9][0-9]*)(?:\.[0-9]+)?$`)
	eventNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,100}$`)
	partnerPattern   = regexp.MustCompile(`^ss-[a-z0-9][a-z0-9_-]*$`)
	lowerSHA256      = regexp.MustCompile(`^[a-f0-9]{64}$`)
	anySHA256        = regexp.MustCompile(`(?i)^[a-f0-9]{64}$`)
	legacyMD5        = regexp.MustCompile(`(?i)^[a-f0-9]{32}$`)
	emailPattern     = regexp.MustCompile("^[a-z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$")
	maidPattern      = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

func validID(value string) bool {
	if value == "" || len(value) > 18 {
		return false
	}
	nonzero := false
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
		nonzero = nonzero || character != '0'
	}
	return nonzero
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
	return validOpaque(value, maximum*4) && utf8.RuneCountInString(value) <= maximum
}

func validOptionalText(value string, maximum int) bool {
	return value == "" || validText(value, maximum)
}

func validPublicURL(value string) bool {
	if len(value) > 8192 || !utf8.ValidString(value) || strings.ContainsFunc(value, unicode.IsControl) {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.Hostname() != "" && parsed.User == nil && parsed.Fragment == ""
}

func validActionSource(value ActionSource) bool {
	switch value {
	case ActionSourceAndroid, ActionSourceIOS, ActionSourceWeb, ActionSourceOffline:
		return true
	default:
		return false
	}
}

func validEventName(value EventName) bool {
	return eventNamePattern.MatchString(string(value))
}

func validPartnerName(value string) bool {
	return value == "" || len(value) <= 100 && partnerPattern.MatchString(value)
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

func validLanguage(value string) bool {
	return len(value) == 2 && value[0] >= 'a' && value[0] <= 'z' && value[1] >= 'a' && value[1] <= 'z'
}

func validFormFactor(value FormFactor) bool {
	switch value {
	case "", FormFactorDesktop, FormFactorLaptop, FormFactorCellphone, FormFactorTablet,
		FormFactorSmartwatch, FormFactorTV, FormFactorVR, FormFactorConsole, FormFactorOther:
		return true
	default:
		return false
	}
}

func validNetworkType(value NetworkType) bool {
	switch value {
	case "", NetworkWiFi, NetworkCellular2G, NetworkCellular3G, NetworkCellular4G,
		NetworkCellular5G, NetworkCellular6G, NetworkEthernet, NetworkUnknown:
		return true
	default:
		return false
	}
}

func validOSFamily(value OSFamily) bool {
	switch value {
	case "", OSFamilyIOS, OSFamilyAndroid, OSFamilyMacOS, OSFamilyWindows, OSFamilyLinux, OSFamilyBSD, OSFamilyOther:
		return true
	default:
		return false
	}
}

func integerInRange(value *int, minimum, maximum int) bool {
	return value == nil || *value >= minimum && *value <= maximum
}

func int64InRange(value *int64, minimum, maximum int64) bool {
	return value == nil || *value >= minimum && *value <= maximum
}

func validateCallOptions(operation string, options []socialhub.CallOption) (socialhub.CallOptions, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return socialhub.CallOptions{}, withOperation(err, operation)
	}
	if resolved.RequestID != "" || resolved.IdempotencyKey != "" || len(resolved.Fields) != 0 {
		return socialhub.CallOptions{}, invalidArgument(operation, "only per-call timeouts are supported by Pinterest conversion submission")
	}
	if resolved.Timeout < 0 {
		return socialhub.CallOptions{}, invalidArgument(operation, "timeout must not be negative")
	}
	return resolved, nil
}

func resolvedCallOptions(resolved socialhub.CallOptions) []socialhub.CallOption {
	if resolved.Timeout == 0 {
		return nil
	}
	return []socialhub.CallOption{socialhub.WithCallTimeout(resolved.Timeout)}
}

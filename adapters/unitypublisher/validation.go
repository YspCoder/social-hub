package unitypublisher

import (
	"math"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"
)

var uuidPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func validOpaque(value string, maximum int) bool {
	return value != "" && strings.TrimSpace(value) == value && len(value) <= maximum && !strings.ContainsAny(value, "\x00\r\n")
}

func validBasicKeyID(value string) bool {
	return validOpaque(value, 1024) && !strings.ContainsRune(value, ':')
}

func validOrganizationID(value string) bool {
	if value == "" || len(value) > 20 || value[0] == '0' {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func validPathID(value string) bool {
	return validOpaque(value, 256) && !strings.ContainsAny(value, "/?#% ,")
}

func validUUID(value string) bool { return uuidPattern.MatchString(value) }

func validText(value string, maximum int) bool {
	return strings.TrimSpace(value) != "" && value == strings.TrimSpace(value) && utf8.ValidString(value) &&
		utf8.RuneCountInString(value) <= maximum && !strings.ContainsRune(value, '\x00')
}

func validOptionalText(value *string, maximum int) bool {
	return value == nil || validText(*value, maximum)
}

func validOptionalURL(value *string) bool {
	if value == nil {
		return true
	}
	parsed, err := url.Parse(*value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && len(*value) <= 4096
}

func finiteNonNegative(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0
}

func validPlatform(value Platform) bool {
	switch value {
	case PlatformAndroid, PlatformIOS, PlatformOSX, PlatformWindows, PlatformLinux, PlatformWebGL,
		PlatformWindowsStore, PlatformPS4, PlatformPS5, PlatformXboxOne, PlatformTVOS, PlatformSwitch, PlatformVisionOS:
		return true
	default:
		return false
	}
}

func validStore(value Store) bool {
	switch value {
	case StoreGooglePlay, StoreAppleAppStore, StoreSamsungGalaxy, StoreAmazonAppStore, StoreMacAppStore,
		StoreUDP, StoreMicrosoftStore, StoreHuaweiAppGallery, StoreAPK:
		return true
	default:
		return false
	}
}

func validWritableAdFormat(value AdFormat) bool {
	return value == AdFormatRewarded || value == AdFormatInterstitial || value == AdFormatBanner
}

func validResponseAdFormat(value AdFormat) bool {
	return validWritableAdFormat(value) || value == AdFormatNative
}

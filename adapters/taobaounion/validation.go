package taobaounion

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

func validateCallOptions(operation string, options []socialhub.CallOption) error {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" {
		return invalidArgument(operation, "TOP assigns request IDs; caller request IDs are not supported")
	}
	if resolved.IdempotencyKey != "" {
		return invalidArgument(operation, "TOP v2 does not define idempotency keys for this workflow")
	}
	if len(resolved.Fields) > 0 {
		return invalidArgument(operation, "field selection is fixed by the typed TOP method")
	}
	return nil
}

func validGatewayURL(value string) bool {
	return value == defaultBaseURL || value == sandboxBaseURL
}

func splitGatewayURL(value string) (string, string, error) {
	parsed, err := url.Parse(value)
	if err != nil || !validGatewayURL(value) {
		return "", "", fmt.Errorf("invalid TOP gateway URL")
	}
	origin := parsed.Scheme + "://" + parsed.Host
	return origin, parsed.Path, nil
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
	_, err := strconv.ParseUint(value, 10, 63)
	return err == nil
}

func validOptionalID(value string, maximum int) bool {
	return value == "" || validOpaque(value, maximum) && !strings.ContainsRune(value, ',')
}

func validPlatform(value LinkPlatform) bool {
	return value == 0 || value == LinkPlatformPC || value == LinkPlatformWireless
}

func validBizScene(value string, itemInfo bool) bool {
	if value == "" {
		return true
	}
	if itemInfo {
		return value == "1" || value == "2" || value == "3"
	}
	return value == "1" || value == "2" || value == "4"
}

func validPromotionType(value string) bool {
	return value == "" || value == "1" || value == "2"
}

func validIP(value string) bool { return value == "" || net.ParseIP(value) != nil }

func validHTTPURL(value string) bool {
	if !validOpaque(value, 8192) {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" && parsed.User == nil
}

func validOrderRange(start, end time.Time) bool {
	return !start.IsZero() && !end.IsZero() && !end.Before(start) && end.Sub(start) <= 3*time.Hour
}

func setString(values url.Values, key, value string) {
	if value != "" {
		values.Set(key, value)
	}
}

func setInt(values url.Values, key string, value int64) {
	if value != 0 {
		values.Set(key, strconv.FormatInt(value, 10))
	}
}

func setBool(values url.Values, key string, value *bool) {
	if value != nil {
		values.Set(key, strconv.FormatBool(*value))
	}
}

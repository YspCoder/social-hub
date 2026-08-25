package shopeeads

import (
	"fmt"
	"math"
	"mime"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

func resolveCallOptions(operation string, options []socialhub.CallOption) (socialhub.CallOptions, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return socialhub.CallOptions{}, invalidArgument(operation, "call options are invalid")
	}
	if resolved.RequestID != "" {
		return socialhub.CallOptions{}, invalidArgument(operation, "Shopee assigns request IDs; caller request IDs are not supported")
	}
	if resolved.IdempotencyKey != "" {
		return socialhub.CallOptions{}, invalidArgument(operation, "read-only Shopee Ads operations do not accept idempotency keys")
	}
	if len(resolved.Fields) > 0 {
		return socialhub.CallOptions{}, invalidArgument(operation, "field selection is fixed by the typed operation")
	}
	if resolved.Timeout < 0 {
		return socialhub.CallOptions{}, invalidArgument(operation, "timeout must not be negative")
	}
	return resolved, nil
}

func resolvedCallOption(resolved socialhub.CallOptions) socialhub.CallOption {
	return func(target *socialhub.CallOptions) error {
		*target = resolved
		target.Fields = append([]string(nil), resolved.Fields...)
		return nil
	}
}

func validShopeeOrigin(value string) bool {
	switch value {
	case "https://partner.shopeemobile.com",
		"https://openplatform.shopee.cn",
		"https://openplatform.shopee.com.br",
		"https://openplatform.sandbox.test-stable.shopee.sg",
		"https://openplatform.sandbox.test-stable.shopee.cn":
		return true
	default:
		return false
	}
}

func validCallbackURL(value string) bool {
	if !validOpaque(value, 4096) {
		return false
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return false
	}
	if parsed.Scheme == "https" {
		return true
	}
	if parsed.Scheme != "http" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "localhost" {
		return true
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}

func validJSONMediaType(value string) bool {
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && mediaType == "application/json"
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

func parsePartnerID(value string) (int64, error) {
	if value == "" || value != strings.TrimSpace(value) {
		return 0, fmt.Errorf("invalid partner ID")
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || !validPartnerID(parsed) {
		return 0, fmt.Errorf("invalid partner ID")
	}
	return parsed, nil
}

func validPartnerID(value int64) bool { return value > 0 && value <= math.MaxUint32 }
func validShopID(value int64) bool    { return value > 0 && value <= math.MaxUint32 }
func validItemID(value int64) bool    { return value > 0 && value <= math.MaxInt32 }

func validDate(value string) bool {
	if len(value) != len("02-01-2006") {
		return false
	}
	parsed, err := time.Parse("02-01-2006", value)
	return err == nil && parsed.Format("02-01-2006") == value
}

func validDateRange(start, end string) bool {
	if !validDate(start) || !validDate(end) {
		return false
	}
	startTime, _ := time.Parse("02-01-2006", start)
	endTime, _ := time.Parse("02-01-2006", end)
	return !endTime.Before(startTime)
}

func dateWithin(value, start, end string) bool {
	if !validDate(value) || !validDateRange(start, end) {
		return false
	}
	date, _ := time.Parse("02-01-2006", value)
	startTime, _ := time.Parse("02-01-2006", start)
	endTime, _ := time.Parse("02-01-2006", end)
	return !date.Before(startTime) && !date.After(endTime)
}

func validIDs(values []int64, maximum int) bool {
	if len(values) == 0 || len(values) > maximum {
		return false
	}
	seen := make(map[int64]struct{}, len(values))
	for _, value := range values {
		if value <= 0 {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func containsID(values []int64, target int64) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func validInfoTypes(values []CampaignInfoType) bool {
	if len(values) == 0 || len(values) > 4 {
		return false
	}
	seen := make(map[CampaignInfoType]struct{}, len(values))
	for _, value := range values {
		if value < CampaignInfoCommon || value > CampaignInfoAutoProduct {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validAdType(value string) bool {
	return value == "" || value == "all" || value == "auto" || value == "manual"
}

func validCampaignAdType(value string) bool { return value == "auto" || value == "manual" }

func validCampaignPlacement(value string) bool {
	return value == "search" || value == "discovery" || value == "all"
}

func validTextList(values []string, maximumItems, maximumRunes int) bool {
	if values == nil || len(values) > maximumItems {
		return false
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validOpaque(value, maximumRunes*4) || utf8.RuneCountInString(value) > maximumRunes {
			return false
		}
		if _, duplicate := seen[value]; duplicate {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validPositiveIDList(values []int64, shopIDs bool) bool {
	if len(values) > 10_000 {
		return false
	}
	seen := make(map[int64]struct{}, len(values))
	for _, value := range values {
		valid := value > 0
		if shopIDs {
			valid = validShopID(value)
		}
		if !valid {
			return false
		}
		if _, duplicate := seen[value]; duplicate {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validTokenSubjectIDs(fields tokenFields) bool {
	return (fields.PartnerID == 0 || validPartnerID(fields.PartnerID)) &&
		(fields.ShopID == 0 || validShopID(fields.ShopID)) && fields.PrincipalID >= 0 &&
		fields.MerchantID >= 0 && fields.SupplierID >= 0 && fields.UserID >= 0 &&
		validPositiveIDList(fields.ShopIDs, true) && validPositiveIDList(fields.MerchantIDs, false) &&
		validPositiveIDList(fields.SupplierIDs, false) && validPositiveIDList(fields.UserIDs, false) &&
		validPositiveIDList(fields.PrincipalIDs, false)
}

func formatID(value int64) string { return strconv.FormatInt(value, 10) }

func joinIDs(values []int64) string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = formatID(value)
	}
	return strings.Join(result, ",")
}

func joinInfoTypes(values []CampaignInfoType) string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = strconv.Itoa(int(value))
	}
	return strings.Join(result, ",")
}

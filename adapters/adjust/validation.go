package adjust

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var (
	decimalPattern  = regexp.MustCompile(`^(?:0|[1-9][0-9]*)(?:\.[0-9]+)?$`)
	currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)
	minimumRevenue  = big.NewRat(1, 1000)
)

func validOpaque(value string, maximum int) bool {
	return value != "" && utf8.ValidString(value) && value == strings.TrimSpace(value) && len(value) <= maximum &&
		!strings.ContainsFunc(value, unicode.IsControl)
}

func validOptionalOpaque(value string, maximum int) bool {
	return value == "" || validOpaque(value, maximum)
}

func validateEvent(request EventRequest, now time.Time) error {
	if !validOpaque(request.EventToken, 4096) {
		return fmt.Errorf("event_token is required")
	}
	if err := validateDevice(request.Device); err != nil {
		return err
	}
	if err := validateCommon(request.CreatedAt, 58*24*time.Hour, request.Environment, request.IPAddress, request.UserAgent, request.CallbackParams, request.PartnerParams, now); err != nil {
		return err
	}
	if request.Revenue == "" && request.Currency != "" || request.Revenue != "" && request.Currency == "" {
		return fmt.Errorf("revenue and currency must be provided together")
	}
	if request.Revenue != "" {
		if !validEventRevenue(request.Revenue) {
			return fmt.Errorf("revenue must be an exact decimal of at least 0.001")
		}
		if !currencyPattern.MatchString(request.Currency) {
			return fmt.Errorf("currency must be an uppercase ISO 4217 code")
		}
	}
	return nil
}

func validateSession(request SessionRequest, now time.Time) error {
	if err := validateDevice(request.Device); err != nil {
		return err
	}
	if !validOSName(request.OSName) {
		return fmt.Errorf("os_name is invalid")
	}
	if err := validateCommon(request.CreatedAt, 0, request.Environment, request.IPAddress, request.UserAgent, request.CallbackParams, request.PartnerParams, now); err != nil {
		return err
	}
	if request.SentAt != nil && request.SentAt.After(now) {
		return fmt.Errorf("sent_at must not be in the future")
	}
	if request.ForwardedFor != "" {
		ip := net.ParseIP(request.ForwardedFor)
		if ip == nil || ip.To4() == nil {
			return fmt.Errorf("forwarded_for must be an IPv4 address")
		}
	}
	for _, value := range []*int64{request.SessionCount, request.SubsessionCount, request.SessionLength, request.TimeSpent} {
		if value != nil && *value < 0 {
			return fmt.Errorf("session counters and durations must not be negative")
		}
	}
	if request.ATTStatus != nil && (*request.ATTStatus < 0 || *request.ATTStatus > 4) {
		return fmt.Errorf("att_status must be between 0 and 4")
	}
	for _, value := range []string{
		request.AppVersion, request.AppVersionShort, request.BundleID, request.PackageName, request.Country,
		request.Language, request.OSVersion, request.CPUType, request.DeviceType, request.DeviceName,
		request.HardwareName, request.InstallReceipt, request.PrimaryDedupeToken, request.GoogleAppSetID,
	} {
		if !validOptionalOpaque(value, 8192) {
			return fmt.Errorf("session contains an invalid metadata value")
		}
	}
	if request.AmazonDMA != nil && (request.AmazonDMA.AdUserData == nil || request.AmazonDMA.AdStorage == nil) {
		return fmt.Errorf("amazon_dma requires both ad_user_data and ad_storage")
	}
	return nil
}

func validateAdRevenue(request AdRevenueRequest, now time.Time) error {
	if err := validateDevice(request.Device); err != nil {
		return err
	}
	if !validAdRevenue(request.Revenue) {
		return fmt.Errorf("revenue must be an exact non-negative decimal")
	}
	if !currencyPattern.MatchString(request.Currency) {
		return fmt.Errorf("currency must be an uppercase ISO 4217 code")
	}
	if request.AdImpressionsCount <= 0 {
		return fmt.Errorf("ad_impressions_count must be positive")
	}
	if err := validateCommon(request.CreatedAt, 28*24*time.Hour, request.Environment, "", "", request.CallbackParams, nil, now); err != nil {
		return err
	}
	for _, value := range []string{request.Network, request.Unit, request.Placement} {
		if !validOptionalOpaque(value, 8192) {
			return fmt.Errorf("ad revenue contains invalid metadata")
		}
	}
	return nil
}

func validateCommon(createdAt *time.Time, maximumAge time.Duration, environment Environment, ipAddress, userAgent string, callback, partner map[string]string, now time.Time) error {
	if createdAt != nil {
		if createdAt.After(now) {
			return fmt.Errorf("created_at must not be in the future")
		}
		if maximumAge > 0 && createdAt.Before(now.Add(-maximumAge)) {
			return fmt.Errorf("created_at is outside the supported lookback window")
		}
	}
	if environment != "" && environment != EnvironmentSandbox && environment != EnvironmentProduction {
		return fmt.Errorf("environment must be sandbox or production")
	}
	if ipAddress != "" {
		ip := net.ParseIP(ipAddress)
		if ip == nil || ip.To4() == nil {
			return fmt.Errorf("ip_address must be an IPv4 address")
		}
	}
	if !validOptionalOpaque(userAgent, 8192) {
		return fmt.Errorf("user_agent is invalid")
	}
	if err := validateParameters("callback_params", callback); err != nil {
		return err
	}
	return validateParameters("partner_params", partner)
}

func validateParameters(name string, values map[string]string) error {
	for key, value := range values {
		if !validOpaque(key, 8192) {
			return fmt.Errorf("%s contains an invalid key", name)
		}
		if !utf8.ValidString(value) {
			return fmt.Errorf("%s contains an invalid UTF-8 value", name)
		}
	}
	if _, err := json.Marshal(values); err != nil {
		return fmt.Errorf("%s is invalid", name)
	}
	return nil
}

func validateDevice(device DeviceIdentifiers) error {
	values := deviceValues(device)
	found := false
	for _, value := range values {
		if value == "" {
			continue
		}
		found = true
		if !validOpaque(value, 4096) {
			return fmt.Errorf("device contains an invalid identifier")
		}
	}
	if !found {
		return fmt.Errorf("at least one device identifier is required")
	}
	return nil
}

func deviceValues(device DeviceIdentifiers) []string {
	return []string{
		device.VIDA, device.RIDA, device.TIFA, device.IDFA, device.GPSADID, device.FireADID, device.OAID, device.WebUUID, device.ADID,
		device.IDFV, device.AndroidID, device.ExternalDeviceID, device.AndroidIDLowerMD5,
		device.AndroidIDLowerSHA1, device.AndroidIDUpperMD5, device.AndroidIDUpperSHA1,
		device.IMEI, device.IMEILowerMD5, device.MEID, device.WindowsNAID, device.WindowsHardwareID, device.PersistentIOSUUID,
	}
}

func parseRevenue(value Decimal) (*big.Rat, bool) {
	raw := string(value)
	if len(raw) == 0 || len(raw) > 128 || !decimalPattern.MatchString(raw) {
		return nil, false
	}
	number, ok := new(big.Rat).SetString(raw)
	return number, ok
}

func validEventRevenue(value Decimal) bool {
	number, ok := parseRevenue(value)
	return ok && number.Cmp(minimumRevenue) >= 0
}

func validAdRevenue(value Decimal) bool {
	_, ok := parseRevenue(value)
	return ok
}

func validOSName(value OSName) bool {
	switch value {
	case OSIOS, OSAndroid, OSAndroidTV, OSFireTV, OSRoku, OSTizen, OSWindows, OSXbox, OSPlayStation, OSServer:
		return true
	default:
		return false
	}
}

package kochava

import (
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var (
	decimalPattern  = regexp.MustCompile(`^-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?$`)
	currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)
	countryPattern  = regexp.MustCompile(`^[A-Z]{2}$`)
	uuidPattern     = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
)

func validateInstall(input InstallRequest) error {
	if err := validateContext(input.Context); err != nil {
		return err
	}
	if !validOptionalOpaque(input.KochavaDeviceID, 4096) {
		return fmt.Errorf("kochava_device_id is invalid")
	}
	if input.AppleSearchAds != nil {
		if err := validateAppleSearchAds(*input.AppleSearchAds); err != nil {
			return err
		}
	}
	if input.InstallReferrer != nil {
		if !validText(input.InstallReferrer.Referrer, 32_768) || input.InstallReferrer.Referrer == "" {
			return fmt.Errorf("install_referrer.referrer is required")
		}
		if input.InstallReferrer.ClickTime != nil && input.InstallReferrer.ClickTime.IsZero() {
			return fmt.Errorf("install_referrer.click_time is invalid")
		}
	}
	return nil
}

func validateEvent(input EventRequest) error {
	if err := validateContext(input.Context); err != nil {
		return err
	}
	if !validOptionalOpaque(input.KochavaDeviceID, 4096) {
		return fmt.Errorf("kochava_device_id is invalid")
	}
	if !validOpaque(string(input.Name), 1024) {
		return fmt.Errorf("event name is required")
	}
	if input.Currency != "" && !currencyPattern.MatchString(input.Currency) {
		return fmt.Errorf("currency must be an uppercase ISO 4217 code")
	}
	return validateProperties(input.Data)
}

func validateUpdateIDFA(input UpdateIDFARequest) error {
	if !validOpaque(input.KochavaDeviceID, 4096) {
		return fmt.Errorf("kochava_device_id is required")
	}
	if !uuidPattern.MatchString(input.IDFA) || strings.EqualFold(input.IDFA, "00000000-0000-0000-0000-000000000000") {
		return fmt.Errorf("idfa must be a non-zero UUID")
	}
	return nil
}

func validateContext(input DeviceContext) error {
	if err := validateDeviceIdentifiers(input.DeviceIDs); err != nil {
		return err
	}
	if net.ParseIP(input.OriginationIP) == nil {
		return fmt.Errorf("origination_ip must be a single IPv4 or IPv6 address")
	}
	if !validOptionalText(input.DeviceUserAgent, 32_768) {
		return fmt.Errorf("device_user_agent is invalid")
	}
	if !validOptionalOpaque(input.DeviceVersion, 4096) {
		return fmt.Errorf("device_version is invalid")
	}
	if input.DeviceUserAgent == "" {
		if input.DeviceVersion == "" || strings.Count(input.DeviceVersion, "-") < 2 {
			return fmt.Errorf("device_version in model-OS-version form is required when device_user_agent is unavailable")
		}
	}
	if !validOptionalOpaque(input.AppVersion, 4096) {
		return fmt.Errorf("app_version is invalid")
	}
	if input.OccurredAt != nil && input.OccurredAt.IsZero() {
		return fmt.Errorf("occurred_at is invalid")
	}
	if input.ATT != nil {
		if err := validateATT(*input.ATT); err != nil {
			return err
		}
	}
	if input.GDPRPrivacyConsent != nil {
		if err := validateGDPR(*input.GDPRPrivacyConsent); err != nil {
			return err
		}
	}
	return nil
}

func validateDeviceIdentifiers(input DeviceIdentifiers) error {
	count := 0
	for _, field := range []struct{ name, value string }{
		{"idfa", input.IDFA}, {"idfv", input.IDFV}, {"adid", input.ADID},
		{"android_id", input.AndroidID}, {"openudid", input.OpenUDID}, {"udid", input.UDID},
	} {
		if field.value != "" {
			count++
			if !validOpaque(field.value, 4096) {
				return fmt.Errorf("device_ids.%s is invalid", field.name)
			}
		}
	}
	reserved := map[string]struct{}{"idfa": {}, "idfv": {}, "adid": {}, "android_id": {}, "openudid": {}, "udid": {}}
	for key, value := range input.Custom {
		if !validOpaque(key, 256) || !validOpaque(value, 4096) {
			return fmt.Errorf("device_ids.custom contains an invalid key or value")
		}
		if _, exists := reserved[strings.ToLower(key)]; exists {
			return fmt.Errorf("device_ids.custom duplicates a typed identifier")
		}
		count++
	}
	if count == 0 {
		return fmt.Errorf("device_ids must contain at least one identifier")
	}
	return nil
}

func validateATT(input AppTrackingTransparency) error {
	if input.Authorized == nil {
		return fmt.Errorf("app_tracking_transparency.authorized is required")
	}
	if input.AuthorizationTime != nil && input.AuthorizationTime.IsZero() {
		return fmt.Errorf("app_tracking_transparency.authorization_time is invalid")
	}
	if input.ResponseDuration != nil && *input.ResponseDuration < 0 {
		return fmt.Errorf("app_tracking_transparency.response_duration must not be negative")
	}
	if input.Detail != "" && input.Detail != ATTAuthorized && input.Detail != ATTDenied &&
		input.Detail != ATTNotDetermined && input.Detail != ATTRestricted {
		return fmt.Errorf("app_tracking_transparency.detail is invalid")
	}
	return nil
}

func validateGDPR(input GDPRPrivacyConsent) error {
	if !validOptionalText(input.TCString, 32_768) {
		return fmt.Errorf("gdpr_privacy_consent.tc_string is invalid")
	}
	if input.GDPRApplies == nil && input.TCString == "" && input.AdUserData == nil && input.AdPersonalization == nil {
		return fmt.Errorf("gdpr_privacy_consent must contain at least one signal")
	}
	if input.TCString == "" && (input.AdUserData == nil || input.AdPersonalization == nil) {
		return fmt.Errorf("gdpr_privacy_consent requires tc_string or both ad_user_data and ad_personalization")
	}
	return nil
}

func validateAppleSearchAds(input AppleSearchAds) error {
	if !validOptionalOpaque(input.AdServicesToken, 128<<10) {
		return fmt.Errorf("apple_search_ads.ad_services_token is invalid")
	}
	if input.AdServicesToken == "" && input.IAd == nil && input.AdServicesAttribution == nil {
		return fmt.Errorf("apple_search_ads must contain a token or attribution result")
	}
	if input.IAd != nil {
		if err := validateIAd(*input.IAd); err != nil {
			return err
		}
	}
	if input.AdServicesAttribution != nil {
		if err := validateAdServices(*input.AdServicesAttribution); err != nil {
			return err
		}
	}
	return nil
}

func validateIAd(input IAdAttribution) error {
	values := []string{
		input.PurchaseDate, input.Keyword, input.AdGroupID, input.CreativeSetID, input.CreativeSetName,
		input.CampaignID, input.LineItemID, input.OrganizationID, input.ConversionDate, input.KeywordID,
		input.ConversionType, input.CountryOrRegion, input.OrganizationName, input.CampaignName,
		input.ClickDate, input.Attributed, input.AdGroupName, input.KeywordMatchType, input.LineItemName,
	}
	nonEmpty := false
	for _, value := range values {
		if value != "" {
			nonEmpty = true
			if !validText(value, 4096) {
				return fmt.Errorf("apple_search_ads.iad contains an invalid value")
			}
		}
	}
	if !nonEmpty {
		return fmt.Errorf("apple_search_ads.iad must not be empty")
	}
	return nil
}

func validateAdServices(input AdServicesAttribution) error {
	nonEmpty := input.ConversionType != "" || input.ClickDate != "" || input.CountryOrRegion != "" || input.Attributed != nil
	for _, value := range []*int64{
		input.KeywordID, input.CreativeSetID, input.OrganizationID, input.CampaignID, input.AdGroupID,
	} {
		if value != nil {
			nonEmpty = true
			if *value <= 0 {
				return fmt.Errorf("apple_search_ads.adservices numeric IDs must be positive")
			}
		}
	}
	if !nonEmpty {
		return fmt.Errorf("apple_search_ads.adservices must not be empty")
	}
	if !validOptionalOpaque(input.ConversionType, 4096) {
		return fmt.Errorf("apple_search_ads.adservices.conversion_type is invalid")
	}
	if input.ClickDate != "" && !validAppleTimestamp(input.ClickDate) {
		return fmt.Errorf("apple_search_ads.adservices.click_date must use an ISO 8601 UTC or offset timestamp")
	}
	if input.CountryOrRegion != "" && !countryPattern.MatchString(input.CountryOrRegion) {
		return fmt.Errorf("apple_search_ads.adservices.country_or_region must be an uppercase country code")
	}
	return nil
}

func validateProperties(input Properties) error {
	seen := make(map[string]struct{}, len(input.Strings)+len(input.Numbers)+len(input.Booleans)+len(input.StringLists))
	checkKey := func(key string) error {
		if !validOpaque(key, 1024) {
			return fmt.Errorf("event_data contains an invalid key")
		}
		if _, exists := seen[key]; exists {
			return fmt.Errorf("event_data contains a duplicate key across value types")
		}
		seen[key] = struct{}{}
		return nil
	}
	for key, value := range input.Strings {
		if err := checkKey(key); err != nil {
			return err
		}
		if !validText(value, 64<<10) {
			return fmt.Errorf("event_data contains an invalid string")
		}
	}
	for key, value := range input.Numbers {
		if err := checkKey(key); err != nil {
			return err
		}
		if !validDecimal(value) {
			return fmt.Errorf("event_data contains an invalid number")
		}
	}
	for key := range input.Booleans {
		if err := checkKey(key); err != nil {
			return err
		}
	}
	for key, values := range input.StringLists {
		if err := checkKey(key); err != nil {
			return err
		}
		for _, value := range values {
			if !validText(value, 64<<10) {
				return fmt.Errorf("event_data contains an invalid string-list value")
			}
		}
	}
	if len(seen) > 16 {
		return fmt.Errorf("event_data must contain at most 16 parameters")
	}
	return nil
}

func validAppleTimestamp(value string) bool {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04Z07:00"} {
		if _, err := time.Parse(layout, value); err == nil {
			return true
		}
	}
	return false
}

func validDecimal(value Decimal) bool {
	raw := string(value)
	return len(raw) > 0 && len(raw) <= 128 && decimalPattern.MatchString(raw)
}

func validOpaque(value string, maximum int) bool {
	return value != "" && value == strings.TrimSpace(value) && validText(value, maximum)
}

func validOptionalOpaque(value string, maximum int) bool {
	return value == "" || validOpaque(value, maximum)
}

func validText(value string, maximum int) bool {
	return len(value) <= maximum && utf8.ValidString(value) && !strings.ContainsFunc(value, unicode.IsControl)
}

func validOptionalText(value string, maximum int) bool {
	return value == "" || validText(value, maximum)
}

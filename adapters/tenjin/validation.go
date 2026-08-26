package tenjin

import (
	"fmt"
	"net"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	decimalPattern      = regexp.MustCompile(`^(?:0|[1-9][0-9]*)(?:\.[0-9]+)?$`)
	installationPattern = regexp.MustCompile(`(?i)^(?:[0-9a-f]{32}|[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})$`)
	bundlePattern       = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,254}$`)
	countryPattern      = regexp.MustCompile(`^[A-Z]{2}$`)
	currencyPattern     = regexp.MustCompile(`^[A-Z]{3}$`)
	languagePattern     = regexp.MustCompile(`^[a-z]{2}$`)
	digitsPattern       = regexp.MustCompile(`^[0-9]+$`)
)

func validateOpen(client *Client, input OpenRequest) error {
	if err := validateEventContext(client, input.Context, false, false); err != nil {
		return err
	}
	if !validOptionalText(input.Referrer, 64<<10) {
		return fmt.Errorf("referrer is invalid")
	}
	if !validOptionalOpaque(input.ODMInfo, 256<<10) {
		return fmt.Errorf("odm_info is invalid")
	}
	return nil
}

func validateCustomEvent(client *Client, input CustomEventRequest) error {
	if err := validateEventContext(client, input.Context, true, true); err != nil {
		return err
	}
	if !validOpaque(string(input.Name), 1024) {
		return fmt.Errorf("event name is required")
	}
	return nil
}

func validatePurchase(client *Client, input PurchaseRequest) error {
	if err := validateEventContext(client, input.Context, true, true); err != nil {
		return err
	}
	if !validOpaque(input.ProductID, 4096) {
		return fmt.Errorf("product_id is required")
	}
	if !validDecimal(input.Price) {
		return fmt.Errorf("price must be a non-negative plain decimal")
	}
	if input.Quantity <= 0 {
		return fmt.Errorf("quantity must be positive")
	}
	if !currencyPattern.MatchString(input.Currency) {
		return fmt.Errorf("currency must be an uppercase ISO 4217 code")
	}
	return nil
}

func validateEventContext(client *Client, input EventContext, includeTrackingStatus, googleRequiresAppVersion bool) error {
	if !validInstallationID(input.Identity.AnalyticsInstallationID) {
		return fmt.Errorf("analytics_installation_id must be a non-zero UUID")
	}
	if !validOptionalOpaque(input.Identity.AdvertisingID, 4096) {
		return fmt.Errorf("advertising_id is invalid")
	}
	if client.platform == PlatformIOS && !validOpaque(input.Identity.DeveloperDeviceID, 4096) {
		return fmt.Errorf("developer_device_id is required for iOS")
	}
	if client.platform != PlatformIOS && !validOptionalOpaque(input.Identity.DeveloperDeviceID, 4096) {
		return fmt.Errorf("developer_device_id is invalid")
	}
	if !validOpaque(input.OSVersion, 1024) {
		return fmt.Errorf("os_version is required")
	}
	if !validOptionalOpaque(input.AppVersion, 1024) {
		return fmt.Errorf("app_version is invalid")
	}
	if input.Country != "" && !countryPattern.MatchString(input.Country) {
		return fmt.Errorf("country must be an uppercase ISO country code")
	}
	if input.IPAddress != "" && net.ParseIP(input.IPAddress) == nil {
		return fmt.Errorf("ip_address must be a single IPv4 or IPv6 address")
	}
	if unavailableAdvertisingID(input.Identity.AdvertisingID) && input.IPAddress == "" {
		return fmt.Errorf("ip_address is required for probabilistic attribution when advertising_id is unavailable")
	}
	for name, value := range map[string]string{
		"os_version_release": input.OSVersionRelease,
		"build_id":           input.BuildID,
		"locale":             input.Locale,
		"device_model":       input.DeviceModel,
		"customer_user_id":   input.CustomerUserID,
	} {
		if !validOptionalOpaque(value, 4096) {
			return fmt.Errorf("%s is invalid", name)
		}
	}
	if input.TrackingStatus != nil && (*input.TrackingStatus < ATTNotDetermined || *input.TrackingStatus > ATTAuthorized) {
		return fmt.Errorf("tracking_status must be between 0 and 3")
	}
	if !includeTrackingStatus && input.TrackingStatus != nil {
		return fmt.Errorf("tracking_status is not part of the app-open contract")
	}
	if client.googleAds {
		if input.OSVersionRelease == "" || input.BuildID == "" || input.Locale == "" || input.DeviceModel == "" {
			return fmt.Errorf("os_version_release, build_id, locale, and device_model are required when Google Ads tracking is enabled")
		}
		if googleRequiresAppVersion && input.AppVersion == "" {
			return fmt.Errorf("app_version is required for Google Ads custom events and purchases")
		}
	}
	if client.metaAEM && includeTrackingStatus && input.TrackingStatus == nil {
		return fmt.Errorf("tracking_status is required for Meta AEM on iOS")
	}
	return nil
}

func validateAdImpression(client *Client, input AdImpressionRequest) error {
	context := input.Context
	if !validOptionalOpaque(context.Identity.AdvertisingID, 4096) || !validOptionalOpaque(context.Identity.DeveloperDeviceID, 4096) {
		return fmt.Errorf("ad impression device identifiers are invalid")
	}
	if context.Identity.AnalyticsInstallationID != "" && !validInstallationID(context.Identity.AnalyticsInstallationID) {
		return fmt.Errorf("analytics_installation_id must be a non-zero UUID when supplied")
	}
	if unavailableAdvertisingID(context.Identity.AdvertisingID) && context.Identity.DeveloperDeviceID == "" && context.Identity.AnalyticsInstallationID == "" {
		return fmt.Errorf("at least one usable device identifier is required")
	}
	if client.platform == PlatformIOS && unavailableAdvertisingID(context.Identity.AdvertisingID) &&
		(context.Identity.DeveloperDeviceID == "" || context.Identity.AnalyticsInstallationID == "") {
		return fmt.Errorf("iOS without an IDFA requires developer_device_id and analytics_installation_id")
	}
	if !validOpaque(context.AppVersion, 1024) {
		return fmt.Errorf("app_version is required")
	}
	if net.ParseIP(context.IPAddress) == nil {
		return fmt.Errorf("ip_address must be a single IPv4 or IPv6 address")
	}
	if context.Country != "" && !countryPattern.MatchString(context.Country) {
		return fmt.Errorf("country must be an uppercase ISO country code")
	}
	if context.Language != "" && !languagePattern.MatchString(context.Language) {
		return fmt.Errorf("language must be a lowercase ISO 639-1 code")
	}
	if context.Connection != "" && context.Connection != ConnectionMobile && context.Connection != ConnectionWiFi {
		return fmt.Errorf("connection_type must be mobile or wifi")
	}
	if !validSourceAppStore(context.SourceAppStore) {
		return fmt.Errorf("source_app_store is invalid")
	}
	if context.AppVersionCode != "" && !digitsPattern.MatchString(context.AppVersionCode) {
		return fmt.Errorf("app_version_code must contain only digits")
	}
	if client.platform == PlatformIOS && (context.AppVersionCode != "" || context.DeviceProduct != "" || context.SourceAppStore != "" || context.OSVersionRelease != "") {
		return fmt.Errorf("app_version_code, device_product, source_app_store, and os_version_release are Android-only ILRD fields")
	}
	if context.ScreenHeight != nil && (*context.ScreenHeight <= 0 || *context.ScreenHeight > 100_000) {
		return fmt.Errorf("screen_height is invalid")
	}
	if context.ScreenWidth != nil && (*context.ScreenWidth <= 0 || *context.ScreenWidth > 100_000) {
		return fmt.Errorf("screen_width is invalid")
	}
	if context.SentAt != nil && (context.SentAt.IsZero() || context.SentAt.UnixMilli() <= 0) {
		return fmt.Errorf("sent_at is invalid")
	}
	for name, value := range map[string]string{
		"app_version_code":    context.AppVersionCode,
		"build_id":            context.BuildID,
		"carrier":             context.Carrier,
		"device":              context.Device,
		"device_brand":        context.DeviceBrand,
		"device_manufacturer": context.DeviceManufacturer,
		"device_model":        context.DeviceModel,
		"device_product":      context.DeviceProduct,
		"locale":              context.Locale,
		"os_version":          context.OSVersion,
		"os_version_release":  context.OSVersionRelease,
		"timezone":            context.Timezone,
		"session_id":          context.SessionID,
	} {
		if !validOptionalOpaque(value, 4096) {
			return fmt.Errorf("%s is invalid", name)
		}
	}
	if !validOptionalText(context.UserAgent, 32<<10) {
		return fmt.Errorf("user_agent is invalid")
	}
	if !validMediation(input.Mediation) {
		return fmt.Errorf("ad_revenue_mediation is invalid")
	}
	if !validOpaque(input.NetworkName, 4096) {
		return fmt.Errorf("network_name is required")
	}
	if !currencyPattern.MatchString(input.Currency) {
		return fmt.Errorf("currency must be an uppercase ISO 4217 code")
	}
	if input.RevenueDecimal == "" && input.RevenueCPM == "" {
		return fmt.Errorf("revenue_decimal or revenue_cpm is required")
	}
	if input.RevenueDecimal != "" && !validDecimal(input.RevenueDecimal) {
		return fmt.Errorf("revenue_decimal must be a non-negative plain decimal")
	}
	if input.RevenueCPM != "" && !validDecimal(input.RevenueCPM) {
		return fmt.Errorf("revenue_cpm must be a non-negative plain decimal")
	}
	if input.MediationCountry != "" && !countryPattern.MatchString(input.MediationCountry) {
		return fmt.Errorf("mediation_country must be an uppercase ISO country code")
	}
	if !validAdFormat(input.Format) {
		return fmt.Errorf("ad_format is invalid")
	}
	for name, value := range map[string]string{
		"ad_unit_id":        input.AdUnitID,
		"precision":         input.Precision,
		"creative_id":       input.CreativeID,
		"placement":         input.Placement,
		"network_placement": input.NetworkPlacement,
		"auction_id":        input.AuctionID,
	} {
		if !validOptionalOpaque(value, 4096) {
			return fmt.Errorf("%s is invalid", name)
		}
	}
	return nil
}

func validPlatform(value Platform) bool {
	return value == PlatformIOS || value == PlatformAndroid || value == PlatformAmazon || value == PlatformAndroidOther
}

func validMediation(value Mediation) bool {
	switch value {
	case MediationMAX, MediationIronSource, MediationAdMob, MediationTopOn, MediationCAS, MediationTradPlus, MediationCustom:
		return true
	default:
		return false
	}
}

func validSourceAppStore(value SourceAppStore) bool {
	return value == "" || value == SourceStoreUnspecified || value == SourceStoreGooglePlay || value == SourceStoreAmazon || value == SourceStoreOther
}

func validAdFormat(value AdFormat) bool {
	switch value {
	case "", AdFormatBanner, AdFormatMREC, AdFormatCrossPromotion, AdFormatNative,
		AdFormatLeaderboard, AdFormatLeader, AdFormatInterstitial, AdFormatInter,
		AdFormatRewarded, AdFormatReward, AdFormatRewardedInterstitial, AdFormatRewardedInter:
		return true
	default:
		return false
	}
}

func validBundleID(value string) bool {
	return bundlePattern.MatchString(value)
}

func validInstallationID(value string) bool {
	if !installationPattern.MatchString(value) {
		return false
	}
	return strings.Trim(strings.ReplaceAll(value, "-", ""), "0") != ""
}

func unavailableAdvertisingID(value string) bool {
	compact := strings.ReplaceAll(strings.TrimSpace(value), "-", "")
	return compact == "" || strings.Trim(compact, "0") == ""
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

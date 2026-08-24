package appsflyer

import (
	"fmt"
	"net"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	iosAppIDPattern    = regexp.MustCompile(`^id[0-9]+$`)
	otherAppIDPattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	appsFlyerIDPattern = regexp.MustCompile(`^[0-9]{13}-[0-9]{1,19}$`)
	sha256Pattern      = regexp.MustCompile(`^[0-9a-f]{64}$`)
	currencyPattern    = regexp.MustCompile(`^[A-Z]{3}$`)
	decimalPattern     = regexp.MustCompile(`^-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?$`)
	uuidPattern        = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

func validateAccountSettings(settings AccountSettings) error {
	if !validPlatform(settings.Platform) {
		return fmt.Errorf("account.settings.platform must be ios, android, or windows")
	}
	if len(settings.AppID) > 512 || !validAppID(settings.AppID, settings.Platform) {
		if settings.Platform == PlatformIOS {
			return fmt.Errorf("account.settings.app_id must use the iOS id<number> form")
		}
		return fmt.Errorf("account.settings.app_id is invalid")
	}
	if !validOptionalOpaque(settings.BundleIdentifier, 512) {
		return fmt.Errorf("account.settings.bundle_identifier is invalid")
	}
	return nil
}

func validateEvent(request EventRequest, platform Platform) error {
	if !appsFlyerIDPattern.MatchString(request.AppsFlyerID) {
		return fmt.Errorf("appsflyer_id must contain a 13-digit timestamp and numeric suffix")
	}
	if !validOpaque(request.EventName, 512) {
		return fmt.Errorf("eventName is required")
	}
	if err := validateEventValues(request.EventValue); err != nil {
		return err
	}
	if request.EventTime != nil && request.EventTime.IsZero() {
		return fmt.Errorf("eventTime is invalid")
	}
	if request.EventCurrency != "" && !currencyPattern.MatchString(request.EventCurrency) {
		return fmt.Errorf("eventCurrency must be an uppercase ISO 4217 code")
	}
	for _, field := range []struct {
		name    string
		value   string
		maximum int
	}{
		{"bundleIdentifier", request.BundleIdentifier, 512}, {"app_version_name", request.AppVersionName, 512},
		{"app_store", request.AppStore, 512}, {"os", request.OS, 128},
		{"customer_user_id", request.CustomerUserID, 4096},
	} {
		if !validOptionalOpaque(field.value, field.maximum) {
			return fmt.Errorf("%s is invalid", field.name)
		}
	}
	if !validOptionalText(request.UserAgent, 8192) {
		return fmt.Errorf("ua is invalid")
	}
	if request.IPAddress != "" {
		ip := net.ParseIP(request.IPAddress)
		if len(request.IPAddress) > 16 || ip == nil || ip.To4() == nil {
			return fmt.Errorf("ip must be an IPv4 address")
		}
	}
	if platform == PlatformIOS && request.OS == "" {
		return fmt.Errorf("os is required for iOS events")
	}
	if err := validateDevice(request.Device, platform); err != nil {
		return err
	}
	if err := validateHashedUser(request.HashedUser); err != nil {
		return err
	}
	if err := validateSharingFilter(request.SharingFilter); err != nil {
		return err
	}
	if err := validateCustomData(request.CustomData, 0); err != nil {
		return err
	}
	if request.AppType != "" && request.AppType != AppTypeAppClip {
		return fmt.Errorf("app_type must be app_clip")
	}
	if request.AppType == AppTypeAppClip && platform != PlatformIOS {
		return fmt.Errorf("app_type app_clip is valid only for iOS apps")
	}
	if request.ATT != nil && (*request.ATT < 0 || *request.ATT > 3) {
		return fmt.Errorf("att must be between 0 and 3")
	}
	if request.ATT != nil && platform != PlatformIOS {
		return fmt.Errorf("att is valid only for iOS apps")
	}
	if request.AIE != nil && platform == PlatformWindows {
		return fmt.Errorf("aie is documented only for Android and iOS apps")
	}
	if request.Consent != nil {
		if platform == PlatformWindows {
			return fmt.Errorf("consent_data is documented only for Android and iOS apps")
		}
		if err := validateConsent(*request.Consent); err != nil {
			return err
		}
	}
	if request.AppSetID != nil {
		if platform != PlatformAndroid {
			return fmt.Errorf("app_set_id is valid only for Android apps")
		}
		if request.AppSetID.Scope != AppSetIDScopeApp && request.AppSetID.Scope != AppSetIDScopeDeveloper {
			return fmt.Errorf("app_set_id.scope must be 1 or 2")
		}
		if !uuidPattern.MatchString(request.AppSetID.ID) {
			return fmt.Errorf("app_set_id.id must be a UUID")
		}
	}
	return nil
}

func validateEventValues(values EventValues) error {
	for key, value := range values {
		if !validOpaque(key, 512) {
			return fmt.Errorf("eventValue contains an invalid key")
		}
		if !validText(value, 8192) {
			return fmt.Errorf("eventValue contains an invalid value")
		}
	}
	return nil
}

func validateDevice(device DeviceIdentifiers, platform Platform) error {
	for _, field := range []struct{ name, value string }{
		{"advertising_id", device.AdvertisingID}, {"oaid", device.OAID}, {"amazon_aid", device.AmazonAID},
		{"imei", device.IMEI}, {"idfa", device.IDFA}, {"idfv", device.IDFV},
	} {
		if !validOptionalOpaque(field.value, 512) {
			return fmt.Errorf("%s is invalid", field.name)
		}
	}
	if device.AdvertisingID != "" && !uuidPattern.MatchString(device.AdvertisingID) {
		return fmt.Errorf("advertising_id must be a UUID")
	}
	if device.IDFA != "" && !uuidPattern.MatchString(device.IDFA) {
		return fmt.Errorf("idfa must be a UUID")
	}
	if device.IDFV != "" && !uuidPattern.MatchString(device.IDFV) {
		return fmt.Errorf("idfv must be a UUID")
	}
	if device.FBLoginID != "" {
		if len(device.FBLoginID) > 128 || strings.IndexFunc(device.FBLoginID, func(character rune) bool {
			return character < '0' || character > '9'
		}) >= 0 {
			return fmt.Errorf("fb_login_id must be a numeric string")
		}
	}
	androidIdentifiers := device.AdvertisingID != "" || device.OAID != "" || device.AmazonAID != "" || device.IMEI != ""
	appleIdentifiers := device.IDFA != "" || device.IDFV != ""
	switch platform {
	case PlatformIOS:
		if androidIdentifiers {
			return fmt.Errorf("Android device identifiers cannot be sent for an iOS app")
		}
	case PlatformAndroid:
		if appleIdentifiers {
			return fmt.Errorf("Apple device identifiers cannot be sent for an Android app")
		}
	case PlatformWindows:
		if androidIdentifiers || appleIdentifiers {
			return fmt.Errorf("mobile advertising identifiers are not documented for Windows events")
		}
	}
	return nil
}

func validateHashedUser(user HashedUserData) error {
	for _, field := range []struct{ name, value string }{
		{"email_hashed", user.Email}, {"phone_number_hashed", user.Phone},
		{"phone_number_e164_hashed", user.PhoneE164}, {"first_name_hashed", user.FirstName},
		{"last_name_hashed", user.LastName},
	} {
		if field.value != "" && !sha256Pattern.MatchString(field.value) {
			return fmt.Errorf("%s must be a lowercase SHA-256 digest", field.name)
		}
	}
	return nil
}

func validateSharingFilter(filter SharingFilter) error {
	if filter.BlockAll && len(filter.Partners) > 0 {
		return fmt.Errorf("sharing_filter cannot block all and list partners together")
	}
	seen := make(map[string]struct{}, len(filter.Partners))
	for _, partner := range filter.Partners {
		if !validOpaque(partner, 512) || partner == "all" {
			return fmt.Errorf("sharing_filter contains an invalid partner")
		}
		if _, exists := seen[partner]; exists {
			return fmt.Errorf("sharing_filter contains a duplicate partner")
		}
		seen[partner] = struct{}{}
	}
	return nil
}

func validateConsent(consent ConsentData) error {
	if (consent.Manual == nil) == (consent.TCF == nil) {
		return fmt.Errorf("consent_data must contain exactly one of manual or tcf")
	}
	if consent.Manual != nil && consent.Manual.GDPRApplies == nil &&
		consent.Manual.AdUserDataEnabled == nil && consent.Manual.AdPersonalizationEnabled == nil {
		return fmt.Errorf("consent_data.manual must contain at least one signal")
	}
	if consent.TCF != nil {
		if consent.TCF.PolicyVersion <= 0 || consent.TCF.CMPSDKID < 0 || consent.TCF.CMPSDKVersion < 0 ||
			(consent.TCF.GDPRApplies < -1 || consent.TCF.GDPRApplies > 1) || !validOpaque(consent.TCF.TCString, 8192) {
			return fmt.Errorf("consent_data.tcf is invalid")
		}
	}
	return nil
}

func validateCustomData(data CustomData, depth int) error {
	if depth > 8 {
		return fmt.Errorf("custom_data nesting exceeds eight levels")
	}
	seen := make(map[string]struct{}, len(data.Strings)+len(data.Numbers)+len(data.Booleans)+len(data.Objects))
	checkKey := func(key string) error {
		if !validOpaque(key, 512) {
			return fmt.Errorf("custom_data contains an invalid key")
		}
		if _, exists := seen[key]; exists {
			return fmt.Errorf("custom_data contains a duplicate key across value types")
		}
		seen[key] = struct{}{}
		return nil
	}
	for key, value := range data.Strings {
		if err := checkKey(key); err != nil {
			return err
		}
		if !validText(value, 8192) {
			return fmt.Errorf("custom_data contains an invalid string")
		}
	}
	for key, value := range data.Numbers {
		if err := checkKey(key); err != nil {
			return err
		}
		if !validDecimal(value) {
			return fmt.Errorf("custom_data contains an invalid number")
		}
	}
	for key := range data.Booleans {
		if err := checkKey(key); err != nil {
			return err
		}
	}
	for key, value := range data.Objects {
		if err := checkKey(key); err != nil {
			return err
		}
		if err := validateCustomData(value, depth+1); err != nil {
			return err
		}
	}
	return nil
}

func customDataEmpty(data CustomData) bool {
	return len(data.Strings) == 0 && len(data.Numbers) == 0 && len(data.Booleans) == 0 && len(data.Objects) == 0
}

func validPlatform(platform Platform) bool {
	return platform == PlatformIOS || platform == PlatformAndroid || platform == PlatformWindows
}

func validAppID(appID string, platform Platform) bool {
	if platform == PlatformIOS {
		return iosAppIDPattern.MatchString(appID)
	}
	return otherAppIDPattern.MatchString(appID)
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

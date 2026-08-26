package singular

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

const (
	maximumEventNameBytes       = 32
	maximumAttributeCharacters  = 500
	maximumGlobalPropertyChars  = 200
	maximumGlobalProperties     = 5
	maximumOpaqueBytes          = 4096
	maximumUserAgentBytes       = 8192
	maximumPurchaseReceiptBytes = 128 << 10
)

var (
	uuidV4Pattern   = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	decimalPattern  = regexp.MustCompile(`^-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?$`)
	currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)
	countryPattern  = regexp.MustCompile(`^[A-Z]{2}$`)
	localePattern   = regexp.MustCompile(`^[A-Za-z]{2}_[A-Za-z]{2}$`)
	sha256Pattern   = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

func validateEvent(request EventRequest) error {
	if !validPlatform(request.Platform) {
		return fmt.Errorf("platform must be Android, iOS, Web, PC, Xbox, PlayStation, Nintendo, MetaQuest, or CTV")
	}
	if !uuidV4Pattern.MatchString(request.SDID) {
		return fmt.Errorf("sdid must be a UUIDv4")
	}
	if !validEventName(string(request.Name)) {
		return fmt.Errorf("event name must contain 1-32 printable ASCII characters without surrounding whitespace")
	}
	if request.OccurredAt != nil && request.OccurredAt.UnixMilli() <= 0 {
		return fmt.Errorf("occurred_at must be after the Unix epoch")
	}
	if err := validateNetworkIdentity(request); err != nil {
		return err
	}
	if err := validatePlatformFields(request); err != nil {
		return err
	}
	if err := validateProperties("attributes", request.Attributes); err != nil {
		return err
	}
	if err := validateGlobalProperties(request.GlobalProperties); err != nil {
		return err
	}
	if !validOptionalText(request.UserAgent, maximumUserAgentBytes) {
		return fmt.Errorf("user_agent is invalid")
	}
	if !validOptionalOpaque(request.CustomUserID, maximumOpaqueBytes) {
		return fmt.Errorf("custom_user_id is invalid")
	}
	if request.Revenue != nil && request.AdRevenue != nil {
		return fmt.Errorf("revenue and ad_revenue are mutually exclusive")
	}
	if request.Revenue != nil {
		if err := validateRevenue(*request.Revenue, request.Platform); err != nil {
			return err
		}
	}
	if request.AdRevenue != nil {
		if request.Name != EventAdMonetizationRevenue {
			return fmt.Errorf("ad_revenue requires event name __ADMON_USER_LEVEL_REVENUE__")
		}
		if err := validateAdRevenue(*request.AdRevenue, request.Attributes); err != nil {
			return err
		}
	} else if request.Name == EventAdMonetizationRevenue {
		return fmt.Errorf("__ADMON_USER_LEVEL_REVENUE__ requires ad_revenue")
	}
	if err := validateSKAN(request.SKAN, request.Platform); err != nil {
		return err
	}
	return validateWeb(request)
}

func validateNetworkIdentity(request EventRequest) error {
	if request.IPAddress != "" && request.UseRequestIP {
		return fmt.Errorf("ip_address and use_request_ip are mutually exclusive")
	}
	if request.IPAddress == "" && !request.UseRequestIP {
		return fmt.Errorf("ip_address or use_request_ip is required")
	}
	if request.IPAddress != "" && net.ParseIP(request.IPAddress) == nil {
		return fmt.Errorf("ip_address must be a single IPv4 or IPv6 address")
	}
	if request.Country != "" && !countryPattern.MatchString(request.Country) {
		return fmt.Errorf("country must be an uppercase ISO 3166-1 alpha-2 code")
	}
	if request.UseRequestIP && request.Country == "" {
		return fmt.Errorf("country is required when use_request_ip is true")
	}
	return nil
}

func validatePlatformFields(request EventRequest) error {
	mobile := request.Platform == PlatformAndroid || request.Platform == PlatformIOS
	if mobile && !validOpaque(request.OSVersion, maximumOpaqueBytes) {
		return fmt.Errorf("os_version is required for mobile events")
	}
	if !mobile && (request.Manufacturer != "" || request.Model != "" || request.Locale != "" || request.Build != "" ||
		request.Connection != "" || request.CarrierName != "" || request.DoNotTrack != nil) {
		return fmt.Errorf("manufacturer, model, locale, build, connection, carrier_name, and do_not_track are mobile-only")
	}
	if (request.Manufacturer == "") != (request.Model == "") {
		return fmt.Errorf("manufacturer and model must be supplied together")
	}
	for _, field := range []struct{ name, value string }{
		{"os_version", request.OSVersion}, {"manufacturer", request.Manufacturer}, {"model", request.Model},
		{"build", request.Build}, {"app_version", request.AppVersion}, {"carrier_name", request.CarrierName},
	} {
		if !validOptionalOpaque(field.value, maximumOpaqueBytes) {
			return fmt.Errorf("%s is invalid", field.name)
		}
	}
	if request.Locale != "" && !localePattern.MatchString(request.Locale) {
		return fmt.Errorf("locale must use the ll_CC form")
	}
	if request.Connection != "" && request.Connection != ConnectionWiFi && request.Connection != ConnectionCarrier {
		return fmt.Errorf("connection must be wifi or carrier")
	}
	if request.Platform == PlatformIOS {
		if request.ATTStatus == nil || *request.ATTStatus < 0 || *request.ATTStatus > 3 {
			return fmt.Errorf("att_status is required for iOS and must be between 0 and 3")
		}
	} else if request.ATTStatus != nil {
		return fmt.Errorf("att_status is valid only for iOS")
	}
	return nil
}

func validateProperties(name string, properties Properties) error {
	seen := make(map[string]struct{}, len(properties.Strings)+len(properties.Numbers)+len(properties.Booleans)+len(properties.StringLists))
	checkKey := func(key string) error {
		if !validOpaqueCharacters(key, maximumAttributeCharacters) {
			return fmt.Errorf("%s contains an invalid key", name)
		}
		if _, exists := seen[key]; exists {
			return fmt.Errorf("%s contains a duplicate key across value types", name)
		}
		seen[key] = struct{}{}
		return nil
	}
	for key, value := range properties.Strings {
		if err := checkKey(key); err != nil {
			return err
		}
		if !validTextCharacters(value, maximumAttributeCharacters) {
			return fmt.Errorf("%s contains an invalid string value", name)
		}
		if isHashAttribute(key) && !sha256Pattern.MatchString(value) {
			return fmt.Errorf("%s.%s must be a lowercase SHA-256 digest", name, key)
		}
	}
	for key, value := range properties.Numbers {
		if err := checkKey(key); err != nil {
			return err
		}
		if isHashAttribute(key) {
			return fmt.Errorf("%s.%s must be a string SHA-256 digest", name, key)
		}
		if !validDecimal(value) {
			return fmt.Errorf("%s contains an invalid number", name)
		}
	}
	for key := range properties.Booleans {
		if err := checkKey(key); err != nil {
			return err
		}
		if isHashAttribute(key) {
			return fmt.Errorf("%s.%s must be a string SHA-256 digest", name, key)
		}
	}
	for key, values := range properties.StringLists {
		if err := checkKey(key); err != nil {
			return err
		}
		if isHashAttribute(key) {
			return fmt.Errorf("%s.%s must be a string SHA-256 digest", name, key)
		}
		for _, value := range values {
			if !validTextCharacters(value, maximumAttributeCharacters) {
				return fmt.Errorf("%s contains an invalid string-list value", name)
			}
		}
	}
	return nil
}

func validateGlobalProperties(properties map[string]string) error {
	if len(properties) > maximumGlobalProperties {
		return fmt.Errorf("global_properties cannot contain more than five entries")
	}
	for key, value := range properties {
		if !validOpaqueCharacters(key, maximumGlobalPropertyChars) || !validTextCharacters(value, maximumGlobalPropertyChars) {
			return fmt.Errorf("global_properties keys and values must not exceed 200 characters")
		}
	}
	return nil
}

func validateRevenue(revenue Revenue, platform Platform) error {
	if revenue.Amount != "" && !validDecimal(revenue.Amount) {
		return fmt.Errorf("revenue.amount must be an exact decimal")
	}
	if (revenue.Amount == "") != (revenue.Currency == "") {
		return fmt.Errorf("revenue.amount and revenue.currency must be supplied together")
	}
	if revenue.Currency != "" && !currencyPattern.MatchString(revenue.Currency) {
		return fmt.Errorf("revenue.currency must be an uppercase ISO 4217 code")
	}
	if revenue.Amount == "" && (revenue.IsRevenueEvent == nil || !*revenue.IsRevenueEvent) {
		return fmt.Errorf("revenue without an amount requires is_revenue_event=true")
	}
	mobile := platform == PlatformAndroid || platform == PlatformIOS
	if revenue.PurchaseReceipt != "" && !mobile {
		return fmt.Errorf("revenue.purchase_receipt is valid only for iOS and Android")
	}
	if !validOptionalText(revenue.PurchaseReceipt, maximumPurchaseReceiptBytes) {
		return fmt.Errorf("revenue.purchase_receipt is invalid")
	}
	if revenue.ReceiptSignature != "" {
		if platform != PlatformAndroid || revenue.PurchaseReceipt == "" {
			return fmt.Errorf("revenue.receipt_signature requires an Android purchase receipt")
		}
		if !validOpaque(revenue.ReceiptSignature, 16_384) {
			return fmt.Errorf("revenue.receipt_signature is invalid")
		}
	}
	for _, field := range []struct{ name, value string }{
		{"product_id", revenue.ProductID}, {"transaction_id", revenue.TransactionID},
	} {
		if !validOptionalOpaque(field.value, maximumOpaqueBytes) {
			return fmt.Errorf("revenue.%s is invalid", field.name)
		}
	}
	return nil
}

func validateAdRevenue(revenue AdRevenue, attributes Properties) error {
	if !validDecimal(revenue.Amount) {
		return fmt.Errorf("ad_revenue.amount must be an exact decimal")
	}
	if !currencyPattern.MatchString(revenue.Currency) {
		return fmt.Errorf("ad_revenue.currency must be an uppercase ISO 4217 code")
	}
	if !validOpaqueCharacters(revenue.AdPlatform, maximumAttributeCharacters) {
		return fmt.Errorf("ad_revenue.ad_platform is required")
	}
	for _, field := range []struct{ name, value string }{
		{"ad_mediation_platform", revenue.MediationPlatform}, {"ad_type", revenue.AdType},
		{"ad_group_type", revenue.AdGroupType}, {"ad_impression_id", revenue.AdImpressionID},
		{"ad_placement_name", revenue.AdPlacementName}, {"ad_unit_id", revenue.AdUnitID},
		{"ad_unit_name", revenue.AdUnitName}, {"ad_group_id", revenue.AdGroupID},
		{"ad_group_name", revenue.AdGroupName}, {"ad_group_priority", revenue.AdGroupPriority},
		{"ad_placement_id", revenue.AdPlacementID},
	} {
		if !validOptionalTextCharacters(field.value, maximumAttributeCharacters) {
			return fmt.Errorf("ad_revenue.%s is invalid", field.name)
		}
	}
	for _, key := range adRevenueAttributeKeys {
		if propertiesContain(attributes, key) {
			return fmt.Errorf("attributes.%s conflicts with typed ad_revenue", key)
		}
	}
	return nil
}

func validateSKAN(skan *SKANData, platform Platform) error {
	if skan == nil {
		return nil
	}
	if platform != PlatformIOS {
		return fmt.Errorf("skan is valid only for iOS")
	}
	if skan.ConversionValue == nil && skan.FirstCallTimestamp == nil && skan.LastCallTimestamp == nil {
		return fmt.Errorf("skan must contain at least one value")
	}
	if skan.ConversionValue != nil && (*skan.ConversionValue < 0 || *skan.ConversionValue > 63) {
		return fmt.Errorf("skan.conversion_value must be between 0 and 63")
	}
	for _, field := range []struct {
		name  string
		value *int64
	}{
		{"first_call_timestamp", skan.FirstCallTimestamp}, {"last_call_timestamp", skan.LastCallTimestamp},
	} {
		if field.value != nil && (*field.value < 1_000_000_000 || *field.value > 9_999_999_999) {
			return fmt.Errorf("skan.%s must be a Unix timestamp in seconds", field.name)
		}
	}
	if skan.FirstCallTimestamp != nil && skan.LastCallTimestamp != nil && *skan.FirstCallTimestamp > *skan.LastCallTimestamp {
		return fmt.Errorf("skan.first_call_timestamp must not exceed last_call_timestamp")
	}
	return nil
}

func validateWeb(request EventRequest) error {
	if request.Platform != PlatformWeb {
		if request.Web != nil || request.Name == EventPageVisit {
			return fmt.Errorf("web data and __PAGE_VISIT__ are valid only for Web")
		}
		return nil
	}
	if request.Web == nil {
		return fmt.Errorf("web data is required for Web events")
	}
	web := *request.Web
	if request.Name == EventPageVisit {
		if !web.ConversionEvent {
			return fmt.Errorf("__PAGE_VISIT__ requires conversion_event=true")
		}
		if len(web.AttributionData) == 0 {
			return fmt.Errorf("__PAGE_VISIT__ requires attribution_data")
		}
		for _, required := range []string{"partner_name", "is_attributed", "partner_campaign_name"} {
			if !validOpaqueCharacters(web.AttributionData[required], maximumAttributeCharacters) {
				return fmt.Errorf("attribution_data.%s is required", required)
			}
		}
		if web.AttributionData["is_attributed"] != "true" {
			return fmt.Errorf("attribution_data.is_attributed must be true")
		}
	} else {
		if web.ConversionEvent {
			return fmt.Errorf("conversion_event=true is valid only for __PAGE_VISIT__")
		}
		if len(web.AttributionData) != 0 {
			return fmt.Errorf("non-conversion Web events must omit attribution_data")
		}
	}
	if !validWebURL(web.LandingPageURL) {
		return fmt.Errorf("web.landing_page_url must be an absolute HTTP(S) URL")
	}
	for key, value := range web.AttributionData {
		if !validOpaqueCharacters(key, maximumAttributeCharacters) || !validTextCharacters(value, maximumAttributeCharacters) {
			return fmt.Errorf("attribution_data contains an invalid key or value")
		}
	}
	for _, field := range []struct {
		name    string
		value   string
		maximum int
	}{
		{"device_user_agent", web.DeviceUserAgent, maximumUserAgentBytes},
		{"page_referrer", web.PageReferrer, maximumUserAgentBytes},
		{"timezone", web.Timezone, maximumOpaqueBytes}, {"os", web.OS, maximumOpaqueBytes},
	} {
		if !validOptionalText(field.value, field.maximum) {
			return fmt.Errorf("web.%s is invalid", field.name)
		}
	}
	if web.PageReferrer != "" && !validWebURL(web.PageReferrer) {
		return fmt.Errorf("web.page_referrer must be an absolute HTTP(S) URL")
	}
	if web.ScreenWidth != nil && *web.ScreenWidth <= 0 || web.ScreenHeight != nil && *web.ScreenHeight <= 0 {
		return fmt.Errorf("web screen dimensions must be positive")
	}
	return nil
}

var adRevenueAttributeKeys = []string{
	"ad_platform", "ad_mediation_platform", "ad_type", "ad_group_type", "ad_impression_id",
	"ad_placement_name", "ad_unit_id", "ad_unit_name", "ad_group_id", "ad_group_name",
	"ad_group_priority", "ad_placement_id",
}

func propertiesContain(properties Properties, key string) bool {
	_, inStrings := properties.Strings[key]
	_, inNumbers := properties.Numbers[key]
	_, inBooleans := properties.Booleans[key]
	_, inLists := properties.StringLists[key]
	return inStrings || inNumbers || inBooleans || inLists
}

func isHashAttribute(key string) bool {
	switch AttributeKey(key) {
	case AttributeEmailHash, AttributePhoneHash, AttributeFirstNameHash, AttributeLastNameHash, AttributePhoneE164Hash:
		return true
	default:
		return false
	}
}

func validPlatform(platform Platform) bool {
	switch platform {
	case PlatformAndroid, PlatformIOS, PlatformWeb, PlatformPC, PlatformXbox, PlatformPlayStation,
		PlatformNintendo, PlatformMetaQuest, PlatformCTV:
		return true
	default:
		return false
	}
}

func validEventName(value string) bool {
	if value == "" || len(value) > maximumEventNameBytes || value != strings.TrimSpace(value) {
		return false
	}
	for index := 0; index < len(value); index++ {
		if value[index] < 0x20 || value[index] > 0x7e {
			return false
		}
	}
	return true
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

func validOpaqueCharacters(value string, maximum int) bool {
	return value != "" && value == strings.TrimSpace(value) && validTextCharacters(value, maximum)
}

func validTextCharacters(value string, maximum int) bool {
	return utf8.ValidString(value) && utf8.RuneCountInString(value) <= maximum && !strings.ContainsFunc(value, unicode.IsControl)
}

func validOptionalTextCharacters(value string, maximum int) bool {
	return value == "" || validTextCharacters(value, maximum)
}

func validWebURL(value string) bool {
	if !validOpaque(value, maximumUserAgentBytes) {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil
}

func prepareCallOptions(operation string, options []socialhub.CallOption) ([]socialhub.CallOption, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return nil, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" || resolved.IdempotencyKey != "" || len(resolved.Fields) != 0 {
		return nil, invalidArgument(operation, "only per-call timeouts are supported by Singular EVENT v2")
	}
	if resolved.Timeout < 0 {
		return nil, invalidArgument(operation, "timeout must not be negative")
	}
	if resolved.Timeout == 0 {
		return nil, nil
	}
	return []socialhub.CallOption{socialhub.WithCallTimeout(resolved.Timeout)}, nil
}

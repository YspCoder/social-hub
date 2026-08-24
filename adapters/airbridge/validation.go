package airbridge

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

const maximumCustomAttributesBytes = 2048

var (
	appNamePattern        = regexp.MustCompile(`^[a-z0-9]+$`)
	uuidV4Pattern         = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-4[0-9a-fA-F]{3}-[89aAbB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)
	decimalPattern        = regexp.MustCompile(`^-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?$`)
	currencyPattern       = regexp.MustCompile(`^[A-Z]{3}$`)
	acceptLanguagePattern = regexp.MustCompile(`^[A-Za-z]{2}$`)
)

func validAppName(value string) bool {
	return len(value) <= 256 && appNamePattern.MatchString(value)
}

func validOpaque(value string, maximum int) bool {
	return value != "" && value == strings.TrimSpace(value) && utf8.RuneCountInString(value) <= maximum && utf8.ValidString(value) &&
		!strings.ContainsFunc(value, unicode.IsControl)
}

func validOptionalOpaque(value string, maximum int) bool {
	return value == "" || validOpaque(value, maximum)
}

func validText(value string, maximum int) bool {
	return utf8.ValidString(value) && utf8.RuneCountInString(value) <= maximum && !strings.ContainsRune(value, '\x00')
}

func validOptionalText(value string, maximum int) bool {
	return value == "" || validText(value, maximum)
}

func validDecimal(value Decimal) bool {
	raw := string(value)
	if len(raw) == 0 || len(raw) > 128 || !decimalPattern.MatchString(raw) {
		return false
	}
	_, ok := new(big.Rat).SetString(raw)
	return ok
}

func validateMobileEvent(request MobileEventRequest, now time.Time) error {
	if err := validateCommonEvent(request.EventUUID, request.EventTimestamp, request.User, request.Goal, request.AcceptLanguage, now); err != nil {
		return err
	}
	if err := validateDevice(request.Device); err != nil {
		return err
	}
	if request.User.ExternalUserID == "" && request.Device.DeviceUUID == "" {
		return fmt.Errorf("user.external_user_id or device.device_uuid is required")
	}
	if request.Device.DeviceUUID != "" && (request.Device.OSName == "" || request.Device.OSVersion == "") {
		return fmt.Errorf("device.os_name and device.os_version are required with device.device_uuid")
	}
	if !validOpaque(request.App.PackageName, 1024) {
		return fmt.Errorf("app.package_name is required")
	}
	if !validOptionalOpaque(request.App.Version, 256) {
		return fmt.Errorf("app.version is invalid")
	}
	return validateMobileIP(request.ForwardedFor, request.Device.ClientIP)
}

func validateWebEvent(request WebEventRequest, now time.Time) error {
	if err := validateCommonEvent(request.EventUUID, request.EventTimestamp, request.User, request.Goal, request.AcceptLanguage, now); err != nil {
		return err
	}
	if !validOptionalOpaque(request.Browser.ClientID, 4096) {
		return fmt.Errorf("browser.client_id is invalid")
	}
	if !validOptionalText(request.Browser.UserAgent, 8192) {
		return fmt.Errorf("browser.user_agent is invalid")
	}
	if !validOptionalOpaque(request.ShortID, 4096) {
		return fmt.Errorf("short_id is invalid")
	}
	if request.Tracking != nil {
		if !validOptionalOpaque(request.Tracking.Channel, 4096) {
			return fmt.Errorf("tracking.channel is invalid")
		}
		if err := validateProperties("tracking.params", request.Tracking.Params); err != nil {
			return err
		}
	}
	if request.User.ExternalUserID == "" {
		if request.Browser.ClientID == "" {
			return fmt.Errorf("user.external_user_id or browser.client_id is required")
		}
		if request.ShortID == "" || request.Tracking == nil || request.Tracking.Channel == "" {
			return fmt.Errorf("short_id and tracking data are required when external_user_id is absent")
		}
	}
	if request.ForwardedFor == "" || net.ParseIP(request.ForwardedFor) == nil {
		return fmt.Errorf("forwarded_for must be a single IPv4 or IPv6 address")
	}
	return nil
}

func validateCommonEvent(eventUUID string, eventTimestamp *time.Time, user User, goal Goal, acceptLanguage string, now time.Time) error {
	if eventUUID != "" && !uuidV4Pattern.MatchString(eventUUID) {
		return fmt.Errorf("event_uuid must be a UUIDv4")
	}
	if eventTimestamp != nil {
		if eventTimestamp.After(now) {
			return fmt.Errorf("event_timestamp must not be in the future")
		}
		if eventTimestamp.Before(now.Add(-24 * time.Hour)) {
			return fmt.Errorf("event_timestamp is outside Airbridge's 24-hour processing window")
		}
	}
	if acceptLanguage != "" && !acceptLanguagePattern.MatchString(acceptLanguage) {
		return fmt.Errorf("accept_language must be an ISO 639-1 code")
	}
	if err := validateUser(user); err != nil {
		return err
	}
	return validateGoal(goal)
}

func validateUser(user User) error {
	for _, field := range []struct {
		name  string
		value string
	}{
		{"external_user_id", user.ExternalUserID},
		{"external_user_email", user.ExternalUserEmail},
		{"external_user_phone", user.ExternalUserPhone},
	} {
		if !validOptionalOpaque(field.value, 4096) {
			return fmt.Errorf("user.%s is invalid", field.name)
		}
	}
	return validateProperties("user.attributes", user.Attributes)
}

func validateDevice(device Device) error {
	for _, field := range []struct {
		name    string
		value   string
		maximum int
	}{
		{"device_uuid", device.DeviceUUID, 4096}, {"gaid", device.GAID, 4096}, {"ifa", device.IFA, 4096},
		{"app_set_id", device.AppSetID, 4096}, {"ifv", device.IFV, 4096}, {"device_model", device.DeviceModel, 1024},
		{"device_identifier", device.DeviceIdentifier, 1024}, {"manufacturer", device.Manufacturer, 1024},
		{"os_version", device.OSVersion, 256}, {"locale", device.Locale, 128}, {"timezone", device.Timezone, 256},
		{"orientation", device.Orientation, 128},
	} {
		if !validOptionalOpaque(field.value, field.maximum) {
			return fmt.Errorf("device.%s is invalid", field.name)
		}
	}
	if device.OSName != "" && device.OSName != OSAndroid && device.OSName != OSIOS {
		return fmt.Errorf("device.os_name must be Android or iOS")
	}
	if device.OSName != OSAndroid && (device.GAID != "" || device.AppSetID != "") {
		return fmt.Errorf("device.gaid and device.app_set_id require Android")
	}
	if device.OSName != OSIOS && (device.IFA != "" || device.IFV != "" || device.AppTrackingTransparency != nil || device.DeviceIdentifier != "") {
		return fmt.Errorf("Apple identifiers and app_tracking_transparency require iOS")
	}
	if device.AppTrackingTransparency != nil && (*device.AppTrackingTransparency < 0 || *device.AppTrackingTransparency > 3) {
		return fmt.Errorf("device.app_tracking_transparency must be between 0 and 3")
	}
	if device.Screen != nil {
		if !validOptionalOpaque(device.Screen.Density, 128) {
			return fmt.Errorf("device.screen.density is invalid")
		}
		if device.Screen.Height != nil && *device.Screen.Height <= 0 || device.Screen.Width != nil && *device.Screen.Width <= 0 {
			return fmt.Errorf("device screen dimensions must be positive")
		}
	}
	if device.Location != nil {
		if device.Location.Latitude != "" && (!validDecimal(device.Location.Latitude) || decimalCompareAbs(device.Location.Latitude, "90") > 0) {
			return fmt.Errorf("device.location.latitude is invalid")
		}
		if device.Location.Longitude != "" && (!validDecimal(device.Location.Longitude) || decimalCompareAbs(device.Location.Longitude, "180") > 0) {
			return fmt.Errorf("device.location.longitude is invalid")
		}
		if !validOptionalOpaque(device.Location.Speed, 128) {
			return fmt.Errorf("device.location.speed is invalid")
		}
	}
	if device.Network != nil && !validOptionalOpaque(device.Network.Carrier, 1024) {
		return fmt.Errorf("device.network.carrier is invalid")
	}
	if device.DMA != nil {
		if device.DMA.EEA == nil && (device.DMA.AdPersonalization != nil || device.DMA.AdUserData != nil) {
			return fmt.Errorf("device.dma.eea is required with DMA ad consent signals")
		}
		if device.DMA.EEA != nil && *device.DMA.EEA && (device.DMA.AdPersonalization == nil || device.DMA.AdUserData == nil) {
			return fmt.Errorf("DMA ad consent signals are required when device.dma.eea is true")
		}
		if device.DMA.EEA == nil && device.DMA.AdPersonalization == nil && device.DMA.AdUserData == nil {
			return fmt.Errorf("device.dma must not be empty")
		}
	}
	return nil
}

func validateMobileIP(forwardedFor, clientIP string) error {
	if forwardedFor != "" && clientIP != "" {
		return fmt.Errorf("forwarded_for and device.client_ip are mutually exclusive")
	}
	value := forwardedFor
	if value == "" {
		value = clientIP
	}
	if value == "" || net.ParseIP(value) == nil {
		return fmt.Errorf("forwarded_for or device.client_ip must be a single IPv4 or IPv6 address")
	}
	return nil
}

func validateGoal(goal Goal) error {
	if !validOpaque(string(goal.Category), 128) {
		return fmt.Errorf("goal.category is required and must not exceed 128 characters")
	}
	if goal.Value != "" && !validDecimal(goal.Value) {
		return fmt.Errorf("goal.value must be an exact decimal")
	}
	if err := validateProperties("goal.custom_attributes", goal.CustomAttributes); err != nil {
		return err
	}
	custom, err := json.Marshal(normalizeProperties(goal.CustomAttributes))
	if err != nil {
		return fmt.Errorf("goal.custom_attributes is invalid")
	}
	if !propertiesEmpty(goal.CustomAttributes) && len(custom) > maximumCustomAttributesBytes {
		return fmt.Errorf("goal.custom_attributes exceeds Airbridge's 2048-byte limit")
	}
	return validateSemanticAttributes(goal.SemanticAttributes)
}

func validateSemanticAttributes(attributes SemanticAttributes) error {
	for _, field := range []struct{ name, value string }{
		{"action", attributes.Action}, {"label", attributes.Label},
	} {
		if !validOptionalOpaque(field.value, 128) {
			return fmt.Errorf("goal.semantic_attributes.%s is invalid", field.name)
		}
	}
	for _, field := range []struct {
		name    string
		value   string
		maximum int
	}{
		{"period", attributes.Period, 1024}, {"product_list_id", attributes.ProductListID, 1024},
		{"cart_id", attributes.CartID, 1024}, {"transaction_id", attributes.TransactionID, 1024},
		{"transaction_type", attributes.TransactionType, 1024},
		{"transaction_paired_event_category", attributes.TransactionPairedEventCategory, 1024},
		{"query", attributes.Query, 1024}, {"wish_list_id", attributes.WishListID, 1024},
		{"content_id", attributes.ContentID, 1024}, {"content_name", attributes.ContentName, 1024},
		{"list_id", attributes.ListID, 1024}, {"rate_id", attributes.RateID, 1024},
		{"achievement_id", attributes.AchievementID, 1024}, {"shared_channel", attributes.SharedChannel, 1024},
		{"description", attributes.Description, 1024}, {"place", attributes.Place, 1024},
		{"schedule_id", attributes.ScheduleID, 1024}, {"type", attributes.Type, 1024}, {"level", attributes.Level, 1024},
	} {
		if !validOptionalText(field.value, field.maximum) {
			return fmt.Errorf("goal.semantic_attributes.%s is invalid", field.name)
		}
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{"currency", attributes.Currency}, {"original_currency", attributes.OriginalCurrency},
	} {
		if field.value != "" && !currencyPattern.MatchString(field.value) {
			return fmt.Errorf("goal.semantic_attributes.%s must be an uppercase ISO 4217 code", field.name)
		}
	}
	for _, field := range []struct {
		name  string
		value Decimal
	}{
		{"total_value", attributes.TotalValue}, {"original_total_value", attributes.OriginalTotalValue},
		{"contribution_margin", attributes.ContributionMargin},
		{"original_contribution_margin", attributes.OriginalContributionMargin},
		{"rate", attributes.Rate}, {"max_rate", attributes.MaxRate}, {"rating_value", attributes.RatingValue},
		{"max_rating_value", attributes.MaxRatingValue}, {"score", attributes.Score},
	} {
		if field.value != "" && !validDecimal(field.value) {
			return fmt.Errorf("goal.semantic_attributes.%s must be an exact decimal", field.name)
		}
	}
	for _, field := range []struct {
		name  string
		value *int64
	}{
		{"renewal_count", attributes.RenewalCount},
		{"transaction_paired_event_timestamp", attributes.TransactionPairedEventTimestamp},
		{"total_quantity", attributes.TotalQuantity},
	} {
		if field.value != nil && *field.value < 0 {
			return fmt.Errorf("goal.semantic_attributes.%s must not be negative", field.name)
		}
	}
	if attributes.Datetime != "" {
		if !validOptionalOpaque(attributes.Datetime, 1024) {
			return fmt.Errorf("goal.semantic_attributes.datetime is invalid")
		}
		if _, err := time.Parse(time.RFC3339, attributes.Datetime); err != nil {
			return fmt.Errorf("goal.semantic_attributes.datetime must use RFC3339")
		}
	}
	for index, product := range attributes.Products {
		if err := validateProduct(product); err != nil {
			return fmt.Errorf("goal.semantic_attributes.products[%d]: %w", index, err)
		}
	}
	for partner, values := range attributes.AdPartners {
		if !validOpaque(partner, 1024) {
			return fmt.Errorf("goal.semantic_attributes.ad_partners contains an invalid partner")
		}
		if err := validateProperties("goal.semantic_attributes.ad_partners."+partner, values); err != nil {
			return err
		}
	}
	return nil
}

func validateProduct(product Product) error {
	if product.ProductID == "" && product.Name == "" && product.Price == "" && product.Quantity == nil &&
		product.Currency == "" && product.Position == nil && product.CategoryID == "" && product.CategoryName == "" &&
		product.BrandID == "" && product.BrandName == "" {
		return fmt.Errorf("product must not be empty")
	}
	for _, field := range []struct{ name, value string }{
		{"product_id", product.ProductID}, {"name", product.Name}, {"category_id", product.CategoryID},
		{"category_name", product.CategoryName}, {"brand_id", product.BrandID}, {"brand_name", product.BrandName},
	} {
		if !validOptionalText(field.value, 1024) {
			return fmt.Errorf("%s is invalid", field.name)
		}
	}
	if product.Price != "" && !validDecimal(product.Price) {
		return fmt.Errorf("price must be an exact decimal")
	}
	if product.Quantity != nil && *product.Quantity < 0 || product.Position != nil && *product.Position < 0 {
		return fmt.Errorf("quantity and position must not be negative")
	}
	if product.Currency != "" && !currencyPattern.MatchString(product.Currency) {
		return fmt.Errorf("currency must be an uppercase ISO 4217 code")
	}
	return nil
}

func validateProperties(name string, properties Properties) error {
	seen := make(map[string]struct{}, len(properties.Strings)+len(properties.Numbers)+len(properties.Booleans))
	checkKey := func(key string) error {
		if !validOpaque(key, 1024) {
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
		if !validText(value, 8192) {
			return fmt.Errorf("%s contains an invalid string", name)
		}
	}
	for key, value := range properties.Numbers {
		if err := checkKey(key); err != nil {
			return err
		}
		if !validDecimal(value) {
			return fmt.Errorf("%s contains an invalid number", name)
		}
	}
	for key := range properties.Booleans {
		if err := checkKey(key); err != nil {
			return err
		}
	}
	return nil
}

func decimalCompareAbs(left Decimal, right string) int {
	leftValue, leftOK := new(big.Rat).SetString(string(left))
	rightValue, rightOK := new(big.Rat).SetString(right)
	if !leftOK || !rightOK {
		return 1
	}
	leftValue.Abs(leftValue)
	return leftValue.Cmp(rightValue)
}

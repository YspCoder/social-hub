package branch

import (
	"fmt"
	"math/big"
	"net"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	decimalPattern       = regexp.MustCompile(`^-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?$`)
	countryPattern       = regexp.MustCompile(`^[A-Z]{2}$`)
	languagePattern      = regexp.MustCompile(`^[A-Za-z]{2,3}(?:[-_][A-Za-z0-9]{2,8})*$`)
	currencyPattern      = regexp.MustCompile(`^[A-Z]{3}$`)
	reservedIPv4Prefixes = []netip.Prefix{
		netip.MustParsePrefix("0.0.0.0/8"),
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("100.64.0.0/10"),
		netip.MustParsePrefix("127.0.0.0/8"),
		netip.MustParsePrefix("169.254.0.0/16"),
		netip.MustParsePrefix("172.16.0.0/12"),
		netip.MustParsePrefix("192.0.0.0/24"),
		netip.MustParsePrefix("192.0.2.0/24"),
		netip.MustParsePrefix("192.88.99.0/24"),
		netip.MustParsePrefix("192.168.0.0/16"),
		netip.MustParsePrefix("198.18.0.0/15"),
		netip.MustParsePrefix("198.51.100.0/24"),
		netip.MustParsePrefix("203.0.113.0/24"),
		netip.MustParsePrefix("224.0.0.0/4"),
		netip.MustParsePrefix("240.0.0.0/4"),
	}
)

func validateStandardEvent(request StandardEventRequest) error {
	if !validStandardEvent(request.Name) {
		return fmt.Errorf("name is not a current Branch Standard Event")
	}
	if !validOptionalOpaque(request.CustomerEventAlias, 512) {
		return fmt.Errorf("customer_event_alias is invalid")
	}
	if err := validateUserData(request.UserData); err != nil {
		return err
	}
	if err := validateProperties("custom_data", request.CustomData); err != nil {
		return err
	}
	if request.EventData != nil {
		if err := validateEventData(*request.EventData); err != nil {
			return err
		}
	}
	if len(request.ContentItems) > 0 && !eventSupportsContent(request.Name) {
		return fmt.Errorf("content_items are valid only for Commerce and Content Standard Events")
	}
	for index, item := range request.ContentItems {
		if err := validateContentItem(item); err != nil {
			return fmt.Errorf("content_items[%d]: %w", index, err)
		}
	}
	return validateIPOverride(request.IPOverride, request.UserData.IP)
}

func validateCustomEvent(request CustomEventRequest) error {
	if !validOpaque(request.Name, 512) {
		return fmt.Errorf("name is required")
	}
	if strings.EqualFold(request.Name, "custom event") {
		return fmt.Errorf("name custom event is reserved by Branch")
	}
	if validStandardEvent(StandardEventName(strings.ToUpper(request.Name))) {
		return fmt.Errorf("a Standard Event name cannot be used as a custom event")
	}
	if err := validateUserData(request.UserData); err != nil {
		return err
	}
	if err := validateProperties("custom_data", request.CustomData); err != nil {
		return err
	}
	if err := validateProperties("meta_data", request.Metadata); err != nil {
		return err
	}
	if request.EventData != nil {
		if err := validateEventData(*request.EventData); err != nil {
			return err
		}
	}
	return validateIPOverride(request.IPOverride, request.UserData.IP)
}

func validateUserData(data UserData) error {
	if data.OS != "" && !validOperatingSystem(data.OS) {
		return fmt.Errorf("user_data.os is invalid")
	}
	if data.OS != OSAndroid && (data.AAID != "" || data.AndroidID != "" || data.LocalIP != "") {
		return fmt.Errorf("Android identifiers and local_ip require user_data.os Android")
	}
	if data.OS != OSIOS && (data.IDFA != "" || data.IDFV != "" || data.AnonID != "") {
		return fmt.Errorf("iOS identifiers and anon_id require user_data.os iOS")
	}
	if data.Environment != "" && data.Environment != EnvironmentFullApp && data.Environment != EnvironmentInstantApp {
		return fmt.Errorf("user_data.environment is invalid")
	}
	for _, field := range []struct {
		name    string
		value   string
		maximum int
	}{
		{"os_version", data.OSVersion, 128}, {"aaid", data.AAID, 512}, {"android_id", data.AndroidID, 512},
		{"idfa", data.IDFA, 512}, {"idfv", data.IDFV, 512}, {"anon_id", data.AnonID, 1024},
		{"browser_fingerprint_id", data.BrowserFingerprintID, 1024},
		{"developer_identity", data.DeveloperIdentity, 4096}, {"google_analytics_id", data.GoogleAnalyticsID, 1024},
		{"randomized_device_token", data.RandomizedDeviceToken, 1024},
		{"brand", data.Brand, 512}, {"app_version", data.AppVersion, 512}, {"model", data.Model, 512},
	} {
		if !validOptionalOpaque(field.value, field.maximum) {
			return fmt.Errorf("user_data.%s is invalid", field.name)
		}
	}
	if !validOptionalText(data.UserAgent, 8192) {
		return fmt.Errorf("user_data.user_agent is invalid")
	}
	if data.HTTPOrigin != "" && !validHTTPURL(data.HTTPOrigin) {
		return fmt.Errorf("user_data.http_origin is invalid")
	}
	if data.HTTPReferrer != "" && !validHTTPURL(data.HTTPReferrer) {
		return fmt.Errorf("user_data.http_referrer is invalid")
	}
	if data.Country != "" && !countryPattern.MatchString(data.Country) {
		return fmt.Errorf("user_data.country must be an uppercase ISO 3166-1 alpha-2 code")
	}
	if data.Language != "" && !languagePattern.MatchString(data.Language) {
		return fmt.Errorf("user_data.language is invalid")
	}
	if data.IP != "" && net.ParseIP(data.IP) == nil {
		return fmt.Errorf("user_data.ip is invalid")
	}
	if data.LocalIP != "" && net.ParseIP(data.LocalIP) == nil {
		return fmt.Errorf("user_data.local_ip is invalid")
	}
	if err := validateAdvertisingIDs(data.AdvertisingIDs); err != nil {
		return err
	}
	for _, field := range []struct {
		name  string
		value *int64
	}{
		{"screen_dpi", data.ScreenDPI}, {"screen_height", data.ScreenHeight}, {"screen_width", data.ScreenWidth},
	} {
		if field.value != nil && *field.value <= 0 {
			return fmt.Errorf("user_data.%s must be positive", field.name)
		}
	}
	if data.DMAEEA == nil && (data.DMAAdPersonalization != nil || data.DMAAdUserData != nil) {
		return fmt.Errorf("user_data.dma_eea is required when DMA consent signals are provided")
	}
	if data.DMAEEA != nil && *data.DMAEEA && (data.DMAAdPersonalization == nil || data.DMAAdUserData == nil) {
		return fmt.Errorf("user_data DMA ad consent signals are required when dma_eea is true")
	}
	identified := data.DeveloperIdentity != "" || data.BrowserFingerprintID != "" ||
		(data.OS == OSIOS && (data.IDFA != "" || data.IDFV != "")) ||
		(data.OS == OSAndroid && (data.AAID != "" || data.AndroidID != ""))
	if !identified {
		return fmt.Errorf("user_data requires a developer identity, browser fingerprint, or platform identifier pair")
	}
	return nil
}

func validateAdvertisingIDs(identifiers map[string]string) error {
	for key, value := range identifiers {
		if !validOpaque(key, 128) || strings.IndexFunc(key, func(character rune) bool {
			return character != '_' && (character < 'a' || character > 'z') && (character < '0' || character > '9')
		}) >= 0 {
			return fmt.Errorf("user_data.advertising_ids contains an invalid key")
		}
		if !validOpaque(value, 1024) {
			return fmt.Errorf("user_data.advertising_ids contains an invalid value")
		}
	}
	return nil
}

func validateEventData(data EventData) error {
	for _, field := range []struct{ name, value string }{
		{"transaction_id", data.TransactionID}, {"coupon", data.Coupon}, {"affiliation", data.Affiliation},
	} {
		if !validOptionalOpaque(field.value, 4096) {
			return fmt.Errorf("event_data.%s is invalid", field.name)
		}
	}
	for _, field := range []struct{ name, value string }{
		{"description", data.Description}, {"search_query", data.SearchQuery},
	} {
		if !validOptionalText(field.value, 8192) {
			return fmt.Errorf("event_data.%s is invalid", field.name)
		}
	}
	for _, field := range []struct {
		name  string
		value Decimal
	}{
		{"revenue", data.Revenue}, {"shipping", data.Shipping}, {"tax", data.Tax},
	} {
		if field.value != "" && !validDecimal(field.value) {
			return fmt.Errorf("event_data.%s must be an exact decimal", field.name)
		}
	}
	if data.Currency != "" && !currencyPattern.MatchString(data.Currency) {
		return fmt.Errorf("event_data.currency must be an uppercase ISO 4217 code")
	}
	return nil
}

func validateContentItem(item ContentItem) error {
	if contentItemEmpty(item) {
		return fmt.Errorf("content item must not be empty")
	}
	if item.Schema != "" && !validContentSchema(item.Schema) {
		return fmt.Errorf("$content_schema is invalid")
	}
	if item.ProductCategory != "" && !validProductCategory(item.ProductCategory) {
		return fmt.Errorf("$product_category is invalid")
	}
	if item.Condition != "" && !validCondition(item.Condition) {
		return fmt.Errorf("$condition is invalid")
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{"$og_title", item.OGTitle}, {"$canonical_identifier", item.CanonicalIdentifier},
		{"$sku", item.SKU}, {"$product_name", item.ProductName}, {"$product_brand", item.ProductBrand},
		{"$product_variant", item.ProductVariant}, {"$address_street", item.AddressStreet},
		{"$address_city", item.AddressCity}, {"$address_region", item.AddressRegion},
		{"$address_country", item.AddressCountry}, {"$address_postal_code", item.AddressPostalCode},
	} {
		if !validOptionalOpaque(field.value, 4096) {
			return fmt.Errorf("%s is invalid", field.name)
		}
	}
	if !validOptionalText(item.OGDescription, 8192) {
		return fmt.Errorf("$og_description is invalid")
	}
	if item.OGImageURL != "" && !validHTTPURL(item.OGImageURL) {
		return fmt.Errorf("$og_image_url is invalid")
	}
	for _, field := range []struct {
		name  string
		value Decimal
	}{
		{"$price", item.Price}, {"$quantity", item.Quantity}, {"$rating_average", item.RatingAverage},
		{"$rating_count", item.RatingCount}, {"$rating_max", item.RatingMax},
	} {
		if field.value != "" && (!validDecimal(field.value) || decimalSign(field.value) < 0) {
			return fmt.Errorf("%s must be a non-negative exact decimal", field.name)
		}
	}
	if item.RatingAverage != "" && item.RatingMax != "" && decimalCompare(item.RatingAverage, item.RatingMax) > 0 {
		return fmt.Errorf("$rating_average must not exceed $rating_max")
	}
	if item.CreationTimestamp != nil && *item.CreationTimestamp < 0 ||
		item.ExpirationTimestamp != nil && *item.ExpirationTimestamp < 0 {
		return fmt.Errorf("content timestamps must not be negative")
	}
	if item.Latitude != "" && (!validDecimal(item.Latitude) || decimalCompareAbs(item.Latitude, "90") > 0) {
		return fmt.Errorf("$latitude is invalid")
	}
	if item.Longitude != "" && (!validDecimal(item.Longitude) || decimalCompareAbs(item.Longitude, "180") > 0) {
		return fmt.Errorf("$longitude is invalid")
	}
	for _, values := range []struct {
		name   string
		values []string
	}{{"$keywords", item.Keywords}, {"$image_captions", item.ImageCaptions}} {
		for _, value := range values.values {
			if !validOpaque(value, 4096) {
				return fmt.Errorf("%s contains an invalid value", values.name)
			}
		}
	}
	return validateProperties("$custom_fields", item.CustomFields)
}

func validateProperties(name string, properties Properties) error {
	seen := make(map[string]struct{}, len(properties.Strings)+len(properties.Numbers)+len(properties.Booleans))
	checkKey := func(key string) error {
		if !validOpaque(key, 512) {
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

func validateIPOverride(override, bodyIP string) error {
	if override == "" {
		return nil
	}
	if !isPublicIPv4(override) {
		return fmt.Errorf("X-IP-Override must be a public IPv4 address")
	}
	if bodyIP != override {
		return fmt.Errorf("X-IP-Override must exactly match user_data.ip")
	}
	return nil
}

func isPublicIPv4(value string) bool {
	address, err := netip.ParseAddr(value)
	if err != nil || !address.Is4() || !address.IsGlobalUnicast() {
		return false
	}
	for _, prefix := range reservedIPv4Prefixes {
		if prefix.Contains(address) {
			return false
		}
	}
	return true
}

func validStandardEvent(value StandardEventName) bool {
	switch value {
	case EventAddToCart, EventAddToWishlist, EventClickAd, EventViewCart, EventInitiatePurchase,
		EventAddPaymentInfo, EventPurchase, EventSpendCredits, EventViewAd,
		EventSearch, EventViewItem, EventViewItems, EventRate, EventShare, EventInitiateStream,
		EventCompleteStream, EventCompleteRegistration, EventCompleteTutorial, EventAchieveLevel,
		EventUnlockAchievement, EventInvite, EventLogin, EventStartTrial, EventSubscribe:
		return true
	default:
		return false
	}
}

func eventSupportsContent(value StandardEventName) bool {
	switch value {
	case EventAddToCart, EventAddToWishlist, EventClickAd, EventViewCart, EventInitiatePurchase,
		EventAddPaymentInfo, EventPurchase, EventSpendCredits, EventViewAd,
		EventSearch, EventViewItem, EventViewItems, EventRate, EventShare, EventInitiateStream, EventCompleteStream:
		return true
	default:
		return false
	}
}

func validOperatingSystem(value OperatingSystem) bool {
	return value == OSAndroid || value == OSIOS || value == OSMac || value == OSLinux || value == OSWindows
}

func validContentSchema(value ContentSchema) bool {
	switch value {
	case SchemaCommerceAuction, SchemaCommerceBusiness, SchemaCommerceOther, SchemaCommerceProduct,
		SchemaCommerceRestaurant, SchemaCommerceService, SchemaCommerceTravelFlight, SchemaCommerceTravelHotel,
		SchemaCommerceTravelOther, SchemaGameState, SchemaMediaImage, SchemaMediaMixed, SchemaMediaMusic,
		SchemaMediaOther, SchemaMediaVideo, SchemaOther, SchemaTextArticle, SchemaTextBlog, SchemaTextOther,
		SchemaTextRecipe, SchemaTextReview, SchemaTextSearchResults, SchemaTextStory, SchemaTextTechnicalDoc:
		return true
	default:
		return false
	}
}

func validProductCategory(value ProductCategory) bool {
	switch value {
	case ProductAnimalsPetSupplies, ProductApparelAccessories, ProductArtsEntertainment, ProductBabyToddler,
		ProductBusinessIndustrial, ProductCamerasOptics, ProductElectronics, ProductFoodBeveragesTobacco,
		ProductFurniture, ProductHardware, ProductHealthBeauty, ProductHomeGarden, ProductLuggageBags,
		ProductMature, ProductMedia, ProductOfficeSupplies, ProductReligiousCeremonial, ProductSoftware,
		ProductSportingGoods, ProductToysGames, ProductVehiclesParts:
		return true
	default:
		return false
	}
}

func validCondition(value ContentCondition) bool {
	switch value {
	case ConditionOther, ConditionNew, ConditionExcellent, ConditionGood, ConditionFair,
		ConditionPoor, ConditionUsed, ConditionRefurbished:
		return true
	default:
		return false
	}
}

func validBranchKey(value string) bool {
	return (strings.HasPrefix(value, "key_live_") || strings.HasPrefix(value, "key_test_")) && validOpaque(value, 16_384)
}

func validDecimal(value Decimal) bool {
	raw := string(value)
	return len(raw) > 0 && len(raw) <= 128 && decimalPattern.MatchString(raw)
}

func decimalSign(value Decimal) int {
	number, _ := new(big.Rat).SetString(string(value))
	return number.Sign()
}

func decimalCompare(left, right Decimal) int {
	leftNumber, _ := new(big.Rat).SetString(string(left))
	rightNumber, _ := new(big.Rat).SetString(string(right))
	return leftNumber.Cmp(rightNumber)
}

func decimalCompareAbs(value Decimal, maximum string) int {
	number, _ := new(big.Rat).SetString(string(value))
	number.Abs(number)
	limit, _ := new(big.Rat).SetString(maximum)
	return number.Cmp(limit)
}

func eventDataEmpty(data EventData) bool {
	return data.TransactionID == "" && data.Revenue == "" && data.Currency == "" && data.Shipping == "" &&
		data.Tax == "" && data.Coupon == "" && data.Affiliation == "" && data.Description == "" && data.SearchQuery == ""
}

func contentItemEmpty(item ContentItem) bool {
	return item.Schema == "" && item.OGTitle == "" && item.OGDescription == "" && item.OGImageURL == "" &&
		item.CanonicalIdentifier == "" && item.PubliclyIndexable == nil && item.LocallyIndexable == nil &&
		item.Price == "" && item.Quantity == "" && item.SKU == "" && item.ProductName == "" &&
		item.ProductBrand == "" && item.ProductCategory == "" && item.ProductVariant == "" &&
		item.RatingAverage == "" && item.RatingCount == "" && item.RatingMax == "" &&
		item.CreationTimestamp == nil && item.ExpirationTimestamp == nil && len(item.Keywords) == 0 &&
		item.AddressStreet == "" && item.AddressCity == "" && item.AddressRegion == "" &&
		item.AddressCountry == "" && item.AddressPostalCode == "" && item.Latitude == "" && item.Longitude == "" &&
		len(item.ImageCaptions) == 0 && item.Condition == "" && propertiesEmpty(item.CustomFields)
}

func propertiesEmpty(properties Properties) bool {
	return len(properties.Strings) == 0 && len(properties.Numbers) == 0 && len(properties.Booleans) == 0
}

func validHTTPURL(value string) bool {
	if len(value) > 8192 || !utf8.ValidString(value) || strings.ContainsFunc(value, unicode.IsControl) {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil
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

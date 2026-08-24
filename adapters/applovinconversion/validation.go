package applovinconversion

import (
	"fmt"
	"math/big"
	"net"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const maximumItemsPerEvent = 1000

var (
	decimalPattern        = regexp.MustCompile(`^(?:0|[1-9][0-9]*)(?:\.[0-9]+)?$`)
	currencyPattern       = regexp.MustCompile(`^[A-Z]{3}$`)
	countryPattern        = regexp.MustCompile(`^[A-Z]{2}$`)
	numericPattern        = regexp.MustCompile(`^[0-9]+$`)
	imageExtensionPattern = regexp.MustCompile(`(?i)\.(?:png|jpe?g)$`)
)

func validPolicy(value AccountPolicy) bool {
	return value == PolicyStandard || value == PolicyLeadGen || value == PolicyRestrictedLeadGen
}

func validOpaque(value string, maximum int) bool {
	return value != "" && utf8.ValidString(value) && value == strings.TrimSpace(value) && len(value) <= maximum &&
		!strings.ContainsFunc(value, unicode.IsControl)
}

func validOptionalOpaque(value string, maximum int) bool {
	return value == "" || validOpaque(value, maximum)
}

func validDecimal(value Decimal, allowZero bool) bool {
	raw := string(value)
	if len(raw) == 0 || len(raw) > 128 || !decimalPattern.MatchString(raw) {
		return false
	}
	number, ok := new(big.Rat).SetString(raw)
	return ok && (number.Sign() > 0 || allowZero && number.Sign() == 0)
}

func decimalAtMost(value Decimal, maximum *big.Rat) bool {
	number, ok := new(big.Rat).SetString(string(value))
	return ok && number.Cmp(maximum) <= 0
}

func validateBatch(policy AccountPolicy, events []ServerEvent) error {
	if len(events) == 0 || len(events) > MaximumBatchSize {
		return fmt.Errorf("events must contain between 1 and %d entries", MaximumBatchSize)
	}
	for index := range events {
		if err := validateEvent(policy, events[index]); err != nil {
			return fmt.Errorf("events[%d]: %w", index, err)
		}
	}
	return nil
}

func validateEvent(policy AccountPolicy, event ServerEvent) error {
	if event.EventTime < time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC).UnixMilli() ||
		event.EventTime > time.Date(9999, time.December, 31, 23, 59, 59, 999_000_000, time.UTC).UnixMilli() {
		return fmt.Errorf("event_time must be a Unix epoch timestamp in milliseconds")
	}
	parsedURL, err := validateSourceURL(event.EventSourceURL, policy == PolicyRestrictedLeadGen)
	if err != nil {
		return err
	}
	if !validOptionalOpaque(event.DedupeID, 512) {
		return fmt.Errorf("dedupe_id is invalid")
	}
	if err := validateUserData(policy, event.UserData); err != nil {
		return err
	}
	if parsedURL.Query().Has("aleid") && event.UserData.ALEID == "" {
		return fmt.Errorf("user_data.aleid is required when event_source_url contains aleid")
	}
	if event.MeasurementPartnerData != nil {
		if policy != PolicyStandard {
			return fmt.Errorf("measurement_partner_data is only allowed by the STANDARD policy")
		}
		if err := validateMeasurement(*event.MeasurementPartnerData); err != nil {
			return err
		}
	}
	if policy == PolicyRestrictedLeadGen && event.Name != EventPageView && event.Name != EventGenerateLead {
		return fmt.Errorf("name is not allowed by the RESTRICTED_LEAD_GEN policy")
	}
	if policy == PolicyLeadGen && event.Name != EventPageView && event.Name != EventGenerateLead && event.Name != EventAppOpen {
		return fmt.Errorf("name is not allowed by the LEAD_GEN policy")
	}
	return validateEventData(event)
}

func validateSourceURL(value string, originOnly bool) (*url.URL, error) {
	if !validOpaque(value, 8192) {
		return nil, fmt.Errorf("event_source_url is invalid")
	}
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" ||
		parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" {
		return nil, fmt.Errorf("event_source_url must be an absolute HTTP(S) URL without credentials or fragment")
	}
	if originOnly && ((parsed.Path != "" && parsed.Path != "/") || parsed.RawPath != "" || parsed.RawQuery != "") {
		return nil, fmt.Errorf("event_source_url must contain only the origin under RESTRICTED_LEAD_GEN")
	}
	return parsed, nil
}

func validateUserData(policy AccountPolicy, value UserData) error {
	if net.ParseIP(value.ClientIPAddress) == nil {
		return fmt.Errorf("user_data.client_ip_address must be an IPv4 or IPv6 address")
	}
	if !validOpaque(value.ClientUserAgent, 8192) {
		return fmt.Errorf("user_data.client_user_agent is required")
	}
	if value.ESI != SourceApp && value.ESI != SourceWeb {
		return fmt.Errorf("user_data.esi must be app or web")
	}
	for _, item := range []string{value.Alart, value.ALEID, value.Axwrt, value.ClientID, value.Email, value.Phone, value.UserID, value.IFA, value.IDFV, value.SID} {
		if !validOptionalOpaque(item, 4096) {
			return fmt.Errorf("user_data contains an invalid identifier")
		}
	}
	if value.CountryCode != "" && !countryPattern.MatchString(value.CountryCode) {
		return fmt.Errorf("user_data.country_code must be an uppercase ISO 3166-1 alpha-2 code")
	}
	if value.Zip != "" && !validOpaque(value.Zip, 64) {
		return fmt.Errorf("user_data.zip is invalid")
	}
	if value.OS != "" && value.OS != OSIOS && value.OS != OSAndroid && value.OS != OSDesktop {
		return fmt.Errorf("user_data.os is invalid")
	}
	if policy == PolicyRestrictedLeadGen {
		if value.Axwrt != "" || value.Email != "" || value.Phone != "" || value.CountryCode != "" || value.IFA != "" ||
			value.IDFV != "" || value.OS != "" || value.SID != "" || value.Zip != "" {
			return fmt.Errorf("user_data contains a field forbidden by RESTRICTED_LEAD_GEN")
		}
		if value.UserID != "" && !numericPattern.MatchString(value.UserID) {
			return fmt.Errorf("user_data.user_id must be numeric under RESTRICTED_LEAD_GEN")
		}
		if value.ClientID == "" && value.Alart == "" && value.UserID == "" {
			return fmt.Errorf("user_data requires client_id, alart, or user_id under RESTRICTED_LEAD_GEN")
		}
		return nil
	}
	if value.ClientID == "" && value.Axwrt == "" && value.Alart == "" && value.UserID == "" && value.Email == "" && value.Phone == "" {
		return fmt.Errorf("user_data requires at least one supported user identifier")
	}
	return nil
}

func validateMeasurement(value MeasurementPartnerData) error {
	if value.AccountingMode != AccountingCash && value.AccountingMode != AccountingAccrual {
		return fmt.Errorf("measurement_partner_data.accounting_mode is invalid")
	}
	if !validAttributionModel(value.AttributionModel) {
		return fmt.Errorf("measurement_partner_data.attribution_model is invalid")
	}
	if !validDecimal(value.AttributionShare, true) || !decimalAtMost(value.AttributionShare, big.NewRat(1, 1)) {
		return fmt.Errorf("measurement_partner_data.attribution_share must be between 0 and 1")
	}
	if value.AttributionLookbackWindowHours != nil && (*value.AttributionLookbackWindowHours < 0 || *value.AttributionLookbackWindowHours > 1_000_000) {
		return fmt.Errorf("measurement_partner_data.attribution_lookback_window_hours is invalid")
	}
	if !validOptionalOpaque(value.CampaignID, 4096) || !validOptionalOpaque(value.CreativeSetID, 4096) {
		return fmt.Errorf("measurement_partner_data contains an invalid identifier")
	}
	for _, timestamp := range []*int64{value.FirstPurchaseTimestamp, value.FirstVisitTimestamp, value.LastPurchaseTimestamp, value.LastVisitTimestamp} {
		if timestamp != nil && *timestamp < 0 {
			return fmt.Errorf("measurement_partner_data contains a negative timestamp")
		}
	}
	return nil
}

func validAttributionModel(value AttributionModel) bool {
	switch value {
	case AttributionLastClick, AttributionFirstClick, AttributionLinear, AttributionTimeDecay,
		AttributionCustomMultiTouch, AttributionLastNonDirectTouch, AttributionClicksViews, AttributionAnyClick:
		return true
	default:
		return false
	}
}

func validateEventData(event ServerEvent) error {
	switch event.Name {
	case EventPageView:
		if event.Data != nil {
			return fmt.Errorf("data must be null for page_view")
		}
	case EventAppOpen:
		if event.UserData.ESI != SourceApp || event.Data != nil {
			return fmt.Errorf("app_open requires user_data.esi app and null data")
		}
	case EventViewItem:
		value, ok := event.Data.(*ViewItemData)
		if !ok || value == nil || !validCommerce(value.Items, value.Currency, value.Value, false) {
			return fmt.Errorf("data is invalid for view_item")
		}
	case EventAddToCart:
		value, ok := event.Data.(*AddToCartData)
		if !ok || value == nil || !validCommerce(value.Items, value.Currency, value.Value, true) {
			return fmt.Errorf("data is invalid for add_to_cart")
		}
	case EventBeginCheckout:
		value, ok := event.Data.(*BeginCheckoutData)
		if !ok || value == nil || !validCurrency(value.Currency) || !validDecimal(value.Value, true) || !validItems(value.Items, false) {
			return fmt.Errorf("data is invalid for begin_checkout")
		}
	case EventPurchase:
		value, ok := event.Data.(*PurchaseData)
		if !ok || value == nil || !validCurrency(value.Currency) || !validItems(value.Items, true) ||
			!validDecimal(value.Shipping, true) || !validDecimal(value.Tax, true) || !validDecimal(value.Value, true) ||
			!validOpaque(value.TransactionID, 4096) {
			return fmt.Errorf("data is invalid for purchase")
		}
	case EventAddPaymentInfo:
		value, ok := event.Data.(*AddPaymentInfoData)
		if !ok || value == nil || !validOptionalCommerce(value.Items, value.Currency, value.Value) || !validPaymentType(value.PaymentType) {
			return fmt.Errorf("data is invalid for add_payment_info")
		}
	case EventRemoveFromCart:
		value, ok := event.Data.(*RemoveFromCartData)
		if !ok || value == nil || !validCommerce(value.Items, value.Currency, value.Value, true) {
			return fmt.Errorf("data is invalid for remove_from_cart")
		}
	case EventSearch:
		value, ok := event.Data.(*SearchData)
		if !ok || value == nil || !validOpaque(value.SearchTerm, 4096) || len(value.Results) > 0 && !validItems(value.Results, false) {
			return fmt.Errorf("data is invalid for search")
		}
	case EventViewCart:
		value, ok := event.Data.(*ViewCartData)
		if !ok || value == nil || !validCommerce(value.Items, value.Currency, value.Value, false) {
			return fmt.Errorf("data is invalid for view_cart")
		}
	case EventGenerateLead:
		value, ok := event.Data.(*GenerateLeadData)
		if !ok || value == nil || !validCurrency(value.Currency) || !validDecimal(value.Value, true) {
			return fmt.Errorf("data is invalid for generate_lead")
		}
	case EventLogin:
		if value, ok := event.Data.(*LoginData); !ok || value == nil {
			return fmt.Errorf("data is invalid for login")
		}
	case EventSignUp:
		value, ok := event.Data.(*SignUpData)
		if !ok || value == nil || !validOptionalOpaque(value.Method, 256) {
			return fmt.Errorf("data is invalid for sign_up")
		}
	case EventSubscribe:
		value, ok := event.Data.(*SubscribeData)
		if !ok || value == nil || !validOptionalMoney(value.Currency, value.Value) {
			return fmt.Errorf("data is invalid for subscribe")
		}
	default:
		return fmt.Errorf("name is unsupported")
	}
	return nil
}

func validCommerce(items []Item, currency string, value Decimal, requireQuantity bool) bool {
	if len(items) == 0 || !validItems(items, requireQuantity) {
		return false
	}
	return validOptionalMoney(currency, value)
}

func validOptionalCommerce(items []Item, currency string, value Decimal) bool {
	return len(items) <= maximumItemsPerEvent && (len(items) == 0 || validItems(items, false)) &&
		validOptionalMoney(currency, value)
}

func validOptionalMoney(currency string, value Decimal) bool {
	if currency != "" && !validCurrency(currency) || value != "" && !validDecimal(value, true) {
		return false
	}
	return value == "" || currency != ""
}

func validCurrency(value string) bool { return currencyPattern.MatchString(value) }

func validPaymentType(value PaymentType) bool {
	return value == "" || value == PaymentCreditCard || value == PaymentDeferred || value == PaymentRedeemable ||
		value == PaymentOnDelivery || value == PaymentWallet || value == PaymentOther
}

func validItems(items []Item, requireQuantity bool) bool {
	if len(items) == 0 || len(items) > maximumItemsPerEvent {
		return false
	}
	for _, item := range items {
		if !validItem(item, requireQuantity) {
			return false
		}
	}
	return true
}

func validItem(item Item, requireQuantity bool) bool {
	if !validOpaque(item.ItemID, 4096) || item.ItemCategoryID < 0 ||
		!validOptionalOpaque(item.ItemName, 4096) || !validOptionalOpaque(item.ItemVariantID, 4096) ||
		!validOptionalOpaque(item.Affiliation, 4096) || !validOptionalOpaque(item.ItemBrand, 4096) ||
		!validOptionalOpaque(item.ItemCategory, 4096) || !validOptionalOpaque(item.ItemCategory2, 4096) {
		return false
	}
	if item.ImageURL != "" {
		parsed, err := validateSourceURL(item.ImageURL, false)
		if err != nil || !imageExtensionPattern.MatchString(path.Ext(parsed.Path)) {
			return false
		}
	}
	if item.Price != "" && !validDecimal(item.Price, true) || item.Discount != "" && !validDecimal(item.Discount, true) {
		return false
	}
	if requireQuantity {
		return validDecimal(item.Quantity, false)
	}
	return item.Quantity == "" || validDecimal(item.Quantity, false)
}

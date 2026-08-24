package conversions

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var (
	decimalPattern  = regexp.MustCompile(`^-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?$`)
	digitsPattern   = regexp.MustCompile(`^[0-9]+$`)
	propertyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,63}$`)
)

const maximumValuesPerField = 1000

var supportedCurrencies = map[string]struct{}{
	"AED": {}, "ALL": {}, "AUD": {}, "BGN": {}, "BHD": {}, "BRL": {}, "CAD": {}, "CHF": {}, "CLP": {}, "CNY": {},
	"COP": {}, "CZK": {}, "DKK": {}, "DZD": {}, "EGP": {}, "EUR": {}, "GBP": {}, "GHS": {}, "GIP": {}, "HKD": {},
	"HRK": {}, "HUF": {}, "IDR": {}, "ILS": {}, "INR": {}, "IQD": {}, "ISK": {}, "JOD": {}, "JPY": {}, "KES": {},
	"KRW": {}, "KWD": {}, "KZT": {}, "LBP": {}, "MAD": {}, "MXN": {}, "MYR": {}, "NGN": {}, "NOK": {}, "NZD": {},
	"OMR": {}, "PEN": {}, "PHP": {}, "PKR": {}, "PLN": {}, "QAR": {}, "RON": {}, "RUB": {}, "SAR": {}, "SEK": {},
	"SGD": {}, "THB": {}, "TRY": {}, "TWD": {}, "TZS": {}, "UAH": {}, "USD": {}, "VND": {}, "XOF": {}, "ZAR": {},
}

type wireEnvelope struct {
	Data []wireEvent `json:"data"`
}

type wireEvent struct {
	EventName             EventName              `json:"event_name"`
	EventTime             int64                  `json:"event_time"`
	EventSourceURL        string                 `json:"event_source_url,omitempty"`
	EventID               string                 `json:"event_id,omitempty"`
	ActionSource          ActionSource           `json:"action_source"`
	Integration           string                 `json:"integration,omitempty"`
	DataProcessingOptions []DataProcessingOption `json:"data_processing_options,omitempty"`
	TestEventCode         string                 `json:"test_event_code,omitempty"`
	UserData              wireUserData           `json:"user_data"`
	CustomData            map[string]any         `json:"custom_data,omitempty"`
	AppData               *wireAppData           `json:"app_data,omitempty"`
}

type wireContent struct {
	ID               string           `json:"id"`
	Quantity         int64            `json:"quantity,omitempty"`
	ItemPrice        Decimal          `json:"item_price,omitempty"`
	Brand            string           `json:"brand,omitempty"`
	DeliveryCategory DeliveryCategory `json:"delivery_category,omitempty"`
}

type wireAppData struct {
	AdvertiserTrackingEnabled int    `json:"advertiser_tracking_enabled"`
	AppID                     string `json:"app_id"`
	ExtInfo                   []any  `json:"extinfo"`
}

func normalizeBatch(assetType AssetType, events []ServerEvent, now time.Time) (wireEnvelope, error) {
	if len(events) == 0 || len(events) > MaximumBatchSize {
		return wireEnvelope{}, fmt.Errorf("events must contain between 1 and %d entries", MaximumBatchSize)
	}
	output := wireEnvelope{Data: make([]wireEvent, len(events))}
	for index := range events {
		event, err := normalizeEvent(assetType, events[index], now)
		if err != nil {
			return wireEnvelope{}, fmt.Errorf("events[%d]: %w", index, err)
		}
		output.Data[index] = event
	}
	return output, nil
}

func normalizeEvent(assetType AssetType, input ServerEvent, now time.Time) (wireEvent, error) {
	if !validEventName(input.EventName) {
		return wireEvent{}, fmt.Errorf("event_name is invalid")
	}
	if !validActionForAsset(input.ActionSource, assetType) {
		return wireEvent{}, fmt.Errorf("action_source is incompatible with the configured asset_type")
	}
	when, err := parseEventTime(input.EventTime)
	if err != nil {
		return wireEvent{}, fmt.Errorf("event_time must be Unix seconds or milliseconds")
	}
	if (input.ActionSource == ActionSourceWeb || input.ActionSource == ActionSourceMobileApp) && when.Before(now.Add(-7*24*time.Hour)) {
		return wireEvent{}, fmt.Errorf("event_time cannot be more than 7 days old for Web or Mobile App events")
	}
	if !validOptionalOpaque(input.EventID, 1024) || !validOptionalOpaque(input.Integration, 1024) ||
		!validOptionalOpaque(input.TestEventCode, 1024) {
		return wireEvent{}, fmt.Errorf("event_id, integration, or test_event_code is invalid")
	}
	if input.ActionSource == ActionSourceWeb || input.ActionSource == ActionSourceMobileApp {
		if !validPublicURL(input.EventSourceURL) {
			return wireEvent{}, fmt.Errorf("event_source_url is required for Web and Mobile App events")
		}
	} else if input.EventSourceURL != "" && !validPublicURL(input.EventSourceURL) {
		return wireEvent{}, fmt.Errorf("event_source_url is invalid")
	}
	if input.ActionSource == ActionSourceMobileApp {
		if input.AppData == nil {
			return wireEvent{}, fmt.Errorf("app_data is required for Mobile App events")
		}
	} else if input.AppData != nil {
		return wireEvent{}, fmt.Errorf("app_data requires action_source MOBILE_APP")
	}
	if err := validateDataProcessing(input.ActionSource, input.DataProcessingOptions); err != nil {
		return wireEvent{}, err
	}
	userData, err := normalizeUserData(input.UserData, input.ActionSource)
	if err != nil {
		return wireEvent{}, err
	}
	var customData map[string]any
	if input.CustomData != nil {
		customData, err = normalizeCustomData(*input.CustomData)
		if err != nil {
			return wireEvent{}, err
		}
	}
	if input.EventName == EventPurchase && (input.CustomData == nil || input.CustomData.Value == "" || input.CustomData.Currency == "") {
		return wireEvent{}, fmt.Errorf("custom_data.value and currency are required for PURCHASE")
	}
	var appData *wireAppData
	if input.AppData != nil {
		normalized, normalizeErr := normalizeAppData(*input.AppData)
		if normalizeErr != nil {
			return wireEvent{}, normalizeErr
		}
		appData = &normalized
	}
	return wireEvent{
		EventName: input.EventName, EventTime: input.EventTime, EventSourceURL: input.EventSourceURL,
		EventID: input.EventID, ActionSource: input.ActionSource, Integration: input.Integration,
		DataProcessingOptions: append([]DataProcessingOption(nil), input.DataProcessingOptions...),
		TestEventCode:         input.TestEventCode, UserData: userData, CustomData: customData, AppData: appData,
	}, nil
}

func parseEventTime(value int64) (time.Time, error) {
	if value <= 0 {
		return time.Time{}, fmt.Errorf("invalid timestamp")
	}
	var result time.Time
	if value >= 1_000_000_000_000 {
		result = time.UnixMilli(value)
	} else {
		result = time.Unix(value, 0)
	}
	if result.Before(time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)) || result.Year() > 9999 {
		return time.Time{}, fmt.Errorf("invalid timestamp")
	}
	return result, nil
}

func validateDataProcessing(action ActionSource, options []DataProcessingOption) error {
	if len(options) > 0 && action != ActionSourceWeb {
		return fmt.Errorf("data_processing_options are supported only for WEB events")
	}
	seen := make(map[DataProcessingOption]struct{}, len(options))
	for _, option := range options {
		if option != DataProcessingLMU && option != DataProcessingDelete {
			return fmt.Errorf("data_processing_options contains an unsupported value")
		}
		if _, found := seen[option]; found {
			return fmt.Errorf("data_processing_options contains a duplicate")
		}
		seen[option] = struct{}{}
	}
	return nil
}

func normalizeCustomData(input CustomData) (map[string]any, error) {
	output := make(map[string]any)
	for _, list := range []struct {
		path   string
		key    string
		values []string
	}{
		{"custom_data.content_categories", "content_category", input.ContentCategories},
		{"custom_data.content_ids", "content_ids", input.ContentIDs},
	} {
		if len(list.values) > 1000 {
			return nil, fmt.Errorf("%s has too many entries", list.path)
		}
		if len(list.values) > 0 {
			copyValues := make([]string, len(list.values))
			for index, value := range list.values {
				if !validOpaque(value, 4096) {
					return nil, fmt.Errorf("%s[%d] is invalid", list.path, index)
				}
				copyValues[index] = value
			}
			output[list.key] = copyValues
		}
	}
	for _, field := range []struct {
		path  string
		key   string
		value string
	}{
		{"custom_data.content_name", "content_name", input.ContentName},
		{"custom_data.order_id", "order_id", input.OrderID},
		{"custom_data.search_string", "search_string", input.SearchString},
		{"custom_data.status", "status", input.Status},
		{"custom_data.event_tag", "event_tag", input.EventTag},
	} {
		if !validOptionalOpaque(field.value, 4096) {
			return nil, fmt.Errorf("%s is invalid", field.path)
		}
		if field.value != "" {
			output[field.key] = field.value
		}
	}
	if input.ContentType != "" {
		if input.ContentType != ContentTypeProduct && input.ContentType != ContentTypeProductGroup {
			return nil, fmt.Errorf("custom_data.content_type is invalid")
		}
		output["content_type"] = input.ContentType
	}
	if input.Currency != "" {
		currency := strings.ToUpper(strings.TrimSpace(input.Currency))
		if _, found := supportedCurrencies[currency]; !found {
			return nil, fmt.Errorf("custom_data.currency is unsupported")
		}
		output["currency"] = currency
	}
	if (input.Value == "") != (input.Currency == "") {
		return nil, fmt.Errorf("custom_data.value and currency must be supplied together")
	}
	for _, field := range []struct {
		path  string
		key   string
		value Decimal
	}{
		{"custom_data.value", "value", input.Value},
		{"custom_data.predicted_ltv", "predicted_ltv", input.PredictedLTV},
	} {
		if field.value != "" {
			if !validDecimal(field.value) {
				return nil, fmt.Errorf("%s is invalid", field.path)
			}
			output[field.key] = field.value
		}
	}
	if input.NumItems != "" {
		if len(input.NumItems) > 20 || !digitsPattern.MatchString(input.NumItems) {
			return nil, fmt.Errorf("custom_data.num_items is invalid")
		}
		output["num_items"] = input.NumItems
	}
	if len(input.Contents) > 1000 {
		return nil, fmt.Errorf("custom_data.contents has too many entries")
	}
	if len(input.Contents) > 0 {
		contents := make([]wireContent, len(input.Contents))
		for index, content := range input.Contents {
			normalized, err := normalizeContent(content)
			if err != nil {
				return nil, fmt.Errorf("custom_data.contents[%d]: %w", index, err)
			}
			contents[index] = normalized
		}
		output["contents"] = contents
	}
	customFields, err := normalizeCustomFields(input.CustomFields)
	if err != nil {
		return nil, err
	}
	if len(customFields) > 0 {
		output["custom_fields"] = customFields
	}
	return output, nil
}

func normalizeContent(input Content) (wireContent, error) {
	if !validOpaque(input.ID, 4096) || input.Quantity < 0 || input.ItemPrice != "" && !validDecimal(input.ItemPrice) ||
		!validOptionalOpaque(input.Brand, 4096) {
		return wireContent{}, fmt.Errorf("id, quantity, item_price, or brand is invalid")
	}
	if input.DeliveryCategory != "" && input.DeliveryCategory != DeliveryCategoryInStore &&
		input.DeliveryCategory != DeliveryCategoryCurbside && input.DeliveryCategory != DeliveryCategoryHomeDelivery {
		return wireContent{}, fmt.Errorf("delivery_category is invalid")
	}
	return wireContent{
		ID: input.ID, Quantity: input.Quantity, ItemPrice: input.ItemPrice,
		Brand: input.Brand, DeliveryCategory: input.DeliveryCategory,
	}, nil
}

func normalizeCustomFields(input CustomFields) (map[string]any, error) {
	if len(input.Strings)+len(input.Numbers)+len(input.Booleans) > maximumValuesPerField {
		return nil, fmt.Errorf("custom_data.custom_fields has too many entries")
	}
	output := make(map[string]any)
	seen := make(map[string]string)
	for _, group := range []struct {
		kind    string
		strings map[string]string
		numbers map[string]Decimal
		bools   map[string]bool
	}{
		{kind: "string", strings: input.Strings},
		{kind: "number", numbers: input.Numbers},
		{kind: "boolean", bools: input.Booleans},
	} {
		for key, value := range group.strings {
			if err := validateCustomFieldKey(key, group.kind, seen); err != nil {
				return nil, err
			}
			if !validOpaque(value, 4096) {
				return nil, fmt.Errorf("custom_data.custom_fields contains an invalid string value")
			}
			output[key] = value
		}
		for key, value := range group.numbers {
			if err := validateCustomFieldKey(key, group.kind, seen); err != nil {
				return nil, err
			}
			if !validDecimal(value) {
				return nil, fmt.Errorf("custom_data.custom_fields contains an invalid number value")
			}
			output[key] = value
		}
		for key, value := range group.bools {
			if err := validateCustomFieldKey(key, group.kind, seen); err != nil {
				return nil, err
			}
			output[key] = value
		}
	}
	return output, nil
}

func validateCustomFieldKey(key, kind string, seen map[string]string) error {
	if !propertyPattern.MatchString(key) {
		return fmt.Errorf("custom_data.custom_fields contains an invalid key")
	}
	if previous, found := seen[key]; found {
		return fmt.Errorf("custom_data.custom_fields key is duplicated across %s and %s values", previous, kind)
	}
	seen[key] = kind
	return nil
}

func normalizeAppData(input AppData) (wireAppData, error) {
	if input.AdvertiserTrackingEnabled == nil || !validOpaque(input.AppID, 1024) {
		return wireAppData{}, fmt.Errorf("app_data requires advertiser_tracking_enabled and app_id")
	}
	extInfo, err := normalizeExtendedDeviceInfo(input.ExtendedDeviceInfo)
	if err != nil {
		return wireAppData{}, err
	}
	trackingEnabled := 0
	if *input.AdvertiserTrackingEnabled {
		trackingEnabled = 1
	}
	return wireAppData{AdvertiserTrackingEnabled: trackingEnabled, AppID: input.AppID, ExtInfo: extInfo}, nil
}

func normalizeExtendedDeviceInfo(input ExtendedDeviceInfo) ([]any, error) {
	if input.Version != "a2" && input.Version != "i2" {
		return nil, fmt.Errorf("app_data.extended_device_info.version must be a2 or i2")
	}
	if !validOpaque(input.OSVersion, 1024) {
		return nil, fmt.Errorf("app_data.extended_device_info.os_version is required")
	}
	for _, field := range []struct {
		path  string
		value string
	}{
		{"app_package_name", input.AppPackageName}, {"short_version", input.ShortVersion},
		{"long_version", input.LongVersion}, {"device_model_name", input.DeviceModelName},
		{"locale", input.Locale}, {"timezone_abbreviation", input.TimezoneAbbreviation},
		{"carrier", input.Carrier}, {"screen_density", input.ScreenDensity},
		{"device_time_zone", input.DeviceTimeZone},
	} {
		if !validOptionalOpaque(field.value, 1024) {
			return nil, fmt.Errorf("app_data.extended_device_info.%s is invalid", field.path)
		}
	}
	if input.ScreenWidth < 0 || input.ScreenHeight < 0 || input.CPUCoreCount < 0 ||
		input.ExternalStorageGB < 0 || input.FreeStorageGB < 0 {
		return nil, fmt.Errorf("app_data.extended_device_info contains a negative numeric field")
	}
	return []any{
		input.Version, input.AppPackageName, input.ShortVersion, input.LongVersion,
		input.OSVersion, input.DeviceModelName, input.Locale, input.TimezoneAbbreviation,
		input.Carrier, input.ScreenWidth, input.ScreenHeight, input.ScreenDensity,
		input.CPUCoreCount, input.ExternalStorageGB, input.FreeStorageGB, input.DeviceTimeZone,
	}, nil
}

func validEventName(value EventName) bool {
	switch value {
	case EventPurchase, EventSave, EventStartCheckout, EventAddCart, EventViewContent, EventAddBilling,
		EventSignUp, EventSearch, EventPageView, EventSubscribe, EventAdClick, EventAdView,
		EventCompleteTutorial, EventLevelComplete, EventInvite, EventLogin, EventShare, EventReserve,
		EventAchievementUnlocked, EventAddToWishlist, EventSpentCredits, EventRate, EventStartTrial,
		EventListView, EventAppInstall, EventAppOpen, EventCustom1, EventCustom2, EventCustom3, EventCustom4, EventCustom5:
		return true
	default:
		return false
	}
}

func validActionForAsset(action ActionSource, assetType AssetType) bool {
	if assetType == AssetTypeSnapApp {
		return action == ActionSourceMobileApp
	}
	return assetType == AssetTypePixel && (action == ActionSourceWeb || isOfflineAction(action))
}

func isOfflineAction(action ActionSource) bool {
	switch action {
	case ActionSourceOffline, ActionSourcePhysicalStore, ActionSourcePhoneCall, ActionSourcePhone,
		ActionSourceEmail, ActionSourceChat, ActionSourceSystemGenerated:
		return true
	default:
		return false
	}
}

func validDecimal(value Decimal) bool {
	return len(value) > 0 && len(value) <= 128 && decimalPattern.MatchString(string(value))
}

func validPublicURL(value string) bool {
	if len(value) > 8192 || !utf8.ValidString(value) || strings.ContainsFunc(value, unicode.IsControl) {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.Hostname() != "" && parsed.User == nil && parsed.Fragment == ""
}

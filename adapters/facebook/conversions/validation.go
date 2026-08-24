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
	currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)
	propertyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,63}$`)
)

var standardCustomDataFields = map[string]struct{}{
	"value": {}, "net_revenue": {}, "currency": {}, "content_name": {}, "content_category": {},
	"content_ids": {}, "contents": {}, "content_type": {}, "order_id": {}, "predicted_ltv": {},
	"num_items": {}, "search_string": {}, "status": {}, "item_number": {}, "delivery_category": {},
}

type wireEventEnvelope struct {
	Data          []wireServerEvent `json:"data"`
	PartnerAgent  string            `json:"partner_agent,omitempty"`
	TestEventCode string            `json:"test_event_code,omitempty"`
	NamespaceID   string            `json:"namespace_id,omitempty"`
	UploadID      string            `json:"upload_id,omitempty"`
	UploadTag     string            `json:"upload_tag,omitempty"`
	UploadSource  string            `json:"upload_source,omitempty"`
}

type wireServerEvent struct {
	EventName                    string                 `json:"event_name"`
	EventTime                    int64                  `json:"event_time"`
	EventSourceURL               string                 `json:"event_source_url,omitempty"`
	EventID                      string                 `json:"event_id,omitempty"`
	ActionSource                 ActionSource           `json:"action_source"`
	OptOut                       *bool                  `json:"opt_out,omitempty"`
	UserData                     wireUserData           `json:"user_data"`
	CustomData                   map[string]any         `json:"custom_data,omitempty"`
	AppData                      *wireAppData           `json:"app_data,omitempty"`
	DataProcessingOptions        []DataProcessingOption `json:"data_processing_options,omitempty"`
	DataProcessingOptionsCountry *int                   `json:"data_processing_options_country,omitempty"`
	DataProcessingOptionsState   *int                   `json:"data_processing_options_state,omitempty"`
	AdvertiserTrackingEnabled    *bool                  `json:"advertiser_tracking_enabled,omitempty"`
	MessagingChannel             MessagingChannel       `json:"messaging_channel,omitempty"`
	ReferrerURL                  string                 `json:"referrer_url,omitempty"`
}

type wireContent struct {
	ID               string           `json:"id,omitempty"`
	Quantity         int64            `json:"quantity,omitempty"`
	ItemPrice        Decimal          `json:"item_price,omitempty"`
	Title            string           `json:"title,omitempty"`
	Description      string           `json:"description,omitempty"`
	Category         string           `json:"category,omitempty"`
	Brand            string           `json:"brand,omitempty"`
	DeliveryCategory DeliveryCategory `json:"delivery_category,omitempty"`
}

type wireAppData struct {
	ApplicationTrackingEnabled *bool  `json:"application_tracking_enabled,omitempty"`
	AdvertiserTrackingEnabled  *bool  `json:"advertiser_tracking_enabled,omitempty"`
	CampaignIDs                string `json:"campaign_ids,omitempty"`
	ConsiderViews              *bool  `json:"consider_views,omitempty"`
	ExtInfo                    []any  `json:"extinfo"`
	IncludeDwellData           *bool  `json:"include_dwell_data,omitempty"`
	IncludeVideoData           *bool  `json:"include_video_data,omitempty"`
	InstallReferrer            string `json:"install_referrer,omitempty"`
	InstallerPackage           string `json:"installer_package,omitempty"`
	ReceiptData                string `json:"receipt_data,omitempty"`
	URLSchemes                 string `json:"url_schemes,omitempty"`
	WindowsAttributionID       string `json:"windows_attribution_id,omitempty"`
}

func normalizeRequest(input SubmitEventsRequest, now time.Time) (wireEventEnvelope, error) {
	if len(input.Events) == 0 {
		return wireEventEnvelope{}, fmt.Errorf("events must not be empty")
	}
	for _, field := range []struct {
		path    string
		value   string
		maximum int
	}{
		{"partner_agent", input.PartnerAgent, 1024}, {"test_event_code", input.TestEventCode, 1024},
		{"namespace_id", input.NamespaceID, 1024}, {"upload_id", input.UploadID, 1024},
		{"upload_tag", input.UploadTag, 1024}, {"upload_source", input.UploadSource, 1024},
	} {
		if !validOptionalOpaque(field.value, field.maximum) {
			return wireEventEnvelope{}, fmt.Errorf("%s is invalid", field.path)
		}
	}
	output := wireEventEnvelope{
		Data: make([]wireServerEvent, len(input.Events)), PartnerAgent: input.PartnerAgent,
		TestEventCode: input.TestEventCode, NamespaceID: input.NamespaceID,
		UploadID: input.UploadID, UploadTag: input.UploadTag, UploadSource: input.UploadSource,
	}
	for index := range input.Events {
		event, err := normalizeEvent(input.Events[index], now)
		if err != nil {
			return wireEventEnvelope{}, fmt.Errorf("events[%d]: %w", index, err)
		}
		output.Data[index] = event
	}
	return output, nil
}

func normalizeEvent(input ServerEvent, now time.Time) (wireServerEvent, error) {
	if !validOpaque(input.EventName, 512) {
		return wireServerEvent{}, fmt.Errorf("event_name is invalid")
	}
	if input.EventTime < time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC).Unix() || input.EventTime > now.Add(5*time.Minute).Unix() {
		return wireServerEvent{}, fmt.Errorf("event_time must be Unix seconds and cannot be in the future")
	}
	if !validActionSource(input.ActionSource) {
		return wireServerEvent{}, fmt.Errorf("action_source is invalid")
	}
	if !validOptionalOpaque(input.EventID, 1024) {
		return wireServerEvent{}, fmt.Errorf("event_id is invalid")
	}
	if input.ActionSource == ActionSourceWebsite && input.EventSourceURL == "" {
		return wireServerEvent{}, fmt.Errorf("event_source_url is required for website events")
	}
	if input.EventSourceURL != "" && !validPublicURL(input.EventSourceURL) {
		return wireServerEvent{}, fmt.Errorf("event_source_url must be an absolute HTTP(S) URL without credentials or fragment")
	}
	if input.ReferrerURL != "" && !validPublicURL(input.ReferrerURL) {
		return wireServerEvent{}, fmt.Errorf("referrer_url must be an absolute HTTP(S) URL without credentials or fragment")
	}
	if input.ActionSource == ActionSourceApp {
		if input.AppData == nil {
			return wireServerEvent{}, fmt.Errorf("app_data is required for app events")
		}
	} else if input.AppData != nil || input.AdvertiserTrackingEnabled != nil {
		return wireServerEvent{}, fmt.Errorf("app_data and advertiser_tracking_enabled require action_source app")
	}
	if input.ActionSource == ActionSourceBusinessMessaging {
		if !validMessagingChannel(input.MessagingChannel) {
			return wireServerEvent{}, fmt.Errorf("messaging_channel is required for business_messaging events")
		}
	} else if input.MessagingChannel != "" {
		return wireServerEvent{}, fmt.Errorf("messaging_channel requires action_source business_messaging")
	}
	if err := validateDataProcessing(input); err != nil {
		return wireServerEvent{}, err
	}
	userData, err := normalizeUserData(input.UserData)
	if err != nil {
		return wireServerEvent{}, err
	}
	var customData map[string]any
	if input.CustomData != nil {
		customData, err = normalizeCustomData(*input.CustomData)
		if err != nil {
			return wireServerEvent{}, err
		}
	}
	var appData *wireAppData
	if input.AppData != nil {
		normalized, normalizeErr := normalizeAppData(*input.AppData)
		if normalizeErr != nil {
			return wireServerEvent{}, normalizeErr
		}
		appData = &normalized
	}
	return wireServerEvent{
		EventName: input.EventName, EventTime: input.EventTime, EventSourceURL: input.EventSourceURL,
		EventID: input.EventID, ActionSource: input.ActionSource, OptOut: input.OptOut,
		UserData: userData, CustomData: customData, AppData: appData,
		DataProcessingOptions:        append([]DataProcessingOption(nil), input.DataProcessingOptions...),
		DataProcessingOptionsCountry: input.DataProcessingOptionsCountry,
		DataProcessingOptionsState:   input.DataProcessingOptionsState,
		AdvertiserTrackingEnabled:    input.AdvertiserTrackingEnabled,
		MessagingChannel:             input.MessagingChannel, ReferrerURL: input.ReferrerURL,
	}, nil
}

func validateDataProcessing(input ServerEvent) error {
	seenLDU := false
	for _, option := range input.DataProcessingOptions {
		if option != DataProcessingOptionLDU || seenLDU {
			return fmt.Errorf("data_processing_options is invalid")
		}
		seenLDU = true
	}
	if input.DataProcessingOptionsCountry != nil && *input.DataProcessingOptionsCountry < 0 ||
		input.DataProcessingOptionsState != nil && *input.DataProcessingOptionsState < 0 {
		return fmt.Errorf("data processing country and state must be non-negative")
	}
	if !seenLDU && (input.DataProcessingOptionsCountry != nil || input.DataProcessingOptionsState != nil) {
		return fmt.Errorf("data processing country and state require LDU")
	}
	return nil
}

func normalizeCustomData(input CustomData) (map[string]any, error) {
	output := make(map[string]any)
	for _, field := range []struct {
		path  string
		value Decimal
		key   string
	}{
		{"custom_data.value", input.Value, "value"},
		{"custom_data.net_revenue", input.NetRevenue, "net_revenue"},
		{"custom_data.predicted_ltv", input.PredictedLTV, "predicted_ltv"},
	} {
		if field.value != "" {
			if !validDecimal(field.value) {
				return nil, fmt.Errorf("%s is invalid", field.path)
			}
			output[field.key] = field.value
		}
	}
	if input.Currency != "" {
		currency := strings.ToUpper(strings.TrimSpace(input.Currency))
		if !currencyPattern.MatchString(currency) {
			return nil, fmt.Errorf("custom_data.currency is invalid")
		}
		output["currency"] = currency
	}
	for _, field := range []struct {
		path  string
		value string
		key   string
	}{
		{"custom_data.content_name", input.ContentName, "content_name"},
		{"custom_data.content_category", input.ContentCategory, "content_category"},
		{"custom_data.content_type", input.ContentType, "content_type"},
		{"custom_data.order_id", input.OrderID, "order_id"},
		{"custom_data.search_string", input.SearchString, "search_string"},
		{"custom_data.status", input.Status, "status"},
		{"custom_data.item_number", input.ItemNumber, "item_number"},
	} {
		if !validOptionalOpaque(field.value, 4096) {
			return nil, fmt.Errorf("%s is invalid", field.path)
		}
		if field.value != "" {
			output[field.key] = field.value
		}
	}
	if len(input.ContentIDs) > 1000 {
		return nil, fmt.Errorf("custom_data.content_ids has too many entries")
	}
	if len(input.ContentIDs) > 0 {
		ids := make([]string, len(input.ContentIDs))
		for index, id := range input.ContentIDs {
			if !validOpaque(id, 4096) {
				return nil, fmt.Errorf("custom_data.content_ids[%d] is invalid", index)
			}
			ids[index] = id
		}
		output["content_ids"] = ids
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
	if input.NumItems != nil {
		if *input.NumItems < 0 {
			return nil, fmt.Errorf("custom_data.num_items is invalid")
		}
		output["num_items"] = *input.NumItems
	}
	if input.DeliveryCategory != "" {
		if !validDeliveryCategory(input.DeliveryCategory) {
			return nil, fmt.Errorf("custom_data.delivery_category is invalid")
		}
		output["delivery_category"] = input.DeliveryCategory
	}
	seen := make(map[string]string)
	for _, properties := range []struct {
		kind    string
		strings map[string]string
		numbers map[string]Decimal
		bools   map[string]bool
	}{
		{kind: "string", strings: input.StringProperties},
		{kind: "number", numbers: input.NumberProperties},
		{kind: "boolean", bools: input.BooleanProperties},
	} {
		for key, value := range properties.strings {
			if err := validatePropertyKey(key, properties.kind, seen); err != nil {
				return nil, err
			}
			if !validOpaque(value, 4096) {
				return nil, fmt.Errorf("custom_data.string_properties contains an invalid value")
			}
			output[key] = value
		}
		for key, value := range properties.numbers {
			if err := validatePropertyKey(key, properties.kind, seen); err != nil {
				return nil, err
			}
			if !validDecimal(value) {
				return nil, fmt.Errorf("custom_data.number_properties contains an invalid value")
			}
			output[key] = value
		}
		for key, value := range properties.bools {
			if err := validatePropertyKey(key, properties.kind, seen); err != nil {
				return nil, err
			}
			output[key] = value
		}
	}
	return output, nil
}

func normalizeContent(input Content) (wireContent, error) {
	if !validOpaque(input.ID, 4096) {
		return wireContent{}, fmt.Errorf("id is invalid")
	}
	if input.Quantity < 0 {
		return wireContent{}, fmt.Errorf("quantity is invalid")
	}
	if input.ItemPrice != "" && !validDecimal(input.ItemPrice) {
		return wireContent{}, fmt.Errorf("item_price is invalid")
	}
	for _, field := range []struct {
		path  string
		value string
	}{
		{"title", input.Title}, {"description", input.Description}, {"category", input.Category}, {"brand", input.Brand},
	} {
		if !validOptionalOpaque(field.value, 4096) {
			return wireContent{}, fmt.Errorf("%s is invalid", field.path)
		}
	}
	if input.DeliveryCategory != "" && !validDeliveryCategory(input.DeliveryCategory) {
		return wireContent{}, fmt.Errorf("delivery_category is invalid")
	}
	return wireContent{
		ID: input.ID, Quantity: input.Quantity, ItemPrice: input.ItemPrice,
		Title: input.Title, Description: input.Description, Category: input.Category,
		Brand: input.Brand, DeliveryCategory: input.DeliveryCategory,
	}, nil
}

func validatePropertyKey(key, kind string, seen map[string]string) error {
	if !propertyPattern.MatchString(key) {
		return fmt.Errorf("custom_data.%s_properties contains an invalid key", kind)
	}
	if _, reserved := standardCustomDataFields[key]; reserved {
		return fmt.Errorf("custom_data.%s_properties key collides with a standard field", kind)
	}
	if previous, found := seen[key]; found {
		return fmt.Errorf("custom_data property key is duplicated across %s and %s properties", previous, kind)
	}
	seen[key] = kind
	return nil
}

func normalizeAppData(input AppData) (wireAppData, error) {
	if input.ExtendedDeviceInfo == nil {
		return wireAppData{}, fmt.Errorf("app_data.extended_device_info is required")
	}
	for _, field := range []struct {
		path    string
		value   string
		maximum int
	}{
		{"app_data.campaign_ids", input.CampaignIDs, 4096},
		{"app_data.install_referrer", input.InstallReferrer, 8192},
		{"app_data.installer_package", input.InstallerPackage, 4096},
		{"app_data.receipt_data", input.ReceiptData, 65_536},
		{"app_data.url_schemes", input.URLSchemes, 8192},
		{"app_data.windows_attribution_id", input.WindowsAttributionID, 4096},
	} {
		if !validOptionalOpaque(field.value, field.maximum) {
			return wireAppData{}, fmt.Errorf("%s is invalid", field.path)
		}
	}
	extInfo, err := normalizeExtendedDeviceInfo(*input.ExtendedDeviceInfo)
	if err != nil {
		return wireAppData{}, err
	}
	return wireAppData{
		ApplicationTrackingEnabled: input.ApplicationTrackingEnabled,
		AdvertiserTrackingEnabled:  input.AdvertiserTrackingEnabled,
		CampaignIDs:                input.CampaignIDs, ConsiderViews: input.ConsiderViews, ExtInfo: extInfo,
		IncludeDwellData: input.IncludeDwellData, IncludeVideoData: input.IncludeVideoData,
		InstallReferrer: input.InstallReferrer, InstallerPackage: input.InstallerPackage,
		ReceiptData: input.ReceiptData, URLSchemes: input.URLSchemes,
		WindowsAttributionID: input.WindowsAttributionID,
	}, nil
}

func normalizeExtendedDeviceInfo(input ExtendedDeviceInfo) ([]any, error) {
	if !validOpaque(input.Version, 64) || !validOpaque(input.AppPackageName, 1024) {
		return nil, fmt.Errorf("app_data.extended_device_info requires version and app_package_name")
	}
	for _, field := range []struct {
		path  string
		value string
	}{
		{"short_version", input.ShortVersion}, {"long_version", input.LongVersion},
		{"os_version", input.OSVersion}, {"device_model_name", input.DeviceModelName},
		{"locale", input.Locale}, {"timezone_abbreviation", input.TimezoneAbbreviation},
		{"carrier", input.Carrier}, {"screen_density", input.ScreenDensity},
		{"device_time_zone", input.DeviceTimeZone},
	} {
		if !validOptionalOpaque(field.value, 1024) {
			return nil, fmt.Errorf("app_data.extended_device_info.%s is invalid", field.path)
		}
	}
	if input.ScreenWidth < 0 || input.ScreenHeight < 0 || input.CPUCoreCount < 0 ||
		input.TotalDiskSpaceGB < 0 || input.FreeDiskSpaceGB < 0 {
		return nil, fmt.Errorf("app_data.extended_device_info contains a negative numeric field")
	}
	return []any{
		input.Version, input.AppPackageName, input.ShortVersion, input.LongVersion,
		input.OSVersion, input.DeviceModelName, input.Locale, input.TimezoneAbbreviation,
		input.Carrier, input.ScreenWidth, input.ScreenHeight, input.ScreenDensity,
		input.CPUCoreCount, input.TotalDiskSpaceGB, input.FreeDiskSpaceGB, input.DeviceTimeZone,
	}, nil
}

func validActionSource(value ActionSource) bool {
	switch value {
	case ActionSourceWebsite, ActionSourceApp, ActionSourcePhysicalStore, ActionSourceSystemGenerated,
		ActionSourceBusinessMessaging, ActionSourceChat, ActionSourceEmail, ActionSourceOther, ActionSourcePhoneCall:
		return true
	default:
		return false
	}
}

func validMessagingChannel(value MessagingChannel) bool {
	return value == MessagingChannelMessenger || value == MessagingChannelWhatsApp || value == MessagingChannelInstagram
}

func validDeliveryCategory(value DeliveryCategory) bool {
	return value == DeliveryCategoryInStore || value == DeliveryCategoryCurbside || value == DeliveryCategoryHomeDelivery
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

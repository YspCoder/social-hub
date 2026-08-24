package conversions

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"time"
	"unicode"
)

const maximumUnixSeconds int64 = 253402300799

type wireEnvelope struct {
	Data []wireEvent `json:"data"`
}

type wireEvent struct {
	ActionSource   ActionSource    `json:"action_source"`
	EventID        string          `json:"event_id"`
	EventName      EventName       `json:"event_name"`
	EventTime      int64           `json:"event_time"`
	EventSourceURL string          `json:"event_source_url,omitempty"`
	OptOut         *bool           `json:"opt_out,omitempty"`
	PartnerName    string          `json:"partner_name,omitempty"`
	UserData       wireUserData    `json:"user_data"`
	CustomData     *wireCustomData `json:"custom_data,omitempty"`

	AppID         string      `json:"app_id,omitempty"`
	AppName       string      `json:"app_name,omitempty"`
	AppVersion    string      `json:"app_version,omitempty"`
	DeviceBrand   string      `json:"device_brand,omitempty"`
	DeviceCarrier string      `json:"device_carrier,omitempty"`
	DeviceModel   string      `json:"device_model,omitempty"`
	DeviceType    string      `json:"device_type,omitempty"`
	OSVersion     string      `json:"os_version,omitempty"`
	WiFi          *bool       `json:"wifi,omitempty"`
	Language      string      `json:"language,omitempty"`
	AppInfo       *AppInfo    `json:"app_info,omitempty"`
	DeviceInfo    *DeviceInfo `json:"device_info,omitempty"`
}

type wireUserData struct {
	Emails       []string `json:"em,omitempty"`
	Phones       []string `json:"ph,omitempty"`
	Genders      []string `json:"ge,omitempty"`
	DatesOfBirth []string `json:"db,omitempty"`
	LastNames    []string `json:"ln,omitempty"`
	FirstNames   []string `json:"fn,omitempty"`
	Cities       []string `json:"ct,omitempty"`
	States       []string `json:"st,omitempty"`
	Zips         []string `json:"zp,omitempty"`
	Countries    []string `json:"country,omitempty"`
	ExternalIDs  []string `json:"external_id,omitempty"`
	HashedMAIDs  []string `json:"hashed_maids,omitempty"`

	ClientIPAddress string `json:"client_ip_address,omitempty"`
	ClientUserAgent string `json:"client_user_agent,omitempty"`
	ClickID         string `json:"click_id,omitempty"`
	PartnerID       string `json:"partner_id,omitempty"`
}

type wireCustomData struct {
	ContentBrand    string        `json:"content_brand,omitempty"`
	ContentCategory string        `json:"content_category,omitempty"`
	ContentIDs      []string      `json:"content_ids,omitempty"`
	ContentName     string        `json:"content_name,omitempty"`
	Contents        []wireContent `json:"contents,omitempty"`
	Currency        string        `json:"currency,omitempty"`
	NumItems        *int64        `json:"num_items,omitempty"`
	OptOutType      OptOutType    `json:"opt_out_type,omitempty"`
	OrderID         string        `json:"order_id,omitempty"`
	PredictedLTV    Decimal       `json:"predicted_ltv,omitempty"`
	SearchString    string        `json:"search_string,omitempty"`
	Value           Decimal       `json:"value,omitempty"`
}

type wireContent struct {
	ID           string  `json:"id,omitempty"`
	ItemBrand    string  `json:"item_brand,omitempty"`
	ItemBrandID  string  `json:"item_brand_id,omitempty"`
	ItemCategory string  `json:"item_category,omitempty"`
	ItemName     string  `json:"item_name,omitempty"`
	ItemPrice    Decimal `json:"item_price,omitempty"`
	Quantity     *int64  `json:"quantity,omitempty"`
}

type hashKind uint8

const (
	hashEmail hashKind = iota
	hashPhone
	hashGender
	hashDateOfBirth
	hashLastName
	hashFirstName
	hashCity
	hashState
	hashZip
	hashCountry
	hashExternalID
	hashMAID
)

func normalizeRequest(input SubmitEventsRequest) (wireEnvelope, error) {
	limit := MaximumBatchSize
	if input.Test {
		limit = MaximumTestBatchSize
	}
	if len(input.Events) == 0 || len(input.Events) > limit {
		return wireEnvelope{}, fmt.Errorf("events must contain between 1 and %d entries", limit)
	}
	output := wireEnvelope{Data: make([]wireEvent, len(input.Events))}
	seen := make(map[string]struct{}, len(input.Events))
	for index := range input.Events {
		event, err := normalizeEvent(input.Events[index])
		if err != nil {
			return wireEnvelope{}, fmt.Errorf("events[%d]: %w", index, err)
		}
		key := strings.ToLower(string(event.EventName)) + "\x00" + event.EventID
		if _, exists := seen[key]; exists {
			return wireEnvelope{}, fmt.Errorf("events[%d] duplicates event_id and event_name within the batch", index)
		}
		seen[key] = struct{}{}
		output.Data[index] = event
	}
	return output, nil
}

func normalizeEvent(input ConversionEvent) (wireEvent, error) {
	if !validActionSource(input.ActionSource) || !validOpaque(input.EventID, 4096) ||
		!validEventName(input.EventName) || input.EventTime <= 0 || input.EventTime > maximumUnixSeconds {
		return wireEvent{}, fmt.Errorf("action_source, event_id, event_name, or Unix-second event_time is invalid")
	}
	if input.EventSourceURL != "" && (input.ActionSource != ActionSourceWeb || !validPublicURL(input.EventSourceURL)) {
		return wireEvent{}, fmt.Errorf("event_source_url must be a public HTTP(S) URL on web events")
	}
	if !validPartnerName(input.PartnerName) {
		return wireEvent{}, fmt.Errorf("partner_name must be omitted or use the Pinterest-assigned lowercase ss-company form")
	}
	for _, field := range []struct {
		path    string
		value   string
		maximum int
	}{
		{"app_id", input.AppID, 4096}, {"app_name", input.AppName, 4096},
		{"app_version", input.AppVersion, 4096}, {"device_brand", input.DeviceBrand, 4096},
		{"device_carrier", input.DeviceCarrier, 4096}, {"device_model", input.DeviceModel, 4096},
		{"device_type", input.DeviceType, 4096}, {"os_version", input.OSVersion, 4096},
	} {
		if !validOptionalText(field.value, field.maximum) {
			return wireEvent{}, fmt.Errorf("%s is invalid", field.path)
		}
	}
	if input.Language != "" && !validLanguage(input.Language) {
		return wireEvent{}, fmt.Errorf("language must be a lowercase ISO 639-1 code")
	}
	user, err := normalizeUserData(input.UserData)
	if err != nil {
		return wireEvent{}, err
	}
	custom, err := normalizeCustomData(input.CustomData)
	if err != nil {
		return wireEvent{}, err
	}
	app, err := normalizeAppInfo(input.AppInfo)
	if err != nil {
		return wireEvent{}, err
	}
	device, err := normalizeDeviceInfo(input.DeviceInfo)
	if err != nil {
		return wireEvent{}, err
	}
	return wireEvent{
		ActionSource: input.ActionSource, EventID: input.EventID, EventName: input.EventName,
		EventTime: input.EventTime, EventSourceURL: input.EventSourceURL, OptOut: input.OptOut,
		PartnerName: input.PartnerName, UserData: user, CustomData: custom,
		AppID: input.AppID, AppName: input.AppName, AppVersion: input.AppVersion,
		DeviceBrand: input.DeviceBrand, DeviceCarrier: input.DeviceCarrier, DeviceModel: input.DeviceModel,
		DeviceType: input.DeviceType, OSVersion: input.OSVersion, WiFi: input.WiFi, Language: input.Language,
		AppInfo: app, DeviceInfo: device,
	}, nil
}

func normalizeUserData(input UserData) (wireUserData, error) {
	var output wireUserData
	fields := []struct {
		path   string
		values []string
		kind   hashKind
		target *[]string
	}{
		{"user_data.emails", input.Emails, hashEmail, &output.Emails},
		{"user_data.phones", input.Phones, hashPhone, &output.Phones},
		{"user_data.genders", input.Genders, hashGender, &output.Genders},
		{"user_data.dates_of_birth", input.DatesOfBirth, hashDateOfBirth, &output.DatesOfBirth},
		{"user_data.last_names", input.LastNames, hashLastName, &output.LastNames},
		{"user_data.first_names", input.FirstNames, hashFirstName, &output.FirstNames},
		{"user_data.cities", input.Cities, hashCity, &output.Cities},
		{"user_data.states", input.States, hashState, &output.States},
		{"user_data.zips", input.Zips, hashZip, &output.Zips},
		{"user_data.countries", input.Countries, hashCountry, &output.Countries},
		{"user_data.external_ids", input.ExternalIDs, hashExternalID, &output.ExternalIDs},
		{"user_data.mobile_advertising_ids", input.MobileAdvertisingIDs, hashMAID, &output.HashedMAIDs},
	}
	for _, field := range fields {
		var err error
		*field.target, err = normalizeMulti(field.values, field.kind, field.path)
		if err != nil {
			return wireUserData{}, err
		}
	}
	if input.ClientIPAddress != "" {
		address := net.ParseIP(input.ClientIPAddress)
		if address == nil || address.IsUnspecified() {
			return wireUserData{}, fmt.Errorf("user_data.client_ip_address is invalid")
		}
	}
	if !validOptionalOpaque(input.ClientUserAgent, 16_384) || !validOptionalOpaque(input.ClickID, 4096) ||
		!validOptionalOpaque(input.PartnerID, 4096) {
		return wireUserData{}, fmt.Errorf("user_data user agent, click_id, or partner_id is invalid")
	}
	output.ClientIPAddress = input.ClientIPAddress
	output.ClientUserAgent = input.ClientUserAgent
	output.ClickID = input.ClickID
	output.PartnerID = input.PartnerID
	if len(output.Emails) == 0 && len(output.HashedMAIDs) == 0 &&
		(output.ClientIPAddress == "" || output.ClientUserAgent == "") {
		return wireUserData{}, fmt.Errorf("user_data requires email, mobile advertising ID, or the IP address and user agent pair")
	}
	return output, nil
}

func normalizeMulti(values []string, kind hashKind, path string) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	if len(values) > MaximumBatchSize {
		return nil, fmt.Errorf("%s has too many values", path)
	}
	seen := make(map[string]struct{}, len(values))
	output := make([]string, 0, len(values))
	for index, value := range values {
		hashed, err := normalizeAndHash(value, kind)
		if err != nil {
			return nil, fmt.Errorf("%s[%d] is invalid", path, index)
		}
		if _, exists := seen[hashed]; exists {
			continue
		}
		seen[hashed] = struct{}{}
		output = append(output, hashed)
	}
	return output, nil
}

func normalizeAndHash(value string, kind hashKind) (string, error) {
	trimmed := strings.TrimSpace(value)
	if !validOpaque(trimmed, 4096) {
		return "", fmt.Errorf("invalid identifier")
	}
	if lowerSHA256.MatchString(trimmed) {
		return trimmed, nil
	}
	if anySHA256.MatchString(trimmed) || legacyMD5.MatchString(trimmed) {
		return "", fmt.Errorf("unsupported digest")
	}
	var normalized string
	switch kind {
	case hashEmail:
		normalized = strings.Map(func(character rune) rune {
			if unicode.IsSpace(character) {
				return -1
			}
			return unicode.ToLower(character)
		}, trimmed)
		if !emailPattern.MatchString(normalized) {
			return "", fmt.Errorf("invalid email")
		}
	case hashPhone:
		normalized = strings.TrimLeft(digitsOnly(trimmed), "0")
		if len(normalized) < 7 || len(normalized) > 15 {
			return "", fmt.Errorf("invalid phone")
		}
	case hashGender:
		normalized = strings.ToLower(trimmed)
		if normalized != "f" && normalized != "m" && normalized != "n" {
			return "", fmt.Errorf("invalid gender")
		}
	case hashDateOfBirth:
		normalized = digitsOnly(trimmed)
		if len(normalized) != 8 {
			return "", fmt.Errorf("invalid date of birth")
		}
		if _, err := time.Parse("20060102", normalized); err != nil {
			return "", fmt.Errorf("invalid date of birth")
		}
	case hashLastName, hashFirstName:
		normalized = strings.ToLower(trimmed)
	case hashCity:
		normalized = strings.Map(func(character rune) rune {
			if unicode.IsLetter(character) || unicode.IsDigit(character) {
				return unicode.ToLower(character)
			}
			return -1
		}, trimmed)
		if normalized == "" {
			return "", fmt.Errorf("invalid city")
		}
	case hashState, hashCountry:
		normalized = strings.ToLower(trimmed)
		if len(normalized) != 2 || normalized[0] < 'a' || normalized[0] > 'z' || normalized[1] < 'a' || normalized[1] > 'z' {
			return "", fmt.Errorf("invalid region")
		}
	case hashZip:
		normalized = digitsOnly(trimmed)
		if normalized == "" {
			return "", fmt.Errorf("invalid zip")
		}
	case hashExternalID:
		normalized = trimmed
	case hashMAID:
		if !maidPattern.MatchString(trimmed) || strings.EqualFold(trimmed, "00000000-0000-0000-0000-000000000000") {
			return "", fmt.Errorf("invalid mobile advertising ID")
		}
		normalized = strings.ToLower(trimmed)
	default:
		return "", fmt.Errorf("unsupported identifier")
	}
	digest := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(digest[:]), nil
}

func normalizeCustomData(input *CustomData) (*wireCustomData, error) {
	if input == nil {
		return nil, nil
	}
	for _, field := range []string{input.ContentBrand, input.ContentCategory, input.ContentName, input.OrderID, input.SearchString} {
		if !validOptionalText(field, 4096) {
			return nil, fmt.Errorf("custom_data contains an invalid text field")
		}
	}
	if len(input.ContentIDs) > MaximumBatchSize {
		return nil, fmt.Errorf("custom_data.content_ids has too many values")
	}
	contentIDs := make([]string, len(input.ContentIDs))
	for index, value := range input.ContentIDs {
		if !validText(value, 4096) {
			return nil, fmt.Errorf("custom_data.content_ids[%d] is invalid", index)
		}
		contentIDs[index] = value
	}
	currency := strings.ToUpper(strings.TrimSpace(input.Currency))
	if currency != "" && !validCurrency(currency) || input.Value != "" && (currency == "" || !validDecimal(input.Value)) ||
		input.PredictedLTV != "" && !validDecimal(input.PredictedLTV) || input.NumItems != nil && *input.NumItems < 0 ||
		input.OptOutType != "" && input.OptOutType != OptOutLDP {
		return nil, fmt.Errorf("custom_data money, item count, or opt-out fields are invalid")
	}
	if len(input.Contents) > MaximumBatchSize {
		return nil, fmt.Errorf("custom_data.contents has too many values")
	}
	contents := make([]wireContent, len(input.Contents))
	for index, item := range input.Contents {
		if !validOptionalText(item.ID, 4096) || !validOptionalText(item.ItemBrand, 4096) ||
			!validOptionalText(item.ItemBrandID, 64) || !validOptionalText(item.ItemCategory, 4096) ||
			!validOptionalText(item.ItemName, 4096) || item.ItemPrice != "" && !validDecimal(item.ItemPrice) ||
			item.Quantity != nil && *item.Quantity < 0 || item.ID == "" && item.ItemBrand == "" && item.ItemBrandID == "" &&
			item.ItemCategory == "" && item.ItemName == "" && item.ItemPrice == "" && item.Quantity == nil {
			return nil, fmt.Errorf("custom_data.contents[%d] is invalid", index)
		}
		contents[index] = wireContent{
			ID: item.ID, ItemBrand: item.ItemBrand, ItemBrandID: item.ItemBrandID,
			ItemCategory: item.ItemCategory, ItemName: item.ItemName, ItemPrice: item.ItemPrice, Quantity: item.Quantity,
		}
	}
	return &wireCustomData{
		ContentBrand: input.ContentBrand, ContentCategory: input.ContentCategory,
		ContentIDs: contentIDs, ContentName: input.ContentName, Contents: contents,
		Currency: currency, NumItems: input.NumItems, OptOutType: input.OptOutType,
		OrderID: input.OrderID, PredictedLTV: input.PredictedLTV, SearchString: input.SearchString, Value: input.Value,
	}, nil
}

func normalizeAppInfo(input *AppInfo) (*AppInfo, error) {
	if input == nil {
		return nil, nil
	}
	for _, field := range []struct {
		value   string
		maximum int
	}{
		{input.AppID, 200}, {input.AppName, 200}, {input.AppPackageName, 200}, {input.AppStore, 100},
		{input.AppVersion, 100}, {input.UserAgent, 16_384},
	} {
		if !validOptionalText(field.value, field.maximum) {
			return nil, fmt.Errorf("app_info contains an invalid string")
		}
	}
	if !int64InRange(input.InstallTime, 0, maximumUnixSeconds) || !integerInRange(input.WindowHeight, 0, 30_720) ||
		!integerInRange(input.WindowWidth, 0, 30_720) {
		return nil, fmt.Errorf("app_info contains an out-of-range number")
	}
	copy := *input
	return &copy, nil
}

func normalizeDeviceInfo(input *DeviceInfo) (*DeviceInfo, error) {
	if input == nil {
		return nil, nil
	}
	for _, field := range []struct {
		value   string
		maximum int
	}{
		{input.Brand, 100}, {input.Carrier, 100}, {input.KernelVersion, 100}, {input.Locale, 35},
		{input.Model, 100}, {input.OSName, 100}, {input.OSReleaseName, 100}, {input.OSVersion, 100},
		{input.Timezone, 40}, {input.TimezoneAbbreviation, 5}, {input.Type, 100},
	} {
		if !validOptionalText(field.value, field.maximum) {
			return nil, fmt.Errorf("device_info contains an invalid string")
		}
	}
	if !validFormFactor(input.FormFactor) || !validNetworkType(input.NetworkType) || !validOSFamily(input.OSFamily) ||
		!integerInRange(input.BatteryLevel, 0, 100) || !integerInRange(input.CPUCores, 0, 1152) ||
		!integerInRange(input.ExternalStorageFreeSpace, 0, 1_048_576) || !integerInRange(input.ExternalStorageSize, 0, 1_048_576) ||
		!integerInRange(input.ScreenDensity, 0, 100_000) || !integerInRange(input.ScreenHeight, 0, 30_720) ||
		!integerInRange(input.ScreenWidth, 0, 30_720) || !integerInRange(input.StorageFreeSpace, 0, 1_048_576) ||
		!integerInRange(input.StorageSize, 0, 1_048_576) {
		return nil, fmt.Errorf("device_info contains an invalid enum or number")
	}
	if len(input.Languages) > 100 {
		return nil, fmt.Errorf("device_info.languages has too many values")
	}
	languages := make([]string, len(input.Languages))
	for index, language := range input.Languages {
		if !validLanguage(language) {
			return nil, fmt.Errorf("device_info.languages[%d] is invalid", index)
		}
		languages[index] = language
	}
	copy := *input
	copy.Languages = languages
	return &copy, nil
}

func digitsOnly(value string) string {
	var output strings.Builder
	for _, character := range value {
		if character >= '0' && character <= '9' {
			output.WriteRune(character)
		}
	}
	return output.String()
}

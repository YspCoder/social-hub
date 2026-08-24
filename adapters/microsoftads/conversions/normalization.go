package conversions

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

type wireEnvelope struct {
	Data         []wireEvent `json:"data"`
	DataProvider string      `json:"dataProvider,omitempty"`
}

type wireEvent struct {
	EventType        EventType        `json:"eventType"`
	EventTime        int64            `json:"eventTime"`
	EventID          string           `json:"eventId,omitempty"`
	EventName        string           `json:"eventName,omitempty"`
	EventSourceURL   string           `json:"eventSourceUrl,omitempty"`
	PageLoadID       string           `json:"pageLoadId,omitempty"`
	ReferrerURL      string           `json:"referrerUrl,omitempty"`
	PageTitle        string           `json:"pageTitle,omitempty"`
	Keywords         string           `json:"keywords,omitempty"`
	AdStorageConsent AdStorageConsent `json:"adStorageConsent,omitempty"`
	UserData         wireUserData     `json:"userData"`
	CustomData       *wireCustomData  `json:"customData,omitempty"`
}

type wireUserData struct {
	MicrosoftClickID string `json:"msclkid,omitempty"`
	Email            string `json:"em,omitempty"`
	Phone            string `json:"ph,omitempty"`
	AnonymousID      string `json:"anonymousId,omitempty"`
	ExternalID       string `json:"externalId,omitempty"`
	ClientUserAgent  string `json:"clientUserAgent,omitempty"`
	ClientIPAddress  string `json:"clientIpAddress,omitempty"`
	IDFA             string `json:"idfa,omitempty"`
	GAID             string `json:"gaid,omitempty"`
}

type wireCustomData struct {
	EventCategory   string         `json:"eventCategory,omitempty"`
	EventLabel      string         `json:"eventLabel,omitempty"`
	EventValue      Decimal        `json:"eventValue,omitempty"`
	SearchTerm      string         `json:"searchTerm,omitempty"`
	TransactionID   string         `json:"transactionId,omitempty"`
	Value           Decimal        `json:"value,omitempty"`
	Currency        string         `json:"currency,omitempty"`
	Items           []wireItem     `json:"items,omitempty"`
	ItemIDs         []string       `json:"itemIds,omitempty"`
	PageType        PageType       `json:"pageType,omitempty"`
	EcommTotalValue Decimal        `json:"ecommTotalValue,omitempty"`
	EcommCategory   string         `json:"ecommCategory,omitempty"`
	HotelData       *wireHotelData `json:"hotelData,omitempty"`
}

type wireItem struct {
	ID       string  `json:"id,omitempty"`
	Quantity *int64  `json:"quantity,omitempty"`
	Price    Decimal `json:"price,omitempty"`
	Name     string  `json:"name,omitempty"`
}

type wireHotelData struct {
	TotalPrice     Decimal `json:"totalPrice,omitempty"`
	BasePrice      Decimal `json:"basePrice,omitempty"`
	CheckinDate    string  `json:"checkinDate"`
	CheckoutDate   string  `json:"checkoutDate,omitempty"`
	LengthOfStay   *int64  `json:"lengthOfStay,omitempty"`
	PartnerHotelID string  `json:"partnerHotelId,omitempty"`
	BookingHref    string  `json:"bookingHref,omitempty"`
}

func normalizeRequest(now time.Time, input SubmitEventsRequest) (wireEnvelope, error) {
	if !validOptionalText(input.DataProvider, 512) {
		return wireEnvelope{}, fmt.Errorf("data_provider is invalid")
	}
	if len(input.Events) == 0 || len(input.Events) > MaximumBatchSize {
		return wireEnvelope{}, fmt.Errorf("events must contain between 1 and %d entries", MaximumBatchSize)
	}
	output := wireEnvelope{DataProvider: input.DataProvider, Data: make([]wireEvent, len(input.Events))}
	for index := range input.Events {
		event, err := normalizeEvent(now, input.Events[index])
		if err != nil {
			return wireEnvelope{}, fmt.Errorf("events[%d]: %w", index, err)
		}
		output.Data[index] = event
	}
	return output, nil
}

func normalizeEvent(now time.Time, input ConversionEvent) (wireEvent, error) {
	if input.EventType != EventTypePageLoad && input.EventType != EventTypeCustom {
		return wireEvent{}, fmt.Errorf("event_type must be pageLoad or custom")
	}
	minimum := now.Add(-MaximumEventAgeDays * 24 * time.Hour).Unix()
	if input.EventTime < minimum || input.EventTime > now.Unix() {
		return wireEvent{}, fmt.Errorf("event_time must be within the past 7 days")
	}
	for _, value := range []string{input.EventID, input.EventName, input.PageTitle, input.Keywords} {
		if !validOptionalText(value, 4096) {
			return wireEvent{}, fmt.Errorf("event contains an invalid text field")
		}
	}
	if input.EventType == EventTypePageLoad && !validURL(input.EventSourceURL) ||
		input.EventSourceURL != "" && !validURL(input.EventSourceURL) ||
		input.ReferrerURL != "" && !validURL(input.ReferrerURL) {
		return wireEvent{}, fmt.Errorf("pageLoad requires event_source_url and event URLs must be absolute HTTP(S) URLs")
	}
	if input.PageLoadID != "" && !validUUIDv4(input.PageLoadID) {
		return wireEvent{}, fmt.Errorf("page_load_id must be a v4 UUID")
	}
	if input.AdStorageConsent != "" && input.AdStorageConsent != ConsentGranted && input.AdStorageConsent != ConsentDenied {
		return wireEvent{}, fmt.Errorf("ad_storage_consent must be G or D")
	}
	user, err := normalizeUserData(input.UserData)
	if err != nil {
		return wireEvent{}, err
	}
	custom, err := normalizeCustomData(input.EventType, input.CustomData)
	if err != nil {
		return wireEvent{}, err
	}
	return wireEvent{
		EventType: input.EventType, EventTime: input.EventTime, EventID: input.EventID,
		EventName: input.EventName, EventSourceURL: input.EventSourceURL, PageLoadID: input.PageLoadID,
		ReferrerURL: input.ReferrerURL, PageTitle: input.PageTitle, Keywords: input.Keywords,
		AdStorageConsent: input.AdStorageConsent, UserData: user, CustomData: custom,
	}, nil
}

func normalizeUserData(input UserData) (wireUserData, error) {
	email, err := normalizeEmail(input.Email)
	if err != nil {
		return wireUserData{}, fmt.Errorf("user_data.email is invalid")
	}
	phone, err := normalizePhone(input.Phone)
	if err != nil {
		return wireUserData{}, fmt.Errorf("user_data.phone is invalid")
	}
	for _, value := range []string{input.AnonymousID, input.ExternalID, input.ClientUserAgent} {
		if !validOptionalOpaque(value, 16_384) {
			return wireUserData{}, fmt.Errorf("user_data contains an invalid anonymous, external, or user-agent value")
		}
	}
	if input.ClientIPAddress != "" && !validIPAddress(input.ClientIPAddress) {
		return wireUserData{}, fmt.Errorf("user_data.client_ip_address is invalid")
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{"msclkid", input.MicrosoftClickID}, {"idfa", input.IDFA}, {"gaid", input.GAID},
	} {
		if field.value != "" && !validUUID(strings.TrimSpace(field.value)) {
			return wireUserData{}, fmt.Errorf("user_data.%s is invalid", field.name)
		}
	}
	if input.AnonymousID == "" && input.ExternalID == "" && email == "" && phone == "" &&
		input.MicrosoftClickID == "" && input.IDFA == "" && input.GAID == "" {
		return wireUserData{}, fmt.Errorf("user_data requires a supported matching identifier")
	}
	return wireUserData{
		MicrosoftClickID: strings.TrimSpace(input.MicrosoftClickID), Email: email, Phone: phone,
		AnonymousID: input.AnonymousID, ExternalID: input.ExternalID, ClientUserAgent: input.ClientUserAgent,
		ClientIPAddress: strings.TrimSpace(input.ClientIPAddress), IDFA: strings.TrimSpace(input.IDFA), GAID: strings.TrimSpace(input.GAID),
	}, nil
}

func normalizeEmail(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	if lowerSHA256.MatchString(trimmed) {
		return trimmed, nil
	}
	if anySHA256.MatchString(trimmed) || legacyMD5.MatchString(trimmed) {
		return "", fmt.Errorf("unsupported digest")
	}
	lower := strings.ToLower(trimmed)
	separator := strings.LastIndexByte(lower, '@')
	if separator <= 0 || separator == len(lower)-1 || strings.ContainsRune(lower[separator+1:], '@') {
		return "", fmt.Errorf("invalid email")
	}
	local, domain := lower[:separator], lower[separator+1:]
	if alias := strings.IndexByte(local, '+'); alias >= 0 {
		local = local[:alias]
	}
	local = strings.ReplaceAll(local, ".", "")
	normalized := local + "@" + domain
	if !emailPattern.MatchString(normalized) {
		return "", fmt.Errorf("invalid email")
	}
	digest := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(digest[:]), nil
}

func normalizePhone(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	if lowerSHA256.MatchString(trimmed) {
		return trimmed, nil
	}
	if anySHA256.MatchString(trimmed) || legacyMD5.MatchString(trimmed) || len(trimmed) < 8 || len(trimmed) > 16 || trimmed[0] != '+' {
		return "", fmt.Errorf("invalid phone")
	}
	for _, character := range trimmed[1:] {
		if character < '0' || character > '9' {
			return "", fmt.Errorf("invalid phone")
		}
	}
	digest := sha256.Sum256([]byte(trimmed))
	return hex.EncodeToString(digest[:]), nil
}

func normalizeCustomData(eventType EventType, input *CustomData) (*wireCustomData, error) {
	if input == nil {
		return nil, nil
	}
	for _, value := range []string{
		input.EventCategory, input.EventLabel, input.SearchTerm, input.TransactionID, input.EcommCategory,
	} {
		if !validOptionalText(value, 4096) {
			return nil, fmt.Errorf("custom_data contains an invalid text field")
		}
	}
	for _, value := range []Decimal{input.EventValue, input.Value, input.EcommTotalValue} {
		if value != "" && !validDecimal(value) {
			return nil, fmt.Errorf("custom_data contains an invalid decimal")
		}
	}
	if eventType == EventTypePageLoad && input.Value != "" {
		return nil, fmt.Errorf("pageLoad events cannot carry revenue value directly")
	}
	currency := strings.ToUpper(strings.TrimSpace(input.Currency))
	if currency != "" && !validCurrency(currency) ||
		(input.Value != "" || input.EcommTotalValue != "" || input.HotelData != nil && (input.HotelData.TotalPrice != "" || input.HotelData.BasePrice != "")) && currency == "" {
		return nil, fmt.Errorf("custom_data revenue requires a valid ISO currency")
	}
	if !validPageType(input.PageType) {
		return nil, fmt.Errorf("custom_data.page_type is invalid")
	}
	if len(input.Items) > MaximumBatchSize || len(input.ItemIDs) > MaximumBatchSize {
		return nil, fmt.Errorf("custom_data item arrays are too large")
	}
	items := make([]wireItem, len(input.Items))
	for index, item := range input.Items {
		if !validOptionalText(item.ID, 4096) || !validOptionalText(item.Name, 4096) ||
			item.Quantity != nil && *item.Quantity < 0 || item.Price != "" && !validDecimal(item.Price) ||
			item.ID == "" && item.Name == "" && item.Quantity == nil && item.Price == "" {
			return nil, fmt.Errorf("custom_data.items[%d] is invalid", index)
		}
		items[index] = wireItem{ID: item.ID, Quantity: item.Quantity, Price: item.Price, Name: item.Name}
	}
	itemIDs := append([]string(nil), input.ItemIDs...)
	for index, value := range itemIDs {
		if !validText(value, 4096) {
			return nil, fmt.Errorf("custom_data.item_ids[%d] is invalid", index)
		}
	}
	hotel, err := normalizeHotel(input.HotelData)
	if err != nil {
		return nil, err
	}
	return &wireCustomData{
		EventCategory: input.EventCategory, EventLabel: input.EventLabel, EventValue: input.EventValue,
		SearchTerm: input.SearchTerm, TransactionID: input.TransactionID, Value: input.Value,
		Currency: currency, Items: items, ItemIDs: itemIDs, PageType: input.PageType,
		EcommTotalValue: input.EcommTotalValue, EcommCategory: input.EcommCategory, HotelData: hotel,
	}, nil
}

func normalizeHotel(input *HotelData) (*wireHotelData, error) {
	if input == nil {
		return nil, nil
	}
	if !validDate(input.CheckinDate) || input.CheckoutDate != "" && !validDate(input.CheckoutDate) ||
		input.CheckoutDate == "" && (input.LengthOfStay == nil || *input.LengthOfStay <= 0) ||
		input.LengthOfStay != nil && *input.LengthOfStay <= 0 ||
		input.TotalPrice != "" && !validDecimal(input.TotalPrice) || input.BasePrice != "" && !validDecimal(input.BasePrice) ||
		!validOptionalText(input.PartnerHotelID, 4096) || !validOptionalText(input.BookingHref, 4096) {
		return nil, fmt.Errorf("custom_data.hotel_data is invalid")
	}
	if input.CheckoutDate != "" && input.CheckoutDate <= input.CheckinDate {
		return nil, fmt.Errorf("custom_data.hotel_data checkout must follow checkin")
	}
	return &wireHotelData{
		TotalPrice: input.TotalPrice, BasePrice: input.BasePrice, CheckinDate: input.CheckinDate,
		CheckoutDate: input.CheckoutDate, LengthOfStay: input.LengthOfStay,
		PartnerHotelID: input.PartnerHotelID, BookingHref: input.BookingHref,
	}, nil
}

func validPageType(value PageType) bool {
	switch value {
	case "", PageTypeCart, PageTypeCategory, PageTypeHome, PageTypeOther,
		PageTypeProduct, PageTypePurchase, PageTypeSearchResults:
		return true
	default:
		return false
	}
}

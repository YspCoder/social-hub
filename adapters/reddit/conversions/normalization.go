package conversions

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"
)

type wireEnvelope struct {
	Data wireRequest `json:"data"`
}

type wireRequest struct {
	TestID string      `json:"test_id,omitempty"`
	Events []wireEvent `json:"events"`
}

type wireEvent struct {
	ClickID        string        `json:"click_id,omitempty"`
	EventAt        int64         `json:"event_at"`
	ActionSource   ActionSource  `json:"action_source"`
	EventSourceURL string        `json:"event_source_url,omitempty"`
	Type           wireEventType `json:"type"`
	Metadata       *wireMetadata `json:"metadata,omitempty"`
	User           *wireUserData `json:"user,omitempty"`
}

type wireEventType struct {
	CustomEventName string       `json:"custom_event_name,omitempty"`
	TrackingType    TrackingType `json:"tracking_type"`
}

type wireMetadata struct {
	ConversionID string        `json:"conversion_id,omitempty"`
	Currency     string        `json:"currency,omitempty"`
	ItemCount    *int32        `json:"item_count,omitempty"`
	Value        Decimal       `json:"value,omitempty"`
	Products     []wireProduct `json:"products,omitempty"`
}

type wireProduct struct {
	Category  string  `json:"category,omitempty"`
	ID        string  `json:"id"`
	Name      string  `json:"name,omitempty"`
	Quantity  *int64  `json:"quantity,omitempty"`
	ItemPrice Decimal `json:"item_price,omitempty"`
}

type wireUserData struct {
	Email       string `json:"email,omitempty"`
	ExternalID  string `json:"external_id,omitempty"`
	IPAddress   string `json:"ip_address,omitempty"`
	PhoneNumber string `json:"phone_number,omitempty"`
	UserAgent   string `json:"user_agent,omitempty"`
	AAID        string `json:"aaid,omitempty"`
	IDFA        string `json:"idfa,omitempty"`
	UUID        string `json:"uuid,omitempty"`

	DataProcessingOptions *DataProcessingOptions `json:"data_processing_options,omitempty"`
	ScreenDimensions      *ScreenDimensions      `json:"screen_dimensions,omitempty"`
}

var (
	lowerSHA256Pattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
	anySHA256Pattern   = regexp.MustCompile(`(?i)^[a-f0-9]{64}$`)
	legacyMD5Pattern   = regexp.MustCompile(`(?i)^[a-f0-9]{32}$`)
	extensionPattern   = regexp.MustCompile(`(?i)(?:ext\.?|x)\s*[0-9]+\s*$`)
)

type signalKind uint8

const (
	signalEmail signalKind = iota
	signalPhone
	signalExternalID
	signalIDFA
	signalAAID
)

func normalizeRequest(input SubmitEventsRequest, now time.Time) (wireEnvelope, error) {
	if len(input.Events) == 0 || len(input.Events) > MaximumBatchSize {
		return wireEnvelope{}, fmt.Errorf("events must contain between 1 and %d entries", MaximumBatchSize)
	}
	if input.TestID != "" {
		if !validOpaque(input.TestID, 4096) {
			return wireEnvelope{}, fmt.Errorf("test_id is invalid")
		}
		if len(input.Events) != 1 {
			return wireEnvelope{}, fmt.Errorf("test requests must contain exactly one event")
		}
	}
	output := wireEnvelope{Data: wireRequest{TestID: input.TestID, Events: make([]wireEvent, len(input.Events))}}
	for index := range input.Events {
		event, err := normalizeEvent(input.Events[index], now)
		if err != nil {
			return wireEnvelope{}, fmt.Errorf("events[%d]: %w", index, err)
		}
		output.Data.Events[index] = event
	}
	return output, nil
}

func normalizeEvent(input ConversionEvent, now time.Time) (wireEvent, error) {
	when := time.UnixMilli(input.EventAt)
	if input.EventAt <= 0 || when.Before(time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)) ||
		when.Before(now.Add(-7*24*time.Hour)) || when.After(now.Add(5*time.Minute)) {
		return wireEvent{}, fmt.Errorf("event_at must be Unix milliseconds from the past seven days and no more than five minutes in the future")
	}
	if !validActionSource(input.ActionSource) || !validTrackingType(input.Type.TrackingType) {
		return wireEvent{}, fmt.Errorf("action_source or tracking_type is invalid")
	}
	if input.Type.TrackingType == TrackingCustom {
		if !validText(input.Type.CustomEventName, 64) {
			return wireEvent{}, fmt.Errorf("custom_event_name is required for CUSTOM and must not exceed 64 characters")
		}
	} else if input.Type.CustomEventName != "" {
		return wireEvent{}, fmt.Errorf("custom_event_name requires tracking_type CUSTOM")
	}
	if !validOptionalOpaque(input.ClickID, 4096) {
		return wireEvent{}, fmt.Errorf("click_id is invalid")
	}
	if input.EventSourceURL != "" {
		if input.ActionSource != ActionSourceWebsite || !validPublicURL(input.EventSourceURL) {
			return wireEvent{}, fmt.Errorf("event_source_url must be a public HTTP(S) URL on WEBSITE events")
		}
	}
	metadata, err := normalizeMetadata(input.Metadata)
	if err != nil {
		return wireEvent{}, err
	}
	user, err := normalizeUserData(input.User)
	if err != nil {
		return wireEvent{}, err
	}
	return wireEvent{
		ClickID: input.ClickID, EventAt: input.EventAt, ActionSource: input.ActionSource,
		EventSourceURL: input.EventSourceURL,
		Type:           wireEventType{TrackingType: input.Type.TrackingType, CustomEventName: input.Type.CustomEventName},
		Metadata:       metadata, User: user,
	}, nil
}

func normalizeMetadata(input *Metadata) (*wireMetadata, error) {
	if input == nil {
		return nil, nil
	}
	if !validOptionalOpaque(input.ConversionID, 4096) || input.ItemCount != nil && *input.ItemCount < 0 {
		return nil, fmt.Errorf("metadata.conversion_id or item_count is invalid")
	}
	currency := strings.ToUpper(strings.TrimSpace(input.Currency))
	if currency != "" && !validCurrency(currency) || input.Value != "" && !validNonnegativeDecimal(input.Value) {
		return nil, fmt.Errorf("metadata.value or ISO 4217 currency is invalid")
	}
	products := make([]wireProduct, len(input.Products))
	for index, product := range input.Products {
		if !validOpaque(product.ID, 4096) || !validOptionalOpaque(product.Category, 4096) ||
			!validOptionalOpaque(product.Name, 4096) || product.Quantity != nil && *product.Quantity < 0 ||
			product.ItemPrice != "" && !validNonnegativeDecimal(product.ItemPrice) {
			return nil, fmt.Errorf("metadata.products[%d] is invalid", index)
		}
		products[index] = wireProduct{
			Category: product.Category, ID: product.ID, Name: product.Name,
			Quantity: product.Quantity, ItemPrice: product.ItemPrice,
		}
	}
	return &wireMetadata{
		ConversionID: input.ConversionID, Currency: currency, ItemCount: input.ItemCount,
		Value: input.Value, Products: products,
	}, nil
}

func normalizeUserData(input *UserData) (*wireUserData, error) {
	if input == nil {
		return nil, nil
	}
	email, err := normalizeAndHash(input.Email, signalEmail)
	if err != nil {
		return nil, fmt.Errorf("user.email is invalid")
	}
	phone, err := normalizeAndHash(input.PhoneNumber, signalPhone)
	if err != nil {
		return nil, fmt.Errorf("user.phone_number is invalid")
	}
	externalID, err := normalizeAndHash(input.ExternalID, signalExternalID)
	if err != nil {
		return nil, fmt.Errorf("user.external_id is invalid")
	}
	idfa, err := normalizeAndHash(input.IDFA, signalIDFA)
	if err != nil {
		return nil, fmt.Errorf("user.idfa is invalid")
	}
	aaid, err := normalizeAndHash(input.AAID, signalAAID)
	if err != nil {
		return nil, fmt.Errorf("user.aaid is invalid")
	}
	pixelUUID, err := normalizePixelUUID(input.UUID)
	if input.IPAddress != "" && net.ParseIP(input.IPAddress) == nil || !validOptionalOpaque(input.UserAgent, 8192) || err != nil {
		return nil, fmt.Errorf("user.ip_address, user_agent, or uuid is invalid")
	}
	dpo, err := normalizeDataProcessing(input.DataProcessingOptions)
	if err != nil {
		return nil, err
	}
	dimensions, err := normalizeScreenDimensions(input.ScreenDimensions)
	if err != nil {
		return nil, err
	}
	return &wireUserData{
		Email: email, ExternalID: externalID, IPAddress: input.IPAddress, PhoneNumber: phone,
		UserAgent: input.UserAgent, AAID: aaid, IDFA: idfa, UUID: pixelUUID,
		DataProcessingOptions: dpo, ScreenDimensions: dimensions,
	}, nil
}

func normalizePixelUUID(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if !pixelCookiePattern.MatchString(value) {
		return "", fmt.Errorf("invalid Reddit Pixel UUID")
	}
	if separator := strings.IndexByte(value, '.'); separator >= 0 {
		return value[separator+1:], nil
	}
	return value, nil
}

func normalizeAndHash(value string, kind signalKind) (string, error) {
	if value == "" {
		return "", nil
	}
	trimmed := strings.TrimSpace(value)
	if !validOpaque(trimmed, 4096) || trimmed != value {
		return "", fmt.Errorf("invalid signal")
	}
	if lowerSHA256Pattern.MatchString(trimmed) {
		return trimmed, nil
	}
	if anySHA256Pattern.MatchString(trimmed) || legacyMD5Pattern.MatchString(trimmed) {
		return "", fmt.Errorf("unsupported digest")
	}
	var normalized string
	var err error
	switch kind {
	case signalEmail:
		normalized, err = normalizeEmail(trimmed)
	case signalPhone:
		normalized, err = normalizePhone(trimmed)
	case signalExternalID:
		normalized = trimmed
	case signalIDFA, signalAAID:
		if !uuidPattern.MatchString(trimmed) || strings.EqualFold(trimmed, "00000000-0000-0000-0000-000000000000") {
			err = fmt.Errorf("invalid mobile advertising ID")
		} else if kind == signalIDFA {
			normalized = strings.ToUpper(trimmed)
		} else {
			normalized = strings.ToLower(trimmed)
		}
	default:
		err = fmt.Errorf("unsupported signal")
	}
	if err != nil || normalized == "" {
		return "", err
	}
	digest := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(digest[:]), nil
}

func normalizeEmail(value string) (string, error) {
	value = strings.ToLower(value)
	parts := strings.Split(value, "@")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid email")
	}
	local := strings.SplitN(parts[0], "+", 2)[0]
	local = strings.ReplaceAll(local, ".", "")
	normalized := local + "@" + parts[1]
	if !emailPattern.MatchString(normalized) {
		return "", fmt.Errorf("invalid email")
	}
	return normalized, nil
}

func normalizePhone(value string) (string, error) {
	value = extensionPattern.ReplaceAllString(value, "")
	var digits strings.Builder
	for _, character := range value {
		if character >= '0' && character <= '9' {
			digits.WriteRune(character)
		}
	}
	if digits.Len() < 7 || digits.Len() > 15 {
		return "", fmt.Errorf("invalid phone")
	}
	return "+" + digits.String(), nil
}

func normalizeDataProcessing(input *DataProcessingOptions) (*DataProcessingOptions, error) {
	if input == nil {
		return nil, nil
	}
	country := strings.ToUpper(strings.TrimSpace(input.Country))
	region := strings.ToUpper(strings.TrimSpace(input.Region))
	if len(input.Modes) != 1 || input.Modes[0] != "LDU" || len(country) != 2 || !asciiLetters(country) ||
		region != "" && (len(region) > 8 || !regionCode(region, country)) {
		return nil, fmt.Errorf("user.data_processing_options requires mode LDU, country, and an optional matching region")
	}
	return &DataProcessingOptions{Country: country, Region: region, Modes: []string{"LDU"}}, nil
}

func normalizeScreenDimensions(input *ScreenDimensions) (*ScreenDimensions, error) {
	if input == nil {
		return nil, nil
	}
	if input.Height < 0 || input.Height > 32767 || input.Width < 0 || input.Width > 32767 {
		return nil, fmt.Errorf("user.screen_dimensions must be between 0 and 32767")
	}
	copy := *input
	return &copy, nil
}

func asciiLetters(value string) bool {
	for _, character := range value {
		if character < 'A' || character > 'Z' {
			return false
		}
	}
	return true
}

func regionCode(value, country string) bool {
	for _, character := range value {
		if (character < 'A' || character > 'Z') && (character < '0' || character > '9') && character != '-' {
			return false
		}
	}
	return !strings.Contains(value, "-") || strings.HasPrefix(value, country+"-")
}

package conversions

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode"
)

const maximumUnixSeconds int64 = 253402300799

type wireEnvelope struct {
	EventSource   EventSource `json:"event_source"`
	EventSourceID string      `json:"event_source_id"`
	TestEventCode string      `json:"test_event_code,omitempty"`
	Data          []wireEvent `json:"data"`
}

type wireEvent struct {
	Event          EventName       `json:"event"`
	EventTime      int64           `json:"event_time"`
	EventID        string          `json:"event_id,omitempty"`
	User           *wireUser       `json:"user,omitempty"`
	Properties     *wireProperties `json:"properties,omitempty"`
	Page           *Page           `json:"page,omitempty"`
	App            *App            `json:"app,omitempty"`
	Ad             *Ad             `json:"ad,omitempty"`
	LimitedDataUse *bool           `json:"limited_data_use,omitempty"`
	Lead           *Lead           `json:"lead,omitempty"`
}

type wireUser struct {
	TikTokClickID string    `json:"ttclid,omitempty"`
	Emails        []string  `json:"email,omitempty"`
	Phones        []string  `json:"phone,omitempty"`
	ExternalIDs   []string  `json:"external_id,omitempty"`
	TikTokCookie  string    `json:"ttp,omitempty"`
	IP            string    `json:"ip,omitempty"`
	UserAgent     string    `json:"user_agent,omitempty"`
	FirstName     string    `json:"first_name,omitempty"`
	LastName      string    `json:"last_name,omitempty"`
	City          string    `json:"city,omitempty"`
	State         string    `json:"state,omitempty"`
	Country       string    `json:"country,omitempty"`
	ZipCode       string    `json:"zip_code,omitempty"`
	IDFA          string    `json:"idfa,omitempty"`
	IDFV          string    `json:"idfv,omitempty"`
	GAID          string    `json:"gaid,omitempty"`
	Locale        string    `json:"locale,omitempty"`
	ATTStatus     ATTStatus `json:"att_status,omitempty"`
}

type wireProperties struct {
	ContentIDs   []string      `json:"content_ids,omitempty"`
	Contents     []wireContent `json:"contents,omitempty"`
	ContentType  ContentType   `json:"content_type,omitempty"`
	Currency     string        `json:"currency,omitempty"`
	Value        Decimal       `json:"value,omitempty"`
	NumItems     *int64        `json:"num_items,omitempty"`
	SearchString string        `json:"search_string,omitempty"`
	Description  string        `json:"description,omitempty"`
	OrderID      string        `json:"order_id,omitempty"`
	ShopID       string        `json:"shop_id,omitempty"`
	CustomerType CustomerType  `json:"customer_type,omitempty"`
}

type wireContent struct {
	Price           Decimal `json:"price,omitempty"`
	Quantity        *int64  `json:"quantity,omitempty"`
	ContentID       string  `json:"content_id,omitempty"`
	ContentCategory string  `json:"content_category,omitempty"`
	ContentName     string  `json:"content_name,omitempty"`
	Brand           string  `json:"brand,omitempty"`
}

func normalizeRequest(source EventSource, sourceID string, input SubmitEventsRequest) (wireEnvelope, error) {
	if !validOptionalOpaque(input.TestEventCode, 512) {
		return wireEnvelope{}, fmt.Errorf("test_event_code is invalid")
	}
	if len(input.Events) == 0 || len(input.Events) > MaximumBatchSize {
		return wireEnvelope{}, fmt.Errorf("events must contain between 1 and %d entries", MaximumBatchSize)
	}
	output := wireEnvelope{
		EventSource: source, EventSourceID: sourceID, TestEventCode: input.TestEventCode,
		Data: make([]wireEvent, len(input.Events)),
	}
	for index := range input.Events {
		event, err := normalizeEvent(source, input.Events[index])
		if err != nil {
			return wireEnvelope{}, fmt.Errorf("events[%d]: %w", index, err)
		}
		output.Data[index] = event
	}
	return output, nil
}

func normalizeEvent(source EventSource, input ConversionEvent) (wireEvent, error) {
	if !validText(string(input.Event), 100) || input.EventTime <= 0 || input.EventTime > maximumUnixSeconds ||
		!validOptionalOpaque(input.EventID, 4096) {
		return wireEvent{}, fmt.Errorf("event, event_time, or event_id is invalid")
	}
	if source == EventSourceOffline && !isWebStandardEvent(input.Event) {
		return wireEvent{}, fmt.Errorf("offline events require a supported Standard Event name")
	}
	if source == EventSourceApp && input.EventID != "" {
		return wireEvent{}, fmt.Errorf("event_id is not supported for app events")
	}
	user, err := normalizeUser(source, input.User)
	if err != nil {
		return wireEvent{}, err
	}
	properties, err := normalizeProperties(source, input.Properties)
	if err != nil {
		return wireEvent{}, err
	}
	page, err := normalizePage(source, input.Page)
	if err != nil {
		return wireEvent{}, err
	}
	app, err := normalizeApp(source, input.App)
	if err != nil {
		return wireEvent{}, err
	}
	ad, err := normalizeAd(source, input.Ad)
	if err != nil {
		return wireEvent{}, err
	}
	lead, err := normalizeLead(source, input.Lead)
	if err != nil {
		return wireEvent{}, err
	}
	if input.LimitedDataUse != nil && source != EventSourceWeb && source != EventSourceApp {
		return wireEvent{}, fmt.Errorf("limited_data_use is only supported for web and app events")
	}
	if input.LimitedDataUse != nil && *input.LimitedDataUse && (user == nil || user.IP == "") {
		return wireEvent{}, fmt.Errorf("limited_data_use requires user.ip")
	}
	return wireEvent{
		Event: input.Event, EventTime: input.EventTime, EventID: input.EventID,
		User: user, Properties: properties, Page: page, App: app, Ad: ad,
		LimitedDataUse: input.LimitedDataUse, Lead: lead,
	}, nil
}

func normalizeUser(source EventSource, input *User) (*wireUser, error) {
	if input == nil {
		return nil, nil
	}
	if source != EventSourceWeb && (input.TikTokClickID != "" || input.TikTokCookie != "" || input.FirstName != "" ||
		input.LastName != "" || input.City != "" || input.State != "" || input.Country != "" || input.ZipCode != "") {
		return nil, fmt.Errorf("web-only user fields were supplied for %s events", source)
	}
	if source != EventSourceWeb && source != EventSourceCRM && len(input.ExternalIDs) > 0 {
		return nil, fmt.Errorf("external_id is only supported for web and CRM events")
	}
	if source != EventSourceWeb && source != EventSourceApp && (input.IP != "" || input.UserAgent != "" || input.Locale != "") {
		return nil, fmt.Errorf("IP, user_agent, and locale are only supported for web and app events")
	}
	if source != EventSourceApp && (input.IDFA != "" || input.IDFV != "" || input.GAID != "" || input.ATTStatus != "") {
		return nil, fmt.Errorf("mobile identifiers and att_status are only supported for app events")
	}
	emails, err := normalizeHashedList(input.Emails, hashEmail, "user.email")
	if err != nil {
		return nil, err
	}
	phones, err := normalizeHashedList(input.Phones, hashPhone, "user.phone")
	if err != nil {
		return nil, err
	}
	externalIDs, err := normalizeHashedList(input.ExternalIDs, hashExternalID, "user.external_id")
	if err != nil {
		return nil, err
	}
	country := strings.ToLower(strings.TrimSpace(input.Country))
	if country != "" && (len(country) != 2 || country[0] < 'a' || country[0] > 'z' || country[1] < 'a' || country[1] > 'z') {
		return nil, fmt.Errorf("user.country is invalid")
	}
	firstName, err := normalizeOptionalHash(input.FirstName, hashName)
	if err != nil {
		return nil, fmt.Errorf("user.first_name is invalid")
	}
	lastName, err := normalizeOptionalHash(input.LastName, hashName)
	if err != nil {
		return nil, fmt.Errorf("user.last_name is invalid")
	}
	zip, err := normalizeZip(input.ZipCode, country)
	if err != nil {
		return nil, fmt.Errorf("user.zip_code is invalid")
	}
	city := compactLower(input.City)
	if input.City != "" && city == "" {
		return nil, fmt.Errorf("user.city is invalid")
	}
	state := compactLower(input.State)
	if input.State != "" && (state == "" || country == "us" && (len(state) != 2 || state[0] < 'a' || state[0] > 'z' || state[1] < 'a' || state[1] > 'z')) {
		return nil, fmt.Errorf("user.state is invalid")
	}
	if !validOptionalOpaque(input.TikTokClickID, 4096) || !validOptionalOpaque(input.TikTokCookie, 4096) ||
		!validOptionalOpaque(input.UserAgent, 16_384) || !validLocale(input.Locale) || !validATTStatus(input.ATTStatus) {
		return nil, fmt.Errorf("user contains an invalid click, cookie, user-agent, locale, or ATT field")
	}
	if input.IP != "" && !validPublicIP(input.IP) {
		return nil, fmt.Errorf("user.ip must be a public IPv4 or IPv6 address")
	}
	idfa, err := normalizeIDFA(input.IDFA)
	if err != nil {
		return nil, fmt.Errorf("user.idfa is invalid")
	}
	idfv := strings.TrimSpace(input.IDFV)
	if idfv != "" && !validUUID(idfv) {
		return nil, fmt.Errorf("user.idfv is invalid")
	}
	gaid, err := normalizeGAID(input.GAID)
	if err != nil {
		return nil, fmt.Errorf("user.gaid is invalid")
	}
	return &wireUser{
		TikTokClickID: input.TikTokClickID, Emails: emails, Phones: phones, ExternalIDs: externalIDs,
		TikTokCookie: input.TikTokCookie, IP: input.IP, UserAgent: input.UserAgent,
		FirstName: firstName, LastName: lastName, City: city, State: state, Country: country, ZipCode: zip,
		IDFA: idfa, IDFV: idfv, GAID: gaid, Locale: input.Locale, ATTStatus: input.ATTStatus,
	}, nil
}

type hashKind uint8

const (
	hashEmail hashKind = iota
	hashPhone
	hashExternalID
	hashName
)

func normalizeHashedList(values []string, kind hashKind, path string) ([]string, error) {
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
		if _, found := seen[hashed]; found {
			continue
		}
		seen[hashed] = struct{}{}
		output = append(output, hashed)
	}
	return output, nil
}

func normalizeOptionalHash(value string, kind hashKind) (string, error) {
	if value == "" {
		return "", nil
	}
	return normalizeAndHash(value, kind)
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
		normalized = strings.ToLower(trimmed)
		if !emailPattern.MatchString(normalized) {
			return "", fmt.Errorf("invalid email")
		}
	case hashPhone:
		if len(trimmed) < 8 || len(trimmed) > 16 || trimmed[0] != '+' {
			return "", fmt.Errorf("invalid E.164 phone")
		}
		for _, character := range trimmed[1:] {
			if character < '0' || character > '9' {
				return "", fmt.Errorf("invalid E.164 phone")
			}
		}
		normalized = trimmed
	case hashExternalID:
		normalized = trimmed
	case hashName:
		normalized = strings.Map(func(character rune) rune {
			if unicode.IsPunct(character) {
				return -1
			}
			return unicode.ToLower(character)
		}, trimmed)
		if strings.TrimSpace(normalized) == "" {
			return "", fmt.Errorf("invalid name")
		}
	default:
		return "", fmt.Errorf("unsupported identifier")
	}
	digest := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(digest[:]), nil
}

func normalizeZip(value, country string) (string, error) {
	if value == "" {
		return "", nil
	}
	trimmed := strings.TrimSpace(value)
	if lowerSHA256.MatchString(trimmed) {
		return trimmed, nil
	}
	if anySHA256.MatchString(trimmed) || legacyMD5.MatchString(trimmed) {
		return "", fmt.Errorf("unsupported digest")
	}
	normalized := strings.Map(func(character rune) rune {
		if unicode.IsSpace(character) || character == '-' {
			return -1
		}
		return unicode.ToLower(character)
	}, trimmed)
	if country == "us" {
		if len(normalized) < 5 {
			return "", fmt.Errorf("invalid US ZIP")
		}
		normalized = normalized[:5]
		for _, character := range normalized {
			if character < '0' || character > '9' {
				return "", fmt.Errorf("invalid US ZIP")
			}
		}
	}
	if !validOpaque(normalized, 64) {
		return "", fmt.Errorf("invalid ZIP")
	}
	digest := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(digest[:]), nil
}

func normalizeIDFA(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || lowerSHA256.MatchString(trimmed) {
		return trimmed, nil
	}
	if anySHA256.MatchString(trimmed) || !validUUID(trimmed) || trimmed != strings.ToUpper(trimmed) {
		return "", fmt.Errorf("invalid IDFA")
	}
	return trimmed, nil
}

func normalizeGAID(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || lowerSHA256.MatchString(trimmed) {
		return trimmed, nil
	}
	if anySHA256.MatchString(trimmed) || !validUUID(trimmed) || trimmed != strings.ToLower(trimmed) {
		return "", fmt.Errorf("invalid GAID")
	}
	return trimmed, nil
}

func normalizeProperties(source EventSource, input *Properties) (*wireProperties, error) {
	if input == nil {
		return nil, nil
	}
	if len(input.ContentIDs) > MaximumBatchSize || len(input.Contents) > MaximumBatchSize {
		return nil, fmt.Errorf("properties content arrays are too large")
	}
	contentIDs := append([]string(nil), input.ContentIDs...)
	for index, value := range contentIDs {
		if !validText(value, 4096) {
			return nil, fmt.Errorf("properties.content_ids[%d] is invalid", index)
		}
	}
	contents := make([]wireContent, len(input.Contents))
	for index, item := range input.Contents {
		if !validOptionalText(item.ContentID, 4096) || !validOptionalText(item.ContentCategory, 4096) ||
			!validOptionalText(item.ContentName, 4096) || !validOptionalText(item.Brand, 4096) ||
			item.Price != "" && !validDecimal(item.Price) || item.Quantity != nil && *item.Quantity < 0 ||
			item.Price == "" && item.Quantity == nil && item.ContentID == "" && item.ContentCategory == "" &&
				item.ContentName == "" && item.Brand == "" {
			return nil, fmt.Errorf("properties.contents[%d] is invalid", index)
		}
		contents[index] = wireContent{
			Price: item.Price, Quantity: item.Quantity, ContentID: item.ContentID,
			ContentCategory: item.ContentCategory, ContentName: item.ContentName, Brand: item.Brand,
		}
	}
	currency := strings.ToUpper(strings.TrimSpace(input.Currency))
	if currency != "" && !validCurrency(currency) || input.Value != "" && (!validDecimal(input.Value) || currency == "") ||
		input.NumItems != nil && *input.NumItems < 0 || !validContentType(input.ContentType) ||
		!validCustomerType(input.CustomerType) || input.CustomerType != "" && source != EventSourceWeb && source != EventSourceApp {
		return nil, fmt.Errorf("properties money, count, type, or customer fields are invalid")
	}
	for _, value := range []string{input.SearchString, input.Description, input.OrderID, input.ShopID} {
		if !validOptionalText(value, 4096) {
			return nil, fmt.Errorf("properties contains an invalid text field")
		}
	}
	return &wireProperties{
		ContentIDs: contentIDs, Contents: contents, ContentType: input.ContentType,
		Currency: currency, Value: input.Value, NumItems: input.NumItems,
		SearchString: input.SearchString, Description: input.Description,
		OrderID: input.OrderID, ShopID: input.ShopID, CustomerType: input.CustomerType,
	}, nil
}

func normalizePage(source EventSource, input *Page) (*Page, error) {
	if source == EventSourceWeb {
		if input == nil || !validHTTPURL(input.URL) || input.Referrer != "" && !validHTTPURL(input.Referrer) {
			return nil, fmt.Errorf("page.url is required and page URLs must be absolute HTTP(S) URLs for web events")
		}
		copy := *input
		return &copy, nil
	}
	if input != nil {
		return nil, fmt.Errorf("page is only supported for web events")
	}
	return nil, nil
}

func normalizeApp(source EventSource, input *App) (*App, error) {
	if source == EventSourceApp {
		if input == nil || !validText(input.AppID, 512) || !validOptionalText(input.AppName, 512) ||
			!validOptionalText(input.AppVersion, 128) {
			return nil, fmt.Errorf("app.app_id is required and app fields must be valid for app events")
		}
		copy := *input
		return &copy, nil
	}
	if input != nil {
		return nil, fmt.Errorf("app is only supported for app events")
	}
	return nil, nil
}

func normalizeAd(source EventSource, input *Ad) (*Ad, error) {
	if input == nil {
		return nil, nil
	}
	if source != EventSourceWeb && source != EventSourceApp {
		return nil, fmt.Errorf("ad is only supported for web and app events")
	}
	if source != EventSourceApp && (input.Callback != "" || input.IsRetargeting != nil || input.Attributed != nil) {
		return nil, fmt.Errorf("ad callback, is_retargeting, and attributed are app-only fields")
	}
	for _, value := range []string{
		input.Callback, input.CampaignID, input.AdID, input.CreativeID, input.AttributionType,
		input.AttributionProvider, input.AttributionModel, input.TouchpointType, input.AttributionMethod,
		input.DeclineReason, input.UTMID, input.UTMSource, input.UTMMedium, input.UTMCampaign,
	} {
		if !validOptionalText(value, 4096) {
			return nil, fmt.Errorf("ad contains an invalid text field")
		}
	}
	if input.AttributionShare != "" && !decimalAtMostOne(input.AttributionShare) ||
		input.AttributionValue != "" && !validDecimal(input.AttributionValue) ||
		input.TouchpointTime != nil && (*input.TouchpointTime <= 0 || *input.TouchpointTime > maximumUnixSeconds) ||
		input.ClickAttributionWindowHR != nil && *input.ClickAttributionWindowHR < 0 ||
		input.ViewAttributionWindowHR != nil && *input.ViewAttributionWindowHR < 0 ||
		input.TouchpointURL != "" && !validHTTPURL(input.TouchpointURL) {
		return nil, fmt.Errorf("ad contains an invalid attribution value, time, window, or URL")
	}
	copy := *input
	return &copy, nil
}

func normalizeLead(source EventSource, input *Lead) (*Lead, error) {
	if source == EventSourceCRM {
		if input == nil || !validText(input.LeadID, 4096) || !validOptionalText(input.LeadEventSource, 4096) {
			return nil, fmt.Errorf("lead.lead_id is required for CRM events")
		}
		copy := *input
		return &copy, nil
	}
	if input != nil {
		return nil, fmt.Errorf("lead is only supported for CRM events")
	}
	return nil, nil
}

func compactLower(value string) string {
	return strings.Map(func(character rune) rune {
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			return unicode.ToLower(character)
		}
		return -1
	}, strings.TrimSpace(value))
}

func isWebStandardEvent(value EventName) bool {
	switch value {
	case EventAddPaymentInfo, EventAddToCart, EventAddToWishlist, EventApplicationApproval,
		EventCompleteRegistration, EventContact, EventCustomizeProduct, EventDownload,
		EventFindLocation, EventInitiateCheckout, EventLead, EventPurchase, EventSchedule,
		EventSearch, EventStartTrial, EventSubmitApplication, EventSubscribe, EventViewContent:
		return true
	default:
		return false
	}
}

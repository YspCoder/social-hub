package airbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	sendMobileEventOperation = "events.send_mobile"
	sendWebEventOperation    = "events.send_web"
)

type wireMobileEventRequest struct {
	EventUUID      string              `json:"eventUUID,omitempty"`
	EventTimestamp *int64              `json:"eventTimestamp,omitempty"`
	User           *wireUser           `json:"user,omitempty"`
	Device         *wireDevice         `json:"device,omitempty"`
	App            wireApp             `json:"app"`
	EventData      wireMobileEventData `json:"eventData"`
}

type wireWebEventRequest struct {
	EventUUID      string           `json:"eventUUID,omitempty"`
	EventTimestamp *int64           `json:"eventTimestamp,omitempty"`
	User           *wireUser        `json:"user,omitempty"`
	Browser        *wireBrowser     `json:"browser,omitempty"`
	EventData      wireWebEventData `json:"eventData"`
}

type wireUser struct {
	ExternalUserID    string         `json:"externalUserID,omitempty"`
	ExternalUserEmail string         `json:"externalUserEmail,omitempty"`
	ExternalUserPhone string         `json:"externalUserPhone,omitempty"`
	Attributes        map[string]any `json:"attributes,omitempty"`
}

type wireDevice struct {
	DeviceUUID              string        `json:"deviceUUID,omitempty"`
	GAID                    string        `json:"gaid,omitempty"`
	IFA                     string        `json:"ifa,omitempty"`
	AppSetID                string        `json:"appSetID,omitempty"`
	IFV                     string        `json:"ifv,omitempty"`
	ClientIP                string        `json:"clientIP,omitempty"`
	LimitAdTracking         *bool         `json:"limitAdTracking,omitempty"`
	DeviceModel             string        `json:"deviceModel,omitempty"`
	AppTrackingTransparency *int          `json:"appTrackingTransparency,omitempty"`
	DeviceIdentifier        string        `json:"deviceIdentifier,omitempty"`
	Manufacturer            string        `json:"manufacturer,omitempty"`
	OSName                  OSName        `json:"osName,omitempty"`
	OSVersion               string        `json:"osVersion,omitempty"`
	Locale                  string        `json:"locale,omitempty"`
	Timezone                string        `json:"timezone,omitempty"`
	Orientation             string        `json:"orientation,omitempty"`
	Screen                  *wireScreen   `json:"screen,omitempty"`
	Location                *wireLocation `json:"location,omitempty"`
	Network                 *wireNetwork  `json:"network,omitempty"`
	Alias                   *wireDMA      `json:"alias,omitempty"`
}

type wireScreen struct {
	Density string `json:"density,omitempty"`
	Height  *int64 `json:"height,omitempty"`
	Width   *int64 `json:"width,omitempty"`
}

type wireLocation struct {
	Latitude  Decimal `json:"latitude,omitempty"`
	Longitude Decimal `json:"longitude,omitempty"`
	Speed     string  `json:"speed,omitempty"`
}

type wireNetwork struct {
	Carrier  string `json:"carrier,omitempty"`
	Cellular *bool  `json:"cellular,omitempty"`
	WiFi     *bool  `json:"wifi,omitempty"`
}

type wireDMA struct {
	EEA               *string `json:"eea,omitempty"`
	AdPersonalization *string `json:"adPersonalization,omitempty"`
	AdUserData        *string `json:"adUserData,omitempty"`
}

type wireBrowser struct {
	ClientID  string `json:"clientID,omitempty"`
	UserAgent string `json:"userAgent,omitempty"`
}

type wireApp struct {
	PackageName string `json:"packageName"`
	Version     string `json:"version,omitempty"`
}

type wireMobileEventData struct {
	Goal wireGoal `json:"goal"`
}

type wireWebEventData struct {
	ShortID      string            `json:"shortID,omitempty"`
	TrackingData *wireTrackingData `json:"trackingData,omitempty"`
	Goal         wireGoal          `json:"goal"`
}

type wireTrackingData struct {
	Channel string         `json:"channel,omitempty"`
	Params  map[string]any `json:"params"`
}

type wireGoal struct {
	Category           EventCategory           `json:"category"`
	Value              Decimal                 `json:"value,omitempty"`
	CustomAttributes   map[string]any          `json:"customAttributes,omitempty"`
	SemanticAttributes *wireSemanticAttributes `json:"semanticAttributes,omitempty"`
}

type wireSemanticAttributes struct {
	Action                          string                    `json:"action,omitempty"`
	Label                           string                    `json:"label,omitempty"`
	TotalValue                      Decimal                   `json:"totalValue,omitempty"`
	OriginalTotalValue              Decimal                   `json:"originalTotalValue,omitempty"`
	Currency                        string                    `json:"currency,omitempty"`
	OriginalCurrency                string                    `json:"originalCurrency,omitempty"`
	Products                        []wireProduct             `json:"products,omitempty"`
	Period                          string                    `json:"period,omitempty"`
	IsRenewal                       *bool                     `json:"isRenewal,omitempty"`
	RenewalCount                    *int64                    `json:"renewalCount,omitempty"`
	ProductListID                   string                    `json:"productListID,omitempty"`
	CartID                          string                    `json:"cartID,omitempty"`
	TransactionID                   string                    `json:"transactionID,omitempty"`
	TransactionType                 string                    `json:"transactionType,omitempty"`
	TransactionPairedEventCategory  string                    `json:"transactionPairedEventCategory,omitempty"`
	TransactionPairedEventTimestamp *int64                    `json:"transactionPairedEventTimestamp,omitempty"`
	TotalQuantity                   *int64                    `json:"totalQuantity,omitempty"`
	Query                           string                    `json:"query,omitempty"`
	WishListID                      string                    `json:"wishListID,omitempty"`
	ContentID                       string                    `json:"contentID,omitempty"`
	ContentName                     string                    `json:"contentName,omitempty"`
	InAppPurchased                  *bool                     `json:"inAppPurchased,omitempty"`
	ContributionMargin              Decimal                   `json:"contributionMargin,omitempty"`
	OriginalContributionMargin      Decimal                   `json:"originalContributionMargin,omitempty"`
	ListID                          string                    `json:"listID,omitempty"`
	RateID                          string                    `json:"rateID,omitempty"`
	Rate                            Decimal                   `json:"rate,omitempty"`
	MaxRate                         Decimal                   `json:"maxRate,omitempty"`
	RatingValue                     Decimal                   `json:"ratingValue,omitempty"`
	MaxRatingValue                  Decimal                   `json:"maxRatingValue,omitempty"`
	AchievementID                   string                    `json:"achievementID,omitempty"`
	SharedChannel                   string                    `json:"sharedChannel,omitempty"`
	Datetime                        string                    `json:"datetime,omitempty"`
	Description                     string                    `json:"description,omitempty"`
	IsRevenue                       *bool                     `json:"isRevenue,omitempty"`
	Place                           string                    `json:"place,omitempty"`
	ScheduleID                      string                    `json:"scheduleID,omitempty"`
	Type                            string                    `json:"type,omitempty"`
	Level                           string                    `json:"level,omitempty"`
	Score                           Decimal                   `json:"score,omitempty"`
	AdPartners                      map[string]map[string]any `json:"adPartners,omitempty"`
	IsFirstPerUser                  *bool                     `json:"isFirstPerUser,omitempty"`
}

type wireProduct struct {
	ProductID    string  `json:"productID,omitempty"`
	Name         string  `json:"name,omitempty"`
	Price        Decimal `json:"price,omitempty"`
	Quantity     *int64  `json:"quantity,omitempty"`
	Currency     string  `json:"currency,omitempty"`
	Position     *int64  `json:"position,omitempty"`
	CategoryID   string  `json:"categoryID,omitempty"`
	CategoryName string  `json:"categoryName,omitempty"`
	BrandID      string  `json:"brandID,omitempty"`
	BrandName    string  `json:"brandName,omitempty"`
}

type eventResponse struct {
	At   string `json:"at"`
	Data string `json:"data"`
}

func (client *Client) SendMobileEvent(ctx context.Context, input MobileEventRequest, options ...socialhub.CallOption) (SubmitResult, error) {
	callOptions, err := eventCallOptions(sendMobileEventOperation, options)
	if err != nil {
		return SubmitResult{}, err
	}
	if err := validateMobileEvent(input, client.clock.Now()); err != nil {
		return SubmitResult{}, invalidArgument(sendMobileEventOperation, err.Error())
	}
	payload := wireMobileEventRequest{
		EventUUID: input.EventUUID, EventTimestamp: unixMilliseconds(input.EventTimestamp),
		User: normalizeUser(input.User), Device: normalizeDevice(input.Device),
		App:       wireApp{PackageName: input.App.PackageName, Version: input.App.Version},
		EventData: wireMobileEventData{Goal: normalizeGoal(input.Goal)},
	}
	return client.submit(ctx, sendMobileEventOperation, "/events/v2/apps/"+client.appName+"/mobile-app/9360",
		input.ForwardedFor, input.Device.ClientIP != "", input.AcceptLanguage, payload, callOptions...)
}

func (client *Client) SendWebEvent(ctx context.Context, input WebEventRequest, options ...socialhub.CallOption) (SubmitResult, error) {
	callOptions, err := eventCallOptions(sendWebEventOperation, options)
	if err != nil {
		return SubmitResult{}, err
	}
	if err := validateWebEvent(input, client.clock.Now()); err != nil {
		return SubmitResult{}, invalidArgument(sendWebEventOperation, err.Error())
	}
	payload := wireWebEventRequest{
		EventUUID: input.EventUUID, EventTimestamp: unixMilliseconds(input.EventTimestamp),
		User: normalizeUser(input.User), Browser: normalizeBrowser(input.Browser),
		EventData: wireWebEventData{ShortID: input.ShortID, TrackingData: normalizeTracking(input.Tracking), Goal: normalizeGoal(input.Goal)},
	}
	return client.submit(ctx, sendWebEventOperation, "/events/v2/apps/"+client.appName+"/web/9320",
		input.ForwardedFor, false, input.AcceptLanguage, payload, callOptions...)
}

func (client *Client) submit(ctx context.Context, operation, path, forwardedFor string, useClientIP bool, acceptLanguage string, payload any, options ...socialhub.CallOption) (SubmitResult, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return SubmitResult{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request, err := client.api.NewRequest(ctx, http.MethodPost, path, nil, bytes.NewReader(encoded), options...)
	if err != nil {
		return SubmitResult{}, withOperation(err, operation)
	}
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	if forwardedFor != "" {
		request.Header.Set("X-Forwarded-For", forwardedFor)
	}
	if useClientIP {
		request.Header.Set("x-airbridge-use-client-ip", "1")
	}
	if acceptLanguage != "" {
		request.Header.Set("Accept-Language", acceptLanguage)
	}
	var response eventResponse
	metadata, err := client.api.DoWithMetadata(request, &response)
	if err != nil {
		return SubmitResult{}, withOperation(err, operation)
	}
	if metadata.StatusCode != http.StatusOK {
		return SubmitResult{}, platformContractError(operation, "Airbridge returned an undocumented success status", metadata.StatusCode)
	}
	if response.At == "" || response.Data == "" {
		return SubmitResult{}, platformContractError(operation, "Airbridge returned an incomplete success response", metadata.StatusCode)
	}
	return SubmitResult{StatusCode: metadata.StatusCode, At: response.At, Data: response.Data}, nil
}

func eventCallOptions(operation string, options []socialhub.CallOption) ([]socialhub.CallOption, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return nil, withOperation(err, operation)
	}
	if resolved.RequestID != "" {
		return nil, invalidArgument(operation, "Airbridge S2S Events API v2 does not document caller-supplied request IDs")
	}
	if resolved.IdempotencyKey != "" {
		return nil, invalidArgument(operation, "Airbridge uses request.event_uuid UUIDv4 for deduplication and does not document an idempotency header")
	}
	if len(resolved.Fields) != 0 {
		return nil, invalidArgument(operation, "Airbridge event responses do not support field selection")
	}
	if resolved.Timeout == 0 {
		return nil, nil
	}
	return []socialhub.CallOption{socialhub.WithCallTimeout(resolved.Timeout)}, nil
}

func unixMilliseconds(value *time.Time) *int64 {
	if value == nil {
		return nil
	}
	milliseconds := value.UnixMilli()
	return &milliseconds
}

func normalizeUser(input User) *wireUser {
	if userEmpty(input) {
		return nil
	}
	return &wireUser{
		ExternalUserID: input.ExternalUserID, ExternalUserEmail: input.ExternalUserEmail,
		ExternalUserPhone: input.ExternalUserPhone, Attributes: normalizeProperties(input.Attributes),
	}
}

func normalizeDevice(input Device) *wireDevice {
	if deviceEmpty(input) {
		return nil
	}
	output := &wireDevice{
		DeviceUUID: input.DeviceUUID, GAID: input.GAID, IFA: input.IFA, AppSetID: input.AppSetID, IFV: input.IFV,
		ClientIP: input.ClientIP, LimitAdTracking: input.LimitAdTracking, DeviceModel: input.DeviceModel,
		AppTrackingTransparency: input.AppTrackingTransparency, DeviceIdentifier: input.DeviceIdentifier,
		Manufacturer: input.Manufacturer, OSName: input.OSName, OSVersion: input.OSVersion,
		Locale: input.Locale, Timezone: input.Timezone, Orientation: input.Orientation,
	}
	if input.Screen != nil {
		output.Screen = &wireScreen{Density: input.Screen.Density, Height: input.Screen.Height, Width: input.Screen.Width}
	}
	if input.Location != nil {
		output.Location = &wireLocation{Latitude: input.Location.Latitude, Longitude: input.Location.Longitude, Speed: input.Location.Speed}
	}
	if input.Network != nil {
		output.Network = &wireNetwork{Carrier: input.Network.Carrier, Cellular: input.Network.Cellular, WiFi: input.Network.WiFi}
	}
	if input.DMA != nil {
		output.Alias = &wireDMA{
			EEA: boolString(input.DMA.EEA), AdPersonalization: boolString(input.DMA.AdPersonalization),
			AdUserData: boolString(input.DMA.AdUserData),
		}
	}
	return output
}

func normalizeBrowser(input Browser) *wireBrowser {
	if input.ClientID == "" && input.UserAgent == "" {
		return nil
	}
	return &wireBrowser{ClientID: input.ClientID, UserAgent: input.UserAgent}
}

func normalizeTracking(input *TrackingData) *wireTrackingData {
	if input == nil {
		return nil
	}
	params := normalizeProperties(input.Params)
	if params == nil {
		params = map[string]any{}
	}
	return &wireTrackingData{Channel: input.Channel, Params: params}
}

func normalizeGoal(input Goal) wireGoal {
	return wireGoal{
		Category: input.Category, Value: input.Value, CustomAttributes: normalizeProperties(input.CustomAttributes),
		SemanticAttributes: normalizeSemanticAttributes(input.SemanticAttributes),
	}
}

func normalizeSemanticAttributes(input SemanticAttributes) *wireSemanticAttributes {
	if semanticAttributesEmpty(input) {
		return nil
	}
	return &wireSemanticAttributes{
		Action: input.Action, Label: input.Label, TotalValue: input.TotalValue, OriginalTotalValue: input.OriginalTotalValue,
		Currency: input.Currency, OriginalCurrency: input.OriginalCurrency, Products: normalizeProducts(input.Products),
		Period: input.Period, IsRenewal: input.IsRenewal, RenewalCount: input.RenewalCount,
		ProductListID: input.ProductListID, CartID: input.CartID, TransactionID: input.TransactionID,
		TransactionType: input.TransactionType, TransactionPairedEventCategory: input.TransactionPairedEventCategory,
		TransactionPairedEventTimestamp: input.TransactionPairedEventTimestamp, TotalQuantity: input.TotalQuantity,
		Query: input.Query, WishListID: input.WishListID, ContentID: input.ContentID, ContentName: input.ContentName,
		InAppPurchased: input.InAppPurchased, ContributionMargin: input.ContributionMargin,
		OriginalContributionMargin: input.OriginalContributionMargin, ListID: input.ListID, RateID: input.RateID,
		Rate: input.Rate, MaxRate: input.MaxRate, RatingValue: input.RatingValue, MaxRatingValue: input.MaxRatingValue,
		AchievementID: input.AchievementID, SharedChannel: input.SharedChannel, Datetime: input.Datetime,
		Description: input.Description, IsRevenue: input.IsRevenue, Place: input.Place, ScheduleID: input.ScheduleID,
		Type: input.Type, Level: input.Level, Score: input.Score, AdPartners: normalizeAdPartners(input.AdPartners),
		IsFirstPerUser: input.IsFirstPerUser,
	}
}

func normalizeProducts(input []Product) []wireProduct {
	if len(input) == 0 {
		return nil
	}
	output := make([]wireProduct, len(input))
	for index, product := range input {
		output[index] = wireProduct{
			ProductID: product.ProductID, Name: product.Name, Price: product.Price, Quantity: product.Quantity,
			Currency: product.Currency, Position: product.Position, CategoryID: product.CategoryID,
			CategoryName: product.CategoryName, BrandID: product.BrandID, BrandName: product.BrandName,
		}
	}
	return output
}

func normalizeAdPartners(input map[string]Properties) map[string]map[string]any {
	if len(input) == 0 {
		return nil
	}
	output := make(map[string]map[string]any, len(input))
	for partner, values := range input {
		normalized := normalizeProperties(values)
		if normalized == nil {
			normalized = map[string]any{}
		}
		output[partner] = normalized
	}
	return output
}

func normalizeProperties(input Properties) map[string]any {
	if propertiesEmpty(input) {
		return nil
	}
	output := make(map[string]any, len(input.Strings)+len(input.Numbers)+len(input.Booleans))
	for key, value := range input.Strings {
		output[key] = value
	}
	for key, value := range input.Numbers {
		output[key] = value
	}
	for key, value := range input.Booleans {
		output[key] = value
	}
	return output
}

func boolString(value *bool) *string {
	if value == nil {
		return nil
	}
	encoded := "0"
	if *value {
		encoded = "1"
	}
	return &encoded
}

func propertiesEmpty(input Properties) bool {
	return len(input.Strings) == 0 && len(input.Numbers) == 0 && len(input.Booleans) == 0
}

func userEmpty(input User) bool {
	return input.ExternalUserID == "" && input.ExternalUserEmail == "" && input.ExternalUserPhone == "" && propertiesEmpty(input.Attributes)
}

func deviceEmpty(input Device) bool {
	return input.DeviceUUID == "" && input.GAID == "" && input.IFA == "" && input.AppSetID == "" && input.IFV == "" &&
		input.ClientIP == "" && input.LimitAdTracking == nil && input.DeviceModel == "" && input.AppTrackingTransparency == nil &&
		input.DeviceIdentifier == "" && input.Manufacturer == "" && input.OSName == "" && input.OSVersion == "" &&
		input.Locale == "" && input.Timezone == "" && input.Orientation == "" && input.Screen == nil && input.Location == nil &&
		input.Network == nil && input.DMA == nil
}

func semanticAttributesEmpty(input SemanticAttributes) bool {
	return input.Action == "" && input.Label == "" && input.TotalValue == "" && input.OriginalTotalValue == "" &&
		input.Currency == "" && input.OriginalCurrency == "" && len(input.Products) == 0 && input.Period == "" &&
		input.IsRenewal == nil && input.RenewalCount == nil && input.ProductListID == "" && input.CartID == "" &&
		input.TransactionID == "" && input.TransactionType == "" && input.TransactionPairedEventCategory == "" &&
		input.TransactionPairedEventTimestamp == nil && input.TotalQuantity == nil && input.Query == "" &&
		input.WishListID == "" && input.ContentID == "" && input.ContentName == "" && input.InAppPurchased == nil &&
		input.ContributionMargin == "" && input.OriginalContributionMargin == "" && input.ListID == "" &&
		input.RateID == "" && input.Rate == "" && input.MaxRate == "" && input.RatingValue == "" &&
		input.MaxRatingValue == "" && input.AchievementID == "" && input.SharedChannel == "" && input.Datetime == "" &&
		input.Description == "" && input.IsRevenue == nil && input.Place == "" && input.ScheduleID == "" &&
		input.Type == "" && input.Level == "" && input.Score == "" && len(input.AdPartners) == 0 && input.IsFirstPerUser == nil
}

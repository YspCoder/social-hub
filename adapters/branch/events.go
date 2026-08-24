package branch

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"social-hub/pkg/socialhub"
)

const (
	trackStandardOperation = "events.track_standard"
	trackCustomOperation   = "events.track_custom"
)

type wireEventRequest struct {
	BranchKey          string            `json:"branch_key"`
	Name               string            `json:"name"`
	CustomerEventAlias string            `json:"customer_event_alias,omitempty"`
	UserData           wireUserData      `json:"user_data"`
	CustomData         map[string]any    `json:"custom_data,omitempty"`
	EventData          *wireEventData    `json:"event_data,omitempty"`
	ContentItems       []wireContentItem `json:"content_items,omitempty"`
	Metadata           map[string]any    `json:"meta_data,omitempty"`
}

type wireUserData struct {
	OS                    OperatingSystem   `json:"os,omitempty"`
	OSVersion             string            `json:"os_version,omitempty"`
	AdvertisingIDs        map[string]string `json:"advertising_ids,omitempty"`
	Environment           Environment       `json:"environment,omitempty"`
	AAID                  string            `json:"aaid,omitempty"`
	AndroidID             string            `json:"android_id,omitempty"`
	IDFA                  string            `json:"idfa,omitempty"`
	IDFV                  string            `json:"idfv,omitempty"`
	AnonID                string            `json:"anon_id,omitempty"`
	LimitAdTracking       *bool             `json:"limit_ad_tracking,omitempty"`
	UserAgent             string            `json:"user_agent,omitempty"`
	BrowserFingerprintID  string            `json:"browser_fingerprint_id,omitempty"`
	HTTPOrigin            string            `json:"http_origin,omitempty"`
	HTTPReferrer          string            `json:"http_referrer,omitempty"`
	DeveloperIdentity     string            `json:"developer_identity,omitempty"`
	GoogleAnalyticsID     string            `json:"google_analytics_id,omitempty"`
	RandomizedDeviceToken string            `json:"randomized_device_token,omitempty"`
	Country               string            `json:"country,omitempty"`
	Language              string            `json:"language,omitempty"`
	IP                    string            `json:"ip,omitempty"`
	LocalIP               string            `json:"local_ip,omitempty"`
	Brand                 string            `json:"brand,omitempty"`
	AppVersion            string            `json:"app_version,omitempty"`
	Model                 string            `json:"model,omitempty"`
	ScreenDPI             *int64            `json:"screen_dpi,omitempty"`
	ScreenHeight          *int64            `json:"screen_height,omitempty"`
	ScreenWidth           *int64            `json:"screen_width,omitempty"`
	DMAEEA                *bool             `json:"dma_eea,omitempty"`
	DMAAdPersonalization  *bool             `json:"dma_ad_personalization,omitempty"`
	DMAAdUserData         *bool             `json:"dma_ad_user_data,omitempty"`
}

type wireEventData struct {
	TransactionID string  `json:"transaction_id,omitempty"`
	Revenue       Decimal `json:"revenue,omitempty"`
	Currency      string  `json:"currency,omitempty"`
	Shipping      Decimal `json:"shipping,omitempty"`
	Tax           Decimal `json:"tax,omitempty"`
	Coupon        string  `json:"coupon,omitempty"`
	Affiliation   string  `json:"affiliation,omitempty"`
	Description   string  `json:"description,omitempty"`
	SearchQuery   string  `json:"search_query,omitempty"`
}

type wireContentItem struct {
	Schema              ContentSchema    `json:"$content_schema,omitempty"`
	OGTitle             string           `json:"$og_title,omitempty"`
	OGDescription       string           `json:"$og_description,omitempty"`
	OGImageURL          string           `json:"$og_image_url,omitempty"`
	CanonicalIdentifier string           `json:"$canonical_identifier,omitempty"`
	PubliclyIndexable   *bool            `json:"$publicly_indexable,omitempty"`
	LocallyIndexable    *bool            `json:"$locally_indexable,omitempty"`
	Price               Decimal          `json:"$price,omitempty"`
	Quantity            Decimal          `json:"$quantity,omitempty"`
	SKU                 string           `json:"$sku,omitempty"`
	ProductName         string           `json:"$product_name,omitempty"`
	ProductBrand        string           `json:"$product_brand,omitempty"`
	ProductCategory     ProductCategory  `json:"$product_category,omitempty"`
	ProductVariant      string           `json:"$product_variant,omitempty"`
	RatingAverage       Decimal          `json:"$rating_average,omitempty"`
	RatingCount         Decimal          `json:"$rating_count,omitempty"`
	RatingMax           Decimal          `json:"$rating_max,omitempty"`
	CreationTimestamp   *int64           `json:"$creation_timestamp,omitempty"`
	ExpirationTimestamp *int64           `json:"$exp_date,omitempty"`
	Keywords            []string         `json:"$keywords,omitempty"`
	AddressStreet       string           `json:"$address_street,omitempty"`
	AddressCity         string           `json:"$address_city,omitempty"`
	AddressRegion       string           `json:"$address_region,omitempty"`
	AddressCountry      string           `json:"$address_country,omitempty"`
	AddressPostalCode   string           `json:"$address_postal_code,omitempty"`
	Latitude            Decimal          `json:"$latitude,omitempty"`
	Longitude           Decimal          `json:"$longitude,omitempty"`
	ImageCaptions       []string         `json:"$image_captions,omitempty"`
	Condition           ContentCondition `json:"$condition,omitempty"`
	CustomFields        map[string]any   `json:"$custom_fields,omitempty"`
}

type eventResponse struct {
	AscendingOnly         *bool   `json:"ascending_only"`
	CoarseKey             *string `json:"coarse_key"`
	Locked                *bool   `json:"locked"`
	UpdateConversionValue *int64  `json:"update_conversion_value"`
}

func (client *Client) TrackStandardEvent(ctx context.Context, input StandardEventRequest, options ...socialhub.CallOption) (SubmitResult, error) {
	if err := validateCallOptions(options); err != nil {
		return SubmitResult{}, err
	}
	if err := validateStandardEvent(input); err != nil {
		return SubmitResult{}, invalidArgument(trackStandardOperation, err.Error())
	}
	if err := client.requireIPOverride(trackStandardOperation, input.IPOverride); err != nil {
		return SubmitResult{}, err
	}
	payload := wireEventRequest{
		BranchKey: client.branchKey, Name: string(input.Name), CustomerEventAlias: input.CustomerEventAlias,
		UserData: normalizeUserData(input.UserData), CustomData: normalizeProperties(input.CustomData),
		EventData: normalizeEventData(input.EventData), ContentItems: normalizeContentItems(input.ContentItems),
	}
	return client.submit(ctx, trackStandardOperation, "/v2/event/standard", input.IPOverride, payload, options...)
}

func (client *Client) TrackCustomEvent(ctx context.Context, input CustomEventRequest, options ...socialhub.CallOption) (SubmitResult, error) {
	if err := validateCallOptions(options); err != nil {
		return SubmitResult{}, err
	}
	if err := validateCustomEvent(input); err != nil {
		return SubmitResult{}, invalidArgument(trackCustomOperation, err.Error())
	}
	if err := client.requireIPOverride(trackCustomOperation, input.IPOverride); err != nil {
		return SubmitResult{}, err
	}
	payload := wireEventRequest{
		BranchKey: client.branchKey, Name: input.Name, UserData: normalizeUserData(input.UserData),
		CustomData: normalizeProperties(input.CustomData), Metadata: normalizeProperties(input.Metadata),
		EventData: normalizeEventData(input.EventData),
	}
	return client.submit(ctx, trackCustomOperation, "/v2/event/custom", input.IPOverride, payload, options...)
}

func (client *Client) submit(ctx context.Context, operation, path, ipOverride string, payload wireEventRequest, options ...socialhub.CallOption) (SubmitResult, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return SubmitResult{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request, err := client.api.NewRequest(ctx, http.MethodPost, path, nil, bytes.NewReader(encoded), options...)
	if err != nil {
		return SubmitResult{}, withOperation(err, operation)
	}
	request.Header.Set("Content-Type", "application/json")
	if ipOverride != "" {
		request.Header.Set("X-IP-Override", ipOverride)
	}
	var response eventResponse
	metadata, err := client.api.DoWithMetadata(request, &response)
	if err != nil {
		return SubmitResult{}, withOperation(err, operation)
	}
	if metadata.StatusCode != http.StatusOK {
		return SubmitResult{}, platformContractError(operation, "Branch returned an undocumented success status", metadata.StatusCode)
	}
	return SubmitResult{
		StatusCode: metadata.StatusCode, AscendingOnly: response.AscendingOnly, CoarseKey: response.CoarseKey,
		Locked: response.Locked, UpdateConversionValue: response.UpdateConversionValue,
	}, nil
}

func validateCallOptions(options []socialhub.CallOption) error {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return err
	}
	if resolved.IdempotencyKey != "" {
		return invalidArgument("events", "Branch Events API v2 does not document an idempotency-key contract")
	}
	return nil
}

func normalizeUserData(input UserData) wireUserData {
	return wireUserData{
		OS: input.OS, OSVersion: input.OSVersion, AdvertisingIDs: cloneStrings(input.AdvertisingIDs),
		Environment: input.Environment, AAID: input.AAID, AndroidID: input.AndroidID, IDFA: input.IDFA, IDFV: input.IDFV,
		AnonID: input.AnonID, LimitAdTracking: input.LimitAdTracking, UserAgent: input.UserAgent,
		BrowserFingerprintID: input.BrowserFingerprintID, HTTPOrigin: input.HTTPOrigin, HTTPReferrer: input.HTTPReferrer,
		DeveloperIdentity: input.DeveloperIdentity, GoogleAnalyticsID: input.GoogleAnalyticsID,
		RandomizedDeviceToken: input.RandomizedDeviceToken,
		Country:               input.Country, Language: input.Language, IP: input.IP, LocalIP: input.LocalIP,
		Brand: input.Brand, AppVersion: input.AppVersion, Model: input.Model,
		ScreenDPI: input.ScreenDPI, ScreenHeight: input.ScreenHeight, ScreenWidth: input.ScreenWidth,
		DMAEEA: input.DMAEEA, DMAAdPersonalization: input.DMAAdPersonalization, DMAAdUserData: input.DMAAdUserData,
	}
}

func normalizeEventData(input *EventData) *wireEventData {
	if input == nil || eventDataEmpty(*input) {
		return nil
	}
	return &wireEventData{
		TransactionID: input.TransactionID, Revenue: input.Revenue, Currency: input.Currency,
		Shipping: input.Shipping, Tax: input.Tax, Coupon: input.Coupon, Affiliation: input.Affiliation,
		Description: input.Description, SearchQuery: input.SearchQuery,
	}
}

func normalizeContentItems(input []ContentItem) []wireContentItem {
	if len(input) == 0 {
		return nil
	}
	output := make([]wireContentItem, len(input))
	for index, item := range input {
		output[index] = wireContentItem{
			Schema: item.Schema, OGTitle: item.OGTitle, OGDescription: item.OGDescription,
			OGImageURL: item.OGImageURL, CanonicalIdentifier: item.CanonicalIdentifier,
			PubliclyIndexable: item.PubliclyIndexable, LocallyIndexable: item.LocallyIndexable,
			Price: item.Price, Quantity: item.Quantity, SKU: item.SKU, ProductName: item.ProductName,
			ProductBrand: item.ProductBrand, ProductCategory: item.ProductCategory, ProductVariant: item.ProductVariant,
			RatingAverage: item.RatingAverage, RatingCount: item.RatingCount, RatingMax: item.RatingMax,
			CreationTimestamp: item.CreationTimestamp, ExpirationTimestamp: item.ExpirationTimestamp,
			Keywords: append([]string(nil), item.Keywords...), AddressStreet: item.AddressStreet,
			AddressCity: item.AddressCity, AddressRegion: item.AddressRegion, AddressCountry: item.AddressCountry,
			AddressPostalCode: item.AddressPostalCode, Latitude: item.Latitude, Longitude: item.Longitude,
			ImageCaptions: append([]string(nil), item.ImageCaptions...), Condition: item.Condition,
			CustomFields: normalizeProperties(item.CustomFields),
		}
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

func cloneStrings(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

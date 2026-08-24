package kochava

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	trackInstallOperation = "s2s.track_install"
	trackEventOperation   = "s2s.track_event"
	updateIDFAOperation   = "s2s.update_idfa"
)

type wireRequest struct {
	Action          string `json:"action"`
	KochavaAppID    string `json:"kochava_app_id"`
	KochavaDeviceID string `json:"kochava_device_id"`
	Data            any    `json:"data"`
}

type wireContext struct {
	UserTime            *int64                  `json:"usertime,omitempty"`
	AppVersion          string                  `json:"app_version,omitempty"`
	DeviceLimitTracking *string                 `json:"device_limit_tracking,omitempty"`
	DeviceVersion       string                  `json:"device_ver"`
	DeviceIDs           map[string]string       `json:"device_ids"`
	DeviceUserAgent     string                  `json:"device_ua"`
	OriginationIP       string                  `json:"origination_ip"`
	ATT                 *wireATT                `json:"app_tracking_transparency,omitempty"`
	GDPRPrivacyConsent  *wireGDPRPrivacyConsent `json:"gdpr_privacy_consent,omitempty"`
}

type wireATT struct {
	Time       *int64    `json:"att_time,omitempty"`
	Authorized bool      `json:"att"`
	Duration   *int64    `json:"att_duration,omitempty"`
	Detail     ATTDetail `json:"att_detail,omitempty"`
}

type wireGDPRPrivacyConsent struct {
	GDPRApplies       *int   `json:"gdpr_applies,omitempty"`
	TCString          string `json:"tc_string,omitempty"`
	AdUserData        *int   `json:"ad_user_data,omitempty"`
	AdPersonalization *int   `json:"ad_personalization,omitempty"`
}

type wireInstallData struct {
	wireContext
	AdServicesToken       string                     `json:"ad_services_token,omitempty"`
	IAdAttributionDetails *wireIAdAttributionDetails `json:"iad_attribution_details,omitempty"`
	AdServicesAttribution *wireAdServicesAttribution `json:"adservices_attribution_details,omitempty"`
	InstallReferrer       *wireInstallReferrer       `json:"install_referrer,omitempty"`
}

type wireIAdAttributionDetails struct {
	Version31 *wireIAdAttribution `json:"Version3.1"`
}

type wireIAdAttribution struct {
	PurchaseDate     string `json:"iad-purchase-date,omitempty"`
	Keyword          string `json:"iad-keyword,omitempty"`
	AdGroupID        string `json:"iad-adgroup-id,omitempty"`
	CreativeSetID    string `json:"iad-creativeset-id,omitempty"`
	CreativeSetName  string `json:"iad-creativeset-name,omitempty"`
	CampaignID       string `json:"iad-campaign-id,omitempty"`
	LineItemID       string `json:"iad-lineitem-id,omitempty"`
	OrganizationID   string `json:"iad-org-id,omitempty"`
	ConversionDate   string `json:"iad-conversion-date,omitempty"`
	KeywordID        string `json:"iad-keyword-id,omitempty"`
	ConversionType   string `json:"iad-conversion-type,omitempty"`
	CountryOrRegion  string `json:"iad-country-or-region,omitempty"`
	OrganizationName string `json:"iad-org-name,omitempty"`
	CampaignName     string `json:"iad-campaign-name,omitempty"`
	ClickDate        string `json:"iad-click-date,omitempty"`
	Attributed       string `json:"iad-attribution,omitempty"`
	AdGroupName      string `json:"iad-adgroup-name,omitempty"`
	KeywordMatchType string `json:"iad-keyword-matchtype,omitempty"`
	LineItemName     string `json:"iad-lineitem-name,omitempty"`
}

type wireAdServicesAttribution struct {
	KeywordID       *int64 `json:"keywordId,omitempty"`
	ConversionType  string `json:"conversionType,omitempty"`
	CreativeSetID   *int64 `json:"creativeSetId,omitempty"`
	OrganizationID  *int64 `json:"orgId,omitempty"`
	CampaignID      *int64 `json:"campaignId,omitempty"`
	AdGroupID       *int64 `json:"adGroupId,omitempty"`
	ClickDate       string `json:"clickDate,omitempty"`
	CountryOrRegion string `json:"countryOrRegion,omitempty"`
	Attributed      *bool  `json:"attribution,omitempty"`
}

type wireInstallReferrer struct {
	Referrer  string `json:"referrer"`
	ClickTime *int64 `json:"referrer_click_time,omitempty"`
}

type wireEventData struct {
	wireContext
	EventName string         `json:"event_name"`
	Currency  string         `json:"currency,omitempty"`
	EventData map[string]any `json:"event_data,omitempty"`
}

type wireUpdateData struct {
	IDFA string `json:"idfa"`
}

func (client *Client) TrackInstall(ctx context.Context, input InstallRequest, options ...socialhub.CallOption) (SubmitResult, error) {
	if err := client.requirePaid(trackInstallOperation); err != nil {
		return SubmitResult{}, err
	}
	callOptions, err := kochavaCallOptions(trackInstallOperation, options)
	if err != nil {
		return SubmitResult{}, err
	}
	if err := validateInstall(input); err != nil {
		return SubmitResult{}, invalidArgument(trackInstallOperation, err.Error())
	}
	data := wireInstallData{wireContext: normalizeContext(input.Context)}
	if input.AppleSearchAds != nil {
		data.AdServicesToken = input.AppleSearchAds.AdServicesToken
		data.IAdAttributionDetails = normalizeIAd(input.AppleSearchAds.IAd)
		data.AdServicesAttribution = normalizeAdServices(input.AppleSearchAds.AdServicesAttribution)
	}
	if input.InstallReferrer != nil {
		data.InstallReferrer = &wireInstallReferrer{
			Referrer: input.InstallReferrer.Referrer, ClickTime: unixSeconds(input.InstallReferrer.ClickTime),
		}
	}
	return client.submit(ctx, trackInstallOperation, input.KochavaDeviceID, "install", data, input.Context.DeviceUserAgent == "", callOptions...)
}

func (client *Client) TrackEvent(ctx context.Context, input EventRequest, options ...socialhub.CallOption) (SubmitResult, error) {
	if err := client.requirePaid(trackEventOperation); err != nil {
		return SubmitResult{}, err
	}
	callOptions, err := kochavaCallOptions(trackEventOperation, options)
	if err != nil {
		return SubmitResult{}, err
	}
	if err := validateEvent(input); err != nil {
		return SubmitResult{}, invalidArgument(trackEventOperation, err.Error())
	}
	data := wireEventData{
		wireContext: normalizeContext(input.Context), EventName: string(input.Name),
		Currency: input.Currency, EventData: normalizeProperties(input.Data),
	}
	return client.submit(ctx, trackEventOperation, input.KochavaDeviceID, "event", data, input.Context.DeviceUserAgent == "", callOptions...)
}

func (client *Client) UpdateIDFA(ctx context.Context, input UpdateIDFARequest, options ...socialhub.CallOption) (SubmitResult, error) {
	if err := client.requirePaid(updateIDFAOperation); err != nil {
		return SubmitResult{}, err
	}
	callOptions, err := kochavaCallOptions(updateIDFAOperation, options)
	if err != nil {
		return SubmitResult{}, err
	}
	if err := validateUpdateIDFA(input); err != nil {
		return SubmitResult{}, invalidArgument(updateIDFAOperation, err.Error())
	}
	return client.submit(ctx, updateIDFAOperation, input.KochavaDeviceID, "update", wireUpdateData{IDFA: input.IDFA}, false, callOptions...)
}

func (client *Client) submit(ctx context.Context, operation, deviceID, action string, data any, unknownUserAgent bool, options ...socialhub.CallOption) (SubmitResult, error) {
	payload := wireRequest{Action: action, KochavaAppID: client.appGUID, KochavaDeviceID: deviceID, Data: data}
	body, err := json.Marshal(payload)
	if err != nil {
		return SubmitResult{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	// Strict Authentication hashes the exact transmitted JSON and Kochava's
	// contract requires forward slashes to use JSON's escaped form.
	body = bytes.ReplaceAll(body, []byte("/"), []byte(`\/`))
	if len(body) >= MaximumPayloadBytes {
		return SubmitResult{}, invalidArgument(operation, "JSON payload must remain under Kochava's 2 MiB limit")
	}
	request, err := client.api.NewRequest(ctx, http.MethodPost, "/track/json", nil, bytes.NewReader(body), options...)
	if err != nil {
		return SubmitResult{}, withOperation(err, operation)
	}
	request.Header.Set("Content-Type", "application/json")
	if unknownUserAgent {
		request.Header.Set("User-Agent", "Unknown")
	}
	if client.apiKey != "" {
		request.Header.Set("Kochava-Api-Key", client.apiKey)
		request.Header.Set("Kochava-Auth-Token", strictAuthToken(client.apiKey, client.appSecret, body))
	}
	metadata, err := client.api.DoWithMetadata(request, nil)
	if err != nil {
		return SubmitResult{}, withOperation(err, operation)
	}
	return SubmitResult{StatusCode: metadata.StatusCode}, nil
}

func kochavaCallOptions(operation string, options []socialhub.CallOption) ([]socialhub.CallOption, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return nil, withOperation(err, operation)
	}
	if resolved.RequestID != "" {
		return nil, invalidArgument(operation, "Kochava S2S does not document caller-supplied request IDs")
	}
	if resolved.IdempotencyKey != "" {
		return nil, invalidArgument(operation, "Kochava S2S does not document an idempotency-key or deduplication contract")
	}
	if len(resolved.Fields) != 0 {
		return nil, invalidArgument(operation, "Kochava S2S responses do not support field selection")
	}
	if resolved.Timeout == 0 {
		return nil, nil
	}
	return []socialhub.CallOption{socialhub.WithCallTimeout(resolved.Timeout)}, nil
}

func normalizeContext(input DeviceContext) wireContext {
	output := wireContext{
		UserTime: unixSeconds(input.OccurredAt), AppVersion: input.AppVersion,
		DeviceVersion: input.DeviceVersion, DeviceIDs: normalizeDeviceIdentifiers(input.DeviceIDs),
		DeviceUserAgent: input.DeviceUserAgent, OriginationIP: input.OriginationIP,
	}
	if input.LimitTracking != nil {
		value := strconv.FormatBool(*input.LimitTracking)
		output.DeviceLimitTracking = &value
	}
	if input.ATT != nil {
		output.ATT = &wireATT{
			Time: unixSeconds(input.ATT.AuthorizationTime), Authorized: *input.ATT.Authorized,
			Duration: input.ATT.ResponseDuration, Detail: input.ATT.Detail,
		}
	}
	if input.GDPRPrivacyConsent != nil {
		output.GDPRPrivacyConsent = &wireGDPRPrivacyConsent{
			GDPRApplies:       boolIntPointer(input.GDPRPrivacyConsent.GDPRApplies),
			TCString:          input.GDPRPrivacyConsent.TCString,
			AdUserData:        boolIntPointer(input.GDPRPrivacyConsent.AdUserData),
			AdPersonalization: boolIntPointer(input.GDPRPrivacyConsent.AdPersonalization),
		}
	}
	return output
}

func normalizeDeviceIdentifiers(input DeviceIdentifiers) map[string]string {
	output := make(map[string]string, 6+len(input.Custom))
	values := []struct{ key, value string }{
		{"idfa", input.IDFA}, {"idfv", input.IDFV}, {"adid", input.ADID},
		{"android_id", input.AndroidID}, {"openudid", input.OpenUDID}, {"udid", input.UDID},
	}
	for _, value := range values {
		if value.value != "" {
			output[value.key] = value.value
		}
	}
	for key, value := range input.Custom {
		output[key] = value
	}
	return output
}

func normalizeProperties(input Properties) map[string]any {
	if len(input.Strings) == 0 && len(input.Numbers) == 0 && len(input.Booleans) == 0 && len(input.StringLists) == 0 {
		return nil
	}
	output := make(map[string]any, len(input.Strings)+len(input.Numbers)+len(input.Booleans)+len(input.StringLists))
	for key, value := range input.Strings {
		output[key] = value
	}
	for key, value := range input.Numbers {
		output[key] = value
	}
	for key, value := range input.Booleans {
		output[key] = value
	}
	for key, value := range input.StringLists {
		output[key] = append([]string(nil), value...)
	}
	return output
}

func normalizeIAd(input *IAdAttribution) *wireIAdAttributionDetails {
	if input == nil {
		return nil
	}
	return &wireIAdAttributionDetails{Version31: &wireIAdAttribution{
		PurchaseDate: input.PurchaseDate, Keyword: input.Keyword, AdGroupID: input.AdGroupID,
		CreativeSetID: input.CreativeSetID, CreativeSetName: input.CreativeSetName, CampaignID: input.CampaignID,
		LineItemID: input.LineItemID, OrganizationID: input.OrganizationID, ConversionDate: input.ConversionDate,
		KeywordID: input.KeywordID, ConversionType: input.ConversionType, CountryOrRegion: input.CountryOrRegion,
		OrganizationName: input.OrganizationName, CampaignName: input.CampaignName, ClickDate: input.ClickDate,
		Attributed: input.Attributed, AdGroupName: input.AdGroupName, KeywordMatchType: input.KeywordMatchType,
		LineItemName: input.LineItemName,
	}}
}

func normalizeAdServices(input *AdServicesAttribution) *wireAdServicesAttribution {
	if input == nil {
		return nil
	}
	return &wireAdServicesAttribution{
		KeywordID: input.KeywordID, ConversionType: input.ConversionType, CreativeSetID: input.CreativeSetID,
		OrganizationID: input.OrganizationID, CampaignID: input.CampaignID, AdGroupID: input.AdGroupID,
		ClickDate: input.ClickDate, CountryOrRegion: input.CountryOrRegion, Attributed: input.Attributed,
	}
}

func strictAuthToken(apiKey, appSecret string, body []byte) string {
	payloadHash := sha1.Sum(body)
	message := appSecret + hex.EncodeToString(payloadHash[:])
	signature := hmac.New(sha256.New, []byte(apiKey))
	_, _ = signature.Write([]byte(message))
	return hex.EncodeToString(signature.Sum(nil))
}

func unixSeconds(value *time.Time) *int64 {
	if value == nil {
		return nil
	}
	seconds := value.Unix()
	return &seconds
}

func boolIntPointer(value *bool) *int {
	if value == nil {
		return nil
	}
	integer := 0
	if *value {
		integer = 1
	}
	return &integer
}

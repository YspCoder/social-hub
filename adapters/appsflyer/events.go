package appsflyer

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"social-hub/pkg/socialhub"
)

const sendEventOperation = "events.send"

type eventPayload struct {
	AppsFlyerID    string `json:"appsflyer_id"`
	EventName      string `json:"eventName"`
	EventValue     string `json:"eventValue"`
	EventTime      string `json:"eventTime,omitempty"`
	EventCurrency  string `json:"eventCurrency,omitempty"`
	BundleID       string `json:"bundleIdentifier,omitempty"`
	AppVersionName string `json:"app_version_name,omitempty"`
	AppStore       string `json:"app_store,omitempty"`
	OS             string `json:"os,omitempty"`
	UserAgent      string `json:"ua,omitempty"`
	IPAddress      string `json:"ip,omitempty"`
	CustomerUserID string `json:"customer_user_id,omitempty"`

	AdvertisingID string `json:"advertising_id,omitempty"`
	OAID          string `json:"oaid,omitempty"`
	AmazonAID     string `json:"amazon_aid,omitempty"`
	IMEI          string `json:"imei,omitempty"`
	IDFA          string `json:"idfa,omitempty"`
	IDFV          string `json:"idfv,omitempty"`
	FBLoginID     string `json:"fb_login_id,omitempty"`

	EmailHashed     string `json:"email_hashed,omitempty"`
	PhoneHashed     string `json:"phone_number_hashed,omitempty"`
	PhoneE164Hashed string `json:"phone_number_e164_hashed,omitempty"`
	FirstNameHashed string `json:"first_name_hashed,omitempty"`
	LastNameHashed  string `json:"last_name_hashed,omitempty"`

	SharingFilter any           `json:"sharing_filter,omitempty"`
	CustomData    string        `json:"custom_data,omitempty"`
	AppType       AppType       `json:"app_type,omitempty"`
	AIE           *bool         `json:"aie,omitempty"`
	ATT           *int          `json:"att,omitempty"`
	Consent       any           `json:"consent_data,omitempty"`
	AppSetID      *wireAppSetID `json:"app_set_id,omitempty"`
}

type wireAppSetID struct {
	Scope AppSetIDScope `json:"scope"`
	ID    string        `json:"id"`
}

type wireConsentData struct {
	Manual *wireManualConsent `json:"manual,omitempty"`
	TCF    *wireTCFConsent    `json:"tcf,omitempty"`
}

type wireManualConsent struct {
	GDPRApplies              *bool `json:"gdpr_applies,omitempty"`
	AdUserDataEnabled        *bool `json:"ad_user_data_enabled,omitempty"`
	AdPersonalizationEnabled *bool `json:"ad_personalization_enabled,omitempty"`
}

type wireTCFConsent struct {
	PolicyVersion int    `json:"policy_version"`
	CMPSDKID      int    `json:"cmp_sdk_id"`
	CMPSDKVersion int    `json:"cmp_sdk_version"`
	GDPRApplies   int    `json:"gdpr_applies"`
	TCString      string `json:"tcstring"`
}

func (client *Client) SendEvent(ctx context.Context, input EventRequest, options ...socialhub.CallOption) (SubmitResult, error) {
	callOptions, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return SubmitResult{}, err
	}
	if callOptions.IdempotencyKey != "" {
		return SubmitResult{}, invalidArgument(sendEventOperation, "AppsFlyer does not provide an idempotency-key contract")
	}
	payload, err := client.normalizeEvent(input)
	if err != nil {
		return SubmitResult{}, invalidArgument(sendEventOperation, err.Error())
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return SubmitResult{}, platformError(sendEventOperation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if len(encoded) > MaximumRequestBytes {
		return SubmitResult{}, invalidArgument(sendEventOperation, "JSON payload exceeds AppsFlyer's 1 KB limit")
	}
	request, err := client.api.NewRequest(
		ctx, http.MethodPost, "/inappevent/"+client.appID, nil, bytes.NewReader(encoded), options...,
	)
	if err != nil {
		return SubmitResult{}, withOperation(err, sendEventOperation)
	}
	request.Header.Set("Content-Type", "application/json")
	metadata, err := client.api.DoWithMetadata(request, nil)
	if err != nil {
		return SubmitResult{}, withOperation(err, sendEventOperation)
	}
	if metadata.StatusCode != http.StatusOK {
		return SubmitResult{}, platformContractError(sendEventOperation, "AppsFlyer returned an undocumented success status", metadata.StatusCode)
	}
	return SubmitResult{StatusCode: metadata.StatusCode}, nil
}

func (client *Client) normalizeEvent(input EventRequest) (eventPayload, error) {
	if err := validateEvent(input, client.platform); err != nil {
		return eventPayload{}, err
	}
	eventValue := ""
	if len(input.EventValue) > 0 {
		encoded, err := json.Marshal(input.EventValue)
		if err != nil {
			return eventPayload{}, err
		}
		eventValue = string(encoded)
	}
	bundleID := input.BundleIdentifier
	if bundleID == "" {
		bundleID = client.bundleIdentifier
	}
	payload := eventPayload{
		AppsFlyerID: input.AppsFlyerID, EventName: input.EventName, EventValue: eventValue,
		EventCurrency: input.EventCurrency, BundleID: bundleID, AppVersionName: input.AppVersionName,
		AppStore: input.AppStore, OS: input.OS, UserAgent: input.UserAgent, IPAddress: input.IPAddress,
		CustomerUserID: input.CustomerUserID,
		AdvertisingID:  input.Device.AdvertisingID, OAID: input.Device.OAID, AmazonAID: input.Device.AmazonAID,
		IMEI: input.Device.IMEI, IDFA: input.Device.IDFA, IDFV: input.Device.IDFV, FBLoginID: input.Device.FBLoginID,
		EmailHashed: input.HashedUser.Email, PhoneHashed: input.HashedUser.Phone,
		PhoneE164Hashed: input.HashedUser.PhoneE164, FirstNameHashed: input.HashedUser.FirstName,
		LastNameHashed: input.HashedUser.LastName,
		SharingFilter:  normalizeSharingFilter(input.SharingFilter),
		AppType:        input.AppType, AIE: input.AIE, ATT: input.ATT,
	}
	if !customDataEmpty(input.CustomData) {
		encoded, err := json.Marshal(normalizeCustomData(input.CustomData))
		if err != nil {
			return eventPayload{}, err
		}
		payload.CustomData = string(encoded)
	}
	if input.EventTime != nil {
		payload.EventTime = input.EventTime.UTC().Format("2006-01-02 15:04:05.000")
	}
	if input.Consent != nil {
		payload.Consent = normalizeConsent(*input.Consent)
	}
	if input.AppSetID != nil {
		payload.AppSetID = &wireAppSetID{Scope: input.AppSetID.Scope, ID: input.AppSetID.ID}
	}
	return payload, nil
}

func normalizeSharingFilter(input SharingFilter) any {
	if input.BlockAll {
		return "all"
	}
	if len(input.Partners) > 0 {
		return append([]string(nil), input.Partners...)
	}
	return nil
}

func normalizeConsent(input ConsentData) wireConsentData {
	output := wireConsentData{}
	if input.Manual != nil {
		output.Manual = &wireManualConsent{
			GDPRApplies: input.Manual.GDPRApplies, AdUserDataEnabled: input.Manual.AdUserDataEnabled,
			AdPersonalizationEnabled: input.Manual.AdPersonalizationEnabled,
		}
	}
	if input.TCF != nil {
		output.TCF = &wireTCFConsent{
			PolicyVersion: input.TCF.PolicyVersion, CMPSDKID: input.TCF.CMPSDKID,
			CMPSDKVersion: input.TCF.CMPSDKVersion, GDPRApplies: input.TCF.GDPRApplies,
			TCString: input.TCF.TCString,
		}
	}
	return output
}

func normalizeCustomData(input CustomData) map[string]any {
	if customDataEmpty(input) {
		return nil
	}
	output := make(map[string]any, len(input.Strings)+len(input.Numbers)+len(input.Booleans)+len(input.Objects))
	for key, value := range input.Strings {
		output[key] = value
	}
	for key, value := range input.Numbers {
		output[key] = value
	}
	for key, value := range input.Booleans {
		output[key] = value
	}
	for key, value := range input.Objects {
		nested := normalizeCustomData(value)
		if nested == nil {
			nested = map[string]any{}
		}
		output[key] = nested
	}
	return output
}

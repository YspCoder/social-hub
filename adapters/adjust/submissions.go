package adjust

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	trackEventOperation     = "s2s.track_event"
	trackSessionOperation   = "s2s.track_session"
	trackAdRevenueOperation = "s2s.track_ad_revenue"
)

func (client *Client) TrackEvent(ctx context.Context, input EventRequest, options ...socialhub.CallOption) (SubmitResult, error) {
	callOptions, err := adjustCallOptions(trackEventOperation, options)
	if err != nil {
		return SubmitResult{}, err
	}
	if err := client.requireCapability(trackEventOperation, ScopeEvents, true, "configured S2S token does not authorize events"); err != nil {
		return SubmitResult{}, err
	}
	if err := validateEvent(input, client.clock.Now()); err != nil {
		return SubmitResult{}, invalidArgument(trackEventOperation, err.Error())
	}
	form := baseForm(client.appToken, input.Device)
	form.Set("event_token", input.EventToken)
	addCommon(form, input.CreatedAt, input.Environment, input.IPAddress, input.UserAgent, input.CallbackParams, input.PartnerParams)
	if input.Revenue != "" {
		form.Set("revenue", input.Revenue.String())
		form.Set("currency", input.Currency)
	}
	return client.submit(ctx, trackEventOperation, "/event", form, callOptions...)
}

func (client *Client) TrackSession(ctx context.Context, input SessionRequest, options ...socialhub.CallOption) (*SessionResult, error) {
	callOptions, err := adjustCallOptions(trackSessionOperation, options)
	if err != nil {
		return nil, err
	}
	if err := client.requireCapability(trackSessionOperation, ScopeSessions, client.sessionEnabled, "S2S session measurement is not recorded as enabled for this account"); err != nil {
		return nil, err
	}
	if err := validateSession(input, client.clock.Now()); err != nil {
		return nil, invalidArgument(trackSessionOperation, err.Error())
	}
	form := baseForm(client.appToken, input.Device)
	form.Set("os_name", string(input.OSName))
	addCommon(form, input.CreatedAt, input.Environment, input.IPAddress, input.UserAgent, input.CallbackParams, input.PartnerParams)
	addISOTime(form, "sent_at", input.SentAt)
	addString(form, "app_version", input.AppVersion)
	addString(form, "app_version_short", input.AppVersionShort)
	addInt64(form, "session_count", input.SessionCount)
	addInt64(form, "subsession_count", input.SubsessionCount)
	addInt64(form, "session_length", input.SessionLength)
	addInt64(form, "time_spent", input.TimeSpent)
	addBool(form, "tracking_enabled", input.TrackingEnabled)
	if input.ATTStatus != nil {
		form.Set("att_status", strconv.Itoa(*input.ATTStatus))
	}
	addString(form, "bundle_id", input.BundleID)
	addString(form, "package_name", input.PackageName)
	addString(form, "country", input.Country)
	addString(form, "language", input.Language)
	addString(form, "os_version", input.OSVersion)
	addString(form, "cpu_type", input.CPUType)
	addString(form, "device_type", input.DeviceType)
	addString(form, "device_name", input.DeviceName)
	addString(form, "hardware_name", input.HardwareName)
	addString(form, "install_receipt", input.InstallReceipt)
	addString(form, "primary_dedupe_token", input.PrimaryDedupeToken)
	addString(form, "google_app_set_id", input.GoogleAppSetID)
	addBool(form, "eea", input.EEA)
	addBool(form, "ad_personalization", input.AdPersonalization)
	addBool(form, "ad_user_data", input.AdUserData)
	addBool(form, "npa", input.NPA)
	if input.AmazonDMA != nil {
		amazon := map[string]string{}
		if input.AmazonDMA.AdUserData != nil {
			amazon["ad_user_data"] = boolString(*input.AmazonDMA.AdUserData)
		}
		if input.AmazonDMA.AdStorage != nil {
			amazon["ad_storage"] = boolString(*input.AmazonDMA.AdStorage)
		}
		encoded, _ := json.Marshal(map[string]any{"amazon_ads": amazon})
		form.Set("dma", string(encoded))
	}
	body, err := encodedForm(trackSessionOperation, form)
	if err != nil {
		return nil, err
	}
	request, err := client.api.NewRequest(ctx, http.MethodPost, "/session", nil, bytes.NewReader(body), callOptions...)
	if err != nil {
		return nil, withOperation(err, trackSessionOperation)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if input.ForwardedFor != "" {
		request.Header.Set("X-Adjust-Forwarded-For", input.ForwardedFor)
	}
	var result SessionResult
	metadata, err := client.api.DoWithMetadata(request, &result)
	if err != nil {
		return nil, withOperation(err, trackSessionOperation)
	}
	if metadata.StatusCode != http.StatusOK {
		return nil, platformContractError(trackSessionOperation, "Adjust returned an undocumented session success status", metadata.StatusCode)
	}
	return &result, nil
}

func (client *Client) TrackAdRevenue(ctx context.Context, input AdRevenueRequest, options ...socialhub.CallOption) (SubmitResult, error) {
	callOptions, err := adjustCallOptions(trackAdRevenueOperation, options)
	if err != nil {
		return SubmitResult{}, err
	}
	if err := client.requireCapability(trackAdRevenueOperation, ScopeAdRevenue, client.adRevenueEnabled, "the Adjust ad revenue package is not recorded as enabled for this account"); err != nil {
		return SubmitResult{}, err
	}
	if err := validateAdRevenue(input, client.clock.Now()); err != nil {
		return SubmitResult{}, invalidArgument(trackAdRevenueOperation, err.Error())
	}
	form := baseForm(client.appToken, input.Device)
	form.Set("revenue", input.Revenue.String())
	form.Set("currency", input.Currency)
	form.Set("ad_impressions_count", strconv.FormatInt(input.AdImpressionsCount, 10))
	form.Set("source", "publisher")
	addCommon(form, input.CreatedAt, input.Environment, "", "", input.CallbackParams, nil)
	addString(form, "ad_revenue_network", input.Network)
	addString(form, "ad_revenue_unit", input.Unit)
	addString(form, "ad_revenue_placement", input.Placement)
	return client.submit(ctx, trackAdRevenueOperation, "/ad_revenue", form, callOptions...)
}

func adjustCallOptions(operation string, options []socialhub.CallOption) ([]socialhub.CallOption, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return nil, withOperation(err, operation)
	}
	if resolved.RequestID != "" {
		return nil, invalidArgument(operation, "Adjust S2S does not document an X-Request-ID request header")
	}
	if resolved.IdempotencyKey != "" {
		return nil, invalidArgument(operation, "Adjust S2S does not document an idempotency-key contract")
	}
	if len(resolved.Fields) != 0 {
		return nil, invalidArgument(operation, "Adjust S2S does not support response field selection")
	}
	if resolved.Timeout == 0 {
		return nil, nil
	}
	return []socialhub.CallOption{socialhub.WithCallTimeout(resolved.Timeout)}, nil
}

func (client *Client) submit(ctx context.Context, operation, path string, form url.Values, options ...socialhub.CallOption) (SubmitResult, error) {
	body, err := encodedForm(operation, form)
	if err != nil {
		return SubmitResult{}, err
	}
	request, err := client.api.NewRequest(ctx, http.MethodPost, path, nil, bytes.NewReader(body), options...)
	if err != nil {
		return SubmitResult{}, withOperation(err, operation)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	metadata, err := client.api.DoWithMetadata(request, nil)
	if err != nil {
		return SubmitResult{}, withOperation(err, operation)
	}
	if metadata.StatusCode != http.StatusOK {
		return SubmitResult{}, platformContractError(operation, "Adjust returned an undocumented success status", metadata.StatusCode)
	}
	return SubmitResult{StatusCode: metadata.StatusCode}, nil
}

func encodedForm(operation string, form url.Values) ([]byte, error) {
	body := []byte(form.Encode())
	if len(body) > MaximumRequestBytes {
		return nil, invalidArgument(operation, "encoded request exceeds the adapter's 1 MiB safety limit")
	}
	return body, nil
}

func baseForm(appToken string, device DeviceIdentifiers) url.Values {
	form := url.Values{"app_token": {appToken}, "s2s": {"1"}}
	fields := []struct{ key, value string }{
		{"vida", device.VIDA}, {"rida", device.RIDA}, {"tifa", device.TIFA},
		{"idfa", device.IDFA}, {"gps_adid", device.GPSADID}, {"fire_adid", device.FireADID},
		{"oaid", device.OAID}, {"web_uuid", device.WebUUID}, {"adid", device.ADID}, {"idfv", device.IDFV},
		{"android_id", device.AndroidID}, {"external_device_id", device.ExternalDeviceID},
		{"android_id_lower_md5", device.AndroidIDLowerMD5}, {"android_id_lower_sha1", device.AndroidIDLowerSHA1},
		{"android_id_upper_md5", device.AndroidIDUpperMD5}, {"android_id_upper_sha1", device.AndroidIDUpperSHA1},
		{"imei", device.IMEI}, {"imei_lower_md5", device.IMEILowerMD5}, {"meid", device.MEID},
		{"win_naid", device.WindowsNAID}, {"win_hwid", device.WindowsHardwareID},
		{"persistent_ios_uuid", device.PersistentIOSUUID},
	}
	for _, field := range fields {
		addString(form, field.key, field.value)
	}
	return form
}

func addCommon(form url.Values, createdAt *time.Time, environment Environment, ipAddress, userAgent string, callback, partner map[string]string) {
	addTime(form, "created_at_unix", createdAt)
	addString(form, "environment", string(environment))
	addString(form, "ip_address", ipAddress)
	addString(form, "user_agent", userAgent)
	addJSONMap(form, "callback_params", callback)
	addJSONMap(form, "partner_params", partner)
}

func addString(form url.Values, key, value string) {
	if value != "" {
		form.Set(key, value)
	}
}

func addTime(form url.Values, key string, value *time.Time) {
	if value != nil {
		form.Set(key, strconv.FormatInt(value.Unix(), 10))
	}
}

func addISOTime(form url.Values, key string, value *time.Time) {
	if value != nil {
		form.Set(key, value.Format("2006-01-02T15:04:05.000-0700"))
	}
}

func addInt64(form url.Values, key string, value *int64) {
	if value != nil {
		form.Set(key, strconv.FormatInt(*value, 10))
	}
}

func addBool(form url.Values, key string, value *bool) {
	if value != nil {
		form.Set(key, boolString(*value))
	}
}

func boolString(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func addJSONMap(form url.Values, key string, value map[string]string) {
	if len(value) == 0 {
		return
	}
	encoded, _ := json.Marshal(value)
	form.Set(key, string(encoded))
}

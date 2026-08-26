package tenjin

import (
	"net/url"
	"strconv"
	"strings"
)

func (client *Client) eventForm(input EventContext, includeTrackingStatus bool) url.Values {
	form := url.Values{
		"analytics_installation_id": {normalizeInstallationID(input.Identity.AnalyticsInstallationID)},
		"advertising_id":            {input.Identity.AdvertisingID},
		"bundle_id":                 {client.bundleID},
		"platform":                  {string(client.platform)},
		"os_version":                {input.OSVersion},
		"sdk_version":               {"server"},
	}
	setOptional(form, "developer_device_id", input.Identity.DeveloperDeviceID)
	setOptional(form, "app_version", input.AppVersion)
	if input.LimitAdTracking != nil {
		form.Set("limit_ad_tracking", boolInt(*input.LimitAdTracking))
	}
	setOptional(form, "country", input.Country)
	setOptional(form, "ip_address", input.IPAddress)
	setConsent(form, "ad_user_data", input.AdUserData)
	setConsent(form, "ad_personalization", input.AdPersonalization)
	setOptional(form, "os_version_release", input.OSVersionRelease)
	setOptional(form, "build_id", input.BuildID)
	setOptional(form, "locale", input.Locale)
	setOptional(form, "device_model", input.DeviceModel)
	setOptional(form, "customer_user_id", input.CustomerUserID)
	if includeTrackingStatus && input.TrackingStatus != nil {
		form.Set("tracking_status", strconv.Itoa(int(*input.TrackingStatus)))
	}
	return form
}

func (client *Client) openForm(input OpenRequest) url.Values {
	form := client.eventForm(input.Context, false)
	setOptional(form, "referrer", input.Referrer)
	setOptional(form, "odm_info", input.ODMInfo)
	return form
}

func (client *Client) customEventForm(input CustomEventRequest) url.Values {
	form := client.eventForm(input.Context, true)
	form.Set("event", string(input.Name))
	if input.Value != nil {
		form.Set("value", strconv.FormatInt(*input.Value, 10))
	}
	return form
}

func (client *Client) purchaseForm(input PurchaseRequest) url.Values {
	form := client.eventForm(input.Context, true)
	form.Set("product_id", input.ProductID)
	form.Set("price", input.Price.String())
	form.Set("quantity", strconv.FormatInt(input.Quantity, 10))
	form.Set("currency", input.Currency)
	if input.AfterPlatformCut != nil {
		form.Set("postcut", boolInt(*input.AfterPlatformCut))
	}
	return form
}

func (client *Client) adImpressionForm(input AdImpressionRequest) url.Values {
	context := input.Context
	form := url.Values{
		"bundle_id":            {client.bundleID},
		"platform":             {string(client.platform)},
		"app_version":          {context.AppVersion},
		"ip_address":           {context.IPAddress},
		"ad_revenue_mediation": {string(input.Mediation)},
		"network_name":         {input.NetworkName},
		"currency":             {input.Currency},
	}
	setOptional(form, "analytics_installation_id", normalizeInstallationID(context.Identity.AnalyticsInstallationID))
	setOptional(form, "advertising_id", context.Identity.AdvertisingID)
	setOptional(form, "developer_device_id", context.Identity.DeveloperDeviceID)
	setOptional(form, "app_version_code", context.AppVersionCode)
	setOptional(form, "build_id", context.BuildID)
	setOptional(form, "carrier", context.Carrier)
	setOptional(form, "connection_type", string(context.Connection))
	setOptional(form, "country", context.Country)
	setOptional(form, "device", context.Device)
	setOptional(form, "device_brand", context.DeviceBrand)
	setOptional(form, "device_manufacturer", context.DeviceManufacturer)
	setOptional(form, "device_model", context.DeviceModel)
	setOptional(form, "device_product", context.DeviceProduct)
	setOptional(form, "language", context.Language)
	if context.LimitAdTracking != nil {
		form.Set("limit_ad_tracking", strconv.FormatBool(*context.LimitAdTracking))
	}
	if context.OptIn != nil {
		form.Set("opt_in", strconv.FormatBool(*context.OptIn))
	}
	setOptional(form, "locale", context.Locale)
	setOptional(form, "os_version", context.OSVersion)
	setOptional(form, "os_version_release", context.OSVersionRelease)
	setOptional(form, "timezone", context.Timezone)
	setOptional(form, "user_agent", context.UserAgent)
	if context.ScreenHeight != nil {
		form.Set("screen_height", strconv.FormatInt(*context.ScreenHeight, 10))
	}
	if context.ScreenWidth != nil {
		form.Set("screen_width", strconv.FormatInt(*context.ScreenWidth, 10))
	}
	if context.SentAt != nil {
		form.Set("sent_at", strconv.FormatInt(context.SentAt.UnixMilli(), 10))
	}
	setOptional(form, "session_id", context.SessionID)
	setOptional(form, "source_app_store", string(context.SourceAppStore))
	setConsent(form, "ad_user_data", context.AdUserData)
	setConsent(form, "ad_personalization", context.AdPersonalization)
	setOptional(form, "revenue_decimal", input.RevenueDecimal.String())
	setOptional(form, "revenue_cpm", input.RevenueCPM.String())
	setOptional(form, "mediation_country", input.MediationCountry)
	setOptional(form, "ad_unit_id", input.AdUnitID)
	setOptional(form, "ad_format", string(input.Format))
	setOptional(form, "precision", input.Precision)
	setOptional(form, "creative_id", input.CreativeID)
	setOptional(form, "placement", input.Placement)
	setOptional(form, "network_placement", input.NetworkPlacement)
	setOptional(form, "auction_id", input.AuctionID)
	return form
}

func setOptional(form url.Values, key, value string) {
	if value != "" {
		form.Set(key, value)
	}
}

func setConsent(form url.Values, key string, value *bool) {
	if value != nil {
		form.Set(key, boolInt(*value))
	}
}

func boolInt(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func normalizeInstallationID(value string) string {
	return strings.ToLower(strings.ReplaceAll(value, "-", ""))
}

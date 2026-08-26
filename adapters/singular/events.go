package singular

import (
	"bytes"
	"context"
	"encoding/json"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

const (
	sendEventOperation       = "events.send"
	maximumEventRequestBytes = 1 << 20
)

type eventResponse struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

func (client *Client) SendEvent(ctx context.Context, input EventRequest, options ...socialhub.CallOption) (SubmitResult, error) {
	requestOptions, err := prepareCallOptions(sendEventOperation, options)
	if err != nil {
		return SubmitResult{}, err
	}
	if err := validateEvent(input); err != nil {
		return SubmitResult{}, invalidArgument(sendEventOperation, err.Error())
	}
	form, err := client.eventForm(input)
	if err != nil {
		return SubmitResult{}, platformError(sendEventOperation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	encoded := form.Encode()
	if len(encoded) > maximumEventRequestBytes {
		return SubmitResult{}, invalidArgument(sendEventOperation, "encoded request exceeds the adapter's 1 MiB safety limit")
	}
	request, err := client.api.NewRequest(ctx, http.MethodPost, "/api/v2/evt", nil, bytes.NewBufferString(encoded), requestOptions...)
	if err != nil {
		return SubmitResult{}, withOperation(err, sendEventOperation)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var response eventResponse
	metadata, err := client.api.DoWithMetadata(request, &response)
	if err != nil {
		return SubmitResult{}, withOperation(err, sendEventOperation)
	}
	if metadata.StatusCode != http.StatusOK {
		return SubmitResult{}, platformContractError(sendEventOperation, "Singular returned an undocumented success status", metadata.StatusCode)
	}
	if !validJSONContentType(metadata.Header.Get("Content-Type")) {
		return SubmitResult{}, platformContractError(sendEventOperation, "Singular returned a non-JSON success response", metadata.StatusCode)
	}
	switch response.Status {
	case "ok":
		if response.Reason != "" {
			return SubmitResult{}, platformContractError(sendEventOperation, "Singular returned an inconsistent success envelope", metadata.StatusCode)
		}
		return SubmitResult{StatusCode: metadata.StatusCode}, nil
	case "error":
		return SubmitResult{}, singularResponseError(sendEventOperation, metadata.StatusCode, response.Reason)
	default:
		return SubmitResult{}, platformContractError(sendEventOperation, "Singular returned an invalid response envelope", metadata.StatusCode)
	}
}

func validJSONContentType(value string) bool {
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && (strings.EqualFold(mediaType, "application/json") || strings.HasSuffix(strings.ToLower(mediaType), "+json"))
}

func (client *Client) eventForm(input EventRequest) (url.Values, error) {
	form := url.Values{
		"a":    {client.sdkKey},
		"p":    {string(input.Platform)},
		"i":    {client.appID},
		"n":    {string(input.Name)},
		"sdid": {input.SDID},
	}
	setOptional(form, "ip", input.IPAddress)
	if input.UseRequestIP {
		form.Set("use_ip", "true")
	}
	setOptional(form, "country", input.Country)
	setOptional(form, "ve", input.OSVersion)
	setOptional(form, "ma", input.Manufacturer)
	setOptional(form, "mo", input.Model)
	setOptional(form, "lc", input.Locale)
	setOptional(form, "bd", input.Build)
	setOptional(form, "app_v", input.AppVersion)
	if input.ATTStatus != nil {
		form.Set("att_authorization_status", strconv.Itoa(*input.ATTStatus))
	}
	if input.OccurredAt != nil {
		form.Set("umilisec", strconv.FormatInt(input.OccurredAt.UnixMilli(), 10))
	}
	setOptional(form, "ua", input.UserAgent)
	setOptional(form, "c", string(input.Connection))
	setOptional(form, "cn", input.CarrierName)
	if input.DoNotTrack != nil {
		form.Set("dnt", boolInt(*input.DoNotTrack))
	}
	setOptional(form, "custom_user_id", input.CustomUserID)
	if input.LimitDataSharing != nil {
		if err := setJSON(form, "data_sharing_options", map[string]bool{"limit_data_sharing": *input.LimitDataSharing}); err != nil {
			return nil, err
		}
	}

	attributes := normalizeProperties(input.Attributes)
	if input.AdRevenue != nil {
		if attributes == nil {
			attributes = make(map[string]any, len(adRevenueAttributeKeys))
		}
		addAdRevenueAttributes(attributes, *input.AdRevenue)
	}
	if len(attributes) > 0 {
		if err := setJSON(form, "e", attributes); err != nil {
			return nil, err
		}
	}
	if len(input.GlobalProperties) > 0 {
		if err := setJSON(form, "global_properties", input.GlobalProperties); err != nil {
			return nil, err
		}
	}
	addRevenue(form, input.Revenue, input.AdRevenue)
	addSKAN(form, input.SKAN)
	if input.Web != nil {
		form.Set("conversion_event", strconv.FormatBool(input.Web.ConversionEvent))
		if len(input.Web.AttributionData) > 0 {
			if err := setJSON(form, "attribution_data", input.Web.AttributionData); err != nil {
				return nil, err
			}
		}
		setOptional(form, "web_url", input.Web.LandingPageURL)
		setOptional(form, "device_user_agent", input.Web.DeviceUserAgent)
		setOptional(form, "web_page_referrer", input.Web.PageReferrer)
		setOptional(form, "timezone", input.Web.Timezone)
		setOptional(form, "os", input.Web.OS)
		if input.Web.ScreenWidth != nil {
			form.Set("screen_width", strconv.FormatInt(*input.Web.ScreenWidth, 10))
		}
		if input.Web.ScreenHeight != nil {
			form.Set("screen_height", strconv.FormatInt(*input.Web.ScreenHeight, 10))
		}
	}
	return form, nil
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

func addRevenue(form url.Values, revenue *Revenue, adRevenue *AdRevenue) {
	if adRevenue != nil {
		form.Set("is_admon_revenue", "true")
		form.Set("is_revenue_event", "true")
		form.Set("amt", adRevenue.Amount.String())
		form.Set("cur", adRevenue.Currency)
		return
	}
	if revenue == nil {
		return
	}
	if revenue.IsRevenueEvent != nil {
		form.Set("is_revenue_event", strconv.FormatBool(*revenue.IsRevenueEvent))
	}
	setOptional(form, "amt", revenue.Amount.String())
	setOptional(form, "cur", revenue.Currency)
	setOptional(form, "purchase_receipt", revenue.PurchaseReceipt)
	setOptional(form, "receipt_signature", revenue.ReceiptSignature)
	setOptional(form, "purchase_product_id", revenue.ProductID)
	setOptional(form, "purchase_transaction_id", revenue.TransactionID)
}

func addAdRevenueAttributes(attributes map[string]any, revenue AdRevenue) {
	attributes["ad_platform"] = revenue.AdPlatform
	values := []struct{ key, value string }{
		{"ad_mediation_platform", revenue.MediationPlatform}, {"ad_type", revenue.AdType},
		{"ad_group_type", revenue.AdGroupType}, {"ad_impression_id", revenue.AdImpressionID},
		{"ad_placement_name", revenue.AdPlacementName}, {"ad_unit_id", revenue.AdUnitID},
		{"ad_unit_name", revenue.AdUnitName}, {"ad_group_id", revenue.AdGroupID},
		{"ad_group_name", revenue.AdGroupName}, {"ad_group_priority", revenue.AdGroupPriority},
		{"ad_placement_id", revenue.AdPlacementID},
	}
	for _, value := range values {
		if value.value != "" {
			attributes[value.key] = value.value
		}
	}
}

func addSKAN(form url.Values, skan *SKANData) {
	if skan == nil {
		return
	}
	if skan.ConversionValue != nil {
		form.Set("skan_conversion_value", strconv.Itoa(*skan.ConversionValue))
	}
	if skan.FirstCallTimestamp != nil {
		form.Set("skan_first_call_timestamp", strconv.FormatInt(*skan.FirstCallTimestamp, 10))
	}
	if skan.LastCallTimestamp != nil {
		form.Set("skan_last_call_timestamp", strconv.FormatInt(*skan.LastCallTimestamp, 10))
	}
}

func setOptional(form url.Values, key, value string) {
	if value != "" {
		form.Set(key, value)
	}
}

func setJSON(form url.Values, key string, value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	form.Set(key, string(encoded))
	return nil
}

func boolInt(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

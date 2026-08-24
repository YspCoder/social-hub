package marketing

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListAds(ctx context.Context, input ListAdsRequest, options ...socialhub.CallOption) (NumberPage[Ad], error) {
	const operation = "ad_list"
	if !validateIDs(input.IDs, 100) || !validateIDs(input.CampaignIDs, 100) ||
		!validateIDs(input.AdGroupIDs, 100) || !validateFields(input.Fields, 100) ||
		input.PrimaryStatus != "" && !validEnumToken(input.PrimaryStatus) ||
		input.SecondaryStatus != "" && !validEnumToken(input.SecondaryStatus) {
		return NumberPage[Ad]{}, invalidArgument(operation, "Ad filters or fields are invalid")
	}
	page, pageSize, err := validatePage(input.Page, input.PageSize)
	if err != nil {
		return NumberPage[Ad]{}, err
	}
	query := url.Values{
		"advertiser_id": {client.advertiserID}, "page": {strconv.Itoa(page)}, "page_size": {strconv.Itoa(pageSize)},
	}
	filtering := map[string]any{}
	if len(input.IDs) > 0 {
		filtering["ad_ids"] = input.IDs
	}
	if len(input.CampaignIDs) > 0 {
		filtering["campaign_ids"] = input.CampaignIDs
	}
	if len(input.AdGroupIDs) > 0 {
		filtering["adgroup_ids"] = input.AdGroupIDs
	}
	if input.PrimaryStatus != "" {
		filtering["primary_status"] = input.PrimaryStatus
	}
	if input.SecondaryStatus != "" {
		filtering["secondary_status"] = input.SecondaryStatus
	}
	if len(filtering) > 0 {
		if err := setJSONQuery(query, "filtering", filtering, operation); err != nil {
			return NumberPage[Ad]{}, err
		}
	}
	if len(input.Fields) > 0 {
		if err := setJSONQuery(query, "fields", input.Fields, operation); err != nil {
			return NumberPage[Ad]{}, err
		}
	}
	var response apiEnvelope[struct {
		List     []Ad      `json:"list"`
		PageInfo *pageInfo `json:"page_info"`
	}]
	header, err := client.getJSON(ctx, operation, "/v1.3/ad/get/", query, &response, options...)
	if err != nil {
		return NumberPage[Ad]{}, err
	}
	data, err := requireEnvelope(operation, response, header)
	if err != nil {
		return NumberPage[Ad]{}, err
	}
	for index := range data.List {
		if err := validateReturnedAd(operation, client.advertiserID, "", data.List[index]); err != nil {
			return NumberPage[Ad]{}, err
		}
		data.List[index].AdvertiserID = client.advertiserID
	}
	return numberPage(operation, data.List, data.PageInfo)
}

func (client *Client) CreateAds(ctx context.Context, input CreateAdsRequest, options ...socialhub.CallOption) ([]Ad, error) {
	const operation = "ad_create"
	if !validID(input.AdGroupID) || len(input.Creatives) == 0 || len(input.Creatives) > 20 {
		return nil, invalidArgument(operation, "an Ad Group ID and between one and 20 creatives are required")
	}
	creatives := make([]map[string]any, 0, len(input.Creatives))
	for _, creative := range input.Creatives {
		if !validRequiredText(creative.Name, 512) ||
			creative.IdentityType != "" && !validEnumToken(creative.IdentityType) ||
			creative.AdFormat != "" && !validEnumToken(creative.AdFormat) ||
			creative.AdText != "" && !validRequiredText(creative.AdText, 4096) ||
			creative.CallToAction != "" && !validEnumToken(creative.CallToAction) ||
			creative.IdentityID != "" && !validOpaque(creative.IdentityID, 256) ||
			creative.IdentityAuthorizedBCID != "" && !validID(creative.IdentityAuthorizedBCID) ||
			creative.VideoID != "" && !validOpaque(creative.VideoID, 256) ||
			creative.TikTokItemID != "" && !validOpaque(creative.TikTokItemID, 256) ||
			creative.LandingPageURL != "" && !validHTTPURL(creative.LandingPageURL) {
			return nil, invalidArgument(operation, "one or more Ad creative fields are invalid")
		}
		if len(creative.ImageIDs) > 10 {
			return nil, invalidArgument(operation, "image_ids exceeds the supported request size")
		}
		for _, imageID := range creative.ImageIDs {
			if !validOpaque(imageID, 256) {
				return nil, invalidArgument(operation, "image_ids contain an invalid asset ID")
			}
		}
		fixed := map[string]any{"ad_name": creative.Name, "operation_status": StatusDisable}
		optionalStrings := map[string]string{
			"identity_type": creative.IdentityType, "identity_id": creative.IdentityID,
			"identity_authorized_bc_id": creative.IdentityAuthorizedBCID, "ad_format": creative.AdFormat,
			"video_id": creative.VideoID, "tiktok_item_id": creative.TikTokItemID,
			"ad_text": creative.AdText, "call_to_action": creative.CallToAction,
			"landing_page_url": creative.LandingPageURL,
		}
		for key, value := range optionalStrings {
			if value != "" {
				fixed[key] = value
			}
		}
		if len(creative.ImageIDs) > 0 {
			fixed["image_ids"] = creative.ImageIDs
		}
		merged, err := mergeFields(operation, fixed, creative.Fields, "ad_id", "ad_name", "operation_status")
		if err != nil {
			return nil, err
		}
		creatives = append(creatives, merged)
	}
	body := map[string]any{"advertiser_id": client.advertiserID, "adgroup_id": input.AdGroupID, "creatives": creatives}
	type createData struct {
		AdIDs []string `json:"ad_ids"`
		Ads   []Ad     `json:"ads"`
		List  []Ad     `json:"list"`
	}
	var response apiEnvelope[json.RawMessage]
	header, err := client.postJSON(ctx, operation, "/v1.3/ad/create/", body, &response, options...)
	if err != nil {
		return nil, err
	}
	rawData, err := requireEnvelope(operation, response, header)
	if err != nil {
		return nil, err
	}
	var data createData
	if err := json.Unmarshal(*rawData, &data); err != nil {
		return nil, platformContractError(operation, "TikTok returned invalid Ad creation data")
	}
	ads := data.Ads
	if len(ads) == 0 {
		ads = data.List
	}
	if len(ads) == 0 && len(data.AdIDs) > 0 {
		ads = make([]Ad, len(data.AdIDs))
		for index, id := range data.AdIDs {
			ads[index] = Ad{ID: id}
		}
	}
	if len(ads) == 0 {
		var single Ad
		if json.Unmarshal(*rawData, &single) == nil && single.ID != "" {
			ads = []Ad{single}
		}
	}
	if len(ads) != len(input.Creatives) {
		return nil, platformContractError(operation, "TikTok did not return one Ad for each requested creative")
	}
	for index := range ads {
		if err := validateReturnedAd(operation, client.advertiserID, input.AdGroupID, ads[index]); err != nil {
			return nil, err
		}
		if ads[index].OperationStatus != "" && ads[index].OperationStatus != StatusDisable {
			return nil, platformContractError(operation, "TikTok did not create every Ad paused")
		}
		ads[index].AdvertiserID, ads[index].AdGroupID, ads[index].OperationStatus = client.advertiserID, input.AdGroupID, StatusDisable
		if ads[index].Name == "" {
			ads[index].Name = input.Creatives[index].Name
		}
	}
	return ads, nil
}

func (client *Client) SetAdStatus(ctx context.Context, adID string, status OperationStatus, options ...socialhub.CallOption) (BatchResult, error) {
	return client.setStatus(ctx, "ad_status_update", "/v1.3/ad/status/update/", "ad_ids", adID, status, options...)
}

func validateReturnedAd(operation, advertiserID, adGroupID string, ad Ad) error {
	if err := requireResourceID(operation, "", ad.ID); err != nil {
		return err
	}
	if err := requireAdvertiser(operation, advertiserID, ad.AdvertiserID); err != nil {
		return err
	}
	if adGroupID != "" && ad.AdGroupID != "" && ad.AdGroupID != adGroupID {
		return platformContractError(operation, "TikTok returned an Ad for another Ad Group")
	}
	return nil
}

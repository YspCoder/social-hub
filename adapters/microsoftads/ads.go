package microsoftads

import (
	"context"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListResponsiveSearchAds(ctx context.Context, campaignID, adGroupID string, options ...socialhub.CallOption) ([]ResponsiveSearchAd, error) {
	const operation = "list_responsive_search_ads"
	if !validNumericID(campaignID) || !validNumericID(adGroupID) {
		return nil, invalidArgument(operation, "campaign and ad group IDs must be nonzero numeric IDs")
	}
	if _, err := client.GetAdGroup(ctx, campaignID, adGroupID, options...); err != nil {
		return nil, err
	}
	var response struct {
		Ads []ResponsiveSearchAd `json:"Ads"`
	}
	_, err := client.postJSON(ctx, operation, client.campaign, "/Ads/QueryByAdGroupId", struct {
		AdGroupID string   `json:"AdGroupId"`
		AdTypes   []string `json:"AdTypes"`
	}{AdGroupID: adGroupID, AdTypes: []string{"ResponsiveSearch"}}, &response, options...)
	if err != nil {
		return nil, err
	}
	return response.Ads, nil
}

func (client *Client) GetResponsiveSearchAd(ctx context.Context, campaignID, adGroupID, adID string, options ...socialhub.CallOption) (*ResponsiveSearchAd, error) {
	const operation = "get_responsive_search_ad"
	if !validNumericID(campaignID) || !validNumericID(adGroupID) || !validNumericID(adID) {
		return nil, invalidArgument(operation, "campaign, ad group, and ad IDs must be nonzero numeric IDs")
	}
	if _, err := client.GetAdGroup(ctx, campaignID, adGroupID, options...); err != nil {
		return nil, err
	}
	var response struct {
		Ads           []ResponsiveSearchAd `json:"Ads"`
		PartialErrors []wireFailure        `json:"PartialErrors"`
	}
	header, err := client.postJSON(ctx, operation, client.campaign, "/Ads/QueryByIds", struct {
		AdGroupID string   `json:"AdGroupId"`
		AdIDs     []string `json:"AdIds"`
		AdTypes   []string `json:"AdTypes"`
	}{AdGroupID: adGroupID, AdIDs: []string{adID}, AdTypes: []string{"ResponsiveSearch"}}, &response, options...)
	if err != nil {
		return nil, err
	}
	if err := checkPartialErrors(operation, header, response.PartialErrors); err != nil {
		return nil, err
	}
	if len(response.Ads) != 1 || response.Ads[0].ID != adID || response.Ads[0].Type != "ResponsiveSearch" {
		return nil, platformContractError(operation, "response ad does not match requested ad group, ID, and type")
	}
	return &response.Ads[0], nil
}

func (client *Client) CreateResponsiveSearchAd(ctx context.Context, campaignID, adGroupID string, input CreateResponsiveSearchAdRequest, options ...socialhub.CallOption) (*ResponsiveSearchAd, error) {
	const operation = "create_responsive_search_ad"
	if !validNumericID(campaignID) || !validNumericID(adGroupID) || !validateFinalURLs(input.FinalURLs) ||
		!validateTextAssets(input.Headlines, 3, 15) || !validateTextAssets(input.Descriptions, 2, 4) ||
		!validAdPath(input.Path1) || !validAdPath(input.Path2) {
		return nil, invalidArgument(operation, "ad group ID, final URLs, 3-15 headlines, 2-4 descriptions, and paths must be valid")
	}
	if err := client.validateAccount(ctx, options...); err != nil {
		return nil, err
	}
	if _, err := client.GetAdGroup(ctx, campaignID, adGroupID, options...); err != nil {
		return nil, err
	}
	payload := responsiveSearchAdWrite{
		Type: stringPointer("ResponsiveSearch"), Status: statusPointer(StatusPaused), FinalURLs: &input.FinalURLs,
		Headlines: assetLinksPointer(input.Headlines), Descriptions: assetLinksPointer(input.Descriptions),
		Path1: optionalStringPointer(input.Path1), Path2: optionalStringPointer(input.Path2),
	}
	var response struct {
		AdIDs         []*string     `json:"AdIds"`
		PartialErrors []wireFailure `json:"PartialErrors"`
	}
	header, err := client.postJSON(ctx, operation, client.campaign, "/Ads", struct {
		AdGroupID string                    `json:"AdGroupId"`
		Ads       []responsiveSearchAdWrite `json:"Ads"`
	}{AdGroupID: adGroupID, Ads: []responsiveSearchAdWrite{payload}}, &response, options...)
	if err != nil {
		return nil, err
	}
	if err := checkPartialErrors(operation, header, response.PartialErrors); err != nil {
		return nil, err
	}
	if len(response.AdIDs) != 1 || response.AdIDs[0] == nil || !validNumericID(*response.AdIDs[0]) {
		return nil, platformContractError(operation, "response did not contain one ad ID")
	}
	return client.GetResponsiveSearchAd(ctx, campaignID, adGroupID, *response.AdIDs[0], options...)
}

func (client *Client) UpdateResponsiveSearchAd(ctx context.Context, campaignID, adGroupID, adID string, input UpdateResponsiveSearchAdRequest, options ...socialhub.CallOption) (*ResponsiveSearchAd, error) {
	const operation = "update_responsive_search_ad"
	if !validNumericID(campaignID) || !validNumericID(adGroupID) || !validNumericID(adID) || input.empty() ||
		(input.FinalURLs != nil && !validateFinalURLs(*input.FinalURLs)) ||
		(input.Headlines != nil && !validateTextAssets(*input.Headlines, 3, 15)) ||
		(input.Descriptions != nil && !validateTextAssets(*input.Descriptions, 2, 4)) ||
		(input.Path1 != nil && !validAdPath(*input.Path1)) || (input.Path2 != nil && !validAdPath(*input.Path2)) {
		return nil, invalidArgument(operation, "IDs and at least one valid ad update field are required")
	}
	if err := client.validateAccount(ctx, options...); err != nil {
		return nil, err
	}
	if _, err := client.GetResponsiveSearchAd(ctx, campaignID, adGroupID, adID, options...); err != nil {
		return nil, err
	}
	payload := responsiveSearchAdWrite{ID: adID, Type: stringPointer("ResponsiveSearch"), FinalURLs: input.FinalURLs, Path1: input.Path1, Path2: input.Path2}
	if input.Headlines != nil {
		payload.Headlines = assetLinksPointer(*input.Headlines)
	}
	if input.Descriptions != nil {
		payload.Descriptions = assetLinksPointer(*input.Descriptions)
	}
	if err := client.updateResponsiveSearchAd(ctx, operation, adGroupID, payload, options...); err != nil {
		return nil, err
	}
	return client.GetResponsiveSearchAd(ctx, campaignID, adGroupID, adID, options...)
}

func (client *Client) SetResponsiveSearchAdStatus(ctx context.Context, campaignID, adGroupID, adID string, status Status, options ...socialhub.CallOption) (*ResponsiveSearchAd, error) {
	const operation = "set_responsive_search_ad_status"
	if !validNumericID(campaignID) || !validNumericID(adGroupID) || !validNumericID(adID) || !validStatus(status) {
		return nil, invalidArgument(operation, "IDs and Active or Paused status are required")
	}
	if err := client.validateAccount(ctx, options...); err != nil {
		return nil, err
	}
	if _, err := client.GetResponsiveSearchAd(ctx, campaignID, adGroupID, adID, options...); err != nil {
		return nil, err
	}
	payload := responsiveSearchAdWrite{ID: adID, Type: stringPointer("ResponsiveSearch"), Status: &status}
	if err := client.updateResponsiveSearchAd(ctx, operation, adGroupID, payload, options...); err != nil {
		return nil, err
	}
	return client.GetResponsiveSearchAd(ctx, campaignID, adGroupID, adID, options...)
}

type responsiveSearchAdWrite struct {
	ID           string       `json:"Id,omitempty"`
	Type         *string      `json:"Type,omitempty"`
	Status       *Status      `json:"Status,omitempty"`
	FinalURLs    *[]string    `json:"FinalUrls,omitempty"`
	Headlines    *[]AssetLink `json:"Headlines,omitempty"`
	Descriptions *[]AssetLink `json:"Descriptions,omitempty"`
	Path1        *string      `json:"Path1,omitempty"`
	Path2        *string      `json:"Path2,omitempty"`
}

func (client *Client) updateResponsiveSearchAd(ctx context.Context, operation, adGroupID string, payload responsiveSearchAdWrite, options ...socialhub.CallOption) error {
	var response struct {
		PartialErrors []wireFailure `json:"PartialErrors"`
	}
	header, err := client.putJSON(ctx, operation, "/Ads", struct {
		AdGroupID string                    `json:"AdGroupId"`
		Ads       []responsiveSearchAdWrite `json:"Ads"`
	}{AdGroupID: adGroupID, Ads: []responsiveSearchAdWrite{payload}}, &response, options...)
	if err != nil {
		return err
	}
	return checkPartialErrors(operation, header, response.PartialErrors)
}

func assetLinksPointer(values []AdTextAsset) *[]AssetLink {
	links := make([]AssetLink, len(values))
	for index, value := range values {
		links[index] = AssetLink{Asset: TextAsset{Type: "TextAsset", Text: value.Text}, PinnedField: value.PinnedField}
	}
	return &links
}

func validAdPath(value string) bool {
	return value == "" || (validRequiredText(value, 15) && !strings.ContainsAny(value, "/?#"))
}

func (input UpdateResponsiveSearchAdRequest) empty() bool {
	return input.FinalURLs == nil && input.Headlines == nil && input.Descriptions == nil && input.Path1 == nil && input.Path2 == nil
}

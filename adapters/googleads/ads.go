package googleads

import (
	"context"

	"social-hub/pkg/socialhub"
)

const responsiveSearchAdQuery = "SELECT ad_group_ad.resource_name, ad_group_ad.ad_group, ad_group_ad.status, ad_group_ad.primary_status, ad_group_ad.ad_strength, ad_group_ad.ad.resource_name, ad_group_ad.ad.id, ad_group_ad.ad.name, ad_group_ad.ad.type, ad_group_ad.ad.final_urls, ad_group_ad.ad.responsive_search_ad.headlines, ad_group_ad.ad.responsive_search_ad.descriptions, ad_group_ad.ad.responsive_search_ad.path1, ad_group_ad.ad.responsive_search_ad.path2 FROM ad_group_ad WHERE ad_group_ad.ad.type = RESPONSIVE_SEARCH_AD"

type adGroupAdRow struct {
	AdGroupAd AdGroupAd `json:"adGroupAd"`
}

type adGroupAdMutateResponse struct {
	Results []struct {
		ResourceName string    `json:"resourceName"`
		AdGroupAd    AdGroupAd `json:"adGroupAd"`
	} `json:"results"`
}

type adMutateResponse struct {
	Results []struct {
		ResourceName string `json:"resourceName"`
		Ad           Ad     `json:"ad"`
	} `json:"results"`
}

func (client *Client) ListResponsiveSearchAds(ctx context.Context, input ListAdsRequest, options ...socialhub.CallOption) (TokenPage[AdGroupAd], error) {
	const operation = "responsive_search_ad_list"
	if !validPageToken(input.PageToken) || input.AdGroupResourceName != "" && !validResourceName(client.customerID, "adGroups", input.AdGroupResourceName) {
		return TokenPage[AdGroupAd]{}, invalidArgument(operation, "page token or Ad Group resource name is invalid")
	}
	query := responsiveSearchAdQuery
	if input.AdGroupResourceName != "" {
		query += " AND ad_group_ad.ad_group = '" + input.AdGroupResourceName + "'"
	}
	query += " ORDER BY ad_group_ad.ad.id"
	response, err := searchRows[adGroupAdRow](ctx, client, operation, query, input.PageToken, false, options...)
	if err != nil {
		return TokenPage[AdGroupAd]{}, err
	}
	items := make([]AdGroupAd, len(response.Results))
	for index, row := range response.Results {
		if err := client.validateAdGroupAd(operation, row.AdGroupAd, input.AdGroupResourceName, true); err != nil {
			return TokenPage[AdGroupAd]{}, err
		}
		items[index] = row.AdGroupAd
	}
	return TokenPage[AdGroupAd]{Items: items, NextPageToken: response.NextPageToken}, nil
}

func (client *Client) CreateResponsiveSearchAd(ctx context.Context, input CreateResponsiveSearchAdRequest, options ...socialhub.CallOption) (*AdGroupAd, error) {
	const operation = "responsive_search_ad_create"
	if !validResourceName(client.customerID, "adGroups", input.AdGroupResourceName) ||
		input.Name != "" && !validRequiredText(input.Name, 255) || !validateFinalURLs(input.FinalURLs) ||
		!validateTextAssets(input.Headlines, 3, 15, 30) || !validateTextAssets(input.Descriptions, 2, 4, 90) ||
		!validOptionalPath(input.Path1) || !validOptionalPath(input.Path2) {
		return nil, invalidArgument(operation, "Ad Group, final URLs, RSA text assets, name, or display paths are invalid")
	}
	responsive := map[string]any{"headlines": input.Headlines, "descriptions": input.Descriptions}
	if input.Path1 != "" {
		responsive["path1"] = input.Path1
	}
	if input.Path2 != "" {
		responsive["path2"] = input.Path2
	}
	fixed := map[string]any{"finalUrls": input.FinalURLs, "responsiveSearchAd": responsive}
	if input.Name != "" {
		fixed["name"] = input.Name
	}
	ad, err := mergeFields(operation, fixed, input.Fields, "id", "type")
	if err != nil {
		return nil, err
	}
	return client.mutateAdGroupAd(ctx, operation, map[string]any{
		"create": map[string]any{"adGroup": input.AdGroupResourceName, "status": StatusPaused, "ad": ad},
	}, input.AdGroupResourceName, true, options...)
}

func (client *Client) UpdateResponsiveSearchAd(ctx context.Context, resourceName string, input UpdateResponsiveSearchAdRequest, options ...socialhub.CallOption) (*Ad, error) {
	const operation = "responsive_search_ad_update"
	if !validResourceName(client.customerID, "ads", resourceName) {
		return nil, invalidArgument(operation, "Ad resource name is invalid or belongs to another Customer")
	}
	resource := map[string]any{"resourceName": resourceName}
	mask := make([]string, 0, 6)
	if input.Name != nil {
		if *input.Name != "" && !validRequiredText(*input.Name, 255) {
			return nil, invalidArgument(operation, "name is invalid")
		}
		resource["name"] = *input.Name
		mask = append(mask, "name")
	}
	if input.FinalURLs != nil {
		if !validateFinalURLs(*input.FinalURLs) {
			return nil, invalidArgument(operation, "final URLs are invalid")
		}
		resource["finalUrls"] = *input.FinalURLs
		mask = append(mask, "final_urls")
	}
	responsive := map[string]any{}
	if input.Headlines != nil {
		if !validateTextAssets(*input.Headlines, 3, 15, 30) {
			return nil, invalidArgument(operation, "headlines are invalid")
		}
		responsive["headlines"] = *input.Headlines
		mask = append(mask, "responsive_search_ad.headlines")
	}
	if input.Descriptions != nil {
		if !validateTextAssets(*input.Descriptions, 2, 4, 90) {
			return nil, invalidArgument(operation, "descriptions are invalid")
		}
		responsive["descriptions"] = *input.Descriptions
		mask = append(mask, "responsive_search_ad.descriptions")
	}
	if input.Path1 != nil {
		if !validOptionalPath(*input.Path1) {
			return nil, invalidArgument(operation, "path1 is invalid")
		}
		responsive["path1"] = *input.Path1
		mask = append(mask, "responsive_search_ad.path1")
	}
	if input.Path2 != nil {
		if !validOptionalPath(*input.Path2) {
			return nil, invalidArgument(operation, "path2 is invalid")
		}
		responsive["path2"] = *input.Path2
		mask = append(mask, "responsive_search_ad.path2")
	}
	if len(responsive) > 0 {
		resource["responsiveSearchAd"] = responsive
	}
	if len(mask) == 0 {
		return nil, invalidArgument(operation, "at least one mutable field is required")
	}
	body := map[string]any{
		"operations":          []any{map[string]any{"update": resource, "updateMask": updateMask(mask)}},
		"responseContentType": "MUTABLE_RESOURCE",
	}
	var response adMutateResponse
	if _, err := client.postJSON(ctx, operation, client.mutatePath("ads"), body, &response, options...); err != nil {
		return nil, err
	}
	if len(response.Results) != 1 {
		return nil, platformContractError(operation, "Google Ads returned an invalid Ad mutate result count")
	}
	result := response.Results[0]
	if err := requireResourceName(operation, client.customerID, "ads", result.ResourceName); err != nil {
		return nil, err
	}
	if result.Ad.ResourceName == "" {
		result.Ad.ResourceName = result.ResourceName
	}
	if result.Ad.ResourceName != result.ResourceName {
		return nil, platformContractError(operation, "Google Ads returned mismatched Ad resource names")
	}
	return &result.Ad, nil
}

func (client *Client) SetAdStatus(ctx context.Context, resourceName string, status Status, options ...socialhub.CallOption) (*AdGroupAd, error) {
	const operation = "ad_status"
	if !validResourceName(client.customerID, "adGroupAds", resourceName) || !validStatus(status) {
		return nil, invalidArgument(operation, "AdGroupAd resource name or status is invalid")
	}
	return client.mutateAdGroupAd(ctx, operation, map[string]any{
		"update": map[string]any{"resourceName": resourceName, "status": status}, "updateMask": "status",
	}, "", false, options...)
}

func (client *Client) RemoveAd(ctx context.Context, resourceName string, options ...socialhub.CallOption) error {
	const operation = "ad_remove"
	if !validResourceName(client.customerID, "adGroupAds", resourceName) {
		return invalidArgument(operation, "AdGroupAd resource name is invalid or belongs to another Customer")
	}
	result, err := client.mutateAdGroupAd(ctx, operation, map[string]any{"remove": resourceName}, "", false, options...)
	if err != nil {
		return err
	}
	if result.ResourceName != resourceName {
		return platformContractError(operation, "Google Ads returned a different removed AdGroupAd")
	}
	return nil
}

func (client *Client) mutateAdGroupAd(ctx context.Context, operation string, mutateOperation map[string]any, expectedAdGroup string, requireAd bool, options ...socialhub.CallOption) (*AdGroupAd, error) {
	body := map[string]any{"operations": []any{mutateOperation}, "responseContentType": "MUTABLE_RESOURCE"}
	var response adGroupAdMutateResponse
	if _, err := client.postJSON(ctx, operation, client.mutatePath("adGroupAds"), body, &response, options...); err != nil {
		return nil, err
	}
	if len(response.Results) != 1 {
		return nil, platformContractError(operation, "Google Ads returned an invalid AdGroupAd mutate result count")
	}
	result := response.Results[0]
	if err := requireResourceName(operation, client.customerID, "adGroupAds", result.ResourceName); err != nil {
		return nil, err
	}
	if result.AdGroupAd.ResourceName == "" {
		result.AdGroupAd.ResourceName = result.ResourceName
	}
	if result.AdGroupAd.ResourceName != result.ResourceName {
		return nil, platformContractError(operation, "Google Ads returned mismatched AdGroupAd resource names")
	}
	if err := client.validateAdGroupAd(operation, result.AdGroupAd, expectedAdGroup, requireAd); err != nil {
		return nil, err
	}
	return &result.AdGroupAd, nil
}

func (client *Client) validateAdGroupAd(operation string, value AdGroupAd, expectedAdGroup string, requireAd bool) error {
	if err := requireResourceName(operation, client.customerID, "adGroupAds", value.ResourceName); err != nil {
		return err
	}
	if value.AdGroup != "" && !validResourceName(client.customerID, "adGroups", value.AdGroup) ||
		expectedAdGroup != "" && value.AdGroup != expectedAdGroup {
		return platformContractError(operation, "Google Ads returned an AdGroupAd for another Ad Group or Customer")
	}
	if value.Ad.ResourceName == "" {
		if requireAd {
			return platformContractError(operation, "Google Ads omitted the mutable Ad resource")
		}
		return nil
	}
	return requireResourceName(operation, client.customerID, "ads", value.Ad.ResourceName)
}

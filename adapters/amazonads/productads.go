package amazonads

import (
	"context"
	"net/http"

	"social-hub/pkg/socialhub"
)

type productAdListEnvelope struct {
	ProductAds   []ProductAd `json:"productAds"`
	NextToken    string      `json:"nextToken"`
	TotalResults int         `json:"totalResults"`
}

type productAdMutationEnvelope struct {
	ProductAds struct {
		Success []struct {
			Index     int       `json:"index"`
			AdID      string    `json:"adId"`
			ProductAd ProductAd `json:"productAd"`
		} `json:"success"`
		Error []mutationFailure `json:"error"`
	} `json:"productAds"`
}

func (client *Client) ListProductAds(ctx context.Context, input ListProductAdsRequest, options ...socialhub.CallOption) (Page[ProductAd], error) {
	const operation = "product_ads_list"
	if !validIDs(input.IDs) || !validIDs(input.CampaignIDs) || !validIDs(input.AdGroupIDs) || !validStates(input.States) || !validList(input.MaxResults, input.NextToken) {
		return Page[ProductAd]{}, invalidArgument(operation, "IDs, states, max results, or next token are invalid")
	}
	body := struct {
		AdIDFilter       *includeFilter[string] `json:"adIdFilter,omitempty"`
		CampaignIDFilter *includeFilter[string] `json:"campaignIdFilter,omitempty"`
		AdGroupIDFilter  *includeFilter[string] `json:"adGroupIdFilter,omitempty"`
		StateFilter      *includeFilter[State]  `json:"stateFilter,omitempty"`
		MaxResults       int                    `json:"maxResults,omitempty"`
		NextToken        string                 `json:"nextToken,omitempty"`
	}{MaxResults: input.MaxResults, NextToken: input.NextToken}
	if len(input.IDs) > 0 {
		body.AdIDFilter = &includeFilter[string]{Include: input.IDs}
	}
	if len(input.CampaignIDs) > 0 {
		body.CampaignIDFilter = &includeFilter[string]{Include: input.CampaignIDs}
	}
	if len(input.AdGroupIDs) > 0 {
		body.AdGroupIDFilter = &includeFilter[string]{Include: input.AdGroupIDs}
	}
	if len(input.States) > 0 {
		body.StateFilter = &includeFilter[State]{Include: input.States}
	}
	var response productAdListEnvelope
	if _, err := client.vendorJSON(ctx, operation, http.MethodPost, "/sp/productAds/list", productAdMediaType, body, &response, false, options...); err != nil {
		return Page[ProductAd]{}, err
	}
	for _, ad := range response.ProductAds {
		if !validID(ad.ID) || !validID(ad.CampaignID) || !validID(ad.AdGroupID) {
			return Page[ProductAd]{}, platformContractError(operation, "Amazon Ads returned an invalid Product Ad, Campaign, or Ad Group ID")
		}
	}
	return Page[ProductAd]{Items: response.ProductAds, NextToken: response.NextToken, TotalResults: response.TotalResults}, nil
}

func (client *Client) CreateProductAd(ctx context.Context, input CreateProductAdRequest, options ...socialhub.CallOption) (*ProductAd, error) {
	const operation = "product_ad_create"
	asinValid := input.ASIN == "" || validASIN(input.ASIN)
	skuValid := input.SKU == "" || validText(input.SKU, 400)
	if !validID(input.CampaignID) || !validID(input.AdGroupID) || !asinValid || !skuValid || (input.ASIN == "") == (input.SKU == "") || input.CustomText != "" && !validText(input.CustomText, 256) {
		return nil, invalidArgument(operation, "Campaign ID, Ad Group ID, exactly one ASIN or SKU, or custom text is invalid")
	}
	resource := ProductAd{CampaignID: input.CampaignID, AdGroupID: input.AdGroupID, ASIN: input.ASIN, SKU: input.SKU, CustomText: input.CustomText, State: StatePaused}
	return client.mutateProductAd(ctx, operation, http.MethodPost, "/sp/productAds", resource, "", options...)
}

func (client *Client) SetProductAdState(ctx context.Context, id string, state State, options ...socialhub.CallOption) (*ProductAd, error) {
	if !validID(id) || !validState(state) {
		return nil, invalidArgument("product_ad_state", "Product Ad ID and ENABLED or PAUSED state are required")
	}
	return client.mutateProductAd(ctx, "product_ad_state", http.MethodPut, "/sp/productAds", ProductAd{ID: id, State: state}, id, options...)
}

func (client *Client) ArchiveProductAd(ctx context.Context, id string, options ...socialhub.CallOption) error {
	const operation = "product_ad_archive"
	if !validID(id) {
		return invalidArgument(operation, "Product Ad ID is invalid")
	}
	body := struct {
		Filter includeFilter[string] `json:"adIdFilter"`
	}{Filter: includeFilter[string]{Include: []string{id}}}
	var response productAdMutationEnvelope
	metadata, err := client.vendorJSON(ctx, operation, http.MethodPost, "/sp/productAds/delete", productAdMediaType, body, &response, true, options...)
	if err != nil {
		return err
	}
	_, err = productAdMutationResult(operation, id, metadata.StatusCode, metadata.Header, response)
	return err
}

func (client *Client) mutateProductAd(ctx context.Context, operation, method, path string, resource ProductAd, expected string, options ...socialhub.CallOption) (*ProductAd, error) {
	body := struct {
		ProductAds []ProductAd `json:"productAds"`
	}{ProductAds: []ProductAd{resource}}
	var response productAdMutationEnvelope
	metadata, err := client.vendorJSON(ctx, operation, method, path, productAdMediaType, body, &response, true, options...)
	if err != nil {
		return nil, err
	}
	return productAdMutationResult(operation, expected, metadata.StatusCode, metadata.Header, response)
}

func productAdMutationResult(operation, expected string, status int, header http.Header, response productAdMutationEnvelope) (*ProductAd, error) {
	if len(response.ProductAds.Error) > 0 {
		return nil, mutationError(operation, status, header, response.ProductAds.Error[0])
	}
	if len(response.ProductAds.Success) != 1 {
		return nil, platformContractError(operation, "Amazon Ads did not return exactly one Product Ad mutation result")
	}
	item := response.ProductAds.Success[0]
	if err := requireMutationID(operation, expected, item.AdID); err != nil {
		return nil, err
	}
	if item.ProductAd.ID == "" {
		item.ProductAd.ID = item.AdID
	}
	if item.ProductAd.ID != item.AdID {
		return nil, platformContractError(operation, "Amazon Ads returned mismatched Product Ad IDs")
	}
	return &item.ProductAd, nil
}

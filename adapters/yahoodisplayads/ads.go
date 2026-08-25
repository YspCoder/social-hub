package yahoodisplayads

import (
	"context"

	"social-hub/pkg/socialhub"
)

const adServicePath = "AdGroupAdService"

type adSelectorRequest struct {
	AccountID     int64        `json:"accountId"`
	CampaignIDs   []int64      `json:"campaignIds,omitempty"`
	AdGroupIDs    []int64      `json:"adGroupIds,omitempty"`
	AdIDs         []int64      `json:"adIds,omitempty"`
	UserStatuses  []UserStatus `json:"userStatuses,omitempty"`
	StartIndex    int32        `json:"startIndex,omitempty"`
	NumberResults int32        `json:"numberResults,omitempty"`
}

type adOperation struct {
	AccountID int64 `json:"accountId"`
	Operand   []Ad  `json:"operand"`
}

func (client *Client) ListAds(ctx context.Context, input AdSelector, options ...socialhub.CallOption) (Page[Ad], error) {
	const operation = "ad_list"
	if !validAdSelector(input) {
		return Page[Ad]{}, invalidArgument(operation, "campaign, ad group, ad IDs, statuses, or pagination are invalid")
	}
	request := adSelectorRequest{
		AccountID: client.advertiserAccountID, CampaignIDs: input.CampaignIDs,
		AdGroupIDs: input.AdGroupIDs, AdIDs: input.AdIDs, UserStatuses: input.UserStatuses,
		StartIndex: input.StartIndex, NumberResults: input.NumberResults,
	}
	return postPage(ctx, client, operation, adServicePath+"/get", request, input.PageRequest,
		MaximumAdPageSize, adEntity, func(value *Ad) error {
			return client.validateAd(operation, value, 0, 0, 0)
		}, options...)
}

func (client *Client) GetAd(ctx context.Context, campaignID, adGroupID, adID int64, options ...socialhub.CallOption) (*Ad, error) {
	const operation = "ad_get"
	if campaignID <= 0 || adGroupID <= 0 || adID <= 0 {
		return nil, invalidArgument(operation, "campaign, ad group, and ad IDs must be positive")
	}
	page, err := client.ListAds(ctx, AdSelector{
		CampaignIDs: []int64{campaignID}, AdGroupIDs: []int64{adGroupID}, AdIDs: []int64{adID},
		PageRequest: PageRequest{StartIndex: 1, NumberResults: 1},
	}, options...)
	if err != nil {
		return nil, withOperation(err, operation)
	}
	if len(page.Items) == 0 {
		return nil, notFound(operation, "ad was not returned")
	}
	if len(page.Items) != 1 {
		return nil, platformContractError(operation, "LINE Yahoo returned multiple ads for one ID")
	}
	if err := client.validateAd(operation, &page.Items[0], campaignID, adGroupID, adID); err != nil {
		return nil, err
	}
	return &page.Items[0], nil
}

func (client *Client) CreateBannerAds(ctx context.Context, campaignID, adGroupID int64, inputs []BannerAdAdd, options ...socialhub.CallOption) (MutationResult[Ad], error) {
	const operation = "banner_ad_create"
	if campaignID <= 0 || adGroupID <= 0 || len(inputs) == 0 || len(inputs) > MaximumAdMutationBatch {
		return MutationResult[Ad]{}, invalidArgument(operation, "campaign ID, ad group ID, and 1-2000 banner ads are required")
	}
	operands := make([]Ad, 0, len(inputs))
	for _, input := range inputs {
		if !validBannerAdAdd(input) {
			return MutationResult[Ad]{}, invalidArgument(operation, "banner ad name, media ID, or final URL is invalid")
		}
		operands = append(operands, Ad{
			CampaignID: campaignID, AdGroupID: adGroupID, AdName: input.Name,
			MediaID: input.MediaID, UserStatus: StatusPaused,
			Ad: &AdCreative{
				AdType: AdTypeBanner, MainMediaFormat: MediaFormatImage,
				BannerAd: &BannerAd{}, FinalURL: input.FinalURL,
			},
		})
	}
	return postMutation(ctx, client, operation, adServicePath+"/add",
		adOperation{AccountID: client.advertiserAccountID, Operand: operands}, len(operands),
		adEntity, func(value *Ad) error {
			return client.validateAdMutation(operation, value, campaignID, adGroupID)
		}, options...)
}

func (client *Client) UpdateAds(ctx context.Context, campaignID, adGroupID int64, inputs []AdUpdate, options ...socialhub.CallOption) (MutationResult[Ad], error) {
	const operation = "ad_update"
	if campaignID <= 0 || adGroupID <= 0 || len(inputs) == 0 || len(inputs) > MaximumAdMutationBatch {
		return MutationResult[Ad]{}, invalidArgument(operation, "campaign ID, ad group ID, and 1-2000 ad updates are required")
	}
	seen := make(map[int64]struct{}, len(inputs))
	operands := make([]Ad, 0, len(inputs))
	for _, input := range inputs {
		if !validAdUpdate(input) {
			return MutationResult[Ad]{}, invalidArgument(operation, "ad update is invalid")
		}
		if _, exists := seen[input.ID]; exists {
			return MutationResult[Ad]{}, invalidArgument(operation, "ad IDs must be unique")
		}
		seen[input.ID] = struct{}{}
		operand := Ad{CampaignID: campaignID, AdGroupID: adGroupID, AdID: input.ID}
		if input.Name != nil {
			operand.AdName = *input.Name
		}
		if input.FinalURL != nil {
			operand.Ad = &AdCreative{FinalURL: *input.FinalURL}
		}
		operands = append(operands, operand)
	}
	return postMutation(ctx, client, operation, adServicePath+"/set",
		adOperation{AccountID: client.advertiserAccountID, Operand: operands}, len(operands),
		adEntity, func(value *Ad) error {
			return client.validateAdMutation(operation, value, campaignID, adGroupID)
		}, options...)
}

func (client *Client) SetAdsEnabled(ctx context.Context, campaignID, adGroupID int64, ids []int64, enabled bool, options ...socialhub.CallOption) (MutationResult[Ad], error) {
	const operation = "ad_set_enabled"
	if campaignID <= 0 || adGroupID <= 0 || !validIDs(ids, MaximumAdMutationBatch, false) {
		return MutationResult[Ad]{}, invalidArgument(operation, "campaign ID, ad group ID, and 1-2000 unique ad IDs are required")
	}
	status := StatusPaused
	if enabled {
		status = StatusActive
	}
	operands := make([]Ad, 0, len(ids))
	for _, id := range ids {
		operands = append(operands, Ad{CampaignID: campaignID, AdGroupID: adGroupID, AdID: id, UserStatus: status})
	}
	return postMutation(ctx, client, operation, adServicePath+"/set",
		adOperation{AccountID: client.advertiserAccountID, Operand: operands}, len(operands),
		adEntity, func(value *Ad) error {
			return client.validateAdMutation(operation, value, campaignID, adGroupID)
		}, options...)
}

func (client *Client) DeleteAds(ctx context.Context, campaignID, adGroupID int64, ids []int64, options ...socialhub.CallOption) (MutationResult[Ad], error) {
	const operation = "ad_delete"
	if campaignID <= 0 || adGroupID <= 0 || !validIDs(ids, MaximumAdMutationBatch, false) {
		return MutationResult[Ad]{}, invalidArgument(operation, "campaign ID, ad group ID, and 1-2000 unique ad IDs are required")
	}
	prepared, err := prepareCallOptions(operation, options)
	if err != nil {
		return MutationResult[Ad]{}, err
	}
	if err := client.requirePausedAds(ctx, operation, campaignID, adGroupID, ids, prepared...); err != nil {
		return MutationResult[Ad]{}, err
	}
	operands := make([]Ad, 0, len(ids))
	for _, id := range ids {
		operands = append(operands, Ad{CampaignID: campaignID, AdGroupID: adGroupID, AdID: id})
	}
	return postMutation(ctx, client, operation, adServicePath+"/remove",
		adOperation{AccountID: client.advertiserAccountID, Operand: operands}, len(operands),
		adEntity, func(value *Ad) error {
			return client.validateAdMutation(operation, value, campaignID, adGroupID)
		}, prepared...)
}

func (client *Client) requirePausedAds(ctx context.Context, operation string, campaignID, adGroupID int64, ids []int64, options ...socialhub.CallOption) error {
	expected := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		expected[id] = struct{}{}
	}
	for start := 0; start < len(ids); start += MaximumAdSelectorIDs {
		end := start + MaximumAdSelectorIDs
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[start:end]
		page, err := client.ListAds(ctx, AdSelector{
			CampaignIDs: []int64{campaignID}, AdGroupIDs: []int64{adGroupID}, AdIDs: chunk,
			PageRequest: PageRequest{StartIndex: 1, NumberResults: int32(len(chunk))},
		}, options...)
		if err != nil {
			return withOperation(err, operation)
		}
		if len(page.Items) != len(chunk) {
			return notFound(operation, "one or more ads were not returned before delete")
		}
		for _, ad := range page.Items {
			if ad.CampaignID != campaignID || ad.AdGroupID != adGroupID {
				return platformContractError(operation, "LINE Yahoo returned an ad outside the requested parent resources")
			}
			if _, exists := expected[ad.AdID]; !exists {
				return platformContractError(operation, "LINE Yahoo returned a duplicate ad or one outside the delete selection")
			}
			delete(expected, ad.AdID)
			if ad.UserStatus != StatusPaused {
				return invalidArgument(operation, "ads must be PAUSED before delete")
			}
		}
	}
	if len(expected) != 0 {
		return platformContractError(operation, "LINE Yahoo omitted one or more ads from the delete preflight")
	}
	return nil
}

func (client *Client) validateAd(operation string, value *Ad, expectedCampaignID, expectedAdGroupID, expectedID int64) error {
	if value == nil || value.AccountID != client.advertiserAccountID || value.CampaignID <= 0 ||
		value.AdGroupID <= 0 || value.AdID <= 0 || !validText(value.AdName, 50) ||
		!validReturnedUserStatus(value.UserStatus) || value.Ad == nil ||
		!validOpaque(string(value.Ad.AdType), 64) || !validOpaque(string(value.Ad.MainMediaFormat), 64) {
		return platformContractError(operation, "LINE Yahoo returned an invalid ad")
	}
	if expectedCampaignID > 0 && value.CampaignID != expectedCampaignID ||
		expectedAdGroupID > 0 && value.AdGroupID != expectedAdGroupID || expectedID > 0 && value.AdID != expectedID {
		return platformContractError(operation, "ad parent or ID did not match the request")
	}
	return nil
}

func (client *Client) validateAdMutation(operation string, value *Ad, expectedCampaignID, expectedAdGroupID int64) error {
	if value == nil || value.AdID <= 0 ||
		value.AccountID != 0 && value.AccountID != client.advertiserAccountID ||
		value.CampaignID != 0 && value.CampaignID != expectedCampaignID ||
		value.AdGroupID != 0 && value.AdGroupID != expectedAdGroupID {
		return platformContractError(operation, "LINE Yahoo returned an invalid ad mutation value")
	}
	return nil
}

var _ AdWorkflow = (*Client)(nil)

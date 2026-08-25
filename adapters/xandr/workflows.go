package xandr

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

const (
	advertiserPath = "/advertiser"
	campaignPath   = "/campaign"
)

func (client *Client) GetAdvertiser(ctx context.Context, advertiserID int64, options ...socialhub.CallOption) (*Advertiser, error) {
	const operation = "advertiser_get"
	if advertiserID <= 0 {
		return nil, invalidArgument(operation, "advertiser ID must be positive")
	}
	query := url.Values{"id": {strconv.FormatInt(advertiserID, 10)}}
	response, meta, err := client.doGET(ctx, operation, advertiserPath, query, options...)
	if err != nil {
		return nil, err
	}
	advertiser, err := decodeAdvertiser(operation, response.Advertiser)
	if err != nil {
		return nil, err
	}
	if advertiser.ID != advertiserID {
		return nil, platformContractError(operation, "Xandr returned a different advertiser")
	}
	advertiser.Meta = meta
	return &advertiser, nil
}

func (client *Client) ListAdvertisers(ctx context.Context, input ListOptions, options ...socialhub.CallOption) (*AdvertiserPage, error) {
	const operation = "advertiser_list"
	if !validListOptions(input) {
		return nil, invalidArgument(operation, "state, search, start_element, or num_elements is invalid")
	}
	query, pageSize := listQuery(input)
	response, meta, err := client.doGET(ctx, operation, advertiserPath, query, options...)
	if err != nil {
		return nil, err
	}
	rawAdvertisers, err := decodeRawList(operation, "advertisers", response.Advertisers)
	if err != nil {
		return nil, err
	}
	if len(rawAdvertisers) > pageSize {
		return nil, platformContractError(operation, "Xandr returned more advertisers than requested")
	}
	advertisers := make([]Advertiser, 0, len(rawAdvertisers))
	seen := make(map[int64]struct{}, len(rawAdvertisers))
	for _, raw := range rawAdvertisers {
		advertiser, err := decodeAdvertiser(operation, raw)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[advertiser.ID]; exists {
			return nil, platformContractError(operation, "Xandr returned duplicate advertisers")
		}
		seen[advertiser.ID] = struct{}{}
		advertiser.Meta = meta
		advertisers = append(advertisers, advertiser)
	}
	page, err := advertiserPage(operation, advertisers, response, input.StartElement, pageSize, meta)
	if err != nil {
		return nil, err
	}
	return page, nil
}

func (client *Client) GetCampaign(ctx context.Context, advertiserID, campaignID int64, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_get"
	if advertiserID <= 0 || campaignID <= 0 {
		return nil, invalidArgument(operation, "advertiser ID and campaign ID must be positive")
	}
	query := url.Values{
		"advertiser_id": {strconv.FormatInt(advertiserID, 10)},
		"id":            {strconv.FormatInt(campaignID, 10)},
	}
	response, meta, err := client.doGET(ctx, operation, campaignPath, query, options...)
	if err != nil {
		return nil, err
	}
	campaign, err := decodeCampaign(operation, response.Campaign)
	if err != nil {
		return nil, err
	}
	if campaign.ID != campaignID || campaign.AdvertiserID != advertiserID {
		return nil, platformContractError(operation, "Xandr returned a different or cross-advertiser campaign")
	}
	campaign.Meta = meta
	return &campaign, nil
}

func (client *Client) ListCampaigns(ctx context.Context, advertiserID int64, input ListOptions, options ...socialhub.CallOption) (*CampaignPage, error) {
	const operation = "campaign_list"
	if advertiserID <= 0 || !validListOptions(input) {
		return nil, invalidArgument(operation, "advertiser ID, state, search, start_element, or num_elements is invalid")
	}
	query, pageSize := listQuery(input)
	query.Set("advertiser_id", strconv.FormatInt(advertiserID, 10))
	response, meta, err := client.doGET(ctx, operation, campaignPath, query, options...)
	if err != nil {
		return nil, err
	}
	rawCampaigns, err := decodeRawList(operation, "campaigns", response.Campaigns)
	if err != nil {
		return nil, err
	}
	if len(rawCampaigns) > pageSize {
		return nil, platformContractError(operation, "Xandr returned more campaigns than requested")
	}
	campaigns := make([]Campaign, 0, len(rawCampaigns))
	seen := make(map[int64]struct{}, len(rawCampaigns))
	for _, raw := range rawCampaigns {
		campaign, err := decodeCampaign(operation, raw)
		if err != nil {
			return nil, err
		}
		if campaign.AdvertiserID != advertiserID {
			return nil, platformContractError(operation, "Xandr returned a cross-advertiser campaign")
		}
		if _, exists := seen[campaign.ID]; exists {
			return nil, platformContractError(operation, "Xandr returned duplicate campaigns")
		}
		seen[campaign.ID] = struct{}{}
		campaign.Meta = meta
		campaigns = append(campaigns, campaign)
	}
	page, err := campaignPage(operation, campaigns, response, input.StartElement, pageSize, meta)
	if err != nil {
		return nil, err
	}
	return page, nil
}

func listQuery(input ListOptions) (url.Values, int) {
	pageSize := normalizedNumElements(input.NumElements)
	query := url.Values{"num_elements": {strconv.Itoa(pageSize)}}
	if input.State != "" {
		query.Set("state", string(input.State))
	}
	if input.Search != "" {
		query.Set("search", input.Search)
	}
	if input.StartElement > 0 {
		query.Set("start_element", strconv.Itoa(input.StartElement))
	}
	return query, pageSize
}

func decodeAdvertiser(operation string, raw json.RawMessage) (Advertiser, error) {
	if len(raw) == 0 || isJSONNull(raw) {
		return Advertiser{}, platformContractError(operation, "Xandr success response omitted advertiser")
	}
	var advertiser Advertiser
	if err := json.Unmarshal(raw, &advertiser); err != nil || advertiser.ID <= 0 ||
		!validResponseText(advertiser.Name, 512) || !validResponseState(advertiser.State) {
		return Advertiser{}, platformContractError(operation, "Xandr returned an invalid advertiser")
	}
	advertiser.Raw = append(json.RawMessage(nil), raw...)
	return advertiser, nil
}

func decodeCampaign(operation string, raw json.RawMessage) (Campaign, error) {
	if len(raw) == 0 || isJSONNull(raw) {
		return Campaign{}, platformContractError(operation, "Xandr success response omitted campaign")
	}
	var campaign Campaign
	if err := json.Unmarshal(raw, &campaign); err != nil || campaign.ID <= 0 || campaign.AdvertiserID <= 0 ||
		!validResponseText(campaign.Name, 512) || !validResponseState(campaign.State) {
		return Campaign{}, platformContractError(operation, "Xandr returned an invalid campaign")
	}
	campaign.Raw = append(json.RawMessage(nil), raw...)
	return campaign, nil
}

func advertiserPage(operation string, values []Advertiser, response responseWire, requestedStart, requestedSize int, meta ResponseMeta) (*AdvertiserPage, error) {
	count, start, size, next, hasMore, err := pageFields(operation, len(values), response, requestedStart, requestedSize)
	if err != nil {
		return nil, err
	}
	return &AdvertiserPage{
		Advertisers: values, Count: count, StartElement: start, NumElements: size,
		HasMore: hasMore, NextStartElement: next, Meta: meta,
	}, nil
}

func campaignPage(operation string, values []Campaign, response responseWire, requestedStart, requestedSize int, meta ResponseMeta) (*CampaignPage, error) {
	count, start, size, next, hasMore, err := pageFields(operation, len(values), response, requestedStart, requestedSize)
	if err != nil {
		return nil, err
	}
	return &CampaignPage{
		Campaigns: values, Count: count, StartElement: start, NumElements: size,
		HasMore: hasMore, NextStartElement: next, Meta: meta,
	}, nil
}

func pageFields(operation string, itemCount int, response responseWire, requestedStart, requestedSize int) (int64, int, int, *int, bool, error) {
	if response.Count == nil || response.StartElement == nil || response.NumElements == nil {
		return 0, 0, 0, nil, false, platformContractError(operation, "Xandr omitted pagination metadata")
	}
	if requestedStart > int(^uint(0)>>1)-itemCount {
		return 0, 0, 0, nil, false, platformContractError(operation, "Xandr pagination offset overflowed")
	}
	offset := int64(requestedStart)
	end := offset + int64(itemCount)
	expectedItems := int64(requestedSize)
	if remaining := *response.Count - offset; remaining < expectedItems {
		expectedItems = remaining
	}
	if expectedItems < 0 {
		expectedItems = 0
	}
	if *response.Count < 0 || *response.StartElement != requestedStart || *response.NumElements != requestedSize ||
		int64(itemCount) != expectedItems || (offset < *response.Count && end > *response.Count) {
		return 0, 0, 0, nil, false, platformContractError(operation, "Xandr returned inconsistent pagination metadata")
	}
	hasMore := end < *response.Count
	var next *int
	if hasMore {
		value := requestedStart + itemCount
		next = &value
	}
	return *response.Count, *response.StartElement, *response.NumElements, next, hasMore, nil
}

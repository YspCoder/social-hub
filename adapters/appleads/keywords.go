package appleads

import (
	"context"
	"encoding/json"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListKeywords(ctx context.Context, campaignID, adGroupID int64, pagination Pagination, options ...socialhub.CallOption) (Page[Keyword], error) {
	const operation = "keywords_list"
	if !validID(campaignID) || !validID(adGroupID) || !validPagination(pagination) {
		return Page[Keyword]{}, invalidArgument(operation, "campaign ID, Ad Group ID, or pagination is invalid")
	}
	if _, err := client.GetAdGroup(ctx, campaignID, adGroupID, options...); err != nil {
		return Page[Keyword]{}, err
	}
	var response responseEnvelope[[]Keyword]
	path := keywordCollectionPath(campaignID, adGroupID)
	if err := client.getJSON(ctx, operation, path, listQuery(pagination), &response, options...); err != nil {
		return Page[Keyword]{}, err
	}
	if err := checkEnvelopeError(operation, response.Error); err != nil {
		return Page[Keyword]{}, err
	}
	for index := range response.Data {
		if err := client.validateKeyword(operation, &response.Data[index], campaignID, adGroupID, 0); err != nil {
			return Page[Keyword]{}, err
		}
	}
	return pageResult(response.Data, response.Pagination), nil
}

func (client *Client) GetKeyword(ctx context.Context, campaignID, adGroupID, keywordID int64, options ...socialhub.CallOption) (*Keyword, error) {
	const operation = "keyword_get"
	if !validID(campaignID) || !validID(adGroupID) || !validID(keywordID) {
		return nil, invalidArgument(operation, "campaign ID, Ad Group ID, and Keyword ID must be positive")
	}
	if _, err := client.GetAdGroup(ctx, campaignID, adGroupID, options...); err != nil {
		return nil, err
	}
	return client.getKeyword(ctx, operation, campaignID, adGroupID, keywordID, options...)
}

func (client *Client) getKeyword(ctx context.Context, operation string, campaignID, adGroupID, keywordID int64, options ...socialhub.CallOption) (*Keyword, error) {
	var response responseEnvelope[Keyword]
	path := keywordCollectionPath(campaignID, adGroupID) + "/" + formatID(keywordID)
	if err := client.getJSON(ctx, operation, path, nil, &response, options...); err != nil {
		return nil, err
	}
	if err := checkEnvelopeError(operation, response.Error); err != nil {
		return nil, err
	}
	if err := client.validateKeyword(operation, &response.Data, campaignID, adGroupID, keywordID); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

type keywordCreate struct {
	Text      string        `json:"text"`
	MatchType MatchType     `json:"matchType"`
	BidAmount *Money        `json:"bidAmount,omitempty"`
	Status    KeywordStatus `json:"status"`
}

func (client *Client) CreateKeywords(ctx context.Context, campaignID, adGroupID int64, inputs []CreateKeywordRequest, options ...socialhub.CallOption) ([]Keyword, error) {
	const operation = "keywords_create"
	if !validID(campaignID) || !validID(adGroupID) || !validCreateKeywords(inputs) {
		return nil, invalidArgument(operation, "campaign ID, Ad Group ID, or Keyword fields are invalid")
	}
	if _, err := client.GetCampaign(ctx, campaignID, options...); err != nil {
		return nil, err
	}
	group, err := client.getAdGroup(ctx, operation, campaignID, adGroupID, options...)
	if err != nil {
		return nil, err
	}
	if group.Deleted || group.Status != AdGroupPaused {
		return nil, invalidArgument(operation, "Ad Group must be undeleted and paused before creating Keywords")
	}
	payload := make([]keywordCreate, len(inputs))
	for index, input := range inputs {
		payload[index] = keywordCreate{Text: input.Text, MatchType: input.MatchType, BidAmount: input.BidAmount, Status: KeywordPaused}
	}
	var response responseEnvelope[[]Keyword]
	if err := client.postJSON(ctx, operation, keywordCollectionPath(campaignID, adGroupID)+"/bulk", payload, &response, options...); err != nil {
		return nil, err
	}
	if err := checkEnvelopeError(operation, response.Error); err != nil {
		return nil, err
	}
	if len(response.Data) != len(inputs) {
		return nil, platformContractError(operation, "Keyword response count did not match the request")
	}
	seen := make(map[int64]struct{}, len(response.Data))
	for index := range response.Data {
		keyword := &response.Data[index]
		if err := client.validateKeyword(operation, keyword, campaignID, adGroupID, 0); err != nil {
			return nil, err
		}
		if keyword.Status != KeywordPaused {
			return nil, platformContractError(operation, "created Keyword was not paused")
		}
		if _, exists := seen[keyword.ID]; exists {
			return nil, platformContractError(operation, "Keyword response contained duplicate IDs")
		}
		seen[keyword.ID] = struct{}{}
	}
	return response.Data, nil
}

type keywordUpdate struct {
	ID        int64          `json:"id"`
	BidAmount *Money         `json:"bidAmount,omitempty"`
	Status    *KeywordStatus `json:"status,omitempty"`
}

func (client *Client) UpdateKeywords(ctx context.Context, campaignID, adGroupID int64, inputs []UpdateKeywordRequest, options ...socialhub.CallOption) ([]Keyword, error) {
	const operation = "keywords_update"
	if !validID(campaignID) || !validID(adGroupID) || !validUpdateKeywords(inputs) {
		return nil, invalidArgument(operation, "campaign ID, Ad Group ID, or Keyword updates are invalid")
	}
	campaign, err := client.GetCampaign(ctx, campaignID, options...)
	if err != nil {
		return nil, err
	}
	group, err := client.getAdGroup(ctx, operation, campaignID, adGroupID, options...)
	if err != nil {
		return nil, err
	}
	for _, input := range inputs {
		if input.Status != nil && *input.Status == KeywordActive &&
			(campaign.Deleted || campaign.Status != CampaignEnabled || group.Deleted || group.Status != AdGroupEnabled) {
			return nil, invalidArgument(operation, "Campaign and Ad Group must be undeleted and enabled before enabling Keywords")
		}
	}
	payload := make([]keywordUpdate, len(inputs))
	expected := make(map[int64]*KeywordStatus, len(inputs))
	for index, input := range inputs {
		payload[index] = keywordUpdate{ID: input.ID, BidAmount: input.BidAmount, Status: input.Status}
		expected[input.ID] = input.Status
	}
	var response responseEnvelope[[]Keyword]
	if err := client.putJSON(ctx, operation, keywordCollectionPath(campaignID, adGroupID)+"/bulk", payload, &response, options...); err != nil {
		return nil, err
	}
	if err := checkEnvelopeError(operation, response.Error); err != nil {
		return nil, err
	}
	if len(response.Data) != len(inputs) {
		return nil, platformContractError(operation, "Keyword response count did not match the request")
	}
	for index := range response.Data {
		keyword := &response.Data[index]
		if err := client.validateKeyword(operation, keyword, campaignID, adGroupID, keyword.ID); err != nil {
			return nil, err
		}
		status, exists := expected[keyword.ID]
		if !exists {
			return nil, platformContractError(operation, "Keyword response ID was not present in the request")
		}
		if status != nil && keyword.Status != *status {
			return nil, platformContractError(operation, "Keyword status did not match the requested state")
		}
		delete(expected, keyword.ID)
	}
	if len(expected) != 0 {
		return nil, platformContractError(operation, "Keyword response omitted requested IDs")
	}
	return response.Data, nil
}

func (client *Client) DeleteKeyword(ctx context.Context, campaignID, adGroupID, keywordID int64, options ...socialhub.CallOption) error {
	const operation = "keyword_delete"
	current, err := client.GetKeyword(ctx, campaignID, adGroupID, keywordID, options...)
	if err != nil {
		return err
	}
	if current.Status != KeywordPaused {
		return invalidArgument(operation, "Keyword must be paused before deletion")
	}
	var response responseEnvelope[json.RawMessage]
	path := keywordCollectionPath(campaignID, adGroupID) + "/" + formatID(keywordID)
	if err := client.deleteJSON(ctx, operation, path, &response, options...); err != nil {
		return err
	}
	return checkEnvelopeError(operation, response.Error)
}

func (client *Client) validateKeyword(operation string, keyword *Keyword, campaignID, adGroupID, expectedID int64) error {
	if keyword == nil || !validID(keyword.ID) || keyword.CampaignID != campaignID || keyword.AdGroupID != adGroupID {
		return platformContractError(operation, "Keyword response has invalid ID or parent ownership")
	}
	if expectedID != 0 && keyword.ID != expectedID {
		return platformContractError(operation, "Keyword response ID did not match the requested Keyword")
	}
	return nil
}

func keywordCollectionPath(campaignID, adGroupID int64) string {
	return "/campaigns/" + formatID(campaignID) + "/adgroups/" + formatID(adGroupID) + "/targetingkeywords"
}

func validCreateKeywords(inputs []CreateKeywordRequest) bool {
	if len(inputs) == 0 || len(inputs) > 1000 {
		return false
	}
	for _, input := range inputs {
		if !validText(input.Text, 80) || input.MatchType != MatchBroad && input.MatchType != MatchExact || !validPositiveMoney(input.BidAmount) {
			return false
		}
	}
	return true
}

func validUpdateKeywords(inputs []UpdateKeywordRequest) bool {
	if len(inputs) == 0 || len(inputs) > 1000 {
		return false
	}
	seen := make(map[int64]struct{}, len(inputs))
	for _, input := range inputs {
		if !validID(input.ID) || input.BidAmount == nil && input.Status == nil || !validPositiveMoney(input.BidAmount) {
			return false
		}
		if input.Status != nil && *input.Status != KeywordActive && *input.Status != KeywordPaused {
			return false
		}
		if _, exists := seen[input.ID]; exists {
			return false
		}
		seen[input.ID] = struct{}{}
	}
	return true
}

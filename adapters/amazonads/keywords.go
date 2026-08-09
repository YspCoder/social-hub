package amazonads

import (
	"context"
	"net/http"

	"social-hub/pkg/socialhub"
)

type keywordListEnvelope struct {
	Keywords     []Keyword `json:"keywords"`
	NextToken    string    `json:"nextToken"`
	TotalResults int       `json:"totalResults"`
}

type keywordMutationEnvelope struct {
	Keywords struct {
		Success []struct {
			Index     int     `json:"index"`
			KeywordID string  `json:"keywordId"`
			Keyword   Keyword `json:"keyword"`
		} `json:"success"`
		Error []mutationFailure `json:"error"`
	} `json:"keywords"`
}

type keywordMutationResource struct {
	ID         string    `json:"keywordId,omitempty"`
	CampaignID string    `json:"campaignId,omitempty"`
	AdGroupID  string    `json:"adGroupId,omitempty"`
	Text       string    `json:"keywordText,omitempty"`
	MatchType  MatchType `json:"matchType,omitempty"`
	Bid        *Decimal  `json:"bid,omitempty"`
	State      State     `json:"state,omitempty"`
}

func (client *Client) ListKeywords(ctx context.Context, input ListKeywordsRequest, options ...socialhub.CallOption) (Page[Keyword], error) {
	const operation = "keywords_list"
	if !validIDs(input.IDs) || !validIDs(input.CampaignIDs) || !validIDs(input.AdGroupIDs) || !validStates(input.States) || !validList(input.MaxResults, input.NextToken) {
		return Page[Keyword]{}, invalidArgument(operation, "IDs, states, max results, or next token are invalid")
	}
	for _, value := range input.MatchTypes {
		if !validMatchType(value) {
			return Page[Keyword]{}, invalidArgument(operation, "match type is invalid")
		}
	}
	body := struct {
		KeywordIDFilter  *includeFilter[string]    `json:"keywordIdFilter,omitempty"`
		CampaignIDFilter *includeFilter[string]    `json:"campaignIdFilter,omitempty"`
		AdGroupIDFilter  *includeFilter[string]    `json:"adGroupIdFilter,omitempty"`
		StateFilter      *includeFilter[State]     `json:"stateFilter,omitempty"`
		MatchTypeFilter  *includeFilter[MatchType] `json:"matchTypeFilter,omitempty"`
		MaxResults       int                       `json:"maxResults,omitempty"`
		NextToken        string                    `json:"nextToken,omitempty"`
	}{MaxResults: input.MaxResults, NextToken: input.NextToken}
	if len(input.IDs) > 0 {
		body.KeywordIDFilter = &includeFilter[string]{Include: input.IDs}
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
	if len(input.MatchTypes) > 0 {
		body.MatchTypeFilter = &includeFilter[MatchType]{Include: input.MatchTypes}
	}
	var response keywordListEnvelope
	if _, err := client.vendorJSON(ctx, operation, http.MethodPost, "/sp/keywords/list", keywordMediaType, body, &response, false, options...); err != nil {
		return Page[Keyword]{}, err
	}
	for _, keyword := range response.Keywords {
		if !validID(keyword.ID) || !validID(keyword.CampaignID) || !validID(keyword.AdGroupID) {
			return Page[Keyword]{}, platformContractError(operation, "Amazon Ads returned an invalid Keyword, Campaign, or Ad Group ID")
		}
	}
	return Page[Keyword]{Items: response.Keywords, NextToken: response.NextToken, TotalResults: response.TotalResults}, nil
}

func (client *Client) CreateKeyword(ctx context.Context, input CreateKeywordRequest, options ...socialhub.CallOption) (*Keyword, error) {
	const operation = "keyword_create"
	if !validID(input.CampaignID) || !validID(input.AdGroupID) || !validText(input.Text, 80) || !validMatchType(input.MatchType) || !validDecimal(string(input.Bid), true) {
		return nil, invalidArgument(operation, "Campaign ID, Ad Group ID, keyword text, match type, or bid is invalid")
	}
	resource := keywordMutationResource{CampaignID: input.CampaignID, AdGroupID: input.AdGroupID, Text: input.Text, MatchType: input.MatchType, Bid: &input.Bid, State: StatePaused}
	return client.mutateKeyword(ctx, operation, http.MethodPost, "/sp/keywords", resource, "", options...)
}

func (client *Client) UpdateKeyword(ctx context.Context, id string, input UpdateKeywordRequest, options ...socialhub.CallOption) (*Keyword, error) {
	const operation = "keyword_update"
	if !validID(id) || input.Bid == nil || !validDecimal(string(*input.Bid), true) {
		return nil, invalidArgument(operation, "Keyword ID and a positive bid are required")
	}
	return client.mutateKeyword(ctx, operation, http.MethodPut, "/sp/keywords", keywordMutationResource{ID: id, Bid: input.Bid}, id, options...)
}

func (client *Client) SetKeywordState(ctx context.Context, id string, state State, options ...socialhub.CallOption) (*Keyword, error) {
	if !validID(id) || !validState(state) {
		return nil, invalidArgument("keyword_state", "Keyword ID and ENABLED or PAUSED state are required")
	}
	return client.mutateKeyword(ctx, "keyword_state", http.MethodPut, "/sp/keywords", keywordMutationResource{ID: id, State: state}, id, options...)
}

func (client *Client) ArchiveKeyword(ctx context.Context, id string, options ...socialhub.CallOption) error {
	const operation = "keyword_archive"
	if !validID(id) {
		return invalidArgument(operation, "Keyword ID is invalid")
	}
	body := struct {
		Filter includeFilter[string] `json:"keywordIdFilter"`
	}{Filter: includeFilter[string]{Include: []string{id}}}
	var response keywordMutationEnvelope
	metadata, err := client.vendorJSON(ctx, operation, http.MethodPost, "/sp/keywords/delete", keywordMediaType, body, &response, true, options...)
	if err != nil {
		return err
	}
	_, err = keywordMutationResult(operation, id, metadata.StatusCode, metadata.Header, response)
	return err
}

func (client *Client) mutateKeyword(ctx context.Context, operation, method, path string, resource keywordMutationResource, expected string, options ...socialhub.CallOption) (*Keyword, error) {
	body := struct {
		Keywords []keywordMutationResource `json:"keywords"`
	}{Keywords: []keywordMutationResource{resource}}
	var response keywordMutationEnvelope
	metadata, err := client.vendorJSON(ctx, operation, method, path, keywordMediaType, body, &response, true, options...)
	if err != nil {
		return nil, err
	}
	return keywordMutationResult(operation, expected, metadata.StatusCode, metadata.Header, response)
}

func keywordMutationResult(operation, expected string, status int, header http.Header, response keywordMutationEnvelope) (*Keyword, error) {
	if len(response.Keywords.Error) > 0 {
		return nil, mutationError(operation, status, header, response.Keywords.Error[0])
	}
	if len(response.Keywords.Success) != 1 {
		return nil, platformContractError(operation, "Amazon Ads did not return exactly one Keyword mutation result")
	}
	item := response.Keywords.Success[0]
	if err := requireMutationID(operation, expected, item.KeywordID); err != nil {
		return nil, err
	}
	if item.Keyword.ID == "" {
		item.Keyword.ID = item.KeywordID
	}
	if item.Keyword.ID != item.KeywordID {
		return nil, platformContractError(operation, "Amazon Ads returned mismatched Keyword IDs")
	}
	return &item.Keyword, nil
}

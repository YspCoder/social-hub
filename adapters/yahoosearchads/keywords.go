package yahoosearchads

import (
	"context"

	"social-hub/pkg/socialhub"
)

const keywordServicePath = "AdGroupCriterionService"

type keywordSelectorRequest struct {
	AccountID     int64        `json:"accountId"`
	CampaignIDs   []int64      `json:"campaignIds,omitempty"`
	AdGroupIDs    []int64      `json:"adGroupIds,omitempty"`
	CriterionIDs  []int64      `json:"criterionIds,omitempty"`
	UserStatuses  []UserStatus `json:"userStatuses,omitempty"`
	Use           CriterionUse `json:"use"`
	StartIndex    int32        `json:"startIndex,omitempty"`
	NumberResults int32        `json:"numberResults,omitempty"`
}

type keywordOperation struct {
	AccountID int64     `json:"accountId"`
	Operand   []Keyword `json:"operand"`
}

func (client *Client) ListKeywords(ctx context.Context, input KeywordSelector, options ...socialhub.CallOption) (Page[Keyword], error) {
	const operation = "keyword_list"
	if !validKeywordSelector(input) {
		return Page[Keyword]{}, invalidArgument(operation, "keyword parent IDs, criterion IDs, statuses, use, or pagination are invalid")
	}
	request := keywordSelectorRequest{
		AccountID: client.advertiserAccountID, CampaignIDs: input.CampaignIDs,
		AdGroupIDs: input.AdGroupIDs, CriterionIDs: input.CriterionIDs,
		UserStatuses: input.UserStatuses, Use: CriterionBiddable,
		StartIndex: input.StartIndex, NumberResults: input.NumberResults,
	}
	return postPage(ctx, client, operation, keywordServicePath+"/get", request, input.PageRequest,
		MaximumPageSize, keywordEntity, func(value *Keyword) error {
			return client.validateKeyword(operation, value, 0, 0, 0)
		}, options...)
}

func (client *Client) GetKeyword(ctx context.Context, campaignID, adGroupID, criterionID int64, options ...socialhub.CallOption) (*Keyword, error) {
	const operation = "keyword_get"
	if campaignID <= 0 || adGroupID <= 0 || criterionID <= 0 {
		return nil, invalidArgument(operation, "campaign, ad group, and criterion IDs must be positive")
	}
	page, err := client.ListKeywords(ctx, KeywordSelector{
		CampaignIDs: []int64{campaignID}, AdGroupIDs: []int64{adGroupID}, CriterionIDs: []int64{criterionID},
		Use: CriterionBiddable, PageRequest: PageRequest{StartIndex: 1, NumberResults: 1},
	}, options...)
	if err != nil {
		return nil, withOperation(err, operation)
	}
	if len(page.Items) == 0 {
		return nil, notFound(operation, "keyword was not returned")
	}
	item := &page.Items[0]
	if len(page.Items) != 1 || item.CampaignID != campaignID || item.AdGroupID != adGroupID || item.ID() != criterionID {
		return nil, platformContractError(operation, "LINE Yahoo returned a different keyword")
	}
	return item, nil
}

func (client *Client) CreateKeywords(ctx context.Context, campaignID, adGroupID int64, inputs []KeywordAdd, options ...socialhub.CallOption) (MutationResult[Keyword], error) {
	const operation = "keyword_create"
	if campaignID <= 0 || adGroupID <= 0 || len(inputs) == 0 || len(inputs) > MaximumMutationBatch {
		return MutationResult[Keyword]{}, invalidArgument(operation, "parent IDs and 1-2000 keywords are required")
	}
	operands := make([]Keyword, 0, len(inputs))
	for _, input := range inputs {
		if !validKeywordAdd(input) {
			return MutationResult[Keyword]{}, invalidArgument(operation, "keyword text, match type, or CPC is invalid")
		}
		cpc := input.CPC
		operands = append(operands, Keyword{
			CampaignID: campaignID, AdGroupID: adGroupID, Use: CriterionBiddable,
			Criterion: Criterion{Keyword: &KeywordText{Text: input.Text, MatchType: input.MatchType}},
			Biddable:  &BiddableKeyword{Bid: &KeywordBid{CPC: &cpc}, UserStatus: StatusPaused},
		})
	}
	return postMutation(ctx, client, operation, keywordServicePath+"/add",
		keywordOperation{AccountID: client.advertiserAccountID, Operand: operands}, len(operands),
		keywordEntity, func(value *Keyword) error {
			return client.validateKeywordMutation(operation, value, campaignID, adGroupID)
		}, options...)
}

func (client *Client) UpdateKeywords(ctx context.Context, campaignID, adGroupID int64, inputs []KeywordUpdate, options ...socialhub.CallOption) (MutationResult[Keyword], error) {
	const operation = "keyword_update"
	if campaignID <= 0 || adGroupID <= 0 || len(inputs) == 0 || len(inputs) > MaximumMutationBatch {
		return MutationResult[Keyword]{}, invalidArgument(operation, "parent IDs and 1-2000 keyword updates are required")
	}
	seen := make(map[int64]struct{}, len(inputs))
	operands := make([]Keyword, 0, len(inputs))
	for _, input := range inputs {
		if !validKeywordUpdate(input) {
			return MutationResult[Keyword]{}, invalidArgument(operation, "keyword update ID or CPC is invalid")
		}
		if _, exists := seen[input.ID]; exists {
			return MutationResult[Keyword]{}, invalidArgument(operation, "criterion IDs must be unique")
		}
		seen[input.ID] = struct{}{}
		cpc := *input.CPC
		operands = append(operands, Keyword{
			CampaignID: campaignID, AdGroupID: adGroupID, Use: CriterionBiddable,
			Criterion: Criterion{ID: input.ID},
			Biddable:  &BiddableKeyword{Bid: &KeywordBid{CPC: &cpc}},
		})
	}
	return postMutation(ctx, client, operation, keywordServicePath+"/set",
		keywordOperation{AccountID: client.advertiserAccountID, Operand: operands}, len(operands),
		keywordEntity, func(value *Keyword) error {
			return client.validateKeywordMutation(operation, value, campaignID, adGroupID)
		}, options...)
}

func (client *Client) SetKeywordsEnabled(ctx context.Context, campaignID, adGroupID int64, ids []int64, enabled bool, options ...socialhub.CallOption) (MutationResult[Keyword], error) {
	const operation = "keyword_set_enabled"
	if campaignID <= 0 || adGroupID <= 0 || !validIDs(ids, MaximumMutationBatch, false) {
		return MutationResult[Keyword]{}, invalidArgument(operation, "parent IDs and 1-2000 unique criterion IDs are required")
	}
	status := StatusPaused
	if enabled {
		status = StatusActive
	}
	operands := make([]Keyword, 0, len(ids))
	for _, id := range ids {
		operands = append(operands, Keyword{
			CampaignID: campaignID, AdGroupID: adGroupID, Use: CriterionBiddable,
			Criterion: Criterion{ID: id}, Biddable: &BiddableKeyword{UserStatus: status},
		})
	}
	return postMutation(ctx, client, operation, keywordServicePath+"/set",
		keywordOperation{AccountID: client.advertiserAccountID, Operand: operands}, len(operands),
		keywordEntity, func(value *Keyword) error {
			return client.validateKeywordMutation(operation, value, campaignID, adGroupID)
		}, options...)
}

func (client *Client) DeleteKeywords(ctx context.Context, campaignID, adGroupID int64, ids []int64, options ...socialhub.CallOption) (MutationResult[Keyword], error) {
	const operation = "keyword_delete"
	if campaignID <= 0 || adGroupID <= 0 || !validIDs(ids, MaximumMutationBatch, false) {
		return MutationResult[Keyword]{}, invalidArgument(operation, "parent IDs and 1-2000 unique criterion IDs are required")
	}
	prepared, err := prepareCallOptions(operation, options)
	if err != nil {
		return MutationResult[Keyword]{}, err
	}
	if err := client.requirePausedKeywords(ctx, operation, campaignID, adGroupID, ids, prepared...); err != nil {
		return MutationResult[Keyword]{}, err
	}
	operands := make([]Keyword, 0, len(ids))
	for _, id := range ids {
		operands = append(operands, Keyword{
			CampaignID: campaignID, AdGroupID: adGroupID, Use: CriterionBiddable,
			Criterion: Criterion{ID: id},
		})
	}
	return postMutation(ctx, client, operation, keywordServicePath+"/remove",
		keywordOperation{AccountID: client.advertiserAccountID, Operand: operands}, len(operands),
		keywordEntity, func(value *Keyword) error {
			return client.validateKeywordMutation(operation, value, campaignID, adGroupID)
		}, prepared...)
}

func (client *Client) requirePausedKeywords(ctx context.Context, operation string, campaignID, adGroupID int64, ids []int64, options ...socialhub.CallOption) error {
	expected := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		expected[id] = struct{}{}
	}
	for start := 0; start < len(ids); start += MaximumSelectorIDs {
		end := start + MaximumSelectorIDs
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[start:end]
		page, err := client.ListKeywords(ctx, KeywordSelector{
			CampaignIDs: []int64{campaignID}, AdGroupIDs: []int64{adGroupID}, CriterionIDs: chunk,
			Use: CriterionBiddable, PageRequest: PageRequest{StartIndex: 1, NumberResults: int32(len(chunk))},
		}, options...)
		if err != nil {
			return withOperation(err, operation)
		}
		if len(page.Items) != len(chunk) {
			return notFound(operation, "one or more keywords were not returned before delete")
		}
		for _, keyword := range page.Items {
			if keyword.CampaignID != campaignID || keyword.AdGroupID != adGroupID {
				return platformContractError(operation, "LINE Yahoo returned a keyword from another parent")
			}
			id := keyword.ID()
			if _, exists := expected[id]; !exists {
				return platformContractError(operation, "LINE Yahoo returned a duplicate keyword or one outside the delete selection")
			}
			delete(expected, id)
			if keyword.Biddable == nil || keyword.Biddable.UserStatus != StatusPaused {
				return invalidArgument(operation, "keywords must be PAUSED before delete")
			}
		}
	}
	if len(expected) != 0 {
		return platformContractError(operation, "LINE Yahoo omitted one or more keywords from the delete preflight")
	}
	return nil
}

func (client *Client) validateKeyword(operation string, value *Keyword, expectedCampaignID, expectedAdGroupID, expectedCriterionID int64) error {
	if value == nil || value.AccountID != client.advertiserAccountID || value.CampaignID <= 0 || value.AdGroupID <= 0 ||
		value.Criterion.ID <= 0 || value.Criterion.Keyword == nil ||
		!validText(value.Criterion.Keyword.Text, 80) || !validKeywordMatchType(value.Criterion.Keyword.MatchType) ||
		value.Use != CriterionBiddable || value.Biddable == nil ||
		(value.Biddable.UserStatus != StatusActive && value.Biddable.UserStatus != StatusPaused) {
		return platformContractError(operation, "LINE Yahoo returned an invalid biddable keyword")
	}
	if expectedCampaignID > 0 && value.CampaignID != expectedCampaignID ||
		expectedAdGroupID > 0 && value.AdGroupID != expectedAdGroupID ||
		expectedCriterionID > 0 && value.Criterion.ID != expectedCriterionID {
		return platformContractError(operation, "keyword parent or criterion ID did not match the request")
	}
	return nil
}

func (client *Client) validateKeywordMutation(operation string, value *Keyword, expectedCampaignID, expectedAdGroupID int64) error {
	if value == nil || value.Criterion.ID <= 0 ||
		value.CampaignID != 0 && value.CampaignID != expectedCampaignID ||
		value.AdGroupID != 0 && value.AdGroupID != expectedAdGroupID ||
		value.AccountID != 0 && value.AccountID != client.advertiserAccountID {
		return platformContractError(operation, "LINE Yahoo returned an invalid keyword mutation value")
	}
	return nil
}

var _ KeywordWorkflow = (*Client)(nil)

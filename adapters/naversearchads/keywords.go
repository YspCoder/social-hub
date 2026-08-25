package naversearchads

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

type keywordWrite struct {
	ID                string          `json:"nccKeywordId,omitempty"`
	AdGroupID         string          `json:"nccAdgroupId,omitempty"`
	CampaignID        string          `json:"nccCampaignId,omitempty"`
	CustomerID        int64           `json:"customerId,omitempty"`
	Text              string          `json:"keyword,omitempty"`
	UseGroupBidAmount *bool           `json:"useGroupBidAmt,omitempty"`
	BidAmount         *int64          `json:"bidAmt,omitempty"`
	UserLock          *bool           `json:"userLock,omitempty"`
	Links             json.RawMessage `json:"links,omitempty"`
	InspectRequest    *string         `json:"inspectRequestMsg,omitempty"`
}

func (client *Client) ListKeywords(ctx context.Context, input ListKeywordsRequest, options ...socialhub.CallOption) (Page[Keyword], error) {
	const operation = "keyword_list"
	if !validID(input.AdGroupID) || !validListOptions(input.ListOptions) {
		return Page[Keyword]{}, invalidArgument(operation, "Ad Group ID or list options are invalid")
	}
	list := normalizeList(input.ListOptions)
	query := listValues(list)
	query.Set("nccAdgroupId", input.AdGroupID)
	var keywords []Keyword
	if err := client.getJSON(ctx, operation, "/ncc/keywords", query, &keywords, options...); err != nil {
		return Page[Keyword]{}, err
	}
	for index := range keywords {
		if err := client.validateKeyword(operation, &keywords[index], "", input.AdGroupID); err != nil {
			return Page[Keyword]{}, err
		}
	}
	return listPage(keywords, list, func(value Keyword) string { return value.ID }), nil
}

func (client *Client) GetKeyword(ctx context.Context, id string, options ...socialhub.CallOption) (*Keyword, error) {
	const operation = "keyword_get"
	if !validID(id) {
		return nil, invalidArgument(operation, "Keyword ID is invalid")
	}
	var keyword Keyword
	if err := client.getJSON(ctx, operation, "/ncc/keywords/"+id, nil, &keyword, options...); err != nil {
		return nil, err
	}
	if err := client.validateKeyword(operation, &keyword, id, ""); err != nil {
		return nil, err
	}
	return &keyword, nil
}

func (client *Client) CreateKeywords(ctx context.Context, adGroupID string, inputs []CreateKeywordRequest, options ...socialhub.CallOption) ([]Keyword, error) {
	const operation = "keywords_create"
	if !validID(adGroupID) || !validCreateKeywords(inputs) {
		return nil, invalidArgument(operation, "Ad Group ID or Keyword fields are invalid")
	}
	prepared, err := prepareCallOptions(operation, options)
	if err != nil {
		return nil, err
	}
	group, err := client.GetAdGroup(ctx, adGroupID, prepared...)
	if err != nil {
		return nil, withOperation(err, operation)
	}
	if group.Status == StatusDeleted {
		return nil, invalidArgument(operation, "deleted Ad Group cannot receive Keywords")
	}
	payload := make([]keywordWrite, len(inputs))
	paused := true
	for index, input := range inputs {
		useGroup := input.UseGroupBidAmount
		inspect := input.InspectRequest
		payload[index] = keywordWrite{
			Text: input.Text, UseGroupBidAmount: &useGroup, BidAmount: input.BidAmount,
			UserLock: &paused, Links: cloneRaw(input.Links),
		}
		if inspect != "" {
			payload[index].InspectRequest = &inspect
		}
	}
	query := url.Values{"nccAdgroupId": {adGroupID}}
	var keywords []Keyword
	if err := client.writeJSON(ctx, operation, http.MethodPost, "/ncc/keywords", query, payload, &keywords, prepared...); err != nil {
		return nil, err
	}
	if len(keywords) != len(inputs) {
		return nil, outcomeUnknownError(operation, platformContractError(operation, "Keyword response count did not match the request"))
	}
	if err := keywordBatchError(operation, keywords); err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(keywords))
	for index := range keywords {
		keyword := &keywords[index]
		if err := client.validateKeyword(operation, keyword, "", adGroupID); err != nil {
			return nil, outcomeUnknownError(operation, err)
		}
		if !keyword.UserLock || keyword.Status != StatusPaused {
			return nil, outcomeUnknownError(operation, platformContractError(operation, "created Keyword was not paused"))
		}
		if _, exists := seen[keyword.ID]; exists {
			return nil, outcomeUnknownError(operation, platformContractError(operation, "Keyword response contained duplicate IDs"))
		}
		seen[keyword.ID] = struct{}{}
	}
	return keywords, nil
}

func (client *Client) UpdateKeywords(ctx context.Context, inputs []UpdateKeywordRequest, options ...socialhub.CallOption) ([]Keyword, error) {
	const operation = "keywords_update"
	fields, err := validateKeywordUpdates(inputs)
	if err != nil {
		return nil, err
	}
	prepared, err := prepareCallOptions(operation, options)
	if err != nil {
		return nil, err
	}
	if strings.Contains(fields, "userLock") {
		for _, input := range inputs {
			if input.Paused != nil && !*input.Paused {
				current, err := client.GetKeyword(ctx, input.ID, prepared...)
				if err != nil {
					return nil, withOperation(err, operation)
				}
				if err := client.requireEnabledParents(ctx, operation, current, prepared...); err != nil {
					return nil, err
				}
			}
		}
	}
	payload := make([]keywordWrite, len(inputs))
	requested := make(map[string]UpdateKeywordRequest, len(inputs))
	for index, input := range inputs {
		payload[index] = keywordWrite{
			ID: input.ID, CustomerID: client.customerID, UseGroupBidAmount: input.UseGroupBidAmount,
			BidAmount: input.BidAmount, UserLock: input.Paused, Links: cloneRaw(input.Links),
			InspectRequest: input.InspectRequest,
		}
		requested[input.ID] = input
	}
	query := url.Values{"fields": {fields}}
	var keywords []Keyword
	if err := client.writeJSON(ctx, operation, http.MethodPut, "/ncc/keywords", query, payload, &keywords, prepared...); err != nil {
		return nil, err
	}
	if len(keywords) != len(inputs) {
		return nil, outcomeUnknownError(operation, platformContractError(operation, "Keyword response count did not match the update batch"))
	}
	if err := keywordBatchError(operation, keywords); err != nil {
		return nil, err
	}
	for index := range keywords {
		keyword := &keywords[index]
		input, exists := requested[keyword.ID]
		if !exists {
			return nil, outcomeUnknownError(operation, platformContractError(operation, "Keyword response ID was not present in the request"))
		}
		if err := client.validateKeyword(operation, keyword, keyword.ID, ""); err != nil {
			return nil, outcomeUnknownError(operation, err)
		}
		if input.Paused != nil && keyword.UserLock != *input.Paused || input.BidAmount != nil && keyword.BidAmount != *input.BidAmount {
			return nil, outcomeUnknownError(operation, platformContractError(operation, "Keyword response did not match requested fields"))
		}
		delete(requested, keyword.ID)
	}
	if len(requested) != 0 {
		return nil, outcomeUnknownError(operation, platformContractError(operation, "Keyword response omitted requested IDs"))
	}
	return keywords, nil
}

func (client *Client) SetKeywordPaused(ctx context.Context, id string, paused bool, options ...socialhub.CallOption) (*Keyword, error) {
	results, err := client.UpdateKeywords(ctx, []UpdateKeywordRequest{{ID: id, Paused: &paused}}, options...)
	if err != nil {
		return nil, withOperation(err, "keyword_set_paused")
	}
	if len(results) != 1 {
		return nil, outcomeUnknownError("keyword_set_paused", platformContractError("keyword_set_paused", "Keyword update returned an unexpected result count"))
	}
	return &results[0], nil
}

func (client *Client) DeleteKeyword(ctx context.Context, id string, options ...socialhub.CallOption) error {
	const operation = "keyword_delete"
	if !validID(id) {
		return invalidArgument(operation, "Keyword ID is invalid")
	}
	prepared, err := prepareCallOptions(operation, options)
	if err != nil {
		return err
	}
	current, err := client.GetKeyword(ctx, id, prepared...)
	if err != nil {
		return withOperation(err, operation)
	}
	if !current.UserLock || current.Status != StatusPaused {
		return invalidArgument(operation, "Keyword must be paused before deletion")
	}
	return client.delete(ctx, operation, "/ncc/keywords/"+id, prepared...)
}

func (client *Client) validateKeyword(operation string, keyword *Keyword, expectedID, expectedAdGroupID string) error {
	if keyword == nil || !validID(keyword.ID) || !validID(keyword.AdGroupID) || !validID(keyword.CampaignID) ||
		keyword.CustomerID != client.customerID {
		return platformContractError(operation, "Keyword response has invalid IDs or customer ownership")
	}
	if expectedID != "" && keyword.ID != expectedID || expectedAdGroupID != "" && keyword.AdGroupID != expectedAdGroupID {
		return platformContractError(operation, "Keyword response ownership did not match the request")
	}
	return nil
}

func (client *Client) requireEnabledParents(ctx context.Context, operation string, keyword *Keyword, options ...socialhub.CallOption) error {
	group, err := client.GetAdGroup(ctx, keyword.AdGroupID, options...)
	if err != nil {
		return withOperation(err, operation)
	}
	campaign, err := client.GetCampaign(ctx, keyword.CampaignID, options...)
	if err != nil {
		return withOperation(err, operation)
	}
	if group.UserLock || campaign.UserLock || campaign.Status != StatusEligible ||
		group.Status != StatusEligible && group.Status != StatusLimitedEligible {
		return invalidArgument(operation, "parent Campaign and Ad Group must be eligible and enabled before enabling a Keyword")
	}
	return nil
}

func validCreateKeywords(inputs []CreateKeywordRequest) bool {
	if len(inputs) == 0 || len(inputs) > MaximumKeywordCreateBatch {
		return false
	}
	seen := make(map[string]struct{}, len(inputs))
	for _, input := range inputs {
		if !validText(input.Text, 100) || !validRawObject(input.Links, true) || !validOptionalText(input.InspectRequest, 1000) {
			return false
		}
		if input.UseGroupBidAmount && input.BidAmount != nil || !input.UseGroupBidAmount &&
			(input.BidAmount == nil || *input.BidAmount < 70 || *input.BidAmount > 100_000) {
			return false
		}
		if _, exists := seen[input.Text]; exists {
			return false
		}
		seen[input.Text] = struct{}{}
	}
	return true
}

func validateKeywordUpdates(inputs []UpdateKeywordRequest) (string, error) {
	const operation = "keywords_update"
	if len(inputs) == 0 || len(inputs) > MaximumKeywordUpdateBatch {
		return "", invalidArgument(operation, "1..200 Keyword updates are required")
	}
	seen := make(map[string]struct{}, len(inputs))
	expectedFields := ""
	for _, input := range inputs {
		if !validID(input.ID) {
			return "", invalidArgument(operation, "Keyword ID is invalid")
		}
		fields := make([]string, 0, 4)
		if input.Paused != nil {
			fields = append(fields, "userLock")
		}
		if input.UseGroupBidAmount != nil || input.BidAmount != nil {
			if input.UseGroupBidAmount == nil || *input.UseGroupBidAmount && input.BidAmount != nil ||
				!*input.UseGroupBidAmount && (input.BidAmount == nil || *input.BidAmount < 70 || *input.BidAmount > 100_000) {
				return "", invalidArgument(operation, "Keyword bid fields are invalid")
			}
			fields = append(fields, "bidAmt")
		}
		if len(input.Links) > 0 {
			if !validRawObject(input.Links, false) {
				return "", invalidArgument(operation, "Keyword links must be a valid JSON object")
			}
			fields = append(fields, "links")
		}
		if input.InspectRequest != nil {
			if !validText(*input.InspectRequest, 1000) {
				return "", invalidArgument(operation, "Keyword inspect request is invalid")
			}
			fields = append(fields, "inspect")
		}
		fieldSet := strings.Join(fields, ",")
		if fieldSet == "" || expectedFields != "" && fieldSet != expectedFields {
			return "", invalidArgument(operation, "every Keyword batch item must update the same nonempty field set")
		}
		expectedFields = fieldSet
		if _, exists := seen[input.ID]; exists {
			return "", invalidArgument(operation, "Keyword IDs must be unique")
		}
		seen[input.ID] = struct{}{}
	}
	return expectedFields, nil
}

func validOptionalText(value string, maximum int) bool {
	return value == "" || validText(value, maximum)
}

func checkKeywordMutation(operation string, keyword *Keyword) error {
	if keyword.ResultStatus == nil || keyword.ResultStatus.Code == 0 {
		return nil
	}
	platformCode := formatInt64(int64(keyword.ResultStatus.Code))
	code, class := classifyError(http.StatusBadRequest, platformCode)
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		HTTPStatus: http.StatusOK, PlatformCode: platformCode,
		PlatformMessage: "NAVER rejected a Keyword mutation result",
	}
}

func keywordBatchError(operation string, keywords []Keyword) error {
	failures := 0
	var first error
	for index := range keywords {
		if err := checkKeywordMutation(operation, &keywords[index]); err != nil {
			failures++
			if first == nil {
				first = err
			}
		}
	}
	if failures == 0 {
		return nil
	}
	if failures < len(keywords) {
		return partialMutationError(operation, first)
	}
	return first
}

package yandexdirect

import (
	"context"
	"sort"

	"social-hub/pkg/socialhub"
)

var keywordFields = []string{
	"Id", "Keyword", "AdGroupId", "CampaignId", "Bid", "AutotargetingSearchBidIsAuto",
	"ContextBid", "StrategyPriority", "UserParam1", "UserParam2", "State", "Status", "ServingStatus",
}

func (client *Client) ListKeywords(ctx context.Context, input ListKeywordsRequest, options ...socialhub.CallOption) (Page[Keyword], error) {
	const operation = "keyword_list"
	selection := input.Selection
	if !validKeywordSelection(selection) || len(selection.IDs)+len(selection.AdGroupIDs)+len(selection.CampaignIDs) == 0 || !validPage(input.Page) {
		return Page[Keyword]{}, invalidArgument(operation, "Keyword selection requires valid IDs, Ad Group IDs, or Campaign IDs and a valid page")
	}
	params := struct {
		SelectionCriteria KeywordSelection `json:"SelectionCriteria"`
		FieldNames        []string         `json:"FieldNames"`
		Page              *PageRequest     `json:"Page,omitempty"`
	}{SelectionCriteria: selection, FieldNames: keywordFields, Page: pagePointer(input.Page)}
	var response struct {
		Keywords  []Keyword `json:"Keywords"`
		LimitedBy *int64    `json:"LimitedBy"`
	}
	metadata, err := client.rpc(ctx, operation, "keywords", "get", params, &response, false, options...)
	if err != nil {
		return Page[Keyword]{}, err
	}
	if len(response.Keywords) > maximumPageItems(input.Page) || response.LimitedBy != nil && *response.LimitedBy <= input.Page.Offset {
		return Page[Keyword]{}, platformContractError(operation, "Yandex returned invalid Keyword pagination")
	}
	for index := range response.Keywords {
		if err := validateKeyword(operation, &response.Keywords[index], 0); err != nil {
			return Page[Keyword]{}, err
		}
	}
	return Page[Keyword]{Items: response.Keywords, LimitedBy: response.LimitedBy, Metadata: metadata}, nil
}

func (client *Client) GetKeyword(ctx context.Context, id int64, options ...socialhub.CallOption) (*Keyword, error) {
	const operation = "keyword_get"
	if id <= 0 {
		return nil, invalidArgument(operation, "Keyword ID must be positive")
	}
	page, err := client.ListKeywords(ctx, ListKeywordsRequest{Selection: KeywordSelection{IDs: []int64{id}}}, options...)
	if err != nil {
		return nil, withOperation(err, operation)
	}
	if len(page.Items) == 0 {
		return nil, notFound(operation, "Keyword was not returned")
	}
	if len(page.Items) != 1 || page.Items[0].ID != id {
		return nil, platformContractError(operation, "Yandex returned a different Keyword")
	}
	return &page.Items[0], nil
}

func (client *Client) CreateKeywords(ctx context.Context, adGroupID int64, inputs []KeywordAdd, options ...socialhub.CallOption) (BatchResult, error) {
	const operation = "keyword_create"
	if adGroupID <= 0 || len(inputs) == 0 || len(inputs) > MaximumKeywordCreateBatch {
		return BatchResult{}, invalidArgument(operation, "positive Ad Group ID and 1-200 Keywords are required")
	}
	for _, input := range inputs {
		if !validKeywordAdd(input) {
			return BatchResult{}, invalidArgument(operation, "Keyword text, bid, priority, or substitution parameter is invalid")
		}
	}
	callOptions, err := prepareCallOptions(operation, options)
	if err != nil {
		return BatchResult{}, err
	}
	ctx, cancel := withCallTimeout(ctx, callOptions.Timeout)
	defer cancel()
	options = nil
	group, err := client.GetAdGroup(ctx, adGroupID, options...)
	if err != nil {
		return BatchResult{}, withOperation(err, operation)
	}
	if group.Type != AdGroupText {
		return BatchResult{}, invalidArgument(operation, "initial Keyword creation supports classic Text Ad Groups only")
	}
	campaign, err := client.GetCampaign(ctx, group.CampaignID, options...)
	if err != nil {
		return BatchResult{}, withOperation(err, operation)
	}
	if !safeNonServingCampaign(campaign) {
		return BatchResult{}, invalidArgument(operation, "parent Campaign must be suspended or draft/off before Keyword creation")
	}
	type write struct {
		KeywordAdd
		AdGroupID int64 `json:"AdGroupId"`
	}
	items := make([]write, len(inputs))
	for index, input := range inputs {
		items[index] = write{KeywordAdd: input, AdGroupID: adGroupID}
	}
	params := struct {
		Keywords []write `json:"Keywords"`
	}{Keywords: items}
	var response struct {
		AddResults []ActionResult `json:"AddResults"`
	}
	metadata, err := client.rpc(ctx, operation, "keywords", "add", params, &response, true, options...)
	if err != nil {
		return BatchResult{Metadata: metadata}, err
	}
	return actionResult(operation, response.AddResults, len(inputs), metadata)
}

func (client *Client) UpdateKeywords(ctx context.Context, inputs []KeywordUpdate, options ...socialhub.CallOption) (BatchResult, error) {
	const operation = "keyword_update"
	if len(inputs) == 0 || len(inputs) > MaximumKeywordMutationBatch {
		return BatchResult{}, invalidArgument(operation, "1-1000 Keyword updates are required")
	}
	seen := make(map[int64]struct{}, len(inputs))
	ids := make([]int64, len(inputs))
	for index, input := range inputs {
		if !validKeywordUpdate(input) {
			return BatchResult{}, invalidArgument(operation, "Keyword update is invalid")
		}
		if _, exists := seen[input.ID]; exists {
			return BatchResult{}, invalidArgument(operation, "Keyword IDs must be unique")
		}
		seen[input.ID] = struct{}{}
		ids[index] = input.ID
	}
	params := struct {
		Keywords []KeywordUpdate `json:"Keywords"`
	}{Keywords: inputs}
	var response struct {
		UpdateResults []ActionResult `json:"UpdateResults"`
	}
	metadata, err := client.rpc(ctx, operation, "keywords", "update", params, &response, true, options...)
	if err != nil {
		return BatchResult{Metadata: metadata}, err
	}
	return actionResult(operation, response.UpdateResults, len(inputs), metadata, ids...)
}

func (client *Client) SuspendKeywords(ctx context.Context, ids []int64, options ...socialhub.CallOption) (BatchResult, error) {
	return client.keywordAction(ctx, "keyword_suspend", "suspend", ids, options...)
}

func (client *Client) ResumeKeywords(ctx context.Context, ids []int64, options ...socialhub.CallOption) (BatchResult, error) {
	return client.keywordAction(ctx, "keyword_resume", "resume", ids, options...)
}

func (client *Client) DeleteKeywords(ctx context.Context, ids []int64, options ...socialhub.CallOption) (BatchResult, error) {
	const operation = "keyword_delete"
	if !validIDs(ids, MaximumKeywordActionBatch, false) {
		return BatchResult{}, invalidArgument(operation, "1-10000 unique Keyword IDs are required")
	}
	callOptions, err := prepareCallOptions(operation, options)
	if err != nil {
		return BatchResult{}, err
	}
	ctx, cancel := withCallTimeout(ctx, callOptions.Timeout)
	defer cancel()
	options = nil
	page, err := client.ListKeywords(ctx, ListKeywordsRequest{
		Selection: KeywordSelection{IDs: ids}, Page: PageRequest{Limit: int64(len(ids))},
	}, options...)
	if err != nil {
		return BatchResult{}, withOperation(err, operation)
	}
	if len(page.Items) != len(ids) {
		return BatchResult{}, notFound(operation, "one or more Keywords were not returned")
	}
	expected := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		expected[id] = struct{}{}
	}
	campaigns := make(map[int64]struct{})
	for _, keyword := range page.Items {
		if _, found := expected[keyword.ID]; !found {
			return BatchResult{}, platformContractError(operation, "Yandex returned a duplicate Keyword or one outside the delete selection")
		}
		delete(expected, keyword.ID)
		if keyword.State == KeywordStateSuspended || keyword.State == KeywordStateOff {
			continue
		}
		campaigns[keyword.CampaignID] = struct{}{}
	}
	if len(expected) != 0 {
		return BatchResult{}, platformContractError(operation, "Yandex omitted one or more Keywords from the delete selection")
	}
	campaignIDs := make([]int64, 0, len(campaigns))
	for campaignID := range campaigns {
		campaignIDs = append(campaignIDs, campaignID)
	}
	sort.Slice(campaignIDs, func(i, j int) bool { return campaignIDs[i] < campaignIDs[j] })
	for _, campaignID := range campaignIDs {
		campaign, err := client.GetCampaign(ctx, campaignID, options...)
		if err != nil {
			return BatchResult{}, withOperation(err, operation)
		}
		if !safeNonServingCampaign(campaign) {
			return BatchResult{}, invalidArgument(operation, "Keywords must be suspended/off or belong to a suspended or draft/off Campaign before delete")
		}
	}
	return client.keywordAction(ctx, operation, "delete", ids, options...)
}

func (client *Client) keywordAction(ctx context.Context, operation, method string, ids []int64, options ...socialhub.CallOption) (BatchResult, error) {
	if !validIDs(ids, MaximumKeywordActionBatch, false) {
		return BatchResult{}, invalidArgument(operation, "1-10000 unique Keyword IDs are required")
	}
	params := struct {
		SelectionCriteria struct {
			IDs []int64 `json:"Ids"`
		} `json:"SelectionCriteria"`
	}{}
	params.SelectionCriteria.IDs = ids
	var response struct {
		SuspendResults []ActionResult `json:"SuspendResults"`
		ResumeResults  []ActionResult `json:"ResumeResults"`
		DeleteResults  []ActionResult `json:"DeleteResults"`
	}
	metadata, err := client.rpc(ctx, operation, "keywords", method, params, &response, true, options...)
	if err != nil {
		return BatchResult{Metadata: metadata}, err
	}
	items := response.SuspendResults
	if method == "resume" {
		items = response.ResumeResults
	} else if method == "delete" {
		items = response.DeleteResults
	}
	return actionResult(operation, items, len(ids), metadata, ids...)
}

func validateKeyword(operation string, keyword *Keyword, expectedID int64) error {
	if keyword == nil || keyword.ID <= 0 || keyword.AdGroupID <= 0 || keyword.CampaignID <= 0 ||
		!validText(keyword.Keyword, 4096) || keyword.Bid < 0 || keyword.ContextBid < 0 ||
		keyword.State == "" || keyword.Status == "" || keyword.StrategyPriority != "" && !validPriority(keyword.StrategyPriority) {
		return platformContractError(operation, "Yandex returned an invalid Keyword")
	}
	if expectedID > 0 && keyword.ID != expectedID {
		return platformContractError(operation, "Keyword ID did not match the request")
	}
	if keyword.AutotargetingSearchBidIsAuto != "" && !validYesNo(keyword.AutotargetingSearchBidIsAuto) {
		return platformContractError(operation, "Yandex returned invalid Keyword autotargeting metadata")
	}
	return nil
}

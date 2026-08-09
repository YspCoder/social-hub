package microsoftads

import (
	"context"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListKeywords(ctx context.Context, campaignID, adGroupID string, options ...socialhub.CallOption) ([]Keyword, error) {
	const operation = "list_keywords"
	if !validNumericID(campaignID) || !validNumericID(adGroupID) {
		return nil, invalidArgument(operation, "campaign and ad group IDs must be nonzero numeric IDs")
	}
	if _, err := client.GetAdGroup(ctx, campaignID, adGroupID, options...); err != nil {
		return nil, err
	}
	var response struct {
		Keywords []Keyword `json:"Keywords"`
	}
	_, err := client.postJSON(ctx, operation, client.campaign, "/Keywords/QueryByAdGroupId", struct {
		AdGroupID string `json:"AdGroupId"`
	}{AdGroupID: adGroupID}, &response, options...)
	if err != nil {
		return nil, err
	}
	return response.Keywords, nil
}

func (client *Client) GetKeyword(ctx context.Context, campaignID, adGroupID, keywordID string, options ...socialhub.CallOption) (*Keyword, error) {
	const operation = "get_keyword"
	if !validNumericID(campaignID) || !validNumericID(adGroupID) || !validNumericID(keywordID) {
		return nil, invalidArgument(operation, "campaign, ad group, and keyword IDs must be nonzero numeric IDs")
	}
	if _, err := client.GetAdGroup(ctx, campaignID, adGroupID, options...); err != nil {
		return nil, err
	}
	var response struct {
		Keywords      []Keyword     `json:"Keywords"`
		PartialErrors []wireFailure `json:"PartialErrors"`
	}
	header, err := client.postJSON(ctx, operation, client.campaign, "/Keywords/QueryByIds", struct {
		AdGroupID  string   `json:"AdGroupId"`
		KeywordIDs []string `json:"KeywordIds"`
	}{AdGroupID: adGroupID, KeywordIDs: []string{keywordID}}, &response, options...)
	if err != nil {
		return nil, err
	}
	if err := checkPartialErrors(operation, header, response.PartialErrors); err != nil {
		return nil, err
	}
	if len(response.Keywords) != 1 || response.Keywords[0].ID != keywordID {
		return nil, platformContractError(operation, "response keyword does not match requested ad group and ID")
	}
	return &response.Keywords[0], nil
}

func (client *Client) CreateKeyword(ctx context.Context, campaignID, adGroupID string, input CreateKeywordRequest, options ...socialhub.CallOption) (*Keyword, error) {
	const operation = "create_keyword"
	if !validNumericID(campaignID) || !validNumericID(adGroupID) || !validRequiredText(input.Text, 100) || !validMatchType(input.MatchType) ||
		(input.Bid != nil && *input.Bid <= 0) || (len(input.FinalURLs) > 0 && !validateFinalURLs(input.FinalURLs)) {
		return nil, invalidArgument(operation, "ad group ID, text, match type, bid, or final URLs are invalid")
	}
	if err := client.validateAccount(ctx, options...); err != nil {
		return nil, err
	}
	if _, err := client.GetAdGroup(ctx, campaignID, adGroupID, options...); err != nil {
		return nil, err
	}
	payload := keywordWrite{Text: &input.Text, MatchType: &input.MatchType, Status: statusPointer(StatusPaused)}
	if input.Bid != nil {
		payload.Bid = &Bid{Amount: *input.Bid}
	}
	if len(input.FinalURLs) > 0 {
		payload.FinalURLs = &input.FinalURLs
	}
	var response struct {
		KeywordIDs    []*string     `json:"KeywordIds"`
		PartialErrors []wireFailure `json:"PartialErrors"`
	}
	header, err := client.postJSON(ctx, operation, client.campaign, "/Keywords", struct {
		AdGroupID string         `json:"AdGroupId"`
		Keywords  []keywordWrite `json:"Keywords"`
	}{AdGroupID: adGroupID, Keywords: []keywordWrite{payload}}, &response, options...)
	if err != nil {
		return nil, err
	}
	if err := checkPartialErrors(operation, header, response.PartialErrors); err != nil {
		return nil, err
	}
	if len(response.KeywordIDs) != 1 || response.KeywordIDs[0] == nil || !validNumericID(*response.KeywordIDs[0]) {
		return nil, platformContractError(operation, "response did not contain one keyword ID")
	}
	return client.GetKeyword(ctx, campaignID, adGroupID, *response.KeywordIDs[0], options...)
}

func (client *Client) UpdateKeyword(ctx context.Context, campaignID, adGroupID, keywordID string, input UpdateKeywordRequest, options ...socialhub.CallOption) (*Keyword, error) {
	const operation = "update_keyword"
	if !validNumericID(campaignID) || !validNumericID(adGroupID) || !validNumericID(keywordID) || input.empty() ||
		(input.Text != nil && !validRequiredText(*input.Text, 100)) ||
		(input.MatchType != nil && !validMatchType(*input.MatchType)) ||
		(input.Bid != nil && *input.Bid <= 0) ||
		(input.FinalURLs != nil && len(*input.FinalURLs) > 0 && !validateFinalURLs(*input.FinalURLs)) {
		return nil, invalidArgument(operation, "IDs and at least one valid keyword update field are required")
	}
	if err := client.validateAccount(ctx, options...); err != nil {
		return nil, err
	}
	if _, err := client.GetKeyword(ctx, campaignID, adGroupID, keywordID, options...); err != nil {
		return nil, err
	}
	payload := keywordWrite{ID: keywordID, Text: input.Text, MatchType: input.MatchType, FinalURLs: input.FinalURLs}
	if input.Bid != nil {
		payload.Bid = &Bid{Amount: *input.Bid}
	}
	if err := client.updateKeyword(ctx, operation, adGroupID, payload, options...); err != nil {
		return nil, err
	}
	return client.GetKeyword(ctx, campaignID, adGroupID, keywordID, options...)
}

func (client *Client) SetKeywordStatus(ctx context.Context, campaignID, adGroupID, keywordID string, status Status, options ...socialhub.CallOption) (*Keyword, error) {
	const operation = "set_keyword_status"
	if !validNumericID(campaignID) || !validNumericID(adGroupID) || !validNumericID(keywordID) || !validStatus(status) {
		return nil, invalidArgument(operation, "IDs and Active or Paused status are required")
	}
	if err := client.validateAccount(ctx, options...); err != nil {
		return nil, err
	}
	if _, err := client.GetKeyword(ctx, campaignID, adGroupID, keywordID, options...); err != nil {
		return nil, err
	}
	if err := client.updateKeyword(ctx, operation, adGroupID, keywordWrite{ID: keywordID, Status: &status}, options...); err != nil {
		return nil, err
	}
	return client.GetKeyword(ctx, campaignID, adGroupID, keywordID, options...)
}

type keywordWrite struct {
	ID        string     `json:"Id,omitempty"`
	Text      *string    `json:"Text,omitempty"`
	Status    *Status    `json:"Status,omitempty"`
	MatchType *MatchType `json:"MatchType,omitempty"`
	Bid       *Bid       `json:"Bid,omitempty"`
	FinalURLs *[]string  `json:"FinalUrls,omitempty"`
}

func (client *Client) updateKeyword(ctx context.Context, operation, adGroupID string, payload keywordWrite, options ...socialhub.CallOption) error {
	var response struct {
		PartialErrors []wireFailure `json:"PartialErrors"`
	}
	header, err := client.putJSON(ctx, operation, "/Keywords", struct {
		AdGroupID string         `json:"AdGroupId"`
		Keywords  []keywordWrite `json:"Keywords"`
	}{AdGroupID: adGroupID, Keywords: []keywordWrite{payload}}, &response, options...)
	if err != nil {
		return err
	}
	return checkPartialErrors(operation, header, response.PartialErrors)
}

func (input UpdateKeywordRequest) empty() bool {
	return input.Text == nil && input.MatchType == nil && input.Bid == nil && input.FinalURLs == nil
}

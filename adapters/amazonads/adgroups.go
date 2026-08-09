package amazonads

import (
	"context"
	"net/http"

	"social-hub/pkg/socialhub"
)

type adGroupListEnvelope struct {
	AdGroups     []AdGroup `json:"adGroups"`
	NextToken    string    `json:"nextToken"`
	TotalResults int       `json:"totalResults"`
}

type adGroupMutationEnvelope struct {
	AdGroups struct {
		Success []struct {
			Index     int     `json:"index"`
			AdGroupID string  `json:"adGroupId"`
			AdGroup   AdGroup `json:"adGroup"`
		} `json:"success"`
		Error []mutationFailure `json:"error"`
	} `json:"adGroups"`
}

type adGroupMutationResource struct {
	ID         string   `json:"adGroupId,omitempty"`
	CampaignID string   `json:"campaignId,omitempty"`
	Name       string   `json:"name,omitempty"`
	DefaultBid *Decimal `json:"defaultBid,omitempty"`
	State      State    `json:"state,omitempty"`
}

func (client *Client) ListAdGroups(ctx context.Context, input ListAdGroupsRequest, options ...socialhub.CallOption) (Page[AdGroup], error) {
	const operation = "ad_groups_list"
	if !validIDs(input.IDs) || !validIDs(input.CampaignIDs) || !validStates(input.States) || !validList(input.MaxResults, input.NextToken) {
		return Page[AdGroup]{}, invalidArgument(operation, "IDs, states, max results, or next token are invalid")
	}
	body := struct {
		AdGroupIDFilter  *includeFilter[string] `json:"adGroupIdFilter,omitempty"`
		CampaignIDFilter *includeFilter[string] `json:"campaignIdFilter,omitempty"`
		StateFilter      *includeFilter[State]  `json:"stateFilter,omitempty"`
		MaxResults       int                    `json:"maxResults,omitempty"`
		NextToken        string                 `json:"nextToken,omitempty"`
	}{MaxResults: input.MaxResults, NextToken: input.NextToken}
	if len(input.IDs) > 0 {
		body.AdGroupIDFilter = &includeFilter[string]{Include: input.IDs}
	}
	if len(input.CampaignIDs) > 0 {
		body.CampaignIDFilter = &includeFilter[string]{Include: input.CampaignIDs}
	}
	if len(input.States) > 0 {
		body.StateFilter = &includeFilter[State]{Include: input.States}
	}
	var response adGroupListEnvelope
	if _, err := client.vendorJSON(ctx, operation, http.MethodPost, "/sp/adGroups/list", adGroupMediaType, body, &response, false, options...); err != nil {
		return Page[AdGroup]{}, err
	}
	for _, adGroup := range response.AdGroups {
		if !validID(adGroup.ID) || !validID(adGroup.CampaignID) {
			return Page[AdGroup]{}, platformContractError(operation, "Amazon Ads returned an invalid Ad Group or Campaign ID")
		}
	}
	return Page[AdGroup]{Items: response.AdGroups, NextToken: response.NextToken, TotalResults: response.TotalResults}, nil
}

func (client *Client) CreateAdGroup(ctx context.Context, input CreateAdGroupRequest, options ...socialhub.CallOption) (*AdGroup, error) {
	const operation = "ad_group_create"
	if !validID(input.CampaignID) || !validText(input.Name, 128) || !validDecimal(string(input.DefaultBid), true) {
		return nil, invalidArgument(operation, "campaign ID, Ad Group name, or default bid is invalid")
	}
	resource := adGroupMutationResource{CampaignID: input.CampaignID, Name: input.Name, DefaultBid: &input.DefaultBid, State: StatePaused}
	return client.mutateAdGroup(ctx, operation, http.MethodPost, "/sp/adGroups", resource, "", options...)
}

func (client *Client) UpdateAdGroup(ctx context.Context, id string, input UpdateAdGroupRequest, options ...socialhub.CallOption) (*AdGroup, error) {
	const operation = "ad_group_update"
	if !validID(id) || input.Name == nil && input.DefaultBid == nil {
		return nil, invalidArgument(operation, "Ad Group ID and at least one update are required")
	}
	resource := adGroupMutationResource{ID: id}
	if input.Name != nil {
		if !validText(*input.Name, 128) {
			return nil, invalidArgument(operation, "Ad Group name is invalid")
		}
		resource.Name = *input.Name
	}
	if input.DefaultBid != nil {
		if !validDecimal(string(*input.DefaultBid), true) {
			return nil, invalidArgument(operation, "default bid is invalid")
		}
		resource.DefaultBid = input.DefaultBid
	}
	return client.mutateAdGroup(ctx, operation, http.MethodPut, "/sp/adGroups", resource, id, options...)
}

func (client *Client) SetAdGroupState(ctx context.Context, id string, state State, options ...socialhub.CallOption) (*AdGroup, error) {
	if !validID(id) || !validState(state) {
		return nil, invalidArgument("ad_group_state", "Ad Group ID and ENABLED or PAUSED state are required")
	}
	return client.mutateAdGroup(ctx, "ad_group_state", http.MethodPut, "/sp/adGroups", adGroupMutationResource{ID: id, State: state}, id, options...)
}

func (client *Client) ArchiveAdGroup(ctx context.Context, id string, options ...socialhub.CallOption) error {
	const operation = "ad_group_archive"
	if !validID(id) {
		return invalidArgument(operation, "Ad Group ID is invalid")
	}
	body := struct {
		Filter includeFilter[string] `json:"adGroupIdFilter"`
	}{Filter: includeFilter[string]{Include: []string{id}}}
	var response adGroupMutationEnvelope
	metadata, err := client.vendorJSON(ctx, operation, http.MethodPost, "/sp/adGroups/delete", adGroupMediaType, body, &response, true, options...)
	if err != nil {
		return err
	}
	_, err = adGroupMutationResult(operation, id, metadata.StatusCode, metadata.Header, response)
	return err
}

func (client *Client) mutateAdGroup(ctx context.Context, operation, method, path string, resource adGroupMutationResource, expected string, options ...socialhub.CallOption) (*AdGroup, error) {
	body := struct {
		AdGroups []adGroupMutationResource `json:"adGroups"`
	}{AdGroups: []adGroupMutationResource{resource}}
	var response adGroupMutationEnvelope
	metadata, err := client.vendorJSON(ctx, operation, method, path, adGroupMediaType, body, &response, true, options...)
	if err != nil {
		return nil, err
	}
	return adGroupMutationResult(operation, expected, metadata.StatusCode, metadata.Header, response)
}

func adGroupMutationResult(operation, expected string, status int, header http.Header, response adGroupMutationEnvelope) (*AdGroup, error) {
	if len(response.AdGroups.Error) > 0 {
		return nil, mutationError(operation, status, header, response.AdGroups.Error[0])
	}
	if len(response.AdGroups.Success) != 1 {
		return nil, platformContractError(operation, "Amazon Ads did not return exactly one Ad Group mutation result")
	}
	item := response.AdGroups.Success[0]
	if err := requireMutationID(operation, expected, item.AdGroupID); err != nil {
		return nil, err
	}
	if item.AdGroup.ID == "" {
		item.AdGroup.ID = item.AdGroupID
	}
	if item.AdGroup.ID != item.AdGroupID {
		return nil, platformContractError(operation, "Amazon Ads returned mismatched Ad Group IDs")
	}
	return &item.AdGroup, nil
}

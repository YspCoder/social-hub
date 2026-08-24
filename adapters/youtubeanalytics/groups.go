package youtubeanalytics

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

type GroupsWorkflow interface {
	ListGroups(context.Context, ListGroupsRequest, ...socialhub.CallOption) (*ListGroupsResponse, error)
	CreateGroup(context.Context, CreateGroupInput, ...socialhub.CallOption) (*Group, error)
	RenameGroup(context.Context, string, string, ...socialhub.CallOption) (*Group, error)
	DeleteGroup(context.Context, string, ...socialhub.CallOption) error
	ListGroupItems(context.Context, string, ...socialhub.CallOption) (*ListGroupItemsResponse, error)
	AddGroupItem(context.Context, AddGroupItemInput, ...socialhub.CallOption) (*AddGroupItemResult, error)
	RemoveGroupItem(context.Context, string, ...socialhub.CallOption) error
}

func (client *Client) ListGroups(ctx context.Context, input ListGroupsRequest, options ...socialhub.CallOption) (*ListGroupsResponse, error) {
	const operation = "groups_list"
	if !validListGroupsRequest(input) {
		return nil, invalidArgument(operation, "specify either unique group IDs or mine=true, with a valid page token")
	}
	if err := client.requireGroupReadScope(operation); err != nil {
		return nil, err
	}
	query := url.Values{}
	if input.Mine {
		query.Set("mine", "true")
	} else {
		query.Set("id", strings.Join(input.IDs, ","))
	}
	if input.PageToken != "" {
		query.Set("pageToken", input.PageToken)
	}
	client.ownerQuery(query)
	var output ListGroupsResponse
	if err := client.getJSON(ctx, operation, "/v2/groups", query, &output, options...); err != nil {
		return nil, err
	}
	if !validGroupsResponse(&output, client.binding.ContentOwnerID != "") {
		return nil, platformContractError(operation, "YouTube Analytics returned a malformed group list")
	}
	if len(input.IDs) > 0 {
		requested := make(map[string]struct{}, len(input.IDs))
		for _, id := range input.IDs {
			requested[id] = struct{}{}
		}
		for _, group := range output.Items {
			if _, found := requested[group.ID]; !found {
				return nil, platformContractError(operation, "YouTube Analytics returned a group outside the requested ID set")
			}
		}
	}
	return &output, nil
}

func (client *Client) CreateGroup(ctx context.Context, input CreateGroupInput, options ...socialhub.CallOption) (*Group, error) {
	const operation = "group_create"
	contentOwner := client.binding.ContentOwnerID != ""
	if !validText(input.Title, 1024, true) || !validResourceKind(input.ItemType, contentOwner) {
		return nil, invalidArgument(operation, "group title or item type is invalid for the configured account")
	}
	if err := client.requireGroupWriteScope(operation); err != nil {
		return nil, err
	}
	query := url.Values{}
	client.ownerQuery(query)
	body := Group{Snippet: &GroupSnippet{Title: input.Title}, ContentDetails: &GroupContentDetails{ItemType: input.ItemType}}
	var output Group
	if _, err := client.postJSON(ctx, operation, "/v2/groups", query, body, &output, options...); err != nil {
		return nil, err
	}
	if !validGroup(&output, contentOwner) || output.Snippet.Title != input.Title || output.ContentDetails.ItemType != input.ItemType {
		return nil, platformContractError(operation, "YouTube Analytics returned a malformed created group")
	}
	return &output, nil
}

func (client *Client) RenameGroup(ctx context.Context, groupID, title string, options ...socialhub.CallOption) (*Group, error) {
	const operation = "group_rename"
	if !validOpaqueID(groupID) || !validText(title, 1024, true) {
		return nil, invalidArgument(operation, "group ID and title are required")
	}
	if err := client.requireGroupWriteScope(operation); err != nil {
		return nil, err
	}
	query := url.Values{}
	client.ownerQuery(query)
	body := Group{ID: groupID, Snippet: &GroupSnippet{Title: title}}
	var output Group
	if err := client.putJSON(ctx, operation, "/v2/groups", query, body, &output, options...); err != nil {
		return nil, err
	}
	if !validGroup(&output, client.binding.ContentOwnerID != "") || output.ID != groupID || output.Snippet.Title != title {
		return nil, platformContractError(operation, "YouTube Analytics returned a malformed or mismatched updated group")
	}
	return &output, nil
}

func (client *Client) DeleteGroup(ctx context.Context, groupID string, options ...socialhub.CallOption) error {
	const operation = "group_delete"
	if !validOpaqueID(groupID) {
		return invalidArgument(operation, "group ID is required")
	}
	if err := client.requireGroupWriteScope(operation); err != nil {
		return err
	}
	query := url.Values{"id": {groupID}}
	client.ownerQuery(query)
	return client.deleteJSON(ctx, operation, "/v2/groups", query, options...)
}

func (client *Client) ListGroupItems(ctx context.Context, groupID string, options ...socialhub.CallOption) (*ListGroupItemsResponse, error) {
	const operation = "group_items_list"
	if !validOpaqueID(groupID) {
		return nil, invalidArgument(operation, "group ID is required")
	}
	if err := client.requireGroupReadScope(operation); err != nil {
		return nil, err
	}
	query := url.Values{"groupId": {groupID}}
	client.ownerQuery(query)
	var output ListGroupItemsResponse
	if err := client.getJSON(ctx, operation, "/v2/groupItems", query, &output, options...); err != nil {
		return nil, err
	}
	if !validGroupItemsResponse(&output, client.binding.ContentOwnerID != "") {
		return nil, platformContractError(operation, "YouTube Analytics returned a malformed group-item list")
	}
	for _, item := range output.Items {
		if item.GroupID != groupID {
			return nil, platformContractError(operation, "YouTube Analytics returned a group item for a different group")
		}
	}
	return &output, nil
}

func (client *Client) AddGroupItem(ctx context.Context, input AddGroupItemInput, options ...socialhub.CallOption) (*AddGroupItemResult, error) {
	const operation = "group_item_add"
	contentOwner := client.binding.ContentOwnerID != ""
	if !validOpaqueID(input.GroupID) || !validOpaqueID(input.ResourceID) || !validResourceKind(input.Kind, contentOwner) {
		return nil, invalidArgument(operation, "group ID, resource ID, or resource kind is invalid for the configured account")
	}
	if err := client.requireGroupWriteScope(operation); err != nil {
		return nil, err
	}
	query := url.Values{}
	client.ownerQuery(query)
	body := GroupItem{GroupID: input.GroupID, Resource: &GroupItemResource{ID: input.ResourceID, Kind: input.Kind}}
	var output GroupItem
	metadata, err := client.postJSON(ctx, operation, "/v2/groupItems", query, body, &output, options...)
	if err != nil {
		return nil, err
	}
	if metadata.StatusCode == http.StatusNoContent {
		return &AddGroupItemResult{AlreadyPresent: true}, nil
	}
	if !validGroupItem(&output, contentOwner) || output.GroupID != input.GroupID || output.Resource.ID != input.ResourceID || output.Resource.Kind != input.Kind {
		return nil, platformContractError(operation, "YouTube Analytics returned a malformed or mismatched group item")
	}
	return &AddGroupItemResult{Item: &output}, nil
}

func (client *Client) RemoveGroupItem(ctx context.Context, groupItemID string, options ...socialhub.CallOption) error {
	const operation = "group_item_remove"
	if !validOpaqueID(groupItemID) {
		return invalidArgument(operation, "group item ID is required")
	}
	if err := client.requireGroupWriteScope(operation); err != nil {
		return err
	}
	query := url.Values{"id": {groupItemID}}
	client.ownerQuery(query)
	return client.deleteJSON(ctx, operation, "/v2/groupItems", query, options...)
}

var _ GroupsWorkflow = (*Client)(nil)

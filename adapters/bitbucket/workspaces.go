package bitbucket

import (
	"context"
	"strconv"

	"social-hub/pkg/socialhub"
)

// ListWorkspaces returns workspaces visible to the current caller together
// with the caller's administrator flag.
func (client *Client) ListWorkspaces(
	ctx context.Context,
	input ListWorkspacesRequest,
	options ...socialhub.CallOption,
) (*Page[WorkspaceAccess], error) {
	const operation = "list_workspaces"
	if !validWorkspaceRequest(input) {
		return nil, invalidArgument(operation, "administrator, sort, pagination, or next query is invalid")
	}
	query, err := pageQuery(input.Page)
	if err != nil {
		return nil, invalidArgument(operation, "pagination or next query is invalid")
	}
	if input.Page.NextQuery == "" {
		if input.Administrator != nil {
			query.Set("administrator", strconv.FormatBool(*input.Administrator))
		}
		if input.Sort != "" {
			query.Set("sort", string(input.Sort))
		}
	}
	const path = "/user/workspaces"
	meta, raw, err := client.getJSON(ctx, operation, path, query, nil, options...)
	if err != nil {
		return nil, err
	}
	return decodePage(operation, path, meta, raw, validWorkspaceAccess, "workspace access")
}

// GetWorkspace returns one workspace by slug or brace-wrapped UUID.
func (client *Client) GetWorkspace(ctx context.Context, workspace string, options ...socialhub.CallOption) (*Workspace, ResponseMeta, error) {
	const operation = "get_workspace"
	if !validResourceSelector(workspace) {
		return nil, ResponseMeta{}, invalidArgument(operation, "workspace must be a safe slug or brace-wrapped UUID")
	}
	var result Workspace
	meta, _, err := client.getJSON(ctx, operation, "/workspaces/"+workspace, nil, &result, options...)
	if err != nil {
		return nil, meta, err
	}
	if !matchesWorkspaceDetail(result, workspace) {
		return nil, meta, platformContractError(operation, "Bitbucket returned a workspace without a valid or matching uuid or slug")
	}
	return &result, meta, nil
}

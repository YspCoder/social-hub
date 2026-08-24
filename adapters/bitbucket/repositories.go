package bitbucket

import (
	"context"
	"net/url"

	"social-hub/pkg/socialhub"
)

// ListRepositories returns repositories owned by one workspace and visible to
// the configured credential.
func (client *Client) ListRepositories(
	ctx context.Context,
	workspace string,
	input ListRepositoriesRequest,
	options ...socialhub.CallOption,
) (*Page[Repository], error) {
	const operation = "list_repositories"
	if !validResourceSelector(workspace) || !validRepositoryRequest(input) {
		return nil, invalidArgument(operation, "workspace, role, filter, sort, pagination, or next query is invalid")
	}
	query, err := pageQuery(input.Page)
	if err != nil {
		return nil, invalidArgument(operation, "pagination or next query is invalid")
	}
	if input.Page.NextQuery == "" {
		setQuery(query, "role", string(input.Role))
		setQuery(query, "q", input.Query)
		setQuery(query, "sort", input.Sort)
	}
	path := "/repositories/" + workspace
	meta, raw, err := client.getJSON(ctx, operation, path, query, nil, options...)
	if err != nil {
		return nil, err
	}
	return decodePage(operation, path, meta, raw, func(value Repository) bool {
		return validRepository(value) && matchesWorkspaceSelector(value.Workspace, workspace)
	}, "repository")
}

// GetRepository returns one repository by workspace and repository slug or
// brace-wrapped UUID selectors.
func (client *Client) GetRepository(
	ctx context.Context,
	workspace string,
	repository string,
	options ...socialhub.CallOption,
) (*Repository, ResponseMeta, error) {
	const operation = "get_repository"
	if !validResourceSelector(workspace) || !validResourceSelector(repository) {
		return nil, ResponseMeta{}, invalidArgument(operation, "workspace and repository must be safe slugs or brace-wrapped UUIDs")
	}
	var result Repository
	meta, _, err := client.getJSON(ctx, operation, repositoryPath(workspace, repository), nil, &result, options...)
	if err != nil {
		return nil, meta, err
	}
	if !matchesRepositorySelector(&result, workspace, repository) {
		return nil, meta, platformContractError(operation, "Bitbucket returned a repository without valid or matching workspace, uuid, slug, full_name, or name fields")
	}
	return &result, meta, nil
}

func repositoryPath(workspace, repository string) string {
	return "/repositories/" + workspace + "/" + repository
}

func setQuery(query url.Values, key, value string) {
	if value != "" {
		query.Set(key, value)
	}
}

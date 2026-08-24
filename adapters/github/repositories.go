package github

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

// ListAuthenticatedRepositories returns repositories visible to the configured token.
func (client *Client) ListAuthenticatedRepositories(
	ctx context.Context,
	input ListAuthenticatedRepositoriesRequest,
	options ...socialhub.CallOption,
) (*Page[Repository], error) {
	const operation = "list_authenticated_repositories"
	if !validAuthenticatedRepositoryRequest(input) {
		return nil, invalidArgument(operation, "visibility, affiliation, type, sort, direction, dates, or pagination is invalid")
	}
	query := make(url.Values)
	setQuery(query, "visibility", string(input.Visibility))
	if len(input.Affiliation) > 0 {
		values := make([]string, len(input.Affiliation))
		for index, value := range input.Affiliation {
			values[index] = string(value)
		}
		query.Set("affiliation", strings.Join(values, ","))
	}
	setQuery(query, "type", string(input.Type))
	setQuery(query, "sort", string(input.Sort))
	setQuery(query, "direction", string(input.Direction))
	setTimeQuery(query, "since", input.Since)
	setTimeQuery(query, "before", input.Before)
	setPaginationQuery(query, input.PerPage, input.Page)
	return client.listRepositories(ctx, operation, "/user/repos", query, options...)
}

// ListRepositoriesForUser returns public repositories for one user.
func (client *Client) ListRepositoriesForUser(
	ctx context.Context,
	username string,
	input ListUserRepositoriesRequest,
	options ...socialhub.CallOption,
) (*Page[Repository], error) {
	const operation = "list_repositories_for_user"
	if !validPathSegment(username) || !validUserRepositoryRequest(input) {
		return nil, invalidArgument(operation, "username, type, sort, direction, or pagination is invalid")
	}
	query := make(url.Values)
	setQuery(query, "type", string(input.Type))
	setQuery(query, "sort", string(input.Sort))
	setQuery(query, "direction", string(input.Direction))
	setPaginationQuery(query, input.PerPage, input.Page)
	return client.listRepositories(ctx, operation, "/users/"+username+"/repos", query, options...)
}

func (client *Client) listRepositories(
	ctx context.Context,
	operation string,
	path string,
	query url.Values,
	options ...socialhub.CallOption,
) (*Page[Repository], error) {
	var repositories []Repository
	meta, raw, err := client.getJSON(ctx, operation, path, query, '[', &repositories, options...)
	if err != nil {
		return nil, err
	}
	for _, repository := range repositories {
		if !validRepository(repository) {
			return nil, platformContractError(operation, "GitHub returned a repository without a valid id, name, full_name, or owner")
		}
	}
	return &Page[Repository]{Items: repositories, Links: pageLinks(meta.Link), Meta: meta, Raw: raw}, nil
}

// GetRepository returns one repository by owner and name.
func (client *Client) GetRepository(
	ctx context.Context,
	owner string,
	repositoryName string,
	options ...socialhub.CallOption,
) (*Repository, ResponseMeta, error) {
	const operation = "get_repository"
	if !validPathSegment(owner) || !validPathSegment(repositoryName) {
		return nil, ResponseMeta{}, invalidArgument(operation, "owner or repository name is invalid")
	}
	var repository Repository
	meta, _, err := client.getJSON(ctx, operation, repositoryPath(owner, repositoryName), nil, '{', &repository, options...)
	if err != nil {
		return nil, meta, err
	}
	if !validRepository(repository) {
		return nil, meta, platformContractError(operation, "GitHub returned a repository without a valid id, name, full_name, or owner")
	}
	return &repository, meta, nil
}

func repositoryPath(owner, repositoryName string) string {
	return "/repos/" + owner + "/" + repositoryName
}

func setQuery(query url.Values, key, value string) {
	if value != "" {
		query.Set(key, value)
	}
}

func setPaginationQuery(query url.Values, perPage, page int) {
	if perPage > 0 {
		query.Set("per_page", strconv.Itoa(perPage))
	}
	if page > 0 {
		query.Set("page", strconv.Itoa(page))
	}
}

func setTimeQuery(query url.Values, key string, value *time.Time) {
	if value != nil {
		query.Set(key, value.UTC().Format(time.RFC3339))
	}
}

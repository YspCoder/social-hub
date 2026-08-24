package gitlab

import (
	"context"
	"net/url"
	"strconv"
	"time"

	"social-hub/pkg/socialhub"
)

// ListProjects returns one offset-paginated page visible to the token.
func (client *Client) ListProjects(
	ctx context.Context,
	input ListProjectsRequest,
	options ...socialhub.CallOption,
) (*Page[Project], error) {
	const operation = "list_projects"
	if !validListProjects(input) {
		return nil, invalidArgument(operation, "visibility, ordering, search, or pagination is invalid")
	}
	query := make(url.Values)
	setQuery(query, "visibility", string(input.Visibility))
	setQuery(query, "order_by", string(input.OrderBy))
	setQuery(query, "sort", string(input.Sort))
	setQuery(query, "search", input.Search)
	if input.Membership {
		query.Set("membership", "true")
	}
	if input.Owned {
		query.Set("owned", "true")
	}
	if input.Archived != nil {
		query.Set("archived", strconv.FormatBool(*input.Archived))
	}
	setPaginationQuery(query, input.PerPage, input.Page)
	var projects []Project
	meta, raw, err := client.getJSON(ctx, operation, "/projects", query, '[', &projects, options...)
	if err != nil {
		return nil, err
	}
	for _, project := range projects {
		if !validProject(project) {
			return nil, platformContractError(operation, "GitLab returned a project without a valid id, name, path, namespace path, or web URL")
		}
	}
	return buildPage(operation, projects, meta, raw)
}

// GetProject returns one project by decimal global project ID.
func (client *Client) GetProject(ctx context.Context, projectID string, options ...socialhub.CallOption) (*Project, ResponseMeta, error) {
	const operation = "get_project"
	if !validDecimalID(projectID) {
		return nil, ResponseMeta{}, invalidArgument(operation, "project ID is invalid")
	}
	var project Project
	meta, _, err := client.getJSON(ctx, operation, projectPath(projectID), nil, '{', &project, options...)
	if err != nil {
		return nil, meta, err
	}
	if !validProject(project) || string(project.ID) != projectID {
		return nil, meta, platformContractError(operation, "GitLab returned an absent or mismatched project ID")
	}
	return &project, meta, nil
}

func projectPath(projectID string) string {
	return "/projects/" + projectID
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

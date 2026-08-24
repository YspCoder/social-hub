package gitlab

import (
	"context"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

// ListProjectIssues returns one offset-paginated project issue page.
func (client *Client) ListProjectIssues(
	ctx context.Context,
	projectID string,
	input ListProjectIssuesRequest,
	options ...socialhub.CallOption,
) (*Page[Issue], error) {
	const operation = "list_project_issues"
	if !validDecimalID(projectID) || !validListProjectIssues(input) {
		return nil, invalidArgument(operation, "project ID, issue filters, ordering, dates, or pagination is invalid")
	}
	query := make(url.Values)
	setQuery(query, "state", string(input.State))
	if len(input.Labels) > 0 {
		query.Set("labels", strings.Join(input.Labels, ","))
	}
	setQuery(query, "order_by", string(input.OrderBy))
	setQuery(query, "sort", string(input.Sort))
	setQuery(query, "search", input.Search)
	setTimeQuery(query, "created_after", input.CreatedAfter)
	setTimeQuery(query, "created_before", input.CreatedBefore)
	setTimeQuery(query, "updated_after", input.UpdatedAfter)
	setTimeQuery(query, "updated_before", input.UpdatedBefore)
	setPaginationQuery(query, input.PerPage, input.Page)
	var issues []Issue
	meta, raw, err := client.getJSON(ctx, operation, projectPath(projectID)+"/issues", query, '[', &issues, options...)
	if err != nil {
		return nil, err
	}
	for _, issue := range issues {
		if !validIssue(issue) || string(issue.ProjectID) != projectID {
			return nil, platformContractError(operation, "GitLab returned an issue without valid or matching project, global, or internal IDs")
		}
	}
	return buildPage(operation, issues, meta, raw)
}

// GetProjectIssue returns one issue by project global ID and project-local IID.
func (client *Client) GetProjectIssue(
	ctx context.Context,
	projectID string,
	issueIID string,
	options ...socialhub.CallOption,
) (*Issue, ResponseMeta, error) {
	const operation = "get_project_issue"
	if !validDecimalID(projectID) || !validDecimalID(issueIID) {
		return nil, ResponseMeta{}, invalidArgument(operation, "project ID or issue IID is invalid")
	}
	var issue Issue
	path := projectPath(projectID) + "/issues/" + issueIID
	meta, _, err := client.getJSON(ctx, operation, path, nil, '{', &issue, options...)
	if err != nil {
		return nil, meta, err
	}
	if !validIssue(issue) || string(issue.ProjectID) != projectID || string(issue.IID) != issueIID {
		return nil, meta, platformContractError(operation, "GitLab returned an absent or mismatched project ID or issue IID")
	}
	return &issue, meta, nil
}

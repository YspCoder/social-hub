package github

import (
	"context"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

// ListIssues returns one repository issue page. GitHub includes pull requests
// in this endpoint; callers can identify them through Issue.PullRequest.
func (client *Client) ListIssues(
	ctx context.Context,
	owner string,
	repositoryName string,
	input ListIssuesRequest,
	options ...socialhub.CallOption,
) (*Page[Issue], error) {
	const operation = "list_issues"
	if !validPathSegment(owner) || !validPathSegment(repositoryName) || !validIssueRequest(input) {
		return nil, invalidArgument(operation, "owner, repository, filters, or pagination is invalid")
	}
	query := make(url.Values)
	setQuery(query, "state", string(input.State))
	if len(input.Labels) > 0 {
		query.Set("labels", strings.Join(input.Labels, ","))
	}
	setQuery(query, "sort", string(input.Sort))
	setQuery(query, "direction", string(input.Direction))
	setTimeQuery(query, "since", input.Since)
	setPaginationQuery(query, input.PerPage, input.Page)
	var issues []Issue
	meta, raw, err := client.getJSON(ctx, operation, repositoryPath(owner, repositoryName)+"/issues", query, '[', &issues, options...)
	if err != nil {
		return nil, err
	}
	for _, issue := range issues {
		if !validIssue(issue) {
			return nil, platformContractError(operation, "GitHub returned an issue without a valid id, number, url, or repository_url")
		}
	}
	return &Page[Issue]{Items: issues, Links: pageLinks(meta.Link), Meta: meta, Raw: raw}, nil
}

// GetIssue returns one issue or pull-request issue representation.
func (client *Client) GetIssue(
	ctx context.Context,
	owner string,
	repositoryName string,
	issueNumber string,
	options ...socialhub.CallOption,
) (*Issue, ResponseMeta, error) {
	const operation = "get_issue"
	if !validPathSegment(owner) || !validPathSegment(repositoryName) || !validDecimalID(issueNumber) {
		return nil, ResponseMeta{}, invalidArgument(operation, "owner, repository, or issue number is invalid")
	}
	var issue Issue
	path := repositoryPath(owner, repositoryName) + "/issues/" + issueNumber
	meta, _, err := client.getJSON(ctx, operation, path, nil, '{', &issue, options...)
	if err != nil {
		return nil, meta, err
	}
	if !validIssue(issue) || string(issue.Number) != issueNumber {
		return nil, meta, platformContractError(operation, "GitHub returned an absent or mismatched issue number")
	}
	return &issue, meta, nil
}

// ListIssueComments returns one page of issue and pull-request conversation comments.
func (client *Client) ListIssueComments(
	ctx context.Context,
	owner string,
	repositoryName string,
	issueNumber string,
	input ListIssueCommentsRequest,
	options ...socialhub.CallOption,
) (*Page[IssueComment], error) {
	const operation = "list_issue_comments"
	if !validPathSegment(owner) || !validPathSegment(repositoryName) || !validDecimalID(issueNumber) || !validIssueCommentRequest(input) {
		return nil, invalidArgument(operation, "owner, repository, issue number, since, or pagination is invalid")
	}
	query := make(url.Values)
	setTimeQuery(query, "since", input.Since)
	setPaginationQuery(query, input.PerPage, input.Page)
	path := repositoryPath(owner, repositoryName) + "/issues/" + issueNumber + "/comments"
	var comments []IssueComment
	meta, raw, err := client.getJSON(ctx, operation, path, query, '[', &comments, options...)
	if err != nil {
		return nil, err
	}
	for _, comment := range comments {
		if !validIssueComment(comment) {
			return nil, platformContractError(operation, "GitHub returned an issue comment without a valid id, url, or issue_url")
		}
	}
	return &Page[IssueComment]{Items: comments, Links: pageLinks(meta.Link), Meta: meta, Raw: raw}, nil
}

// GetIssueComment returns one repository issue comment by decimal comment ID.
func (client *Client) GetIssueComment(
	ctx context.Context,
	owner string,
	repositoryName string,
	commentID string,
	options ...socialhub.CallOption,
) (*IssueComment, ResponseMeta, error) {
	const operation = "get_issue_comment"
	if !validPathSegment(owner) || !validPathSegment(repositoryName) || !validDecimalID(commentID) {
		return nil, ResponseMeta{}, invalidArgument(operation, "owner, repository, or comment ID is invalid")
	}
	var comment IssueComment
	path := repositoryPath(owner, repositoryName) + "/issues/comments/" + commentID
	meta, _, err := client.getJSON(ctx, operation, path, nil, '{', &comment, options...)
	if err != nil {
		return nil, meta, err
	}
	if !validIssueComment(comment) || string(comment.ID) != commentID {
		return nil, meta, platformContractError(operation, "GitHub returned an absent or mismatched issue-comment ID")
	}
	return &comment, meta, nil
}

var _ ReadWorkflow = (*Client)(nil)

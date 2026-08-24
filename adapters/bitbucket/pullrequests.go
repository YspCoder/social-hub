package bitbucket

import (
	"context"

	"social-hub/pkg/socialhub"
)

// ListPullRequests returns one filtered pull request page for a repository.
func (client *Client) ListPullRequests(
	ctx context.Context,
	workspace string,
	repository string,
	input ListPullRequestsRequest,
	options ...socialhub.CallOption,
) (*Page[PullRequest], error) {
	const operation = "list_pull_requests"
	if !validResourceSelector(workspace) || !validResourceSelector(repository) || !validPullRequestRequest(input) {
		return nil, invalidArgument(operation, "workspace, repository, states, filter, sort, pagination, or next query is invalid")
	}
	query, err := pageQuery(input.Page)
	if err != nil {
		return nil, invalidArgument(operation, "pagination or next query is invalid")
	}
	if input.Page.NextQuery == "" {
		for _, state := range input.States {
			query.Add("state", string(state))
		}
		setQuery(query, "q", input.Query)
		setQuery(query, "sort", input.Sort)
	}
	path := pullRequestsPath(workspace, repository)
	meta, raw, err := client.getJSON(ctx, operation, path, query, nil, options...)
	if err != nil {
		return nil, err
	}
	return decodePage(operation, path, meta, raw, func(value PullRequest) bool {
		return validPullRequest(value) && matchesRepositorySelector(value.Destination.Repository, workspace, repository)
	}, "pull request")
}

// GetPullRequest returns one pull request by repository-local integer ID.
func (client *Client) GetPullRequest(
	ctx context.Context,
	workspace string,
	repository string,
	pullRequestID ID,
	options ...socialhub.CallOption,
) (*PullRequest, ResponseMeta, error) {
	const operation = "get_pull_request"
	if !validResourceSelector(workspace) || !validResourceSelector(repository) || !validID(pullRequestID) {
		return nil, ResponseMeta{}, invalidArgument(operation, "workspace, repository, or pull request ID is invalid")
	}
	var result PullRequest
	path := pullRequestPath(workspace, repository, pullRequestID)
	meta, _, err := client.getJSON(ctx, operation, path, nil, &result, options...)
	if err != nil {
		return nil, meta, err
	}
	if !matchesPullRequest(result, workspace, repository, pullRequestID) {
		return nil, meta, platformContractError(operation, "Bitbucket returned an absent or mismatched pull request, workspace, or repository")
	}
	return &result, meta, nil
}

// ListPullRequestComments returns global, inline, and reply comments for one
// pull request.
func (client *Client) ListPullRequestComments(
	ctx context.Context,
	workspace string,
	repository string,
	pullRequestID ID,
	input ListPullRequestCommentsRequest,
	options ...socialhub.CallOption,
) (*Page[PullRequestComment], error) {
	const operation = "list_pull_request_comments"
	if !validResourceSelector(workspace) || !validResourceSelector(repository) || !validID(pullRequestID) || !validPullRequestCommentRequest(input) {
		return nil, invalidArgument(operation, "workspace, repository, pull request ID, filter, sort, pagination, or next query is invalid")
	}
	query, err := pageQuery(input.Page)
	if err != nil {
		return nil, invalidArgument(operation, "pagination or next query is invalid")
	}
	if input.Page.NextQuery == "" {
		setQuery(query, "q", input.Query)
		setQuery(query, "sort", input.Sort)
	}
	path := pullRequestCommentsPath(workspace, repository, pullRequestID)
	meta, raw, err := client.getJSON(ctx, operation, path, query, nil, options...)
	if err != nil {
		return nil, err
	}
	return decodePage(operation, path, meta, raw, func(value PullRequestComment) bool {
		return matchesPullRequestComment(value, pullRequestID)
	}, "pull request comment")
}

// GetPullRequestComment returns one comment by pull request and comment IDs.
func (client *Client) GetPullRequestComment(
	ctx context.Context,
	workspace string,
	repository string,
	pullRequestID ID,
	commentID ID,
	options ...socialhub.CallOption,
) (*PullRequestComment, ResponseMeta, error) {
	const operation = "get_pull_request_comment"
	if !validResourceSelector(workspace) || !validResourceSelector(repository) || !validID(pullRequestID) || !validID(commentID) {
		return nil, ResponseMeta{}, invalidArgument(operation, "workspace, repository, pull request ID, or comment ID is invalid")
	}
	var result PullRequestComment
	path := pullRequestCommentsPath(workspace, repository, pullRequestID) + "/" + commentID.String()
	meta, _, err := client.getJSON(ctx, operation, path, nil, &result, options...)
	if err != nil {
		return nil, meta, err
	}
	if result.ID != commentID || !matchesPullRequestComment(result, pullRequestID) {
		return nil, meta, platformContractError(operation, "Bitbucket returned an absent or mismatched pull request or comment ID")
	}
	return &result, meta, nil
}

func pullRequestsPath(workspace, repository string) string {
	return repositoryPath(workspace, repository) + "/pullrequests"
}

func pullRequestPath(workspace, repository string, pullRequestID ID) string {
	return pullRequestsPath(workspace, repository) + "/" + pullRequestID.String()
}

func pullRequestCommentsPath(workspace, repository string, pullRequestID ID) string {
	return pullRequestPath(workspace, repository, pullRequestID) + "/comments"
}

var _ ReadWorkflow = (*Client)(nil)

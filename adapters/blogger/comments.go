package blogger

import (
	"context"
	"net/url"

	"social-hub/pkg/socialhub"
)

// GetComment returns one comment by blog, post, and comment provider IDs.
func (client *Client) GetComment(ctx context.Context, input GetCommentRequest, options ...socialhub.CallOption) (Comment, error) {
	const operation = "get_comment"
	if !validResourceID(input.BlogID) || !validResourceID(input.PostID) ||
		!validResourceID(input.CommentID) || !validView(input.View) {
		return Comment{}, invalidArgument(operation, "blog ID, post ID, comment ID, or view is invalid")
	}
	query := make(url.Values)
	setStringQuery(query, "view", string(input.View))
	var comment Comment
	meta, _, err := client.getJSON(ctx, operation,
		"/v3/blogs/"+input.BlogID+"/posts/"+input.PostID+"/comments/"+input.CommentID,
		query, &comment, options...)
	if err != nil {
		return Comment{}, err
	}
	comment.Meta = meta
	if !validCommentResponse(comment, input.BlogID, input.PostID, input.CommentID) {
		return Comment{}, platformContractError(operation, "Blogger returned a comment with an invalid kind, ID, or ownership")
	}
	return comment, nil
}

// ListComments returns one provider-controlled page of comments for a post.
func (client *Client) ListComments(ctx context.Context, input ListCommentsRequest, options ...socialhub.CallOption) (CommentList, error) {
	const operation = "list_comments"
	if !validResourceID(input.BlogID) || !validResourceID(input.PostID) || !validPageToken(input.PageToken) ||
		!validCommentStatus(input.Status) || !validDateRange(input.StartDate, input.EndDate) || !validView(input.View) {
		return CommentList{}, invalidArgument(operation, "blog ID, post ID, pagination, status, dates, or view are invalid")
	}
	query := commentListQuery(input.FetchBodies, input.PageToken, input.Status, input.StartDate, input.EndDate, input.MaxResults)
	setStringQuery(query, "view", string(input.View))
	var comments CommentList
	meta, _, err := client.getJSON(ctx, operation, "/v3/blogs/"+input.BlogID+"/posts/"+input.PostID+"/comments", query, &comments, options...)
	if err != nil {
		return CommentList{}, err
	}
	comments.Meta = meta
	if !validCommentListResponse(comments, input.BlogID, input.PostID) {
		return CommentList{}, platformContractError(operation, "Blogger returned an invalid comment list or pagination token")
	}
	return comments, nil
}

// ListBlogComments returns one provider-controlled page of comments across a blog.
func (client *Client) ListBlogComments(ctx context.Context, input ListBlogCommentsRequest, options ...socialhub.CallOption) (CommentList, error) {
	const operation = "list_blog_comments"
	if !validResourceID(input.BlogID) || !validPageToken(input.PageToken) ||
		!validCommentStatus(input.Status) || !validDateRange(input.StartDate, input.EndDate) {
		return CommentList{}, invalidArgument(operation, "blog ID, pagination, status, or dates are invalid")
	}
	query := commentListQuery(input.FetchBodies, input.PageToken, input.Status, input.StartDate, input.EndDate, input.MaxResults)
	var comments CommentList
	meta, _, err := client.getJSON(ctx, operation, "/v3/blogs/"+input.BlogID+"/comments", query, &comments, options...)
	if err != nil {
		return CommentList{}, err
	}
	comments.Meta = meta
	if !validCommentListResponse(comments, input.BlogID, "") {
		return CommentList{}, platformContractError(operation, "Blogger returned an invalid blog comment list or pagination token")
	}
	return comments, nil
}

func commentListQuery(fetchBodies *bool, pageToken string, status CommentStatus, startDate, endDate string, maxResults uint32) url.Values {
	query := make(url.Values)
	setBoolQuery(query, "fetchBodies", fetchBodies)
	setStringQuery(query, "pageToken", pageToken)
	setStringQuery(query, "status", string(status))
	setStringQuery(query, "startDate", startDate)
	setStringQuery(query, "endDate", endDate)
	setUint32Query(query, "maxResults", maxResults)
	return query
}

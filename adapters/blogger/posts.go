package blogger

import (
	"context"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

// GetPost returns one post by blog and post provider IDs.
func (client *Client) GetPost(ctx context.Context, input GetPostRequest, options ...socialhub.CallOption) (Post, error) {
	const operation = "get_post"
	if !validResourceID(input.BlogID) || !validResourceID(input.PostID) || !validView(input.View) {
		return Post{}, invalidArgument(operation, "blog ID, post ID, or view is invalid")
	}
	query := make(url.Values)
	setBoolQuery(query, "fetchBody", input.FetchBody)
	setBoolQuery(query, "fetchImages", input.FetchImages)
	setUint32Query(query, "maxComments", input.MaxComments)
	setStringQuery(query, "view", string(input.View))
	var post Post
	meta, _, err := client.getJSON(ctx, operation, "/v3/blogs/"+input.BlogID+"/posts/"+input.PostID, query, &post, options...)
	if err != nil {
		return Post{}, err
	}
	post.Meta = meta
	if !validPostResponse(post, input.BlogID, input.PostID) {
		return Post{}, platformContractError(operation, "Blogger returned a post with an invalid kind, ID, or blog ownership")
	}
	return post, nil
}

// GetPostByPath resolves a post path within one blog.
func (client *Client) GetPostByPath(ctx context.Context, input GetPostByPathRequest, options ...socialhub.CallOption) (Post, error) {
	const operation = "get_post_by_path"
	if !validResourceID(input.BlogID) || !validPostPath(input.Path) || !validView(input.View) {
		return Post{}, invalidArgument(operation, "blog ID, post path, or view is invalid")
	}
	query := make(url.Values)
	query.Set("path", input.Path)
	setUint32Query(query, "maxComments", input.MaxComments)
	setStringQuery(query, "view", string(input.View))
	var post Post
	meta, _, err := client.getJSON(ctx, operation, "/v3/blogs/"+input.BlogID+"/posts/bypath", query, &post, options...)
	if err != nil {
		return Post{}, err
	}
	post.Meta = meta
	if !validPostResponse(post, input.BlogID, "") {
		return Post{}, platformContractError(operation, "Blogger returned a post with an invalid kind, ID, or blog ownership")
	}
	return post, nil
}

// ListPosts returns one provider-controlled page of posts without interpreting page tokens.
func (client *Client) ListPosts(ctx context.Context, input ListPostsRequest, options ...socialhub.CallOption) (PostList, error) {
	const operation = "list_posts"
	if !validResourceID(input.BlogID) || !validPageToken(input.PageToken) || !validPostStatus(input.Status) ||
		!validDateRange(input.StartDate, input.EndDate) || !validView(input.View) || !validPostOrder(input.OrderBy) ||
		!validSortOption(input.Sort) || !validLabels(input.Labels) {
		return PostList{}, invalidArgument(operation, "blog ID, pagination, status, dates, view, order, sort, or labels are invalid")
	}
	query := make(url.Values)
	setBoolQuery(query, "fetchBodies", input.FetchBodies)
	setBoolQuery(query, "fetchImages", input.FetchImages)
	setStringQuery(query, "pageToken", input.PageToken)
	setStringQuery(query, "status", string(input.Status))
	setUint32Query(query, "maxResults", input.MaxResults)
	setStringQuery(query, "startDate", input.StartDate)
	setStringQuery(query, "endDate", input.EndDate)
	setStringQuery(query, "view", string(input.View))
	setStringQuery(query, "orderBy", string(input.OrderBy))
	setStringQuery(query, "sortOption", string(input.Sort))
	if len(input.Labels) > 0 {
		query.Set("labels", strings.Join(input.Labels, ","))
	}
	var posts PostList
	meta, _, err := client.getJSON(ctx, operation, "/v3/blogs/"+input.BlogID+"/posts", query, &posts, options...)
	if err != nil {
		return PostList{}, err
	}
	posts.Meta = meta
	if !validPostListResponse(posts, input.BlogID) {
		return PostList{}, platformContractError(operation, "Blogger returned an invalid post list or pagination token")
	}
	return posts, nil
}

// SearchPosts searches one blog using Blogger's provider-native query syntax.
func (client *Client) SearchPosts(ctx context.Context, input SearchPostsRequest, options ...socialhub.CallOption) (PostList, error) {
	const operation = "search_posts"
	if !validResourceID(input.BlogID) || !validOpaque(input.Query, 4096) || !validPostOrder(input.OrderBy) {
		return PostList{}, invalidArgument(operation, "blog ID, search query, or order is invalid")
	}
	query := make(url.Values)
	query.Set("q", input.Query)
	setStringQuery(query, "orderBy", string(input.OrderBy))
	setBoolQuery(query, "fetchBodies", input.FetchBodies)
	var posts PostList
	meta, _, err := client.getJSON(ctx, operation, "/v3/blogs/"+input.BlogID+"/posts/search", query, &posts, options...)
	if err != nil {
		return PostList{}, err
	}
	posts.Meta = meta
	if !validPostListResponse(posts, input.BlogID) {
		return PostList{}, platformContractError(operation, "Blogger returned an invalid post search result")
	}
	return posts, nil
}

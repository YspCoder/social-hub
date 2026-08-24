package blogger

import (
	"context"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

// GetBlog returns one Blogger blog by provider ID.
func (client *Client) GetBlog(ctx context.Context, input GetBlogRequest, options ...socialhub.CallOption) (Blog, error) {
	const operation = "get_blog"
	if !validResourceID(input.BlogID) || !validView(input.View) {
		return Blog{}, invalidArgument(operation, "blog ID or view is invalid")
	}
	query := make(url.Values)
	setUint32Query(query, "maxPosts", input.MaxPosts)
	setStringQuery(query, "view", string(input.View))
	var blog Blog
	meta, _, err := client.getJSON(ctx, operation, "/v3/blogs/"+input.BlogID, query, &blog, options...)
	if err != nil {
		return Blog{}, err
	}
	blog.Meta = meta
	if !validBlogResponse(blog, input.BlogID) {
		return Blog{}, platformContractError(operation, "Blogger returned a blog with an invalid kind, ID, or embedded post ownership")
	}
	return blog, nil
}

// GetBlogByURL resolves a public or authorized Blogger blog URL.
func (client *Client) GetBlogByURL(ctx context.Context, input GetBlogByURLRequest, options ...socialhub.CallOption) (Blog, error) {
	const operation = "get_blog_by_url"
	if !validBlogURL(input.URL) || !validView(input.View) {
		return Blog{}, invalidArgument(operation, "blog URL or view is invalid")
	}
	query := make(url.Values)
	query.Set("url", input.URL)
	setStringQuery(query, "view", string(input.View))
	var blog Blog
	meta, _, err := client.getJSON(ctx, operation, "/v3/blogs/byurl", query, &blog, options...)
	if err != nil {
		return Blog{}, err
	}
	blog.Meta = meta
	if !validBlogResponse(blog, "") {
		return Blog{}, platformContractError(operation, "Blogger returned a blog with an invalid kind, ID, or embedded post ownership")
	}
	return blog, nil
}

// ListBlogsByUser returns blogs visible for a user ID such as "self".
func (client *Client) ListBlogsByUser(ctx context.Context, input ListBlogsByUserRequest, options ...socialhub.CallOption) (BlogList, error) {
	const operation = "list_blogs_by_user"
	if !validResourceID(input.UserID) || !validBlogStatus(input.Status) || !validView(input.Role) || !validView(input.View) {
		return BlogList{}, invalidArgument(operation, "user ID, status, role, or view is invalid")
	}
	query := make(url.Values)
	setStringQuery(query, "status", string(input.Status))
	setBoolQuery(query, "fetchUserInfo", input.FetchUserInfo)
	setStringQuery(query, "role", string(input.Role))
	setStringQuery(query, "view", string(input.View))
	var blogs BlogList
	meta, _, err := client.getJSON(ctx, operation, "/v3/users/"+input.UserID+"/blogs", query, &blogs, options...)
	if err != nil {
		return BlogList{}, err
	}
	blogs.Meta = meta
	if !validBlogListResponse(blogs) {
		return BlogList{}, platformContractError(operation, "Blogger returned an invalid blog list")
	}
	return blogs, nil
}

func setBoolQuery(query url.Values, key string, value *bool) {
	if value != nil {
		query.Set(key, strconv.FormatBool(*value))
	}
}

func setUint32Query(query url.Values, key string, value uint32) {
	if value > 0 {
		query.Set(key, strconv.FormatUint(uint64(value), 10))
	}
}

func setStringQuery(query url.Values, key, value string) {
	if value != "" {
		query.Set(key, value)
	}
}

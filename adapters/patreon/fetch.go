package patreon

import (
	"context"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

const (
	userFields = "full_name,first_name,last_name,vanity,image_url,thumb_url,url"
	postFields = "app_status,content,embed_data,embed_url,is_paid,is_public,published_at,title,url"
)

func (client *Client) GetUser(ctx context.Context, userID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	if userID != "" && userID != "me" && (client.userID == "" || userID != client.userID) {
		return nil, invalidArgument("get_user", "Patreon API v2 identity can only fetch the authorized user")
	}
	api, err := client.requireAPI("get_user")
	if err != nil {
		return nil, err
	}
	if err := client.requireScopes("get_user", "identity"); err != nil {
		return nil, err
	}
	query := url.Values{"fields[user]": {userFields}, "include": {"null"}}
	var response userResponse
	if err := api.JSON(ctx, http.MethodGet, "/identity", query, nil, &response, options...); err != nil {
		return nil, err
	}
	if response.Data.Type != "user" || !validResourceID(response.Data.ID) || client.userID != "" && response.Data.ID != client.userID {
		return nil, platformError("get_user", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return mapUser(client.accountID, response.Data), nil
}

func (client *Client) GetPost(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if !validResourceID(postID) {
		return nil, invalidArgument("get_post", "Post ID is invalid")
	}
	api, err := client.requireAPI("get_post")
	if err != nil {
		return nil, err
	}
	if err := client.requireScopes("get_post", "campaigns.posts"); err != nil {
		return nil, err
	}
	var response postResponse
	if err := api.JSON(ctx, http.MethodGet, resourcePath("posts", postID), postQuery(nil), nil, &response, options...); err != nil {
		return nil, err
	}
	if !client.validPost(response.Data, postID) {
		return nil, platformError("get_post", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return mapPost(client.accountID, response.Data, client.clock.Now()), nil
}

func (client *Client) ListPosts(ctx context.Context, input socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	if input.UserID != "" && (client.userID == "" || input.UserID != client.userID) {
		return socialhub.Page[socialhub.Post]{}, invalidArgument("list_posts", "user filter must match the configured Patreon creator")
	}
	if input.StartTime != nil || input.EndTime != nil {
		return socialhub.Page[socialhub.Post]{}, unsupported("list_posts", "Patreon campaign Post pagination does not expose time filters")
	}
	query, err := pageQuery(input.MaxResults, input.Cursor)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	api, err := client.requireAPI("list_posts")
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	if err := client.requireScopes("list_posts", "campaigns.posts"); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	query = postQuery(query)
	var response postListResponse
	path := resourcePath("campaigns", client.campaignID) + "/posts"
	if err := api.JSON(ctx, http.MethodGet, path, query, nil, &response, options...); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	items := make([]socialhub.Post, 0, len(response.Data))
	observedAt := client.clock.Now()
	for _, post := range response.Data {
		if client.validPost(post, "") {
			items = append(items, *mapPost(client.accountID, post, observedAt))
		}
	}
	page := socialhub.Page[socialhub.Post]{Items: items}
	if err := setNextCursor(&page, response.Meta.Pagination.Cursors.Next); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	return page, nil
}

func (client *Client) ListComments(_ context.Context, input socialhub.ListCommentsRequest, _ ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	if !validResourceID(input.PostID) || input.MaxResults < 0 {
		return socialhub.Page[socialhub.Comment]{}, invalidArgument("list_comments", "Post ID and non-negative max_results are required")
	}
	return socialhub.Page[socialhub.Comment]{}, unsupported("list_comments", "Patreon API v2 does not expose Post comments")
}

func (client *Client) validPost(post postResource, expectedID string) bool {
	if post.Type != "post" || !validResourceID(post.ID) || expectedID != "" && post.ID != expectedID {
		return false
	}
	return post.Relationships.Campaign.Data == nil || post.Relationships.Campaign.Data.ID == client.campaignID
}

func postQuery(query url.Values) url.Values {
	if query == nil {
		query = url.Values{}
	}
	query.Set("fields[post]", postFields)
	query.Set("include", "null")
	return query
}

func setNextCursor[T any](page *socialhub.Page[T], next string) error {
	if next == "" {
		return nil
	}
	if !validCursor(next) {
		return platformError("pagination", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	page.NextCursor = stringPointer(next)
	page.HasMore = true
	return nil
}

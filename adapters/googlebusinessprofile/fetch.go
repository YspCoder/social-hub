package googlebusinessprofile

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

const defaultLocalPostPageSize = 20
const maxLocalPostPageSize = 100

func (client *Client) GetUser(ctx context.Context, userID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	if userID != "" && userID != "me" && userID != client.locationID && userID != client.locationResource() {
		return nil, invalidArgument("get_user", "user filter must identify the configured business location")
	}
	if err := client.requireScope("get_user"); err != nil {
		return nil, err
	}
	var response Location
	if err := client.api.JSON(ctx, http.MethodGet, "/"+client.locationResource(), nil, nil, &response, options...); err != nil {
		return nil, err
	}
	if err := client.validateLocation("get_user", &response); err != nil {
		return nil, err
	}
	return mapLocation(client.accountID, &response), nil
}

func (client *Client) GetPost(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	post, err := client.getLocalPost(ctx, postID, options...)
	if err != nil {
		return nil, err
	}
	return mapPost(client.accountID, client.locationID, post), nil
}

func (client *Client) ListPosts(ctx context.Context, input socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	if input.UserID != "" && input.UserID != client.locationID && input.UserID != client.locationResource() {
		return socialhub.Page[socialhub.Post]{}, invalidArgument("list_posts", "user filter must identify the configured business location")
	}
	if input.StartTime != nil || input.EndTime != nil {
		return socialhub.Page[socialhub.Post]{}, unsupported("list_posts", "Google Business Profile Local Posts list does not support time filters")
	}
	pageSize, err := localPostPageSize(input.MaxResults)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	if input.Cursor != "" && !validOpaque(input.Cursor, 4096) {
		return socialhub.Page[socialhub.Post]{}, invalidArgument("list_posts", "page token is invalid")
	}
	if err := client.requireScope("list_posts"); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	query := url.Values{"pageSize": {strconv.Itoa(pageSize)}}
	if input.Cursor != "" {
		query.Set("pageToken", input.Cursor)
	}
	var response localPostListResponse
	path := "/" + client.locationResource() + "/localPosts"
	if err := client.api.JSON(ctx, http.MethodGet, path, query, nil, &response, options...); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	items := make([]socialhub.Post, 0, len(response.LocalPosts))
	for index := range response.LocalPosts {
		post := &response.LocalPosts[index]
		if err := client.validateLocalPost("list_posts", post, ""); err != nil {
			return socialhub.Page[socialhub.Post]{}, err
		}
		items = append(items, *mapPost(client.accountID, client.locationID, post))
	}
	page := socialhub.Page[socialhub.Post]{Items: items}
	if response.NextPageToken != "" {
		if !validOpaque(response.NextPageToken, 4096) {
			return socialhub.Page[socialhub.Post]{}, platformError("list_posts", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
		page.NextCursor, page.HasMore = stringPointer(response.NextPageToken), true
	}
	return page, nil
}

func (client *Client) ListComments(context.Context, socialhub.ListCommentsRequest, ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	return socialhub.Page[socialhub.Comment]{}, unsupported("list_comments", "Google Business Profile reviews are location resources; use ReviewWorkflow instead of treating them as post comments")
}

func (client *Client) getLocalPost(ctx context.Context, postID string, options ...socialhub.CallOption) (*LocalPost, error) {
	if !validResourceSegment(postID) {
		return nil, invalidArgument("get_post", "local post ID must be a bounded resource ID segment")
	}
	if err := client.requireScope("get_post"); err != nil {
		return nil, err
	}
	var response LocalPost
	if err := client.api.JSON(ctx, http.MethodGet, "/"+client.localPostResource(postID), nil, nil, &response, options...); err != nil {
		return nil, err
	}
	if err := client.validateLocalPost("get_post", &response, postID); err != nil {
		return nil, err
	}
	return &response, nil
}

func localPostPageSize(value int) (int, error) {
	if value < 0 || value > maxLocalPostPageSize {
		return 0, invalidArgument("pagination", "max_results must be between 0 and 100")
	}
	if value == 0 {
		value = defaultLocalPostPageSize
	}
	return value, nil
}

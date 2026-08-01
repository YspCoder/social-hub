package mastodon

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

func (c *Client) GetUser(ctx context.Context, userID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	path := "/api/v1/accounts/" + url.PathEscape(userID)
	if userID == "" {
		if err := c.requireAnyScope("get_user", "profile", "read:accounts"); err != nil {
			return nil, err
		}
		path = "/api/v1/accounts/verify_credentials"
	}
	var response mastodonAccount
	if err := c.transport.JSON(ctx, http.MethodGet, path, nil, nil, &response, options...); err != nil {
		return nil, err
	}
	if response.ID == "" || (userID != "" && response.ID != userID) {
		return nil, platformError("get_user", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return mapAccount(c.accountID, response), nil
}

func (c *Client) GetPost(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if strings.TrimSpace(postID) == "" {
		return nil, invalidArgument("get_post", "status ID is required")
	}
	var response mastodonStatus
	if err := c.transport.JSON(ctx, http.MethodGet, "/api/v1/statuses/"+url.PathEscape(postID), nil, nil, &response, options...); err != nil {
		return nil, err
	}
	if response.ID != postID {
		return nil, platformError("get_post", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return mapStatus(c.accountID, response, c.clock.Now()), nil
}

func (c *Client) ListPosts(ctx context.Context, input socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	if input.StartTime != nil || input.EndTime != nil {
		return socialhub.Page[socialhub.Post]{}, unsupported("list_posts", "Mastodon account-status pagination does not accept exact time ranges")
	}
	if err := c.requireScopes("list_posts", "read:statuses"); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	userID := input.UserID
	if userID == "" {
		userID = c.userID
	}
	if userID == "" {
		user, err := c.GetUser(ctx, "", options...)
		if err != nil {
			return socialhub.Page[socialhub.Post]{}, err
		}
		userID = user.ID
	}
	query := url.Values{}
	setPageQuery(query, input.Cursor, input.MaxResults)
	var response []mastodonStatus
	metadata, err := c.getJSON(ctx, "/api/v1/accounts/"+url.PathEscape(userID)+"/statuses", query, &response, options...)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	return mapStatusPage(c.accountID, response, c.clock.Now(), metadata.Header), nil
}

func (c *Client) ListComments(ctx context.Context, input socialhub.ListCommentsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	if strings.TrimSpace(input.PostID) == "" {
		return socialhub.Page[socialhub.Comment]{}, invalidArgument("list_comments", "status ID is required")
	}
	if input.Cursor != "" {
		return socialhub.Page[socialhub.Comment]{}, unsupported("list_comments", "Mastodon status context does not use cursor pagination")
	}
	var response mastodonContext
	if err := c.transport.JSON(ctx, http.MethodGet, "/api/v1/statuses/"+url.PathEscape(input.PostID)+"/context", nil, nil, &response, options...); err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	limit := len(response.Descendants)
	if input.MaxResults > 0 && input.MaxResults < limit {
		limit = input.MaxResults
	}
	items := make([]socialhub.Comment, 0, limit)
	for _, status := range response.Descendants[:limit] {
		items = append(items, mapComment(c.accountID, input.PostID, status))
	}
	return socialhub.Page[socialhub.Comment]{Items: items}, nil
}

func (c *Client) Home(ctx context.Context, input TimelineRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	if input.MaxResults < 0 {
		return socialhub.Page[socialhub.Post]{}, invalidArgument("home_timeline", "max results must not be negative")
	}
	if err := c.requireScopes("home_timeline", "read:statuses"); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	query := url.Values{}
	setPageQuery(query, input.Cursor, input.MaxResults)
	var response []mastodonStatus
	metadata, err := c.getJSON(ctx, "/api/v1/timelines/home", query, &response, options...)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	return mapStatusPage(c.accountID, response, c.clock.Now(), metadata.Header), nil
}

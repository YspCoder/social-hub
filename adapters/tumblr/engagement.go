package tumblr

import (
	"context"
	"net/http"
	"strings"

	"social-hub/pkg/socialhub"
)

func (c *Client) Like(ctx context.Context, postID, reblogKey string, options ...socialhub.CallOption) error {
	return c.postEngagement(ctx, "like", postID, reblogKey, options...)
}

func (c *Client) Unlike(ctx context.Context, postID, reblogKey string, options ...socialhub.CallOption) error {
	return c.postEngagement(ctx, "unlike", postID, reblogKey, options...)
}

func (c *Client) Follow(ctx context.Context, blogURL string, options ...socialhub.CallOption) error {
	return c.postFollow(ctx, "follow", blogURL, options...)
}

func (c *Client) Unfollow(ctx context.Context, blogURL string, options ...socialhub.CallOption) error {
	return c.postFollow(ctx, "unfollow", blogURL, options...)
}

func (c *Client) postEngagement(ctx context.Context, action, postID, reblogKey string, options ...socialhub.CallOption) error {
	if !validPostID(postID) || strings.TrimSpace(reblogKey) == "" {
		return invalidArgument(action, "numeric post ID and reblog key are required")
	}
	user, err := c.requireUser(action)
	if err != nil {
		return err
	}
	if err := c.requireScopes(action, "write"); err != nil {
		return err
	}
	body := map[string]string{"id": postID, "reblog_key": strings.TrimSpace(reblogKey)}
	return c.request(ctx, user, http.MethodPost, "/user/"+action, nil, body, nil, options...)
}

func (c *Client) postFollow(ctx context.Context, action, blogURL string, options ...socialhub.CallOption) error {
	blogURL = strings.TrimSpace(blogURL)
	if !validWebURL(blogURL) {
		return invalidArgument(action, "absolute HTTP(S) blog URL is required")
	}
	user, err := c.requireUser(action)
	if err != nil {
		return err
	}
	if err := c.requireScopes(action, "write"); err != nil {
		return err
	}
	return c.request(ctx, user, http.MethodPost, "/user/"+action, nil, map[string]string{"url": blogURL}, nil, options...)
}

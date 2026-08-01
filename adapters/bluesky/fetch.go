package bluesky

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

func (c *Client) GetUser(ctx context.Context, userID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	actor := userID
	if actor == "" {
		actor = c.repo
	}
	if strings.TrimSpace(actor) == "" {
		return nil, invalidArgument("get_user", "actor handle or DID is required")
	}
	var response bskyActor
	if err := c.get(ctx, "app.bsky.actor.getProfile", url.Values{"actor": {actor}}, &response, options...); err != nil {
		return nil, err
	}
	if response.DID == "" || response.Handle == "" {
		return nil, platformError("get_user", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return mapActor(c.accountID, response), nil
}

func (c *Client) GetPost(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	view, err := c.getPostView(ctx, postID, options...)
	if err != nil {
		return nil, err
	}
	return mapPost(c.accountID, *view, c.clock.Now())
}

func (c *Client) getPostView(ctx context.Context, postID string, options ...socialhub.CallOption) (*bskyPostView, error) {
	parsed, err := parseRecordURI(postID)
	if err != nil || parsed.Collection != collectionPost {
		return nil, invalidArgument("get_post", "post ID must be an app.bsky.feed.post AT URI")
	}
	query := url.Values{}
	query.Add("uris", postID)
	var response bskyPostsResponse
	if err := c.get(ctx, "app.bsky.feed.getPosts", query, &response, options...); err != nil {
		return nil, err
	}
	if len(response.Posts) != 1 || response.Posts[0].URI != postID || response.Posts[0].CID == "" {
		return nil, platformError("get_post", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	return &response.Posts[0], nil
}

func (c *Client) ListPosts(ctx context.Context, input socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	if input.StartTime != nil || input.EndTime != nil {
		return socialhub.Page[socialhub.Post]{}, unsupported("list_posts", "Bluesky author-feed pagination does not accept exact time ranges")
	}
	actor := input.UserID
	if actor == "" {
		actor = c.repo
	}
	query := url.Values{"actor": {actor}}
	if err := setPageQuery(query, input.Cursor, input.MaxResults); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	var response bskyFeedResponse
	if err := c.get(ctx, "app.bsky.feed.getAuthorFeed", query, &response, options...); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	return mapFeedPage(c.accountID, response, c.clock.Now())
}

func (c *Client) ListComments(ctx context.Context, input socialhub.ListCommentsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	parsed, err := parseRecordURI(input.PostID)
	if err != nil || parsed.Collection != collectionPost {
		return socialhub.Page[socialhub.Comment]{}, invalidArgument("list_comments", "post ID must be an app.bsky.feed.post AT URI")
	}
	if input.Cursor != "" {
		return socialhub.Page[socialhub.Comment]{}, unsupported("list_comments", "Bluesky thread views do not use cursor pagination")
	}
	if input.MaxResults < 0 {
		return socialhub.Page[socialhub.Comment]{}, invalidArgument("list_comments", "max results must not be negative")
	}
	query := url.Values{"uri": {input.PostID}, "parentHeight": {"0"}}
	if input.MaxResults > 0 {
		depth := input.MaxResults
		if depth > 1000 {
			depth = 1000
		}
		query.Set("depth", strconv.Itoa(depth))
	}
	var response bskyThreadResponse
	if err := c.get(ctx, "app.bsky.feed.getPostThread", query, &response, options...); err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	if response.Thread.Post.URI != input.PostID || response.Thread.NotFound || response.Thread.Blocked {
		return socialhub.Page[socialhub.Comment]{}, platformError("list_comments", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	items := make([]socialhub.Comment, 0)
	var walk func([]bskyThreadNode) error
	walk = func(nodes []bskyThreadNode) error {
		for _, node := range nodes {
			if input.MaxResults > 0 && len(items) >= input.MaxResults {
				return nil
			}
			if node.Post.URI == "" || node.NotFound || node.Blocked {
				continue
			}
			comment, err := mapComment(c.accountID, input.PostID, node.Post)
			if err != nil {
				return err
			}
			items = append(items, comment)
			if err := walk(node.Replies); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(response.Thread.Replies); err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	return socialhub.Page[socialhub.Comment]{Items: items}, nil
}

func (c *Client) Home(ctx context.Context, input TimelineRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	query := url.Values{}
	if err := setPageQuery(query, input.Cursor, input.MaxResults); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	if input.Algorithm != "" {
		query.Set("algorithm", input.Algorithm)
	}
	var response bskyFeedResponse
	if err := c.get(ctx, "app.bsky.feed.getTimeline", query, &response, options...); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	return mapFeedPage(c.accountID, response, c.clock.Now())
}

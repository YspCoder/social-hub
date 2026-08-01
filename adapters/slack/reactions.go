package slack

import (
	"context"
	"strings"

	"social-hub/pkg/socialhub"
)

func (c *Client) React(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	return c.setReaction(ctx, input, false, options...)
}

func (c *Client) RemoveReaction(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	return c.setReaction(ctx, input, true, options...)
}

func (c *Client) setReaction(ctx context.Context, input socialhub.ReactionRequest, remove bool, options ...socialhub.CallOption) error {
	if err := c.requireScopes("reactions", "reactions:write"); err != nil {
		return err
	}
	if input.Kind != socialhub.ReactionLike {
		return unsupported("reactions", "the common Slack reactor maps only like to the thumbsup emoji")
	}
	if strings.TrimSpace(input.ActorID) != "" && c.actorID != "" && strings.TrimSpace(input.ActorID) != c.actorID {
		return invalidArgument("reactions", "actor_id must match the configured Slack actor")
	}
	channelID, timestamp, err := parseCompositeID(input.TargetID, "reactions")
	if err != nil {
		return err
	}
	method := "reactions.add"
	if remove {
		method = "reactions.remove"
	}
	return c.call(ctx, method, struct {
		Name      string `json:"name"`
		Channel   string `json:"channel"`
		Timestamp string `json:"timestamp"`
	}{Name: "thumbsup", Channel: channelID, Timestamp: timestamp}, nil, options...)
}

func (c *Client) Comment(ctx context.Context, input socialhub.CreateCommentRequest, options ...socialhub.CallOption) (*socialhub.Comment, error) {
	channelID, rootTS, err := parseCompositeID(input.PostID, "chat.postMessage")
	if err != nil {
		return nil, err
	}
	if input.ParentID != nil {
		return nil, unsupported("chat.postMessage", "Slack threads are flat and do not support nested comment parents")
	}
	post, err := c.PostMessage(ctx, PostMessageRequest{ChannelID: channelID, Text: input.Text, ThreadPostID: compositeID(channelID, rootTS)}, options...)
	if err != nil {
		return nil, err
	}
	comment := socialhub.Comment{
		Platform: "slack", AccountID: c.accountID, ID: post.ID, PostID: input.PostID,
		AuthorID: post.AuthorID, Text: input.Text, CreatedAt: post.CreatedAt, Extensions: post.Extensions,
	}
	return &comment, nil
}

func (c *Client) DeleteComment(ctx context.Context, commentID string, options ...socialhub.CallOption) error {
	return c.DeletePost(ctx, commentID, options...)
}

package discord

import (
	"context"
	"net/http"
	"strings"

	"social-hub/pkg/socialhub"
)

const likeEmoji = "👍"

func (c *Client) React(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	return c.setReaction(ctx, input, false, options...)
}

func (c *Client) RemoveReaction(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	return c.setReaction(ctx, input, true, options...)
}

func (c *Client) setReaction(ctx context.Context, input socialhub.ReactionRequest, remove bool, options ...socialhub.CallOption) error {
	if input.ActorID != "" && input.ActorID != "@me" {
		return unsupported("react", "a bot can only create or remove its own reaction")
	}
	if input.Kind != socialhub.ReactionLike {
		return unsupported("react", "the common Discord reactor maps only like to a thumbs-up emoji")
	}
	channelID, messageID, err := parseMessageID("react", input.TargetID, c.defaultChannelID)
	if err != nil {
		return err
	}
	method := http.MethodPut
	if remove {
		method = http.MethodDelete
	}
	path := channelMessagePath(channelID, messageID) + "/reactions/" + likeEmoji + "/@me"
	return c.request(ctx, method, path, nil, nil, nil, options...)
}

func (c *Client) Comment(ctx context.Context, input socialhub.CreateCommentRequest, options ...socialhub.CallOption) (*socialhub.Comment, error) {
	if strings.TrimSpace(input.Text) == "" {
		return nil, invalidArgument("comment", "comment text is required")
	}
	rootChannelID, _, err := parseMessageID("comment", input.PostID, c.defaultChannelID)
	if err != nil {
		return nil, err
	}
	targetID := input.PostID
	var parentID *string
	if input.ParentID != nil {
		targetID = *input.ParentID
		parentID = stringPointer(targetID)
	}
	targetChannelID, _, err := parseMessageID("comment", targetID, c.defaultChannelID)
	if err != nil {
		return nil, err
	}
	if targetChannelID != rootChannelID {
		return nil, invalidArgument("comment", "post and parent comment must belong to the same channel")
	}
	message, err := c.sendText(ctx, rootChannelID, input.Text, &targetID, options...)
	if err != nil {
		return nil, err
	}
	return c.mapComment(input.PostID, *message, parentID), nil
}

func (c *Client) DeleteComment(ctx context.Context, commentID string, options ...socialhub.CallOption) error {
	channelID, messageID, err := parseMessageID("delete_comment", commentID, c.defaultChannelID)
	if err != nil {
		return err
	}
	return c.deleteMessage(ctx, channelID, messageID, options...)
}

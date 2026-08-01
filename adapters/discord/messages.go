package discord

import (
	"context"
	"net/http"
	"strings"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

func (c *Client) SendMessage(ctx context.Context, input socialhub.SendMessageRequest, options ...socialhub.CallOption) (*socialhub.Message, error) {
	if !validSnowflake(input.ConversationID) || input.Text == nil || strings.TrimSpace(*input.Text) == "" {
		return nil, invalidArgument("send_message", "conversation ID and non-empty text are required")
	}
	if len(input.RecipientIDs) > 0 {
		return nil, unsupported("send_message", "Discord messages address a channel, not a separate recipient list")
	}
	if len(input.MediaIDs) > 0 {
		return nil, unsupported("send_message", "Discord attachments must be uploaded with the message request")
	}
	message, err := c.sendText(ctx, input.ConversationID, *input.Text, input.ReplyToID, options...)
	if err != nil {
		return nil, err
	}
	return c.mapMessage(*message, socialhub.DirectionOutbound), nil
}

func (c *Client) GetMessage(ctx context.Context, id string, options ...socialhub.CallOption) (*socialhub.Message, error) {
	channelID, messageID, err := parseMessageID("get_message", id, c.defaultChannelID)
	if err != nil {
		return nil, err
	}
	message, err := c.getMessage(ctx, channelID, messageID, options...)
	if err != nil {
		return nil, err
	}
	return c.mapMessage(*message, socialhub.DirectionInbound), nil
}

func (c *Client) Publish(ctx context.Context, input socialhub.CreatePostRequest, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if c.defaultChannelID == "" {
		return nil, invalidArgument("publish", "account.settings.default_channel_id is required")
	}
	if input.Text == nil || strings.TrimSpace(*input.Text) == "" {
		return nil, invalidArgument("publish", "non-empty text is required")
	}
	if len(input.MediaIDs) > 0 {
		return nil, unsupported("publish", "Discord attachments must be uploaded with the message request")
	}
	if input.QuotePostID != nil || input.Visibility != nil {
		return nil, unsupported("publish", "quote and visibility overrides are not Discord message parameters")
	}
	message, err := c.sendText(ctx, c.defaultChannelID, *input.Text, input.ReplyToID, options...)
	if err != nil {
		return nil, err
	}
	return c.mapPost(*message), nil
}

func (c *Client) PublishStatus(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.PublishStatus, error) {
	post, err := c.GetPost(ctx, postID, options...)
	if err != nil {
		return nil, err
	}
	return post.Status, nil
}

func (c *Client) DeletePost(ctx context.Context, postID string, options ...socialhub.CallOption) error {
	channelID, messageID, err := parseMessageID("delete_post", postID, c.defaultChannelID)
	if err != nil {
		return err
	}
	return c.deleteMessage(ctx, channelID, messageID, options...)
}

func (c *Client) sendText(ctx context.Context, channelID, text string, replyToID *string, options ...socialhub.CallOption) (*discordMessage, error) {
	if utf8.RuneCountInString(text) > 2000 {
		return nil, invalidArgument("send_message", "text must not exceed 2000 characters")
	}
	input := messageCreate{Content: text, AllowedMentions: allowedMentions{Parse: []string{}, RepliedUser: false}}
	if replyToID != nil {
		replyChannelID, replyMessageID, err := parseMessageID("send_message", *replyToID, channelID)
		if err != nil {
			return nil, err
		}
		if replyChannelID != channelID {
			return nil, invalidArgument("send_message", "reply target must belong to the destination channel")
		}
		input.MessageReference = &messageReference{MessageID: replyMessageID, ChannelID: channelID}
	}
	var response discordMessage
	if err := c.request(ctx, http.MethodPost, "/channels/"+channelID+"/messages", nil, input, &response, options...); err != nil {
		return nil, err
	}
	if !validSnowflake(response.ID) || response.ChannelID == "" {
		return nil, wrapError("send_message", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &response, nil
}

func (c *Client) getMessage(ctx context.Context, channelID, messageID string, options ...socialhub.CallOption) (*discordMessage, error) {
	var response discordMessage
	if err := c.get(ctx, channelMessagePath(channelID, messageID), nil, &response, options...); err != nil {
		return nil, err
	}
	if response.ID == "" || response.ChannelID == "" {
		return nil, wrapError("get_message", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &response, nil
}

func (c *Client) deleteMessage(ctx context.Context, channelID, messageID string, options ...socialhub.CallOption) error {
	return c.request(ctx, http.MethodDelete, channelMessagePath(channelID, messageID), nil, nil, nil, options...)
}

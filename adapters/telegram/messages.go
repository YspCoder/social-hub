package telegram

import (
	"context"
	"strconv"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"social-hub/pkg/socialhub"
)

func (c *Client) SendMessage(ctx context.Context, input socialhub.SendMessageRequest, options ...socialhub.CallOption) (*socialhub.Message, error) {
	if input.ConversationID == "" || input.Text == nil || strings.TrimSpace(*input.Text) == "" {
		return nil, invalidArgument("send_message", "conversation ID and non-empty text are required")
	}
	if len(input.RecipientIDs) > 0 {
		return nil, unsupported("send_message", "Telegram addresses messages to a chat, not a separate recipient list")
	}
	if len(input.MediaIDs) > 0 {
		return nil, unsupported("send_message", "use BotWorkflow.SendMedia so the Telegram media type is explicit")
	}
	message, err := c.sendText(ctx, input.ConversationID, *input.Text, input.ReplyToID, options...)
	if err != nil {
		return nil, err
	}
	return mapMessage(c.accountID, message, socialhub.DirectionOutbound), nil
}

func (c *Client) GetMessage(context.Context, string, ...socialhub.CallOption) (*socialhub.Message, error) {
	return nil, unsupported("get_message", "Bot API does not provide arbitrary message lookup")
}

func (c *Client) Publish(ctx context.Context, input socialhub.CreatePostRequest, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if c.defaultChatID == "" {
		return nil, invalidArgument("publish", "account.settings.default_chat_id is required")
	}
	if input.Text == nil || strings.TrimSpace(*input.Text) == "" {
		return nil, invalidArgument("publish", "non-empty text is required")
	}
	if len(input.MediaIDs) > 0 {
		return nil, unsupported("publish", "use BotWorkflow.SendMedia so the Telegram media type is explicit")
	}
	if input.QuotePostID != nil || input.Visibility != nil {
		return nil, unsupported("publish", "quote and visibility overrides are not sendMessage parameters")
	}
	message, err := c.sendText(ctx, c.defaultChatID, *input.Text, input.ReplyToID, options...)
	if err != nil {
		return nil, err
	}
	return mapPost(c.accountID, message), nil
}

func (c *Client) PublishStatus(context.Context, string, ...socialhub.CallOption) (*socialhub.PublishStatus, error) {
	return nil, unsupported("publish_status", "Bot API does not provide message lookup for publication polling")
}

func (c *Client) DeletePost(ctx context.Context, postID string, options ...socialhub.CallOption) error {
	if c.defaultChatID == "" {
		return invalidArgument("delete_post", "account.settings.default_chat_id is required")
	}
	messageID, err := parseMessageID("delete_post", postID)
	if err != nil {
		return err
	}
	callCtx, cancel, err := resolveCallContext(ctx, options...)
	if err != nil {
		return err
	}
	defer cancel()
	ok, err := c.bot.DeleteMessage(callCtx, &tgbot.DeleteMessageParams{ChatID: c.defaultChatID, MessageID: messageID})
	if err != nil {
		return mapError("delete_post", err)
	}
	if !ok {
		return wrapError("delete_post", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return nil
}

func (c *Client) sendText(ctx context.Context, chatID, text string, replyToID *string, options ...socialhub.CallOption) (*models.Message, error) {
	if len([]rune(text)) > 4096 {
		return nil, invalidArgument("send_message", "text must not exceed 4096 characters")
	}
	params := &tgbot.SendMessageParams{ChatID: chatID, Text: text}
	if replyToID != nil {
		messageID, err := parseMessageID("send_message", *replyToID)
		if err != nil {
			return nil, err
		}
		params.ReplyParameters = &models.ReplyParameters{MessageID: messageID}
	}
	callCtx, cancel, err := resolveCallContext(ctx, options...)
	if err != nil {
		return nil, err
	}
	defer cancel()
	message, err := c.bot.SendMessage(callCtx, params)
	if err != nil {
		return nil, mapError("send_message", err)
	}
	if message == nil {
		return nil, wrapError("send_message", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return message, nil
}

func parseMessageID(operation, value string) (int, error) {
	messageID, err := strconv.Atoi(value)
	if err != nil || messageID <= 0 {
		return 0, invalidArgument(operation, "message ID must be a positive integer")
	}
	return messageID, nil
}

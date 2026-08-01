package twitch

import (
	"context"
	"encoding/json"
	"strings"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

func (c *Client) SendMessage(ctx context.Context, input socialhub.SendMessageRequest, options ...socialhub.CallOption) (*socialhub.Message, error) {
	if strings.TrimSpace(input.ConversationID) == "" || input.Text == nil || strings.TrimSpace(*input.Text) == "" {
		return nil, invalidArgument("send_message", "broadcaster conversation ID and non-empty text are required")
	}
	if c.userID == "" {
		return nil, invalidArgument("send_message", "account.settings.user_id is required")
	}
	if utf8.RuneCountInString(*input.Text) > 500 {
		return nil, invalidArgument("send_message", "chat text must not exceed 500 characters")
	}
	if len(input.RecipientIDs) > 0 {
		return nil, unsupported("send_message", "Twitch chat addresses a broadcaster channel, not a recipient list")
	}
	if len(input.MediaIDs) > 0 {
		return nil, unsupported("send_message", "Twitch chat messages do not accept generic media IDs")
	}
	if err := c.requireScope("send_message", "user:write:chat"); err != nil {
		return nil, err
	}
	body := struct {
		BroadcasterID string `json:"broadcaster_id"`
		SenderID      string `json:"sender_id"`
		Message       string `json:"message"`
		ReplyParentID string `json:"reply_parent_message_id,omitempty"`
	}{BroadcasterID: input.ConversationID, SenderID: c.userID, Message: *input.Text}
	if input.ReplyToID != nil {
		if strings.TrimSpace(*input.ReplyToID) == "" {
			return nil, invalidArgument("send_message", "reply parent message ID must not be empty")
		}
		body.ReplyParentID = *input.ReplyToID
	}
	var response struct {
		Data []struct {
			MessageID  string `json:"message_id"`
			Sent       bool   `json:"is_sent"`
			DropReason *struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"drop_reason"`
		} `json:"data"`
	}
	if err := c.request(ctx, "POST", "/chat/messages", nil, body, &response, options...); err != nil {
		return nil, err
	}
	if len(response.Data) != 1 || response.Data[0].MessageID == "" {
		return nil, platformError("send_message", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if !response.Data[0].Sent {
		err := &socialhub.Error{
			Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent, Platform: "twitch", Product: productName,
			Op: "send_message",
		}
		if response.Data[0].DropReason != nil {
			err.PlatformCode = boundedMessage(response.Data[0].DropReason.Code, 64)
			err.PlatformMessage = boundedMessage(response.Data[0].DropReason.Message, 512)
		}
		return nil, err
	}
	now := c.clock.Now()
	extension, _ := json.Marshal(response.Data[0])
	return &socialhub.Message{
		Platform: "twitch", AccountID: c.accountID, ID: response.Data[0].MessageID,
		ConversationID: input.ConversationID, SenderID: stringPointer(c.userID), Text: stringPointer(*input.Text),
		ReplyToID: input.ReplyToID, SentAt: &now, Direction: socialhub.DirectionOutbound,
		Extensions: map[string]json.RawMessage{"twitch.chat_message": extension},
	}, nil
}

func (c *Client) GetMessage(context.Context, string, ...socialhub.CallOption) (*socialhub.Message, error) {
	return nil, unsupported("get_message", "Helix does not provide arbitrary chat-message lookup")
}

package lark

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

const maxMessageContentBytes = 150 << 10

func (c *Client) Send(ctx context.Context, input SendRequest, options ...socialhub.CallOption) (*socialhub.Message, error) {
	if err := c.requireMessageWrite("im.message.create"); err != nil {
		return nil, err
	}
	if !validReceiveIDType(input.ReceiveIDType) || !validOpaqueID(input.ReceiveID, 512) {
		return nil, invalidArgument("im.message.create", "receive_id_type and a bounded receive_id are required")
	}
	if err := validateMessageContent(input.MessageType, input.Content); err != nil {
		return nil, err
	}
	uuid, err := requestUUID(options...)
	if err != nil {
		return nil, err
	}
	var response struct {
		Data wireMessage `json:"data"`
	}
	if err := c.call(ctx, "im.message.create", http.MethodPost, "/open-apis/im/v1/messages", url.Values{
		"receive_id_type": {string(input.ReceiveIDType)},
	}, struct {
		ReceiveID   string `json:"receive_id"`
		MessageType string `json:"msg_type"`
		Content     string `json:"content"`
		UUID        string `json:"uuid,omitempty"`
	}{ReceiveID: input.ReceiveID, MessageType: input.MessageType, Content: string(input.Content), UUID: uuid}, &response, true, options...); err != nil {
		return nil, err
	}
	if !validMessageID(response.Data.MessageID) || !validChatID(response.Data.ChatID) {
		return nil, platformError("im.message.create", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return mapMessage(c.accountID, c.actorID, response.Data), nil
}

func (c *Client) Reply(ctx context.Context, input ReplyRequest, options ...socialhub.CallOption) (*socialhub.Message, error) {
	if err := c.requireMessageWrite("im.message.reply"); err != nil {
		return nil, err
	}
	if !validMessageID(input.MessageID) {
		return nil, invalidArgument("im.message.reply", "message_id must be a Lark message ID")
	}
	if err := validateMessageContent(input.MessageType, input.Content); err != nil {
		return nil, err
	}
	uuid, err := requestUUID(options...)
	if err != nil {
		return nil, err
	}
	var response struct {
		Data wireMessage `json:"data"`
	}
	path := "/open-apis/im/v1/messages/" + url.PathEscape(input.MessageID) + "/reply"
	if err := c.call(ctx, "im.message.reply", http.MethodPost, path, nil, struct {
		MessageType   string `json:"msg_type"`
		Content       string `json:"content"`
		ReplyInThread bool   `json:"reply_in_thread,omitempty"`
		UUID          string `json:"uuid,omitempty"`
	}{MessageType: input.MessageType, Content: string(input.Content), ReplyInThread: input.ReplyInThread, UUID: uuid}, &response, true, options...); err != nil {
		return nil, err
	}
	if !validMessageID(response.Data.MessageID) || !validChatID(response.Data.ChatID) {
		return nil, platformError("im.message.reply", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return mapMessage(c.accountID, c.actorID, response.Data), nil
}

func (c *Client) Update(ctx context.Context, input UpdateRequest, options ...socialhub.CallOption) (*socialhub.Message, error) {
	if c.tokenKind != TokenTenant {
		return nil, unsupported("im.message.update", "message editing requires tenant_access_token")
	}
	if err := c.requireMessageWrite("im.message.update"); err != nil {
		return nil, err
	}
	if !validMessageID(input.MessageID) || (input.MessageType != "text" && input.MessageType != "post") {
		return nil, invalidArgument("im.message.update", "message_id and text or post message_type are required")
	}
	if err := validateMessageContent(input.MessageType, input.Content); err != nil {
		return nil, err
	}
	var response struct {
		Data wireMessage `json:"data"`
	}
	path := "/open-apis/im/v1/messages/" + url.PathEscape(input.MessageID)
	if err := c.call(ctx, "im.message.update", http.MethodPut, path, nil, struct {
		MessageType string `json:"msg_type"`
		Content     string `json:"content"`
	}{MessageType: input.MessageType, Content: string(input.Content)}, &response, false, options...); err != nil {
		return nil, err
	}
	if response.Data.MessageID != input.MessageID || !validChatID(response.Data.ChatID) {
		return nil, platformError("im.message.update", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return mapMessage(c.accountID, c.actorID, response.Data), nil
}

func (c *Client) Delete(ctx context.Context, messageID string, options ...socialhub.CallOption) error {
	if err := c.requireMessageWrite("im.message.delete"); err != nil {
		return err
	}
	if !validMessageID(messageID) {
		return invalidArgument("im.message.delete", "message_id must be a Lark message ID")
	}
	return c.call(ctx, "im.message.delete", http.MethodDelete, "/open-apis/im/v1/messages/"+url.PathEscape(messageID), nil, nil, nil, false, options...)
}

func (c *Client) SendMessage(ctx context.Context, input socialhub.SendMessageRequest, options ...socialhub.CallOption) (*socialhub.Message, error) {
	messageType, content, err := c.commonMessageContent(input.Text, input.MediaIDs)
	if err != nil {
		return nil, err
	}
	if input.ReplyToID != nil {
		return c.Reply(ctx, ReplyRequest{MessageID: strings.TrimSpace(*input.ReplyToID), MessageType: messageType, Content: content}, options...)
	}
	receiveID, receiveType, err := c.commonReceiver(input)
	if err != nil {
		return nil, err
	}
	return c.Send(ctx, SendRequest{ReceiveIDType: receiveType, ReceiveID: receiveID, MessageType: messageType, Content: content}, options...)
}

func (c *Client) commonReceiver(input socialhub.SendMessageRequest) (string, ReceiveIDType, error) {
	conversationID := strings.TrimSpace(input.ConversationID)
	if conversationID != "" {
		if len(input.RecipientIDs) != 0 || !validChatID(conversationID) {
			return "", "", invalidArgument("messages.send", "conversation_id must be a Lark chat ID and cannot be combined with recipient_ids")
		}
		return conversationID, ReceiveChatID, nil
	}
	if len(input.RecipientIDs) != 1 || !validOpaqueID(input.RecipientIDs[0], 512) {
		return "", "", invalidArgument("messages.send", "exactly one recipient_id is required when conversation_id is empty")
	}
	return input.RecipientIDs[0], ReceiveIDType(c.userIDType), nil
}

func (c *Client) commonMessageContent(text *string, mediaIDs []string) (string, json.RawMessage, error) {
	if text != nil && strings.TrimSpace(*text) != "" {
		if len(mediaIDs) != 0 {
			return "", nil, unsupported("messages.send", "Lark messages have one msg_type; send text and media as separate messages or use a typed post/card")
		}
		encoded, _ := json.Marshal(map[string]string{"text": *text})
		return "text", encoded, nil
	}
	if len(mediaIDs) != 1 || !validOpaqueID(mediaIDs[0], 512) {
		return "", nil, invalidArgument("messages.send", "a non-empty text or exactly one media_id is required")
	}
	mediaID := mediaIDs[0]
	c.uploadMu.Lock()
	media, found := c.media[mediaID]
	c.uploadMu.Unlock()
	messageType := "file"
	if found {
		messageType = messageTypeForMedia(media.Type)
	} else if strings.HasPrefix(mediaID, "img_") {
		messageType = "image"
	}
	key := "file_key"
	if messageType == "image" {
		key = "image_key"
	}
	encoded, _ := json.Marshal(map[string]string{key: mediaID})
	return messageType, encoded, nil
}

func (c *Client) Publish(ctx context.Context, input socialhub.CreatePostRequest, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if c.defaultChatID == "" {
		return nil, unsupported("publish", "configure default_chat_id for common Lark publishing")
	}
	if input.QuotePostID != nil || input.Visibility != nil {
		return nil, unsupported("publish", "Lark chat messages do not expose common quote or per-message visibility controls")
	}
	request := socialhub.SendMessageRequest{ConversationID: c.defaultChatID, Text: input.Text, MediaIDs: input.MediaIDs, ReplyToID: input.ReplyToID}
	message, err := c.SendMessage(ctx, request, options...)
	if err != nil {
		return nil, err
	}
	var wire wireMessage
	if extension := message.Extensions["lark.message"]; len(extension) > 0 {
		_ = json.Unmarshal(extension, &wire)
	}
	return mapPost(c.accountID, wire, c.clock.Now()), nil
}

func (c *Client) PublishStatus(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.PublishStatus, error) {
	post, err := c.GetPost(ctx, postID, options...)
	if err != nil {
		return nil, err
	}
	if post.Status == nil {
		return nil, platformError("publish_status", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return post.Status, nil
}

func (c *Client) DeletePost(ctx context.Context, postID string, options ...socialhub.CallOption) error {
	return c.Delete(ctx, postID, options...)
}

func requestUUID(options ...socialhub.CallOption) (string, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return "", err
	}
	uuid := strings.TrimSpace(resolved.IdempotencyKey)
	if uuid != "" && (!validText(uuid, 50) || uuid != resolved.IdempotencyKey) {
		return "", invalidArgument("idempotency", "Lark uuid must contain 1 to 50 characters without control characters")
	}
	return uuid, nil
}

func validateMessageContent(messageType string, content json.RawMessage) error {
	allowed := map[string]bool{
		"text": true, "post": true, "interactive": true, "image": true, "file": true,
		"audio": true, "media": true, "sticker": true, "share_chat": true, "share_user": true,
	}
	if !allowed[messageType] || len(content) == 0 || len(content) > maxMessageContentBytes || !json.Valid(content) || content[0] != '{' {
		return invalidArgument("message_content", "message_type and a JSON object content of at most 150 KB are required")
	}
	return nil
}

func validReceiveIDType(value ReceiveIDType) bool {
	switch value {
	case ReceiveOpenID, ReceiveUnionID, ReceiveUserID, ReceiveEmail, ReceiveChatID:
		return true
	default:
		return false
	}
}

func validMessageID(value string) bool {
	return strings.HasPrefix(strings.TrimSpace(value), "om_") && validOpaqueID(value, 512)
}

func validText(value string, maximum int) bool {
	value = strings.TrimSpace(value)
	if value == "" || len([]rune(value)) > maximum {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}

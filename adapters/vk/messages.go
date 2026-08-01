package vk

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"math"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

func (c *Client) SendMessage(ctx context.Context, input socialhub.SendMessageRequest, options ...socialhub.CallOption) (*socialhub.Message, error) {
	if c.tokenKind == TokenService {
		return nil, tokenPermission("messages.send", "service tokens cannot send VK messages")
	}
	peerID, err := strconv.ParseInt(strings.TrimSpace(input.ConversationID), 10, 64)
	if err != nil || peerID == 0 {
		return nil, invalidArgument("messages.send", "conversation_id must be a non-zero VK peer ID")
	}
	if len(input.RecipientIDs) > 0 {
		return nil, unsupported("messages.send", "common Messenger sends to one peer; use separate calls for multiple peers")
	}
	text := ""
	if input.Text != nil {
		text = *input.Text
	}
	if strings.TrimSpace(text) == "" && len(input.MediaIDs) == 0 {
		return nil, invalidArgument("messages.send", "message text or an attachment is required")
	}
	if len(input.MediaIDs) > 10 {
		return nil, invalidArgument("messages.send", "VK messages accept at most ten common attachments")
	}
	for _, attachment := range input.MediaIDs {
		if !validAttachment(attachment) {
			return nil, invalidArgument("messages.send", "media_ids must use VK attachment identifiers")
		}
	}
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return nil, err
	}
	randomID, err := vkRandomID(resolved.IdempotencyKey)
	if err != nil {
		return nil, err
	}
	values := url.Values{
		"peer_id": {strconv.FormatInt(peerID, 10)}, "random_id": {strconv.FormatInt(randomID, 10)},
	}
	if text != "" {
		values.Set("message", text)
	}
	if len(input.MediaIDs) > 0 {
		values.Set("attachment", strings.Join(input.MediaIDs, ","))
	}
	if input.ReplyToID != nil {
		replyID, err := strconv.ParseInt(strings.TrimSpace(*input.ReplyToID), 10, 64)
		if err != nil || replyID <= 0 {
			return nil, invalidArgument("messages.send", "reply_to_id must be a positive VK message ID")
		}
		values.Set("reply_to", strconv.FormatInt(replyID, 10))
	}
	if c.tokenKind == TokenCommunity {
		values.Set("group_id", strconv.FormatInt(c.groupID, 10))
	}
	var raw json.RawMessage
	if err := c.method(ctx, "messages.send", values, &raw, options...); err != nil {
		return nil, err
	}
	messageID, err := decodeMessageID(raw)
	if err != nil {
		return nil, err
	}
	message := &socialhub.Message{
		Platform: "vk", AccountID: c.accountID, ID: strconv.FormatInt(messageID, 10), ConversationID: strconv.FormatInt(peerID, 10),
		RecipientIDs: []string{strconv.FormatInt(peerID, 10)}, Text: stringPointer(text), SentAt: timePointer(c.clock.Now()), Direction: socialhub.DirectionOutbound,
	}
	if input.ReplyToID != nil {
		reply := strings.TrimSpace(*input.ReplyToID)
		message.ReplyToID = &reply
	}
	return message, nil
}

func (c *Client) GetMessage(ctx context.Context, messageID string, options ...socialhub.CallOption) (*socialhub.Message, error) {
	if c.tokenKind == TokenService {
		return nil, tokenPermission("messages.getById", "service tokens cannot read VK messages")
	}
	id, err := strconv.ParseInt(strings.TrimSpace(messageID), 10, 64)
	if err != nil || id <= 0 {
		return nil, invalidArgument("messages.getById", "message ID must be a positive integer")
	}
	values := url.Values{"message_ids": {strconv.FormatInt(id, 10)}, "extended": {"0"}}
	if c.tokenKind == TokenCommunity {
		values.Set("group_id", strconv.FormatInt(c.groupID, 10))
	}
	var response struct {
		Count int           `json:"count"`
		Items []wireMessage `json:"items"`
	}
	if err := c.method(ctx, "messages.getById", values, &response, options...); err != nil {
		return nil, err
	}
	if len(response.Items) != 1 || response.Items[0].ID != id {
		return nil, platformError("messages.getById", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	return mapMessage(c.accountID, response.Items[0]), nil
}

func vkRandomID(key string) (int64, error) {
	if strings.TrimSpace(key) != "" {
		value, err := strconv.ParseInt(key, 10, 32)
		if err != nil || value <= 0 {
			return 0, invalidArgument("messages.send", "idempotency key must be a positive 32-bit decimal VK random_id")
		}
		return value, nil
	}
	var buffer [4]byte
	if _, err := rand.Read(buffer[:]); err != nil {
		return 0, platformError("messages.send", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	return int64(binary.BigEndian.Uint32(buffer[:])%math.MaxInt32) + 1, nil
}

func decodeMessageID(raw json.RawMessage) (int64, error) {
	var numeric int64
	if err := json.Unmarshal(raw, &numeric); err == nil && numeric > 0 {
		return numeric, nil
	}
	var object struct {
		MessageID int64 `json:"message_id"`
	}
	if err := json.Unmarshal(raw, &object); err == nil && object.MessageID > 0 {
		return object.MessageID, nil
	}
	return 0, platformError("messages.send", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
}

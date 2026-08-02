package lemmy

import (
	"context"
	"net/http"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) SendPrivateMessage(ctx context.Context, recipientID, content string, options ...socialhub.CallOption) (*PrivateMessage, error) {
	if !validID(recipientID) || !validMessageContent(content) {
		return nil, invalidArgument("send_private_message", "recipient ID and non-empty content within the Lemmy limit are required")
	}
	payload := struct {
		Content     string `json:"content"`
		RecipientID int64  `json:"recipient_id"`
	}{Content: content, RecipientID: mustID(recipientID)}
	var response privateMessageResponse
	if err := client.requestJSON(ctx, http.MethodPost, "/private_message", nil, payload, &response, options...); err != nil {
		return nil, err
	}
	if !validPrivateMessageView(response.PrivateMessageView) || response.PrivateMessageView.PrivateMessage.RecipientID != mustID(recipientID) {
		return nil, platformError("send_private_message", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	mapped := client.mapPrivateMessage(response.PrivateMessageView)
	mapped.Common.Direction = socialhub.DirectionOutbound
	return &mapped, nil
}

func (client *Client) ListPrivateMessages(ctx context.Context, creatorID, cursor string, maximum int, options ...socialhub.CallOption) (socialhub.Page[PrivateMessage], error) {
	query, page, pageSize, err := pageQuery(cursor, maximum)
	if err != nil {
		return socialhub.Page[PrivateMessage]{}, err
	}
	if creatorID != "" {
		if !validID(creatorID) {
			return socialhub.Page[PrivateMessage]{}, invalidArgument("list_private_messages", "creator ID must be a positive integer")
		}
		query.Set("creator_id", creatorID)
	}
	var response privateMessagesResponse
	if err := client.requestJSON(ctx, http.MethodGet, "/private_message/list", query, nil, &response, options...); err != nil {
		return socialhub.Page[PrivateMessage]{}, err
	}
	result := socialhub.Page[PrivateMessage]{Items: make([]PrivateMessage, 0, len(response.PrivateMessages))}
	for _, view := range response.PrivateMessages {
		if !validPrivateMessageView(view) {
			return socialhub.Page[PrivateMessage]{}, platformError("list_private_messages", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
		result.Items = append(result.Items, client.mapPrivateMessage(view))
	}
	result.NextCursor, result.PrevCursor, result.HasMore = pageCursors(len(result.Items), page, pageSize)
	return result, nil
}

func (client *Client) EditPrivateMessage(ctx context.Context, messageID, content string, options ...socialhub.CallOption) (*PrivateMessage, error) {
	if !validID(messageID) || !validMessageContent(content) {
		return nil, invalidArgument("edit_private_message", "message ID and non-empty content within the Lemmy limit are required")
	}
	payload := struct {
		PrivateMessageID int64  `json:"private_message_id"`
		Content          string `json:"content"`
	}{PrivateMessageID: mustID(messageID), Content: content}
	return client.mutatePrivateMessage(ctx, http.MethodPut, "/private_message", "edit_private_message", messageID, payload, options...)
}

func (client *Client) DeletePrivateMessage(ctx context.Context, messageID string, options ...socialhub.CallOption) error {
	if !validID(messageID) {
		return invalidArgument("delete_private_message", "message ID must be a positive integer")
	}
	payload := struct {
		PrivateMessageID int64 `json:"private_message_id"`
		Deleted          bool  `json:"deleted"`
	}{PrivateMessageID: mustID(messageID), Deleted: true}
	message, err := client.mutatePrivateMessage(ctx, http.MethodPost, "/private_message/delete", "delete_private_message", messageID, payload, options...)
	if err != nil {
		return err
	}
	if !message.Deleted {
		return platformError("delete_private_message", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return nil
}

func (client *Client) MarkPrivateMessageRead(ctx context.Context, messageID string, read bool, options ...socialhub.CallOption) (*PrivateMessage, error) {
	if !validID(messageID) {
		return nil, invalidArgument("mark_private_message_read", "message ID must be a positive integer")
	}
	payload := struct {
		PrivateMessageID int64 `json:"private_message_id"`
		Read             bool  `json:"read"`
	}{PrivateMessageID: mustID(messageID), Read: read}
	message, err := client.mutatePrivateMessage(ctx, http.MethodPost, "/private_message/mark_as_read", "mark_private_message_read", messageID, payload, options...)
	if err != nil {
		return nil, err
	}
	if message.Read != read {
		return nil, platformError("mark_private_message_read", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return message, nil
}

func (client *Client) mutatePrivateMessage(ctx context.Context, method, path, operation, messageID string, payload any, options ...socialhub.CallOption) (*PrivateMessage, error) {
	var response privateMessageResponse
	if err := client.requestJSON(ctx, method, path, nil, payload, &response, options...); err != nil {
		return nil, err
	}
	if !validPrivateMessageView(response.PrivateMessageView) || response.PrivateMessageView.PrivateMessage.ID != mustID(messageID) {
		return nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	mapped := client.mapPrivateMessage(response.PrivateMessageView)
	return &mapped, nil
}

func validMessageContent(content string) bool {
	return strings.TrimSpace(content) != "" && validBody(content, 10000)
}

func validPrivateMessageView(view wirePrivateMessageView) bool {
	message := view.PrivateMessage
	return message.ID > 0 && message.CreatorID > 0 && message.RecipientID > 0 &&
		view.Creator.ID == message.CreatorID && view.Recipient.ID == message.RecipientID
}

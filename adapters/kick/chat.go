package kick

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

func (client *Client) SendChat(ctx context.Context, input SendChatRequest, options ...socialhub.CallOption) (*ChatResult, error) {
	if err := client.requireUserToken("send_chat"); err != nil {
		return nil, err
	}
	if err := client.requireScope("send_chat", "chat:write"); err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.Content) == "" || utf8.RuneCountInString(input.Content) > 500 {
		return nil, invalidArgument("send_chat", "content must contain 1-500 characters")
	}
	if input.Type != "user" && input.Type != "bot" {
		return nil, invalidArgument("send_chat", "type must be user or bot")
	}
	broadcasterID := input.BroadcasterUserID
	if broadcasterID == "" {
		broadcasterID = client.broadcasterUserID
	}
	if input.Type == "user" && !validPositiveID(broadcasterID) {
		return nil, invalidArgument("send_chat", "broadcaster_user_id is required for user messages")
	}
	if input.Type == "bot" {
		broadcasterID = ""
	}
	if input.ReplyToMessageID != "" && !validPathID(input.ReplyToMessageID, 512) {
		return nil, invalidArgument("send_chat", "reply_to_message_id is invalid")
	}
	body := struct {
		BroadcasterUserID int64  `json:"broadcaster_user_id,omitempty"`
		Content           string `json:"content"`
		ReplyToMessageID  string `json:"reply_to_message_id,omitempty"`
		Type              string `json:"type"`
	}{
		BroadcasterUserID: positiveInt64(broadcasterID), Content: input.Content,
		ReplyToMessageID: input.ReplyToMessageID, Type: input.Type,
	}
	var response responseEnvelope[ChatResult]
	if err := client.request(ctx, http.MethodPost, "/public/v1/chat", nil, body, &response, options...); err != nil {
		return nil, err
	}
	if !response.Data.IsSent || !validPathID(response.Data.MessageID, 512) {
		return nil, platformError("send_chat", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &response.Data, nil
}

func (client *Client) DeleteChat(ctx context.Context, messageID string, options ...socialhub.CallOption) error {
	if err := client.requireUserToken("delete_chat"); err != nil {
		return err
	}
	if err := client.requireScope("delete_chat", "moderation:chat_message:manage"); err != nil {
		return err
	}
	if !validPathID(messageID, 512) {
		return invalidArgument("delete_chat", "message ID is invalid")
	}
	return client.request(ctx, http.MethodDelete, "/public/v1/chat/"+url.PathEscape(messageID), nil, nil, nil, options...)
}

var _ ChatWorkflow = (*Client)(nil)

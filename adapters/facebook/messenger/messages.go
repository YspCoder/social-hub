package messenger

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

func (c *Client) SendMessage(ctx context.Context, input socialhub.SendMessageRequest, options ...socialhub.CallOption) (*socialhub.Message, error) {
	recipientID := strings.TrimSpace(input.ConversationID)
	if !validNumericID(recipientID) || input.Text == nil || strings.TrimSpace(*input.Text) == "" {
		return nil, invalidArgument("send_message", "PSID conversation ID and non-empty text are required")
	}
	if len(input.RecipientIDs) > 1 || (len(input.RecipientIDs) == 1 && strings.TrimSpace(input.RecipientIDs[0]) != recipientID) {
		return nil, invalidArgument("send_message", "recipient IDs must be empty or contain only the conversation PSID")
	}
	if len(input.MediaIDs) > 0 {
		return nil, unsupported("send_message", "use MessageWorkflow.SendAttachment so the Messenger attachment type is explicit")
	}
	replyTo := ""
	if input.ReplyToID != nil {
		replyTo = strings.TrimSpace(*input.ReplyToID)
	}
	return c.SendText(ctx, TextMessageRequest{
		RecipientID: recipientID, Text: *input.Text, Type: MessagingResponse, ReplyToID: replyTo,
	}, options...)
}

func (c *Client) GetMessage(context.Context, string, ...socialhub.CallOption) (*socialhub.Message, error) {
	return nil, unsupported("get_message", "Messenger Platform does not provide arbitrary message lookup")
}

// SendText sends one RESPONSE or UPDATE text message inside Meta's standard
// messaging window.
func (c *Client) SendText(ctx context.Context, input TextMessageRequest, options ...socialhub.CallOption) (*socialhub.Message, error) {
	recipientID := strings.TrimSpace(input.RecipientID)
	text := strings.TrimSpace(input.Text)
	messagingType := defaultMessagingType(input.Type)
	if !validNumericID(recipientID) || text == "" || utf8.RuneCountInString(input.Text) > 2000 || !validMessagingType(messagingType) {
		return nil, invalidArgument("send_text", "PSID, RESPONSE or UPDATE type, and text containing at most 2,000 characters are required")
	}
	replyTo := strings.TrimSpace(input.ReplyToID)
	if input.ReplyToID != "" && !validOpaqueID(replyTo, 512) {
		return nil, invalidArgument("send_text", "reply message ID is invalid")
	}
	message := map[string]any{"text": input.Text}
	if replyTo != "" {
		message["reply_to"] = map[string]string{"mid": replyTo}
	}
	body := map[string]any{
		"recipient":      map[string]string{"id": recipientID},
		"messaging_type": messagingType,
		"message":        message,
	}
	return c.send(ctx, "send_text", recipientID, &input.Text, stringPointer(replyTo), nil, body, options...)
}

// SendAttachment sends one image, audio, video, or file by public HTTPS URL or
// reusable attachment ID.
func (c *Client) SendAttachment(ctx context.Context, input AttachmentMessageRequest, options ...socialhub.CallOption) (*socialhub.Message, error) {
	recipientID := strings.TrimSpace(input.RecipientID)
	messagingType := defaultMessagingType(input.Type)
	attachmentID := strings.TrimSpace(input.Reference.ID)
	attachmentURL := strings.TrimSpace(input.Reference.URL)
	if !validNumericID(recipientID) || !validMessagingType(messagingType) || !validAttachmentType(input.Attachment) {
		return nil, invalidArgument("send_attachment", "PSID, RESPONSE or UPDATE type, and supported attachment type are required")
	}
	if (attachmentID == "") == (attachmentURL == "") {
		return nil, invalidArgument("send_attachment", "exactly one attachment ID or public HTTPS URL is required")
	}
	if attachmentID != "" && !validOpaqueID(attachmentID, 512) {
		return nil, invalidArgument("send_attachment", "attachment ID is invalid")
	}
	if attachmentURL != "" && !validRemoteURL(attachmentURL) {
		return nil, invalidArgument("send_attachment", "attachment URL must be public HTTPS without credentials or fragments")
	}
	if input.Reference.Reusable && attachmentURL == "" {
		return nil, invalidArgument("send_attachment", "reusable is valid only when sending an attachment URL")
	}
	replyTo := strings.TrimSpace(input.ReplyToID)
	if input.ReplyToID != "" && !validOpaqueID(replyTo, 512) {
		return nil, invalidArgument("send_attachment", "reply message ID is invalid")
	}
	payload := map[string]any{}
	if attachmentID != "" {
		payload["attachment_id"] = attachmentID
	} else {
		payload["url"] = attachmentURL
		if input.Reference.Reusable {
			payload["is_reusable"] = true
		}
	}
	message := map[string]any{"attachment": map[string]any{
		"type": input.Attachment, "payload": payload,
	}}
	if replyTo != "" {
		message["reply_to"] = map[string]string{"mid": replyTo}
	}
	body := map[string]any{
		"recipient":      map[string]string{"id": recipientID},
		"messaging_type": messagingType,
		"message":        message,
	}
	media := &socialhub.Media{ID: attachmentID, URL: attachmentURL, Type: normalizedAttachmentType(input.Attachment), State: socialhub.MediaStateReady}
	return c.send(ctx, "send_attachment", recipientID, nil, stringPointer(replyTo), media, body, options...)
}

func (c *Client) send(ctx context.Context, operation, recipientID string, text, replyTo *string, media *socialhub.Media, body any, options ...socialhub.CallOption) (*socialhub.Message, error) {
	var result SendResult
	path := "/" + url.PathEscape(c.pageID) + "/messages"
	if err := c.api.JSON(ctx, http.MethodPost, path, nil, body, &result, options...); err != nil {
		return nil, err
	}
	if !validOpaqueID(result.MessageID, 512) || result.RecipientID != recipientID {
		return nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	now := c.clock.Now()
	extension, _ := json.Marshal(result)
	message := &socialhub.Message{
		Platform: "facebook", AccountID: c.accountID, ID: result.MessageID, ConversationID: recipientID,
		RecipientIDs: []string{recipientID}, Text: text, ReplyToID: replyTo, SentAt: &now, Direction: socialhub.DirectionOutbound,
		Extensions: map[string]json.RawMessage{"facebook.messenger_send_result": extension},
	}
	if media != nil {
		message.Media = []socialhub.Media{*media}
	}
	return message, nil
}

func defaultMessagingType(value MessagingType) MessagingType {
	if value == "" {
		return MessagingResponse
	}
	return value
}

func validMessagingType(value MessagingType) bool {
	return value == MessagingResponse || value == MessagingUpdate
}

func validAttachmentType(value AttachmentType) bool {
	switch value {
	case AttachmentImage, AttachmentAudio, AttachmentVideo, AttachmentFile:
		return true
	default:
		return false
	}
}

func validRemoteURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil && parsed.Fragment == ""
}

func normalizedAttachmentType(value AttachmentType) socialhub.MediaType {
	switch value {
	case AttachmentImage:
		return socialhub.MediaTypeImage
	case AttachmentAudio:
		return socialhub.MediaTypeAudio
	case AttachmentVideo:
		return socialhub.MediaTypeVideo
	default:
		return socialhub.MediaTypeDocument
	}
}

func stringPointer(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	copy := value
	return &copy
}

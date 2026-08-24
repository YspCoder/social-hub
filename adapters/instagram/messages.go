package instagram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"social-hub/pkg/socialhub"
)

const messagingScope = "instagram_business_manage_messages"

func (c *Client) SendMessage(ctx context.Context, input socialhub.SendMessageRequest, options ...socialhub.CallOption) (*socialhub.Message, error) {
	recipientID := strings.TrimSpace(input.ConversationID)
	if !validMessagingID(recipientID) || input.Text == nil || strings.TrimSpace(*input.Text) == "" {
		return nil, invalidArgument("send_message", "IGSID conversation ID and non-empty text are required")
	}
	if len(input.RecipientIDs) > 1 || (len(input.RecipientIDs) == 1 && strings.TrimSpace(input.RecipientIDs[0]) != recipientID) {
		return nil, invalidArgument("send_message", "recipient IDs must be empty or contain only the conversation IGSID")
	}
	if len(input.MediaIDs) > 0 {
		return nil, unsupported("send_message", "use MessagingWorkflow.SendMedia or SharePublishedMedia so the attachment type is explicit")
	}
	if input.ReplyToID != nil {
		return nil, unsupported("send_message", "Instagram Login Send API does not document reply-to for this workflow")
	}
	return c.SendText(ctx, TextMessageRequest{RecipientID: recipientID, Text: *input.Text}, options...)
}

func (c *Client) GetMessage(ctx context.Context, messageID string, options ...socialhub.CallOption) (*socialhub.Message, error) {
	messageID = strings.TrimSpace(messageID)
	if !validMessageID(messageID) {
		return nil, invalidArgument("get_message", "message ID is required and must be valid")
	}
	if err := c.requireScope("get_message", messagingScope); err != nil {
		return nil, err
	}
	query := url.Values{"fields": {"id,created_time,from,to,message,reply_to"}}
	var response messageDetail
	if err := c.transport.JSON(ctx, http.MethodGet, "/"+url.PathEscape(messageID), query, nil, &response, options...); err != nil {
		return nil, err
	}
	if response.ID != messageID || !validMessagingID(response.From.ID) || len(response.To.Data) != 1 {
		return nil, wrapError("get_message", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return c.mapMessageDetail(response)
}

// SendText sends one text message inside Instagram's permitted messaging
// window. The conversation must have been initiated by the Instagram user.
func (c *Client) SendText(ctx context.Context, input TextMessageRequest, options ...socialhub.CallOption) (*socialhub.Message, error) {
	recipientID := strings.TrimSpace(input.RecipientID)
	if !validMessagingID(recipientID) || strings.TrimSpace(input.Text) == "" {
		return nil, invalidArgument("send_text", "IGSID and non-empty text are required")
	}
	body := map[string]any{
		"recipient": map[string]string{"id": recipientID},
		"message":   map[string]string{"text": input.Text},
	}
	return c.sendMessaging(ctx, "send_text", recipientID, &input.Text, nil, body, options...)
}

// SendMedia sends one image, audio file, or video by public HTTPS URL.
func (c *Client) SendMedia(ctx context.Context, input MediaMessageRequest, options ...socialhub.CallOption) (*socialhub.Message, error) {
	recipientID := strings.TrimSpace(input.RecipientID)
	mediaURL := strings.TrimSpace(input.URL)
	if !validMessagingID(recipientID) || !validMessageMediaType(input.Type) || !validMessageMediaURL(mediaURL) {
		return nil, invalidArgument("send_media", "IGSID, image/audio/video type, and a public HTTPS URL are required")
	}
	body := map[string]any{
		"recipient": map[string]string{"id": recipientID},
		"message": map[string]any{"attachment": map[string]any{
			"type": input.Type, "payload": map[string]string{"url": mediaURL},
		}},
	}
	media := &socialhub.Media{URL: mediaURL, Type: normalizedMessageMediaType(input.Type), State: socialhub.MediaStateReady}
	return c.sendMessaging(ctx, "send_media", recipientID, nil, media, body, options...)
}

// SharePublishedMedia sends media owned by the configured professional
// account as a MEDIA_SHARE attachment.
func (c *Client) SharePublishedMedia(ctx context.Context, input PublishedMediaMessageRequest, options ...socialhub.CallOption) (*socialhub.Message, error) {
	recipientID := strings.TrimSpace(input.RecipientID)
	mediaID := strings.TrimSpace(input.MediaID)
	if !validMessagingID(recipientID) || !validMessagingID(mediaID) {
		return nil, invalidArgument("share_published_media", "IGSID and owned Instagram media ID are required")
	}
	body := map[string]any{
		"recipient": map[string]string{"id": recipientID},
		"message": map[string]any{"attachment": map[string]any{
			"type": "MEDIA_SHARE", "payload": map[string]string{"id": mediaID},
		}},
	}
	media := &socialhub.Media{ID: mediaID, Type: socialhub.MediaTypeDocument, State: socialhub.MediaStateReady}
	return c.sendMessaging(ctx, "share_published_media", recipientID, nil, media, body, options...)
}

// SendReaction adds or removes one reaction from a message.
func (c *Client) SendReaction(ctx context.Context, input MessageReactionRequest, options ...socialhub.CallOption) (*ReactionResult, error) {
	recipientID := strings.TrimSpace(input.RecipientID)
	messageID := strings.TrimSpace(input.MessageID)
	reaction := strings.TrimSpace(input.Reaction)
	if !validMessagingID(recipientID) || !validMessageID(messageID) ||
		(input.Action != MessageReactionAdd && input.Action != MessageReactionRemove) || reaction == "" {
		return nil, invalidArgument("send_reaction", "IGSID, message ID, react or unreact action, and reaction are required")
	}
	if err := c.requireScope("send_reaction", messagingScope); err != nil {
		return nil, err
	}
	body := map[string]any{
		"recipient":     map[string]string{"id": recipientID},
		"sender_action": input.Action,
		"payload":       map[string]string{"message_id": messageID, "reaction": reaction},
	}
	var result ReactionResult
	if err := c.transport.JSON(ctx, http.MethodPost, c.messagesPath(), nil, body, &result, options...); err != nil {
		return nil, err
	}
	if result.RecipientID != recipientID {
		return nil, wrapError("send_reaction", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &result, nil
}

// GetMessagingUserProfile reads the consented profile for one IGSID.
func (c *Client) GetMessagingUserProfile(ctx context.Context, igsid string, options ...socialhub.CallOption) (*MessagingUserProfile, error) {
	igsid = strings.TrimSpace(igsid)
	if !validMessagingID(igsid) || igsid == c.userID {
		return nil, invalidArgument("get_messaging_user_profile", "a customer IGSID is required")
	}
	if err := c.requireScope("get_messaging_user_profile", "instagram_business_basic"); err != nil {
		return nil, err
	}
	if err := c.requireScope("get_messaging_user_profile", messagingScope); err != nil {
		return nil, err
	}
	query := url.Values{"fields": {"id,name,username,profile_pic,follower_count,is_verified_user,is_user_follow_business,is_business_follow_user"}}
	var profile MessagingUserProfile
	if err := c.transport.JSON(ctx, http.MethodGet, "/"+url.PathEscape(igsid), query, nil, &profile, options...); err != nil {
		return nil, err
	}
	if profile.ID != igsid {
		return nil, wrapError("get_messaging_user_profile", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &profile, nil
}

func (c *Client) sendMessaging(ctx context.Context, operation, recipientID string, text *string, media *socialhub.Media, body any, options ...socialhub.CallOption) (*socialhub.Message, error) {
	if err := c.requireScope(operation, messagingScope); err != nil {
		return nil, err
	}
	var result SendResult
	if err := c.transport.JSON(ctx, http.MethodPost, c.messagesPath(), nil, body, &result, options...); err != nil {
		return nil, err
	}
	if result.RecipientID != recipientID || !validMessageID(result.MessageID) {
		return nil, wrapError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	now := c.clock.Now()
	extension, _ := json.Marshal(result)
	message := &socialhub.Message{
		Platform: "instagram", AccountID: c.accountID, ID: result.MessageID, ConversationID: recipientID,
		SenderID: stringPointer(c.userID), RecipientIDs: []string{recipientID}, Text: text, SentAt: &now,
		Direction: socialhub.DirectionOutbound, Extensions: map[string]json.RawMessage{"instagram.messaging_send_result": extension},
	}
	if media != nil {
		message.Media = []socialhub.Media{*media}
	}
	return message, nil
}

func (c *Client) messagesPath() string {
	return "/" + url.PathEscape(c.userID) + "/messages"
}

func (c *Client) mapMessageDetail(input messageDetail) (*socialhub.Message, error) {
	sentAt, err := parseGraphTime(input.CreatedTime)
	if err != nil {
		return nil, wrapError("get_message", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	recipientIDs := make([]string, 0, len(input.To.Data))
	configuredRecipients := 0
	conversationID := ""
	for _, recipient := range input.To.Data {
		if !validMessagingID(recipient.ID) {
			return nil, wrapError("get_message", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
		recipientIDs = append(recipientIDs, recipient.ID)
		if recipient.ID == c.userID {
			configuredRecipients++
		} else if conversationID == "" {
			conversationID = recipient.ID
		}
	}
	direction := socialhub.DirectionInbound
	if input.From.ID == c.userID {
		direction = socialhub.DirectionOutbound
		if configuredRecipients != 0 || conversationID == "" {
			return nil, wrapError("get_message", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
	} else {
		conversationID = input.From.ID
		if configuredRecipients != 1 {
			return nil, wrapError("get_message", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
	}
	extension, _ := json.Marshal(input)
	message := &socialhub.Message{
		Platform: "instagram", AccountID: c.accountID, ID: input.ID, ConversationID: conversationID,
		SenderID: stringPointer(input.From.ID), RecipientIDs: recipientIDs, Text: stringPointer(input.Message),
		SentAt: &sentAt, Direction: direction, Extensions: map[string]json.RawMessage{"instagram.message": extension},
	}
	if input.ReplyTo != nil {
		message.ReplyToID = stringPointer(firstNonEmpty(input.ReplyTo.ID, input.ReplyTo.MessageID))
	}
	return message, nil
}

func parseGraphTime(value string) (time.Time, error) {
	var lastError error
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05-0700"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed.UTC(), nil
		}
		lastError = err
	}
	return time.Time{}, lastError
}

func validMessagingID(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func validMessageID(value string) bool {
	if value == "" || len(value) > 512 || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validMessageMediaType(value MessageMediaType) bool {
	return value == MessageMediaImage || value == MessageMediaAudio || value == MessageMediaVideo
}

func validMessageMediaURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil && parsed.Fragment == ""
}

func normalizedMessageMediaType(value MessageMediaType) socialhub.MediaType {
	switch value {
	case MessageMediaImage:
		return socialhub.MediaTypeImage
	case MessageMediaAudio:
		return socialhub.MediaTypeAudio
	default:
		return socialhub.MediaTypeVideo
	}
}

var _ socialhub.Messenger = (*Client)(nil)
var _ MessagingWorkflow = (*Client)(nil)
var _ MessagingProfileWorkflow = (*Client)(nil)

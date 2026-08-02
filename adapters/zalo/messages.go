package zalo

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

func (c *Client) SendMessage(ctx context.Context, input socialhub.SendMessageRequest, options ...socialhub.CallOption) (*socialhub.Message, error) {
	userID := strings.TrimSpace(input.ConversationID)
	if !validNumericID(userID) || input.Text == nil || strings.TrimSpace(*input.Text) == "" {
		return nil, invalidArgument("send_message", "Zalo user ID and non-empty text are required")
	}
	if len(input.RecipientIDs) > 1 || (len(input.RecipientIDs) == 1 && input.RecipientIDs[0] != userID) {
		return nil, invalidArgument("send_message", "recipient IDs must be empty or contain only the conversation user ID")
	}
	if len(input.MediaIDs) > 0 {
		return nil, unsupported("send_message", "Zalo attachments use separate typed consultation-message operations")
	}
	if input.ReplyToID != nil {
		return nil, unsupported("send_message", "quoted Zalo replies are not represented by the common Messenger")
	}
	result, err := c.SendConsultationText(ctx, userID, *input.Text, options...)
	if err != nil {
		return nil, err
	}
	sentAt, err := parseMilliseconds(result.SentTime)
	if err != nil {
		return nil, platformError("send_message", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	extension, _ := json.Marshal(result)
	return &socialhub.Message{
		Platform: "zalo", AccountID: c.accountID, ID: result.MessageID, ConversationID: userID,
		RecipientIDs: []string{userID}, Text: input.Text, SentAt: &sentAt, Direction: socialhub.DirectionOutbound,
		Extensions: map[string]json.RawMessage{"zalo.send_result": extension},
	}, nil
}

func (c *Client) GetMessage(context.Context, string, ...socialhub.CallOption) (*socialhub.Message, error) {
	return nil, unsupported("get_message", "Zalo OA OpenAPI does not provide arbitrary message lookup")
}

// SendConsultationText sends one v3 consultation text message. The recipient
// must satisfy Zalo's current interaction window and OA quota rules.
func (c *Client) SendConsultationText(ctx context.Context, userID, text string, options ...socialhub.CallOption) (*SendResult, error) {
	userID = strings.TrimSpace(userID)
	if !validNumericID(userID) || strings.TrimSpace(text) == "" || utf8.RuneCountInString(text) > 2000 {
		return nil, invalidArgument("send_consultation_text", "user ID and text containing at most 2,000 characters are required")
	}
	body := map[string]any{
		"recipient": map[string]string{"user_id": userID},
		"message":   map[string]string{"text": text},
	}
	result, err := request[SendResult](ctx, c, http.MethodPost, "/v3.0/oa/message/cs", nil, body, "send_consultation_text", options...)
	if err != nil {
		return nil, err
	}
	if !validOpaqueID(result.MessageID, 256) || result.UserID != userID {
		return nil, platformError("send_consultation_text", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if _, err := parseMilliseconds(result.SentTime); err != nil {
		return nil, platformError("send_consultation_text", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return &result, nil
}

func parseMilliseconds(value string) (time.Time, error) {
	milliseconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || milliseconds < 0 {
		return time.Time{}, invalidArgument("timestamp", "timestamp must be non-negative milliseconds")
	}
	return time.UnixMilli(milliseconds).UTC(), nil
}

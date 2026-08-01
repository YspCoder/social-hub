package line

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

func (c *Client) SendMessage(ctx context.Context, input socialhub.SendMessageRequest, options ...socialhub.CallOption) (*socialhub.Message, error) {
	to := strings.TrimSpace(input.ConversationID)
	if !validRecipientID(to) || input.Text == nil || strings.TrimSpace(*input.Text) == "" {
		return nil, invalidArgument("send_message", "LINE recipient ID and non-empty text are required")
	}
	if len(input.RecipientIDs) > 0 {
		return nil, unsupported("send_message", "common Messenger sends to one LINE conversation ID; use Multicast for multiple users")
	}
	if len(input.MediaIDs) > 0 {
		return nil, unsupported("send_message", "LINE outbound media uses typed HTTPS message objects rather than reusable media IDs")
	}
	if input.ReplyToID != nil {
		return nil, unsupported("send_message", "LINE replies require a webhook reply token or quote token; use MessageWorkflow")
	}
	result, err := c.Push(ctx, PushRequest{To: to, Messages: []MessageObject{TextMessage{Text: *input.Text}}}, options...)
	if err != nil {
		return nil, err
	}
	now := c.clock.Now()
	extension, _ := json.Marshal(result.SentMessages[0])
	return &socialhub.Message{
		Platform: "line", AccountID: c.accountID, ID: result.SentMessages[0].ID, ConversationID: to,
		RecipientIDs: []string{to}, Text: input.Text, SentAt: &now, Direction: socialhub.DirectionOutbound,
		Extensions: map[string]json.RawMessage{"line.sent_message": extension},
	}, nil
}

func (c *Client) GetMessage(context.Context, string, ...socialhub.CallOption) (*socialhub.Message, error) {
	return nil, unsupported("get_message", "Messaging API does not provide arbitrary message lookup or history")
}

func (c *Client) Push(ctx context.Context, input PushRequest, options ...socialhub.CallOption) (*SendResult, error) {
	to := strings.TrimSpace(input.To)
	if !validRecipientID(to) {
		return nil, invalidArgument("push", "recipient must be a LINE user, group, or room ID")
	}
	if to[0] != 'U' {
		for _, message := range input.Messages {
			switch typed := message.(type) {
			case VideoMessage:
				if typed.TrackingID != "" {
					return nil, invalidArgument("push", "video tracking ID is unavailable for group or room recipients")
				}
			case *VideoMessage:
				if typed != nil && typed.TrackingID != "" {
					return nil, invalidArgument("push", "video tracking ID is unavailable for group or room recipients")
				}
			}
		}
	}
	messages, err := encodeMessages(input.Messages)
	if err != nil {
		return nil, err
	}
	units, err := validateAggregationUnits(input.CustomAggregationUnits)
	if err != nil {
		return nil, err
	}
	body := map[string]any{"to": to, "messages": messages, "notificationDisabled": input.NotificationDisabled}
	if len(units) > 0 {
		body["customAggregationUnits"] = units
	}
	var result SendResult
	if err := c.request(ctx, c.api, http.MethodPost, "/v2/bot/message/push", nil, body, &result, true, options...); err != nil {
		return nil, err
	}
	if err := validateSendResult(result, len(messages), "push"); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) Reply(ctx context.Context, input ReplyRequest, options ...socialhub.CallOption) (*SendResult, error) {
	replyToken := strings.TrimSpace(input.ReplyToken)
	if !validOpaque(replyToken, 512) {
		return nil, invalidArgument("reply", "webhook reply token is required")
	}
	messages, err := encodeMessages(input.Messages)
	if err != nil {
		return nil, err
	}
	body := map[string]any{"replyToken": replyToken, "messages": messages, "notificationDisabled": input.NotificationDisabled}
	var result SendResult
	if err := c.request(ctx, c.api, http.MethodPost, "/v2/bot/message/reply", nil, body, &result, false, options...); err != nil {
		return nil, err
	}
	if err := validateSendResult(result, len(messages), "reply"); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) Multicast(ctx context.Context, input MulticastRequest, options ...socialhub.CallOption) error {
	if len(input.To) == 0 || len(input.To) > 500 {
		return invalidArgument("multicast", "between 1 and 500 LINE user IDs are required")
	}
	recipients := make([]string, len(input.To))
	seen := make(map[string]struct{}, len(input.To))
	for index, value := range input.To {
		value = strings.TrimSpace(value)
		if !validLINEID(value, 'U') {
			return invalidArgument("multicast", "multicast recipients must be LINE user IDs")
		}
		if _, exists := seen[value]; exists {
			return invalidArgument("multicast", "multicast recipients must be unique")
		}
		seen[value], recipients[index] = struct{}{}, value
	}
	messages, err := encodeMessages(input.Messages)
	if err != nil {
		return err
	}
	units, err := validateAggregationUnits(input.CustomAggregationUnits)
	if err != nil {
		return err
	}
	body := map[string]any{"to": recipients, "messages": messages, "notificationDisabled": input.NotificationDisabled}
	if len(units) > 0 {
		body["customAggregationUnits"] = units
	}
	return c.request(ctx, c.api, http.MethodPost, "/v2/bot/message/multicast", nil, body, nil, true, options...)
}

func (c *Client) Broadcast(ctx context.Context, input BroadcastRequest, options ...socialhub.CallOption) error {
	messages, err := encodeMessages(input.Messages)
	if err != nil {
		return err
	}
	units, err := validateAggregationUnits(input.CustomAggregationUnits)
	if err != nil {
		return err
	}
	body := map[string]any{"messages": messages, "notificationDisabled": input.NotificationDisabled}
	if len(units) > 0 {
		body["customAggregationUnits"] = units
	}
	return c.request(ctx, c.api, http.MethodPost, "/v2/bot/message/broadcast", nil, body, nil, true, options...)
}

func (message TextMessage) lineMessage() (map[string]any, error) {
	if strings.TrimSpace(message.Text) == "" || utf8.RuneCountInString(message.Text) > 5000 {
		return nil, invalidArgument("message", "text must contain between 1 and 5000 characters")
	}
	result := map[string]any{"type": "text", "text": message.Text}
	if message.QuoteToken != "" {
		if !validOpaque(message.QuoteToken, 512) {
			return nil, invalidArgument("message", "quote token is invalid")
		}
		result["quoteToken"] = message.QuoteToken
	}
	return result, nil
}

func (message StickerMessage) lineMessage() (map[string]any, error) {
	if !validOpaque(message.PackageID, 64) || !validOpaque(message.StickerID, 64) {
		return nil, invalidArgument("message", "sticker package and sticker IDs are required")
	}
	result := map[string]any{"type": "sticker", "packageId": message.PackageID, "stickerId": message.StickerID}
	if message.QuoteToken != "" {
		if !validOpaque(message.QuoteToken, 512) {
			return nil, invalidArgument("message", "quote token is invalid")
		}
		result["quoteToken"] = message.QuoteToken
	}
	return result, nil
}

func (message ImageMessage) lineMessage() (map[string]any, error) {
	if !validHTTPSURL(message.OriginalContentURL) || !validHTTPSURL(message.PreviewImageURL) {
		return nil, invalidArgument("message", "image content and preview URLs must be HTTPS URLs without credentials")
	}
	return map[string]any{"type": "image", "originalContentUrl": message.OriginalContentURL, "previewImageUrl": message.PreviewImageURL}, nil
}

func (message VideoMessage) lineMessage() (map[string]any, error) {
	if !validHTTPSURL(message.OriginalContentURL) || !validHTTPSURL(message.PreviewImageURL) {
		return nil, invalidArgument("message", "video content and preview URLs must be HTTPS URLs without credentials")
	}
	result := map[string]any{"type": "video", "originalContentUrl": message.OriginalContentURL, "previewImageUrl": message.PreviewImageURL}
	if message.TrackingID != "" {
		if !validTrackingID(message.TrackingID) {
			return nil, invalidArgument("message", "video tracking ID is invalid")
		}
		result["trackingId"] = message.TrackingID
	}
	return result, nil
}

func (message AudioMessage) lineMessage() (map[string]any, error) {
	if !validHTTPSURL(message.OriginalContentURL) || message.Duration <= 0 {
		return nil, invalidArgument("message", "audio requires an HTTPS content URL and positive duration")
	}
	return map[string]any{"type": "audio", "originalContentUrl": message.OriginalContentURL, "duration": message.Duration.Milliseconds()}, nil
}

func (message LocationMessage) lineMessage() (map[string]any, error) {
	if strings.TrimSpace(message.Title) == "" || utf8.RuneCountInString(message.Title) > 100 || strings.TrimSpace(message.Address) == "" || utf8.RuneCountInString(message.Address) > 100 || math.IsNaN(message.Latitude) || math.IsNaN(message.Longitude) || math.IsInf(message.Latitude, 0) || math.IsInf(message.Longitude, 0) || message.Latitude < -90 || message.Latitude > 90 || message.Longitude < -180 || message.Longitude > 180 {
		return nil, invalidArgument("message", "location requires title, address, and valid coordinates")
	}
	return map[string]any{"type": "location", "title": message.Title, "address": message.Address, "latitude": message.Latitude, "longitude": message.Longitude}, nil
}

func encodeMessages(messages []MessageObject) ([]map[string]any, error) {
	if len(messages) == 0 || len(messages) > 5 {
		return nil, invalidArgument("messages", "between 1 and 5 message objects are required")
	}
	result := make([]map[string]any, len(messages))
	for index, message := range messages {
		if message == nil || (reflect.ValueOf(message).Kind() == reflect.Pointer && reflect.ValueOf(message).IsNil()) {
			return nil, invalidArgument("messages", "message objects must not be nil")
		}
		encoded, err := message.lineMessage()
		if err != nil {
			return nil, err
		}
		result[index] = encoded
	}
	return result, nil
}

func validateSendResult(result SendResult, expected int, operation string) error {
	if len(result.SentMessages) != expected {
		return platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	for _, message := range result.SentMessages {
		if !validOpaque(message.ID, 256) {
			return platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
	}
	return nil
}

func validateAggregationUnits(values []string) ([]string, error) {
	if len(values) > 1 {
		return nil, invalidArgument("aggregation_units", "at most one custom aggregation unit is supported")
	}
	result := make([]string, len(values))
	for index, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || len(value) > 30 {
			return nil, invalidArgument("aggregation_units", "aggregation unit must contain 1 to 30 ASCII letters, digits, or underscores")
		}
		for _, character := range value {
			if character != '_' && !(character >= 'a' && character <= 'z') && !(character >= 'A' && character <= 'Z') && !(character >= '0' && character <= '9') {
				return nil, invalidArgument("aggregation_units", "aggregation unit must contain 1 to 30 ASCII letters, digits, or underscores")
			}
		}
		result[index] = value
	}
	return result, nil
}

func validLINEID(value string, prefixes ...byte) bool {
	if len(value) != 33 {
		return false
	}
	validPrefix := false
	for _, prefix := range prefixes {
		if value[0] == prefix {
			validPrefix = true
			break
		}
	}
	if !validPrefix {
		return false
	}
	for _, character := range value[1:] {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return false
		}
	}
	return true
}

func validRecipientID(value string) bool { return validLINEID(value, 'U', 'C', 'R') }

func validOpaque(value string, maximum int) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maximum {
		return false
	}
	for _, character := range value {
		if character <= 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}

func validHTTPSURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && utf8.RuneCountInString(value) <= 2000 && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil
}

func validTrackingID(value string) bool {
	if value == "" || len(value) > 100 {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') || strings.ContainsRune("-.=,+*()%$&;:@{}!?<>[]", character) {
			continue
		}
		return false
	}
	return true
}

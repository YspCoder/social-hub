package viber

import (
	"context"
	"encoding/json"
	"math"
	"net/url"
	"path"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

const (
	maxVideoBytes = 26_000_000
	maxFileBytes  = 50_000_000
)

func (c *Client) SendMessage(ctx context.Context, input socialhub.SendMessageRequest, options ...socialhub.CallOption) (*socialhub.Message, error) {
	receiver := strings.TrimSpace(input.ConversationID)
	if !validOpaqueID(receiver) || input.Text == nil || strings.TrimSpace(*input.Text) == "" {
		return nil, invalidArgument("send_message", "conversation ID and non-empty text are required")
	}
	if len(input.RecipientIDs) > 1 || (len(input.RecipientIDs) == 1 && input.RecipientIDs[0] != receiver) {
		return nil, invalidArgument("send_message", "recipient IDs must be empty or contain only the conversation ID")
	}
	if len(input.MediaIDs) > 0 {
		return nil, unsupported("send_message", "use MessageWorkflow.Send so the Viber media type and remote URL are explicit")
	}
	if input.ReplyToID != nil {
		return nil, unsupported("send_message", "Viber Bot API does not expose a reply-to message parameter")
	}
	result, err := c.Send(ctx, SendRequest{Receiver: receiver, Message: TextMessage{Text: *input.Text}}, options...)
	if err != nil {
		return nil, err
	}
	return c.mapOutboundMessage(receiver, TextMessage{Text: *input.Text}, "", result), nil
}

func (c *Client) GetMessage(context.Context, string, ...socialhub.CallOption) (*socialhub.Message, error) {
	return nil, unsupported("get_message", "Viber Bot API does not provide arbitrary message lookup")
}

// Send sends one typed message to a subscribed Viber user.
func (c *Client) Send(ctx context.Context, input SendRequest, options ...socialhub.CallOption) (*SendResult, error) {
	receiver := strings.TrimSpace(input.Receiver)
	if !validOpaqueID(receiver) {
		return nil, invalidArgument("send", "a bounded Viber subscriber ID is required")
	}
	body, err := c.messageBody(input.Message, input.TrackingData, input.MinAPIVersion)
	if err != nil {
		return nil, err
	}
	body["receiver"] = receiver
	var result SendResult
	if err := c.request(ctx, "/pa/send_message", body, &result, options...); err != nil {
		return nil, err
	}
	if !validMessageToken(result.MessageToken.String()) {
		return nil, platformError("send", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if result.BillingStatus < 0 || result.BillingStatus > 5 || (result.ChatHostname != "" && !validOpaqueID(result.ChatHostname)) {
		return nil, platformError("send", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &result, nil
}

// Broadcast sends one typed message to at most 300 unique subscribers.
func (c *Client) Broadcast(ctx context.Context, input BroadcastRequest, options ...socialhub.CallOption) (*SendResult, error) {
	receivers, err := validateRecipients(input.Receivers, 300, "broadcast")
	if err != nil {
		return nil, err
	}
	body, err := c.messageBody(input.Message, input.TrackingData, input.MinAPIVersion)
	if err != nil {
		return nil, err
	}
	body["broadcast_list"] = receivers
	var result SendResult
	if err := c.request(ctx, "/pa/broadcast_message", body, &result, options...); err != nil {
		return nil, err
	}
	if !validMessageToken(result.MessageToken.String()) {
		return nil, platformError("broadcast", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	for _, failure := range result.FailedList {
		if !validOpaqueID(failure.Receiver) || failure.Status == 0 {
			return nil, platformError("broadcast", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
	}
	return &result, nil
}

func (c *Client) messageBody(message MessageObject, trackingData string, minimumVersion int) (map[string]any, error) {
	if message == nil || (reflect.ValueOf(message).Kind() == reflect.Pointer && reflect.ValueOf(message).IsNil()) {
		return nil, invalidArgument("message", "a typed message is required")
	}
	if utf8.RuneCountInString(trackingData) > 4096 {
		return nil, invalidArgument("message", "tracking data must not exceed 4096 characters")
	}
	if minimumVersion == 0 {
		minimumVersion = 1
	}
	if minimumVersion < 1 || minimumVersion > 99 {
		return nil, invalidArgument("message", "minimum API version must be between 1 and 99")
	}
	body, err := message.viberMessage()
	if err != nil {
		return nil, err
	}
	body["sender"] = c.sender
	body["min_api_version"] = minimumVersion
	if trackingData != "" {
		body["tracking_data"] = trackingData
	}
	return body, nil
}

func (message TextMessage) viberMessage() (map[string]any, error) {
	if strings.TrimSpace(message.Text) == "" || utf8.RuneCountInString(message.Text) > 7000 {
		return nil, invalidArgument("message", "text must contain between 1 and 7000 characters")
	}
	return map[string]any{"type": "text", "text": message.Text}, nil
}

func (message PictureMessage) viberMessage() (map[string]any, error) {
	if utf8.RuneCountInString(message.Text) > 768 || !validMediaURL(message.MediaURL, ".jpeg", ".jpg", ".png", ".gif") {
		return nil, invalidArgument("message", "picture requires a JPEG, PNG, or GIF URL and at most 768 description characters")
	}
	if message.ThumbnailURL != "" && !validMediaURL(message.ThumbnailURL, ".jpeg", ".jpg", ".png", ".gif") {
		return nil, invalidArgument("message", "picture thumbnail must be a JPEG, PNG, or GIF URL")
	}
	body := map[string]any{"type": "picture", "text": message.Text, "media": message.MediaURL}
	if message.ThumbnailURL != "" {
		body["thumbnail"] = message.ThumbnailURL
	}
	return body, nil
}

func (message VideoMessage) viberMessage() (map[string]any, error) {
	if !validMediaURL(message.MediaURL, ".mp4") || message.Size <= 0 || message.Size > maxVideoBytes {
		return nil, invalidArgument("message", "video requires an MP4 URL and size between 1 byte and 26 MB")
	}
	if message.ThumbnailURL != "" && !validMediaURL(message.ThumbnailURL, ".jpeg", ".jpg") {
		return nil, invalidArgument("message", "video thumbnail must be a JPEG URL")
	}
	if message.Duration < 0 || message.Duration > 180*time.Second || message.Duration%time.Second != 0 {
		return nil, invalidArgument("message", "video duration must be whole seconds between 0 and 180")
	}
	body := map[string]any{"type": "video", "media": message.MediaURL, "size": message.Size}
	if message.ThumbnailURL != "" {
		body["thumbnail"] = message.ThumbnailURL
	}
	if message.Duration > 0 {
		body["duration"] = int64(message.Duration / time.Second)
	}
	return body, nil
}

func (message FileMessage) viberMessage() (map[string]any, error) {
	filename := strings.TrimSpace(message.Filename)
	if !validFileURL(message.MediaURL) || message.Size <= 0 || message.Size > maxFileBytes ||
		filename == "" || utf8.RuneCountInString(filename) > 256 || path.Base(strings.ReplaceAll(filename, "\\", "/")) != filename || path.Ext(filename) == "" {
		return nil, invalidArgument("message", "file requires a remote URL with extension, a plain filename with extension, and size between 1 byte and 50 MB")
	}
	return map[string]any{"type": "file", "media": message.MediaURL, "size": message.Size, "file_name": filename}, nil
}

func (message ContactMessage) viberMessage() (map[string]any, error) {
	if strings.TrimSpace(message.Name) == "" || utf8.RuneCountInString(message.Name) > 28 ||
		strings.TrimSpace(message.PhoneNumber) == "" || utf8.RuneCountInString(message.PhoneNumber) > 18 {
		return nil, invalidArgument("message", "contact name and phone number must not exceed 28 and 18 characters")
	}
	return map[string]any{"type": "contact", "contact": map[string]string{"name": message.Name, "phone_number": message.PhoneNumber}}, nil
}

func (message LocationMessage) viberMessage() (map[string]any, error) {
	if math.IsNaN(message.Latitude) || math.IsInf(message.Latitude, 0) || math.IsNaN(message.Longitude) || math.IsInf(message.Longitude, 0) ||
		message.Latitude < -90 || message.Latitude > 90 || message.Longitude < -180 || message.Longitude > 180 {
		return nil, invalidArgument("message", "location coordinates are outside valid latitude or longitude bounds")
	}
	return map[string]any{"type": "location", "location": Location{Latitude: message.Latitude, Longitude: message.Longitude}}, nil
}

func (message URLMessage) viberMessage() (map[string]any, error) {
	if !validRemoteURL(message.URL, 2000) {
		return nil, invalidArgument("message", "URL message requires an absolute HTTP(S) URL no longer than 2000 characters")
	}
	return map[string]any{"type": "url", "media": message.URL}, nil
}

func (message StickerMessage) viberMessage() (map[string]any, error) {
	if message.StickerID <= 0 {
		return nil, invalidArgument("message", "sticker ID must be a positive integer")
	}
	return map[string]any{"type": "sticker", "sticker_id": message.StickerID}, nil
}

func validateRecipients(values []string, maximum int, operation string) ([]string, error) {
	if len(values) == 0 || len(values) > maximum {
		return nil, invalidArgument(operation, "recipient count is outside the documented limit")
	}
	result := make([]string, len(values))
	seen := make(map[string]struct{}, len(values))
	for index, value := range values {
		value = strings.TrimSpace(value)
		if !validOpaqueID(value) {
			return nil, invalidArgument(operation, "recipient IDs must be bounded non-empty Viber subscriber IDs")
		}
		if _, exists := seen[value]; exists {
			return nil, invalidArgument(operation, "recipient IDs must be unique")
		}
		seen[value], result[index] = struct{}{}, value
	}
	return result, nil
}

func validOpaqueID(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 256 {
		return false
	}
	for _, character := range value {
		if character <= 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}

func validMessageToken(value string) bool {
	if value == "" {
		return false
	}
	_, err := strconv.ParseUint(value, 10, 64)
	return err == nil
}

func validMediaURL(value string, extensions ...string) bool {
	if !validRemoteURL(value, 2000) {
		return false
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}
	extension := strings.ToLower(path.Ext(parsed.Path))
	for _, allowed := range extensions {
		if extension == allowed {
			return true
		}
	}
	return false
}

func validFileURL(value string) bool {
	if !validRemoteURL(value, 2000) {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && path.Ext(parsed.Path) != ""
}

func (c *Client) mapOutboundMessage(receiver string, message MessageObject, trackingData string, result *SendResult) *socialhub.Message {
	id := result.MessageToken.String()
	now := c.clock.Now().UTC()
	var text *string
	var media []socialhub.Media
	switch typed := message.(type) {
	case TextMessage:
		text = stringPointer(typed.Text)
	case PictureMessage:
		text = optionalStringPointer(typed.Text)
		media = []socialhub.Media{{URL: typed.MediaURL, Type: socialhub.MediaTypeImage, State: socialhub.MediaStateReady}}
	case VideoMessage:
		duration, size := typed.Duration, typed.Size
		media = []socialhub.Media{{URL: typed.MediaURL, Type: socialhub.MediaTypeVideo, Size: &size, Duration: &duration, State: socialhub.MediaStateReady}}
	case FileMessage:
		size := typed.Size
		media = []socialhub.Media{{URL: typed.MediaURL, Type: socialhub.MediaTypeDocument, Size: &size, State: socialhub.MediaStateReady}}
	case URLMessage:
		text = stringPointer(typed.URL)
	}
	extension, _ := json.Marshal(struct {
		Type          string `json:"type"`
		TrackingData  string `json:"tracking_data,omitempty"`
		ChatHostname  string `json:"chat_hostname,omitempty"`
		BillingStatus int    `json:"billing_status,omitempty"`
	}{Type: messageType(message), TrackingData: trackingData, ChatHostname: result.ChatHostname, BillingStatus: result.BillingStatus})
	return &socialhub.Message{
		Platform: "viber", AccountID: c.accountID, ID: id, ConversationID: receiver,
		RecipientIDs: []string{receiver}, Text: text, Media: media, SentAt: &now, Direction: socialhub.DirectionOutbound,
		Extensions: map[string]json.RawMessage{"viber.message": extension},
	}
}

func messageType(message MessageObject) string {
	switch message.(type) {
	case TextMessage:
		return "text"
	case PictureMessage:
		return "picture"
	case VideoMessage:
		return "video"
	case FileMessage:
		return "file"
	case ContactMessage:
		return "contact"
	case LocationMessage:
		return "location"
	case URLMessage:
		return "url"
	case StickerMessage:
		return "sticker"
	default:
		return "unknown"
	}
}

func stringPointer(value string) *string { return &value }

func optionalStringPointer(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

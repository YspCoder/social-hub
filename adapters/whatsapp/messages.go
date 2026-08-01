package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type messageResponse struct {
	MessagingProduct string `json:"messaging_product"`
	Contacts         []struct {
		Input string `json:"input"`
		WAID  string `json:"wa_id"`
	} `json:"contacts"`
	Messages []struct {
		ID     string `json:"id"`
		Status string `json:"message_status"`
	} `json:"messages"`
}

func (c *Client) SendMessage(ctx context.Context, input socialhub.SendMessageRequest, options ...socialhub.CallOption) (*socialhub.Message, error) {
	to := strings.TrimSpace(input.ConversationID)
	if !validRecipient(to) || input.Text == nil || strings.TrimSpace(*input.Text) == "" {
		return nil, invalidArgument("send_message", "recipient conversation ID and non-empty text are required")
	}
	if utf8.RuneCountInString(*input.Text) > 4096 {
		return nil, invalidArgument("send_message", "text body must not exceed 4096 characters")
	}
	if len(input.RecipientIDs) > 0 {
		return nil, unsupported("send_message", "WhatsApp Cloud API sends to one conversation ID")
	}
	if len(input.MediaIDs) > 0 {
		return nil, unsupported("send_message", "use MessageWorkflow.SendMedia so the WhatsApp media type is explicit")
	}
	if err := c.requireScope("send_message", "whatsapp_business_messaging"); err != nil {
		return nil, err
	}
	body := map[string]any{
		"messaging_product": "whatsapp", "recipient_type": "individual", "to": to,
		"type": "text", "text": map[string]any{"preview_url": false, "body": *input.Text},
	}
	var replyTo *string
	if input.ReplyToID != nil {
		replyID := strings.TrimSpace(*input.ReplyToID)
		if replyID == "" {
			return nil, invalidArgument("send_message", "reply message ID must not be empty")
		}
		body["context"] = map[string]string{"message_id": replyID}
		replyTo = &replyID
	}
	return c.send(ctx, "send_message", to, input.Text, replyTo, body, options...)
}

func (c *Client) GetMessage(context.Context, string, ...socialhub.CallOption) (*socialhub.Message, error) {
	return nil, unsupported("get_message", "Cloud API does not provide arbitrary message lookup")
}

func (c *Client) SendMedia(ctx context.Context, input MediaMessageRequest, options ...socialhub.CallOption) (*socialhub.Message, error) {
	to := strings.TrimSpace(input.To)
	mediaID := strings.TrimSpace(input.Media.ID)
	mediaLink := strings.TrimSpace(input.Media.Link)
	if !validRecipient(to) || !validMediaKind(input.Type) {
		return nil, invalidArgument("send_media", "recipient and supported media type are required")
	}
	if (mediaID == "") == (mediaLink == "") {
		return nil, invalidArgument("send_media", "exactly one media ID or public HTTPS link is required")
	}
	if mediaLink != "" && !validHTTPSURL(mediaLink) {
		return nil, invalidArgument("send_media", "media link must be a public HTTPS URL without credentials")
	}
	if input.Type == MediaAudio || input.Type == MediaSticker {
		if input.Caption != "" || input.Filename != "" {
			return nil, invalidArgument("send_media", "audio and sticker messages do not accept caption or filename")
		}
	}
	if input.Filename != "" && input.Type != MediaDocument {
		return nil, invalidArgument("send_media", "filename is supported only for documents")
	}
	if err := c.requireScope("send_media", "whatsapp_business_messaging"); err != nil {
		return nil, err
	}
	media := map[string]any{}
	if mediaID != "" {
		media["id"] = mediaID
	} else {
		media["link"] = mediaLink
	}
	if input.Caption != "" {
		media["caption"] = input.Caption
	}
	if input.Filename != "" {
		media["filename"] = input.Filename
	}
	body := map[string]any{
		"messaging_product": "whatsapp", "recipient_type": "individual", "to": to,
		"type": string(input.Type), string(input.Type): media,
	}
	replyID := strings.TrimSpace(input.ReplyToID)
	if replyID != "" {
		body["context"] = map[string]string{"message_id": replyID}
	}
	return c.send(ctx, "send_media", to, nil, stringPointer(replyID), body, options...)
}

func (c *Client) SendTemplate(ctx context.Context, input TemplateMessageRequest, options ...socialhub.CallOption) (*socialhub.Message, error) {
	to := strings.TrimSpace(input.To)
	languageCode := strings.TrimSpace(input.LanguageCode)
	if !validRecipient(to) || !validName(input.Name) || languageCode == "" {
		return nil, invalidArgument("send_template", "recipient, template name, and language code are required")
	}
	for _, component := range input.Components {
		if !validName(component.Type) {
			return nil, invalidArgument("send_template", "template component type is invalid")
		}
		for _, parameter := range component.Parameters {
			trimmed := bytes.TrimSpace(parameter)
			if len(trimmed) == 0 || !json.Valid(trimmed) || trimmed[0] != '{' {
				return nil, invalidArgument("send_template", "template parameters must be JSON objects")
			}
		}
	}
	if err := c.requireScope("send_template", "whatsapp_business_messaging"); err != nil {
		return nil, err
	}
	template := map[string]any{"name": input.Name, "language": map[string]string{"code": languageCode}}
	if len(input.Components) > 0 {
		template["components"] = input.Components
	}
	body := map[string]any{
		"messaging_product": "whatsapp", "recipient_type": "individual", "to": to,
		"type": "template", "template": template,
	}
	replyID := strings.TrimSpace(input.ReplyToID)
	if replyID != "" {
		body["context"] = map[string]string{"message_id": replyID}
	}
	return c.send(ctx, "send_template", to, nil, stringPointer(replyID), body, options...)
}

func (c *Client) SendReaction(ctx context.Context, to, messageID, emoji string, options ...socialhub.CallOption) (*socialhub.Message, error) {
	to = strings.TrimSpace(to)
	messageID = strings.TrimSpace(messageID)
	if !validRecipient(to) || messageID == "" || (emoji != "" && (strings.TrimSpace(emoji) == "" || utf8.RuneCountInString(emoji) > 8)) {
		return nil, invalidArgument("send_reaction", "recipient and message ID are required; emoji must be empty or contain at most 8 characters")
	}
	if err := c.requireScope("send_reaction", "whatsapp_business_messaging"); err != nil {
		return nil, err
	}
	body := map[string]any{
		"messaging_product": "whatsapp", "recipient_type": "individual", "to": to, "type": "reaction",
		"reaction": map[string]string{"message_id": messageID, "emoji": emoji},
	}
	return c.send(ctx, "send_reaction", to, nil, &messageID, body, options...)
}

func (c *Client) MarkRead(ctx context.Context, messageID string, options ...socialhub.CallOption) error {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return invalidArgument("mark_read", "inbound message ID is required")
	}
	if err := c.requireScope("mark_read", "whatsapp_business_messaging"); err != nil {
		return err
	}
	body := map[string]string{"messaging_product": "whatsapp", "status": "read", "message_id": messageID}
	var response successPayload
	if err := c.request(ctx, http.MethodPut, c.phonePath("messages"), nil, body, &response, options...); err != nil {
		return err
	}
	return requireSuccess(response, "mark_read")
}

func (c *Client) send(ctx context.Context, operation, to string, text, replyTo *string, body any, options ...socialhub.CallOption) (*socialhub.Message, error) {
	var response messageResponse
	if err := c.request(ctx, http.MethodPost, c.phonePath("messages"), nil, body, &response, options...); err != nil {
		return nil, err
	}
	if response.MessagingProduct != "whatsapp" || len(response.Messages) != 1 || strings.TrimSpace(response.Messages[0].ID) == "" {
		return nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	now := c.clock.Now()
	extension, _ := json.Marshal(response)
	return &socialhub.Message{
		Platform: "whatsapp", AccountID: c.accountID, ID: response.Messages[0].ID, ConversationID: to,
		RecipientIDs: []string{to}, Text: text, ReplyToID: replyTo, SentAt: &now, Direction: socialhub.DirectionOutbound,
		Extensions: map[string]json.RawMessage{"whatsapp.message": extension},
	}, nil
}

func validRecipient(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if character <= 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}

func validName(value string) bool {
	if strings.TrimSpace(value) == "" || len(value) > 512 {
		return false
	}
	for _, character := range value {
		if !(character == '_' || character == '.' || character == '-' || (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9')) {
			return false
		}
	}
	return true
}

func validHTTPSURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil
}

func validMediaKind(value MediaKind) bool {
	switch value {
	case MediaImage, MediaVideo, MediaAudio, MediaDocument, MediaSticker:
		return true
	default:
		return false
	}
}

func stringPointer(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	copy := value
	return &copy
}

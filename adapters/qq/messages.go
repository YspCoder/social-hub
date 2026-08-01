package qq

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

type messagePayload struct {
	MessageType     *int              `json:"msg_type,omitempty"`
	Content         string            `json:"content,omitempty"`
	Markdown        *markdownBlock    `json:"markdown,omitempty"`
	Media           *mediaBlock       `json:"media,omitempty"`
	Image           string            `json:"image,omitempty"`
	MessageID       string            `json:"msg_id,omitempty"`
	EventID         string            `json:"event_id,omitempty"`
	MessageSequence int               `json:"msg_seq,omitempty"`
	Wakeup          bool              `json:"is_wakeup,omitempty"`
	Reference       *messageReference `json:"message_reference,omitempty"`
}

type markdownBlock struct {
	Content string `json:"content"`
}

type mediaBlock struct {
	FileInfo string `json:"file_info"`
}

type messageReference struct {
	MessageID string `json:"message_id"`
}

type messageResponse struct {
	APIError
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	ExtInfo   struct {
		ReferenceIndex string `json:"ref_idx"`
	} `json:"ext_info"`
	Data struct {
		MessageAudit struct {
			AuditID string `json:"audit_id"`
		} `json:"message_audit"`
	} `json:"data"`
}

func (c *Client) Send(ctx context.Context, input MessageRequest, options ...socialhub.CallOption) (*MessageResult, error) {
	if err := validateTarget(input.Target); err != nil {
		return nil, err
	}
	if input.Content == nil {
		return nil, invalidArgument("send_message", "message content is required")
	}
	selectors := 0
	for _, selected := range []bool{input.ReplyToID != "", input.EventID != "", input.Wakeup} {
		if selected {
			selectors++
		}
	}
	if selectors > 1 {
		return nil, invalidArgument("send_message", "reply message, event, and wakeup selectors are mutually exclusive")
	}
	if input.Sequence < 0 || input.Sequence > math.MaxInt32 {
		return nil, invalidArgument("send_message", "sequence must be a non-negative 32-bit integer")
	}
	if input.Target.Scene == SceneChannel && (input.Sequence != 0 || input.Wakeup) {
		return nil, unsupported("send_message", "channel messages do not use msg_seq or is_wakeup")
	}
	for _, value := range []string{input.ReplyToID, input.EventID, input.ReferenceID} {
		if value != "" && !validOpaque(value, 512) {
			return nil, invalidArgument("send_message", "message and event identifiers are invalid")
		}
	}
	payload := messagePayload{
		MessageID: input.ReplyToID, EventID: input.EventID,
		MessageSequence: input.Sequence, Wakeup: input.Wakeup,
	}
	if input.ReferenceID != "" {
		payload.Reference = &messageReference{MessageID: input.ReferenceID}
	}
	if err := input.Content.apply(input.Target.Scene, &payload); err != nil {
		return nil, err
	}
	var response messageResponse
	if err := c.api.JSON(ctx, http.MethodPost, input.Target.messagePath(), nil, payload, &response, options...); err != nil {
		return nil, err
	}
	code := response.EffectiveCode()
	if code == 304023 || code == 304024 {
		if !validOpaque(response.Data.MessageAudit.AuditID, 256) {
			return nil, platformError("send_message", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
		return &MessageResult{Target: input.Target, PendingAudit: true, AuditID: response.Data.MessageAudit.AuditID}, nil
	}
	if err := c.responseError(ctx, "send_message", response.APIError); err != nil {
		return nil, err
	}
	if !validOpaque(response.ID, 512) {
		return nil, platformError("send_message", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	sentAt, err := parseOptionalTime(response.Timestamp)
	if err != nil {
		return nil, platformError("send_message", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return &MessageResult{
		ID: response.ID, Target: input.Target, SentAt: sentAt, ReferenceIndex: response.ExtInfo.ReferenceIndex,
	}, nil
}

func (c *Client) Retract(ctx context.Context, target Target, messageID string, options ...socialhub.CallOption) error {
	if err := validateTarget(target); err != nil {
		return err
	}
	if !validOpaque(messageID, 512) {
		return invalidArgument("retract_message", "message ID is invalid")
	}
	return c.api.JSON(ctx, http.MethodDelete, target.messagePath()+"/"+messageID, nil, nil, nil, options...)
}

func (c *Client) SendMessage(ctx context.Context, input socialhub.SendMessageRequest, options ...socialhub.CallOption) (*socialhub.Message, error) {
	target, err := ParseConversationID(input.ConversationID)
	if err != nil {
		return nil, err
	}
	if len(input.RecipientIDs) != 0 {
		return nil, unsupported("send_message", "QQ conversation ID already identifies the target")
	}
	if len(input.MediaIDs) != 0 {
		return nil, unsupported("send_message", "use MessageWorkflow with MediaContent")
	}
	if input.Text == nil {
		return nil, invalidArgument("send_message", "text is required")
	}
	replyID := ""
	if input.ReplyToID != nil {
		replyID = *input.ReplyToID
	}
	result, err := c.Send(ctx, MessageRequest{Target: target, Content: TextContent{Text: *input.Text}, ReplyToID: replyID}, options...)
	if err != nil {
		return nil, err
	}
	messageID := result.ID
	if result.PendingAudit {
		messageID = "audit:" + result.AuditID
	}
	extensions := map[string]json.RawMessage{}
	if encoded, marshalErr := json.Marshal(result); marshalErr == nil {
		extensions["qq.delivery"] = encoded
	}
	return &socialhub.Message{
		Platform: "qq", AccountID: c.accountID, ID: messageID, ConversationID: target.ConversationID(),
		RecipientIDs: []string{target.ID}, Text: input.Text, ReplyToID: input.ReplyToID,
		SentAt: result.SentAt, Direction: socialhub.DirectionOutbound, Extensions: extensions,
	}, nil
}

func (c *Client) GetMessage(context.Context, string, ...socialhub.CallOption) (*socialhub.Message, error) {
	return nil, unsupported("get_message", "QQ Bot OpenAPI does not expose arbitrary C2C/group message lookup")
}

func (target Target) messagePath() string {
	switch target.Scene {
	case SceneC2C:
		return "/v2/users/" + target.ID + "/messages"
	case SceneGroup:
		return "/v2/groups/" + target.ID + "/messages"
	default:
		return "/channels/" + target.ID + "/messages"
	}
}

func (target Target) filesPath() string {
	if target.Scene == SceneC2C {
		return "/v2/users/" + target.ID + "/files"
	}
	return "/v2/groups/" + target.ID + "/files"
}

func parseOptionalTime(value string) (*time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, err
	}
	parsed = parsed.UTC()
	return &parsed, nil
}

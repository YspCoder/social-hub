package wecom

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

// MessageContent is the closed set of application message content supported by
// this adapter. Use TextContent, MarkdownContent, ImageContent, VoiceContent,
// VideoContent, or FileContent.
type MessageContent interface {
	apply(*applicationMessagePayload) error
}

// TextContent sends a plain-text application message.
type TextContent struct{ Content string }

func (content TextContent) apply(payload *applicationMessagePayload) error {
	if content.Content == "" || len(content.Content) > 2048 {
		return invalidArgument("send_application_message", "text content must contain 1 to 2048 bytes")
	}
	payload.MessageType, payload.Text = "text", &textBlock{Content: content.Content}
	return nil
}

// MarkdownContent sends WeCom-supported Markdown.
type MarkdownContent struct{ Content string }

func (content MarkdownContent) apply(payload *applicationMessagePayload) error {
	if content.Content == "" || len(content.Content) > 2048 {
		return invalidArgument("send_application_message", "markdown content must contain 1 to 2048 bytes")
	}
	payload.MessageType, payload.Markdown = "markdown", &markdownBlock{Content: content.Content}
	return nil
}

// ImageContent sends a temporary image media ID.
type ImageContent struct{ MediaID string }

func (content ImageContent) apply(payload *applicationMessagePayload) error {
	if !validMediaID(content.MediaID) {
		return invalidArgument("send_application_message", "image media ID is invalid")
	}
	payload.MessageType, payload.Image = "image", &mediaBlock{MediaID: content.MediaID}
	return nil
}

// VoiceContent sends a temporary AMR voice media ID.
type VoiceContent struct{ MediaID string }

func (content VoiceContent) apply(payload *applicationMessagePayload) error {
	if !validMediaID(content.MediaID) {
		return invalidArgument("send_application_message", "voice media ID is invalid")
	}
	payload.MessageType, payload.Voice = "voice", &mediaBlock{MediaID: content.MediaID}
	return nil
}

// VideoContent sends a temporary MP4 media ID with optional display metadata.
type VideoContent struct {
	MediaID     string
	Title       string
	Description string
}

func (content VideoContent) apply(payload *applicationMessagePayload) error {
	if !validMediaID(content.MediaID) || len(content.Title) > 128 || len(content.Description) > 512 {
		return invalidArgument("send_application_message", "video media ID or display metadata is invalid")
	}
	payload.MessageType = "video"
	payload.Video = &videoBlock{MediaID: content.MediaID, Title: content.Title, Description: content.Description}
	return nil
}

// FileContent sends a temporary ordinary-file media ID.
type FileContent struct{ MediaID string }

func (content FileContent) apply(payload *applicationMessagePayload) error {
	if !validMediaID(content.MediaID) {
		return invalidArgument("send_application_message", "file media ID is invalid")
	}
	payload.MessageType, payload.File = "file", &mediaBlock{MediaID: content.MediaID}
	return nil
}

// RecipientSet selects members, departments, or tags. ToAll cannot be combined
// with any explicit recipient.
type RecipientSet struct {
	UserIDs  []string
	PartyIDs []int64
	TagIDs   []int64
	ToAll    bool
}

// ApplicationMessageRequest sends one typed content object to a recipient set.
type ApplicationMessageRequest struct {
	Recipients             RecipientSet
	Content                MessageContent
	Safe                   bool
	EnableIDTranslation    bool
	EnableDuplicateCheck   bool
	DuplicateCheckInterval time.Duration
}

// SendResult preserves partial-delivery diagnostics returned with a successful
// WeCom request.
type SendResult struct {
	MessageID         string       `json:"message_id,omitempty"`
	Recipients        RecipientSet `json:"-"`
	InvalidUserIDs    []string     `json:"invalid_user_ids,omitempty"`
	InvalidPartyIDs   []string     `json:"invalid_party_ids,omitempty"`
	InvalidTagIDs     []string     `json:"invalid_tag_ids,omitempty"`
	UnlicensedUserIDs []string     `json:"unlicensed_user_ids,omitempty"`
}

// ApplicationMessageWorkflow exposes typed self-built application messages.
type ApplicationMessageWorkflow interface {
	SendApplicationMessage(context.Context, ApplicationMessageRequest, ...socialhub.CallOption) (*SendResult, error)
}

// IncomingMessage is the stable common subset of decrypted WeCom callback XML.
type IncomingMessage struct {
	ToUserName   string `xml:"ToUserName" json:"to_user_name"`
	FromUserName string `xml:"FromUserName" json:"from_user_name"`
	CreateTime   int64  `xml:"CreateTime" json:"create_time"`
	MessageType  string `xml:"MsgType" json:"message_type"`
	Content      string `xml:"Content" json:"content,omitempty"`
	MessageID    string `xml:"MsgId" json:"message_id,omitempty"`
	AgentID      int64  `xml:"AgentID" json:"agent_id,omitempty"`
	MediaID      string `xml:"MediaId" json:"media_id,omitempty"`
	PicURL       string `xml:"PicUrl" json:"pic_url,omitempty"`
	Event        string `xml:"Event" json:"event,omitempty"`
	EventKey     string `xml:"EventKey" json:"event_key,omitempty"`
	ChangeType   string `xml:"ChangeType" json:"change_type,omitempty"`
	Raw          []byte `xml:"-" json:"-"`
}

func validMediaID(value string) bool {
	return validBoundedValue(value, 512, false)
}

func validBoundedValue(value string, maximum int, allowPipe bool) bool {
	if value == "" || strings.TrimSpace(value) != value || len(value) > maximum || (!allowPipe && strings.Contains(value, "|")) {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}

func stringPointer(value string) *string {
	if value == "" {
		return nil
	}
	copy := value
	return &copy
}

func jsonExtension(value any) map[string]json.RawMessage {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return map[string]json.RawMessage{"wecom.corp_api": encoded}
}

var _ ApplicationMessageWorkflow = (*Client)(nil)

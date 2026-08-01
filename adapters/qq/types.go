package qq

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

// Scene identifies the QQ conversation namespace for an opaque target ID.
type Scene string

const (
	SceneC2C     Scene = "c2c"
	SceneGroup   Scene = "group"
	SceneChannel Scene = "channel"
)

// Target preserves the scene-specific meaning of a QQ openid or channel ID.
type Target struct {
	Scene Scene  `json:"scene"`
	ID    string `json:"id"`
}

// ConversationID encodes a target for the common Messenger interface.
func (target Target) ConversationID() string { return string(target.Scene) + ":" + target.ID }

// ParseConversationID decodes a c2c:, group:, or channel: common conversation ID.
func ParseConversationID(value string) (Target, error) {
	sceneValue, id, found := strings.Cut(value, ":")
	if !found {
		return Target{}, invalidArgument("conversation_id", "conversation ID must use c2c:, group:, or channel: prefix")
	}
	target := Target{Scene: Scene(sceneValue), ID: id}
	if err := validateTarget(target); err != nil {
		return Target{}, err
	}
	return target, nil
}

// MessageContent is the closed set of typed QQ message content supported by
// this adapter.
type MessageContent interface {
	apply(Scene, *messagePayload) error
}

type TextContent struct{ Text string }

func (content TextContent) apply(scene Scene, payload *messagePayload) error {
	if strings.TrimSpace(content.Text) == "" || len(content.Text) > 1<<20 {
		return invalidArgument("send_message", "text must be non-empty and at most 1 MiB")
	}
	payload.Content = content.Text
	if scene != SceneChannel {
		value := 0
		payload.MessageType = &value
	}
	return nil
}

type MarkdownContent struct{ Markdown string }

func (content MarkdownContent) apply(scene Scene, payload *messagePayload) error {
	if strings.TrimSpace(content.Markdown) == "" || len(content.Markdown) > 1<<20 {
		return invalidArgument("send_message", "Markdown must be non-empty and at most 1 MiB")
	}
	payload.Markdown = &markdownBlock{Content: content.Markdown}
	if scene != SceneChannel {
		value := 2
		payload.MessageType = &value
	}
	return nil
}

// MediaContent sends a file_info returned by the matching C2C/group upload
// endpoint.
type MediaContent struct{ FileInfo string }

func (content MediaContent) apply(scene Scene, payload *messagePayload) error {
	if scene == SceneChannel {
		return unsupported("send_message", "channel messages do not accept C2C/group file_info")
	}
	if !validBoundedString(content.FileInfo, 65536) {
		return invalidArgument("send_message", "file_info is invalid")
	}
	value := 7
	payload.MessageType = &value
	payload.Media = &mediaBlock{FileInfo: content.FileInfo}
	return nil
}

// ChannelImageContent sends an HTTP(S) image URL to a channel.
type ChannelImageContent struct{ URL string }

func (content ChannelImageContent) apply(scene Scene, payload *messagePayload) error {
	if scene != SceneChannel {
		return unsupported("send_message", "channel image URLs are only valid for channel targets")
	}
	if !validMediaURL(content.URL) {
		return invalidArgument("send_message", "channel image URL must be an absolute HTTP(S) URL without credentials")
	}
	payload.Image = content.URL
	return nil
}

// MessageRequest sends one typed message to a scene-specific QQ target.
type MessageRequest struct {
	Target      Target
	Content     MessageContent
	ReplyToID   string
	EventID     string
	Sequence    int
	Wakeup      bool
	ReferenceID string
}

// MessageResult preserves asynchronous audit state returned for channel sends.
type MessageResult struct {
	ID             string     `json:"id,omitempty"`
	Target         Target     `json:"target"`
	SentAt         *time.Time `json:"sent_at,omitempty"`
	PendingAudit   bool       `json:"pending_audit,omitempty"`
	AuditID        string     `json:"audit_id,omitempty"`
	ReferenceIndex string     `json:"reference_index,omitempty"`
}

// MessageWorkflow exposes scene-aware QQ sends and two-minute retraction.
type MessageWorkflow interface {
	Send(context.Context, MessageRequest, ...socialhub.CallOption) (*MessageResult, error)
	Retract(context.Context, Target, string, ...socialhub.CallOption) error
}

// MediaFileType is QQ's URL-upload media type.
type MediaFileType int

const (
	MediaFileImage MediaFileType = 1
	MediaFileVideo MediaFileType = 2
	MediaFileVoice MediaFileType = 3
	MediaFileFile  MediaFileType = 4
)

type UploadURLRequest struct {
	Target   Target
	Type     MediaFileType
	URL      string
	Filename string
}

type MediaAsset struct {
	Target    Target        `json:"target"`
	Type      MediaFileType `json:"type"`
	FileUUID  string        `json:"file_uuid"`
	FileInfo  string        `json:"file_info"`
	TTL       time.Duration `json:"ttl,omitempty"`
	ExpiresAt *time.Time    `json:"expires_at,omitempty"`
	RawURL    string        `json:"raw_url,omitempty"`
}

type URLMediaWorkflow interface {
	UploadURL(context.Context, UploadURLRequest, ...socialhub.CallOption) (*MediaAsset, error)
}

type WebhookEvent struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Sequence *int64          `json:"sequence,omitempty"`
	Data     json.RawMessage `json:"data"`
	Raw      json.RawMessage `json:"-"`
}

type WebhookWorkflow interface {
	ValidationResponse([]byte) ([]byte, error)
}

func validateTarget(target Target) error {
	if target.Scene != SceneC2C && target.Scene != SceneGroup && target.Scene != SceneChannel {
		return invalidArgument("target", "scene must be c2c, group, or channel")
	}
	if !validOpaque(target.ID, 256) {
		return invalidArgument("target", "target ID is invalid")
	}
	return nil
}

func validOpaque(value string, maximum int) bool {
	if value == "" || strings.TrimSpace(value) != value || len(value) > maximum || strings.ContainsAny(value, "/\\?#") {
		return false
	}
	return validBoundedString(value, maximum)
}

func validBoundedString(value string, maximum int) bool {
	if value == "" || strings.TrimSpace(value) != value || len(value) > maximum {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	return utf8.ValidString(value)
}

func validMediaURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil && parsed.Fragment == ""
}

var _ MessageWorkflow = (*Client)(nil)
var _ URLMediaWorkflow = (*Client)(nil)
var _ WebhookWorkflow = (*Client)(nil)

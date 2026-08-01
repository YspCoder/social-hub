package line

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"social-hub/pkg/socialhub"
)

// MessageObject is the closed set of outbound message objects supported by
// this adapter. Use one of TextMessage, StickerMessage, ImageMessage,
// VideoMessage, AudioMessage, or LocationMessage.
type MessageObject interface {
	lineMessage() (map[string]any, error)
}

// TextMessage sends UTF-8 text and can quote a prior message by quote token.
type TextMessage struct {
	Text       string
	QuoteToken string
}

// StickerMessage sends a LINE sticker from a documented package.
type StickerMessage struct {
	PackageID  string
	StickerID  string
	QuoteToken string
}

// ImageMessage sends an HTTPS image with an HTTPS preview.
type ImageMessage struct {
	OriginalContentURL string
	PreviewImageURL    string
}

// VideoMessage sends an HTTPS video with a preview image.
type VideoMessage struct {
	OriginalContentURL string
	PreviewImageURL    string
	TrackingID         string
}

// AudioMessage sends HTTPS audio. Duration is encoded in milliseconds.
type AudioMessage struct {
	OriginalContentURL string
	Duration           time.Duration
}

// LocationMessage sends a geographic location.
type LocationMessage struct {
	Title     string
	Address   string
	Latitude  float64
	Longitude float64
}

// PushRequest sends up to five messages to one user, group, or room.
type PushRequest struct {
	To                     string
	Messages               []MessageObject
	NotificationDisabled   bool
	CustomAggregationUnits []string
}

// ReplyRequest consumes a webhook reply token once.
type ReplyRequest struct {
	ReplyToken           string
	Messages             []MessageObject
	NotificationDisabled bool
}

// MulticastRequest sends the same messages to up to 500 users.
type MulticastRequest struct {
	To                     []string
	Messages               []MessageObject
	NotificationDisabled   bool
	CustomAggregationUnits []string
}

// BroadcastRequest sends messages to all eligible friends of the channel.
type BroadcastRequest struct {
	Messages               []MessageObject
	NotificationDisabled   bool
	CustomAggregationUnits []string
}

// SentMessage identifies one accepted message and its optional quote token.
type SentMessage struct {
	ID         string `json:"id"`
	QuoteToken string `json:"quoteToken,omitempty"`
}

// SendResult preserves the per-message identifiers returned for push and
// reply calls.
type SendResult struct {
	SentMessages []SentMessage `json:"sentMessages"`
}

// MessageWorkflow exposes LINE-specific send modes and message objects.
type MessageWorkflow interface {
	Push(context.Context, PushRequest, ...socialhub.CallOption) (*SendResult, error)
	Reply(context.Context, ReplyRequest, ...socialhub.CallOption) (*SendResult, error)
	Multicast(context.Context, MulticastRequest, ...socialhub.CallOption) error
	Broadcast(context.Context, BroadcastRequest, ...socialhub.CallOption) error
}

// Profile is a LINE user profile visible to the configured channel.
type Profile struct {
	DisplayName   string `json:"displayName"`
	UserID        string `json:"userId"`
	PictureURL    string `json:"pictureUrl,omitempty"`
	StatusMessage string `json:"statusMessage,omitempty"`
	Language      string `json:"language,omitempty"`
}

// ProfileWorkflow reads direct and chat-scoped user profiles.
type ProfileWorkflow interface {
	GetProfile(context.Context, string, ...socialhub.CallOption) (*Profile, error)
	GetGroupMemberProfile(context.Context, string, string, ...socialhub.CallOption) (*Profile, error)
	GetRoomMemberProfile(context.Context, string, string, ...socialhub.CallOption) (*Profile, error)
}

// Content is a streaming response. The caller must close Body.
type Content struct {
	Body               io.ReadCloser
	ContentType        string
	ContentLength      int64
	ContentDisposition string
}

// TranscodingStatus describes preparation of inbound video or audio content.
type TranscodingStatus string

const (
	TranscodingProcessing TranscodingStatus = "processing"
	TranscodingSucceeded  TranscodingStatus = "succeeded"
	TranscodingFailed     TranscodingStatus = "failed"
)

// ContentWorkflow streams inbound content and reads preparation state.
type ContentWorkflow interface {
	DownloadContent(context.Context, string, ...socialhub.CallOption) (*Content, error)
	DownloadPreview(context.Context, string, ...socialhub.CallOption) (*Content, error)
	GetTranscodingStatus(context.Context, string, ...socialhub.CallOption) (TranscodingStatus, error)
}

// MessageQuota is the channel's current monthly target limit.
type MessageQuota struct {
	Type  string `json:"type"`
	Value *int64 `json:"value,omitempty"`
}

// QuotaConsumption is the number of messages sent in the current month.
type QuotaConsumption struct {
	TotalUsage int64 `json:"totalUsage"`
}

// QuotaWorkflow reads server-owned monthly quota state.
type QuotaWorkflow interface {
	GetMessageQuota(context.Context, ...socialhub.CallOption) (*MessageQuota, error)
	GetQuotaConsumption(context.Context, ...socialhub.CallOption) (*QuotaConsumption, error)
}

// EventSource identifies the user, group, or room that produced an event.
type EventSource struct {
	Type    string `json:"type"`
	UserID  string `json:"userId,omitempty"`
	GroupID string `json:"groupId,omitempty"`
	RoomID  string `json:"roomId,omitempty"`
}

// ContentProvider describes LINE-hosted or external message media.
type ContentProvider struct {
	Type               string `json:"type"`
	OriginalContentURL string `json:"originalContentUrl,omitempty"`
	PreviewImageURL    string `json:"previewImageUrl,omitempty"`
}

// IncomingMessage preserves the common fields of LINE webhook message types.
type IncomingMessage struct {
	ID                  string          `json:"id"`
	Type                string          `json:"type"`
	Text                string          `json:"text,omitempty"`
	QuoteToken          string          `json:"quoteToken,omitempty"`
	QuotedMessageID     string          `json:"quotedMessageId,omitempty"`
	PackageID           string          `json:"packageId,omitempty"`
	StickerID           string          `json:"stickerId,omitempty"`
	StickerResourceType string          `json:"stickerResourceType,omitempty"`
	Keywords            []string        `json:"keywords,omitempty"`
	FileName            string          `json:"fileName,omitempty"`
	FileSize            int64           `json:"fileSize,omitempty"`
	Duration            int64           `json:"duration,omitempty"`
	Title               string          `json:"title,omitempty"`
	Address             string          `json:"address,omitempty"`
	Latitude            float64         `json:"latitude,omitempty"`
	Longitude           float64         `json:"longitude,omitempty"`
	ContentProvider     ContentProvider `json:"contentProvider,omitempty"`
	Raw                 json.RawMessage `json:"-"`
}

// PostbackContent preserves the developer-defined data and typed parameter
// object from a postback webhook event.
type PostbackContent struct {
	Data   string          `json:"data"`
	Params json.RawMessage `json:"params,omitempty"`
}

// WebhookEvent is one normalized LINE webhook event with raw JSON retained for
// event types that evolve independently of this SDK.
type WebhookEvent struct {
	Destination  string
	ID           string
	Type         string
	Mode         string
	Timestamp    *time.Time
	Source       *EventSource
	ReplyToken   string
	IsRedelivery bool
	Message      *IncomingMessage
	Postback     *PostbackContent
	Raw          json.RawMessage
}

// TokenInfo describes a verified short-lived or long-lived channel token.
type TokenInfo struct {
	ChannelID string
	ExpiresAt time.Time
	Scopes    []string
}

var _ MessageWorkflow = (*Client)(nil)
var _ ProfileWorkflow = (*Client)(nil)
var _ ContentWorkflow = (*Client)(nil)
var _ QuotaWorkflow = (*Client)(nil)

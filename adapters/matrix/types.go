package matrix

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	EventTypeMessage   = "m.room.message"
	EventTypeReaction  = "m.reaction"
	EventTypeEncrypted = "m.room.encrypted"

	MessageTypeText   = "m.text"
	MessageTypeNotice = "m.notice"
	MessageTypeEmote  = "m.emote"
	MessageTypeImage  = "m.image"
	MessageTypeVideo  = "m.video"
	MessageTypeAudio  = "m.audio"
	MessageTypeFile   = "m.file"

	RelationAnnotation = "m.annotation"
	RelationThread     = "m.thread"
)

// Event is the stable subset shared by Matrix room event responses.
type Event struct {
	Type           string          `json:"type"`
	RoomID         string          `json:"room_id,omitempty"`
	EventID        string          `json:"event_id"`
	Sender         string          `json:"sender"`
	OriginServerTS int64           `json:"origin_server_ts"`
	Content        json.RawMessage `json:"content"`
	Unsigned       json.RawMessage `json:"unsigned,omitempty"`
}

// MessageContent represents unencrypted m.room.message content.
type MessageContent struct {
	MessageType   string     `json:"msgtype"`
	Body          string     `json:"body"`
	Format        string     `json:"format,omitempty"`
	FormattedBody string     `json:"formatted_body,omitempty"`
	URL           string     `json:"url,omitempty"`
	Info          *MediaInfo `json:"info,omitempty"`
	RelatesTo     *Relation  `json:"m.relates_to,omitempty"`
}

// MediaInfo carries Matrix attachment metadata.
type MediaInfo struct {
	MIMEType string `json:"mimetype,omitempty"`
	Size     int64  `json:"size,omitempty"`
	Width    int    `json:"w,omitempty"`
	Height   int    `json:"h,omitempty"`
	Duration int64  `json:"duration,omitempty"`
}

// Relation preserves reply, thread, and annotation relationships.
type Relation struct {
	RelationType string     `json:"rel_type,omitempty"`
	EventID      string     `json:"event_id,omitempty"`
	Key          string     `json:"key,omitempty"`
	InReplyTo    *InReplyTo `json:"m.in_reply_to,omitempty"`
}

// InReplyTo identifies the event referenced by a reply.
type InReplyTo struct {
	EventID string `json:"event_id"`
}

// EventReference identifies a Matrix event in one room.
type EventReference struct {
	RoomID  string
	EventID string
	ID      string
}

// RoomMessagesRequest selects a page from one room timeline.
type RoomMessagesRequest struct {
	RoomID     string
	Cursor     string
	MaxResults int
	Direction  string
}

// SendTextRequest sends an unencrypted text, notice, or emote event.
type SendTextRequest struct {
	RoomID       string
	MessageType  string
	Text         string
	ReplyToID    string
	ThreadRootID string
}

// SendMediaRequest sends an existing mxc URI as a room message.
type SendMediaRequest struct {
	RoomID      string
	MessageType string
	Body        string
	MXCURI      string
	MIME        string
	Size        int64
	Width       int
	Height      int
	Duration    time.Duration
}

// ReactionEventRequest creates an m.annotation event.
type ReactionEventRequest struct {
	RoomID        string
	TargetEventID string
	Key           string
}

// UploadRequest describes raw content for the Matrix media repository.
type UploadRequest struct {
	Filename string
	MIME     string
	Size     int64
}

// SyncRequest controls one incremental /sync request.
type SyncRequest struct {
	Since     string
	Timeout   time.Duration
	FullState bool
}

// SyncResponse preserves the next cursor and joined-room timelines.
type SyncResponse struct {
	NextBatch string    `json:"next_batch"`
	Rooms     SyncRooms `json:"rooms"`
}

// SyncRooms groups joined rooms returned by /sync.
type SyncRooms struct {
	Join map[string]JoinedRoom `json:"join"`
}

// JoinedRoom contains the incremental timeline for one joined room.
type JoinedRoom struct {
	Timeline Timeline `json:"timeline"`
}

// Timeline is a bounded room event page from /sync.
type Timeline struct {
	Events    []Event `json:"events"`
	Limited   bool    `json:"limited"`
	PrevBatch string  `json:"prev_batch"`
}

// EventWorkflow exposes Matrix-specific room event operations.
type EventWorkflow interface {
	GetEvent(context.Context, string, string, ...socialhub.CallOption) (*Event, error)
	ListRoomMessages(context.Context, RoomMessagesRequest, ...socialhub.CallOption) (socialhub.Page[Event], error)
	SendText(context.Context, SendTextRequest, ...socialhub.CallOption) (*EventReference, error)
	SendMedia(context.Context, SendMediaRequest, ...socialhub.CallOption) (*EventReference, error)
	SendReaction(context.Context, ReactionEventRequest, ...socialhub.CallOption) (*EventReference, error)
	Redact(context.Context, string, string, string, ...socialhub.CallOption) (*EventReference, error)
}

// MediaWorkflow exposes Matrix's single-request content upload.
type MediaWorkflow interface {
	Upload(context.Context, UploadRequest, io.Reader, ...socialhub.CallOption) (*socialhub.Media, error)
}

// SyncWorkflow exposes incremental Matrix client sync.
type SyncWorkflow interface {
	Sync(context.Context, SyncRequest, ...socialhub.CallOption) (*SyncResponse, error)
}

type profileResponse struct {
	DisplayName string `json:"displayname"`
	AvatarURL   string `json:"avatar_url"`
}

type eventIDResponse struct {
	EventID string `json:"event_id"`
}

type roomMessagesResponse struct {
	Start string  `json:"start"`
	End   string  `json:"end"`
	Chunk []Event `json:"chunk"`
}

type relationsResponse struct {
	Chunk     []Event `json:"chunk"`
	NextBatch string  `json:"next_batch"`
	PrevBatch string  `json:"prev_batch"`
}

type uploadResponse struct {
	ContentURI string `json:"content_uri"`
}

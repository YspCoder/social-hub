package lark

import (
	"context"
	"encoding/json"

	"social-hub/pkg/socialhub"
)

// ReceiveIDType selects how SendRequest.ReceiveID is interpreted.
type ReceiveIDType string

const (
	ReceiveOpenID  ReceiveIDType = "open_id"
	ReceiveUnionID ReceiveIDType = "union_id"
	ReceiveUserID  ReceiveIDType = "user_id"
	ReceiveEmail   ReceiveIDType = "email"
	ReceiveChatID  ReceiveIDType = "chat_id"
)

// SendRequest preserves Lark's receiver and JSON content model.
type SendRequest struct {
	ReceiveIDType ReceiveIDType
	ReceiveID     string
	MessageType   string
	Content       json.RawMessage
}

// ReplyRequest replies to a message and can create a topic thread.
type ReplyRequest struct {
	MessageID     string
	MessageType   string
	Content       json.RawMessage
	ReplyInThread bool
}

// UpdateRequest edits a text or post message.
type UpdateRequest struct {
	MessageID   string
	MessageType string
	Content     json.RawMessage
}

// MessageWorkflow exposes raw Lark message types without reducing cards and
// rich posts to common text.
type MessageWorkflow interface {
	Send(context.Context, SendRequest, ...socialhub.CallOption) (*socialhub.Message, error)
	Reply(context.Context, ReplyRequest, ...socialhub.CallOption) (*socialhub.Message, error)
	Update(context.Context, UpdateRequest, ...socialhub.CallOption) (*socialhub.Message, error)
	Delete(context.Context, string, ...socialhub.CallOption) error
}

// Reaction is the platform reaction identifier returned by Lark.
type Reaction struct {
	ID        string
	MessageID string
	EmojiType string
	ActorID   string
}

// ReactionWorkflow exposes arbitrary Lark emoji and exact reaction-ID removal.
type ReactionWorkflow interface {
	AddReaction(context.Context, string, string, ...socialhub.CallOption) (*Reaction, error)
	DeleteReaction(context.Context, string, string, ...socialhub.CallOption) error
}

// EventPayload preserves a verified Open Platform event.
type EventPayload struct {
	Schema     string
	ID         string
	Type       string
	AppID      string
	TenantKey  string
	CreateTime string
	Challenge  string
	Message    *socialhub.Message
	Reaction   *ReactionEvent
	Raw        json.RawMessage
}

// ReactionEvent is emitted for reaction-created and reaction-deleted events.
type ReactionEvent struct {
	MessageID  string
	EmojiType  string
	ActorID    string
	ActionTime string
	Added      bool
}

var _ MessageWorkflow = (*Client)(nil)
var _ ReactionWorkflow = (*Client)(nil)

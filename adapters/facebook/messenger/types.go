package messenger

import (
	"context"
	"encoding/json"
	"time"

	"social-hub/pkg/socialhub"
)

// MessagingType declares why a Page is sending a message inside Meta's
// standard messaging window.
type MessagingType string

const (
	MessagingResponse MessagingType = "RESPONSE"
	MessagingUpdate   MessagingType = "UPDATE"
)

// AttachmentType identifies one supported Messenger attachment.
type AttachmentType string

const (
	AttachmentImage AttachmentType = "image"
	AttachmentAudio AttachmentType = "audio"
	AttachmentVideo AttachmentType = "video"
	AttachmentFile  AttachmentType = "file"
)

// TextMessageRequest sends text to one Page-scoped user.
type TextMessageRequest struct {
	RecipientID string
	Text        string
	Type        MessagingType
	ReplyToID   string
}

// AttachmentReference selects either a public HTTPS URL or an attachment ID
// previously returned by Meta.
type AttachmentReference struct {
	ID       string
	URL      string
	Reusable bool
}

// AttachmentMessageRequest sends one typed media or file attachment.
type AttachmentMessageRequest struct {
	RecipientID string
	Type        MessagingType
	Attachment  AttachmentType
	Reference   AttachmentReference
	ReplyToID   string
}

// SendResult identifies a message accepted by Messenger Platform.
type SendResult struct {
	RecipientID string `json:"recipient_id"`
	MessageID   string `json:"message_id"`
}

// MessageWorkflow exposes Messenger-specific outbound message operations.
type MessageWorkflow interface {
	SendText(context.Context, TextMessageRequest, ...socialhub.CallOption) (*socialhub.Message, error)
	SendAttachment(context.Context, AttachmentMessageRequest, ...socialhub.CallOption) (*socialhub.Message, error)
}

// UserProfile is the basic PSID-scoped profile available after Meta grants the
// Business Asset User Profile Access feature.
type UserProfile struct {
	ID         string `json:"id"`
	Name       string `json:"name,omitempty"`
	FirstName  string `json:"first_name,omitempty"`
	LastName   string `json:"last_name,omitempty"`
	ProfilePic string `json:"profile_pic,omitempty"`
}

// ProfileWorkflow reads the basic profile for one Page-scoped user.
type ProfileWorkflow interface {
	GetUserProfile(context.Context, string, ...socialhub.CallOption) (*UserProfile, error)
}

// EventParty identifies the sender or recipient of a Page messaging event.
type EventParty struct {
	ID string `json:"id"`
}

// QuickReply preserves the payload selected by an inbound quick reply.
type QuickReply struct {
	Payload string `json:"payload"`
}

// Attachment preserves an inbound Messenger attachment. Payload remains raw
// because its shape is selected by Type and evolves independently.
type Attachment struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// IncomingMessage is the documented Page messaging webhook message payload.
type IncomingMessage struct {
	ID          string       `json:"mid"`
	Text        string       `json:"text,omitempty"`
	IsEcho      bool         `json:"is_echo,omitempty"`
	QuickReply  *QuickReply  `json:"quick_reply,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
	ReplyTo     *struct {
		ID string `json:"mid"`
	} `json:"reply_to,omitempty"`
}

// DeliveryReceipt reports messages delivered through Messenger.
type DeliveryReceipt struct {
	MessageIDs []string `json:"mids,omitempty"`
	Watermark  int64    `json:"watermark"`
}

// ReadReceipt reports the latest timestamp read by a Page-scoped user.
type ReadReceipt struct {
	Watermark int64 `json:"watermark"`
}

// Postback reports a button, Get Started, or persistent-menu selection.
type Postback struct {
	MessageID string          `json:"mid,omitempty"`
	Title     string          `json:"title,omitempty"`
	Payload   string          `json:"payload,omitempty"`
	Referral  json.RawMessage `json:"referral,omitempty"`
}

// Reaction reports a user adding or removing a reaction from a message.
type Reaction struct {
	MessageID string `json:"mid"`
	Action    string `json:"action"`
	Reaction  string `json:"reaction,omitempty"`
	Emoji     string `json:"emoji,omitempty"`
}

// WebhookEvent preserves one Page messaging event and its normalized message,
// when the event contains one.
type WebhookEvent struct {
	PageID            string
	EntryTime         time.Time
	Sender            EventParty
	Recipient         EventParty
	Timestamp         time.Time
	Message           *IncomingMessage
	Delivery          *DeliveryReceipt
	Read              *ReadReceipt
	Postback          *Postback
	Reaction          *Reaction
	NormalizedMessage *socialhub.Message
	Raw               json.RawMessage
}

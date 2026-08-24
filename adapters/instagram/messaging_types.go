package instagram

import (
	"context"
	"encoding/json"
	"time"

	"social-hub/pkg/socialhub"
)

// MessageMediaType identifies an Instagram Messaging API attachment sent by
// public HTTPS URL.
type MessageMediaType string

const (
	MessageMediaImage MessageMediaType = "image"
	MessageMediaAudio MessageMediaType = "audio"
	MessageMediaVideo MessageMediaType = "video"
)

// MessageReactionAction selects whether a reaction is added or removed.
type MessageReactionAction string

const (
	MessageReactionAdd    MessageReactionAction = "react"
	MessageReactionRemove MessageReactionAction = "unreact"
)

// TextMessageRequest sends text to one Instagram-scoped user ID.
type TextMessageRequest struct {
	RecipientID string
	Text        string
}

// MediaMessageRequest sends one image, audio file, or video by public HTTPS
// URL.
type MediaMessageRequest struct {
	RecipientID string
	Type        MessageMediaType
	URL         string
}

// PublishedMediaMessageRequest shares media owned by the configured
// professional account.
type PublishedMediaMessageRequest struct {
	RecipientID string
	MediaID     string
}

// MessageReactionRequest adds or removes one reaction from a message.
type MessageReactionRequest struct {
	RecipientID string
	MessageID   string
	Action      MessageReactionAction
	Reaction    string
}

// SendResult identifies a message accepted by Instagram Messaging API.
type SendResult struct {
	RecipientID string `json:"recipient_id"`
	MessageID   string `json:"message_id"`
}

// ReactionResult identifies the recipient for an accepted reaction mutation.
type ReactionResult struct {
	RecipientID string `json:"recipient_id"`
}

// MessagingWorkflow exposes Instagram-specific outbound messaging operations.
type MessagingWorkflow interface {
	SendText(context.Context, TextMessageRequest, ...socialhub.CallOption) (*socialhub.Message, error)
	SendMedia(context.Context, MediaMessageRequest, ...socialhub.CallOption) (*socialhub.Message, error)
	SharePublishedMedia(context.Context, PublishedMediaMessageRequest, ...socialhub.CallOption) (*socialhub.Message, error)
	SendReaction(context.Context, MessageReactionRequest, ...socialhub.CallOption) (*ReactionResult, error)
}

// MessagingUserProfile is an IGSID-scoped profile available after the user
// establishes messaging consent.
type MessagingUserProfile struct {
	ID                  string `json:"id"`
	Name                string `json:"name,omitempty"`
	Username            string `json:"username,omitempty"`
	ProfilePictureURL   string `json:"profile_pic,omitempty"`
	FollowerCount       int64  `json:"follower_count,omitempty"`
	Verified            bool   `json:"is_verified_user,omitempty"`
	UserFollowsBusiness bool   `json:"is_user_follow_business,omitempty"`
	BusinessFollowsUser bool   `json:"is_business_follow_user,omitempty"`
}

// MessagingProfileWorkflow reads an IGSID-scoped profile.
type MessagingProfileWorkflow interface {
	GetMessagingUserProfile(context.Context, string, ...socialhub.CallOption) (*MessagingUserProfile, error)
}

type messageParty struct {
	ID       string `json:"id"`
	Username string `json:"username,omitempty"`
}

type messageRecipientList struct {
	Data []messageParty `json:"data"`
}

// MessageReplyReference identifies the original message in a reply webhook or
// message lookup response.
type MessageReplyReference struct {
	ID        string `json:"id,omitempty"`
	MessageID string `json:"mid,omitempty"`
}

type messageDetail struct {
	ID          string                 `json:"id"`
	CreatedTime string                 `json:"created_time"`
	From        messageParty           `json:"from"`
	To          messageRecipientList   `json:"to"`
	Message     string                 `json:"message"`
	ReplyTo     *MessageReplyReference `json:"reply_to,omitempty"`
}

// WebhookParty identifies one party in an Instagram messaging callback.
type WebhookParty struct {
	ID string `json:"id"`
}

// WebhookAttachment preserves an inbound attachment payload selected by Type.
type WebhookAttachment struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// IncomingMessage is the documented Instagram messaging webhook payload.
type IncomingMessage struct {
	ID          string                 `json:"mid"`
	Text        string                 `json:"text,omitempty"`
	IsEcho      bool                   `json:"is_echo,omitempty"`
	Attachments []WebhookAttachment    `json:"attachments,omitempty"`
	ReplyTo     *MessageReplyReference `json:"reply_to,omitempty"`
}

// ReadReceipt identifies the message read by the Instagram user.
type ReadReceipt struct {
	MessageID string `json:"mid"`
}

// MessagePostback preserves an icebreaker or menu selection.
type MessagePostback struct {
	MessageID string          `json:"mid,omitempty"`
	Title     string          `json:"title,omitempty"`
	Payload   string          `json:"payload,omitempty"`
	Referral  json.RawMessage `json:"referral,omitempty"`
}

// MessageReaction preserves an inbound reaction mutation.
type MessageReaction struct {
	MessageID string `json:"mid"`
	Action    string `json:"action"`
	Reaction  string `json:"reaction,omitempty"`
	Emoji     string `json:"emoji,omitempty"`
}

// MessagingWebhookEvent preserves one Instagram messaging event and its
// normalized message when applicable.
type MessagingWebhookEvent struct {
	InstagramUserID   string
	EntryTime         time.Time
	Sender            WebhookParty
	Recipient         WebhookParty
	Timestamp         time.Time
	Message           *IncomingMessage
	Read              *ReadReceipt
	Postback          *MessagePostback
	Reaction          *MessageReaction
	Referral          json.RawMessage
	NormalizedMessage *socialhub.Message
	Raw               json.RawMessage
}

package microsoftteams

import (
	"context"
	"encoding/json"
	"time"

	"social-hub/pkg/socialhub"
)

// TargetKind distinguishes a chat from a team channel.
type TargetKind string

const (
	TargetChat    TargetKind = "chat"
	TargetChannel TargetKind = "channel"
)

// Target locates a Teams conversation.
type Target struct {
	Kind      TargetKind
	ChatID    string
	TeamID    string
	ChannelID string
}

// MessageRef locates either a root message or a reply.
type MessageRef struct {
	Target  Target
	RootID  string
	ReplyID string
}

// MessageBody preserves Graph's text or HTML body contract.
type MessageBody struct {
	ContentType string `json:"contentType"`
	Content     string `json:"content"`
}

// HostedContent is inline message content, not a SharePoint or OneDrive file.
type HostedContent struct {
	TemporaryID  string `json:"@microsoft.graph.temporaryId,omitempty"`
	ID           string `json:"id,omitempty"`
	ContentBytes []byte `json:"contentBytes,omitempty"`
	ContentType  string `json:"contentType,omitempty"`
}

// Attachment preserves a Graph chatMessage attachment.
type Attachment struct {
	ID           string `json:"id,omitempty"`
	ContentType  string `json:"contentType,omitempty"`
	ContentURL   string `json:"contentUrl,omitempty"`
	Content      string `json:"content,omitempty"`
	Name         string `json:"name,omitempty"`
	ThumbnailURL string `json:"thumbnailUrl,omitempty"`
}

// Identity is a Teams user or application identity.
type Identity struct {
	ID               string `json:"id,omitempty"`
	DisplayName      string `json:"displayName,omitempty"`
	IdentityProvider string `json:"identityProvider,omitempty"`
	UserIdentityType string `json:"userIdentityType,omitempty"`
}

// IdentitySet identifies the user or application responsible for an action.
type IdentitySet struct {
	User        *Identity `json:"user,omitempty"`
	Application *Identity `json:"application,omitempty"`
}

// ChannelIdentity locates a channel message.
type ChannelIdentity struct {
	TeamID    string `json:"teamId,omitempty"`
	ChannelID string `json:"channelId,omitempty"`
}

// Reaction preserves Graph reaction type and actor details.
type Reaction struct {
	ReactionType       string      `json:"reactionType"`
	CreatedDateTime    *time.Time  `json:"createdDateTime,omitempty"`
	DisplayName        string      `json:"displayName,omitempty"`
	ReactionContentURL string      `json:"reactionContentUrl,omitempty"`
	User               IdentitySet `json:"user,omitempty"`
}

// ChatMessage is the typed Microsoft Graph v1.0 chatMessage representation.
type ChatMessage struct {
	ID                   string           `json:"id"`
	ReplyToID            string           `json:"replyToId,omitempty"`
	ETag                 string           `json:"etag,omitempty"`
	MessageType          string           `json:"messageType,omitempty"`
	CreatedDateTime      *time.Time       `json:"createdDateTime,omitempty"`
	LastModifiedDateTime *time.Time       `json:"lastModifiedDateTime,omitempty"`
	LastEditedDateTime   *time.Time       `json:"lastEditedDateTime,omitempty"`
	DeletedDateTime      *time.Time       `json:"deletedDateTime,omitempty"`
	Subject              string           `json:"subject,omitempty"`
	Summary              string           `json:"summary,omitempty"`
	ChatID               string           `json:"chatId,omitempty"`
	Importance           string           `json:"importance,omitempty"`
	Locale               string           `json:"locale,omitempty"`
	WebURL               string           `json:"webUrl,omitempty"`
	From                 *IdentitySet     `json:"from,omitempty"`
	Body                 MessageBody      `json:"body"`
	ChannelIdentity      *ChannelIdentity `json:"channelIdentity,omitempty"`
	Attachments          []Attachment     `json:"attachments,omitempty"`
	HostedContents       []HostedContent  `json:"hostedContents,omitempty"`
	Reactions            []Reaction       `json:"reactions,omitempty"`
	Raw                  json.RawMessage  `json:"-"`
}

// SendRequest sends a root message to a chat or channel.
type SendRequest struct {
	Target         Target
	Body           MessageBody
	Subject        string
	Importance     string
	Attachments    []Attachment
	HostedContents []HostedContent
}

// ReplyRequest sends a reply under a root message.
type ReplyRequest struct {
	Parent         MessageRef
	Body           MessageBody
	Attachments    []Attachment
	HostedContents []HostedContent
}

// UpdateRequest edits a normal message body.
type UpdateRequest struct {
	Message MessageRef
	Body    MessageBody
}

// ListMessagesRequest retrieves root messages for a target.
type ListMessagesRequest struct {
	Target     Target
	Cursor     string
	MaxResults int
}

// ListRepliesRequest retrieves replies under one root message.
type ListRepliesRequest struct {
	Parent     MessageRef
	Cursor     string
	MaxResults int
}

// MessagePage is a typed cursor page of Graph messages.
type MessagePage struct {
	Items      []ChatMessage
	NextCursor string
	HasMore    bool
}

// MessageWorkflow exposes Graph message operations without flattening HTML or hosted content.
type MessageWorkflow interface {
	Send(context.Context, SendRequest, ...socialhub.CallOption) (*ChatMessage, error)
	Reply(context.Context, ReplyRequest, ...socialhub.CallOption) (*ChatMessage, error)
	Get(context.Context, MessageRef, ...socialhub.CallOption) (*ChatMessage, error)
	List(context.Context, ListMessagesRequest, ...socialhub.CallOption) (MessagePage, error)
	ListReplies(context.Context, ListRepliesRequest, ...socialhub.CallOption) (MessagePage, error)
	Update(context.Context, UpdateRequest, ...socialhub.CallOption) (*ChatMessage, error)
	SoftDelete(context.Context, MessageRef, ...socialhub.CallOption) error
}

// ReactionWorkflow exposes arbitrary Unicode and Graph compatibility reactions.
type ReactionWorkflow interface {
	SetReaction(context.Context, MessageRef, string, ...socialhub.CallOption) error
	UnsetReaction(context.Context, MessageRef, string, ...socialhub.CallOption) error
}

// Subscription is a basic, unencrypted Microsoft Graph change subscription.
type Subscription struct {
	ID                       string     `json:"id"`
	Resource                 string     `json:"resource,omitempty"`
	ChangeType               string     `json:"changeType,omitempty"`
	NotificationURL          string     `json:"notificationUrl,omitempty"`
	LifecycleNotificationURL string     `json:"lifecycleNotificationUrl,omitempty"`
	ExpirationDateTime       *time.Time `json:"expirationDateTime,omitempty"`
	ClientState              string     `json:"clientState,omitempty"`
	IncludeResourceData      bool       `json:"includeResourceData"`
}

// CreateSubscriptionRequest creates a basic change notification subscription.
type CreateSubscriptionRequest struct {
	Resource                 string
	ChangeTypes              []string
	NotificationURL          string
	LifecycleNotificationURL string
	ExpirationDateTime       time.Time
}

// SubscriptionWorkflow manages basic notifications without encrypted resource data.
type SubscriptionWorkflow interface {
	CreateSubscription(context.Context, CreateSubscriptionRequest, ...socialhub.CallOption) (*Subscription, error)
	RenewSubscription(context.Context, string, time.Time, ...socialhub.CallOption) (*Subscription, error)
	DeleteSubscription(context.Context, string, ...socialhub.CallOption) error
}

// ResourceData preserves the lightweight identity included in basic notifications.
type ResourceData struct {
	ODataType string `json:"@odata.type,omitempty"`
	ODataID   string `json:"@odata.id,omitempty"`
	ID        string `json:"id,omitempty"`
}

// Notification is one verified basic Microsoft Graph change notification.
type Notification struct {
	ID                             string          `json:"id,omitempty"`
	SubscriptionID                 string          `json:"subscriptionId"`
	SubscriptionExpirationDateTime *time.Time      `json:"subscriptionExpirationDateTime,omitempty"`
	ClientState                    string          `json:"clientState"`
	ChangeType                     string          `json:"changeType"`
	Resource                       string          `json:"resource"`
	TenantID                       string          `json:"tenantId,omitempty"`
	ResourceData                   ResourceData    `json:"resourceData,omitempty"`
	Raw                            json.RawMessage `json:"-"`
}

var _ MessageWorkflow = (*Client)(nil)
var _ ReactionWorkflow = (*Client)(nil)
var _ SubscriptionWorkflow = (*Client)(nil)

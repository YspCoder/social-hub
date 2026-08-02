package zalo

import (
	"context"
	"encoding/json"
	"time"

	"social-hub/pkg/socialhub"
)

// MessageQuota describes the quota bucket selected for an accepted message.
type MessageQuota struct {
	Type        string `json:"quota_type"`
	Remain      string `json:"remain,omitempty"`
	Total       string `json:"total,omitempty"`
	ExpiredDate string `json:"expired_date,omitempty"`
	OwnerType   string `json:"owner_type,omitempty"`
	OwnerID     string `json:"owner_id,omitempty"`
}

// SendResult identifies an accepted Zalo consultation message.
type SendResult struct {
	MessageID string        `json:"message_id"`
	UserID    string        `json:"user_id"`
	SentTime  string        `json:"sent_time"`
	Quota     *MessageQuota `json:"quota,omitempty"`
}

// MessageWorkflow exposes the OA v3 consultation text-message operation.
type MessageWorkflow interface {
	SendConsultationText(context.Context, string, string, ...socialhub.CallOption) (*SendResult, error)
}

// OAProfile describes the Official Account represented by the access token.
type OAProfile struct {
	ID                   string `json:"oaid"`
	Name                 string `json:"name"`
	Description          string `json:"description,omitempty"`
	Alias                string `json:"oa_alias,omitempty"`
	Verified             bool   `json:"is_verified"`
	Type                 int    `json:"oa_type"`
	CategoryName         string `json:"cate_name,omitempty"`
	FollowerCount        int64  `json:"num_follower,omitempty"`
	Avatar               string `json:"avatar,omitempty"`
	Cover                string `json:"cover,omitempty"`
	PackageName          string `json:"package_name,omitempty"`
	PackageValidThrough  string `json:"package_valid_through_date,omitempty"`
	PackageAutoRenewDate string `json:"package_auto_renew_date,omitempty"`
	LinkedZCA            string `json:"linked_ZCA,omitempty"`
}

// TagsAndNotes contains OA-owned CRM annotations for a user.
type TagsAndNotes struct {
	Notes    []string `json:"notes,omitempty"`
	TagNames []string `json:"tag_names,omitempty"`
}

// SharedUserInfo contains fields the user explicitly shared with the OA.
type SharedUserInfo struct {
	Address   string          `json:"address,omitempty"`
	City      string          `json:"city,omitempty"`
	District  string          `json:"district,omitempty"`
	Phone     json.RawMessage `json:"phone,omitempty"`
	Name      string          `json:"name,omitempty"`
	BirthDate string          `json:"user_dob,omitempty"`
}

// UserProfile is the OA-scoped profile returned by Zalo's management API.
type UserProfile struct {
	UserID              string            `json:"user_id"`
	UserIDByApp         string            `json:"user_id_by_app,omitempty"`
	ExternalID          string            `json:"user_external_id,omitempty"`
	DisplayName         string            `json:"display_name,omitempty"`
	Alias               string            `json:"user_alias,omitempty"`
	Sensitive           bool              `json:"is_sensitive"`
	LastInteractionDate string            `json:"user_last_interaction_date,omitempty"`
	Follower            bool              `json:"user_is_follower"`
	Avatar              string            `json:"avatar,omitempty"`
	Avatars             map[string]string `json:"avatars,omitempty"`
	DynamicParam        string            `json:"dynamic_param,omitempty"`
	TagsAndNotes        TagsAndNotes      `json:"tags_and_notes_info,omitempty"`
	SharedInfo          SharedUserInfo    `json:"shared_info,omitempty"`
}

// ProfileWorkflow reads the configured OA and OA-scoped user profiles.
type ProfileWorkflow interface {
	GetOA(context.Context, ...socialhub.CallOption) (*OAProfile, error)
	GetUserProfile(context.Context, string, ...socialhub.CallOption) (*UserProfile, error)
}

// Attachment is one inbound Zalo message attachment. Payload remains raw
// because its schema is selected by Type and evolves independently.
type Attachment struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// IncomingMessage preserves common inbound and outbound webhook message fields.
type IncomingMessage struct {
	ID          string       `json:"msg_id"`
	Text        string       `json:"text,omitempty"`
	QuoteMsgID  string       `json:"quote_msg_id,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

// EventParty identifies the sender or recipient of a webhook event.
type EventParty struct {
	ID string `json:"id"`
}

// WebhookEvent is the typed payload retained inside socialhub.Event.
type WebhookEvent struct {
	AppID             string
	EventName         string
	Sender            EventParty
	Recipient         EventParty
	UserIDByApp       string
	Timestamp         time.Time
	RetryCount        int
	Message           *IncomingMessage
	NormalizedMessage *socialhub.Message
	Raw               json.RawMessage
}

var _ MessageWorkflow = (*Client)(nil)
var _ ProfileWorkflow = (*Client)(nil)

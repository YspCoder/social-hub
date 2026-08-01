package whatsapp

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"social-hub/pkg/socialhub"
)

// MediaKind identifies the media object in a WhatsApp message.
type MediaKind string

const (
	MediaImage    MediaKind = "image"
	MediaVideo    MediaKind = "video"
	MediaAudio    MediaKind = "audio"
	MediaDocument MediaKind = "document"
	MediaSticker  MediaKind = "sticker"
)

// MediaReference selects either an uploaded media ID or a public HTTPS URL.
type MediaReference struct {
	ID   string `json:"id,omitempty"`
	Link string `json:"link,omitempty"`
}

// MediaMessageRequest sends one typed media message.
type MediaMessageRequest struct {
	To        string
	Type      MediaKind
	Media     MediaReference
	Caption   string
	Filename  string
	ReplyToID string
}

// TemplateComponent preserves an evolving WhatsApp template component while
// retaining typed component placement.
type TemplateComponent struct {
	Type       string            `json:"type"`
	SubType    string            `json:"sub_type,omitempty"`
	Index      string            `json:"index,omitempty"`
	Parameters []json.RawMessage `json:"parameters,omitempty"`
}

// TemplateMessageRequest sends one approved message template.
type TemplateMessageRequest struct {
	To           string
	Name         string
	LanguageCode string
	Components   []TemplateComponent
	ReplyToID    string
}

// MessageWorkflow exposes WhatsApp-specific outbound message actions.
type MessageWorkflow interface {
	SendMedia(context.Context, MediaMessageRequest, ...socialhub.CallOption) (*socialhub.Message, error)
	SendTemplate(context.Context, TemplateMessageRequest, ...socialhub.CallOption) (*socialhub.Message, error)
	SendReaction(context.Context, string, string, string, ...socialhub.CallOption) (*socialhub.Message, error)
	MarkRead(context.Context, string, ...socialhub.CallOption) error
}

// MediaUploadRequest streams one media object to the Cloud API.
type MediaUploadRequest struct {
	Filename string
	MIME     string
	Size     int64
	Reader   io.Reader
}

// MediaInfo describes an encrypted Cloud API media object.
type MediaInfo struct {
	ID               string `json:"id"`
	URL              string `json:"url"`
	MIME             string `json:"mime_type"`
	SHA256           string `json:"sha256"`
	Size             int64  `json:"file_size"`
	MessagingProduct string `json:"messaging_product"`
}

// MediaWorkflow exposes direct media upload and metadata lifecycle.
type MediaWorkflow interface {
	UploadMedia(context.Context, MediaUploadRequest, ...socialhub.CallOption) (*MediaInfo, error)
	GetMedia(context.Context, string, ...socialhub.CallOption) (*MediaInfo, error)
	DeleteMedia(context.Context, string, ...socialhub.CallOption) error
}

// BusinessProfile is the public profile attached to a business phone number.
type BusinessProfile struct {
	MessagingProduct  string   `json:"messaging_product,omitempty"`
	About             string   `json:"about,omitempty"`
	Address           string   `json:"address,omitempty"`
	Description       string   `json:"description,omitempty"`
	Email             string   `json:"email,omitempty"`
	ProfilePictureURL string   `json:"profile_picture_url,omitempty"`
	Websites          []string `json:"websites,omitempty"`
	Vertical          string   `json:"vertical,omitempty"`
}

// BusinessProfileUpdate uses pointers to distinguish omitted fields from
// fields intentionally cleared by the caller.
type BusinessProfileUpdate struct {
	About                *string
	Address              *string
	Description          *string
	Email                *string
	ProfilePictureHandle *string
	Websites             *[]string
	Vertical             *string
}

// BusinessProfileWorkflow reads and updates one phone-number profile.
type BusinessProfileWorkflow interface {
	GetBusinessProfile(context.Context, ...socialhub.CallOption) (*BusinessProfile, error)
	UpdateBusinessProfile(context.Context, BusinessProfileUpdate, ...socialhub.CallOption) error
}

// WebhookMetadata identifies the business phone number that received a change.
type WebhookMetadata struct {
	DisplayPhoneNumber string `json:"display_phone_number"`
	PhoneNumberID      string `json:"phone_number_id"`
}

// WebhookContact is the sender profile included beside inbound messages.
type WebhookContact struct {
	WAID string `json:"wa_id"`
	Name string
}

// MessageWebhookPayload preserves one inbound message and its raw object.
type MessageWebhookPayload struct {
	BusinessAccountID string
	Metadata          WebhookMetadata
	Contact           *WebhookContact
	From              string
	ID                string
	Timestamp         *time.Time
	Type              string
	ReplyToID         string
	Raw               json.RawMessage
}

// StatusWebhookPayload preserves one outbound-delivery status and raw object.
type StatusWebhookPayload struct {
	BusinessAccountID string
	Metadata          WebhookMetadata
	ID                string
	Status            string
	RecipientID       string
	Timestamp         *time.Time
	Conversation      json.RawMessage
	Pricing           json.RawMessage
	Errors            json.RawMessage
	Raw               json.RawMessage
}

var _ MessageWorkflow = (*Client)(nil)
var _ MediaWorkflow = (*Client)(nil)
var _ BusinessProfileWorkflow = (*Client)(nil)

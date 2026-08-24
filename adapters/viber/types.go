package viber

import (
	"context"
	"encoding/json"
	"time"

	"social-hub/pkg/socialhub"
)

// APIStatus is returned by every Viber REST Bot API operation.
type APIStatus struct {
	Status        int    `json:"status"`
	StatusMessage string `json:"status_message"`
}

func (s APIStatus) viberStatus() (int, string) { return s.Status, s.StatusMessage }

type statusCarrier interface {
	viberStatus() (int, string)
}

// Sender is the configured Bot identity attached to outbound messages.
type Sender struct {
	Name   string `json:"name"`
	Avatar string `json:"avatar,omitempty"`
}

// MessageObject is the closed set of outbound message types supported by this
// adapter. Use TextMessage, PictureMessage, VideoMessage, FileMessage,
// ContactMessage, LocationMessage, URLMessage, or StickerMessage.
type MessageObject interface {
	viberMessage() (map[string]any, error)
}

// TextMessage sends formatted Viber text.
type TextMessage struct {
	Text string
}

// PictureMessage sends a caller-hosted image and optional thumbnail.
type PictureMessage struct {
	Text         string
	MediaURL     string
	ThumbnailURL string
}

// VideoMessage sends a caller-hosted MP4/H264 video.
type VideoMessage struct {
	MediaURL     string
	ThumbnailURL string
	Size         int64
	Duration     time.Duration
}

// FileMessage sends a caller-hosted document.
type FileMessage struct {
	MediaURL string
	Size     int64
	Filename string
}

// ContactMessage sends one name and phone-number pair.
type ContactMessage struct {
	Name        string
	PhoneNumber string
}

// LocationMessage sends geographic coordinates.
type LocationMessage struct {
	Latitude  float64
	Longitude float64
}

// URLMessage sends a URL or an animated GIF URL.
type URLMessage struct {
	URL string
}

// StickerMessage sends a documented Viber sticker ID.
type StickerMessage struct {
	StickerID int64
}

// SendRequest sends one typed message to a subscribed user.
type SendRequest struct {
	Receiver      string
	Message       MessageObject
	TrackingData  string
	MinAPIVersion int
}

// BroadcastRequest sends one typed message to as many as 300 subscribers.
type BroadcastRequest struct {
	Receivers     []string
	Message       MessageObject
	TrackingData  string
	MinAPIVersion int
}

// FailedRecipient preserves one per-user broadcast failure.
type FailedRecipient struct {
	Receiver      string `json:"receiver"`
	Status        int    `json:"status"`
	StatusMessage string `json:"status_message"`
}

// SendResult identifies an accepted send and its billing category.
type SendResult struct {
	APIStatus
	MessageToken  json.Number       `json:"message_token"`
	ChatHostname  string            `json:"chat_hostname,omitempty"`
	BillingStatus int               `json:"billing_status,omitempty"`
	FailedList    []FailedRecipient `json:"failed_list,omitempty"`
}

// MessageWorkflow exposes Viber-specific typed send and broadcast operations.
type MessageWorkflow interface {
	Send(context.Context, SendRequest, ...socialhub.CallOption) (*SendResult, error)
	Broadcast(context.Context, BroadcastRequest, ...socialhub.CallOption) (*SendResult, error)
}

// Location contains Viber account or message coordinates.
type Location struct {
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lon"`
}

// Member is a legacy Public Chat member returned by account info.
type Member struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar,omitempty"`
	Role   string `json:"role,omitempty"`
}

// AccountInfo describes the configured Bot account.
type AccountInfo struct {
	APIStatus
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	URI              string    `json:"uri"`
	Icon             string    `json:"icon,omitempty"`
	Background       string    `json:"background,omitempty"`
	Category         string    `json:"category,omitempty"`
	Subcategory      string    `json:"subcategory,omitempty"`
	Location         *Location `json:"location,omitempty"`
	Country          string    `json:"country,omitempty"`
	Webhook          string    `json:"webhook,omitempty"`
	EventTypes       []string  `json:"event_types,omitempty"`
	SubscribersCount int64     `json:"subscribers_count,omitempty"`
	Members          []Member  `json:"members,omitempty"`
}

// UserDetails contains the profile fields Viber discloses for a subscriber.
type UserDetails struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Avatar          string `json:"avatar,omitempty"`
	Country         string `json:"country,omitempty"`
	Language        string `json:"language,omitempty"`
	PrimaryDeviceOS string `json:"primary_device_os,omitempty"`
	APIVersion      int    `json:"api_version,omitempty"`
	ViberVersion    string `json:"viber_version,omitempty"`
	MCC             int    `json:"mcc,omitempty"`
	MNC             int    `json:"mnc,omitempty"`
	DeviceType      string `json:"device_type,omitempty"`
}

type userDetailsResponse struct {
	APIStatus
	MessageToken json.Number `json:"message_token"`
	User         UserDetails `json:"user"`
}

// OnlineState is Viber's subscriber presence code.
type OnlineState int

const (
	Online            OnlineState = 0
	Offline           OnlineState = 1
	OnlineUndisclosed OnlineState = 2
	OnlineTryLater    OnlineState = 3
	OnlineUnavailable OnlineState = 4
)

// OnlineStatus describes one subscriber's current presence state.
type OnlineStatus struct {
	ID               string      `json:"id"`
	State            OnlineState `json:"online_status"`
	StatusMessage    string      `json:"online_status_message,omitempty"`
	LastOnlineMillis int64       `json:"last_online,omitempty"`
}

type onlineResponse struct {
	APIStatus
	Users []OnlineStatus `json:"users"`
}

// AccountWorkflow reads account, subscriber, and presence data.
type AccountWorkflow interface {
	GetAccountInfo(context.Context, ...socialhub.CallOption) (*AccountInfo, error)
	GetUserDetails(context.Context, string, ...socialhub.CallOption) (*UserDetails, error)
	GetOnline(context.Context, []string, ...socialhub.CallOption) ([]OnlineStatus, error)
}

// WebhookEventType is accepted by set_webhook event filtering.
type WebhookEventType string

const (
	WebhookDelivered           WebhookEventType = "delivered"
	WebhookSeen                WebhookEventType = "seen"
	WebhookFailed              WebhookEventType = "failed"
	WebhookSubscribed          WebhookEventType = "subscribed"
	WebhookUnsubscribed        WebhookEventType = "unsubscribed"
	WebhookConversationStarted WebhookEventType = "conversation_started"
	WebhookMessage             WebhookEventType = "message"
)

// SetWebhookRequest configures callbacks. A nil EventTypes slice requests all
// events; a non-nil empty slice requests only Viber's mandatory events.
type SetWebhookRequest struct {
	URL        string
	EventTypes []WebhookEventType
	SendName   *bool
	SendPhoto  *bool
}

// WebhookResult reports the event filter accepted by Viber.
type WebhookResult struct {
	APIStatus
	EventTypes []WebhookEventType `json:"event_types,omitempty"`
}

// WebhookWorkflow manages the callback endpoint for the configured bot.
type WebhookWorkflow interface {
	SetWebhook(context.Context, SetWebhookRequest, ...socialhub.CallOption) (*WebhookResult, error)
	RemoveWebhook(context.Context, ...socialhub.CallOption) error
}

// Contact is embedded in contact-type inbound messages.
type Contact struct {
	Name        string `json:"name"`
	PhoneNumber string `json:"phone_number"`
	Avatar      string `json:"avatar,omitempty"`
}

// IncomingMessage preserves Viber's inbound message union.
type IncomingMessage struct {
	Type         string    `json:"type"`
	Text         string    `json:"text,omitempty"`
	Media        string    `json:"media,omitempty"`
	Location     *Location `json:"location,omitempty"`
	Contact      *Contact  `json:"contact,omitempty"`
	TrackingData string    `json:"tracking_data,omitempty"`
	Filename     string    `json:"file_name,omitempty"`
	FileSize     int64     `json:"file_size,omitempty"`
	Duration     int64     `json:"duration,omitempty"`
	StickerID    int64     `json:"sticker_id,omitempty"`
}

// WebhookEvent is the typed callback payload retained inside socialhub.Event.
type WebhookEvent struct {
	Event             string
	Timestamp         time.Time
	MessageToken      string
	UserID            string
	User              *UserDetails
	Sender            *UserDetails
	Message           *IncomingMessage
	NormalizedMessage *socialhub.Message
	ConversationType  string
	Context           string
	Subscribed        *bool
	Description       string
	Raw               json.RawMessage
}

var _ MessageWorkflow = (*Client)(nil)
var _ AccountWorkflow = (*Client)(nil)
var _ WebhookWorkflow = (*Client)(nil)

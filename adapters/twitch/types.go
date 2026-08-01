package twitch

import (
	"context"
	"encoding/json"
	"time"

	"social-hub/pkg/socialhub"
)

// Stream is one active Twitch broadcast.
type Stream struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	UserLogin    string    `json:"user_login"`
	UserName     string    `json:"user_name"`
	GameID       string    `json:"game_id"`
	GameName     string    `json:"game_name"`
	Type         string    `json:"type"`
	Title        string    `json:"title"`
	Tags         []string  `json:"tags"`
	ViewerCount  int64     `json:"viewer_count"`
	StartedAt    time.Time `json:"started_at"`
	Language     string    `json:"language"`
	ThumbnailURL string    `json:"thumbnail_url"`
	Mature       bool      `json:"is_mature"`
}

// StreamRequest filters active streams.
type StreamRequest struct {
	UserIDs    []string
	UserLogins []string
	GameIDs    []string
	Languages  []string
	Cursor     string
	MaxResults int
}

// Channel describes broadcaster-controlled channel metadata.
type Channel struct {
	BroadcasterID       string   `json:"broadcaster_id"`
	BroadcasterLogin    string   `json:"broadcaster_login"`
	BroadcasterName     string   `json:"broadcaster_name"`
	BroadcasterLanguage string   `json:"broadcaster_language"`
	GameID              string   `json:"game_id"`
	GameName            string   `json:"game_name"`
	Title               string   `json:"title"`
	Delay               int      `json:"delay"`
	Tags                []string `json:"tags"`
	ContentLabels       []string `json:"content_classification_labels"`
	BrandedContent      bool     `json:"is_branded_content"`
}

// Clip is a published Twitch clip.
type Clip struct {
	ID              string
	URL             string
	EmbedURL        string
	BroadcasterID   string
	BroadcasterName string
	CreatorID       string
	CreatorName     string
	VideoID         string
	GameID          string
	Language        string
	Title           string
	ViewCount       int64
	CreatedAt       time.Time
	ThumbnailURL    string
	Duration        time.Duration
	VODOffset       *int64
	Featured        bool
}

// ClipRequest selects clips by exactly one Twitch dimension.
type ClipRequest struct {
	IDs           []string
	BroadcasterID string
	GameID        string
	StartedAt     *time.Time
	EndedAt       *time.Time
	Featured      *bool
	Cursor        string
	MaxResults    int
}

// ClipCreation identifies an asynchronously-created clip.
type ClipCreation struct {
	ID      string
	EditURL string
}

// ScheduleSegment is one broadcaster schedule entry.
type ScheduleSegment struct {
	ID           string
	StartTime    time.Time
	EndTime      time.Time
	Title        string
	CanceledAt   *time.Time
	CategoryID   string
	CategoryName string
	Recurring    bool
}

// ScheduleVacation describes a channel vacation window.
type ScheduleVacation struct {
	StartTime time.Time
	EndTime   time.Time
}

// Schedule contains one broadcaster schedule page.
type Schedule struct {
	BroadcasterID    string
	BroadcasterLogin string
	BroadcasterName  string
	Segments         []ScheduleSegment
	Vacation         *ScheduleVacation
	NextCursor       *string
}

// LiveWorkflow exposes Twitch-specific live discovery and clip operations.
type LiveWorkflow interface {
	ListStreams(context.Context, StreamRequest, ...socialhub.CallOption) (socialhub.Page[Stream], error)
	GetChannel(context.Context, string, ...socialhub.CallOption) (*Channel, error)
	GetSchedule(context.Context, string, string, int, ...socialhub.CallOption) (*Schedule, error)
	ListClips(context.Context, ClipRequest, ...socialhub.CallOption) (socialhub.Page[Clip], error)
	CreateClip(context.Context, string, bool, ...socialhub.CallOption) (*ClipCreation, error)
}

// EventSubTransport is the response representation of an EventSub transport.
type EventSubTransport struct {
	Method         string `json:"method"`
	Callback       string `json:"callback,omitempty"`
	SessionID      string `json:"session_id,omitempty"`
	ConnectedAt    string `json:"connected_at,omitempty"`
	DisconnectedAt string `json:"disconnected_at,omitempty"`
}

// EventSubSubscription describes one Twitch event subscription.
type EventSubSubscription struct {
	ID        string            `json:"id"`
	Status    string            `json:"status"`
	Type      string            `json:"type"`
	Version   string            `json:"version"`
	Cost      int               `json:"cost"`
	Condition map[string]string `json:"condition"`
	Transport EventSubTransport `json:"transport"`
	CreatedAt time.Time         `json:"created_at"`
}

// EventSubPage reports subscriptions and their aggregate cost budget.
type EventSubPage struct {
	Items        []EventSubSubscription
	Total        int
	TotalCost    int
	MaxTotalCost int
	NextCursor   *string
}

// EventSubListRequest filters subscriptions. Type and Status are exclusive.
type EventSubListRequest struct {
	Type       string
	Status     string
	Cursor     string
	MaxResults int
}

// EventSubPayload preserves typed subscription metadata and raw event data.
type EventSubPayload struct {
	Subscription EventSubSubscription
	Event        json.RawMessage
	Challenge    string
	Raw          json.RawMessage
}

// EventSubWorkflow manages webhook subscriptions using an app access token.
type EventSubWorkflow interface {
	CreateWebhookSubscription(context.Context, string, string, map[string]string, string, ...socialhub.CallOption) (*EventSubPage, error)
	ListSubscriptions(context.Context, EventSubListRequest, ...socialhub.CallOption) (*EventSubPage, error)
	DeleteSubscription(context.Context, string, ...socialhub.CallOption) error
}

type twitchUser struct {
	ID              string          `json:"id"`
	Login           string          `json:"login"`
	DisplayName     string          `json:"display_name"`
	Type            string          `json:"type"`
	BroadcasterType string          `json:"broadcaster_type"`
	Description     string          `json:"description"`
	ProfileImageURL string          `json:"profile_image_url"`
	OfflineImageURL string          `json:"offline_image_url"`
	CreatedAt       time.Time       `json:"created_at"`
	Raw             json.RawMessage `json:"-"`
}

type twitchVideo struct {
	ID            string          `json:"id"`
	StreamID      string          `json:"stream_id"`
	UserID        string          `json:"user_id"`
	UserLogin     string          `json:"user_login"`
	UserName      string          `json:"user_name"`
	Title         string          `json:"title"`
	Description   string          `json:"description"`
	CreatedAt     time.Time       `json:"created_at"`
	PublishedAt   time.Time       `json:"published_at"`
	URL           string          `json:"url"`
	ThumbnailURL  string          `json:"thumbnail_url"`
	Viewable      string          `json:"viewable"`
	ViewCount     int64           `json:"view_count"`
	Language      string          `json:"language"`
	Type          string          `json:"type"`
	Duration      string          `json:"duration"`
	MutedSegments json.RawMessage `json:"muted_segments"`
}

type pagination struct {
	Cursor string `json:"cursor"`
}

type userPage struct {
	Data []twitchUser `json:"data"`
}

type videoPage struct {
	Data       []twitchVideo `json:"data"`
	Pagination pagination    `json:"pagination"`
}

type streamPage struct {
	Data       []Stream   `json:"data"`
	Pagination pagination `json:"pagination"`
}

type channelPage struct {
	Data []Channel `json:"data"`
}

type clipPage struct {
	Data       []clipWire `json:"data"`
	Pagination pagination `json:"pagination"`
}

type clipWire struct {
	ID              string    `json:"id"`
	URL             string    `json:"url"`
	EmbedURL        string    `json:"embed_url"`
	BroadcasterID   string    `json:"broadcaster_id"`
	BroadcasterName string    `json:"broadcaster_name"`
	CreatorID       string    `json:"creator_id"`
	CreatorName     string    `json:"creator_name"`
	VideoID         string    `json:"video_id"`
	GameID          string    `json:"game_id"`
	Language        string    `json:"language"`
	Title           string    `json:"title"`
	ViewCount       int64     `json:"view_count"`
	CreatedAt       time.Time `json:"created_at"`
	ThumbnailURL    string    `json:"thumbnail_url"`
	Duration        float64   `json:"duration"`
	VODOffset       *int64    `json:"vod_offset"`
	Featured        bool      `json:"is_featured"`
}

type scheduleResponse struct {
	Data struct {
		Segments         []scheduleSegmentWire `json:"segments"`
		BroadcasterID    string                `json:"broadcaster_id"`
		BroadcasterName  string                `json:"broadcaster_name"`
		BroadcasterLogin string                `json:"broadcaster_login"`
		Vacation         *scheduleVacationWire `json:"vacation"`
	} `json:"data"`
	Pagination pagination `json:"pagination"`
}

type scheduleSegmentWire struct {
	ID         string     `json:"id"`
	StartTime  time.Time  `json:"start_time"`
	EndTime    time.Time  `json:"end_time"`
	Title      string     `json:"title"`
	CanceledAt *time.Time `json:"canceled_until"`
	Category   *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"category"`
	Recurring bool `json:"is_recurring"`
}

type scheduleVacationWire struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

type eventSubAPIPage struct {
	Data         []EventSubSubscription `json:"data"`
	Total        int                    `json:"total"`
	TotalCost    int                    `json:"total_cost"`
	MaxTotalCost int                    `json:"max_total_cost"`
	Pagination   pagination             `json:"pagination"`
}

var _ LiveWorkflow = (*Client)(nil)
var _ EventSubWorkflow = (*Client)(nil)

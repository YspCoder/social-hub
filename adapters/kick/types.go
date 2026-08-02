package kick

import (
	"context"
	"time"

	"social-hub/pkg/socialhub"
)

// User is a Kick user returned by the Public API.
type User struct {
	UserID         int64  `json:"user_id"`
	Name           string `json:"name"`
	Email          string `json:"email,omitempty"`
	ProfilePicture string `json:"profile_picture"`
}

// Category is a Kick livestream category.
type Category struct {
	ID        int64    `json:"id"`
	Name      string   `json:"name"`
	Tags      []string `json:"tags,omitempty"`
	Thumbnail string   `json:"thumbnail"`
}

// Stream is the current stream attached to a channel response.
type Stream struct {
	CustomTags  []string   `json:"custom_tags"`
	Key         string     `json:"key,omitempty"`
	URL         string     `json:"url,omitempty"`
	IsLive      bool       `json:"is_live"`
	IsMature    bool       `json:"is_mature"`
	Language    string     `json:"language"`
	StartTime   *time.Time `json:"start_time,omitempty"`
	Thumbnail   string     `json:"thumbnail"`
	ViewerCount int64      `json:"viewer_count"`
}

// Channel describes a Kick broadcaster channel.
type Channel struct {
	ActiveSubscribersCount   int64    `json:"active_subscribers_count"`
	BannerPicture            string   `json:"banner_picture"`
	BroadcasterUserID        int64    `json:"broadcaster_user_id"`
	CanceledSubscribersCount int64    `json:"canceled_subscribers_count"`
	Category                 Category `json:"category"`
	ChannelDescription       string   `json:"channel_description"`
	Slug                     string   `json:"slug"`
	Stream                   Stream   `json:"stream"`
	StreamTitle              string   `json:"stream_title"`
}

// Livestream is one active V2 Kick livestream.
type Livestream struct {
	ID               string             `json:"id"`
	BroadcasterUser  LivestreamUser     `json:"broadcaster_user"`
	Category         LivestreamCategory `json:"category"`
	Channel          LivestreamChannel  `json:"channel"`
	HasMatureContent bool               `json:"has_mature_content"`
	LanguageCode     string             `json:"language_code"`
	StartedAt        time.Time          `json:"started_at"`
	Tags             []string           `json:"tags"`
	Thumbnail        string             `json:"thumbnail"`
	Title            string             `json:"title"`
	ViewerCount      int64              `json:"viewer_count"`
}

type LivestreamUser struct {
	ID             int64  `json:"id"`
	ProfilePicture string `json:"profile_picture"`
	Username       string `json:"username"`
}

type LivestreamCategory struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Thumbnail string `json:"thumbnail"`
}

type LivestreamChannel struct {
	Slug string `json:"slug"`
}

type ChannelListRequest struct {
	BroadcasterUserIDs []string
	Slugs              []string
}

type UpdateChannelRequest struct {
	StreamTitle *string
	CategoryID  *int64
	CustomTags  *[]string
}

type LivestreamListRequest struct {
	CategoryIDs   []string
	LanguageCodes []string
	Cursor        string
	Limit         int
}

type CategoryListRequest struct {
	Names  []string
	Tags   []string
	IDs    []string
	Cursor string
	Limit  int
}

type SendChatRequest struct {
	BroadcasterUserID string
	Content           string
	ReplyToMessageID  string
	Type              string
}

type ChatResult struct {
	IsSent    bool   `json:"is_sent"`
	MessageID string `json:"message_id"`
}

type EventSubscriptionRequest struct {
	Name    string `json:"name"`
	Version int    `json:"version"`
}

type CreateSubscriptionsRequest struct {
	BroadcasterUserID string
	Events            []EventSubscriptionRequest
}

type SubscriptionResult struct {
	Error          string `json:"error"`
	Name           string `json:"name"`
	SubscriptionID string `json:"subscription_id"`
	Version        int    `json:"version"`
}

type Subscription struct {
	AppID             string    `json:"app_id"`
	BroadcasterUserID int64     `json:"broadcaster_user_id"`
	CreatedAt         time.Time `json:"created_at"`
	Event             string    `json:"event"`
	ID                string    `json:"id"`
	Method            string    `json:"method"`
	UpdatedAt         time.Time `json:"updated_at"`
	Version           int       `json:"version"`
}

type TokenIntrospection struct {
	Active    bool   `json:"active"`
	ClientID  string `json:"client_id"`
	TokenType string `json:"token_type"`
	Scope     string `json:"scope"`
	ExpiresAt int64  `json:"exp"`
}

type UserWorkflow interface {
	ListUsers(context.Context, []string, ...socialhub.CallOption) ([]User, error)
}

type ChannelWorkflow interface {
	ListChannels(context.Context, ChannelListRequest, ...socialhub.CallOption) ([]Channel, error)
	UpdateChannel(context.Context, UpdateChannelRequest, ...socialhub.CallOption) error
}

type LivestreamWorkflow interface {
	ListLivestreams(context.Context, LivestreamListRequest, ...socialhub.CallOption) (socialhub.Page[Livestream], error)
	ListUserLivestreams(context.Context, []string, ...socialhub.CallOption) ([]Livestream, error)
}

type CategoryWorkflow interface {
	ListCategories(context.Context, CategoryListRequest, ...socialhub.CallOption) (socialhub.Page[Category], error)
}

type ChatWorkflow interface {
	SendChat(context.Context, SendChatRequest, ...socialhub.CallOption) (*ChatResult, error)
	DeleteChat(context.Context, string, ...socialhub.CallOption) error
}

type SubscriptionWorkflow interface {
	ListSubscriptions(context.Context, string, ...socialhub.CallOption) ([]Subscription, error)
	CreateSubscriptions(context.Context, CreateSubscriptionsRequest, ...socialhub.CallOption) ([]SubscriptionResult, error)
	DeleteSubscriptions(context.Context, []string, ...socialhub.CallOption) error
	FetchWebhookPublicKey(context.Context, ...socialhub.CallOption) (string, error)
}

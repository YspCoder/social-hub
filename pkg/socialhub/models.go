// Package socialhub defines the stable, platform-neutral contracts used by
// social-hub adapters and applications.
package socialhub

import (
	"encoding/json"
	"time"
)

// Platform identifies a social platform without assuming a specific API
// product on that platform.
type Platform string

// AccountID identifies one configured platform account within an application.
type AccountID string

// MediaType is the normalized kind of a media object.
type MediaType string

const (
	MediaTypeImage     MediaType = "image"
	MediaTypeVideo     MediaType = "video"
	MediaTypeAudio     MediaType = "audio"
	MediaTypeDocument  MediaType = "document"
	MediaTypeAnimation MediaType = "animation"
)

// MediaState describes the lifecycle of uploaded media.
type MediaState string

const (
	MediaStateCreated    MediaState = "created"
	MediaStateUploading  MediaState = "uploading"
	MediaStateProcessing MediaState = "processing"
	MediaStateReady      MediaState = "ready"
	MediaStateFailed     MediaState = "failed"
	MediaStateExpired    MediaState = "expired"
)

// PublishState describes the lifecycle of a post publication.
type PublishState string

const (
	PublishStatePending   PublishState = "pending"
	PublishStatePublished PublishState = "published"
	PublishStateFailed    PublishState = "failed"
)

// Direction describes whether a message was received or sent by the configured
// account.
type Direction string

const (
	DirectionInbound  Direction = "inbound"
	DirectionOutbound Direction = "outbound"
)

// RelationType describes how a post references another post.
type RelationType string

const (
	RelationReply  RelationType = "reply"
	RelationQuote  RelationType = "quote"
	RelationRepost RelationType = "repost"
)

// User is the minimum common representation of a platform user.
type User struct {
	Platform    Platform                   `json:"platform"`
	AccountID   AccountID                  `json:"account_id"`
	ID          string                     `json:"id"`
	Username    *string                    `json:"username,omitempty"`
	DisplayName *string                    `json:"display_name,omitempty"`
	AvatarURL   *string                    `json:"avatar_url,omitempty"`
	ProfileURL  *string                    `json:"profile_url,omitempty"`
	AccountType *string                    `json:"account_type,omitempty"`
	Extensions  map[string]json.RawMessage `json:"extensions,omitempty"`
}

// Post is the minimum common representation of platform content.
type Post struct {
	Platform   Platform                   `json:"platform"`
	AccountID  AccountID                  `json:"account_id"`
	ID         string                     `json:"id"`
	AuthorID   *string                    `json:"author_id,omitempty"`
	Text       *string                    `json:"text,omitempty"`
	Media      []Media                    `json:"media,omitempty"`
	Relations  []PostRelation             `json:"relations,omitempty"`
	CreatedAt  *time.Time                 `json:"created_at,omitempty"`
	URL        *string                    `json:"url,omitempty"`
	Visibility *string                    `json:"visibility,omitempty"`
	Status     *PublishStatus             `json:"status,omitempty"`
	Metrics    []Metric                   `json:"metrics,omitempty"`
	Extensions map[string]json.RawMessage `json:"extensions,omitempty"`
}

// PostRelation preserves reply, quote, and repost semantics.
type PostRelation struct {
	Type   RelationType `json:"type"`
	PostID string       `json:"post_id"`
}

// Media describes an uploaded or remote media object.
type Media struct {
	ID         string                     `json:"id,omitempty"`
	URL        string                     `json:"url,omitempty"`
	MIME       string                     `json:"mime,omitempty"`
	Type       MediaType                  `json:"type"`
	Size       *int64                     `json:"size,omitempty"`
	Width      *int                       `json:"width,omitempty"`
	Height     *int                       `json:"height,omitempty"`
	Duration   *time.Duration             `json:"duration,omitempty"`
	State      MediaState                 `json:"state"`
	ExpiresAt  *time.Time                 `json:"expires_at,omitempty"`
	Extensions map[string]json.RawMessage `json:"extensions,omitempty"`
}

// Comment is a flat comment representation. ParentID preserves thread shape.
type Comment struct {
	Platform   Platform                   `json:"platform"`
	AccountID  AccountID                  `json:"account_id"`
	ID         string                     `json:"id"`
	PostID     string                     `json:"post_id"`
	AuthorID   *string                    `json:"author_id,omitempty"`
	ParentID   *string                    `json:"parent_id,omitempty"`
	Text       string                     `json:"text"`
	CreatedAt  *time.Time                 `json:"created_at,omitempty"`
	Metrics    []Metric                   `json:"metrics,omitempty"`
	Extensions map[string]json.RawMessage `json:"extensions,omitempty"`
}

// Message is the minimum common representation of a direct or channel message.
type Message struct {
	Platform       Platform                   `json:"platform"`
	AccountID      AccountID                  `json:"account_id"`
	ID             string                     `json:"id"`
	ConversationID string                     `json:"conversation_id"`
	SenderID       *string                    `json:"sender_id,omitempty"`
	RecipientIDs   []string                   `json:"recipient_ids,omitempty"`
	Text           *string                    `json:"text,omitempty"`
	Media          []Media                    `json:"media,omitempty"`
	ReplyToID      *string                    `json:"reply_to_id,omitempty"`
	SentAt         *time.Time                 `json:"sent_at,omitempty"`
	Direction      Direction                  `json:"direction"`
	Extensions     map[string]json.RawMessage `json:"extensions,omitempty"`
}

// Metric preserves a metric's definition and observation time instead of
// assuming that similarly named platform metrics have identical semantics.
type Metric struct {
	Name       string            `json:"name"`
	Value      float64           `json:"value"`
	AsOf       time.Time         `json:"as_of"`
	Window     string            `json:"window,omitempty"`
	Dimensions map[string]string `json:"dimensions,omitempty"`
	Definition string            `json:"definition,omitempty"`
}

// PublishStatus is the normalized state of an asynchronous publication.
type PublishStatus struct {
	ID        string       `json:"id"`
	State     PublishState `json:"state"`
	Message   string       `json:"message,omitempty"`
	UpdatedAt *time.Time   `json:"updated_at,omitempty"`
}

// Page is a cursor-based page of results.
type Page[T any] struct {
	Items      []T     `json:"items"`
	NextCursor *string `json:"next_cursor,omitempty"`
	PrevCursor *string `json:"prev_cursor,omitempty"`
	HasMore    bool    `json:"has_more"`
}

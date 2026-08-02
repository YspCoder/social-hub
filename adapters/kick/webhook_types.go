package kick

import (
	"encoding/json"
	"time"
)

// WebhookMetadata preserves delivery metadata carried in Kick headers.
type WebhookMetadata struct {
	MessageID        string          `json:"-"`
	SubscriptionID   string          `json:"-"`
	EventType        string          `json:"-"`
	Version          string          `json:"-"`
	MessageTimestamp time.Time       `json:"-"`
	Raw              json.RawMessage `json:"-"`
}

type WebhookBadge struct {
	Text  string `json:"text"`
	Type  string `json:"type"`
	Count *int64 `json:"count,omitempty"`
}

type WebhookIdentity struct {
	UsernameColor string         `json:"username_color"`
	Badges        []WebhookBadge `json:"badges"`
}

// WebhookUser permits nullable identity fields used by anonymous gift events.
type WebhookUser struct {
	IsAnonymous    bool             `json:"is_anonymous"`
	UserID         *int64           `json:"user_id"`
	Username       *string          `json:"username"`
	IsVerified     *bool            `json:"is_verified"`
	ProfilePicture *string          `json:"profile_picture"`
	ChannelSlug    *string          `json:"channel_slug"`
	Identity       *WebhookIdentity `json:"identity"`
}

type ChatReply struct {
	MessageID string      `json:"message_id"`
	Sender    WebhookUser `json:"sender"`
	Content   string      `json:"content"`
}

type ChatEmotePosition struct {
	Start int `json:"s"`
	End   int `json:"e"`
}

type ChatEmote struct {
	EmoteID   string              `json:"emote_id"`
	Positions []ChatEmotePosition `json:"positions"`
}

type ChatMessageEvent struct {
	WebhookMetadata
	MessageID   string      `json:"message_id"`
	RepliesTo   *ChatReply  `json:"replies_to"`
	Broadcaster WebhookUser `json:"broadcaster"`
	Sender      WebhookUser `json:"sender"`
	Content     string      `json:"content"`
	Emotes      []ChatEmote `json:"emotes"`
	CreatedAt   time.Time   `json:"created_at"`
}

type ChannelFollowedEvent struct {
	WebhookMetadata
	Broadcaster WebhookUser `json:"broadcaster"`
	Follower    WebhookUser `json:"follower"`
}

type ChannelSubscriptionRenewalEvent struct {
	WebhookMetadata
	Broadcaster WebhookUser `json:"broadcaster"`
	Subscriber  WebhookUser `json:"subscriber"`
	Duration    int64       `json:"duration"`
	CreatedAt   time.Time   `json:"created_at"`
	ExpiresAt   time.Time   `json:"expires_at"`
}

type ChannelSubscriptionGiftsEvent struct {
	WebhookMetadata
	Broadcaster WebhookUser   `json:"broadcaster"`
	Gifter      WebhookUser   `json:"gifter"`
	Giftees     []WebhookUser `json:"giftees"`
	CreatedAt   time.Time     `json:"created_at"`
	ExpiresAt   time.Time     `json:"expires_at"`
}

type ChannelSubscriptionNewEvent struct {
	WebhookMetadata
	Broadcaster WebhookUser `json:"broadcaster"`
	Subscriber  WebhookUser `json:"subscriber"`
	Duration    int64       `json:"duration"`
	CreatedAt   time.Time   `json:"created_at"`
	ExpiresAt   time.Time   `json:"expires_at"`
}

type RewardSummary struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Cost        int64  `json:"cost"`
	Description string `json:"description"`
}

type ChannelRewardRedemptionUpdatedEvent struct {
	WebhookMetadata
	ID          string        `json:"id"`
	UserInput   string        `json:"user_input"`
	Status      string        `json:"status"`
	RedeemedAt  time.Time     `json:"redeemed_at"`
	Reward      RewardSummary `json:"reward"`
	Redeemer    WebhookUser   `json:"redeemer"`
	Broadcaster WebhookUser   `json:"broadcaster"`
}

type LivestreamStatusUpdatedEvent struct {
	WebhookMetadata
	Broadcaster WebhookUser `json:"broadcaster"`
	IsLive      bool        `json:"is_live"`
	Title       string      `json:"title"`
	StartedAt   time.Time   `json:"started_at"`
	EndedAt     *time.Time  `json:"ended_at"`
}

type LivestreamEventCategory struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Thumbnail string `json:"thumbnail"`
}

type LivestreamEventMetadata struct {
	Title            string                  `json:"title"`
	Language         string                  `json:"language"`
	HasMatureContent bool                    `json:"has_mature_content"`
	Category         LivestreamEventCategory `json:"category"`
}

type LivestreamMetadataUpdatedEvent struct {
	WebhookMetadata
	Broadcaster WebhookUser             `json:"broadcaster"`
	Metadata    LivestreamEventMetadata `json:"metadata"`
}

type ModerationMetadata struct {
	Reason    string     `json:"reason"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at"`
}

type ModerationBannedEvent struct {
	WebhookMetadata
	Broadcaster WebhookUser        `json:"broadcaster"`
	Moderator   WebhookUser        `json:"moderator"`
	BannedUser  WebhookUser        `json:"banned_user"`
	Metadata    ModerationMetadata `json:"metadata"`
}

type KicksGift struct {
	Amount            int64  `json:"amount"`
	Name              string `json:"name"`
	Type              string `json:"type"`
	Tier              string `json:"tier"`
	Message           string `json:"message"`
	PinnedTimeSeconds int64  `json:"pinned_time_seconds"`
}

type KicksGiftedEvent struct {
	WebhookMetadata
	Broadcaster WebhookUser `json:"broadcaster"`
	Sender      WebhookUser `json:"sender"`
	Gift        KicksGift   `json:"gift"`
	CreatedAt   time.Time   `json:"created_at"`
}

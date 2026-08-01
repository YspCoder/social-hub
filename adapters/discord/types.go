package discord

import "time"

type discordUser struct {
	ID         string `json:"id"`
	Username   string `json:"username"`
	GlobalName string `json:"global_name"`
	Avatar     string `json:"avatar"`
	Bot        bool   `json:"bot"`
	System     bool   `json:"system"`
}

type discordMessage struct {
	ID                string              `json:"id"`
	ChannelID         string              `json:"channel_id"`
	GuildID           string              `json:"guild_id"`
	Author            discordUser         `json:"author"`
	Content           string              `json:"content"`
	Timestamp         time.Time           `json:"timestamp"`
	EditedTimestamp   *time.Time          `json:"edited_timestamp"`
	Attachments       []discordAttachment `json:"attachments"`
	MessageReference  *messageReference   `json:"message_reference"`
	ReferencedMessage *discordMessage     `json:"referenced_message"`
	WebhookID         string              `json:"webhook_id"`
	Type              int                 `json:"type"`
	Flags             int                 `json:"flags"`
	Pinned            bool                `json:"pinned"`
}

type discordAttachment struct {
	ID          string  `json:"id"`
	Filename    string  `json:"filename"`
	ContentType string  `json:"content_type"`
	Size        int64   `json:"size"`
	URL         string  `json:"url"`
	Width       *int    `json:"width"`
	Height      *int    `json:"height"`
	Duration    float64 `json:"duration_secs"`
	Flags       int     `json:"flags"`
}

type messageReference struct {
	MessageID string `json:"message_id,omitempty"`
	ChannelID string `json:"channel_id,omitempty"`
	GuildID   string `json:"guild_id,omitempty"`
}

type allowedMentions struct {
	Parse       []string `json:"parse"`
	RepliedUser bool     `json:"replied_user"`
}

type messageCreate struct {
	Content          string            `json:"content"`
	AllowedMentions  allowedMentions   `json:"allowed_mentions"`
	MessageReference *messageReference `json:"message_reference,omitempty"`
}

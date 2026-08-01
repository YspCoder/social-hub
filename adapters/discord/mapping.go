package discord

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

func (c *Client) mapUser(user discordUser) socialhub.User {
	var username, displayName, avatarURL, profileURL *string
	if user.Username != "" {
		username = stringPointer(user.Username)
	}
	if user.GlobalName != "" {
		displayName = stringPointer(user.GlobalName)
	} else if user.Username != "" {
		displayName = stringPointer(user.Username)
	}
	if user.Avatar != "" && user.ID != "" {
		extension := ".png"
		if strings.HasPrefix(user.Avatar, "a_") {
			extension = ".gif"
		}
		avatarURL = stringPointer(c.cdnURL + "/avatars/" + user.ID + "/" + user.Avatar + extension)
	}
	if user.ID != "" {
		profileURL = stringPointer("https://discord.com/users/" + user.ID)
	}
	accountType := "user"
	if user.Bot {
		accountType = "bot"
	} else if user.System {
		accountType = "system"
	}
	extension, _ := json.Marshal(struct {
		Bot    bool `json:"bot,omitempty"`
		System bool `json:"system,omitempty"`
	}{Bot: user.Bot, System: user.System})
	return socialhub.User{
		Platform: "discord", AccountID: c.accountID, ID: user.ID, Username: username, DisplayName: displayName,
		AvatarURL: avatarURL, ProfileURL: profileURL, AccountType: &accountType,
		Extensions: map[string]json.RawMessage{"discord.user": extension},
	}
}

func (c *Client) mapMessage(message discordMessage, direction socialhub.Direction) *socialhub.Message {
	var text, senderID, replyToID *string
	if message.Content != "" {
		text = stringPointer(message.Content)
	}
	if message.Author.ID != "" {
		senderID = stringPointer(message.Author.ID)
	}
	if message.MessageReference != nil && message.MessageReference.MessageID != "" {
		channelID := firstNonEmpty(message.MessageReference.ChannelID, message.ChannelID)
		replyToID = stringPointer(composeMessageID(channelID, message.MessageReference.MessageID))
	}
	var sentAt *time.Time
	if !message.Timestamp.IsZero() {
		value := message.Timestamp.UTC()
		sentAt = &value
	}
	extension, _ := json.Marshal(struct {
		RawMessageID string     `json:"raw_message_id"`
		GuildID      string     `json:"guild_id,omitempty"`
		WebhookID    string     `json:"webhook_id,omitempty"`
		Type         int        `json:"type"`
		Flags        int        `json:"flags,omitempty"`
		Pinned       bool       `json:"pinned,omitempty"`
		EditedAt     *time.Time `json:"edited_at,omitempty"`
	}{RawMessageID: message.ID, GuildID: message.GuildID, WebhookID: message.WebhookID, Type: message.Type, Flags: message.Flags, Pinned: message.Pinned, EditedAt: message.EditedTimestamp})
	return &socialhub.Message{
		Platform: "discord", AccountID: c.accountID, ID: composeMessageID(message.ChannelID, message.ID),
		ConversationID: message.ChannelID, SenderID: senderID, Text: text, Media: mapAttachments(message.Attachments),
		ReplyToID: replyToID, SentAt: sentAt, Direction: direction,
		Extensions: map[string]json.RawMessage{"discord.message": extension},
	}
}

func (c *Client) mapPost(message discordMessage) *socialhub.Post {
	mapped := c.mapMessage(message, socialhub.DirectionInbound)
	post := &socialhub.Post{
		Platform: "discord", AccountID: c.accountID, ID: mapped.ID, AuthorID: mapped.SenderID, Text: mapped.Text,
		Media: mapped.Media, CreatedAt: mapped.SentAt, Extensions: mapped.Extensions,
		Status: &socialhub.PublishStatus{ID: mapped.ID, State: socialhub.PublishStatePublished, UpdatedAt: mapped.SentAt},
	}
	if mapped.ReplyToID != nil {
		post.Relations = []socialhub.PostRelation{{Type: socialhub.RelationReply, PostID: *mapped.ReplyToID}}
	}
	if message.ChannelID != "" && message.ID != "" {
		guildID := firstNonEmpty(message.GuildID, "@me")
		post.URL = stringPointer("https://discord.com/channels/" + guildID + "/" + message.ChannelID + "/" + message.ID)
	}
	return post
}

func (c *Client) mapComment(postID string, message discordMessage, parentID *string) *socialhub.Comment {
	mapped := c.mapMessage(message, socialhub.DirectionOutbound)
	return &socialhub.Comment{
		Platform: "discord", AccountID: c.accountID, ID: mapped.ID, PostID: postID, AuthorID: mapped.SenderID,
		ParentID: parentID, Text: firstString(mapped.Text), CreatedAt: mapped.SentAt, Extensions: mapped.Extensions,
	}
}

func mapAttachments(attachments []discordAttachment) []socialhub.Media {
	result := make([]socialhub.Media, 0, len(attachments))
	for _, attachment := range attachments {
		media := socialhub.Media{
			ID: attachment.ID, URL: attachment.URL, MIME: attachment.ContentType, Type: mediaType(attachment.ContentType),
			Size: optionalInt64(attachment.Size), Width: attachment.Width, Height: attachment.Height, State: socialhub.MediaStateReady,
		}
		if attachment.Duration > 0 {
			duration := time.Duration(attachment.Duration * float64(time.Second))
			media.Duration = &duration
		}
		extension, _ := json.Marshal(struct {
			Filename string `json:"filename,omitempty"`
			Flags    int    `json:"flags,omitempty"`
		}{Filename: attachment.Filename, Flags: attachment.Flags})
		media.Extensions = map[string]json.RawMessage{"discord.attachment": extension}
		result = append(result, media)
	}
	return result
}

func mediaType(contentType string) socialhub.MediaType {
	switch {
	case strings.HasPrefix(contentType, "image/"):
		return socialhub.MediaTypeImage
	case strings.HasPrefix(contentType, "video/"):
		return socialhub.MediaTypeVideo
	case strings.HasPrefix(contentType, "audio/"):
		return socialhub.MediaTypeAudio
	default:
		return socialhub.MediaTypeDocument
	}
}

func composeMessageID(channelID, messageID string) string {
	return channelID + "/" + messageID
}

func parseMessageID(operation, value, defaultChannelID string) (string, string, error) {
	parts := strings.Split(value, "/")
	if len(parts) == 1 && defaultChannelID != "" && validSnowflake(parts[0]) {
		return defaultChannelID, parts[0], nil
	}
	if len(parts) != 2 || !validSnowflake(parts[0]) || !validSnowflake(parts[1]) {
		return "", "", invalidArgument(operation, "message ID must be channel_id/message_id, or a raw snowflake when a default channel is configured")
	}
	return parts[0], parts[1], nil
}

func validSnowflake(value string) bool {
	if value == "" || len(value) > 20 {
		return false
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	return err == nil && parsed > 0
}

func channelMessagePath(channelID, messageID string) string {
	return fmt.Sprintf("/channels/%s/messages/%s", channelID, messageID)
}

func stringPointer(value string) *string { return &value }

func optionalInt64(value int64) *int64 {
	if value <= 0 {
		return nil
	}
	return &value
}

func firstString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

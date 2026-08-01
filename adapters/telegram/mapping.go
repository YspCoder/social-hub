package telegram

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot/models"

	"social-hub/pkg/socialhub"
)

func resolveCallContext(ctx context.Context, options ...socialhub.CallOption) (context.Context, context.CancelFunc, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return nil, nil, err
	}
	if resolved.Timeout > 0 {
		callCtx, cancel := context.WithTimeout(ctx, resolved.Timeout)
		return callCtx, cancel, nil
	}
	return ctx, func() {}, nil
}

func mapUser(accountID socialhub.AccountID, user models.User) socialhub.User {
	var username, displayName, profileURL *string
	if user.Username != "" {
		value := user.Username
		username = &value
		profile := "https://t.me/" + user.Username
		profileURL = &profile
	}
	name := strings.TrimSpace(strings.Join([]string{user.FirstName, user.LastName}, " "))
	if name != "" {
		displayName = &name
	}
	accountType := "user"
	if user.IsBot {
		accountType = "bot"
	}
	extension, _ := json.Marshal(struct {
		LanguageCode string `json:"language_code,omitempty"`
		IsPremium    bool   `json:"is_premium,omitempty"`
	}{LanguageCode: user.LanguageCode, IsPremium: user.IsPremium})
	return socialhub.User{
		Platform: "telegram", AccountID: accountID, ID: strconv.FormatInt(user.ID, 10), Username: username,
		DisplayName: displayName, ProfileURL: profileURL, AccountType: &accountType,
		Extensions: map[string]json.RawMessage{"telegram.user": extension},
	}
}

func mapMessage(accountID socialhub.AccountID, message *models.Message, direction socialhub.Direction) *socialhub.Message {
	if message == nil {
		return nil
	}
	conversationID := strconv.FormatInt(message.Chat.ID, 10)
	var senderID *string
	if message.From != nil {
		value := strconv.FormatInt(message.From.ID, 10)
		senderID = &value
	} else if message.SenderChat != nil {
		value := strconv.FormatInt(message.SenderChat.ID, 10)
		senderID = &value
	}
	var text *string
	if message.Text != "" {
		value := message.Text
		text = &value
	} else if message.Caption != "" {
		value := message.Caption
		text = &value
	}
	var replyToID *string
	if message.ReplyToMessage != nil {
		value := strconv.Itoa(message.ReplyToMessage.ID)
		replyToID = &value
	}
	var sentAt *time.Time
	if message.Date > 0 {
		value := time.Unix(int64(message.Date), 0).UTC()
		sentAt = &value
	}
	extension, _ := json.Marshal(struct {
		ChatType        models.ChatType `json:"chat_type"`
		ChatUsername    string          `json:"chat_username,omitempty"`
		MessageThreadID int             `json:"message_thread_id,omitempty"`
		MediaGroupID    string          `json:"media_group_id,omitempty"`
	}{ChatType: message.Chat.Type, ChatUsername: message.Chat.Username, MessageThreadID: message.MessageThreadID, MediaGroupID: message.MediaGroupID})
	var recipientIDs []string
	if direction == socialhub.DirectionOutbound {
		recipientIDs = []string{conversationID}
	}
	return &socialhub.Message{
		Platform: "telegram", AccountID: accountID, ID: strconv.Itoa(message.ID), ConversationID: conversationID,
		SenderID: senderID, RecipientIDs: recipientIDs, Text: text, Media: mapMedia(message), ReplyToID: replyToID,
		SentAt: sentAt, Direction: direction, Extensions: map[string]json.RawMessage{"telegram.message": extension},
	}
}

func mapPost(accountID socialhub.AccountID, message *models.Message) *socialhub.Post {
	mapped := mapMessage(accountID, message, socialhub.DirectionOutbound)
	post := &socialhub.Post{
		Platform: "telegram", AccountID: accountID, ID: mapped.ID, AuthorID: mapped.SenderID, Text: mapped.Text,
		Media: mapped.Media, CreatedAt: mapped.SentAt,
		Status:     &socialhub.PublishStatus{ID: mapped.ID, State: socialhub.PublishStatePublished, UpdatedAt: mapped.SentAt},
		Extensions: mapped.Extensions,
	}
	if mapped.ReplyToID != nil {
		post.Relations = []socialhub.PostRelation{{Type: socialhub.RelationReply, PostID: *mapped.ReplyToID}}
	}
	if message.Chat.Username != "" && message.ID > 0 {
		value := "https://t.me/" + message.Chat.Username + "/" + strconv.Itoa(message.ID)
		post.URL = &value
	}
	return post
}

func mapMedia(message *models.Message) []socialhub.Media {
	var result []socialhub.Media
	if len(message.Photo) > 0 {
		photo := message.Photo[0]
		for _, candidate := range message.Photo[1:] {
			if candidate.Width*candidate.Height > photo.Width*photo.Height {
				photo = candidate
			}
		}
		size := int64(photo.FileSize)
		result = append(result, socialhub.Media{ID: photo.FileID, Type: socialhub.MediaTypeImage, Size: optionalSize(size), Width: intPointer(photo.Width), Height: intPointer(photo.Height), State: socialhub.MediaStateReady, Extensions: mediaExtension(photo.FileUniqueID)})
	}
	if message.Video != nil {
		duration := time.Duration(message.Video.Duration) * time.Second
		result = append(result, socialhub.Media{ID: message.Video.FileID, MIME: message.Video.MimeType, Type: socialhub.MediaTypeVideo, Size: optionalSize(message.Video.FileSize), Width: intPointer(message.Video.Width), Height: intPointer(message.Video.Height), Duration: &duration, State: socialhub.MediaStateReady, Extensions: mediaExtension(message.Video.FileUniqueID)})
	}
	if message.Audio != nil {
		duration := time.Duration(message.Audio.Duration) * time.Second
		result = append(result, socialhub.Media{ID: message.Audio.FileID, MIME: message.Audio.MimeType, Type: socialhub.MediaTypeAudio, Size: optionalSize(message.Audio.FileSize), Duration: &duration, State: socialhub.MediaStateReady, Extensions: mediaExtension(message.Audio.FileUniqueID)})
	}
	if message.Document != nil {
		result = append(result, socialhub.Media{ID: message.Document.FileID, MIME: message.Document.MimeType, Type: socialhub.MediaTypeDocument, Size: optionalSize(message.Document.FileSize), State: socialhub.MediaStateReady, Extensions: mediaExtension(message.Document.FileUniqueID)})
	}
	if message.Animation != nil {
		duration := time.Duration(message.Animation.Duration) * time.Second
		result = append(result, socialhub.Media{ID: message.Animation.FileID, MIME: message.Animation.MimeType, Type: socialhub.MediaTypeAnimation, Size: optionalSize(message.Animation.FileSize), Width: intPointer(message.Animation.Width), Height: intPointer(message.Animation.Height), Duration: &duration, State: socialhub.MediaStateReady, Extensions: mediaExtension(message.Animation.FileUniqueID)})
	}
	return result
}

func mediaExtension(uniqueID string) map[string]json.RawMessage {
	extension, _ := json.Marshal(struct {
		FileUniqueID string `json:"file_unique_id"`
	}{FileUniqueID: uniqueID})
	return map[string]json.RawMessage{"telegram.file": extension}
}

func optionalSize(value int64) *int64 {
	if value <= 0 {
		return nil
	}
	return &value
}

func intPointer(value int) *int { return &value }

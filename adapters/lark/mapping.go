package lark

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

func mapUser(accountID socialhub.AccountID, requestedID string, input wireUser) *socialhub.User {
	displayName := firstNonEmpty(input.Name, input.EnName, input.Nickname)
	avatar := firstNonEmpty(input.Avatar.AvatarOrigin, input.Avatar.Avatar640, input.Avatar.Avatar240, input.Avatar.Avatar72)
	extension, _ := json.Marshal(input)
	return &socialhub.User{
		Platform: "lark", AccountID: accountID, ID: requestedID,
		Username: stringPointer(firstNonEmpty(input.UserID, input.OpenID)), DisplayName: stringPointer(displayName),
		AvatarURL: stringPointer(avatar), Extensions: map[string]json.RawMessage{"lark.user": extension},
	}
}

func mapMessage(accountID socialhub.AccountID, actorID string, input wireMessage) *socialhub.Message {
	text, media := messageContent(input)
	direction := socialhub.DirectionInbound
	if actorID != "" && input.Sender.ID == actorID {
		direction = socialhub.DirectionOutbound
	}
	message := &socialhub.Message{
		Platform: "lark", AccountID: accountID, ID: input.MessageID, ConversationID: input.ChatID,
		SenderID: stringPointer(input.Sender.ID), Text: text, Media: media, Direction: direction,
	}
	if input.ParentID != "" {
		message.ReplyToID = stringPointer(input.ParentID)
	}
	if created, ok := larkTime(input.CreateTime); ok {
		message.SentAt = &created
	}
	extension, _ := json.Marshal(input)
	message.Extensions = map[string]json.RawMessage{"lark.message": extension}
	return message
}

func mapPost(accountID socialhub.AccountID, input wireMessage, observedAt time.Time) *socialhub.Post {
	text, media := messageContent(input)
	visibility := "chat"
	updatedAt := observedAt
	if updated, ok := larkTime(input.UpdateTime); ok {
		updatedAt = updated
	}
	statusMessage := ""
	if input.Deleted {
		statusMessage = "message withdrawn"
	}
	post := &socialhub.Post{
		Platform: "lark", AccountID: accountID, ID: input.MessageID, AuthorID: stringPointer(input.Sender.ID),
		Text: text, Media: media, Visibility: &visibility,
		Status: &socialhub.PublishStatus{ID: input.MessageID, State: socialhub.PublishStatePublished, Message: statusMessage, UpdatedAt: &updatedAt},
		URL:    stringPointer(input.MessageAppLink),
	}
	if created, ok := larkTime(input.CreateTime); ok {
		post.CreatedAt = &created
	}
	if input.ParentID != "" {
		post.Relations = append(post.Relations, socialhub.PostRelation{Type: socialhub.RelationReply, PostID: input.ParentID})
	}
	extension, _ := json.Marshal(input)
	post.Extensions = map[string]json.RawMessage{"lark.message": extension}
	return post
}

func mapComment(accountID socialhub.AccountID, rootID string, input wireMessage, observedAt time.Time) socialhub.Comment {
	post := mapPost(accountID, input, observedAt)
	comment := socialhub.Comment{
		Platform: "lark", AccountID: accountID, ID: input.MessageID, PostID: rootID,
		AuthorID: post.AuthorID, CreatedAt: post.CreatedAt, Extensions: post.Extensions,
	}
	if post.Text != nil {
		comment.Text = *post.Text
	}
	if input.ParentID != "" && input.ParentID != rootID {
		comment.ParentID = stringPointer(input.ParentID)
	}
	return comment
}

func messageContent(input wireMessage) (*string, []socialhub.Media) {
	content := strings.TrimSpace(input.Body.Content)
	if content == "" {
		return nil, nil
	}
	var value map[string]json.RawMessage
	if json.Unmarshal([]byte(content), &value) != nil {
		return nil, nil
	}
	if input.MessageType == "text" {
		var text string
		if json.Unmarshal(value["text"], &text) == nil {
			return stringPointer(text), nil
		}
	}
	keyName := "file_key"
	mediaType := socialhub.MediaTypeDocument
	switch input.MessageType {
	case "image", "sticker":
		keyName, mediaType = "image_key", socialhub.MediaTypeImage
	case "audio":
		mediaType = socialhub.MediaTypeAudio
	case "media":
		mediaType = socialhub.MediaTypeVideo
	case "file":
		mediaType = socialhub.MediaTypeDocument
	default:
		return nil, nil
	}
	var key string
	if json.Unmarshal(value[keyName], &key) != nil || strings.TrimSpace(key) == "" {
		return nil, nil
	}
	extension, _ := json.Marshal(map[string]string{"message_type": input.MessageType})
	return nil, []socialhub.Media{{
		ID: key, Type: mediaType, State: socialhub.MediaStateReady,
		Extensions: map[string]json.RawMessage{"lark.resource": extension},
	}}
}

func larkTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	number, err := strconv.ParseInt(value, 10, 64)
	if err != nil || number <= 0 {
		return time.Time{}, false
	}
	switch len(value) {
	case 1, 2, 3, 4, 5, 6, 7, 8, 9, 10:
		return time.Unix(number, 0).UTC(), true
	case 11, 12, 13:
		return time.UnixMilli(number).UTC(), true
	case 14, 15, 16:
		return time.UnixMicro(number).UTC(), true
	default:
		return time.Time{}, false
	}
}

func firstUserID(value eventUserID) string {
	return firstNonEmpty(value.OpenID, value.UserID, value.UnionID)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func stringPointer(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	copy := value
	return &copy
}

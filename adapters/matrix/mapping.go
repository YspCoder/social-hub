package matrix

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

func (client *Client) mapUser(userID string, input profileResponse) *socialhub.User {
	username := strings.TrimPrefix(strings.SplitN(userID, ":", 2)[0], "@")
	extensions := map[string]json.RawMessage{}
	addExtension(extensions, "matrix.avatar_mxc", input.AvatarURL)
	return &socialhub.User{
		Platform: "matrix", AccountID: client.accountID, ID: userID,
		Username: optionalPointer(username), DisplayName: optionalPointer(input.DisplayName),
		AvatarURL: optionalPointer(input.AvatarURL), AccountType: pointer("matrix_user"), Extensions: extensions,
	}
}

func (client *Client) mapPost(roomID string, event Event) (*socialhub.Post, error) {
	content, err := messageContent(event)
	if err != nil {
		return nil, err
	}
	createdAt := matrixTime(event.OriginServerTS)
	media := messageMedia(event.EventID, content)
	relations := []socialhub.PostRelation{}
	if replyID := replyEventID(content.RelatesTo); replyID != "" {
		relations = append(relations, socialhub.PostRelation{Type: socialhub.RelationReply, PostID: composeID(roomID, replyID)})
	}
	extensions := eventExtensions(roomID, event, content)
	return &socialhub.Post{
		Platform: "matrix", AccountID: client.accountID, ID: composeID(roomID, event.EventID), AuthorID: optionalPointer(event.Sender),
		Text: optionalPointer(content.Body), Media: media, Relations: relations, CreatedAt: createdAt,
		URL: pointer(matrixToURL(roomID, event.EventID)), Visibility: pointer("room"),
		Status:     &socialhub.PublishStatus{ID: composeID(roomID, event.EventID), State: socialhub.PublishStatePublished, UpdatedAt: createdAt},
		Extensions: extensions,
	}, nil
}

func (client *Client) mapMessage(roomID string, event Event, direction socialhub.Direction) (*socialhub.Message, error) {
	content, err := messageContent(event)
	if err != nil {
		return nil, err
	}
	message := &socialhub.Message{
		Platform: "matrix", AccountID: client.accountID, ID: composeID(roomID, event.EventID), ConversationID: roomID,
		SenderID: optionalPointer(event.Sender), RecipientIDs: []string{roomID}, Text: optionalPointer(content.Body),
		Media: messageMedia(event.EventID, content), SentAt: matrixTime(event.OriginServerTS), Direction: direction,
		Extensions: eventExtensions(roomID, event, content),
	}
	if replyID := replyEventID(content.RelatesTo); replyID != "" {
		message.ReplyToID = pointer(composeID(roomID, replyID))
	}
	return message, nil
}

func (client *Client) mapComment(roomID, rootEventID string, event Event) (*socialhub.Comment, error) {
	content, err := messageContent(event)
	if err != nil {
		return nil, err
	}
	parentID := replyEventID(content.RelatesTo)
	if parentID == rootEventID {
		parentID = ""
	}
	return &socialhub.Comment{
		Platform: "matrix", AccountID: client.accountID, ID: composeID(roomID, event.EventID), PostID: composeID(roomID, rootEventID),
		AuthorID: optionalPointer(event.Sender), ParentID: optionalCompositeID(roomID, parentID), Text: content.Body,
		CreatedAt: matrixTime(event.OriginServerTS), Extensions: eventExtensions(roomID, event, content),
	}, nil
}

func messageContent(event Event) (MessageContent, error) {
	if event.Type == EventTypeEncrypted {
		return MessageContent{}, unsupported("map_event", "encrypted Matrix events require an E2EE-capable client")
	}
	if event.Type != EventTypeMessage || !validEventID(event.EventID) {
		return MessageContent{}, unsupported("map_event", "only valid unencrypted m.room.message events map to common content")
	}
	var content MessageContent
	if err := json.Unmarshal(event.Content, &content); err != nil || !validMessageType(content.MessageType) || !json.Valid(event.Content) {
		return MessageContent{}, platformError("map_event", socialhub.CodePlatformError, socialhub.ClassPermanent, firstError(err, errors.New("invalid Matrix message content")))
	}
	return content, nil
}

func validMessageType(value string) bool {
	switch value {
	case MessageTypeText, MessageTypeNotice, MessageTypeEmote, MessageTypeImage, MessageTypeVideo, MessageTypeAudio, MessageTypeFile:
		return true
	default:
		return false
	}
}

func messageMedia(eventID string, content MessageContent) []socialhub.Media {
	var mediaType socialhub.MediaType
	switch content.MessageType {
	case MessageTypeImage:
		mediaType = socialhub.MediaTypeImage
	case MessageTypeVideo:
		mediaType = socialhub.MediaTypeVideo
	case MessageTypeAudio:
		mediaType = socialhub.MediaTypeAudio
	case MessageTypeFile:
		mediaType = socialhub.MediaTypeDocument
	default:
		return nil
	}
	media := socialhub.Media{ID: eventID, URL: content.URL, Type: mediaType, State: socialhub.MediaStateReady}
	if content.Info != nil {
		media.MIME = content.Info.MIMEType
		if content.Info.Size > 0 {
			media.Size = &content.Info.Size
		}
		if content.Info.Width > 0 {
			media.Width = &content.Info.Width
		}
		if content.Info.Height > 0 {
			media.Height = &content.Info.Height
		}
		if content.Info.Duration > 0 {
			duration := time.Duration(content.Info.Duration) * time.Millisecond
			media.Duration = &duration
		}
	}
	return []socialhub.Media{media}
}

func eventExtensions(roomID string, event Event, content MessageContent) map[string]json.RawMessage {
	extensions := map[string]json.RawMessage{}
	addExtension(extensions, "matrix.room_id", roomID)
	addExtension(extensions, "matrix.event_id", event.EventID)
	addExtension(extensions, "matrix.event_type", event.Type)
	addExtension(extensions, "matrix.msgtype", content.MessageType)
	addExtension(extensions, "matrix.formatted_body", content.FormattedBody)
	addExtension(extensions, "matrix.mxc_uri", content.URL)
	return extensions
}

func composeID(roomID, eventID string) string {
	return "mx:" + base64.RawURLEncoding.EncodeToString([]byte(roomID)) + "." + base64.RawURLEncoding.EncodeToString([]byte(eventID))
}

func parseCompositeID(operation, value, defaultRoomID string) (string, string, error) {
	if validEventID(value) && validRoomID(defaultRoomID) {
		return defaultRoomID, value, nil
	}
	if !strings.HasPrefix(value, "mx:") {
		return "", "", invalidArgument(operation, "event ID must be a Matrix composite ID or a raw event ID with a configured default room")
	}
	parts := strings.Split(strings.TrimPrefix(value, "mx:"), ".")
	if len(parts) != 2 {
		return "", "", invalidArgument(operation, "Matrix composite event ID is malformed")
	}
	roomBytes, roomErr := base64.RawURLEncoding.DecodeString(parts[0])
	eventBytes, eventErr := base64.RawURLEncoding.DecodeString(parts[1])
	roomID, eventID := string(roomBytes), string(eventBytes)
	if roomErr != nil || eventErr != nil || !validRoomID(roomID) || !validEventID(eventID) {
		return "", "", invalidArgument(operation, "Matrix composite event ID is malformed")
	}
	return roomID, eventID, nil
}

func matrixTime(milliseconds int64) *time.Time {
	if milliseconds <= 0 {
		return nil
	}
	value := time.UnixMilli(milliseconds).UTC()
	return &value
}

func matrixToURL(roomID, eventID string) string {
	return "https://matrix.to/#/" + url.PathEscape(roomID) + "/" + url.PathEscape(eventID)
}

func replyEventID(relation *Relation) string {
	if relation == nil {
		return ""
	}
	if relation.InReplyTo != nil {
		return relation.InReplyTo.EventID
	}
	if relation.RelationType == RelationThread {
		return relation.EventID
	}
	return ""
}

func optionalCompositeID(roomID, eventID string) *string {
	if eventID == "" {
		return nil
	}
	value := composeID(roomID, eventID)
	return &value
}

func addExtension(target map[string]json.RawMessage, key string, value any) {
	encoded, err := json.Marshal(value)
	if err == nil && string(encoded) != "null" && string(encoded) != `""` {
		target[key] = encoded
	}
}

func pointer(value string) *string { return &value }

func optionalPointer(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func firstError(primary, fallback error) error {
	if primary != nil {
		return primary
	}
	return fallback
}

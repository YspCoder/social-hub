package slack

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

func mapUser(accountID socialhub.AccountID, input wireUser) *socialhub.User {
	displayName := firstNonEmpty(input.Profile.DisplayName, input.Profile.RealName, input.RealName, input.Name)
	accountType := "person"
	if input.IsBot {
		accountType = "bot"
	} else if input.IsAppUser {
		accountType = "app"
	}
	extension, _ := json.Marshal(input)
	return &socialhub.User{
		Platform: "slack", AccountID: accountID, ID: input.ID, Username: stringPointer(input.Name),
		DisplayName: stringPointer(displayName), AvatarURL: stringPointer(input.Profile.Image192), AccountType: &accountType,
		Extensions: map[string]json.RawMessage{"slack.user": extension},
	}
}

func mapPost(accountID socialhub.AccountID, channelID string, input wireMessage, observedAt time.Time) *socialhub.Post {
	id := compositeID(channelID, messageTimestamp(input))
	authorID := firstNonEmpty(input.User, input.BotID)
	visibility := conversationVisibility(channelID)
	post := &socialhub.Post{
		Platform: "slack", AccountID: accountID, ID: id, AuthorID: stringPointer(authorID), Text: stringPointer(input.Text),
		Visibility: &visibility, Status: &socialhub.PublishStatus{ID: id, State: socialhub.PublishStatePublished, UpdatedAt: timePointer(observedAt)},
		Metrics: []socialhub.Metric{metric("thread_replies", input.ReplyCount, observedAt)},
	}
	if created, ok := parseTimestamp(messageTimestamp(input)); ok {
		post.CreatedAt = &created
	}
	if input.ThreadTS != "" && input.ThreadTS != input.TS {
		post.Relations = append(post.Relations, socialhub.PostRelation{Type: socialhub.RelationReply, PostID: compositeID(channelID, input.ThreadTS)})
	}
	for _, file := range input.Files {
		post.Media = append(post.Media, mapFile(file))
	}
	for _, reaction := range input.Reactions {
		post.Metrics = append(post.Metrics, metric("reaction."+reaction.Name, reaction.Count, observedAt))
	}
	extension, _ := json.Marshal(input)
	post.Extensions = map[string]json.RawMessage{"slack.message": extension}
	return post
}

func mapComment(accountID socialhub.AccountID, channelID, rootTS string, input wireMessage, observedAt time.Time) socialhub.Comment {
	post := mapPost(accountID, channelID, input, observedAt)
	comment := socialhub.Comment{
		Platform: "slack", AccountID: accountID, ID: post.ID, PostID: compositeID(channelID, rootTS),
		AuthorID: post.AuthorID, Text: input.Text, CreatedAt: post.CreatedAt, Metrics: post.Metrics,
		Extensions: post.Extensions,
	}
	if input.ThreadTS != "" && input.ThreadTS != rootTS && input.ThreadTS != input.TS {
		comment.ParentID = stringPointer(compositeID(channelID, input.ThreadTS))
	}
	return comment
}

func mapMessage(accountID socialhub.AccountID, actorID, channelID string, input wireMessage) *socialhub.Message {
	authorID := firstNonEmpty(input.User, input.BotID)
	direction := socialhub.DirectionInbound
	if actorID != "" && authorID == actorID {
		direction = socialhub.DirectionOutbound
	}
	message := &socialhub.Message{
		Platform: "slack", AccountID: accountID, ID: compositeID(channelID, messageTimestamp(input)), ConversationID: channelID,
		SenderID: stringPointer(authorID), Text: stringPointer(input.Text), Direction: direction,
	}
	if input.ThreadTS != "" && input.ThreadTS != input.TS {
		message.ReplyToID = stringPointer(compositeID(channelID, input.ThreadTS))
	}
	if sent, ok := parseTimestamp(messageTimestamp(input)); ok {
		message.SentAt = &sent
	}
	for _, file := range input.Files {
		message.Media = append(message.Media, mapFile(file))
	}
	extension, _ := json.Marshal(input)
	message.Extensions = map[string]json.RawMessage{"slack.message": extension}
	return message
}

func mapFile(input wireFile) socialhub.Media {
	mediaType := socialhub.MediaTypeDocument
	switch {
	case strings.HasPrefix(input.Mimetype, "image/"):
		mediaType = socialhub.MediaTypeImage
	case strings.HasPrefix(input.Mimetype, "video/"):
		mediaType = socialhub.MediaTypeVideo
	case strings.HasPrefix(input.Mimetype, "audio/"):
		mediaType = socialhub.MediaTypeAudio
	}
	location := firstNonEmpty(input.URLPrivate, input.Thumb360)
	media := socialhub.Media{
		ID: input.ID, URL: location, MIME: input.Mimetype, Type: mediaType, State: socialhub.MediaStateReady,
		Width: intPointer(input.Width), Height: intPointer(input.Height),
	}
	if input.Size > 0 {
		media.Size = int64Pointer(input.Size)
	}
	if input.DurationMS > 0 {
		duration := time.Duration(input.DurationMS) * time.Millisecond
		media.Duration = &duration
	}
	extension, _ := json.Marshal(input)
	media.Extensions = map[string]json.RawMessage{"slack.file": extension}
	return media
}

func compositeID(channelID, timestamp string) string { return channelID + ":" + timestamp }

func parseCompositeID(value, operation string) (string, string, error) {
	parts := strings.SplitN(strings.TrimSpace(value), ":", 2)
	if len(parts) != 2 || !validSlackID(parts[0], "CGD") || !validTimestamp(parts[1]) {
		return "", "", invalidArgument(operation, "Slack message ID must be channel_id:timestamp")
	}
	return parts[0], parts[1], nil
}

func validTimestamp(value string) bool {
	_, ok := parseTimestamp(value)
	return ok
}

func parseTimestamp(value string) (time.Time, bool) {
	parts := strings.SplitN(strings.TrimSpace(value), ".", 2)
	seconds, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || seconds <= 0 {
		return time.Time{}, false
	}
	nanos := int64(0)
	if len(parts) == 2 {
		fraction := parts[1]
		if fraction == "" || len(fraction) > 9 {
			return time.Time{}, false
		}
		for _, character := range fraction {
			if character < '0' || character > '9' {
				return time.Time{}, false
			}
		}
		fraction += strings.Repeat("0", 9-len(fraction))
		nanos, err = strconv.ParseInt(fraction, 10, 64)
		if err != nil {
			return time.Time{}, false
		}
	}
	return time.Unix(seconds, nanos).UTC(), true
}

func messageTimestamp(input wireMessage) string {
	if input.TS != "" {
		return input.TS
	}
	return input.DeletedTS
}

func conversationVisibility(channelID string) string {
	if strings.HasPrefix(channelID, "D") {
		return "direct"
	}
	if strings.HasPrefix(channelID, "G") {
		return "private"
	}
	return "workspace"
}

func metric(name string, value int, observedAt time.Time) socialhub.Metric {
	return socialhub.Metric{Name: name, Value: float64(value), AsOf: observedAt, Window: "lifetime", Definition: "Slack message counter"}
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
	return &value
}

func intPointer(value int) *int {
	if value <= 0 {
		return nil
	}
	return &value
}

func int64Pointer(value int64) *int64        { return &value }
func timePointer(value time.Time) *time.Time { return &value }

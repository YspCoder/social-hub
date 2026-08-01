package vk

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

func mapUser(accountID socialhub.AccountID, input wireUser) *socialhub.User {
	id := strconv.FormatInt(input.ID, 10)
	name := strings.TrimSpace(input.FirstName + " " + input.LastName)
	username := firstNonEmpty(input.ScreenName, input.Domain)
	accountType := "person"
	extension, _ := json.Marshal(input)
	return &socialhub.User{
		Platform: "vk", AccountID: accountID, ID: id, Username: stringPointer(username), DisplayName: stringPointer(name),
		AvatarURL: stringPointer(input.Photo200), ProfileURL: stringPointer("https://vk.ru/id" + id), AccountType: &accountType,
		Extensions: map[string]json.RawMessage{"vk.user": extension},
	}
}

func mapGroup(accountID socialhub.AccountID, input wireGroup) *socialhub.User {
	id := "-" + strconv.FormatInt(input.ID, 10)
	username := firstNonEmpty(input.ScreenName, "club"+strconv.FormatInt(input.ID, 10))
	accountType := firstNonEmpty(input.Type, "community")
	extension, _ := json.Marshal(input)
	return &socialhub.User{
		Platform: "vk", AccountID: accountID, ID: id, Username: stringPointer(username), DisplayName: stringPointer(input.Name),
		AvatarURL: stringPointer(input.Photo200), ProfileURL: stringPointer("https://vk.ru/" + username), AccountType: &accountType,
		Extensions: map[string]json.RawMessage{"vk.group": extension},
	}
}

func mapPost(accountID socialhub.AccountID, input wirePost, observedAt time.Time) *socialhub.Post {
	id := compositeID(input.OwnerID, input.ID)
	authorID := strconv.FormatInt(input.FromID, 10)
	visibility := "public"
	if input.FriendsOnly != 0 {
		visibility = "friends"
	}
	state := socialhub.PublishStatePublished
	if input.PostType == "postpone" {
		state = socialhub.PublishStatePending
	}
	post := &socialhub.Post{
		Platform: "vk", AccountID: accountID, ID: id, AuthorID: stringPointer(authorID), Text: stringPointer(input.Text),
		URL: stringPointer("https://vk.ru/wall" + id), Visibility: &visibility,
		Status: &socialhub.PublishStatus{ID: id, State: state, UpdatedAt: timePointer(observedAt)},
		Metrics: []socialhub.Metric{
			metric("likes", input.Likes.Count, observedAt), metric("comments", input.Comments.Count, observedAt),
			metric("reposts", input.Reposts.Count, observedAt), metric("views", input.Views.Count, observedAt),
		},
	}
	if input.Date > 0 {
		created := time.Unix(input.Date, 0).UTC()
		post.CreatedAt = &created
	}
	for _, attachment := range input.Attachments {
		if media, ok := mapAttachment(attachment); ok {
			post.Media = append(post.Media, media)
		}
	}
	for _, source := range input.CopyHistory {
		if source.OwnerID != 0 && source.ID != 0 {
			post.Relations = append(post.Relations, socialhub.PostRelation{Type: socialhub.RelationRepost, PostID: compositeID(source.OwnerID, source.ID)})
		}
	}
	extension, _ := json.Marshal(input)
	post.Extensions = map[string]json.RawMessage{"vk.post": extension}
	return post
}

func mapComment(accountID socialhub.AccountID, postID string, ownerID int64, input wireComment, observedAt time.Time) socialhub.Comment {
	if input.PostOwnerID != 0 {
		ownerID = input.PostOwnerID
	} else if input.OwnerID != 0 {
		ownerID = input.OwnerID
	}
	comment := socialhub.Comment{
		Platform: "vk", AccountID: accountID, ID: compositeID(ownerID, input.ID), PostID: postID,
		AuthorID: stringPointer(strconv.FormatInt(input.FromID, 10)), Text: input.Text,
		Metrics: []socialhub.Metric{metric("likes", input.Likes.Count, observedAt), metric("thread_replies", input.Thread.Count, observedAt)},
	}
	if input.ReplyToComment > 0 {
		comment.ParentID = stringPointer(compositeID(ownerID, input.ReplyToComment))
	}
	if input.Date > 0 {
		created := time.Unix(input.Date, 0).UTC()
		comment.CreatedAt = &created
	}
	extension, _ := json.Marshal(input)
	comment.Extensions = map[string]json.RawMessage{"vk.comment": extension}
	return comment
}

func mapMessage(accountID socialhub.AccountID, input wireMessage) *socialhub.Message {
	direction := socialhub.DirectionInbound
	if input.Out != 0 {
		direction = socialhub.DirectionOutbound
	}
	message := &socialhub.Message{
		Platform: "vk", AccountID: accountID, ID: strconv.FormatInt(input.ID, 10),
		ConversationID: strconv.FormatInt(input.PeerID, 10), SenderID: stringPointer(strconv.FormatInt(input.FromID, 10)),
		Text: stringPointer(input.Text), Direction: direction,
	}
	if input.ReplyMessage != nil && input.ReplyMessage.ID > 0 {
		message.ReplyToID = stringPointer(strconv.FormatInt(input.ReplyMessage.ID, 10))
	}
	if input.Date > 0 {
		sent := time.Unix(input.Date, 0).UTC()
		message.SentAt = &sent
	}
	for _, attachment := range input.Attachments {
		if media, ok := mapAttachment(attachment); ok {
			message.Media = append(message.Media, media)
		}
	}
	extension, _ := json.Marshal(input)
	message.Extensions = map[string]json.RawMessage{"vk.message": extension}
	return message
}

func mapAttachment(input wireAttachment) (socialhub.Media, bool) {
	switch input.Type {
	case "photo":
		width, height, location := input.Photo.Width, input.Photo.Height, ""
		for _, size := range input.Photo.Sizes {
			if size.Width*size.Height >= width*height {
				width, height, location = size.Width, size.Height, size.URL
			}
		}
		return socialhub.Media{ID: photoAttachmentID(input.Photo), URL: location, Type: socialhub.MediaTypeImage, Width: intPointer(width), Height: intPointer(height), State: socialhub.MediaStateReady}, true
	case "video":
		location, maxArea := "", 0
		for _, image := range input.Video.Image {
			if area := image.Width * image.Height; location == "" || area > maxArea {
				location, maxArea = image.URL, area
			}
		}
		duration := time.Duration(input.Video.Duration) * time.Second
		return socialhub.Media{ID: fmt.Sprintf("video%d_%d", input.Video.OwnerID, input.Video.ID), URL: location, Type: socialhub.MediaTypeVideo, Duration: &duration, State: socialhub.MediaStateReady}, true
	case "audio":
		duration := time.Duration(input.Audio.Duration) * time.Second
		return socialhub.Media{ID: fmt.Sprintf("audio%d_%d", input.Audio.OwnerID, input.Audio.ID), URL: input.Audio.URL, Type: socialhub.MediaTypeAudio, Duration: &duration, State: socialhub.MediaStateReady}, true
	case "doc":
		size := input.Doc.Size
		return socialhub.Media{ID: fmt.Sprintf("doc%d_%d", input.Doc.OwnerID, input.Doc.ID), URL: input.Doc.URL, MIME: input.Doc.Ext, Type: socialhub.MediaTypeDocument, Size: &size, State: socialhub.MediaStateReady}, true
	default:
		return socialhub.Media{}, false
	}
}

func metric(name string, value int, observedAt time.Time) socialhub.Metric {
	return socialhub.Metric{Name: name, Value: float64(value), AsOf: observedAt, Window: "lifetime", Definition: "VK object counter"}
}

func compositeID(ownerID, itemID int64) string {
	return strconv.FormatInt(ownerID, 10) + "_" + strconv.FormatInt(itemID, 10)
}

func parseCompositeID(value, operation string) (int64, int64, error) {
	value = strings.TrimSpace(strings.TrimPrefix(value, "wall"))
	parts := strings.Split(value, "_")
	if len(parts) != 2 {
		return 0, 0, invalidArgument(operation, "VK object ID must be owner_id_item_id")
	}
	ownerID, ownerErr := strconv.ParseInt(parts[0], 10, 64)
	itemID, itemErr := strconv.ParseInt(parts[1], 10, 64)
	if ownerErr != nil || itemErr != nil || ownerID == 0 || itemID <= 0 {
		return 0, 0, invalidArgument(operation, "VK object ID must contain a non-zero owner and positive item ID")
	}
	return ownerID, itemID, nil
}

func photoAttachmentID(photo wirePhoto) string {
	value := fmt.Sprintf("photo%d_%d", photo.OwnerID, photo.ID)
	if photo.AccessKey != "" {
		value += "_" + photo.AccessKey
	}
	return value
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

func timePointer(value time.Time) *time.Time { return &value }

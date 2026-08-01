package microsoftteams

import (
	"encoding/json"
	"strings"

	"social-hub/pkg/socialhub"
)

func decodeMessage(raw json.RawMessage) (*ChatMessage, error) {
	var message ChatMessage
	if len(raw) == 0 || json.Unmarshal(raw, &message) != nil || !validOpaqueID(message.ID, 2048) {
		return nil, platformError("decode_message", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	message.Raw = append(json.RawMessage(nil), raw...)
	return &message, nil
}

func mapMessage(accountID socialhub.AccountID, ref MessageRef, input ChatMessage, direction socialhub.Direction) socialhub.Message {
	id, _ := EncodeMessageRef(ref)
	conversationID, _ := ConversationRef(ref.Target)
	message := socialhub.Message{
		Platform: "microsoft-teams", AccountID: accountID, ID: id, ConversationID: conversationID,
		SenderID: stringPointer(senderID(input.From)), SentAt: input.CreatedDateTime, Direction: direction,
		Media: mapMedia(input), Extensions: messageExtensions(input),
	}
	if strings.EqualFold(input.Body.ContentType, "text") {
		message.Text = stringPointer(input.Body.Content)
	}
	if ref.ReplyID != "" {
		parentID, _ := EncodeMessageRef(MessageRef{Target: ref.Target, RootID: ref.RootID})
		message.ReplyToID = &parentID
	}
	return message
}

func mapPost(accountID socialhub.AccountID, ref MessageRef, input ChatMessage) socialhub.Post {
	id, _ := EncodeMessageRef(ref)
	post := socialhub.Post{
		Platform: "microsoft-teams", AccountID: accountID, ID: id,
		AuthorID: stringPointer(senderID(input.From)), CreatedAt: input.CreatedDateTime,
		Media: mapMedia(input), URL: stringPointer(input.WebURL), Extensions: messageExtensions(input),
		Status: &socialhub.PublishStatus{ID: id, State: socialhub.PublishStatePublished, UpdatedAt: input.LastModifiedDateTime},
	}
	visibility := "organization"
	post.Visibility = &visibility
	if strings.EqualFold(input.Body.ContentType, "text") {
		post.Text = stringPointer(input.Body.Content)
	}
	if ref.ReplyID != "" {
		parentID, _ := EncodeMessageRef(MessageRef{Target: ref.Target, RootID: ref.RootID})
		post.Relations = []socialhub.PostRelation{{Type: socialhub.RelationReply, PostID: parentID}}
	}
	return post
}

func mapComment(accountID socialhub.AccountID, ref MessageRef, input ChatMessage) socialhub.Comment {
	id, _ := EncodeMessageRef(ref)
	postID, _ := EncodeMessageRef(MessageRef{Target: ref.Target, RootID: ref.RootID})
	comment := socialhub.Comment{
		Platform: "microsoft-teams", AccountID: accountID, ID: id, PostID: postID,
		AuthorID: stringPointer(senderID(input.From)), CreatedAt: input.CreatedDateTime,
		Text: input.Body.Content, Extensions: messageExtensions(input),
	}
	return comment
}

func senderID(from *IdentitySet) string {
	if from == nil {
		return ""
	}
	if from.User != nil {
		return from.User.ID
	}
	if from.Application != nil {
		return from.Application.ID
	}
	return ""
}

func (c *Client) direction(input ChatMessage) socialhub.Direction {
	if c.actorID != "" && senderID(input.From) == c.actorID {
		return socialhub.DirectionOutbound
	}
	return socialhub.DirectionInbound
}

func mapMedia(input ChatMessage) []socialhub.Media {
	media := make([]socialhub.Media, 0, len(input.HostedContents)+len(input.Attachments))
	for _, hosted := range input.HostedContents {
		media = append(media, socialhub.Media{ID: hosted.ID, MIME: hosted.ContentType, Type: mediaType(hosted.ContentType), State: socialhub.MediaStateReady})
	}
	for _, attachment := range input.Attachments {
		mime := ""
		if strings.Contains(attachment.ContentType, "/") {
			mime = attachment.ContentType
		}
		media = append(media, socialhub.Media{ID: attachment.ID, URL: attachment.ContentURL, MIME: mime, Type: mediaType(attachment.ContentType), State: socialhub.MediaStateReady})
	}
	return media
}

func mediaType(contentType string) socialhub.MediaType {
	switch {
	case strings.HasPrefix(strings.ToLower(contentType), "image/"):
		return socialhub.MediaTypeImage
	case strings.HasPrefix(strings.ToLower(contentType), "video/"):
		return socialhub.MediaTypeVideo
	case strings.HasPrefix(strings.ToLower(contentType), "audio/"):
		return socialhub.MediaTypeAudio
	default:
		return socialhub.MediaTypeDocument
	}
}

func messageExtensions(input ChatMessage) map[string]json.RawMessage {
	raw := input.Raw
	if len(raw) == 0 {
		raw, _ = json.Marshal(input)
	}
	return map[string]json.RawMessage{"microsoft_graph.chat_message": append(json.RawMessage(nil), raw...)}
}

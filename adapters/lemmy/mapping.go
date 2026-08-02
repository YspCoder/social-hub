package lemmy

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

func (client *Client) mapUser(input wirePersonView) *socialhub.User {
	accountType := "member"
	if input.IsAdmin {
		accountType = "admin"
	} else if input.Person.BotAccount {
		accountType = "bot"
	}
	return &socialhub.User{
		Platform: "lemmy", AccountID: client.accountID, ID: strconv.FormatInt(input.Person.ID, 10),
		Username: stringPointer(input.Person.Name), DisplayName: stringPointer(input.Person.DisplayName),
		AvatarURL: client.absoluteURL(input.Person.Avatar), ProfileURL: client.absoluteURL(input.Person.ActorID), AccountType: &accountType,
		Extensions: map[string]json.RawMessage{"lemmy.person_view": append(json.RawMessage(nil), input.Raw...)},
	}
}

func (client *Client) mapPost(input wirePostView) Post {
	id := strconv.FormatInt(input.Post.ID, 10)
	authorID := optionalDecimalID(input.Post.CreatorID)
	languageID := ""
	if input.Post.LanguageID > 0 {
		languageID = strconv.FormatInt(input.Post.LanguageID, 10)
	}
	visibility := strings.ToLower(input.Community.Visibility)
	if visibility == "" {
		visibility = "public"
	}
	state := socialhub.PublishStatePublished
	if input.Post.Deleted {
		state, visibility = socialhub.PublishStateFailed, "deleted"
	} else if input.Post.Removed {
		state, visibility = socialhub.PublishStateFailed, "removed"
	}
	createdAt := parseTimestamp(input.Post.Published)
	updatedAt := parseTimestamp(input.Post.Updated)
	observedAt := client.clock.Now()
	common := socialhub.Post{
		Platform: "lemmy", AccountID: client.accountID, ID: id,
		AuthorID: authorID,
		Text:     stringPointer(firstNonEmpty(input.Post.Body, input.Post.Name)), CreatedAt: createdAt,
		URL: client.absoluteURL(input.Post.APID), Visibility: &visibility,
		Status: &socialhub.PublishStatus{ID: id, State: state, UpdatedAt: updatedAt},
		Metrics: []socialhub.Metric{
			{Name: "comments", Value: float64(input.Counts.Comments), AsOf: observedAt, Definition: "Lemmy post comment count"},
			{Name: "score", Value: float64(input.Counts.Score), AsOf: observedAt, Definition: "Lemmy net vote score"},
			{Name: "upvotes", Value: float64(input.Counts.Upvotes), AsOf: observedAt, Definition: "Lemmy post upvote count"},
			{Name: "downvotes", Value: float64(input.Counts.Downvotes), AsOf: observedAt, Definition: "Lemmy post downvote count"},
		},
		Extensions: map[string]json.RawMessage{"lemmy.post_view": append(json.RawMessage(nil), input.Raw...)},
	}
	if media := client.mapPostMedia(input); media != nil {
		common.Media = append(common.Media, *media)
	}
	return Post{
		Common: common, Title: input.Post.Name, Body: input.Post.Body, ExternalURL: input.Post.URL, AltText: input.Post.AltText,
		CommunityID: strconv.FormatInt(input.Post.CommunityID, 10), CommunityName: input.Community.Name,
		CommunityTitle: input.Community.Title, CommunityActorID: input.Community.ActorID,
		ActivityPubID: input.Post.APID, LanguageID: languageID,
		Local: input.Post.Local, NSFW: input.Post.NSFW, Locked: input.Post.Locked, Removed: input.Post.Removed,
		Deleted: input.Post.Deleted, FeaturedCommunity: input.Post.FeaturedCommunity, FeaturedLocal: input.Post.FeaturedLocal,
		Score: input.Counts.Score, Upvotes: input.Counts.Upvotes, Downvotes: input.Counts.Downvotes,
		Comments: input.Counts.Comments, Raw: append(json.RawMessage(nil), input.Raw...),
	}
}

func (client *Client) mapPostMedia(input wirePostView) *socialhub.Media {
	mediaURL, mimeType := "", ""
	width, height := 0, 0
	if input.ImageDetails != nil && isMediaMIME(input.ImageDetails.ContentType) {
		mediaURL, mimeType = input.ImageDetails.Link, input.ImageDetails.ContentType
		width, height = input.ImageDetails.Width, input.ImageDetails.Height
	} else if isMediaMIME(input.Post.URLContentType) {
		mediaURL, mimeType = input.Post.URL, input.Post.URLContentType
	} else if input.Post.ThumbnailURL != "" {
		mediaURL, mimeType = input.Post.ThumbnailURL, "image/*"
	}
	resolved := client.absoluteURL(mediaURL)
	if resolved == nil {
		return nil
	}
	mediaType := mediaTypeFromMIME(mimeType)
	media := &socialhub.Media{URL: *resolved, MIME: mimeType, Type: mediaType, State: socialhub.MediaStateReady}
	if width > 0 {
		media.Width = intPointer(width)
	}
	if height > 0 {
		media.Height = intPointer(height)
	}
	return media
}

func (client *Client) mapComment(input wireCommentView) (socialhub.Comment, error) {
	if input.Comment.ID <= 0 || input.Comment.PostID <= 0 {
		return socialhub.Comment{}, platformError("map_comment", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	parentID, err := parentFromPath(input.Comment.Path, input.Comment.ID)
	if err != nil {
		return socialhub.Comment{}, platformError("map_comment", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	observedAt := client.clock.Now()
	return socialhub.Comment{
		Platform: "lemmy", AccountID: client.accountID, ID: strconv.FormatInt(input.Comment.ID, 10),
		PostID: strconv.FormatInt(input.Comment.PostID, 10), AuthorID: optionalDecimalID(input.Comment.CreatorID),
		ParentID: stringPointer(parentID), Text: input.Comment.Content, CreatedAt: parseTimestamp(input.Comment.Published),
		Metrics: []socialhub.Metric{
			{Name: "score", Value: float64(input.Counts.Score), AsOf: observedAt, Definition: "Lemmy comment net vote score"},
			{Name: "upvotes", Value: float64(input.Counts.Upvotes), AsOf: observedAt, Definition: "Lemmy comment upvote count"},
			{Name: "downvotes", Value: float64(input.Counts.Downvotes), AsOf: observedAt, Definition: "Lemmy comment downvote count"},
			{Name: "children", Value: float64(input.Counts.ChildCount), AsOf: observedAt, Definition: "Lemmy direct child comment count"},
		},
		Extensions: map[string]json.RawMessage{"lemmy.comment_view": append(json.RawMessage(nil), input.Raw...)},
	}, nil
}

func (client *Client) mapPrivateMessage(input wirePrivateMessageView) PrivateMessage {
	id := strconv.FormatInt(input.PrivateMessage.ID, 10)
	creatorID := strconv.FormatInt(input.PrivateMessage.CreatorID, 10)
	recipientID := strconv.FormatInt(input.PrivateMessage.RecipientID, 10)
	conversationID := creatorID + ":" + recipientID
	if input.PrivateMessage.RecipientID < input.PrivateMessage.CreatorID {
		conversationID = recipientID + ":" + creatorID
	}
	direction := socialhub.DirectionInbound
	if input.Creator.Name == client.username {
		direction = socialhub.DirectionOutbound
	}
	text := input.PrivateMessage.Content
	return PrivateMessage{
		Common: socialhub.Message{
			Platform: "lemmy", AccountID: client.accountID, ID: id,
			ConversationID: conversationID, SenderID: &creatorID, RecipientIDs: []string{recipientID},
			Text: &text, SentAt: parseTimestamp(input.PrivateMessage.Published), Direction: direction,
			Extensions: map[string]json.RawMessage{"lemmy.private_message_view": append(json.RawMessage(nil), input.Raw...)},
		},
		ActivityPubID: input.PrivateMessage.APID, Deleted: input.PrivateMessage.Deleted,
		Read: input.PrivateMessage.Read, Local: input.PrivateMessage.Local, Raw: append(json.RawMessage(nil), input.Raw...),
	}
}

func optionalDecimalID(value int64) *string {
	if value <= 0 {
		return nil
	}
	return stringPointer(strconv.FormatInt(value, 10))
}

func parentFromPath(path string, commentID int64) (string, error) {
	if path == "" {
		return "", nil
	}
	parts := strings.Split(path, ".")
	if len(parts) < 2 || parts[0] != "0" {
		return "", fmt.Errorf("lemmy: invalid comment path")
	}
	ids := make([]int64, 0, len(parts)-1)
	for _, part := range parts[1:] {
		if !validID(part) {
			return "", fmt.Errorf("lemmy: invalid comment path")
		}
		ids = append(ids, mustID(part))
	}
	if ids[len(ids)-1] != commentID {
		return "", fmt.Errorf("lemmy: comment path does not end in comment id")
	}
	if len(ids) == 1 {
		return "", nil
	}
	return strconv.FormatInt(ids[len(ids)-2], 10), nil
}

func parseTimestamp(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05.999999", "2006-01-02T15:04:05"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			parsed = parsed.UTC()
			return &parsed
		}
	}
	return nil
}

func (client *Client) absoluteURL(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	base, err := url.Parse(client.baseURL + "/")
	if err != nil {
		return nil
	}
	reference, err := url.Parse(value)
	if err != nil {
		return nil
	}
	resolved := base.ResolveReference(reference)
	if resolved.Scheme != "http" && resolved.Scheme != "https" || resolved.User != nil {
		return nil
	}
	result := resolved.String()
	return &result
}

func isMediaMIME(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(value, "image/") || strings.HasPrefix(value, "video/") || strings.HasPrefix(value, "audio/")
}

func mediaTypeFromMIME(value string) socialhub.MediaType {
	value = strings.ToLower(value)
	switch {
	case value == "image/gif":
		return socialhub.MediaTypeAnimation
	case strings.HasPrefix(value, "video/"):
		return socialhub.MediaTypeVideo
	case strings.HasPrefix(value, "audio/"):
		return socialhub.MediaTypeAudio
	default:
		return socialhub.MediaTypeImage
	}
}

func intPointer(value int) *int {
	copy := value
	return &copy
}

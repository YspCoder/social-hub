package wordpresscom

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

func mapUser(accountID socialhub.AccountID, input wpUser) *socialhub.User {
	extension := input.Raw
	if len(extension) == 0 {
		extension, _ = json.Marshal(input)
	}
	accountType := "wordpress.com"
	return &socialhub.User{
		Platform: "wordpress.com", AccountID: accountID, ID: strconv.FormatInt(input.ID, 10),
		Username: stringPointer(firstNonEmpty(input.Username, input.Login)), DisplayName: stringPointer(firstNonEmpty(input.DisplayName, input.Name)),
		AvatarURL: stringPointer(input.AvatarURL), ProfileURL: stringPointer(firstNonEmpty(input.ProfileURL, input.URL)), AccountType: &accountType,
		Extensions: map[string]json.RawMessage{"wordpress.user": append(json.RawMessage(nil), extension...)},
	}
}

func mapPost(accountID socialhub.AccountID, input wpPost, observedAt time.Time) *socialhub.Post {
	id := strconv.FormatInt(input.ID, 10)
	text := firstNonEmpty(input.Content, input.Title, input.Excerpt)
	visibility := input.Status
	if input.Status == "publish" {
		visibility = "public"
	}
	extension := input.Raw
	if len(extension) == 0 {
		extension, _ = json.Marshal(input)
	}
	post := &socialhub.Post{
		Platform: "wordpress.com", AccountID: accountID, ID: id,
		Text: stringPointer(text), CreatedAt: input.Date, URL: stringPointer(firstNonEmpty(input.URL, input.ShortURL)),
		Visibility: stringPointer(visibility), Status: mapPublishStatus(id, input.Status, input.Modified, input.Date, observedAt),
		Extensions: map[string]json.RawMessage{"wordpress.post": append(json.RawMessage(nil), extension...)},
		Metrics: []socialhub.Metric{
			{Name: "comments", Value: float64(input.CommentCount), AsOf: observedAt, Definition: "WordPress Post comment count"},
			{Name: "likes", Value: float64(input.LikeCount), AsOf: observedAt, Definition: "WordPress.com Post like count"},
		},
	}
	if input.Author.ID > 0 {
		post.AuthorID = stringPointer(strconv.FormatInt(input.Author.ID, 10))
	}
	keys := make([]string, 0, len(input.Attachments))
	for key := range input.Attachments {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		media := mapMedia(input.Attachments[key])
		if media.URL != "" {
			post.Media = append(post.Media, media)
		}
	}
	if len(post.Media) == 0 && input.FeaturedImage != "" {
		post.Media = append(post.Media, socialhub.Media{URL: input.FeaturedImage, Type: socialhub.MediaTypeImage, State: socialhub.MediaStateReady})
	}
	return post
}

func mapPublishStatus(id, status string, modified, created *time.Time, observedAt time.Time) *socialhub.PublishStatus {
	state := socialhub.PublishStatePending
	switch status {
	case "publish", "private":
		state = socialhub.PublishStatePublished
	case "trash", "deleted":
		state = socialhub.PublishStateFailed
	}
	updated := modified
	if updated == nil {
		updated = created
	}
	if updated == nil {
		updated = &observedAt
	}
	return &socialhub.PublishStatus{ID: id, State: state, Message: status, UpdatedAt: updated}
}

func mapComment(accountID socialhub.AccountID, postID string, input wpComment, observedAt time.Time) socialhub.Comment {
	if postID == "" {
		postID = referenceID(input.Post)
	}
	text := firstNonEmpty(input.RawContent, input.Content)
	extension := input.Raw
	if len(extension) == 0 {
		extension, _ = json.Marshal(input)
	}
	comment := socialhub.Comment{
		Platform: "wordpress.com", AccountID: accountID, ID: strconv.FormatInt(input.ID, 10), PostID: postID,
		Text: text, CreatedAt: input.Date,
		Metrics:    []socialhub.Metric{{Name: "likes", Value: float64(input.LikeCount), AsOf: observedAt, Definition: "WordPress.com comment like count"}},
		Extensions: map[string]json.RawMessage{"wordpress.comment": append(json.RawMessage(nil), extension...)},
	}
	if input.Author.ID > 0 {
		comment.AuthorID = stringPointer(strconv.FormatInt(input.Author.ID, 10))
	}
	if parent := referenceID(input.Parent); parent != "" {
		comment.ParentID = stringPointer(parent)
	}
	return comment
}

func referenceID(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "false" || string(raw) == "null" {
		return ""
	}
	var reference wpReference
	if json.Unmarshal(raw, &reference) == nil && reference.ID > 0 {
		return strconv.FormatInt(reference.ID, 10)
	}
	var id int64
	if json.Unmarshal(raw, &id) == nil && id > 0 {
		return strconv.FormatInt(id, 10)
	}
	return ""
}

func mapMedia(input wpMedia) socialhub.Media {
	mediaType := mediaType(input.MIME, input.Extension)
	extension := input.Raw
	if len(extension) == 0 {
		extension, _ = json.Marshal(input)
	}
	media := socialhub.Media{
		ID: strconv.FormatInt(input.ID, 10), URL: firstNonEmpty(input.URL, input.GUID), MIME: input.MIME,
		Type: mediaType, State: socialhub.MediaStateReady,
		Extensions: map[string]json.RawMessage{"wordpress.media": append(json.RawMessage(nil), extension...)},
	}
	if input.Size > 0 {
		size := input.Size
		media.Size = &size
	}
	if input.Width > 0 {
		media.Width = intPointer(input.Width)
	}
	if input.Height > 0 {
		media.Height = intPointer(input.Height)
	}
	if input.Length > 0 && (mediaType == socialhub.MediaTypeAudio || mediaType == socialhub.MediaTypeVideo) {
		duration := time.Duration(input.Length) * time.Second
		media.Duration = &duration
	}
	return media
}

func mediaType(mimeType, extension string) socialhub.MediaType {
	switch {
	case strings.HasPrefix(mimeType, "image/gif"):
		return socialhub.MediaTypeAnimation
	case strings.HasPrefix(mimeType, "image/"):
		return socialhub.MediaTypeImage
	case strings.HasPrefix(mimeType, "video/"):
		return socialhub.MediaTypeVideo
	case strings.HasPrefix(mimeType, "audio/"):
		return socialhub.MediaTypeAudio
	case strings.EqualFold(extension, "gif"):
		return socialhub.MediaTypeAnimation
	default:
		return socialhub.MediaTypeDocument
	}
}

func stringPointer(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	copy := value
	return &copy
}

func intPointer(value int) *int {
	copy := value
	return &copy
}

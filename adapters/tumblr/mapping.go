package tumblr

import (
	"encoding/json"
	"math"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

func mapBlog(accountID socialhub.AccountID, blog tumblrBlog) *socialhub.User {
	id := firstNonEmpty(blog.UUID, blog.Name)
	avatar := ""
	for _, candidate := range blog.Avatar {
		if candidate.URL != "" {
			avatar = candidate.URL
			break
		}
	}
	accountType := "blog"
	extension, _ := json.Marshal(blog)
	return &socialhub.User{
		Platform: "tumblr", AccountID: accountID, ID: id,
		Username: stringPointer(blog.Name), DisplayName: stringPointer(blog.Title), AvatarURL: stringPointer(avatar),
		ProfileURL: stringPointer(blog.URL), AccountType: &accountType,
		Extensions: map[string]json.RawMessage{"tumblr.blog": extension},
	}
}

func mapPost(accountID socialhub.AccountID, input tumblrPost, now time.Time) *socialhub.Post {
	id := input.identifier()
	authorID := firstNonEmpty(input.TumblelogUUID, input.Blog.UUID, input.BlogName, input.Blog.Name)
	text := postText(input)
	createdAt := unixPointer(input.Timestamp)
	visibility := input.State
	if visibility == "" {
		visibility = "published"
	}
	status := postStatus(id, visibility, createdAt, now)
	post := &socialhub.Post{
		Platform: "tumblr", AccountID: accountID, ID: id, AuthorID: stringPointer(authorID), Text: stringPointer(text),
		Media: mapPostMedia(input.Content), CreatedAt: createdAt, URL: stringPointer(firstNonEmpty(input.PostURL, input.ShortURL)),
		Visibility: stringPointer(visibility), Status: status,
		Metrics: []socialhub.Metric{{
			Name: "tumblr.note_count", Value: float64(input.NoteCount), AsOf: now,
			Definition: "Tumblr aggregate count of likes, replies, and reblogs",
		}},
	}
	if parentID := string(input.ParentPostID); parentID != "" {
		post.Relations = append(post.Relations, socialhub.PostRelation{Type: socialhub.RelationRepost, PostID: parentID})
	}
	raw := input.Raw
	if len(raw) == 0 {
		raw, _ = json.Marshal(input)
	}
	post.Extensions = map[string]json.RawMessage{"tumblr.post": append(json.RawMessage(nil), raw...)}
	return post
}

func mapPosts(accountID socialhub.AccountID, posts []tumblrPost, now time.Time) []socialhub.Post {
	items := make([]socialhub.Post, 0, len(posts))
	for _, post := range posts {
		if post.identifier() == "" {
			continue
		}
		items = append(items, *mapPost(accountID, post, now))
	}
	return items
}

func mapNPFPost(input tumblrPost) *NPFPost {
	return &NPFPost{
		ID: input.identifier(), BlogUUID: firstNonEmpty(input.TumblelogUUID, input.Blog.UUID), BlogName: firstNonEmpty(input.BlogName, input.Blog.Name),
		PostURL: input.PostURL, ParentPostID: string(input.ParentPostID), ParentBlogUUID: input.ParentTumblelogUUID,
		ReblogKey: input.ReblogKey, State: NPFState(input.State), QueuedState: input.QueuedState,
		ScheduledPublishTime: unixPointer(input.ScheduledPublishTime), PublishOn: input.PublishOn,
		InteractabilityReblog: input.InteractabilityReblog, Timestamp: unixPointer(input.Timestamp),
		Tags: append([]string(nil), input.Tags...), Content: cloneRawMessages(input.Content), Layout: cloneRawMessages(input.Layout),
		Trail: cloneRawMessages(input.Trail), Raw: append(json.RawMessage(nil), input.Raw...),
	}
}

func mapNote(input tumblrNote) Note {
	return Note{
		Type: input.Type, Timestamp: unixFloatPointer(input.Timestamp), BlogName: input.BlogName, BlogUUID: input.BlogUUID,
		BlogURL: input.BlogURL, PostID: string(input.PostID), ReplyID: string(input.ReplyID),
		ReplyText: input.ReplyText, AddedText: input.AddedText, Tags: append([]string(nil), input.Tags...),
		Raw: append(json.RawMessage(nil), input.Raw...),
	}
}

func mapNotes(items []tumblrNote) []Note {
	result := make([]Note, 0, len(items))
	for _, item := range items {
		result = append(result, mapNote(item))
	}
	return result
}

func postStatus(id, state string, observed *time.Time, now time.Time) *socialhub.PublishStatus {
	publishState := socialhub.PublishStatePending
	if state == "published" || state == "private" {
		publishState = socialhub.PublishStatePublished
	}
	updated := observed
	if updated == nil {
		updated = &now
	}
	return &socialhub.PublishStatus{ID: id, State: publishState, UpdatedAt: updated}
}

func postText(input tumblrPost) string {
	parts := make([]string, 0, len(input.Content))
	for _, raw := range input.Content {
		var block tumblrContentBlock
		if json.Unmarshal(raw, &block) == nil && block.Type == "text" && strings.TrimSpace(block.Text) != "" {
			parts = append(parts, block.Text)
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, "\n\n")
	}
	legacyBody := input.Body
	if strings.TrimSpace(legacyBody) == "" {
		legacyBody = input.Summary
	}
	return strings.TrimSpace(strings.Join(nonEmptyStrings(input.Title, legacyBody), "\n\n"))
}

func mapPostMedia(content []json.RawMessage) []socialhub.Media {
	var result []socialhub.Media
	for _, raw := range content {
		var block tumblrContentBlock
		if json.Unmarshal(raw, &block) != nil {
			continue
		}
		mediaType := socialhub.MediaType("")
		switch block.Type {
		case "image":
			mediaType = socialhub.MediaTypeImage
		case "video":
			mediaType = socialhub.MediaTypeVideo
		case "audio":
			mediaType = socialhub.MediaTypeAudio
		default:
			continue
		}
		objects := decodeMediaObjects(block.Media)
		if len(objects) == 0 && block.URL != "" {
			objects = []tumblrMediaObject{{URL: block.URL}}
		}
		if len(objects) == 0 || objects[0].URL == "" {
			continue
		}
		selected := objects[0]
		media := socialhub.Media{ID: selected.URL, URL: selected.URL, MIME: selected.Type, Type: mediaType, State: socialhub.MediaStateReady}
		if selected.Width > 0 {
			media.Width = intPointer(selected.Width)
		}
		if selected.Height > 0 {
			media.Height = intPointer(selected.Height)
		}
		if selected.Duration > 0 {
			duration := time.Duration(selected.Duration * float64(time.Second))
			media.Duration = &duration
		}
		result = append(result, media)
	}
	return result
}

func decodeMediaObjects(raw json.RawMessage) []tumblrMediaObject {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var list []tumblrMediaObject
	if json.Unmarshal(raw, &list) == nil {
		return list
	}
	var single tumblrMediaObject
	if json.Unmarshal(raw, &single) == nil {
		return []tumblrMediaObject{single}
	}
	return nil
}

func unixPointer(seconds int64) *time.Time {
	if seconds <= 0 {
		return nil
	}
	value := time.Unix(seconds, 0).UTC()
	return &value
}

func unixFloatPointer(seconds float64) *time.Time {
	if seconds <= 0 || math.IsInf(seconds, 0) || math.IsNaN(seconds) {
		return nil
	}
	whole, fraction := math.Modf(seconds)
	value := time.Unix(int64(whole), int64(fraction*float64(time.Second))).UTC()
	return &value
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

func cloneRawMessages(values []json.RawMessage) []json.RawMessage {
	result := make([]json.RawMessage, len(values))
	for index, value := range values {
		result[index] = append(json.RawMessage(nil), value...)
	}
	return result
}

func nonEmptyStrings(values ...string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			result = append(result, value)
		}
	}
	return result
}

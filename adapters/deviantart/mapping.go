package deviantart

import (
	"encoding/json"
	"html"
	"strconv"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

func mapUser(accountID socialhub.AccountID, input User, profile *Profile) *socialhub.User {
	extension, _ := json.Marshal(input)
	displayName, profileURL := "", ""
	if profile != nil {
		extension, _ = json.Marshal(profile)
		displayName, profileURL = profile.RealName, profile.ProfileURL
	} else if input.Profile != nil {
		displayName = input.Profile.RealName
	}
	accountType := input.Type
	return &socialhub.User{
		Platform: "deviantart", AccountID: accountID, ID: input.UserID,
		Username: stringPointer(input.Username), DisplayName: stringPointer(displayName), AvatarURL: stringPointer(input.UserIcon),
		ProfileURL: stringPointer(profileURL), AccountType: stringPointer(accountType),
		Extensions: map[string]json.RawMessage{"deviantart.user": extension},
	}
}

func (client *Client) mapDeviation(input Deviation) (*socialhub.Post, error) {
	if !validResourceID(input.DeviationID) {
		return nil, platformError("map_deviation", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	extension, _ := json.Marshal(input)
	createdAt := parseTimestamp(input.PublishedTime)
	text := firstNonEmpty(editorExcerpt(input.TextContent), input.Excerpt, input.FormattedExcerpt, input.Title)
	post := &socialhub.Post{
		Platform: "deviantart", AccountID: client.accountID, ID: input.DeviationID,
		AuthorID: stringPointer(input.Author.UserID), Text: stringPointer(html.UnescapeString(text)), CreatedAt: createdAt,
		URL: stringPointer(input.URL), Visibility: stringPointer("public"),
		Extensions: map[string]json.RawMessage{"deviantart.deviation": extension},
	}
	if input.IsPublished || createdAt != nil {
		post.Status = &socialhub.PublishStatus{ID: input.DeviationID, State: socialhub.PublishStatePublished, UpdatedAt: createdAt}
	}
	post.Media = mapMedia(input)
	observedAt := client.clock.Now()
	post.Metrics = []socialhub.Metric{
		{Name: "comments", Value: float64(input.Stats.Comments), AsOf: observedAt, Definition: "DeviantArt Deviation comment count"},
		{Name: "favourites", Value: float64(input.Stats.Favourites), AsOf: observedAt, Definition: "DeviantArt Deviation favourite count"},
	}
	return post, nil
}

func mapMedia(input Deviation) []socialhub.Media {
	media := make([]socialhub.Media, 0, max(1, len(input.Videos)))
	for index, video := range input.Videos {
		if strings.TrimSpace(video.Src) == "" {
			continue
		}
		item := socialhub.Media{
			ID: input.DeviationID + ":video:" + strconv.Itoa(index), URL: video.Src, Type: socialhub.MediaTypeVideo,
			Size: int64Pointer(video.FileSize), Duration: durationPointer(video.Duration), State: socialhub.MediaStateReady,
		}
		media = append(media, item)
	}
	if len(media) > 0 {
		return media
	}
	image := input.Content
	if image == nil || image.Src == "" {
		image = input.Preview
	}
	if image == nil || image.Src == "" {
		image = input.SocialPreview
	}
	if image != nil && image.Src != "" {
		media = append(media, socialhub.Media{
			ID: input.DeviationID, URL: image.Src, Type: socialhub.MediaTypeImage, Size: int64Pointer(image.FileSize),
			Width: intPointer(image.Width), Height: intPointer(image.Height), State: socialhub.MediaStateReady,
		})
	}
	return media
}

func (client *Client) mapComment(postID string, input Comment) (*socialhub.Comment, error) {
	if !validResourceID(postID) || !validResourceID(input.CommentID) {
		return nil, platformError("map_comment", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	extension, _ := json.Marshal(input)
	return &socialhub.Comment{
		Platform: "deviantart", AccountID: client.accountID, ID: input.CommentID, PostID: postID,
		AuthorID: stringPointer(input.User.UserID), ParentID: cleanStringPointer(input.ParentID), Text: input.Body,
		CreatedAt:  parseTimestamp(input.Posted),
		Metrics:    []socialhub.Metric{{Name: "likes", Value: float64(input.Likes), AsOf: client.clock.Now(), Definition: "DeviantArt comment like count"}},
		Extensions: map[string]json.RawMessage{"deviantart.comment": extension},
	}, nil
}

func parseTimestamp(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return &parsed
		}
	}
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds >= 0 {
		parsed := time.Unix(seconds, 0).UTC()
		return &parsed
	}
	return nil
}

func editorExcerpt(value *EditorText) string {
	if value == nil {
		return ""
	}
	return value.Excerpt
}

func stringPointer(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	copy := value
	return &copy
}

func cleanStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	return stringPointer(*value)
}

func intPointer(value int) *int {
	if value <= 0 {
		return nil
	}
	copy := value
	return &copy
}

func int64Pointer(value int64) *int64 {
	if value <= 0 {
		return nil
	}
	copy := value
	return &copy
}

func durationPointer(seconds int64) *time.Duration {
	if seconds <= 0 || seconds > int64((365*24*time.Hour)/time.Second) {
		return nil
	}
	value := time.Duration(seconds) * time.Second
	return &value
}

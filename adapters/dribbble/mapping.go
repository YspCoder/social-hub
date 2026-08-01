package dribbble

import (
	"encoding/json"
	"html"
	"strconv"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

func mapUser(accountID socialhub.AccountID, input User) *socialhub.User {
	extension, _ := json.Marshal(input)
	accountType := "member"
	if input.CanUploadShot {
		accountType = "player"
	}
	return &socialhub.User{
		Platform: "dribbble", AccountID: accountID, ID: strconv.FormatInt(input.ID, 10),
		Username: stringPointer(input.Login), DisplayName: stringPointer(html.UnescapeString(input.Name)), AvatarURL: stringPointer(input.AvatarURL),
		ProfileURL: stringPointer(input.HTMLURL), AccountType: stringPointer(accountType),
		Extensions: map[string]json.RawMessage{"dribbble.user": extension},
	}
}

func mapShot(accountID socialhub.AccountID, input Shot, observedAt time.Time) *socialhub.Post {
	id := strconv.FormatInt(input.ID, 10)
	extension, _ := json.Marshal(input)
	post := &socialhub.Post{
		Platform: "dribbble", AccountID: accountID, ID: id,
		Text: stringPointer(firstNonEmpty(input.DescriptionText, input.Description)), CreatedAt: timePointer(input.PublishedAt),
		URL: stringPointer(input.HTMLURL), Visibility: stringPointer("public"),
		Status:     &socialhub.PublishStatus{ID: id, State: socialhub.PublishStatePublished, UpdatedAt: timePointer(input.UpdatedAt)},
		Extensions: map[string]json.RawMessage{"dribbble.shot": extension},
	}
	owner := input.User
	if owner == nil {
		owner = input.Team
	}
	if owner != nil && owner.ID > 0 {
		post.AuthorID = stringPointer(strconv.FormatInt(owner.ID, 10))
	}
	if input.Video != nil && input.Video.URL != "" {
		duration := time.Duration(input.Video.Duration) * time.Second
		post.Media = []socialhub.Media{{
			ID: strconv.FormatInt(input.Video.ID, 10), URL: input.Video.URL, Type: socialhub.MediaTypeVideo, MIME: "video/" + videoSubtype(input.Video.Filename),
			Size: int64Pointer(input.Video.Size), Width: intPointer(input.Video.Width), Height: intPointer(input.Video.Height), Duration: durationPointer(duration), State: socialhub.MediaStateReady,
		}}
	} else if imageURL := firstNonEmpty(input.Images.HiDPI, input.Images.Normal, input.Images.Teaser); imageURL != "" {
		mediaType := socialhub.MediaTypeImage
		if input.Animated {
			mediaType = socialhub.MediaTypeAnimation
		}
		post.Media = []socialhub.Media{{ID: id, URL: imageURL, Type: mediaType, Width: intPointer(input.Width), Height: intPointer(input.Height), State: socialhub.MediaStateReady}}
	}
	metrics := []struct {
		name       string
		value      int64
		definition string
	}{
		{"views", input.ViewsCount, "Dribbble Shot view count"}, {"likes", input.LikesCount, "Dribbble Shot like count"},
		{"comments", input.CommentsCount, "Dribbble Shot comment count"}, {"attachments", input.AttachmentsCount, "Dribbble Shot attachment count"},
	}
	for _, metric := range metrics {
		post.Metrics = append(post.Metrics, socialhub.Metric{Name: metric.name, Value: float64(metric.value), AsOf: observedAt, Definition: metric.definition})
	}
	return post
}

func videoSubtype(filename string) string {
	if strings.HasSuffix(strings.ToLower(filename), ".webm") {
		return "webm"
	}
	return "mp4"
}

func timePointer(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	copy := value
	return &copy
}

func stringPointer(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	copy := value
	return &copy
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

func durationPointer(value time.Duration) *time.Duration {
	if value <= 0 {
		return nil
	}
	return &value
}

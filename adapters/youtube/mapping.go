package youtube

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

var isoDuration = regexp.MustCompile(`^PT(?:(\d+)H)?(?:(\d+)M)?(?:(\d+(?:\.\d+)?)S)?$`)

func mapChannel(accountID socialhub.AccountID, input youtubeChannel) *socialhub.User {
	extension, _ := json.Marshal(input.Statistics)
	return &socialhub.User{
		Platform: "youtube", AccountID: accountID, ID: input.ID, Username: stringPointer(input.Snippet.CustomURL),
		DisplayName: stringPointer(input.Snippet.Title), AvatarURL: stringPointer(bestThumbnail(input.Snippet.Thumbnails).URL),
		ProfileURL: stringPointer("https://www.youtube.com/channel/" + input.ID), AccountType: stringPointer("channel"),
		Extensions: map[string]json.RawMessage{"youtube.channel": extension},
	}
}

func mapVideo(accountID socialhub.AccountID, input youtubeVideo, observedAt time.Time) *socialhub.Post {
	state := socialhub.PublishStatePending
	if input.Status.UploadStatus == "processed" {
		state = socialhub.PublishStatePublished
	} else if input.Status.UploadStatus == "failed" || input.Status.UploadStatus == "rejected" || input.Status.UploadStatus == "deleted" {
		state = socialhub.PublishStateFailed
	}
	duration := parseISODuration(input.ContentDetails.Duration)
	thumb := bestThumbnail(input.Snippet.Thumbnails)
	mediaExtension, _ := json.Marshal(struct {
		ThumbnailURL string `json:"thumbnail_url,omitempty"`
		UploadStatus string `json:"upload_status,omitempty"`
	}{ThumbnailURL: thumb.URL, UploadStatus: input.Status.UploadStatus})
	postExtension, _ := json.Marshal(struct {
		MadeForKids             bool `json:"made_for_kids,omitempty"`
		SelfDeclaredMadeForKids bool `json:"self_declared_made_for_kids,omitempty"`
		ContainsSyntheticMedia  bool `json:"contains_synthetic_media,omitempty"`
	}{input.Status.MadeForKids, input.Status.SelfDeclaredMadeForKids, input.Status.ContainsSyntheticMedia})
	post := &socialhub.Post{
		Platform: "youtube", AccountID: accountID, ID: input.ID, AuthorID: stringPointer(input.Snippet.ChannelID),
		Text: stringPointer(firstNonEmpty(input.Snippet.Description, input.Snippet.Title)), CreatedAt: input.Snippet.PublishedAt,
		URL: stringPointer("https://www.youtube.com/watch?v=" + input.ID), Visibility: stringPointer(input.Status.PrivacyStatus),
		Status:     &socialhub.PublishStatus{ID: input.ID, State: state, UpdatedAt: input.Snippet.PublishedAt},
		Media:      []socialhub.Media{{ID: input.ID, Type: socialhub.MediaTypeVideo, Duration: duration, Width: intPointer(thumb.Width), Height: intPointer(thumb.Height), State: mediaState(state), Extensions: map[string]json.RawMessage{"youtube.video": mediaExtension}}},
		Extensions: map[string]json.RawMessage{"youtube.video": postExtension},
	}
	for name, value := range map[string]string{"views": input.Statistics.ViewCount, "likes": input.Statistics.LikeCount, "comments": input.Statistics.CommentCount, "favorites": input.Statistics.FavoriteCount} {
		if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
			post.Metrics = append(post.Metrics, socialhub.Metric{Name: name, Value: float64(parsed), AsOf: observedAt, Definition: "YouTube video " + name + " count"})
		}
	}
	return post
}

func mapSearchResult(accountID socialhub.AccountID, input searchResult) socialhub.Post {
	thumb := bestThumbnail(input.Snippet.Thumbnails)
	return socialhub.Post{
		Platform: "youtube", AccountID: accountID, ID: input.ID.VideoID, AuthorID: stringPointer(input.Snippet.ChannelID),
		Text: stringPointer(firstNonEmpty(input.Snippet.Description, input.Snippet.Title)), CreatedAt: input.Snippet.PublishedAt,
		URL:    stringPointer("https://www.youtube.com/watch?v=" + input.ID.VideoID),
		Status: &socialhub.PublishStatus{ID: input.ID.VideoID, State: socialhub.PublishStatePublished, UpdatedAt: input.Snippet.PublishedAt},
		Media:  []socialhub.Media{{ID: input.ID.VideoID, Type: socialhub.MediaTypeVideo, Width: intPointer(thumb.Width), Height: intPointer(thumb.Height), State: socialhub.MediaStateReady}},
	}
}

func mapComment(accountID socialhub.AccountID, postID string, input youtubeComment, observedAt time.Time) socialhub.Comment {
	var metrics []socialhub.Metric
	if input.Snippet.LikeCount > 0 {
		metrics = append(metrics, socialhub.Metric{Name: "likes", Value: float64(input.Snippet.LikeCount), AsOf: observedAt, Definition: "YouTube comment likeCount"})
	}
	return socialhub.Comment{
		Platform: "youtube", AccountID: accountID, ID: input.ID, PostID: postID,
		AuthorID: stringPointer(input.Snippet.AuthorChannelID.Value), ParentID: stringPointer(input.Snippet.ParentID),
		Text: input.Snippet.TextOriginal, CreatedAt: input.Snippet.PublishedAt, Metrics: metrics,
	}
}

func bestThumbnail(input thumbnails) thumbnail {
	for _, value := range []thumbnail{input.Maxres, input.High, input.Medium, input.Default} {
		if value.URL != "" {
			return value
		}
	}
	return thumbnail{}
}

func parseISODuration(value string) *time.Duration {
	matches := isoDuration.FindStringSubmatch(value)
	if matches == nil {
		return nil
	}
	var seconds float64
	if matches[1] != "" {
		hours, _ := strconv.ParseFloat(matches[1], 64)
		seconds += hours * 3600
	}
	if matches[2] != "" {
		minutes, _ := strconv.ParseFloat(matches[2], 64)
		seconds += minutes * 60
	}
	if matches[3] != "" {
		value, _ := strconv.ParseFloat(matches[3], 64)
		seconds += value
	}
	duration := time.Duration(seconds * float64(time.Second))
	return &duration
}

func mediaState(value socialhub.PublishState) socialhub.MediaState {
	switch value {
	case socialhub.PublishStatePublished:
		return socialhub.MediaStateReady
	case socialhub.PublishStateFailed:
		return socialhub.MediaStateFailed
	default:
		return socialhub.MediaStateProcessing
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
	if value <= 0 {
		return nil
	}
	copy := value
	return &copy
}

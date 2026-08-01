package tiktok

import (
	"encoding/json"
	"time"

	"social-hub/pkg/socialhub"
)

func mapUser(accountID socialhub.AccountID, input tiktokUser) *socialhub.User {
	extension, _ := json.Marshal(struct {
		UnionID        string `json:"union_id,omitempty"`
		Bio            string `json:"bio_description,omitempty"`
		Verified       bool   `json:"is_verified,omitempty"`
		FollowerCount  int64  `json:"follower_count,omitempty"`
		FollowingCount int64  `json:"following_count,omitempty"`
		LikesCount     int64  `json:"likes_count,omitempty"`
		VideoCount     int64  `json:"video_count,omitempty"`
	}{
		UnionID: input.UnionID, Bio: input.BioDescription, Verified: input.IsVerified,
		FollowerCount: input.FollowerCount, FollowingCount: input.FollowingCount, LikesCount: input.LikesCount, VideoCount: input.VideoCount,
	})
	return &socialhub.User{
		Platform: "tiktok", AccountID: accountID, ID: input.OpenID, Username: stringPointer(input.Username),
		DisplayName: stringPointer(input.DisplayName), AvatarURL: stringPointer(input.AvatarURL), ProfileURL: stringPointer(input.ProfileDeepLink),
		AccountType: stringPointer("creator"), Extensions: map[string]json.RawMessage{"tiktok.user": extension},
	}
}

func mapVideo(accountID socialhub.AccountID, authorID string, input tiktokVideo, observedAt time.Time) *socialhub.Post {
	var createdAt *time.Time
	if input.CreateTime > 0 {
		value := time.Unix(input.CreateTime, 0).UTC()
		createdAt = &value
	}
	var duration *time.Duration
	if input.Duration > 0 {
		value := time.Duration(input.Duration) * time.Second
		duration = &value
	}
	extension, _ := json.Marshal(struct {
		CoverImageURL string `json:"cover_image_url,omitempty"`
		EmbedLink     string `json:"embed_link,omitempty"`
		EmbedHTML     string `json:"embed_html,omitempty"`
	}{CoverImageURL: input.CoverImageURL, EmbedLink: input.EmbedLink, EmbedHTML: input.EmbedHTML})
	post := &socialhub.Post{
		Platform: "tiktok", AccountID: accountID, ID: input.ID, AuthorID: stringPointer(authorID),
		Text: stringPointer(firstNonEmpty(input.VideoDescription, input.Title)), CreatedAt: createdAt, URL: stringPointer(input.ShareURL),
		Status: &socialhub.PublishStatus{ID: input.ID, State: socialhub.PublishStatePublished, UpdatedAt: createdAt},
		Media:  []socialhub.Media{{ID: input.ID, Type: socialhub.MediaTypeVideo, Duration: duration, Width: intPointer(input.Width), Height: intPointer(input.Height), State: socialhub.MediaStateReady, Extensions: map[string]json.RawMessage{"tiktok.video": extension}}},
	}
	for name, value := range map[string]int64{"likes": input.LikeCount, "comments": input.CommentCount, "shares": input.ShareCount, "views": input.ViewCount} {
		post.Metrics = append(post.Metrics, socialhub.Metric{Name: name, Value: float64(value), AsOf: observedAt, Definition: "TikTok public video " + name + " count"})
	}
	return post
}

func stringPointer(value string) *string {
	if value == "" {
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

package kuaishou

import (
	"encoding/json"
	"time"

	"social-hub/pkg/socialhub"
)

type kuaishouUser struct {
	Name    string `json:"name"`
	Sex     string `json:"sex"`
	Fan     int64  `json:"fan"`
	Follow  int64  `json:"follow"`
	Head    string `json:"head"`
	BigHead string `json:"bigHead"`
	City    string `json:"city"`
}

type userEnvelope struct {
	resultEnvelope
	User kuaishouUser `json:"user_info"`
}

type videoInfo struct {
	PhotoID      string `json:"photo_id"`
	Caption      string `json:"caption"`
	Cover        string `json:"cover"`
	PlayURL      string `json:"play_url"`
	CreateTime   int64  `json:"create_time"`
	LikeCount    int64  `json:"like_count"`
	CommentCount int64  `json:"comment_count"`
	ViewCount    int64  `json:"view_count"`
	Pending      bool   `json:"pending"`
}

type publishEnvelope struct {
	resultEnvelope
	Video videoInfo `json:"video_info"`
}

func mapUser(accountID socialhub.AccountID, openID string, input kuaishouUser, observedAt time.Time) *socialhub.User {
	extension, _ := json.Marshal(map[string]any{
		"sex":         input.Sex,
		"city":        input.City,
		"fan":         input.Fan,
		"follow":      input.Follow,
		"big_head":    input.BigHead,
		"observed_at": observedAt,
	})
	return &socialhub.User{
		Platform: "kuaishou", AccountID: accountID, ID: openID,
		DisplayName: stringPointer(input.Name), AvatarURL: stringPointer(firstNonEmpty(input.BigHead, input.Head)),
		Extensions: map[string]json.RawMessage{"kuaishou.user": extension},
	}
}

func mapVideo(accountID socialhub.AccountID, openID string, input videoInfo, observedAt time.Time) *socialhub.Post {
	var createdAt *time.Time
	if input.CreateTime > 0 {
		value := time.UnixMilli(input.CreateTime)
		createdAt = &value
	}
	state := socialhub.PublishStatePublished
	mediaState := socialhub.MediaStateReady
	message := ""
	if input.Pending {
		state = socialhub.PublishStatePending
		mediaState = socialhub.MediaStateProcessing
		message = "submitted for Kuaishou processing"
	}
	mediaExtension, _ := json.Marshal(map[string]any{"cover": input.Cover})
	return &socialhub.Post{
		Platform: "kuaishou", AccountID: accountID, ID: input.PhotoID, AuthorID: stringPointer(openID), Text: stringPointer(input.Caption),
		Media:     []socialhub.Media{{URL: input.PlayURL, Type: socialhub.MediaTypeVideo, State: mediaState, Extensions: map[string]json.RawMessage{"kuaishou.video": mediaExtension}}},
		CreatedAt: createdAt, URL: stringPointer(input.PlayURL),
		Status: &socialhub.PublishStatus{ID: input.PhotoID, State: state, Message: message, UpdatedAt: createdAt},
		Metrics: []socialhub.Metric{
			{Name: "likes", Value: float64(input.LikeCount), AsOf: observedAt, Definition: "Kuaishou video_info.like_count"},
			{Name: "comments", Value: float64(input.CommentCount), AsOf: observedAt, Definition: "Kuaishou video_info.comment_count"},
			{Name: "views", Value: float64(input.ViewCount), AsOf: observedAt, Definition: "Kuaishou video_info.view_count"},
		},
	}
}

func stringPointer(value string) *string {
	if value == "" {
		return nil
	}
	copy := value
	return &copy
}

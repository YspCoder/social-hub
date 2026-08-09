package toutiao

import (
	"encoding/json"
	"time"

	"social-hub/pkg/socialhub"
)

type userData struct {
	apiResponse
	OpenID   string `json:"open_id"`
	UnionID  string `json:"union_id"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
}

type userEnvelope struct {
	Data  userData      `json:"data"`
	Extra responseExtra `json:"extra"`
}

type videoStatistics struct {
	DiggCount    int64 `json:"digg_count"`
	PlayCount    int64 `json:"play_count"`
	ShareCount   int64 `json:"share_count"`
	ForwardCount int64 `json:"forward_count"`
	CommentCount int64 `json:"comment_count"`
}

type toutiaoVideo struct {
	ItemID     string          `json:"item_id"`
	VideoID    string          `json:"video_id"`
	Title      string          `json:"title"`
	Cover      string          `json:"cover"`
	CreateTime int64           `json:"create_time"`
	Statistics videoStatistics `json:"statistics"`
}

type videoListData struct {
	apiResponse
	List    []toutiaoVideo `json:"list"`
	Cursor  flexibleInt64  `json:"cursor"`
	HasMore bool           `json:"has_more"`
}

type videoListEnvelope struct {
	Data  videoListData `json:"data"`
	Extra responseExtra `json:"extra"`
}

type createVideoEnvelope struct {
	Data struct {
		apiResponse
		ItemID string `json:"item_id"`
	} `json:"data"`
	Extra responseExtra `json:"extra"`
}

type uploadVideo struct {
	VideoID string `json:"video_id"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
}

type videoUploadEnvelope struct {
	Data struct {
		apiResponse
		Video uploadVideo `json:"video"`
	} `json:"data"`
	Extra responseExtra `json:"extra"`
}

type uploadInitEnvelope struct {
	Data struct {
		apiResponse
		UploadID string `json:"upload_id"`
	} `json:"data"`
	Extra responseExtra `json:"extra"`
}

func mapUser(accountID socialhub.AccountID, input userData) *socialhub.User {
	extension, _ := json.Marshal(map[string]string{"union_id": input.UnionID})
	return &socialhub.User{
		Platform: platformName, AccountID: accountID, ID: input.OpenID,
		DisplayName: stringPointer(input.Nickname), AvatarURL: stringPointer(input.Avatar),
		Extensions: map[string]json.RawMessage{"toutiao.user": extension},
	}
}

func mapVideo(accountID socialhub.AccountID, openID string, input toutiaoVideo, observedAt time.Time) *socialhub.Post {
	var createdAt *time.Time
	if input.CreateTime > 0 {
		value := time.Unix(input.CreateTime, 0)
		createdAt = &value
	}
	mediaExtension, _ := json.Marshal(map[string]string{"cover": input.Cover})
	metrics := []socialhub.Metric{
		{Name: "digg", Value: float64(input.Statistics.DiggCount), AsOf: observedAt, Definition: "Toutiao statistics.digg_count"},
		{Name: "comments", Value: float64(input.Statistics.CommentCount), AsOf: observedAt, Definition: "Toutiao statistics.comment_count"},
		{Name: "plays", Value: float64(input.Statistics.PlayCount), AsOf: observedAt, Definition: "Toutiao statistics.play_count"},
		{Name: "shares", Value: float64(input.Statistics.ShareCount), AsOf: observedAt, Definition: "Toutiao statistics.share_count"},
		{Name: "forwards", Value: float64(input.Statistics.ForwardCount), AsOf: observedAt, Definition: "Toutiao statistics.forward_count"},
	}
	return &socialhub.Post{
		Platform: platformName, AccountID: accountID, ID: input.ItemID,
		AuthorID: stringPointer(openID), Text: stringPointer(input.Title), CreatedAt: createdAt,
		Media: []socialhub.Media{{
			ID: input.VideoID, Type: socialhub.MediaTypeVideo, State: socialhub.MediaStateReady,
			Extensions: map[string]json.RawMessage{"toutiao.video": mediaExtension},
		}},
		Status:  &socialhub.PublishStatus{ID: input.ItemID, State: socialhub.PublishStatePublished, UpdatedAt: createdAt},
		Metrics: metrics,
	}
}

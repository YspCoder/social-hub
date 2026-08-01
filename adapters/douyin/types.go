package douyin

import (
	"encoding/json"
	"time"

	"social-hub/pkg/socialhub"
)

type baseEnvelope struct {
	Data  APIResponse   `json:"data"`
	Extra responseExtra `json:"extra"`
}

type douyinUser struct {
	APIResponse
	OpenID      string `json:"open_id"`
	UnionID     string `json:"union_id"`
	Nickname    string `json:"nickname"`
	Avatar      string `json:"avatar"`
	Country     string `json:"country"`
	Province    string `json:"province"`
	City        string `json:"city"`
	Gender      int    `json:"gender"`
	AccountRole string `json:"e_account_role"`
}

type userEnvelope struct {
	Data  douyinUser    `json:"data"`
	Extra responseExtra `json:"extra"`
}

type douyinStatistics struct {
	DiggCount     int64 `json:"digg_count"`
	DownloadCount int64 `json:"download_count"`
	PlayCount     int64 `json:"play_count"`
	ShareCount    int64 `json:"share_count"`
	ForwardCount  int64 `json:"forward_count"`
	CommentCount  int64 `json:"comment_count"`
}

type douyinVideo struct {
	ItemID      string           `json:"item_id"`
	VideoID     string           `json:"video_id"`
	Title       string           `json:"title"`
	CreateTime  int64            `json:"create_time"`
	VideoStatus int              `json:"video_status"`
	ShareURL    string           `json:"share_url"`
	Cover       string           `json:"cover"`
	IsTop       bool             `json:"is_top"`
	IsReviewed  bool             `json:"is_reviewed"`
	MediaType   int              `json:"media_type"`
	Statistics  douyinStatistics `json:"statistics"`
}

type videoListData struct {
	APIResponse
	List    []douyinVideo `json:"list"`
	Cursor  flexibleInt64 `json:"cursor"`
	HasMore bool          `json:"has_more"`
}

type videoListEnvelope struct {
	Data  videoListData `json:"data"`
	Extra responseExtra `json:"extra"`
}

type createVideoResponse struct {
	Data struct {
		APIResponse
		ItemID string `json:"item_id"`
	} `json:"data"`
	Extra responseExtra `json:"extra"`
}

type douyinComment struct {
	CommentID         string `json:"comment_id"`
	CommentUserID     string `json:"comment_user_id"`
	Content           string `json:"content"`
	CreateTime        int64  `json:"create_time"`
	DiggCount         int64  `json:"digg_count"`
	ReplyCommentTotal int64  `json:"reply_comment_total"`
	ReplyToCommentID  string `json:"reply_to_comment_id"`
}

type commentListData struct {
	APIResponse
	List    []douyinComment `json:"list"`
	Cursor  flexibleInt64   `json:"cursor"`
	HasMore bool            `json:"has_more"`
}

type commentListEnvelope struct {
	Data  commentListData `json:"data"`
	Extra responseExtra   `json:"extra"`
}

type commentReplyEnvelope struct {
	Data struct {
		APIResponse
		CommentID string `json:"comment_id"`
	} `json:"data"`
	Extra responseExtra `json:"extra"`
}

func mapUser(accountID socialhub.AccountID, input douyinUser) *socialhub.User {
	accountType := "personal"
	if input.AccountRole != "" {
		accountType = input.AccountRole
	}
	extension, _ := json.Marshal(map[string]any{
		"union_id": input.UnionID,
		"country":  input.Country,
		"province": input.Province,
		"city":     input.City,
		"gender":   input.Gender,
	})
	return &socialhub.User{
		Platform:    "douyin",
		AccountID:   accountID,
		ID:          input.OpenID,
		DisplayName: stringPointer(input.Nickname),
		AvatarURL:   stringPointer(input.Avatar),
		AccountType: stringPointer(accountType),
		Extensions:  map[string]json.RawMessage{"douyin.user": extension},
	}
}

func mapVideo(accountID socialhub.AccountID, openID string, input douyinVideo, observedAt time.Time) *socialhub.Post {
	createdAt := time.Unix(input.CreateTime, 0)
	var created *time.Time
	if input.CreateTime > 0 {
		created = &createdAt
	}
	state := socialhub.PublishStatePending
	message := "under review"
	mediaState := socialhub.MediaStateProcessing
	if input.IsReviewed {
		state = socialhub.PublishStatePublished
		message = ""
		mediaState = socialhub.MediaStateReady
	}
	mediaExtension, _ := json.Marshal(map[string]any{"cover": input.Cover})
	postExtension, _ := json.Marshal(map[string]any{"video_status": input.VideoStatus, "is_reviewed": input.IsReviewed, "is_top": input.IsTop, "media_type": input.MediaType})
	metrics := []socialhub.Metric{
		{Name: "digg", Value: float64(input.Statistics.DiggCount), AsOf: observedAt, Definition: "Douyin statistics.digg_count"},
		{Name: "comments", Value: float64(input.Statistics.CommentCount), AsOf: observedAt, Definition: "Douyin statistics.comment_count"},
		{Name: "plays", Value: float64(input.Statistics.PlayCount), AsOf: observedAt, Definition: "Douyin statistics.play_count"},
		{Name: "shares", Value: float64(input.Statistics.ShareCount), AsOf: observedAt, Definition: "Douyin statistics.share_count"},
		{Name: "forwards", Value: float64(input.Statistics.ForwardCount), AsOf: observedAt, Definition: "Douyin statistics.forward_count"},
		{Name: "downloads", Value: float64(input.Statistics.DownloadCount), AsOf: observedAt, Definition: "Douyin statistics.download_count"},
	}
	return &socialhub.Post{
		Platform:  "douyin",
		AccountID: accountID,
		ID:        input.ItemID,
		AuthorID:  stringPointer(openID),
		Text:      stringPointer(input.Title),
		Media: []socialhub.Media{{
			ID:         input.VideoID,
			Type:       socialhub.MediaTypeVideo,
			State:      mediaState,
			Extensions: map[string]json.RawMessage{"douyin.video": mediaExtension},
		}},
		CreatedAt:  created,
		URL:        stringPointer(input.ShareURL),
		Status:     &socialhub.PublishStatus{ID: input.ItemID, State: state, Message: message, UpdatedAt: created},
		Metrics:    metrics,
		Extensions: map[string]json.RawMessage{"douyin.post": postExtension},
	}
}

func mapComment(accountID socialhub.AccountID, postID string, input douyinComment, observedAt time.Time) socialhub.Comment {
	createdAt := time.Unix(input.CreateTime, 0)
	var created *time.Time
	if input.CreateTime > 0 {
		created = &createdAt
	}
	comment := socialhub.Comment{Platform: "douyin", AccountID: accountID, ID: input.CommentID, PostID: postID, AuthorID: stringPointer(input.CommentUserID), Text: input.Content, CreatedAt: created}
	if input.ReplyToCommentID != "" {
		comment.ParentID = stringPointer(input.ReplyToCommentID)
	}
	comment.Metrics = []socialhub.Metric{{Name: "digg", Value: float64(input.DiggCount), AsOf: observedAt, Definition: "Douyin comment digg_count"}}
	return comment
}

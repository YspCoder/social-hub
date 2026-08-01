package instagram

import "time"

type instagramUser struct {
	ID                string `json:"id"`
	UserID            string `json:"user_id"`
	Username          string `json:"username"`
	Name              string `json:"name"`
	AccountType       string `json:"account_type"`
	ProfilePictureURL string `json:"profile_picture_url"`
	FollowersCount    int64  `json:"followers_count"`
	MediaCount        int64  `json:"media_count"`
}

type instagramMedia struct {
	ID               string             `json:"id"`
	Caption          string             `json:"caption"`
	MediaType        string             `json:"media_type"`
	MediaProductType string             `json:"media_product_type"`
	MediaURL         string             `json:"media_url"`
	Permalink        string             `json:"permalink"`
	ThumbnailURL     string             `json:"thumbnail_url"`
	Timestamp        *time.Time         `json:"timestamp"`
	Username         string             `json:"username"`
	Children         instagramMediaList `json:"children"`
}

type instagramMediaList struct {
	Data   []instagramMedia `json:"data"`
	Paging graphPaging      `json:"paging"`
}

type instagramComment struct {
	ID        string     `json:"id"`
	Text      string     `json:"text"`
	Timestamp *time.Time `json:"timestamp"`
	Username  string     `json:"username"`
	From      struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	} `json:"from"`
	ParentID  string `json:"parent_id"`
	LikeCount int64  `json:"like_count"`
}

type instagramCommentList struct {
	Data   []instagramComment `json:"data"`
	Paging graphPaging        `json:"paging"`
}

type graphPaging struct {
	Cursors struct {
		Before *string `json:"before"`
		After  *string `json:"after"`
	} `json:"cursors"`
	Next string `json:"next"`
}

type idResponse struct {
	ID string `json:"id"`
}

type successResponse struct {
	Success bool `json:"success"`
}

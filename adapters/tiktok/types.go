package tiktok

type userEnvelope struct {
	Data struct {
		User tiktokUser `json:"user"`
	} `json:"data"`
	Error apiError `json:"error"`
}

type tiktokUser struct {
	OpenID          string `json:"open_id"`
	UnionID         string `json:"union_id"`
	AvatarURL       string `json:"avatar_url"`
	DisplayName     string `json:"display_name"`
	BioDescription  string `json:"bio_description"`
	ProfileDeepLink string `json:"profile_deep_link"`
	Username        string `json:"username"`
	IsVerified      bool   `json:"is_verified"`
	FollowerCount   int64  `json:"follower_count"`
	FollowingCount  int64  `json:"following_count"`
	LikesCount      int64  `json:"likes_count"`
	VideoCount      int64  `json:"video_count"`
}

type videoEnvelope struct {
	Data struct {
		Videos  []tiktokVideo `json:"videos"`
		Cursor  int64         `json:"cursor"`
		HasMore bool          `json:"has_more"`
	} `json:"data"`
	Error apiError `json:"error"`
}

type tiktokVideo struct {
	ID               string `json:"id"`
	CreateTime       int64  `json:"create_time"`
	CoverImageURL    string `json:"cover_image_url"`
	ShareURL         string `json:"share_url"`
	VideoDescription string `json:"video_description"`
	Duration         int64  `json:"duration"`
	Height           int    `json:"height"`
	Width            int    `json:"width"`
	Title            string `json:"title"`
	EmbedHTML        string `json:"embed_html"`
	EmbedLink        string `json:"embed_link"`
	LikeCount        int64  `json:"like_count"`
	CommentCount     int64  `json:"comment_count"`
	ShareCount       int64  `json:"share_count"`
	ViewCount        int64  `json:"view_count"`
}

type creatorEnvelope struct {
	Data  CreatorInfo `json:"data"`
	Error apiError    `json:"error"`
}

type publishEnvelope struct {
	Data struct {
		PublishID string `json:"publish_id"`
		UploadURL string `json:"upload_url"`
	} `json:"data"`
	Error apiError `json:"error"`
}

type statusEnvelope struct {
	Data struct {
		Status                   string   `json:"status"`
		FailReason               string   `json:"fail_reason"`
		PubliclyAvailablePostIDs []string `json:"publicaly_available_post_id"`
		UploadedBytes            int64    `json:"uploaded_bytes"`
	} `json:"data"`
	Error apiError `json:"error"`
}
